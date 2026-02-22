# ralph-plans

## Skills

Skills are modular instruction sets stored in `.claude/library/<name>/SKILL.md`.

- **Load a skill**: `/load <name>` reads the skill into context
- **Load multiple**: `/load name1 name2` loads several at once

## Skillsets

Skillsets are composite bundles of skills defined in `.claude/skillsets/<name>.json`.

- **Load a skillset**: `/skillset <name>` loads all preloaded skills and advertises on-demand ones
- **Skillset format**: JSON with `preload` (loaded immediately) and `advertise` (available via `/load`)

### Available skillsets

- `meta` - For improving the .claude/ system (preloads: jj, align)

### Goal Authoring for This Repo

When creating goals targeting this repository, use `--org ai4mgreenly --repo ralph-plans`.
