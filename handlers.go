package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
)

var prCache = newPRCache()

func registerRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /goals", handleCreateGoal(db))
	mux.HandleFunc("GET /goals/{id}", handleGetGoal(db))
	mux.HandleFunc("GET /goals", handleListGoals(db))
	mux.HandleFunc("PATCH /goals/{id}/queue", handleQueue(db))
	mux.HandleFunc("PATCH /goals/{id}/start", handleStart(db))
	mux.HandleFunc("PATCH /goals/{id}/submitted", handleSubmitted(db))
	mux.HandleFunc("PATCH /goals/{id}/stuck", handleStuck(db))
	mux.HandleFunc("PATCH /goals/{id}/requeue", handleRequeue(db))
	mux.HandleFunc("PATCH /goals/{id}/cancel", handleCancel(db))
	mux.HandleFunc("PATCH /goals/{id}/pr", handleSetPR(db))
	mux.HandleFunc("POST /goals/{id}/comments", handleCreateComment(db))
	mux.HandleFunc("GET /goals/{id}/comments", handleListComments(db))
	mux.HandleFunc("POST /goals/{id}/dependencies", handleAddDependency(db))
	mux.HandleFunc("DELETE /goals/{id}/dependencies/{dep_id}", handleRemoveDependency(db))
	mux.HandleFunc("GET /goals/{id}/dependencies", handleListDependencies(db))
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func goalIDFromRequest(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// --- handlers ---

func handleCreateGoal(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Org       string  `json:"org"`
			Repo      string  `json:"repo"`
			Title     string  `json:"title"`
			Body      string  `json:"body"`
			Model     *string `json:"model"`
			Reasoning *string `json:"reasoning"`
		}
		if err := readJSON(r, &req); err != nil {
			writeErr(w, 400, "invalid JSON")
			return
		}
		if req.Org == "" || req.Repo == "" || req.Title == "" || req.Body == "" {
			writeErr(w, 400, "org, repo, title, and body are required")
			return
		}
		// Validate model if provided
		if req.Model != nil {
			validModels := map[string]bool{"haiku": true, "sonnet": true, "opus": true}
			if !validModels[*req.Model] {
				writeErr(w, 400, "model must be one of: haiku, sonnet, opus")
				return
			}
		}
		// Validate reasoning if provided
		if req.Reasoning != nil {
			validReasoning := map[string]bool{"none": true, "low": true, "med": true, "high": true}
			if !validReasoning[*req.Reasoning] {
				writeErr(w, 400, "reasoning must be one of: none, low, med, high")
				return
			}
		}
		id, err := createGoal(db, req.Org, req.Repo, req.Title, req.Body, req.Model, req.Reasoning)
		if err != nil {
			writeErr(w, 500, "failed to create goal")
			return
		}
		writeJSON(w, 201, map[string]any{"ok": true, "id": id})
	}
}

func handleGetGoal(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		g, err := getGoal(db, id)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		}
		if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}

		// If goal is submitted and has a PR, check GitHub for terminal state
		if g.Status == "submitted" && g.PR != nil {
			// Try to get from cache first
			state, cached := prCache.get(g.Org, g.Repo, *g.PR)
			if !cached {
				// Not in cache, check GitHub
				freshState, err := checkPRState(g.Org, g.Repo, *g.PR)
				if err == nil {
					// Successfully got PR state, cache it
					state = freshState
					prCache.set(g.Org, g.Repo, *g.PR, *freshState)
				}
				// If error, state remains nil and we don't transition
			}

			// If we have a state, check if PR is in terminal state
			if state != nil {
				var newStatus string
				if state.Merged {
					newStatus = "merged"
				} else if state.Closed {
					newStatus = "rejected"
				}

				// Transition if needed
				if newStatus != "" {
					// Use updateGoalStatus to transition from submitted to merged/rejected
					if err := updateGoalStatus(db, id, "submitted", newStatus); err == nil {
						// Successfully transitioned, update our in-memory goal
						g.Status = newStatus
						// Refresh from DB to get updated timestamp
						if freshGoal, err := getGoal(db, id); err == nil {
							g = freshGoal
						}
					}
					// If transition fails (e.g., already transitioned by another request),
					// we'll just return the current state
				}
			}
		}

		writeJSON(w, 200, map[string]any{
			"ok":         true,
			"id":         g.ID,
			"org":        g.Org,
			"repo":       g.Repo,
			"title":      g.Title,
			"body":       g.Body,
			"status":     g.Status,
			"model":      g.Model,
			"reasoning":  g.Reasoning,
			"pr":         g.PR,
			"created_at": g.CreatedAt,
			"updated_at": g.UpdatedAt,
		})
	}
}

func handleListGoals(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		org := r.URL.Query().Get("org")
		repo := r.URL.Query().Get("repo")

		// Parse pagination parameters
		pageStr := r.URL.Query().Get("page")
		perPageStr := r.URL.Query().Get("per_page")

		var limit, offset int
		var page, perPage int
		paginated := pageStr != ""

		if paginated {
			var err error
			page, err = strconv.Atoi(pageStr)
			if err != nil || page <= 0 {
				writeErr(w, 400, "page must be a positive integer")
				return
			}

			perPage = 20 // default
			if perPageStr != "" {
				perPage, err = strconv.Atoi(perPageStr)
				if err != nil || perPage <= 0 {
					writeErr(w, 400, "per_page must be a positive integer")
					return
				}
			}

			// Clamp per_page to max 100
			if perPage > 100 {
				perPage = 100
			}

			limit = perPage
			offset = (page - 1) * perPage
		}

		goals, total, err := listGoals(db, status, org, repo, limit, offset)
		if err != nil {
			writeErr(w, 500, "failed to list goals")
			return
		}
		if goals == nil {
			goals = []GoalSummary{}
		}

		if paginated {
			writeJSON(w, 200, map[string]any{
				"ok":       true,
				"items":    goals,
				"page":     page,
				"per_page": perPage,
				"total":    total,
			})
		} else {
			writeJSON(w, 200, map[string]any{"ok": true, "items": goals})
		}
	}
}

func handleQueue(db *sql.DB) http.HandlerFunc {
	return transitionHandler(db, "draft", "queued")
}

func handleStart(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		g, err := getGoal(db, id)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		}
		if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		if g.Status != "queued" {
			writeErr(w, 409, "cannot transition from "+g.Status+" to running")
			return
		}
		unmet, err := hasUnmetDependencies(db, id)
		if err != nil {
			writeErr(w, 500, "failed to check dependencies")
			return
		}
		if unmet {
			writeErr(w, 409, "goal has unmet dependencies")
			return
		}
		if err := updateGoalStatus(db, id, "queued", "running"); err != nil {
			writeErr(w, 500, "failed to update status")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func handleSubmitted(db *sql.DB) http.HandlerFunc {
	return transitionHandler(db, "running", "submitted")
}

func handleStuck(db *sql.DB) http.HandlerFunc {
	return transitionHandler(db, "running", "stuck")
}

func handleRequeue(db *sql.DB) http.HandlerFunc {
	return transitionHandler(db, "stuck", "queued")
}

func handleCancel(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		g, err := getGoal(db, id)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		}
		if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		if isTerminal(g.Status) {
			writeErr(w, 409, "goal is already "+g.Status)
			return
		}
		if err := updateGoalStatus(db, id, g.Status, "cancelled"); err != nil {
			writeErr(w, 500, "failed to update status")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func handleCreateComment(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		// Verify goal exists
		if _, err := getGoal(db, id); err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		} else if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		var req struct {
			Body string `json:"body"`
		}
		if err := readJSON(r, &req); err != nil {
			writeErr(w, 400, "invalid JSON")
			return
		}
		if req.Body == "" {
			writeErr(w, 400, "body is required")
			return
		}
		cid, err := createComment(db, id, req.Body)
		if err != nil {
			writeErr(w, 500, "failed to create comment")
			return
		}
		writeJSON(w, 201, map[string]any{"ok": true, "id": cid, "goal_id": id})
	}
}

func handleListComments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		comments, err := listComments(db, id)
		if err != nil {
			writeErr(w, 500, "failed to list comments")
			return
		}
		if comments == nil {
			comments = []Comment{}
		}
		writeJSON(w, 200, map[string]any{"ok": true, "items": comments})
	}
}

func handleSetPR(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		// Verify goal exists
		if _, err := getGoal(db, id); err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		} else if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		var req struct {
			PR int `json:"pr"`
		}
		if err := readJSON(r, &req); err != nil {
			writeErr(w, 400, "invalid JSON")
			return
		}
		if req.PR <= 0 {
			writeErr(w, 400, "pr must be a positive integer")
			return
		}
		if err := updateGoalPR(db, id, req.PR); err != nil {
			writeErr(w, 500, "failed to update pr")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

// dependencyAllowedStatuses are the statuses that allow adding/removing dependencies.
var dependencyAllowedStatuses = map[string]bool{
	"draft":  true,
	"queued": true,
	"stuck":  true,
}

func handleAddDependency(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		g, err := getGoal(db, id)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		}
		if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		if !dependencyAllowedStatuses[g.Status] {
			writeErr(w, 409, "cannot modify dependencies when goal is "+g.Status)
			return
		}
		var req struct {
			DependsOnID int64 `json:"depends_on_id"`
		}
		if err := readJSON(r, &req); err != nil {
			writeErr(w, 400, "invalid JSON")
			return
		}
		if req.DependsOnID == 0 {
			writeErr(w, 400, "depends_on_id is required")
			return
		}
		if req.DependsOnID == id {
			writeErr(w, 400, "goal cannot depend on itself")
			return
		}
		// Check that the dependency goal exists
		if _, err := getGoal(db, req.DependsOnID); err == sql.ErrNoRows {
			writeErr(w, 404, "dependency goal not found")
			return
		} else if err != nil {
			writeErr(w, 500, "failed to get dependency goal")
			return
		}
		if err := addDependency(db, id, req.DependsOnID); err != nil {
			writeErr(w, 500, "failed to add dependency")
			return
		}
		writeJSON(w, 201, map[string]any{"ok": true})
	}
}

func handleRemoveDependency(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		g, err := getGoal(db, id)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		}
		if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		if !dependencyAllowedStatuses[g.Status] {
			writeErr(w, 409, "cannot modify dependencies when goal is "+g.Status)
			return
		}
		depIDStr := r.PathValue("dep_id")
		depID, err := strconv.ParseInt(depIDStr, 10, 64)
		if err != nil {
			writeErr(w, 400, "invalid dep_id")
			return
		}
		if err := removeDependency(db, id, depID); err == sql.ErrNoRows {
			writeErr(w, 404, "dependency not found")
			return
		} else if err != nil {
			writeErr(w, 500, "failed to remove dependency")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func handleListDependencies(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		if _, err := getGoal(db, id); err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		} else if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		deps, err := listDependencies(db, id)
		if err != nil {
			writeErr(w, 500, "failed to list dependencies")
			return
		}
		if deps == nil {
			deps = []int64{}
		}
		writeJSON(w, 200, map[string]any{"ok": true, "items": deps})
	}
}

// transitionHandler creates a handler for simple from→to status transitions.
func transitionHandler(db *sql.DB, from, to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := goalIDFromRequest(r)
		if err != nil {
			writeErr(w, 400, "invalid goal id")
			return
		}
		g, err := getGoal(db, id)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "goal not found")
			return
		}
		if err != nil {
			writeErr(w, 500, "failed to get goal")
			return
		}
		if g.Status != from {
			writeErr(w, 409, "cannot transition from "+g.Status+" to "+to)
			return
		}
		if err := updateGoalStatus(db, id, from, to); err != nil {
			writeErr(w, 500, "failed to update status")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}
