package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
)

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

	t.Run("running to done transition works", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Transition", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		transitionToRunning(t, db, id)

		// Transition to done via API
		req := httptest.NewRequest("PATCH", "/goals/"+strconv.FormatInt(id, 10)+"/done", nil)
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
		if g.Status != "done" {
			t.Fatalf("expected status=done, got %s", g.Status)
		}
	})

	t.Run("full lifecycle draft to done", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Full Lifecycle", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		if err := updateGoalStatus(db, id, "draft", "queued"); err != nil {
			t.Fatal(err)
		}
		if err := updateGoalStatus(db, id, "queued", "running"); err != nil {
			t.Fatal(err)
		}
		if err := updateGoalStatus(db, id, "running", "done"); err != nil {
			t.Fatal(err)
		}

		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.Status != "done" {
			t.Fatalf("expected status=done, got %s", g.Status)
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
		{"done", true},
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
		{"queued", "cancelled", true},
		{"running", "done", true},
		{"running", "stuck", true},
		{"running", "cancelled", true},
		{"running", "queued", false},
		{"stuck", "queued", true},
		{"stuck", "cancelled", true},
		{"done", "running", false},
		{"done", "cancelled", false},
		{"cancelled", "draft", false},
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

	t.Run("cannot cancel done goal", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Cancel Done", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		transitionToRunning(t, db, id)
		if err := updateGoalStatus(db, id, "running", "done"); err != nil {
			t.Fatal(err)
		}

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

	t.Run("cannot cancel cancelled goal", func(t *testing.T) {
		id, err := createGoal(db, "org", "repo", "Test Cancel Cancelled", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := updateGoalStatus(db, id, "draft", "cancelled"); err != nil {
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

// Helper function to transition a goal to running status
func transitionToRunning(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	if err := updateGoalStatus(db, id, "draft", "queued"); err != nil {
		t.Fatal(err)
	}
	if err := updateGoalStatus(db, id, "queued", "running"); err != nil {
		t.Fatal(err)
	}
}
