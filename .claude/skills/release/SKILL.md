---
name: release
description: "ONLY activated by the explicit /release slash command. Never auto-triggered by conversation content. Project-local maintenance skill for releasing THIS repo (the rimsky-docs marketplace / rimsky plugin): verify the docs gate is green, decide the semver bump from changes since the last tag, bump the plugin's own version, then commit, advance the default branch to the release, tag, and push — a full release on main in one command, no follow-up."
---

# /release — fully release rimsky-docs to main, in one command

Releases **this repository** — the rimsky-docs plugin marketplace shipping the
`rimsky` documentation skill. One command takes the current state to a
**fully-published release on the default branch** (`main`, the branch consumers'
marketplaces pull): it verifies the docs gate is green, decides a semver bump,
bumps the plugin's own `version`, commits, advances `main` to the release, tags
it `vX.Y.Z`, and pushes `main` and the tag to `origin`.

**No follow-up, no second review.** `/build-docs` already runs the full gate
(build + test + 7 lints) and its `/refine-docs` review loop to converge the
corpus before you ever get here — so `/release` does **not** re-review the docs,
it ships them. Run it from wherever you are; it gets the release onto `main`
itself. The only things that stop it are a genuinely broken gate or a real merge
conflict (see preflight) — neither of which a single-branch workflow produces.

It bumps rimsky-docs' **own** release version — not the rimsky version. The
rimsky release the corpus documents lives in `reconciledAgainst`, owned by
`/build-docs`; this skill never touches it. (A doc-only improvement against the
same rimsky release is still a real rimsky-docs release, and bumps `version`.)

Repo-maintenance tool for the plugin author. Lives in `.claude/skills/`, not the
distributed `rimsky/skills/` — do not add it to the router
`rimsky/skills/rimsky/SKILL.md`. **The user invoking `/release` is the
authorization to commit, advance `main`, and push.** Run end to end without
pausing. Do not generate release notes.

## Preflight — abort with a clear message if any fail

1. **Git state.** Inside a git repo, on a branch (not detached HEAD). An
   `origin` remote exists (`git remote get-url origin`).
2. **Manifest.** `rimsky/.claude-plugin/plugin.json` exists with a `version`
   field (the plugin's own semver) and a `reconciledAgainst` field (the rimsky
   release documented).
3. **Green gate.** Resolve `RIMSKY_REPO` at the `reconciledAgainst` tag exactly
   as `/build-docs` preflight does — an exported override, else the
   `.build-docs/rimsky-core@<tag>` run-scratch cache, else acquire that tag; and
   per build-docs' own rule, pin to the **tag**, never a live/dirty working tree.
   Then, from the repo's `cmd/` directory:

   ```bash
   cd cmd && go build ./... && go test ./... \
     && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-lint all
   ```

   This runs against the working tree, so it validates exactly what is about to
   ship (including any uncommitted work). All must pass (the lint suite is 7/7).
   If red, abort — re-run `/build-docs` / `/refine-docs` to fix, then `/release`
   again. If `RIMSKY_REPO` cannot be resolved (offline, no cache), abort: the
   gate cannot be verified, so the release does not ship (override by exporting
   `RIMSKY_REPO` at the `reconciledAgainst` tag).

There is **no clean-tree precondition** — `/release` commits whatever is pending
as part of the release (it was just gated above). A green gate means the working
tree is releasable.

## Procedure

Capture the two branch names up front:

```bash
work=$(git rev-parse --abbrev-ref HEAD)                          # where you are now
default=$(git remote show origin | sed -n 's/.*HEAD branch: //p') # usually "main"
[ -n "$default" ] || default=main
```

`$default` is the consumer-facing branch the release lands on.

### 1. Survey the changes since the last tag

```bash
last_tag=$(git describe --tags --abbrev=0 2>/dev/null)
```

The change set is `"$last_tag"..HEAD` plus any uncommitted work (`git diff HEAD`,
`git status --short`). Read enough to judge the bump, and note whether
`reconciledAgainst` moved (and to what kind of rimsky release).

- **No tag (first release)** → tag at the current `version`, no bump (step 3's
  first-release note).
- **Nothing to release** → a tag exists, `"$last_tag"..HEAD` is empty, and the
  tree is clean → report "nothing to release since `$last_tag`" and stop.

### 2. Read the current version

Read `version` from `rimsky/.claude-plugin/plugin.json` (e.g. `1.0.0`). Tags are
this string prefixed with `v`; the field carries no `v`.

### 3. Decide the bump

Judge **major / minor / patch** from what the change set does to rimsky-docs'
**consumer surface** — the published corpus an agent reads and links into, the
skill router and its routing, the two entry points (`SKILL.md` and
`docs/agents/llms.txt`), and the rimsky release the corpus documents.

| Level | Bump | When |
|-------|------|------|
| **major** (`X`) | breaking | A published surface or the router is removed or renamed so a consumer's references or the skill's invocation break; an incompatible restructure of the corpus layout; the router triggering/`description` contract changes incompatibly. |
| **minor** (`Y`) | feature | A new capability the corpus exposes — a new concept cluster, protocol guide, cookbook recipe, pattern, reference, or surface — or `reconciledAgainst` advances to a rimsky release that **adds documented capabilities** (a rimsky minor/major). Backward-compatible. |
| **patch** (`Z`) | fix | Everything else: doc fixes, prose tightening, recipe/guide corrections, link fixes, lint/tooling fixes, and `reconciledAgainst` advancing to a rimsky **patch** (bug-fix) release. |

Print the level and a one-line rationale (name the surface; note any
`reconciledAgainst` move and the kind of rimsky release it tracked). Highest
level wins; a genuine tie rounds **up** (say so). Compute the new version `$NEW`.
*(First release: no bump — `$NEW` is the current version.)*

### 4. Bump, commit, advance `$default`, tag

- **Bump:** Edit `version` in `rimsky/.claude-plugin/plugin.json` to `$NEW` with
  the Edit tool (single-line; do **not** touch `reconciledAgainst`). *(First
  release: skip the bump.)*
- **Commit** on `$work` — captures the bump plus any pending work:

  ```bash
  git add -A
  git commit -m "Release v$NEW" \
    -m "Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
  ```

  *(First release with an already-clean tree: nothing to commit — skip to the tag.)*
- **Advance `$default` to the release.** If `$work` == `$default`, it is already
  there. Otherwise:

  ```bash
  git switch "$default"
  git merge --ff-only "$work"
  ```

  A fast-forward is the normal case (your branch is `$default` plus the reviewed
  release commit). If `$default` is **not** a fast-forward of `$work` (it has
  commits `$work` lacks), try a clean merge (`git merge --no-edit "$work"`); if
  that **conflicts**, abort and report exactly which files conflict — resolving a
  merge conflict is the one thing `/release` cannot do for you. (In a
  single-branch workflow `$default` only ever moves via `/release`, so this stays
  a fast-forward.)
- **Tag** the release commit now on `$default`:

  ```bash
  git tag -a "v$NEW" -m "Release v$NEW"
  ```

### 5. Push

```bash
git push origin "$default"
git push origin "v$NEW"
```

### 6. Report

Print: previous → new version; the bump level and its one-line rationale; the
`reconciledAgainst` value (confirmed untouched); the gate result; the release
commit SHA; the tag; that `$default` was advanced (fast-forward / merge) and you
are now on it; and confirmation that `$default` and the tag were pushed.

## Notes

- Bumps only `version`. `reconciledAgainst` is `/build-docs`' to set — the way
  ok-planner's release skill never edits its conduct version.
- It leaves you on `$default` at the new tag. Your `$work` branch (if any) now
  points at the same commit — delete it or keep building on it (the next
  `/release` fast-forwards `$default` again).
- First release (no prior tag): tags the current committed `HEAD` at the current
  `version` as the inaugural release. Every release after diffs cleanly against
  the previous tag.
- The merge-conflict abort is the sole case that needs a hand: it only arises if
  `$default` gained commits out-of-band (a direct edit to `main`), which the
  normal build → release flow never does.
