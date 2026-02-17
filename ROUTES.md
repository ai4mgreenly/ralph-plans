# API Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/goals` | Create a goal |
| GET | `/goals` | List goals (query: `status`, `org`, `repo`, `page`, `per_page`) |
| GET | `/goals/{id}` | Get a single goal (auto-checks PR state if submitted) |
| PATCH | `/goals/{id}/queue` | Transition draft → queued |
| PATCH | `/goals/{id}/start` | Transition queued → running |
| PATCH | `/goals/{id}/submitted` | Transition running → submitted |
| PATCH | `/goals/{id}/stuck` | Transition running → stuck |
| PATCH | `/goals/{id}/requeue` | Transition stuck → queued |
| PATCH | `/goals/{id}/cancel` | Cancel any non-terminal goal |
| PATCH | `/goals/{id}/pr` | Set the pull request number for a goal |
| POST | `/goals/{id}/comments` | Add a comment to a goal |
| GET | `/goals/{id}/comments` | List comments for a goal |

## GET /goals - Pagination

The `GET /goals` endpoint supports optional pagination via query parameters.

### Query Parameters

- `status` (optional) - Filter by goal status
- `org` (optional) - Filter by organization
- `repo` (optional) - Filter by repository
- `page` (optional) - Page number (1-indexed). When omitted, all results are returned.
- `per_page` (optional) - Items per page. Default: 20, Maximum: 100

### Response Format

**Without pagination** (backward compatible):
```json
{
  "ok": true,
  "items": [...]
}
```

**With pagination** (when `page` is provided):
```json
{
  "ok": true,
  "items": [...],
  "page": 1,
  "per_page": 20,
  "total": 42
}
```

### Examples

**Get all goals with status=submitted:**
```
GET /goals?status=submitted
```

**Get first page of goals with status=submitted (5 per page):**
```
GET /goals?status=submitted&page=1&per_page=5
```

**Get second page:**
```
GET /goals?status=submitted&page=2&per_page=5
```

### Validation

- `page` must be a positive integer (returns 400 if invalid)
- `per_page` must be a positive integer (returns 400 if invalid)
- `per_page` values above 100 are clamped to 100

## GET /goals/{id} - Automatic PR State Checking

When fetching a goal with `status=submitted` that has an associated PR number, the API automatically checks the PR's state on GitHub and may transition the goal status:

- **PR merged** → goal transitions to `merged` status
- **PR closed (not merged)** → goal transitions to `rejected` status
- **PR still open** → goal remains `submitted`
- **GitHub API error** → goal remains `submitted` (no change on error)

PR state results are cached for 60 seconds per goal to minimize GitHub API calls. Terminal states (`merged` and `rejected`) are written permanently to the database and never polled again.
