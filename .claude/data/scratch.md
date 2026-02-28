# Local-First Git Architecture

## Vision

GitHub becomes a dumb backup mirror. The local bare repos on `/mnt/store/git/` are the source of truth. Ralph manages the entire goal lifecycle locally — no PRs, no polling, no webhooks.

## Architecture

```
/mnt/store/git/<org>/<repo>     Bare repos (canonical source of truth)
    remote: github.com          Push-only backup
        │
        ├── cloned by ──────►  ~/projects/<repo>  (human working copies)
        └── cloned by ──────►  ~/.local/state/ralph/clones/  (ralph working copies)
```

## Goal Lifecycle (new)

```
draft → queued → running → done
  │       │        │
  └───────┴────────┴──→ cancelled
                   └──→ stuck → queued (retry)
```

No more `submitted`, `merged`, `rejected`. Just `done`.

## Merge Flow

1. Ralph finishes work, commits
2. Push branch to GitHub (backup, after every commit)
3. Run `.ralph/check` in the working clone (deterministic test gate)
4. If check passes → squash-merge onto main in the bare repo
5. Push main to GitHub (backup)
6. Transition goal to `done`
7. If check fails → mark `stuck`, retry logic kicks in

## Completed

- [x] Create `/mnt/store/git/` directory (owned by ai4mgreenly)
- [x] Clone all repos as bare to `/mnt/store/git/<org>/<repo>`
    - ai4mgreenly: ralph, ralph-plans, ralph-shows, ralph-runs, ralph-logs, ralph-counts, ralph-pipeline, 1brc
    - mgreenly: ikigai
- [x] Push ralph-pipeline initial commit to GitHub (was never pushed)
- [x] Make ralph-pipeline public
- [x] Remove branch protection from: ralph-plans, ralph-shows, ralph-runs, ralph-logs, ralph-counts
- [x] Update all ~/projects/ working copies to use local bare repo as remote

## Remaining

### ralph-plans (this repo)

- [ ] Delete `github.go` (PR state checking via gh CLI)
- [ ] Delete `poller.go` (background PR polling goroutine)
- [ ] Remove `pr` column from goals table
- [ ] Remove states: `submitted`, `merged`, `rejected`
- [ ] Add state: `done` (terminal)
- [ ] Simplify state machine: draft → queued → running → done (+ stuck, cancelled)
- [ ] Add `handleDone` endpoint (running → done)
- [ ] Remove `handleSubmitted`, `handleSetPR` endpoints
- [ ] Remove PR auto-check from `handleGetGoal`
- [ ] Remove PR cache code
- [ ] Remove poller startup from `main.go`
- [ ] Update `isTerminal()` — done and cancelled are terminal
- [ ] Update tests to match new states
- [ ] Add `.ralph/check` script to this repo

### ralph-runs (orchestrator)

- [ ] Clone from `/mnt/store/git/<org>/<repo>` instead of `git@github.com:...`
- [ ] Push branch to GitHub after every commit (backup)
- [ ] After goal completes: run `.ralph/check` in clone
- [ ] If check passes: squash-merge onto main in bare repo, push main to GitHub
- [ ] If check fails: mark stuck, retry
- [ ] Remove all `gh pr create` / `gh pr merge` code
- [ ] Remove `goal-submitted` usage — replace with `goal-done`
- [ ] Remove PR description generation (Haiku-powered)
- [ ] Update clone path logic if needed

### ralph-pipeline (goal scripts)

- [ ] Remove `goal-submit` script
- [ ] Add `goal-done` script
- [ ] Update any references to submitted/merged/rejected states

### Per-repo setup

- [ ] Add `.ralph/check` to each repo that ralph will work on (ikigai, 1brc, etc.)

### Bare repo maintenance

- [ ] Decide if bare repos need a sync mechanism to push to GitHub periodically
- [ ] Or push to GitHub happens inline as part of the merge flow (current plan)
