# ralph-plans

Goal storage and state machine for the Ralph pipeline. A Go HTTP API backed by SQLite that manages goal lifecycle — creation, queuing, execution tracking, and completion. All other Ralph services interact with goals through this API.

Philosophy: deliberately minimalist. Go standard library HTTP, SQLite for persistence, no ORM or framework.

## Architecture

Part of a multi-service system:

| Service | Language | Port | Purpose |
|---------|----------|------|---------|
| **ralph-plans** | Go + SQLite | 5001 | This project — Goal storage and state machine |
| **ralph-shows** | Deno + Preact | 5000 | Web UI dashboard |
| **ralph-runs** | Ruby | 5002 | Orchestrator + agent loop |
| **ralph-logs** | Go | 5003 | Real-time log streaming |
| **ralph-counts** | Python | 5004 | Metrics dashboard |

### How It Works

An HTTP API that stores goals in SQLite and enforces a state machine for goal lifecycle transitions. Other services (ralph-runs, ralph-shows) call this API to create, query, and transition goals. No auth — all services are localhost-only.

### Source Layout

```
*.go                    # All Go source files at root
├── main.go             # Entry point, HTTP server setup
├── db.go               # SQLite schema + queries
├── handlers.go         # HTTP route handlers
├── transitions.go      # State machine transition logic
└── logger.go           # Request logging (JSONL)
```

### Goal Scripts

Goal management scripts live in `scripts/goal-*/run` (Ruby, return JSON). Symlinked from `scripts/bin/` and available on PATH via `.envrc`.

| Script | Purpose |
|--------|---------|
| `goal-list` | List goals by status |
| `goal-get` | Get a single goal |
| `goal-create` | Create a new goal (draft) |
| `goal-queue` | Queue a draft goal |
| `goal-start` | Mark a goal as running |
| `goal-submit` | Mark a goal as submitted (PR created) |
| `goal-stuck` | Mark a goal as stuck |
| `goal-retry` | Retry a stuck goal |
| `goal-cancel` | Cancel a goal |
| `goal-comment` | Add a comment to a goal |
| `goal-comments` | List comments on a goal |

## Development

### Tech Stack

- **Go** standard library
- **SQLite** via mattn/go-sqlite3
- No framework — `net/http` ServeMux

### Commands

```sh
make build   # Build binary
./launch.sh  # Run the server
```

### Version Control

This project uses **git**.

### Code Style

- Go standard library idioms, no framework
- All source files at package root (no `cmd/` or `internal/`)
- Goal scripts are Ruby, return JSON: `{"ok": true/false, ...}`
- Minimalist — no abstractions for one-time operations

### Environment

Configured via `.envrc` (direnv). `PATH` includes `scripts/bin/` for direct script access. Services communicate via `RALPH_*_HOST/PORT` env vars.

## Directory Structure

```
ralph-plans/
├── main.go                              # Entry point
├── db.go                                # SQLite schema + queries
├── handlers.go                          # HTTP route handlers
├── transitions.go                       # State machine logic
├── logger.go                            # Request logging
├── go.mod                               # Go module
├── go.sum                               # Dependency checksums
├── Makefile                             # Build commands
├── launch.sh                            # Entry point: runs the server
├── scripts/
│   ├── bin/                             # Symlinks to goal scripts (on PATH)
│   └── goal-*/run                       # Goal state management scripts
├── .claude/
│   ├── library/                         # Skills (modular instruction sets)
│   └── skillsets/                       # Composite skill bundles
├── .envrc                               # direnv config
└── AGENTS.md                            # This file
```

## Skills

Skills are modular instruction sets in `.claude/library/<name>/SKILL.md`.

- **Load a skill**: `/load <name>` reads the skill into context
- **Load multiple**: `/load name1 name2`

### Skillsets

Composite bundles in `.claude/skillsets/<name>.json`:

```json
{
  "preload": ["skill-a"],
  "advertise": [{"skill": "skill-b", "description": "When to use"}]
}
```

- `preload` — loaded immediately when skillset is activated
- `advertise` — shown as available, loaded on demand with `/load`

Available skillsets:

- `meta` — For improving the .claude/ system (preloads: jj, pipeline, goal-authoring, align)

### For Ralph

When Ralph executes a goal in this repo, it receives only `AGENTS.md` as project context. This file is responsible for getting Ralph everything it needs.

## Goal Authoring

Goals are markdown files with required sections: `## Objective`, `## Reference`, `## Outcomes`, `## Acceptance`.

Key principles: specify WHAT not HOW, reference liberally, make discovery explicit, include measurable acceptance criteria, trust Ralph to iterate.

Full guide: `.claude/library/goal-authoring/SKILL.md`

## Common Tasks

**Adding an API endpoint:** Add handler in `handlers.go`, register route in `registerRoutes()`, add transition logic in `transitions.go` if needed.

**Adding a goal command:** Create `scripts/<name>/run` (Ruby, returns JSON), symlink from `scripts/bin/<name>`.

**Adding a skill:** Create `.claude/library/<name>/SKILL.md` with YAML frontmatter (name, description). Add to relevant skillset JSON.
