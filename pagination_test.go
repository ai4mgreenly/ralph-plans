package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestPagination(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create test goals
	for i := 1; i <= 15; i++ {
		_, err := createGoal(db, "org1", "repo1", "Goal "+string(rune('A'+i-1)), "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Transition all goals to done for easier filtering
	for i := 1; i <= 15; i++ {
		err := updateGoalStatus(db, int64(i), "draft", "queued")
		if err != nil {
			t.Fatal(err)
		}
		err = updateGoalStatus(db, int64(i), "queued", "running")
		if err != nil {
			t.Fatal(err)
		}
		err = updateGoalStatus(db, int64(i), "running", "done")
		if err != nil {
			t.Fatal(err)
		}
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	registerRoutes(mux, db)

	t.Run("unpaginated returns all results", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?status=done", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if !resp["ok"].(bool) {
			t.Fatal("expected ok=true")
		}

		items := resp["items"].([]any)
		if len(items) != 15 {
			t.Fatalf("expected 15 items, got %d", len(items))
		}

		// Should NOT have pagination metadata
		if _, exists := resp["page"]; exists {
			t.Fatal("unpaginated response should not have 'page' field")
		}
		if _, exists := resp["per_page"]; exists {
			t.Fatal("unpaginated response should not have 'per_page' field")
		}
		if _, exists := resp["total"]; exists {
			t.Fatal("unpaginated response should not have 'total' field")
		}
	})

	t.Run("paginated first page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?status=done&page=1&per_page=5", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if !resp["ok"].(bool) {
			t.Fatal("expected ok=true")
		}

		items := resp["items"].([]any)
		if len(items) != 5 {
			t.Fatalf("expected 5 items, got %d", len(items))
		}

		if resp["page"].(float64) != 1 {
			t.Fatalf("expected page=1, got %v", resp["page"])
		}
		if resp["per_page"].(float64) != 5 {
			t.Fatalf("expected per_page=5, got %v", resp["per_page"])
		}
		if resp["total"].(float64) != 15 {
			t.Fatalf("expected total=15, got %v", resp["total"])
		}
	})

	t.Run("paginated second page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?status=done&page=2&per_page=5", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		items := resp["items"].([]any)
		if len(items) != 5 {
			t.Fatalf("expected 5 items, got %d", len(items))
		}

		if resp["page"].(float64) != 2 {
			t.Fatalf("expected page=2, got %v", resp["page"])
		}

		// Verify offset - second page should have IDs 10-6 (descending order)
		firstItem := items[0].(map[string]any)
		firstID := int(firstItem["id"].(float64))
		if firstID != 10 {
			t.Fatalf("expected first item on page 2 to have id=10, got %d", firstID)
		}
	})

	t.Run("per_page defaults to 20", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?status=done&page=1", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)

		if resp["per_page"].(float64) != 20 {
			t.Fatalf("expected per_page=20 (default), got %v", resp["per_page"])
		}
	})

	t.Run("per_page clamped to 100", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?status=done&page=1&per_page=200", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)

		if resp["per_page"].(float64) != 100 {
			t.Fatalf("expected per_page=100 (clamped), got %v", resp["per_page"])
		}
	})

	t.Run("invalid page returns error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?page=0", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)

		if resp["ok"].(bool) {
			t.Fatal("expected ok=false for invalid page")
		}
	})

	t.Run("invalid per_page returns error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?page=1&per_page=-5", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)

		if resp["ok"].(bool) {
			t.Fatal("expected ok=false for invalid per_page")
		}
	})

	t.Run("non-numeric page returns error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/goals?page=abc", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}
