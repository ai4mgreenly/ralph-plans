# PLAN.md — ralph-plans

## Database

SQLite at `~/.local/state/ralph/plans.db`.

### Schema

```sql
CREATE TABLE goals (
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
);

CREATE TABLE goal_transitions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    goal_id    INTEGER NOT NULL REFERENCES goals(id),
    from_status TEXT,
    to_status   TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE goal_comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    goal_id    INTEGER NOT NULL REFERENCES goals(id),
    body       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
```

### Indexes

```sql
CREATE INDEX idx_goals_status        ON goals(status);
CREATE INDEX idx_goals_org_repo      ON goals(org, repo);
CREATE INDEX idx_comments_goal_id    ON goal_comments(goal_id);
CREATE INDEX idx_transitions_goal_id ON goal_transitions(goal_id);
```

## API Routes

All responses return `{"ok": true/false, ...}`.

### Goal CRUD

| Method | Route | Purpose | Script |
|--------|-------|---------|--------|
| POST   | `/goals` | Create a goal (status: draft) | `goal-create` |
| GET    | `/goals/:id` | Get a single goal | `goal-get` |
| GET    | `/goals` | List goals | `goal-list` |

#### POST /goals

Request:
```json
{
    "org": "mgreenly",
    "repo": "ikigai",
    "title": "Add feature X",
    "body": "## Objective\n...",
    "review": false
}
```

Response:
```json
{"ok": true, "id": 42}
```

#### GET /goals/:id

Response:
```json
{
    "ok": true,
    "id": 42,
    "org": "mgreenly",
    "repo": "ikigai",
    "title": "Add feature X",
    "body": "## Objective\n...",
    "status": "queued",
    "review": false,
    "created_at": "2026-02-11T12:00:00Z",
    "updated_at": "2026-02-11T12:05:00Z"
}
```

#### GET /goals

Query params:
- `status` — filter by status (draft, queued, running, reviewing, done, stuck, cancelled)
- `org` — filter by org
- `repo` — filter by repo

Response:
```json
{
    "ok": true,
    "items": [
        {
            "id": 42,
            "org": "mgreenly",
            "repo": "ikigai",
            "title": "Add feature X",
            "status": "queued",
            "review": false,
            "depends": [3, 7]
        }
    ]
}
```

### Status Transitions

| Method | Route | Transition | Used by |
|--------|-------|------------|---------|
| PATCH  | `/goals/:id/queue` | draft → queued | `goal-queue` script |
| PATCH  | `/goals/:id/start` | queued → running | orchestrator |
| PATCH  | `/goals/:id/done` | running → done (review=false) | orchestrator |
| PATCH  | `/goals/:id/stuck` | running → stuck | orchestrator |
| PATCH  | `/goals/:id/review` | see below | orchestrator + human |
| PATCH  | `/goals/:id/requeue` | stuck → queued | orchestrator (retry) |
| PATCH  | `/goals/:id/cancel` | any non-terminal → cancelled | human |

Each transition endpoint validates the current status before applying. Returns `{"ok": false, ...}` if the transition is invalid. All transitions are recorded in the `goal_transitions` table.

#### PATCH /goals/:id/review

This endpoint handles two flows:

**Orchestrator sets reviewing status** (running → reviewing, only when review=true):
```json
{"action": "set"}
```

**Human approves** (reviewing → done):
```json
{"action": "approve"}
```

**Human rejects** (reviewing → queued, with feedback comment):
```json
{"action": "reject", "feedback": "Widget doesn't render"}
```

#### PATCH /goals/:id/cancel

Cancels a goal from any non-terminal status (draft, queued, running, reviewing, stuck). Returns error if goal is already `done` or `cancelled`.

### Comments

| Method | Route | Purpose | Script |
|--------|-------|---------|--------|
| POST   | `/goals/:id/comments` | Add a comment | `goal-comment` |
| GET    | `/goals/:id/comments` | List comments | (for orchestrator retry context) |

#### POST /goals/:id/comments

Request:
```json
{"body": "Spot-check rejected:\n\nWidget doesn't render"}
```

Response:
```json
{"ok": true, "id": 1, "goal_id": 42}
```

#### GET /goals/:id/comments

Response:
```json
{
    "ok": true,
    "items": [
        {"id": 1, "goal_id": 42, "body": "...", "created_at": "2026-02-11T12:10:00Z"}
    ]
}
```

## Status Transition Rules

```
draft ──→ queued ──→ running ──→ done
  ↓         ↓          ↓
  cancelled cancelled  ├──→ reviewing ──→ done
                       │        ↓
                       │      cancelled
                       │
                       └──→ stuck ──→ queued (retry)
                              ↓
                            cancelled
```

Valid transitions:
| From | To | Trigger |
|------|----|---------|
| draft | queued | `goal-queue` script |
| draft | cancelled | human cancels |
| queued | running | orchestrator picks up goal |
| queued | cancelled | human cancels |
| running | done | ralph succeeds (review=false) |
| running | reviewing | ralph succeeds (review=true) |
| running | stuck | ralph fails after max retries |
| running | cancelled | human cancels |
| reviewing | done | human approves |
| reviewing | queued | human rejects (with feedback) |
| reviewing | cancelled | human cancels |
| stuck | queued | manual retry / requeue |
| stuck | cancelled | human cancels |

## Not in scope

- **PR operations** — PRs stay in GitHub. `goal-retry` (which labels PRs) remains a GitHub-only script.
- **Stories** — Not part of this service.
- **Stats / logs** — Handled by other services (ralph-runs, ralph-logs, ralph-counts).

## Server

- **Port** — Defaults to 1970, overridable by CLI argument (e.g., `ralph-plans --port 7070`)
- **Auth** — Localhost only. Binds to `127.0.0.1`; no tokens or credentials needed.

## Logging

API events are logged to `~/.local/state/ralph/logs/ralph-plans.jsonl`. Each line is a JSON object:

```json
{"time":"2026-02-11T12:00:00Z","method":"PATCH","path":"/goals/42/queue","status":200,"goal_id":42,"duration_ms":3}
```

The log file is append-only. The directory is created on startup if it doesn't exist.

## Open questions

None currently.
