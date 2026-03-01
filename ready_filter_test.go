package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestReadyFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create goal A (no dependencies)
	idA, err := createGoal(db, "org1", "repo1", "Goal A", "Body A", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create goal B (depends on A)
	idB, err := createGoal(db, "org1", "repo1", "Goal B", "Body B", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Queue both goals
	for _, id := range []int64{idA, idB} {
		if err := updateGoalStatus(db, id, "draft", "queued"); err != nil {
			t.Fatal(err)
		}
	}

	// Add dependency: B depends on A
	if err := addDependency(db, idB, idA); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerRoutes(mux, db)

	goalIDs := func(items []any) []int {
		var ids []int
		for _, item := range items {
			m := item.(map[string]any)
			ids = append(ids, int(m["id"].(float64)))
		}
		return ids
	}

	contains := func(ids []int, id int64) bool {
		for _, v := range ids {
			if int64(v) == id {
				return true
			}
		}
		return false
	}

	t.Run("without ready returns all queued goals", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?status=queued", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		items := resp["items"].([]any)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("ready=true excludes B (unmet dependency)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?status=queued&ready=true", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		items := resp["items"].([]any)
		ids := goalIDs(items)

		if !contains(ids, idA) {
			t.Fatalf("expected goal A (id=%d) to be in ready results, got ids=%v", idA, ids)
		}
		if contains(ids, idB) {
			t.Fatalf("expected goal B (id=%d) to be excluded from ready results, got ids=%v", idB, ids)
		}
	})

	t.Run("after marking A done, B appears in ready results", func(t *testing.T) {
		// Transition A to done: queued -> running -> done
		if err := updateGoalStatus(db, idA, "queued", "running"); err != nil {
			t.Fatal(err)
		}
		if err := updateGoalStatus(db, idA, "running", "done"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/goals?status=queued&ready=true", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		items := resp["items"].([]any)
		ids := goalIDs(items)

		if !contains(ids, idB) {
			t.Fatalf("expected goal B (id=%d) to appear after A is done, got ids=%v", idB, ids)
		}
	})
}
