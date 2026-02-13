# API Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/goals` | Create a goal |
| GET | `/goals` | List goals (query: `status`, `org`, `repo`) |
| GET | `/goals/{id}` | Get a single goal |
| PATCH | `/goals/{id}/queue` | Transition draft → queued |
| PATCH | `/goals/{id}/start` | Transition queued → running |
| PATCH | `/goals/{id}/done` | Transition running → done (non-review goals only) |
| PATCH | `/goals/{id}/stuck` | Transition running → stuck |
| PATCH | `/goals/{id}/review` | Review actions: set, approve, reject (body: `action`, `feedback`) |
| PATCH | `/goals/{id}/requeue` | Transition stuck → queued |
| PATCH | `/goals/{id}/cancel` | Cancel any non-terminal goal |
| POST | `/goals/{id}/comments` | Add a comment to a goal |
| GET | `/goals/{id}/comments` | List comments for a goal |
