---
name: build-docs
description: "ONLY activated by explicit /build-docs slash command. Never auto-triggered by conversation content."
---

# Build Docs: Reconcile the published docs against rimsky

`rimsky-docs` publishes documentation, lint tooling, runnable examples, and
deployment bundles for [rimsky](https://github.com/fallguyconsulting/rimsky).
This skill drives one job: **bring every published artifact in this repo
current against the rimsky source of truth**, then prove it by building,
testing, generating, and linting.

The skill is the orchestrator. It does not write docs or run builds itself —
it dispatches one subagent per surface, calls the repo's Go binaries for
mechanical generation, runs every test and smoke check, gates on the lint
suite, runs a review-and-fix loop, and reports. A run is not done until tests,
smoke checks, and lints are all green.

## The source of truth

rimsky is checked out as a sibling of this repo. Three kinds of source feed the
published surface; every surface reconciles against one of them:

- **Design concept catalog** — `${RIMSKY_REPO}/.ok-planner/design/concepts/*.md`.
  One file per noun in rimsky, carrying the full architectural story. Feeds
  `docs/concepts/` and the conceptual material in the five-pager.
- **Code, protos, and config schema** — `${RIMSKY_REPO}/protocols/proto/v1/*.proto`,
  the protocols module's hand-written Go packages
  (`${RIMSKY_REPO}/protocols/{claimproducer,lifecycle,action,serverkit,publisherkit,conformance}`),
  rimsky's code-side READMEs (`executors/`, etc.), the `rimsky.yml`
  config schema, and the control-API routes. Feeds the protocol / store /
  executor / blob-backend / mcp-server reference and the operator material.
  These docs are **generated from the code**.
- **Runtime behavior** — verified by actually building and running the
  executable bundles (`examples/`, `quickstart/`, `deploy/`).

Treat the rimsky checkout as read-only. This skill never writes into rimsky;
it only reads from it.

## What this skill owns

| Surface | Source | Ownership |
|---|---|---|
| `docs/concepts/*.md` | design concept catalog | Skill-owned. Near-verbatim copy of `design/concepts/*.md`, minus design-process scaffolding (tensions/discovery refs). Overwrite. |
| `docs/protocols/*.md` guides, `docs/stores/`, `docs/executors/`, `docs/blob-backends/`, `docs/mcp-servers/` | code + protos + rimsky READMEs + the generated references | Agent-written explanation layer; refine against source. |
| `docs/protocols/reference/*.md` | rimsky `protocols/proto/v1/*.proto` | Mechanical (`rimsky-docs-proto`); never hand-edit. |
| `docs/protocols/go-packages.md` | rimsky `protocols/` hand-written Go packages (godoc) | Mechanical (`rimsky-docs-gopkg`); never hand-edit. |
| `docs/operator-guide.md`, `docs/comparison.md`, `docs/roadmap.md`, `docs/licensing.md` | code + design + `deploy/` | Generated/derived; refine. |
| `docs/glossary.md` | rimsky's `concepts.md` catalog | Mechanical; published verbatim by the glossary binary. Never hand-edit. |
| `docs/agents/llms-full.txt`, root `llms.txt` / `llms-full.txt` | the Go binaries | Mechanical; regenerate, never hand-edit. |
| `docs/humans/` (five-pager) | rimsky `README.md` + concepts | Hand-shaped. Refine facts/vocab/links; preserve narrative. |
| `docs/cookbook/*.md` | cookbook entries + concepts + `deploy/`, `quickstart/` | Hand-initiated, skill-drafted. New entries via `cookbook add`. |
| `docs/agents/llms.txt` | the files it links to | Curated index. Flag drift; do not overwrite. |
| `docs/agents/errors/`, `docs/agents/examples/` | code error classes + examples | Mostly human. Flag drift/additions; do not overwrite. |
| `examples/` | rimsky protos + claim-producer API | Runtime artifact. `go build` + `go test` every run. |
| `quickstart/` | `rimsky.yml` schema + image/CLI surface | Runtime artifact. Config + link validation + live smoke test. |
| `deploy/` | `rimsky.yml` schema + Helm + images | Runtime artifact. Config + Helm validation + `smoke-test.sh`. |

"Refine" on a hand-shaped surface means: **bring the facts, vocabulary, and
links current against source while preserving the human's narrative
structure** — update what is stale, never flatten editorial work. Generated
surfaces have no such constraint; they are fully skill-owned.

## Invocation

- `/build-docs` — full reconcile pass over every surface.
- `/build-docs cookbook add "<pattern name>"` — draft a new cookbook entry for
  the named pattern, then fold it into the normal pass.

There is no single-surface or delta-only mode. Every run reconciles the full
surface from the current source. The model is **create if missing, refine if
present, idempotent** — running twice against unchanged source changes nothing.

## Process

### 0. Preflight

1. Resolve the rimsky checkout. Default: `../rimsky` relative to this repo
   root. Confirm it exists and looks like rimsky (`.ok-planner/design/concepts/`
   and `protocols/proto/v1/` both present). If not, stop and tell the user the
   sibling rimsky checkout is missing — this is a genuine blocker.
2. Export `RIMSKY_REPO` as an absolute path to that checkout. Every Go binary
   and every subagent prompt uses it.
3. If invoked as `cookbook add "<name>"`, note the new entry name; it joins the
   cookbook surface's work list in step 1.

### 1. Reconcile every surface (parallel subagents)

Dispatch one subagent per surface using the templates below. Independent
surfaces run in parallel — send them in a single message with multiple `Agent`
calls. Each subagent reconciles its whole surface (create-if-missing /
refine-if-present) and returns a per-surface change list plus any items it
flagged for human attention.

Surfaces:
- concepts
- code-reference (the prose guides for protocols / stores / executors /
  blob-backends / mcp-servers, including the protocols module's optional Go
  helper packages — the *generated* references under `docs/protocols/reference/`
  (wire) and `docs/protocols/go-packages.md` (Go packages) are mechanical, step 2)
- humans (five-pager)
- cookbook
- agents-index (llms.txt + errors/ + examples/ drift flagging)
- bundles (examples / quickstart / deploy prose reconciliation only — the
  build/test/smoke happens in step 3, not here)

### 2. Mechanical generation

After the subagents finish, regenerate the mechanical artifacts:

```bash
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-llms-full
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-glossary
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-proto
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-gopkg
```

`rimsky-docs-llms-full` writes `docs/agents/llms-full.txt` + the repo-root copy
(concatenating rimsky's design concept bodies and the published `docs/protocols/`
guides). `rimsky-docs-glossary` publishes `docs/glossary.md` as a verbatim copy
of rimsky's `.ok-planner/design/concepts.md` catalog. `rimsky-docs-proto`
generates the wire-protocol reference under `docs/protocols/reference/` from
rimsky's `protocols/proto/v1/*.proto` (services, messages, fields, enums + their
proto comments). `rimsky-docs-gopkg` generates `docs/protocols/go-packages.md`
from the godoc of the protocols module's hand-written Go packages
(`claimproducer`, `lifecycle`, `action`, `serverkit`, `publisherkit`,
`conformance`) — the optional helpers a Go service may import; the `proto/`
bindings are excluded (they are the wire reference's job). All of
these are mechanical — never hand-edit them.

### 3. Build and test everything — always

No gating, no flags. Run all of it every pass:

```bash
# Go tooling
cd cmd && go build ./... && go test ./...

# Runnable example — scenario tests against the sibling rimsky checkout
# (examples/go.mod replaces protocols with ../../rimsky/protocols)
cd examples && go build ./... && go test ./...

# quickstart smoke — compose builds the images from the rimsky checkout
cd quickstart && docker compose up -d --build && ./rimsky health && docker compose down

# deploy smoke — compose builds the rimsky images; rimsky-services images pull
cd deploy && docker compose up -d --build && curl -fsS http://localhost:8080/healthz && docker compose down
```

If `examples` or the bundles fail to build/run, the artifact has drifted from
rimsky's current proto / config / image surface. That is a real finding —
hand it to a fixer subagent (step 5), do not paper over it.

**Docker-build precondition.** The quickstart and deploy compose files build
rimsky's images directly via `build:` sections whose context is the rimsky
checkout (`${RIMSKY_REPO:-../../rimsky}`); the Dockerfiles live in rimsky
(`Dockerfile.all`, `Dockerfile.go-base` at its root, the stub Dockerfiles in
their packages). So `RIMSKY_REPO` must resolve to a rimsky checkout. The deploy
stack additionally pulls `rimsky-services/*` images from ghcr (a separate repo);
without registry access its smoke test can't complete — note that rather than
treating it as drift. If Docker itself is unavailable, that is a blocker for the
smoke portion — report it rather than skipping silently.

### 4. Lint gate

```bash
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-lint all
```

If any lint fails, hand the failures to a fixer subagent (step 5) and re-run
until clean.

### 5. Review and fix loop

For each surface that changed, dispatch a reviewer subagent (template below)
that reads the surface as it now exists and reports real issues. Hand every
issue — plus any red test, red smoke check, or failed lint — to a fixer
subagent. Re-review, re-test, re-lint. Iterate until everything is green and
review is clean. Do not triage or defer findings.

### 6. Report

Print a per-surface summary: what was created, what was refined, what was
flagged for human attention (hand-shaped surfaces the skill chose not to
overwrite), and the final test / smoke / lint results. Do not commit — this
repo's rules forbid commits unless the user explicitly asks.

## Surface subagent templates

Dispatch each as an `Agent` (general-purpose). Every prompt is self-contained:
the subagent has not seen this conversation. Always pass the absolute
`RIMSKY_REPO` path and the repo root.

### concepts

> Reconcile the published per-concept pages `docs/concepts/*.md` against
> rimsky's design concept catalog.
>
> Source of truth: `${RIMSKY_REPO}/.ok-planner/design/concepts/*.md`. These are
> self-contained concept files. Frontmatter is exactly `concept` / `status` /
> `aliases` / `references`. Bodies are "What it is / Purpose / Boundaries /
> Invariants" prose that references only within the design folder, via tokens
> like `concept:<slug>`, `@blessed-invariant N`, and `spec:…`. There are no code
> references and no `definition`/anchor frontmatter — that older schema is
> retired; do not reintroduce it.
>
> The published page is a **near-verbatim copy** of the design concept with the
> design-*process* scaffolding stripped:
> - Drop the `references:` frontmatter block (it lists `_discover/` discovery
>   notes). Keep `concept`, `status`, `aliases`.
> - Remove inline references that point at the design process — anything citing
>   the tensions catalog (`tensions/…`) or discovery notes (`_discover/…`).
> - Leave everything else intact: the conceptual prose, and the intra-concept
>   `concept:<slug>` links (those resolve within `docs/concepts/`). Do NOT
>   rewrite or "improve" the prose, and do NOT author new frontmatter fields.
>
> For every concept in the source: ensure `docs/concepts/<slug>.md` exists
> (create if missing, refresh from source if present). Remove published pages
> whose source concept no longer exists. Keep the set internally consistent:
> every surviving `concept:<slug>` link must point at a concept that still has a
> published page.
>
> The glossary (`docs/glossary.md`) is NOT your job — it is published verbatim
> from rimsky's `concepts.md` by the glossary binary in the mechanical step.
>
> Return: pages created / refreshed / removed, and any surviving
> `concept:<slug>` link that does not resolve to a published concept.

### code-reference

> Reconcile the prose **guides** for the code-facing surfaces against rimsky's
> code, protos, and config schema. Surfaces: `docs/protocols/*.md` (the
> "Implementing an X" guides), `docs/stores/`, `docs/executors/`,
> `docs/blob-backends/`, `docs/mcp-servers/`.
>
> You write the *explanations*, not the reference. Two machine-readable
> references are generated mechanically and you must NOT hand-write or duplicate
> them — point at them instead: the wire reference under
> `docs/protocols/reference/` (`rimsky-docs-proto`, from the `.proto` files), and
> `docs/protocols/go-packages.md` (`rimsky-docs-gopkg`, the godoc of the
> protocols module's hand-written Go packages). The guide's job is the practical
> "how to implement this, what to watch out for" narrative that a generated
> reference can't express.
>
> This includes the Go story for the protocols module. rimsky has **no separate
> Go SDK**: the `protocols` module is the single public Go module, carrying the
> wire contract plus optional helper packages (`serverkit`, `publisherkit`,
> `action`) and contract ergonomics (`claimproducer`, `lifecycle`). Frame these
> as *conveniences* — a Go service may use them or implement straight against the
> wire contract. Do not call them an "SDK" or an "API"; do not imply they are
> required. The worked "build a producer/executor/subscriber/publisher in Go"
> narrative belongs in `docs/protocols/` guides, cross-linking into
> `docs/protocols/go-packages.md` rather than restating signatures.
>
> Sources: `${RIMSKY_REPO}/protocols/proto/v1/*.proto` and the generated
> `docs/protocols/reference/`; the protocols module's hand-written Go packages
> and the generated `docs/protocols/go-packages.md`; rimsky's code-side READMEs,
> the `rimsky.yml` config schema, and the control-API routes. When the code says
> something different from a guide, the code wins.
>
> For each guide: create if the underlying protocol/store/executor/helper exists
> in rimsky but no guide does; refine if present; remove if the underlying thing
> is gone. Verify every proto symbol, Go symbol, config field, and API route a
> guide names actually exists in the source. Follow rimsky's citation grammar
> (`${RIMSKY_REPO}/.claude/rules/citation-grammar.md`); the source's vocabulary
> is the reference — match it, don't impose your own.
>
> Return: guides created / refined / removed, and any named symbol / field /
> route that does not resolve in the source.

### humans (five-pager)

> Reconcile the human-facing five-pager under `docs/humans/` against current
> rimsky.
>
> Primary source: rimsky's `README.md` (the canonical five-pager narrative) plus
> the concept pages it references. This surface is hand-shaped: **refine, do not
> regenerate.** Bring facts, vocabulary, links, and primitive names current;
> preserve the existing narrative structure and voice. The five-pager must
> include a page that calls out the surprises a reader would not expect —
> notably rimsky's loop and recursion support.
>
> If `docs/humans/` is empty or missing pages, create the page set from the
> README narrative, split into coherent pages (what rimsky is; mental model and
> vocabulary; the surprises; a worked example end-to-end; where to go next —
> handing off to the agents docs and the cookbook). Validate that every internal
> link resolves into the published surface.
>
> Return: pages created / refined, links that don't resolve, and any place where
> source and the existing narrative conflict in a way you chose not to overwrite
> (flag for human attention rather than flattening).

### cookbook

> Reconcile `docs/cookbook/` — patterns one level more general than tutorials
> (e.g. "build a one-node queue worker", "fan out over a group of folders with a
> partitioned claim", "modify local files via an executor proxy").
>
> For each existing cookbook entry: refine it against the current concepts it
> references and against `deploy/` and `quickstart/`. Every recipe must be
> executable on top of the published `quickstart/` or `deploy/` stack — a reader
> should be able to run it. Structure each: problem statement, the rimsky shape
> (which primitives and why), a complete walkthrough that runs on the published
> stack, and a "without rimsky" baseline.
>
> {{IF cookbook add}} Additionally, draft a new entry for the pattern named
> "<NAME>": pick the concepts it exercises, write the walkthrough against the
> published stack, and place it at `docs/cookbook/<slug>.md`. {{END IF}}
>
> Do not invent new patterns beyond existing entries and any explicitly
> requested one.
>
> Return: entries created / refined, and any recipe whose walkthrough you could
> not make runnable against the current `quickstart/` or `deploy/` stack.

### agents-index

> Audit the curated agent surface for drift; do not overwrite it.
>
> Read `docs/agents/llms.txt` and every file it links to; verify the links
> resolve and the index still describes what those files contain. Read
> `docs/agents/errors/` against rimsky's actual error classes and
> `docs/agents/examples/` against the runnable examples. These surfaces are
> human-curated — your job is to flag drift (dead links, missing error classes
> newly present in rimsky, examples that no longer match), not to rewrite.
>
> Do not touch `docs/agents/llms-full.txt` — it is mechanically regenerated.
>
> Return: a list of drift findings for human attention. Make no edits.

### bundles (prose only)

> Reconcile the prose of the executable bundles against their code and against
> the published docs. Surfaces: `examples/atomic-staging-fs-producer/README.md`,
> `quickstart/README.md`, `deploy/` docs, and the published walkthrough
> `docs/agents/examples/atomic-staging.md`.
>
> Do not build or test here — that happens in the orchestrator. Your job is text
> coherence: the READMEs and walkthroughs must match what the code and configs
> actually do, and every doc link must resolve into the published surface (the
> quickstart README's links to `docs/concepts/`, `docs/humans/`, etc. must
> point at real files).
>
> Return: prose reconciled, and any link or claim that does not match the code /
> config / published surface.

## Reviewer subagent template

> Review the `<SURFACE>` documentation surface in rimsky-docs as it now exists
> (read the files; this is not a diff review). Source of truth for this surface
> is `<SOURCE>`. Report real issues only — factual drift from source, broken
> links, citation-grammar violations, internal
> inconsistency, recipes that won't run. Do not nitpick style. Return a flat
> list of issues, each with a file:line and a one-line description of what is
> wrong and what it should be.

## Fixer subagent template

> Fix every issue in the list below in the `<SURFACE>` surface of rimsky-docs.
> Do not triage, defer, or mark anything out of scope — fix them all, including
> any that look pre-existing. Source of truth: `<SOURCE>`. Follow rimsky's
> citation grammar in `${RIMSKY_REPO}/.claude/rules/citation-grammar.md`, and
> treat the source's vocabulary as the reference. After fixing, confirm the
> relevant generation / test / lint command for this surface passes. Return what
> you changed.
>
> Issues:
> <ISSUE LIST>

## Notes

- The `RIMSKY_REPO` convention is load-bearing for the binaries and for the
  pre-release reconciliation gate in `${RIMSKY_REPO}/scripts/release.sh`. Always
  set it; the binaries exit non-zero with a help message when it is unset.
- This skill never commits. After a green run, the working tree holds the
  reconciled docs; the user decides when to commit.
- If a surface's source is genuinely absent (e.g. a reference doc whose
  protocol was removed from rimsky), removing the published doc is the correct
  reconcile action — an empty or missing source means the published artifact
  should not exist.
