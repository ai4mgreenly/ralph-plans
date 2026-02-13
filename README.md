# ralph-plans

HTTP API service for managing pipeline goals. Replaces GitHub Issues as the goal backend for the Ralph autonomous development pipeline.

## Overview

ralph-plans is a lightweight Go service that stores and manages goals — units of work executed by Ralph (an autonomous AI agent). Goals are created, queued, executed, and tracked through a simple REST API backed by SQLite.

## Part of the Ralph ecosystem

| Service | Language | Role |
|---------|----------|------|
| **ralph-runs** | Ruby | Orchestrator — polls for queued goals, spawns agents |
| **ralph-plans** | Go | Goal storage and lifecycle management (this service) |
| **ralph-logs** | Go | WebSocket server streaming agent activity |
| **ralph-counts** | Python | Dashboard for aggregated metrics |

All services communicate through the shared state directory at `~/.local/state/ralph/`.

## Storage

SQLite database at `~/.local/state/ralph/plans.db`.

## Development

```bash
go run .
```

## License

Private.
