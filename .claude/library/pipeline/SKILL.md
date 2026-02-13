---
name: pipeline
description: ralph-plans goal pipeline scripts
---

# Pipeline

## Deriving Parameters

- **org**: Parse from the git remote origin URL. Run `git remote get-url origin` and extract the org/user segment (e.g. `git@github.com:ai4mgreenly/ralph-plans.git` → `ai4mgreenly`).
- **repo**: The target repository for the goal (passed explicitly).

## Scripts

All scripts are in `scripts/` and require `RALPH_PLANS_HOST` and `RALPH_PLANS_PORT` environment variables.

### goal-create

Create a goal. Body is read from stdin.

```
echo "body" | goal-create --title "..." --org ORG --repo REPO [--review]
```

### goal-get

Read a single goal.

```
goal-get <id>
```

### goal-list

List goals with optional filters.

```
goal-list [--status STATUS] [--org ORG] [--repo REPO]
```

Statuses: draft, queued, running, reviewing, done, stuck, cancelled

### goal-queue

Move a goal from draft to queued (makes it available to the orchestrator).

```
goal-queue <id>
```

### goal-retry

Requeue a stuck goal for retry (stuck → queued).

```
goal-retry <id>
```

### goal-comment

Append a comment to a goal. Body from stdin.

```
echo "comment text" | goal-comment <id>
```

### goal-spot-check

Approve or reject a goal in reviewing state.

```
goal-spot-check <id> approve
goal-spot-check <id> reject --feedback "..."
```

## Status Lifecycle

```
draft → queued → running → done
                    ↓
                  stuck → queued (via retry)
                    ↓
                running → reviewing → done (via approve)
                                    → queued (via reject)

Any non-terminal state → cancelled
```
