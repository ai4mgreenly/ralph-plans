# Local-First Git Architecture

## Overview

GitHub is a backup mirror. The local bare repos on `/mnt/store/git/<org>/<repo>` are the source of truth. Ralph manages the entire goal lifecycle locally — no PRs, no webhooks, no polling for external state.

## Layout

```
/mnt/store/git/<org>/<repo>          Bare repos (canonical)
    remote → github.com              Push-only backup

~/projects/<repo>                    Human working copies
    remote → /mnt/store/git/...      Clone from bare repo

~/.local/state/ralph/clones/...      Ralph working copies
    remote → /mnt/store/git/...      Clone from bare repo
```

## Goal Lifecycle

```
draft → queued → running → done
  │       │        │
  └───────┴────────┴──→ cancelled
                   └──→ stuck → queued (retry)
```

States `submitted`, `merged`, and `rejected` are removed. There is no PR step. `done` is the terminal success state.

## Merge Flow

1. Ralph works in a clone, committing as it goes
2. After every commit, push the branch to GitHub (backup)
3. When ralph finishes, run `.ralph/check` in the clone (test gate)
4. If check passes → squash-merge onto main in the bare repo → push main to GitHub
5. Transition goal to `done`
6. If check fails → mark `stuck`, retry logic kicks in

## Test Gate

Each repo defines a `.ralph/check` executable script at its root. Exit 0 means tests pass, non-zero means failure. This is the gate before any merge to main. It replaces GitHub Actions workflows for ralph's purposes.

---

## Repo Changes

### ralph-plans

The goal state machine and storage API. This is the central service.

**Remove:**
- `github.go` — PR state checking via `gh` CLI
- `poller.go` — background PR polling goroutine
- PR cache code
- `pr` column from goals table
- States: `submitted`, `merged`, `rejected`
- Endpoints: `handleSubmitted`, `handleSetPR`
- PR auto-check logic in `handleGetGoal`
- Poller startup in `main.go`

**Add:**
- State: `done` (terminal success)
- Endpoint: `handleDone` (running → done)
- `.ralph/check` script

**Update:**
- State machine transitions
- `isTerminal()` function
- Database migration to handle removed states
- All tests

### ralph-runs

The orchestrator that spawns ralph agents and manages clones.

**Remove:**
- All `gh pr create` / `gh pr merge` code
- PR description generation (Haiku-powered)
- `goal-submitted` usage

**Add:**
- Clone from `/mnt/store/git/<org>/<repo>` instead of `git@github.com:...`
- Push branch to GitHub after every commit (backup)
- Run `.ralph/check` after ralph finishes
- Squash-merge onto main in the bare repo when check passes
- Push main to GitHub after merge
- Use `goal-done` instead of `goal-submitted`

**Update:**
- Clone URL construction
- Post-completion flow (local merge replaces PR creation)

### ralph-pipeline

Goal management scripts used as a Claude plugin.

**Remove:**
- `goal-submit` script

**Add:**
- `goal-done` script

**Update:**
- Any references to submitted/merged/rejected states

### ralph-shows

Web UI dashboard.

**Update:**
- Remove any display of PR status, PR links, submitted/merged/rejected states
- Update status displays for the new state machine (draft, queued, running, done, stuck, cancelled)

### ralph-counts

Metrics dashboard.

**Update:**
- Remove any metrics tied to PR state or submitted/merged/rejected
- Update to track `done` as the success metric

### ralph-logs

Real-time log streaming.

**Review:**
- Likely no changes needed — logs are keyed on goal execution, not PR state
- Confirm and adjust if any PR-related log filtering exists

### ralph

Top-level documentation/config repo.

**Review:**
- Update any architecture docs that reference the PR-based workflow
- Update to reflect local-first git model

### 1brc, ikigai

Target repos that ralph works on.

**Add:**
- `.ralph/check` script defining the test gate for each repo

### ralph-pipeline (plugin)

**Update:**
- Skills that reference PR workflow or submitted state
- Goal authoring docs if they mention PRs
