---
name: refine-docs
description: "ONLY activated by the explicit /refine-docs slash command, or invoked by the build-docs skill at its review stage. Never auto-triggered by conversation content."
---

# Refine Docs: the review → fix → converge loop for rimsky-docs

This skill is the correction-and-convergence loop for the published docs. It
reads the working tree as it now stands, reviews every (or a named) surface
against the rimsky source of truth, fixes every issue found, and repeats until
it converges — then renders the run report.

It runs two ways, identical mechanics either way:

- **Standalone — `/refine-docs`** — "run it a few more times just in case" after
  a `build-docs` pass or any hand-edit. No reconcile, no mechanical generation;
  just review → fix → lint → report against the current tree.
- **Invoked by `build-docs`** at its review stage (its step 5), over the
  surfaces that run changed. Same loop, same journal, same report renderer.

## Source of truth (`RIMSKY_REPO`)

Reviewers verify the docs against rimsky, so `RIMSKY_REPO` must point at a
read-only rimsky checkout at the **latest public release tag** (the docs track
releases, not `dev`). Resolve it in this order:

1. If `RIMSKY_REPO` is already exported, use it as-is — a `build-docs` run that
   invoked this skill has already set it, or a developer set it to a local
   checkout.
2. Else, if a `.build-docs/rimsky-core@<tag>` cache from a recent run exists, use
   the newest one.
3. Else resolve and shallow-clone the latest release tag, exactly as build-docs
   preflight does:

   ```bash
   RIMSKY_TAG=$(gh api repos/rimsky-ai/rimsky-core/releases/latest --jq .tag_name)
   dest=".build-docs/rimsky-core@${RIMSKY_TAG}"
   [ -d "$dest" ] || git clone --depth 1 --branch "$RIMSKY_TAG" \
     https://github.com/rimsky-ai/rimsky-core "$dest"
   export RIMSKY_REPO="$(cd "$dest" && pwd)"
   ```

Record in the report which ref was used. Treat the checkout as read-only.

## The run journal

This skill is the canonical owner of the run journal — an append-only file at
`.build-docs/journal.md` (gitignored run scratch; the durable output is the
printed `report.md`, not the journal). When invoked by build-docs the journal
already exists (build-docs opened it in preflight and its reconcile subagents
have written `decision`/`flag` entries); this skill appends to it. Standalone,
this skill opens it fresh.

**Entry kinds** — one greppable line each (Markdown list or NDJSON):

- **`decision`** — a judgment call among materially different options. Fields:
  who (`orchestrator` / surface / `fixer:<surface>`), file/surface, what was
  chosen, the alternative, one-line why. Log only calls that genuinely could
  have gone the other way — not routine edits — so the table stays signal-dense.
- **`flag`** — something that cannot be resolved by editing this repo. Subtype:
  `source-conflict` (a published doc faithfully mirrors stale rimsky source),
  `unimplemented` (a documented feature absent from the code), or
  `declined-addition` (a curated-surface addition left for human curation).
- **`round`** — emitted by the loop below: round number, issues found / fixed /
  still-open.

## The loop

Each **round**:

1. For each surface in scope (every surface when standalone; the surfaces that
   changed when invoked by build-docs), dispatch a reviewer subagent (template
   below) that reads the surface as it now exists and reports real issues —
   factual drift from source, broken links, internal inconsistency, recipes that
   won't run. The **skill-packaging** surface is in scope like any other: the
   corpus lives under `rimsky/skills/rimsky/docs/`, wrapped by the `rimsky` skill
   (`rimsky/skills/rimsky/SKILL.md`) inside the marketplace's one plugin
   (`rimsky/.claude-plugin/plugin.json`). A "real issue" there is a `SKILL.md`
   router link that does not resolve under the corpus, drift between the two
   entry points (`SKILL.md` vs `rimsky/skills/rimsky/docs/agents/llms.txt`), or a
   `plugin.json` `version` that no longer matches the reconciled rimsky release.
2. Hand every issue — plus any red test or failed lint passed in
   as a seed — to a fixer subagent. Fixers never triage or defer; they fix
   everything and return `decision`/`flag` entries for the journal.
3. Re-run the lint gate (below). Append a `round` entry: round number, found,
   fixed, still-open.

**Convergence.** Stop when **two consecutive rounds find nothing new** (the
docs-equivalent of loop-until-dry) or after a **max of 9 rounds**, whichever
comes first. Run one global loop (collect every surface's issues per round);
re-review only the surfaces a fixer touched. If the cap is hit with issues still
open, record the open items as `flag` entries — never truncate silently.

## Cold-read acceptance (prose surfaces)

For each hand-shaped prose surface this run changed, run the **cold read** from
the style spec (`.claude/rules/agent-doc-style.md`): dispatch a fresh `Agent`
(general-purpose) with **no prior context**, give it only the changed doc(s) plus
a representative task, and have it report where it had to guess or look elsewhere.
Feed that friction back as issues for the next round. A prose surface is not
converged until a cold reader can use it without guessing.

## Lint gate

```bash
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-lint all
```

Hand any failure to a fixer and re-run until clean.

## Report

Render the journal into `.build-docs/report.md` and print it:

1. **Convergence summary** — from the `round` entries: rounds run, found / fixed
   / open per round, and the stop reason ("2 empty rounds → converged" or "hit
   9-round cap, N open").
2. **Attention table** — from the `decision` and `flag` entries, three sections
   (the docs analog of ok-planner's divergence table):
   - **Judgment calls** — the materially-different-option `decision` entries
     (who · surface · chosen vs. alternative · why). The "creative" choices the
     agents and orchestrator had to make, surfaced for review.
   - **Source-side conflicts** — `flag: source-conflict` / `unimplemented`:
     places the published doc and the source-of-truth can't both be made right
     from this repo (a verbatim concept mirroring stale design source, a
     documented-but-unimplemented feature, a stale proto comment showing through
     a generated reference). These need a rimsky-side change.
   - **Declined additions** — `flag: declined-addition`: curated-surface
     additions the skill did not auto-write (e.g. newly-present error classes),
     left for human curation.

When `build-docs` invoked this skill, it prepends the run header (the resolved
ref, the build / test results, and the per-surface created/refined/
removed summary) to this report. Do not commit — repo rules forbid commits
unless the user explicitly asks.

## Reviewer subagent template

> Review the `<SURFACE>` documentation surface in rimsky-docs as it now exists
> (read the files; this is not a diff review). Source of truth for this surface
> is `<SOURCE>`. Report real issues only — factual drift from source, broken
> links, citation-grammar violations, internal inconsistency, recipes that won't
> run, and violations of the agent-doc style spec
> (`.claude/rules/agent-doc-style.md`): a ramp/motivation opening instead of
> assertion-first, enumerable facts buried in prose instead of a table, missing
> boundaries (no "does NOT own"), reasoning compressed away, or a missing
> `@source:` anchor. Judge against the spec, not prose taste. Return a flat list
> of issues, each with a file:line and a one-line description of what is wrong and
> what it should be.

## Fixer subagent template

> Fix every issue in the list below in the `<SURFACE>` surface of rimsky-docs.
> Do not triage, defer, or mark anything out of scope — fix them all, including
> any that look pre-existing. Source of truth: `<SOURCE>`. Follow rimsky's
> citation grammar in `${RIMSKY_REPO}/.claude/rules/citation-grammar.md`, and
> treat the source's vocabulary as the reference. After fixing, confirm the
> relevant generation / test / lint command for this surface passes.
>
> Edit only files in the `<SURFACE>` surface. Fixers may run in parallel against
> one working tree, so do **not** edit another surface or a shared cross-cutting
> file (the lint tooling under `cmd/`, the `symbol-existence`
> `verifiedInternalSymbols` allowlist, the root `llms.txt` / `llms-full.txt`
> copies) — two fixers touching the same file race. If a fix *requires* such a
> change (e.g. a real new symbol the prose must name needs an allowlist entry),
> return it as a `flag` describing the exact edit and let the orchestrator apply
> it serially after the round.
>
> Return what you changed, plus — separately, for the run journal — any
> `decision` entries (a fix that could have gone more than one way: what you
> chose, the alternative, why) and any `flag` entries (`source-conflict` /
> `unimplemented` / `declined-addition`).
>
> Issues:
> <ISSUE LIST>
