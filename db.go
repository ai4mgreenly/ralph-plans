package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Goal struct {
	ID        int64  `json:"id"`
	Org       string `json:"org"`
	Repo      string `json:"repo"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	Review    bool   `json:"review"`
	Retries   int    `json:"retries"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type GoalSummary struct {
	ID     int64  `json:"id"`
	Org    string `json:"org"`
	Repo   string `json:"repo"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Review bool   `json:"review"`
}

type Comment struct {
	ID        int64  `json:"id"`
	GoalID    int64  `json:"goal_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, err
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS goals (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			org         TEXT    NOT NULL,
			repo        TEXT    NOT NULL,
			title       TEXT    NOT NULL,
			body        TEXT    NOT NULL,
			status      TEXT    NOT NULL DEFAULT 'draft'
			            CHECK (status IN ('draft','queued','running','reviewing','done','stuck','cancelled')),
			review      INTEGER NOT NULL DEFAULT 0,
			retries     INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
			updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		)`,
		`CREATE TABLE IF NOT EXISTS goal_transitions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			goal_id     INTEGER NOT NULL REFERENCES goals(id),
			from_status TEXT,
			to_status   TEXT    NOT NULL,
			created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		)`,
		`CREATE TABLE IF NOT EXISTS goal_comments (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			goal_id     INTEGER NOT NULL REFERENCES goals(id),
			body        TEXT    NOT NULL,
			created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_goals_status        ON goals(status)`,
		`CREATE INDEX IF NOT EXISTS idx_goals_org_repo      ON goals(org, repo)`,
		`CREATE INDEX IF NOT EXISTS idx_comments_goal_id    ON goal_comments(goal_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transitions_goal_id ON goal_transitions(goal_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func createGoal(db *sql.DB, org, repo, title, body string, review bool) (int64, error) {
	reviewInt := 0
	if review {
		reviewInt = 1
	}
	res, err := db.Exec(
		`INSERT INTO goals (org, repo, title, body, review) VALUES (?, ?, ?, ?, ?)`,
		org, repo, title, body, reviewInt,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func getGoal(db *sql.DB, id int64) (*Goal, error) {
	row := db.QueryRow(
		`SELECT id, org, repo, title, body, status, review, retries, created_at, updated_at FROM goals WHERE id = ?`, id,
	)
	var g Goal
	var reviewInt int
	err := row.Scan(&g.ID, &g.Org, &g.Repo, &g.Title, &g.Body, &g.Status, &reviewInt, &g.Retries, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	g.Review = reviewInt != 0
	return &g, nil
}

func listGoals(db *sql.DB, status, org, repo string, limit, offset int) ([]GoalSummary, int, error) {
	// Build WHERE clause
	whereClause := `WHERE 1=1`
	var args []any
	if status != "" {
		whereClause += ` AND status = ?`
		args = append(args, status)
	}
	if org != "" {
		whereClause += ` AND org = ?`
		args = append(args, org)
	}
	if repo != "" {
		whereClause += ` AND repo = ?`
		args = append(args, repo)
	}

	// Get total count when pagination is requested
	total := 0
	if limit > 0 {
		countQuery := `SELECT COUNT(*) FROM goals ` + whereClause
		if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	// Build main query
	query := `SELECT id, org, repo, title, status, review FROM goals ` + whereClause + ` ORDER BY id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var goals []GoalSummary
	for rows.Next() {
		var g GoalSummary
		var reviewInt int
		if err := rows.Scan(&g.ID, &g.Org, &g.Repo, &g.Title, &g.Status, &reviewInt); err != nil {
			return nil, 0, err
		}
		g.Review = reviewInt != 0
		goals = append(goals, g)
	}
	return goals, total, rows.Err()
}

func updateGoalStatus(db *sql.DB, id int64, from, to string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE goals SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		to, now, id, from,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}

	_, err = tx.Exec(
		`INSERT INTO goal_transitions (goal_id, from_status, to_status) VALUES (?, ?, ?)`,
		id, from, to,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func createComment(db *sql.DB, goalID int64, body string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO goal_comments (goal_id, body) VALUES (?, ?)`,
		goalID, body,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func listComments(db *sql.DB, goalID int64) ([]Comment, error) {
	rows, err := db.Query(
		`SELECT id, goal_id, body, created_at FROM goal_comments WHERE goal_id = ? ORDER BY id`, goalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.GoalID, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
