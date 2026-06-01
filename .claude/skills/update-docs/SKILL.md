---
name: update-docs
description: "ONLY activated by the explicit /update-docs slash command. Never auto-triggered by conversation content. The single entry for bringing the rimsky-docs corpus current: decides whether a full reconcile (/build-docs) or just the review→converge loop (/refine-docs) is needed, runs it, and leaves the result uncommitted for review."
---

# /update-docs — bring the docs current, agent's choice of depth

One front door for "make the docs current." It decides which underlying pass is
needed and runs it, so you never have to remember `/build-docs` vs
`/refine-docs`. It leaves the result **uncommitted** — you review, then
`/release` commits, fast-forwards `main`, tags, and pushes.

The go-forward flow:

```
/update-docs   →   (review)   →   /release
```

Project-local maintenance tool — lives in `.claude/skills/`, not the distributed
`rimsky/skills/`; do not add it to the router. It **dispatches** to the real
skills (it does not reimplement them) and **never commits** — `/build-docs` and
`/refine-docs` never commit either; the commit is `/release`'s job.

## How it decides

The question is "did the source move?" — the rimsky release the corpus is pinned
to (`reconciledAgainst` in `rimsky/.claude-plugin/plugin.json`) versus the latest
public rimsky release.

1. **Read the pin.** `reconciledAgainst` from `rimsky/.claude-plugin/plugin.json`
   (e.g. `v0.4.1`).
2. **Resolve the latest rimsky release**, the way `/build-docs` preflight does:

   ```bash
   gh api repos/rimsky-ai/rimsky-core/releases/latest --jq .tag_name
   # fallbacks: git ls-remote --tags --sort=-v:refname https://github.com/rimsky-ai/rimsky-core
   #            | grep -oE 'refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' | sed 's@refs/tags/@@' | head -1
   #   or, if RIMSKY_REPO points at a local checkout: its newest release tag.
   ```

3. **Route** (invoke the chosen skill via the Skill tool; let it run to
   completion — its own gate must go green):

   | State | Action |
   |---|---|
   | Latest rimsky release is **newer** than `reconciledAgainst` — **or you cannot determine the latest** | **`/build-docs`** — full reconcile to the latest release: it pulls the rimsky source, regenerates the mechanical references, runs the 7-lint gate, runs `/refine-docs` at its review stage, stamps `reconciledAgainst`, and reports. The safe default when uncertain, because it is idempotent (a no-op against unchanged source) and resolves/acquires the source itself. |
   | Pinned release is current **and** the working tree has uncommitted doc edits | **`/refine-docs`** — converge the hand-edits (the review→fix→lint loop; no source pull or regen). |
   | Pinned release is current **and** the working tree is clean | Nothing to do — report that the corpus is current against `<reconciledAgainst>` and stop. Do not run a reconcile for its own sake. |

## After it runs

Report:

- which pass ran, and **why** (the pin vs. the latest release, and the tree
  state);
- the per-surface changes it made (lift from that skill's report);
- that the changes are **uncommitted**, and the next step is to review them and
  run **`/release`** (which commits any pending work, fast-forwards `main`, tags,
  and pushes).

Do **not** commit, tag, or push — that is `/release`'s job. Do **not** edit
`reconciledAgainst` yourself; `/build-docs` stamps it when it reconciles against
a new release.
