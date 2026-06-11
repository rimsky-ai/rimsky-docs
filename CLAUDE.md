# rimsky-docs

rimsky-docs is a **Claude Code plugin marketplace** shipping one plugin,
`rimsky`, which bundles an agent-facing documentation corpus for
[rimsky](https://github.com/rimsky-ai/rimsky-core) — a pre-v1, project-agnostic
reactive node-graph orchestration platform that is **not** in agents' training
data. The plugin ships two skills: the main one bundles the corpus (which lives
under `rimsky/skills/rimsky/docs/`), and `rimsky/skills/rimsky-version/` is a
small reporter that echoes the plugin's two version numbers as literals (stamped
by `/release` each release; it reads nothing at runtime).
`rimsky/skills/rimsky/SKILL.md` is the Claude Code entry point and
`…/docs/agents/llms.txt` the entry for other agents. The corpus is reconciled
against a pinned rimsky release, recorded in
`rimsky/.claude-plugin/plugin.json`'s `reconciledAgainst` field.

Conventions — agent-doc style, the cold-read accuracy model, citation grammar,
and the standing project rules (pre-v1 "break freely", the Go build/test/lint
gate after any `cmd/` change, project-agnostic naming, "fix every bug") — live in
`.claude/rules/` and are the authority. **This file covers how the repo is
maintained.**

## Maintaining the docs — two commands

After rimsky cuts a release (or anytime the docs need to move):

```
/update-docs   →   (review the diff)   →   /release
```

- **`/update-docs`** — the front door. It checks the pinned `reconciledAgainst`
  against the latest rimsky release and picks the right pass: a new release →
  `/build-docs` (full reconcile); the same release with local edits →
  `/refine-docs` (converge only); already current and clean → nothing. It leaves
  the result **uncommitted** for you to review. You do **not** choose between
  `/build-docs` and `/refine-docs` yourself — `/update-docs` decides.
- **`/release`** — ships it: re-runs the gate, bumps the plugin's own `version`,
  commits any pending work, fast-forwards `main`, tags `vX.Y.Z`, and pushes. One
  command, no follow-up.

`/version` (read-only) prints both version numbers when you need them.

## The two versions

`plugin.json` carries **two independent versions** — never pin one to the other
(an earlier mistake; that is why they are two fields):

- **`version`** — rimsky-docs' own release semver. `/release` bumps it from the
  change set. A doc-only improvement against the *same* rimsky release is still a
  real release and bumps it, so a marketplace sees the update.
- **`reconciledAgainst`** — the rimsky release the corpus documents.
  `/build-docs` stamps it; `/release` never touches it.

## The maintenance skills

These live in `.claude/skills/` and are **not** part of the distributed plugin
(only `rimsky/skills/` ships). You normally invoke just the first two.

| Skill | Role | Invoke directly? |
|---|---|---|
| `/update-docs` | Front door — routes to the right reconcile pass; leaves it uncommitted | **Yes** |
| `/release` | Ship — gate, bump `version`, commit, fast-forward `main`, tag, push | **Yes** |
| `/version` | Read-only — print `version` + `reconciledAgainst` | To check |
| `/build-docs` | Full reconcile against a rimsky release (regenerate references, gate, runs `/refine-docs`, stamps `reconciledAgainst`) | Rarely — `/update-docs` calls it |
| `/refine-docs` | The review → fix → converge loop | Rarely — `/build-docs` and `/update-docs` call it |

The gate `/build-docs` and `/release` run is `cd cmd && go build ./... && go test
./... && RIMSKY_REPO=<rimsky checkout at the reconciledAgainst tag> go run
./rimsky-docs-lint all` (the seven structural lints). `/build-docs` additionally
executes the cookbook journey walkthroughs against the published images at the
reconciled release (its artifact gate, step 3c — Docker required); `/release`
does not. See `.claude/rules/rules.md` for the full post-change checklist.
