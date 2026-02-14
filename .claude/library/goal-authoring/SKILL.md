---
name: goal-authoring
description: Writing effective goals for Ralph execution
---

# Goal Authoring

Writing effective goals for Ralph loop execution.

## Core Principle

**Ralph has unlimited context through iteration.** Don't artificially limit goals or references — Ralph can read all project documents, iterate through failures, and persist until outcomes are achieved.

## Goal Body Format

Goals are created via `goal-create` with body on stdin. The body must follow this structure:

```markdown
## Objective
[Complete objective — not tiny steps]

## Reference
[All relevant docs + codebase examples]

## Outcomes
[Measurable, verifiable outcomes]

## Acceptance
[Success criteria: tests pass, specific commands succeed, etc.]
```

## Key Principles

1. **Specify WHAT, never HOW** — Outcomes, not steps/order
2. **Reference liberally** — All relevant docs, Ralph reads across iterations
3. **One cohesive objective** — Not artificially small, not entire release
4. **Complete acceptance criteria** — Ralph needs to know when done
5. **Trust Ralph to iterate** — Discovers path, learns from failures
6. **Be explicit about discovery** — If work requires finding all instances of X, state it clearly in both objective and outcomes. Ralph has gotten stuck when discovery wasn't explicit. Write "Discover and fix all hardcoded paths" not "Fix hardcoded paths" or "Fix paths at lines 10, 25, 40"

## Example: Bad vs Good

**Bad (context-limited thinking):**
```markdown
## Objective
Create the config struct.

## Reference
docs/config.md section 2.1

## Outcomes
- Struct defined in src/config.h
```

Problems: Artificially small, minimal reference, no acceptance criteria.

**Good (leverages unlimited context):**
```markdown
## Objective
Implement configuration loading with environment variable overrides per docs/config.md.

## Reference
- docs/config.md — Format spec and defaults
- docs/environment.md — Env var override behavior
- src/main.c — Entry point where config is loaded
- tests/unit/config/ — Existing test structure

## Outcomes
- Config loading implemented per docs/config.md
- Environment variable overrides working
- Default values applied when env vars absent
- Unit tests in tests/unit/config/ pass

## Acceptance
- All tests pass
- CONFIG_DIR=/tmp/test ./app starts with correct config path
```

Why better: Complete objective, comprehensive references, measurable outcomes, clear acceptance.

**Discovery example:**
```markdown
## Objective
Discover all hardcoded /usr/local/etc path references in src/ and update them to use CONFIG_DIR environment variable.

## Reference
- src/paths.c — Central path resolution
- .envrc — Shows CONFIG_DIR=$PWD/etc

## Outcomes
- All hardcoded /usr/local/etc references in src/ discovered via grep
- All discovered files updated to use getenv("CONFIG_DIR")
- No hardcoded /usr/local/etc paths remain in src/

## Acceptance
- All tests pass
- grep -r "/usr/local/etc" src/ returns no hardcoded paths
```

Why explicit discovery matters: Objective starts with "Discover", first outcome confirms discovery happened. Ralph won't skip the grep step.

## Anti-Patterns

- **Step-by-step instructions** — "First do X, then Y" (Ralph discovers path)
- **Minimal references** — "Save context" (Ralph iterates, include all relevant)
- **Vague outcomes** — "Feature implemented" (Be specific and measurable)
- **Tiny goals** — Breaking cohesive work into artificial steps
- **Pre-discovered work** — Listing specific file:line locations to fix (Ralph should discover)

**Do this:** Comprehensive objective, all references, measurable outcomes, trust Ralph to iterate and discover.
