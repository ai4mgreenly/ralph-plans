package main

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Goal struct {
	ID        int64   `json:"id"`
	Org       string  `json:"org"`
	Repo      string  `json:"repo"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	Status    string  `json:"status"`
	Retries   int     `json:"retries"`
	Model     *string `json:"model"`
	Reasoning *string `json:"reasoning"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type GoalSummary struct {
	ID        int64   `json:"id"`
	Org       string  `json:"org"`
	Repo      string  `json:"repo"`
	Title     string  `json:"title"`
	Status    string  `json:"status"`
	Model     *string `json:"model"`
	Reasoning *string `json:"reasoning"`
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
			            CHECK (status IN ('draft','queued','running','done','stuck','cancelled')),
			retries     INTEGER NOT NULL DEFAULT 0,
			model       TEXT    CHECK (model IS NULL OR model IN ('haiku','sonnet','opus')),
			reasoning   TEXT    CHECK (reasoning IS NULL OR reasoning IN ('none','low','med','high')),
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
		`CREATE TABLE IF NOT EXISTS goal_dependencies (
			goal_id        INTEGER NOT NULL REFERENCES goals(id),
			depends_on_id  INTEGER NOT NULL REFERENCES goals(id),
			PRIMARY KEY (goal_id, depends_on_id)
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

	// Add model and reasoning columns to existing tables (for backwards compatibility)
	alterStmts := []string{
		`ALTER TABLE goals ADD COLUMN model TEXT CHECK (model IS NULL OR model IN ('haiku','sonnet','opus'))`,
		`ALTER TABLE goals ADD COLUMN reasoning TEXT CHECK (reasoning IS NULL OR reasoning IN ('none','low','med','high'))`,
	}
	for _, s := range alterStmts {
		_, err := db.Exec(s)
		if err != nil {
			// Ignore duplicate column errors - column already exists
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}

	// Recreate table if constraint is outdated (e.g. still has 'submitted'/'merged'/'rejected')
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, testErr := tx.Exec(`INSERT INTO goals (org, repo, title, body, status) VALUES ('__test', '__test', '__test', '__test', 'done')`)
	tx.Rollback()

	if testErr != nil && strings.Contains(testErr.Error(), "CHECK constraint failed") {
		// Disable FKs so goal_transitions/goal_comments don't block the rename+drop
		recreateStmts := []string{
			`PRAGMA foreign_keys=OFF`,
			`DROP TABLE IF EXISTS goals_old`,
			`PRAGMA legacy_alter_table=ON`,
			`ALTER TABLE goals RENAME TO goals_old`,
			`PRAGMA legacy_alter_table=OFF`,
			`CREATE TABLE goals (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				org         TEXT    NOT NULL,
				repo        TEXT    NOT NULL,
				title       TEXT    NOT NULL,
				body        TEXT    NOT NULL,
				status      TEXT    NOT NULL DEFAULT 'draft'
				            CHECK (status IN ('draft','queued','running','done','stuck','cancelled')),
				retries     INTEGER NOT NULL DEFAULT 0,
				model       TEXT    CHECK (model IS NULL OR model IN ('haiku','sonnet','opus')),
				reasoning   TEXT    CHECK (reasoning IS NULL OR reasoning IN ('none','low','med','high')),
				created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
				updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
			)`,
			`INSERT INTO goals (id, org, repo, title, body, status, retries, model, reasoning, created_at, updated_at)
			 SELECT id, org, repo, title, body,
			        CASE
			            WHEN status IN ('submitted','merged') THEN 'done'
			            WHEN status = 'rejected' THEN 'cancelled'
			            ELSE status
			        END,
			        retries, model, reasoning, created_at, updated_at FROM goals_old`,
			`DROP TABLE goals_old`,
			`CREATE INDEX IF NOT EXISTS idx_goals_status ON goals(status)`,
			`CREATE INDEX IF NOT EXISTS idx_goals_org_repo ON goals(org, repo)`,
			`PRAGMA foreign_keys=ON`,
		}
		for _, s := range recreateStmts {
			if _, err := db.Exec(s); err != nil {
				return err
			}
		}
	}

	// Fix FK references in goal_transitions/goal_comments if they point to goals_old.
	var transitionsSQL string
	db.QueryRow(`SELECT sql FROM sqlite_master WHERE name='goal_transitions'`).Scan(&transitionsSQL)
	if strings.Contains(transitionsSQL, "goals_old") {
		fixFKStmts := []string{
			`PRAGMA foreign_keys=OFF`,
			`ALTER TABLE goal_transitions RENAME TO goal_transitions_old`,
			`CREATE TABLE goal_transitions (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				goal_id     INTEGER NOT NULL REFERENCES goals(id),
				from_status TEXT,
				to_status   TEXT    NOT NULL,
				created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
			)`,
			`INSERT INTO goal_transitions SELECT * FROM goal_transitions_old`,
			`DROP TABLE goal_transitions_old`,
			`CREATE INDEX IF NOT EXISTS idx_transitions_goal_id ON goal_transitions(goal_id)`,
			`ALTER TABLE goal_comments RENAME TO goal_comments_old`,
			`CREATE TABLE goal_comments (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				goal_id     INTEGER NOT NULL REFERENCES goals(id),
				body        TEXT    NOT NULL,
				created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
			)`,
			`INSERT INTO goal_comments SELECT * FROM goal_comments_old`,
			`DROP TABLE goal_comments_old`,
			`CREATE INDEX IF NOT EXISTS idx_comments_goal_id ON goal_comments(goal_id)`,
			`PRAGMA foreign_keys=ON`,
		}
		for _, s := range fixFKStmts {
			if _, err := db.Exec(s); err != nil {
				return err
			}
		}
	}

	return nil
}

func createGoal(db *sql.DB, org, repo, title, body string, model, reasoning *string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO goals (org, repo, title, body, model, reasoning) VALUES (?, ?, ?, ?, ?, ?)`,
		org, repo, title, body, model, reasoning,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func getGoal(db *sql.DB, id int64) (*Goal, error) {
	row := db.QueryRow(
		`SELECT id, org, repo, title, body, status, retries, model, reasoning, created_at, updated_at FROM goals WHERE id = ?`, id,
	)
	var g Goal
	err := row.Scan(&g.ID, &g.Org, &g.Repo, &g.Title, &g.Body, &g.Status, &g.Retries, &g.Model, &g.Reasoning, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
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
	query := `SELECT id, org, repo, title, status, model, reasoning FROM goals ` + whereClause + ` ORDER BY id DESC`
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
		if err := rows.Scan(&g.ID, &g.Org, &g.Repo, &g.Title, &g.Status, &g.Model, &g.Reasoning); err != nil {
			return nil, 0, err
		}
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

func addDependency(db *sql.DB, goalID, dependsOnID int64) error {
	_, err := db.Exec(
		`INSERT INTO goal_dependencies (goal_id, depends_on_id) VALUES (?, ?)`,
		goalID, dependsOnID,
	)
	return err
}

func removeDependency(db *sql.DB, goalID, dependsOnID int64) error {
	res, err := db.Exec(
		`DELETE FROM goal_dependencies WHERE goal_id = ? AND depends_on_id = ?`,
		goalID, dependsOnID,
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
	return nil
}

func listDependencies(db *sql.DB, goalID int64) ([]int64, error) {
	rows, err := db.Query(
		`SELECT depends_on_id FROM goal_dependencies WHERE goal_id = ? ORDER BY depends_on_id`,
		goalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func hasUnmetDependencies(db *sql.DB, goalID int64) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM goal_dependencies gd
		 JOIN goals g ON g.id = gd.depends_on_id
		 WHERE gd.goal_id = ? AND g.status != 'done'`,
		goalID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
