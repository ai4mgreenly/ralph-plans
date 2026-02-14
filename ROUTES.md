# API Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/goals` | Create a goal |
| GET | `/goals` | List goals (query: `status`, `org`, `repo`, `page`, `per_page`) |
| GET | `/goals/{id}` | Get a single goal |
| PATCH | `/goals/{id}/queue` | Transition draft → queued |
| PATCH | `/goals/{id}/start` | Transition queued → running |
| PATCH | `/goals/{id}/done` | Transition running → done |
| PATCH | `/goals/{id}/stuck` | Transition running → stuck |
| PATCH | `/goals/{id}/requeue` | Transition stuck → queued |
| PATCH | `/goals/{id}/cancel` | Cancel any non-terminal goal |
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

**Get all goals with status=done:**
```
GET /goals?status=done
```

**Get first page of goals with status=done (5 per page):**
```
GET /goals?status=done&page=1&per_page=5
```

**Get second page:**
```
GET /goals?status=done&page=2&per_page=5
```

### Validation

- `page` must be a positive integer (returns 400 if invalid)
- `per_page` must be a positive integer (returns 400 if invalid)
- `per_page` values above 100 are clamped to 100
