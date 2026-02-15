package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestModelReasoningFields(t *testing.T) {
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

	t.Run("create goal without model/reasoning returns null", func(t *testing.T) {
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

		// Get the goal and verify model and reasoning are null
		id := int64(resp["id"].(float64))
		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.Model != nil {
			t.Fatalf("expected model to be nil, got %v", *g.Model)
		}
		if g.Reasoning != nil {
			t.Fatalf("expected reasoning to be nil, got %v", *g.Reasoning)
		}
	})

	t.Run("create goal with model and reasoning", func(t *testing.T) {
		model := "opus"
		reasoning := "high"
		payload := map[string]any{
			"org":       "test-org",
			"repo":      "test-repo",
			"title":     "Test Goal with Model",
			"body":      "Test body",
			"model":     model,
			"reasoning": reasoning,
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

		// Get the goal and verify model and reasoning
		id := int64(resp["id"].(float64))
		g, err := getGoal(db, id)
		if err != nil {
			t.Fatal(err)
		}
		if g.Model == nil || *g.Model != model {
			t.Fatalf("expected model=%s, got %v", model, g.Model)
		}
		if g.Reasoning == nil || *g.Reasoning != reasoning {
			t.Fatalf("expected reasoning=%s, got %v", reasoning, g.Reasoning)
		}
	})

	t.Run("invalid model is rejected", func(t *testing.T) {
		payload := map[string]any{
			"org":   "test-org",
			"repo":  "test-repo",
			"title": "Test Goal",
			"body":  "Test body",
			"model": "invalid-model",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/goals", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["ok"].(bool) {
			t.Fatal("expected ok=false for invalid model")
		}
		if resp["error"].(string) != "model must be one of: haiku, sonnet, opus" {
			t.Fatalf("unexpected error message: %s", resp["error"])
		}
	})

	t.Run("invalid reasoning is rejected", func(t *testing.T) {
		payload := map[string]any{
			"org":       "test-org",
			"repo":      "test-repo",
			"title":     "Test Goal",
			"body":      "Test body",
			"reasoning": "invalid-level",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/goals", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["ok"].(bool) {
			t.Fatal("expected ok=false for invalid reasoning")
		}
		if resp["error"].(string) != "reasoning must be one of: none, low, med, high" {
			t.Fatalf("unexpected error message: %s", resp["error"])
		}
	})

	t.Run("GET goal returns model and reasoning", func(t *testing.T) {
		// Create a goal with model and reasoning
		model := "sonnet"
		reasoning := "med"
		id, err := createGoal(db, "test-org", "test-repo", "Test", "Body", &model, &reasoning)
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest("GET", "/goals/"+string(rune(id+'0')), nil)
		req.SetPathValue("id", string(rune(id+'0')))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["model"] == nil || resp["model"].(string) != model {
			t.Fatalf("expected model=%s, got %v", model, resp["model"])
		}
		if resp["reasoning"] == nil || resp["reasoning"].(string) != reasoning {
			t.Fatalf("expected reasoning=%s, got %v", reasoning, resp["reasoning"])
		}
	})

	t.Run("list goals includes model and reasoning", func(t *testing.T) {
		// Create a goal with model and reasoning
		model := "haiku"
		reasoning := "low"
		_, err := createGoal(db, "test-org", "test-repo", "List Test", "Body", &model, &reasoning)
		if err != nil {
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
			if goal["title"].(string) == "List Test" {
				found = true
				if goal["model"] == nil || goal["model"].(string) != model {
					t.Fatalf("expected model=%s in list, got %v", model, goal["model"])
				}
				if goal["reasoning"] == nil || goal["reasoning"].(string) != reasoning {
					t.Fatalf("expected reasoning=%s in list, got %v", reasoning, goal["reasoning"])
				}
				break
			}
		}
		if !found {
			t.Fatal("goal not found in list")
		}
	})

	t.Run("all valid model values accepted", func(t *testing.T) {
		validModels := []string{"haiku", "sonnet", "opus"}
		for _, model := range validModels {
			m := model
			payload := map[string]any{
				"org":   "test-org",
				"repo":  "test-repo",
				"title": "Test " + model,
				"body":  "Test body",
				"model": m,
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/goals", bytes.NewReader(body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != 201 {
				t.Fatalf("expected 201 for model=%s, got %d: %s", model, w.Code, w.Body.String())
			}
		}
	})

	t.Run("all valid reasoning values accepted", func(t *testing.T) {
		validReasoning := []string{"none", "low", "med", "high"}
		for _, reasoning := range validReasoning {
			r := reasoning
			payload := map[string]any{
				"org":       "test-org",
				"repo":      "test-repo",
				"title":     "Test " + reasoning,
				"body":      "Test body",
				"reasoning": r,
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/goals", bytes.NewReader(body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != 201 {
				t.Fatalf("expected 201 for reasoning=%s, got %d: %s", reasoning, w.Code, w.Body.String())
			}
		}
	})
}
