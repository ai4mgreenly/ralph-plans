# PROJECT.md — ralph-plans

This document provides comprehensive context for AI agents exploring this project for the first time.

## Purpose

ralph-plans is an HTTP API service that serves as the goal backend for the Ralph pipeline. It replaces GitHub Issues (previously used for goal storage via labels and issue bodies) with a standalone SQLite-backed service.

The pipeline flow is:

```
Human writes goal → goal-create → ralph-plans API → orchestrator picks up queued goals → Ralph executes → PR merges
```

## Architecture

### Nano-service philosophy

The Ralph ecosystem follows a nano-service pattern: small, independent, single-purpose services communicating through a shared filesystem hub at `~/.local/state/ralph/`. Each service is a standalone repository with minimal dependencies. ralph-plans fits this pattern as a single Go binary serving an HTTP API.

### How ralph-plans integrates

The orchestrator (ralph-runs) does **not** call this API directly. Instead, it invokes goal-* shell scripts that wrap API calls. This indirection keeps the orchestrator decoupled from the backend implementation.

```
ralph-runs (orchestrator)
    └── calls goal-* scripts (shell)
            └── calls ralph-plans HTTP API
                    └── reads/writes SQLite
```

Updated goal-* scripts are developed in this repository first, then shared to other projects once proven.

### What stays in GitHub

Only PR operations remain on GitHub. Goal storage, status tracking, comments, and lifecycle management all move to this service.

## Tech stack

- **Go** with `modernc.org/sqlite` (pure Go SQLite, no CGo, single binary)
- **SQLite** database at `~/.local/state/ralph/plans.db`

## Data model

### Goals

Goals are units of work for Ralph to execute autonomously. Each goal has:

- **id**: Auto-increment integer, unique across all repos
- **org/repo**: Repository the goal targets (e.g., `mgreenly/ikigai`)
- **title**: Short description
- **body**: Markdown specification — describes WHAT to achieve, not HOW
- **status**: Lifecycle state (see below)
- **review**: Boolean flag — if true, goal pauses in `reviewing` status for human approval before transitioning to `done`
- **created_at / updated_at**: Timestamps

### Goal statuses

```
draft → queued → running → done
  ↓       ↓        ↓
  cancelled cancelled ├→ reviewing → done
                      │      ↓
                      │    cancelled
                      └→ stuck → queued (retry)
                            ↓
                          cancelled
```

| Status | Meaning |
|--------|---------|
| `draft` | Created but not ready for execution |
| `queued` | Ready for the orchestrator to pick up |
| `running` | Currently being executed by Ralph |
| `reviewing` | Awaiting human review (when `review` flag is set) |
| `done` | Completed successfully |
| `stuck` | Failed after retries |
| `cancelled` | Abandoned by human decision |

### State history

All status transitions are recorded in a `goal_transitions` table with timestamps, providing a full audit trail of each goal's lifecycle.

### Comments

Goals can have comments attached (feedback, context, retry information). Comments are append-only.

## API endpoints

The API provides endpoints corresponding to each goal-* script:

| Script | Method | Endpoint | Purpose |
|--------|--------|----------|---------|
| `goal-create` | POST | `/goals` | Create a new goal (status: draft) |
| `goal-get` | GET | `/goals/:id` | Retrieve a single goal |
| `goal-list` | GET | `/goals` | List goals, filterable by status and repo |
| `goal-queue` | PATCH | `/goals/:id/queue` | Transition draft → queued |
| `goal-comment` | POST | `/goals/:id/comments` | Add a comment to a goal |
| `goal-retry` | PATCH | `/goals/:id/retry` | Mark for retry (stuck → queued) |
| `goal-review` | PATCH | `/goals/:id/review` | Set reviewing, approve, or reject |
| `goal-cancel` | PATCH | `/goals/:id/cancel` | Cancel a goal |

All endpoints return JSON in the form `{"ok": true/false, ...}` to match the existing script contract. Server defaults to port 1970, overridable via CLI argument.

## Goal authoring principles

Goal bodies follow specific guidelines (relevant for understanding the data):

- Specify **WHAT** (outcomes), never **HOW** (implementation steps)
- Reference relevant files — Ralph reads them across iterations
- Include measurable acceptance criteria
- Never pre-discover work (no specific line numbers or code snippets)
- Trust Ralph to iterate and discover the path

## File locations

| Path | Purpose |
|------|---------|
| `~/.local/state/ralph/plans.db` | SQLite database |
| `~/.local/state/ralph/clones/` | Temporary working directories per goal |
| `~/.local/state/ralph/goals/` | Archived goal definition files |
| `~/.local/state/ralph/logs/` | Orchestrator and agent logs |
| `~/.local/state/ralph/logs/ralph-plans.jsonl` | API event log (JSONL) |
| `~/.local/state/ralph/stats.jsonl` | Per-goal execution metrics |

## Development context

- This project is early-stage; the directory structure under `~/.local/state/ralph/` is not fixed
- Goal scripts are developed here first, then shared to other projects
- The previous backend (GitHub Issues) used labels for status and issue numbers for identity
- The new system uses auto-increment IDs but still tracks org/repo per goal
