---
name: pipeline
description: Pipeline commands for creating and managing goals
---

# Pipeline

Autonomous development pipeline. Goals are executable units of work. Ralph (the agent loop) picks up queued goals, executes them in isolated clones, and creates pull requests from the results. All goal scripts return JSON (`{"ok": true/false, ...}`).

## Flow

```
Human writes goal → goal-create (draft) → goal-queue (queued) → Ralph executes → PR merges
```

## Default Workflow: Goals-First

**The goals-first workflow is the default for all work.** Local changes are rare exceptions that require explicit user instruction.

**Standard workflow:**

1. **Discuss** — User and Claude discuss the change and approach
2. **Create goal** — Claude creates the goal with clear acceptance criteria
3. **Queue immediately** — Goal is queued right after creation (default behavior)
4. **Ralph executes** — The orchestrator picks up the goal and runs it autonomously
5. **PR merges** — Completed work is merged via PR

**Default behaviors:**

- **Always queue after creation** — No manual testing or "trying it first" unless user explicitly requests it
- **No local changes** — Claude does not make local changes directly; work goes through Ralph

**When to make local changes (exceptions only):**

- User explicitly requests direct changes: "make this change now", "edit this file", "fix this directly"
- User explicitly says: "don't create a goal for this", "do this locally", "make this change here"

**If unsure:** Default to creating and queuing a goal. The user will specify if they want an exception.

## Goal Statuses

`draft` → `queued` → `running` → `done` (or `stuck`)

## Goal Commands

| Command | Usage | Does |
|---------|-------|------|
| `goal-create` | `--title "..." --org ORG --repo REPO [--model MODEL] [--reasoning LEVEL] < body.md` | Create goal (draft). Body via stdin. Optional model (haiku/sonnet/opus) and reasoning level (none/low/med/high). |
| `goal-list` | `[--status STATUS] [--org ORG] [--repo REPO]` | List goals, optionally filtered |
| `goal-get` | `<id>` | Read goal body + status |
| `goal-queue` | `<id>` | Transition draft → queued |
| `goal-cancel` | `<id>` | Cancel a non-terminal goal |

## Creating a Goal

```bash
cat <<'EOF' | goal-create --title "Add feature X" --org myorg --repo myrepo
## Objective
What should be accomplished.

## Reference
Relevant files, docs, and examples.

## Outcomes
Measurable, verifiable results.

## Acceptance
Success criteria.
EOF
```

Then queue it:

```bash
goal-queue <id>
```

## Goal Authoring

Goal bodies **must** follow the `goal-authoring` skill guidelines (`/load goal-authoring`). Key rules:

- Specify **WHAT**, never **HOW** — outcomes, not steps
- Reference relevant files — Ralph reads them across iterations
- Include measurable **acceptance criteria**
- Never pre-discover work (no specific line numbers or code snippets)
- Trust Ralph to iterate and discover the path

## Key Rules

- **Body via stdin** — `goal-create` reads body from stdin
- **org + repo required** — Every goal targets a specific GitHub org/repo
- Goals go through Ralph — don't make local changes unless explicitly asked
