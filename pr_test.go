package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPRField(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create HTTP handler
	mux := http.NewServeMux()
	registerRoutes(mux, db)

	t.Run("create goal without pr returns null", func(t *testing.T) {
		payload := map[string]any{
			"org":   "test-org",
			"repo":  "test-repo",
			"title": "Test Goal",
			"body":  "Test body",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/goals", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 201 {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp["ok"].(bool) {
			t.Fatal("expected ok=true")
		}

		// Get the goal and verify pr is null
		id := int64(resp["id"].(float64))
		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.PR != nil {
			t.Fatalf("expected pr to be nil, got %v", *g.PR)
		}
	})

	t.Run("GET goal returns pr field as null when unset", func(t *testing.T) {
		// Create a goal without PR
		id, err := createGoal(db, "test-org", "test-repo", "Test", "Body", nil, nil)
		if err != nil {
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
		if resp["pr"] != nil {
			t.Fatalf("expected pr to be null, got %v", resp["pr"])
		}
	})

	t.Run("PATCH goal pr sets value", func(t *testing.T) {
		// Create a goal
		id, err := createGoal(db, "test-org", "test-repo", "Test PR", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Set PR to 42
		payload := map[string]any{"pr": 42}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/goals/"+strconv.FormatInt(id, 10)+"/pr", bytes.NewReader(body))
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp["ok"].(bool) {
			t.Fatal("expected ok=true")
		}

		// Verify the PR was set
		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.PR == nil || *g.PR != 42 {
			t.Fatalf("expected pr=42, got %v", g.PR)
		}
	})

	t.Run("GET goal returns pr after set", func(t *testing.T) {
		// Create a goal and set PR
		id, err := createGoal(db, "test-org", "test-repo", "Test PR Get", "Body", nil, nil)
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
		if resp["pr"] == nil {
			t.Fatal("expected pr to be set")
		}
		prValue := int(resp["pr"].(float64))
		if prValue != 123 {
			t.Fatalf("expected pr=123, got %d", prValue)
		}
	})

	t.Run("list goals includes pr field", func(t *testing.T) {
		// Create a goal with PR
		id, err := createGoal(db, "test-org", "test-repo", "List PR Test", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := updateGoalPR(db, id, 999); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/goals", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		items := resp["items"].([]any)

		// Find our goal in the list
		found := false
		for _, item := range items {
			goal := item.(map[string]any)
			if goal["title"].(string) == "List PR Test" {
				found = true
				if goal["pr"] == nil {
					t.Fatal("expected pr to be set in list")
				}
				prValue := int(goal["pr"].(float64))
				if prValue != 999 {
					t.Fatalf("expected pr=999 in list, got %d", prValue)
				}
				break
			}
		}
		if !found {
			t.Fatal("goal not found in list")
		}
	})

	t.Run("PATCH pr rejects zero", func(t *testing.T) {
		// Create a goal
		id, err := createGoal(db, "test-org", "test-repo", "Test Zero PR", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		payload := map[string]any{"pr": 0}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/goals/"+strconv.FormatInt(id, 10)+"/pr", bytes.NewReader(body))
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["ok"].(bool) {
			t.Fatal("expected ok=false for zero pr")
		}
	})

	t.Run("PATCH pr rejects negative", func(t *testing.T) {
		// Create a goal
		id, err := createGoal(db, "test-org", "test-repo", "Test Negative PR", "Body", nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		payload := map[string]any{"pr": -1}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/goals/"+strconv.FormatInt(id, 10)+"/pr", bytes.NewReader(body))
		req.SetPathValue("id", strconv.FormatInt(id, 10))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["ok"].(bool) {
			t.Fatal("expected ok=false for negative pr")
		}
	})

	t.Run("PATCH pr returns 404 for non-existent goal", func(t *testing.T) {
		payload := map[string]any{"pr": 123}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("PATCH", "/goals/99999/pr", bytes.NewReader(body))
		req.SetPathValue("id", "99999")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}
