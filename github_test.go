package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestPRCache(t *testing.T) {
	cache := newPRCache()

	t.Run("get returns false for missing entry", func(t *testing.T) {
		_, ok := cache.get("org", "repo", 123)
		if ok {
			t.Fatal("expected ok=false for missing entry")
		}
	})

	t.Run("set and get within TTL", func(t *testing.T) {
		state := PRState{Merged: true, Closed: true, Open: false}
		cache.set("org", "repo", 123, state)

		retrieved, ok := cache.get("org", "repo", 123)
		if !ok {
			t.Fatal("expected ok=true for cached entry")
		}
		if !retrieved.Merged {
			t.Fatal("expected merged=true")
		}
	})

	t.Run("get returns false after TTL expires", func(t *testing.T) {
		cache := newPRCache()
		state := PRState{Open: true}
		cache.set("org", "repo", 456, state)

		// Manually expire the cache entry
		key := "org/repo/456"
		cache.mu.Lock()
		entry := cache.entries[key]
		entry.ExpiresAt = time.Now().Add(-1 * time.Second)
		cache.entries[key] = entry
		cache.mu.Unlock()

		_, ok := cache.get("org", "repo", 456)
		if ok {
			t.Fatal("expected ok=false for expired entry")
		}
	})

	t.Run("different goals have separate cache entries", func(t *testing.T) {
		cache := newPRCache()
		cache.set("org", "repo", 1, PRState{Merged: true})
		cache.set("org", "repo", 2, PRState{Open: true})

		state1, ok1 := cache.get("org", "repo", 1)
		state2, ok2 := cache.get("org", "repo", 2)

		if !ok1 || !ok2 {
			t.Fatal("expected both entries to be cached")
		}
		if !state1.Merged || state2.Merged {
			t.Fatal("states should be independent")
		}
	})
}

func TestGetGoalNoPRCheck(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	t.Run("draft goal does not trigger PR check", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := updateGoalPR(db, id, 123); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/goals/"+strconv.FormatInt(id, 10), nil)
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["status"] != "draft" {
			t.Fatalf("expected status=draft, got %v", resp["status"])
		}
	})

	t.Run("submitted goal without PR does not trigger check", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test 2", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		// Transition to submitted
		if err := updateGoalStatus(db, id, "draft", "queued"); err != nil {
			t.Fatal(err)
		}
		if err := updateGoalStatus(db, id, "queued", "running"); err != nil {
			t.Fatal(err)
		}
		if err := updateGoalStatus(db, id, "running", "submitted"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/goals/"+strconv.FormatInt(id, 10), nil)
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["status"] != "submitted" {
			t.Fatalf("expected status=submitted, got %v", resp["status"])
		}
		if resp["pr"] != nil {
			t.Fatalf("expected pr=null, got %v", resp["pr"])
		}
	})
}

func TestStatusTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	t.Run("running to submitted transition works", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Transition", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		// Transition to running
		if err := updateGoalStatus(db, id, "draft", "queued"); err != nil {
			t.Fatal(err)
		}
		if err := updateGoalStatus(db, id, "queued", "running"); err != nil {
			t.Fatal(err)
		}

		// Transition to submitted via API
		req := httptest.NewRequest("PATCH", "/goals/"+strconv.FormatInt(id, 10)+"/submitted", nil)
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify status
		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.Status != "submitted" {
			t.Fatalf("expected status=submitted, got %s", g.Status)
		}
	})

	t.Run("submitted to merged transition works", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Merged", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		transitionToSubmitted(t, db, id)

		// Manually transition to merged
		if err := updateGoalStatus(db, id, "submitted", "merged"); err != nil {
			t.Fatal(err)
		}

		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.Status != "merged" {
			t.Fatalf("expected status=merged, got %s", g.Status)
		}
	})

	t.Run("submitted to rejected transition works", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Rejected", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		transitionToSubmitted(t, db, id)

		// Manually transition to rejected
		if err := updateGoalStatus(db, id, "submitted", "rejected"); err != nil {
			t.Fatal(err)
		}

		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.Status != "rejected" {
			t.Fatalf("expected status=rejected, got %s", g.Status)
		}
	})
}

func TestTerminalStatuses(t *testing.T) {
	tests := []struct {
		status   string
		terminal bool
	}{
		{"draft", false},
		{"queued", false},
		{"running", false},
		{"submitted", true},
		{"merged", true},
		{"rejected", true},
		{"stuck", false},
		{"cancelled", true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := isTerminal(tt.status)
			if result != tt.terminal {
				t.Errorf("isTerminal(%q) = %v, want %v", tt.status, result, tt.terminal)
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from  string
		to    string
		valid bool
	}{
		{"draft", "queued", true},
		{"draft", "cancelled", true},
		{"draft", "running", false},
		{"queued", "running", true},
		{"running", "submitted", true},
		{"running", "stuck", true},
		{"running", "cancelled", true},
		{"running", "merged", false},
		{"submitted", "merged", true},
		{"submitted", "rejected", true},
		{"submitted", "running", false},
		{"stuck", "queued", true},
		{"merged", "submitted", false},
		{"rejected", "submitted", false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"_to_"+tt.to, func(t *testing.T) {
			result := canTransition(tt.from, tt.to)
			if result != tt.valid {
				t.Errorf("canTransition(%q, %q) = %v, want %v", tt.from, tt.to, result, tt.valid)
			}
		})
	}
}

func TestCancelTerminalGoal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	t.Run("cannot cancel submitted goal", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Cancel", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		transitionToSubmitted(t, db, id)

		req := httptest.NewRequest("PATCH", "/goals/"+strconv.FormatInt(id, 10)+"/cancel", nil)
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 409 {
			t.Fatalf("expected 409, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["ok"].(bool) {
			t.Fatal("expected ok=false for terminal goal")
		}
	})

	t.Run("cannot cancel merged goal", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Cancel Merged", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		transitionToSubmitted(t, db, id)
		if err := updateGoalStatus(db, id, "submitted", "merged"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("PATCH", "/goals/"+strconv.FormatInt(id, 10)+"/cancel", nil)
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 409 {
			t.Fatalf("expected 409, got %d", w.Code)
		}
	})
}

func TestGoalPRIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	t.Run("submitted goal with PR includes PR in response", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test PR Response", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		transitionToSubmitted(t, db, id)
		if err := updateGoalPR(db, id, 999); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/goals/"+strconv.FormatInt(id, 10), nil)
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["status"] != "submitted" {
			t.Fatalf("expected status=submitted, got %v", resp["status"])
		}
		if resp["pr"] == nil {
			t.Fatal("expected pr to be set")
		}
		prValue := int(resp["pr"].(float64))
		if prValue != 999 {
			t.Fatalf("expected pr=999, got %d", prValue)
		}
	})
}

// Helper function to transition a goal to submitted status
func transitionToSubmitted(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	if err := updateGoalStatus(db, id, "draft", "queued"); err != nil {
		t.Fatal(err)
	}
	if err := updateGoalStatus(db, id, "queued", "running"); err != nil {
		t.Fatal(err)
	}
	if err := updateGoalStatus(db, id, "running", "submitted"); err != nil {
		t.Fatal(err)
	}
}
