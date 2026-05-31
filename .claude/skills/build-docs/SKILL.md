---
name: build-docs
description: "ONLY activated by explicit /build-docs slash command. Never auto-triggered by conversation content."
---

# Build Docs: Reconcile the published docs against rimsky

`rimsky-docs` publishes **agent-facing** documentation, lint tooling, and a
deployment bundle for [rimsky](https://github.com/rimsky-ai/rimsky-core). The
docs are written for a coding agent pointed at rimsky ("can rimsky help with my
project? if so, build a template / deployment plan for it") — not for a human
reading a tour. This skill drives one job: **bring every published artifact in
this repo current against the rimsky source of truth**, then prove it by
building, testing, generating, and linting.

The skill is the orchestrator. It does not write docs or run builds itself —
it dispatches one subagent per surface, calls the repo's Go binaries for
mechanical generation, runs every test and smoke check, gates on the lint
suite, runs a review-and-fix loop, and reports. A run is not done until tests,
smoke checks, and lints are all green.

## The source of truth

The source of truth is the **latest public release** of rimsky — the official
repo `https://github.com/rimsky-ai/rimsky-core` at its newest release tag, not
an unreleased `dev` working tree. Preflight (step 0) resolves that tag and
acquires a read-only checkout of it; `${RIMSKY_REPO}` points there. Three kinds
of source feed the published surface; every surface reconciles against one of
them:

- **Design concept catalog** — `${RIMSKY_REPO}/.ok-planner/design/concepts/*.md`.
  One file per noun in rimsky, carrying the full architectural story. Feeds
  `docs/concepts/`.
- **Code, protos, and config schema** — `${RIMSKY_REPO}/lib/protocols/proto/v1/*.proto`,
  the protocols module's hand-written Go packages
  (`${RIMSKY_REPO}/lib/protocols/{claimproducer,lifecycle,action,serverkit,publisherkit,conformance}`),
  rimsky's code-side READMEs (`lib/services/executors/`, etc.), the `rimsky.yml`
  config schema, and the control-API routes. Feeds the protocol / store /
  executor / blob-backend / mcp-server reference and the operator material.
  These docs are **generated from the code**.
- **Runtime behavior** — verified by actually building and running the
  executable `deploy/` bundle.

Treat the rimsky checkout as read-only. This skill never writes into rimsky;
it only reads from it.

### rimsky layout (v0.4.0 and later)

rimsky restructured at v0.4.0 — its Go code moved under `lib/` and the module
was renamed. The orchestrator must thread these paths into every subagent
prompt (the templates below assume them) and into the mechanical binaries:

- **Wire protos** — `${RIMSKY_REPO}/lib/protocols/proto/v1/*.proto`.
- **Public Go module** (the single module a service imports) —
  `${RIMSKY_REPO}/lib/protocols`, module path
  `github.com/rimsky-ai/rimsky-core/lib/protocols`. It carries the wire bindings
  (`proto/v1/gen`) plus the optional helper packages `claimproducer`,
  `lifecycle`, `action`, `serverkit`, `publisherkit`, `conformance`. There is
  **no separate Go SDK**.
- **Reference services** — reintegrated in-tree under
  `${RIMSKY_REPO}/lib/services/{stores,executors,sensors,subscribers}`, each with
  its own `Dockerfile.*`. There is **no separate `rimsky-services` repo** and the
  deploy stack does **not** pull their images from ghcr — it builds them from the
  checkout. The standalone stub executor (`lib/services/test/stubexecutor/`) is
  Execute-only — it advertises no attribute schema, so a node with an
  `attributes:` block on it fails dispatch with `executor_schema_unavailable`;
  the `deploy/` stack's `http-node`/`claude-agent` (stub mode) *do* advertise a
  schema. The stub claim-producer is an in-process test double at
  `test/support/stores/stub/` with **no** standalone binary, so the `deploy/`
  stack uses the filesystem store as its claim-producer.
- **Control plane + MCP server** — `${RIMSKY_REPO}/lib/control/controlapi` (MCP
  server under `.../mcp`); config loader at `lib/control/config`. The control-API
  health route is `/health` (the dashboard's is `/healthz`).
- **Dockerfiles** — `${RIMSKY_REPO}/dockerfiles/`: `Dockerfile.rimsky` (all role
  binaries + `rimsky-entrypoint` under one PID-1, self-contained — the all-in-one
  role image), `Dockerfile.all-in-one` (zero-config layer
  `FROM rimsky:latest`, not self-contained), `Dockerfile.go-base` (single-binary
  builder selected by the `BINARY` arg, e.g. `rimsky-control-api`). Per-service
  Dockerfiles live under `lib/services/.../Dockerfile.*`. All build contexts are
  the rimsky-core repo root.
- **Conformance** — the `rimsky conformance <protocol>` CLI subcommand
  (`cmd/rimsky/conformance.go`), not the retired standalone
  `cmd/rimsky-*-conformance` binaries; the importable runners live under
  `lib/protocols/conformance/`.

The dashboard is a separate sibling repo (`rimsky-dashboard`, default
`../rimsky-dashboard`, override `RIMSKY_DASHBOARD_REPO`) that the `deploy/` stack
builds from source.

## What this skill owns

| Surface | Source | Ownership |
|---|---|---|
| `docs/concepts/*.md` | design concept catalog | Skill-owned. Near-verbatim copy of `design/concepts/*.md`, minus design-process scaffolding (tensions/discovery refs). Overwrite. |
| `docs/protocols/*.md` guides, `docs/stores/`, `docs/executors/`, `docs/blob-backends/`, `docs/mcp-servers/` | code + protos + rimsky READMEs + the generated references | Agent-written explanation layer; refine against source. |
| `docs/protocols/reference/*.md` | rimsky `lib/protocols/proto/v1/*.proto` | Mechanical (`rimsky-docs-proto`); never hand-edit. |
| `docs/protocols/go-packages.md` | rimsky `lib/protocols/` hand-written Go packages (godoc) | Mechanical (`rimsky-docs-gopkg`); never hand-edit. |
| `docs/reference/template-schema.md` | rimsky `lib/foundation/spec/` structs | Mechanical (`rimsky-docs-template-ref`); never hand-edit. |
| `docs/reference/rest-api.md` | rimsky `lib/control/controlapi/actions.go` | Mechanical (`rimsky-docs-rest-ref`); never hand-edit. |
| `docs/reference/cli.md` | the `rimsky` CLI's `help` output | Mechanical (`rimsky-docs-cli-ref`); never hand-edit. |
| `docs/operator-guide.md`, `docs/comparison.md`, `docs/roadmap.md`, `docs/licensing.md` | code + design + `deploy/` | Generated/derived; refine. |
| `docs/glossary.md` | rimsky's `concepts.md` catalog | Mechanical; published verbatim by the glossary binary. Never hand-edit. |
| `docs/agents/llms-full.txt`, root `llms.txt` / `llms-full.txt` | the Go binaries | Mechanical; regenerate, never hand-edit. |
| `docs/cookbook/*.md` | rimsky's primitives + concepts + `deploy/` | Derived: the minimal canonical set of patterns rimsky's primitives span, reconciled against the concepts and the published `deploy/` stack. Skill-owned set — add canonical patterns, merge/retire redundant ones, refine the rest. |
| `docs/agents/llms.txt` | the files it links to | Curated index. Flag drift; do not overwrite. |
| `docs/agents/errors/`, `docs/agents/examples/` | code error classes + template/instance examples | Mostly human. Flag drift/additions; do not overwrite. |
| `docs/services/` | rimsky `lib/services/` + `deploy/` config | Derive-and-verify catalog of the bundled services (config / ports / protocols / image). Refine against source. |
| `docs/images/` | rimsky `dockerfiles/` + per-service Dockerfiles + `deploy/` | Derive-and-verify catalog of the official images. Refine against source. |
| `deploy/` | `rimsky.yml` schema + `lib/services/` images | Runtime artifact. Config + link validation + full-stack smoke (compose `up --build` → control-API `/health` → `down`). All services build from the rimsky checkout — no ghcr. |

"Refine" on a hand-shaped surface means: **bring the facts, vocabulary, and
links current against source while preserving the human's narrative
structure** — update what is stale, never flatten editorial work. Generated
surfaces have no such constraint; they are fully skill-owned.

## The run journal

Every run keeps an append-only journal at `.build-docs/journal.md` (gitignored
run scratch — the durable output is the printed `report.md`, not the journal).
It is the shared spine and the handoff to the review stage: the reconcile
subagents and the orchestrator append `decision` and `flag` entries as they go
(judgment calls among materially different options; things that can't be
resolved from this repo), then `/refine-docs` appends `round` entries and renders
the journal into the report. **`/refine-docs` is the canonical owner of the
journal schema (`decision` / `flag` / `round`) and the report renderer** — see
that skill for the entry format and the attention-table layout.

## Invocation

- `/build-docs` — full reconcile pass over every surface (preflight → reconcile →
  mechanical → build/test → lint → review/refine → report). The review/refine
  loop is the separate **`/refine-docs`** skill, which this skill invokes at its
  review stage.

There is no single-surface or delta-only mode. Every run reconciles the whole
surface from the current source. The model is **create if missing, refine if
present, idempotent** — running twice against unchanged source changes nothing.
To re-run just the review → fix → converge loop afterward, use `/refine-docs`.

## Process

### 0. Preflight

1. **Resolve the source-of-truth release.** The published docs track the latest
   *public release* of rimsky, not unreleased `dev` work. Resolve the latest
   release tag of `https://github.com/rimsky-ai/rimsky-core`:

   ```bash
   RIMSKY_TAG=$(gh api repos/rimsky-ai/rimsky-core/releases/latest --jq .tag_name)
   # fallback when gh is unavailable:
   #   RIMSKY_TAG=$(git ls-remote --tags --sort=-v:refname \
   #     https://github.com/rimsky-ai/rimsky-core \
   #     | grep -oE 'refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$' | sed 's@refs/tags/@@' | head -1)
   ```

2. **Acquire that tag, read-only.** Shallow-clone it into the gitignored
   run-scratch cache (reuse if the tag already matches), and point `RIMSKY_REPO`
   at it. Every Go binary and every subagent prompt uses `RIMSKY_REPO`.

   ```bash
   dest=".build-docs/rimsky-core@${RIMSKY_TAG}"
   [ -d "$dest" ] || git clone --depth 1 --branch "$RIMSKY_TAG" \
     https://github.com/rimsky-ai/rimsky-core "$dest"
   export RIMSKY_REPO="$(cd "$dest" && pwd)"
   ```

   Treat the checkout as read-only (standing rule). Confirm it looks like rimsky
   (`.ok-planner/design/concepts/` and `lib/protocols/proto/v1/` both present —
   note the `lib/` prefix as of v0.4.0). If the repo is unreachable and no
   override is set, stop — this is a genuine blocker.

   **Override for local dev / offline.** If `RIMSKY_REPO` is already exported
   (e.g. pointing at a local sibling checkout), use it as-is and skip the
   resolve+clone. Record in the report which ref the run reconciled against (the
   resolved tag, or `local: <branch>@<sha>`), because a local `dev` tree may be
   ahead of the latest release.

3. **Open the run journal.** Create the gitignored `.build-docs/journal.md` (see
   "The run journal"). Its first line records the resolved ref. Every phase —
   reconcile subagents, the orchestrator, and the review/refine loop — appends
   `decision` / `flag` / `round` entries to it; step 6 renders it.


### 1. Reconcile every surface (parallel subagents)

Dispatch one subagent per surface using the templates below. Independent
surfaces run in parallel — send them in a single message with multiple `Agent`
calls. Each subagent reconciles its whole surface (create-if-missing /
refine-if-present) and returns a per-surface change list plus any items it
flagged for human attention.

Every surface prompt must also instruct the subagent to return, separately from
its change list, two structured lists for the run journal (see "The run
journal"): **`decision`** entries — judgment calls it made among materially
different options (what it chose, the alternative, one-line why) — and
**`flag`** entries — things it could not resolve from this repo (`source-conflict`
/ `unimplemented` / `declined-addition`). The orchestrator appends every
returned entry to `.build-docs/journal.md`, plus its own entries for
orchestrator-level calls (e.g. a bundle-topology decision, or choosing to flag
rather than edit a verbatim concept page).

Surfaces:
- concepts
- code-reference (the prose guides for protocols / stores / executors /
  blob-backends / mcp-servers, including the protocols module's optional Go
  helper packages — the *generated* references under `docs/protocols/reference/`
  (wire) and `docs/protocols/go-packages.md` (Go packages) are mechanical, step 2)
- cookbook
- catalogs (the `docs/services/` and `docs/images/` derive-and-verify
  catalogs — the *generated* references under `docs/reference/` are mechanical,
  step 2)
- agents-index (llms.txt + errors/ + examples/ drift flagging)
- bundles (`deploy/` prose reconciliation only — the build/test/smoke happens in
  step 3, not here)

### 2. Mechanical generation

After the subagents finish, regenerate the mechanical artifacts:

```bash
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-llms-full
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-glossary
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-proto
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-gopkg
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-template-ref
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-rest-ref
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-cli-ref
```

`rimsky-docs-llms-full` writes `docs/agents/llms-full.txt` + the repo-root copy
(concatenating rimsky's design concept bodies and the published `docs/protocols/`
guides). `rimsky-docs-glossary` publishes `docs/glossary.md` as a verbatim copy
of rimsky's `.ok-planner/design/concepts.md` catalog. `rimsky-docs-proto`
generates the wire-protocol reference under `docs/protocols/reference/` from
rimsky's `lib/protocols/proto/v1/*.proto` (services, messages, fields, enums + their
proto comments). `rimsky-docs-gopkg` generates `docs/protocols/go-packages.md`
from the godoc of the protocols module's hand-written Go packages
(`claimproducer`, `lifecycle`, `action`, `serverkit`, `publisherkit`,
`conformance`) — the optional helpers a Go service may import; the `proto/`
bindings are excluded (they are the wire reference's job).

The three `docs/reference/` generators are the **definitive specifics layer** an
agent reads to build a template / deployment plan: `rimsky-docs-template-ref`
writes `docs/reference/template-schema.md` (the complete template / `rimsky.yml`
schema, from the spec structs under `lib/foundation/spec/`); `rimsky-docs-rest-ref`
writes `docs/reference/rest-api.md` (every control-API route + method + per-action
auth gate, from the action registry `lib/control/controlapi/actions.go`); and
`rimsky-docs-cli-ref` writes `docs/reference/cli.md` from the `rimsky` CLI's own
`help` output (it shells `go run ./cmd/rimsky` inside `${RIMSKY_REPO}`, so that
checkout must build — slower than the others).

All of these are mechanical — never hand-edit them.

### 3. Build and test everything — always

No gating, no flags. Run all of it every pass:

```bash
# Go tooling (the docs generators + lint)
cd cmd && go build ./... && go test ./...

# deploy smoke — compose builds ALL images from the rimsky checkout (core
# processes via dockerfiles/Dockerfile.go-base, the reintegrated lib/services
# stores/executors/sensors via their own Dockerfiles, dashboard from the
# sibling). The control-API health route is /health. Many host ports are
# published; override the *_HOST_PORT vars if they collide.
cd deploy && docker compose up -d --build && curl -fsS http://localhost:8080/health && docker compose down -v
```

If the `deploy/` bundle fails to build/run, the artifact has drifted from
rimsky's current config / image surface. That is a real finding — hand it to a
fixer subagent (step 5), do not paper over it.

**Docker-build precondition.** The `deploy/` compose file builds
every rimsky image directly via `build:` sections whose context is the rimsky
checkout (`${RIMSKY_REPO:-../../rimsky-core}`); the Dockerfiles live under
`${RIMSKY_REPO}/dockerfiles/` (`Dockerfile.rimsky`, `Dockerfile.all-in-one`,
`Dockerfile.go-base`) and per-service under `lib/services/.../Dockerfile.*` (see
the layout note above). So `RIMSKY_REPO` must resolve to a rimsky-core checkout.
As of the services-reintegration there is **no ghcr pull** — the deploy stack
builds its stores/executors/sensors from `lib/services/`, so its smoke runs
fully offline. The dashboard builds from the `rimsky-dashboard` sibling
(`RIMSKY_DASHBOARD_REPO`, default `../../rimsky-dashboard`). If Docker itself is
unavailable, that is a blocker for the smoke portion — report it rather than
skipping silently.

### 4. Lint gate

```bash
cd cmd && RIMSKY_REPO="$RIMSKY_REPO" go run ./rimsky-docs-lint all
```

If any lint fails, hand the failures to a fixer subagent (step 5) and re-run
until clean.

### 5. Review/refine — invoke `/refine-docs`

Invoke the `/refine-docs` skill (via the Skill tool). It runs the review → fix →
converge loop over the surfaces this run changed, against the same `RIMSKY_REPO`
and the same `.build-docs/journal.md` this run has been writing; it re-runs the
lint gate each round, converges (two dry rounds, or nine rounds max), and renders
the convergence summary + attention table into `.build-docs/report.md`. Pass it
any red test or smoke result from step 3 as seed issues. See `/refine-docs` for
the loop mechanics and the journal / report format — including the reviewer and
fixer subagent templates, which now live there.

### 6. Report

`/refine-docs` (step 5) has already rendered the **convergence summary** and the
**attention table** (judgment calls · source-side conflicts · declined
additions) into `.build-docs/report.md` from the run journal. Prepend the run
header and print the whole thing:

- the ref reconciled against (release tag, or `local: <branch>@<sha>` override);
- the final build / test / smoke / lint results;
- the per-surface change summary: created / refined / removed.

Do not commit — this repo's rules forbid commits unless the user explicitly
asks.

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
> Sources (rimsky v0.4.0 — the Go code lives under `lib/`):
> `${RIMSKY_REPO}/lib/protocols/proto/v1/*.proto` and the generated
> `docs/protocols/reference/`; the protocols module's hand-written Go packages at
> `${RIMSKY_REPO}/lib/protocols/{claimproducer,lifecycle,action,serverkit,publisherkit,conformance}`
> and the generated `docs/protocols/go-packages.md`; rimsky's code-side READMEs
> and concrete service implementations under
> `${RIMSKY_REPO}/lib/services/{stores,executors,sensors,subscribers}` (reintegrated
> in-tree — there is **no** separate `rimsky-services` repo); the `rimsky.yml`
> config schema (loader at `lib/control/config`); and the control-API routes
> (`lib/control/controlapi`). Conformance is the `rimsky conformance <protocol>`
> CLI subcommand, not a standalone `cmd/rimsky-*-conformance` binary. When the
> code says something different from a guide, the code wins.
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

### cookbook

> Reconcile `docs/cookbook/` to the **minimal canonical set of patterns rimsky's
> primitives span** — patterns one level more general than tutorials (e.g. "build
> a one-node queue worker", "fan out over a group of folders with a partitioned
> claim", "modify local files via an executor proxy", "a loop that retries until
> a payload settles"). The cookbook is a *derived* surface: its membership comes
> from what rimsky can actually do, not from ad-hoc ideas.
>
> Work it as coverage → parsimony, not intake:
> 1. **Enumerate broadly.** From the published concepts (`docs/concepts/`) and
>    rimsky's primitives — claims, claim scopes, named locks, fan-out, cascade,
>    loops/recursion, the executor/publisher/subscriber protocols, sensors —
>    enumerate the distinct usage patterns those primitives and their
>    combinations enable.
> 2. **Reduce to a minimal unique set.** Cluster near-duplicates and pick one
>    canonical representative per cluster; drop variations that teach no distinct
>    lesson. The goal is a small spanning set, not an exhaustive pile.
> 3. **Check coverage.** Look for primitive-combinations no canonical pattern
>    exercises yet — those are gaps to add. Do not silently prune: every
>    candidate you merged or dropped is a `decision` entry (what you kept, what it
>    subsumed, why), and any combination you can't yet ground on the stacks is a
>    `flag`.
> 4. **Reconcile the cookbook to that set.** Create canonical patterns that are
>    missing; merge/retire entries that are redundant or no longer canonical;
>    refine the survivors. Structure each: problem statement, the rimsky shape
>    (which primitives and why), a complete walkthrough that runs on the published
>    `deploy/` stack, and a "without rimsky" baseline. Every recipe must be
>    runnable on the `deploy/` stack — a reader should be able to run it.
>
> Return: the canonical pattern set (created / refined / merged / retired, with a
> one-line rationale each), the `decision` entries for merges and drops, and any
> pattern whose walkthrough you could not make runnable against the current
> `deploy/` stack (as `flag` entries).

### agents-index

> Audit the curated agent surface for drift; do not overwrite it.
>
> Read `docs/agents/llms.txt` and every file it links to; verify the links
> resolve and the index still describes what those files contain. Read
> `docs/agents/errors/` against rimsky's actual error classes and
> `docs/agents/examples/` (template/instance walkthroughs) against rimsky's
> current behavior. These surfaces are human-curated — your job is to flag drift
> (dead links, missing error classes newly present in rimsky, walkthroughs that
> no longer match rimsky), not to rewrite.
>
> Do not touch `docs/agents/llms-full.txt` — it is mechanically regenerated.
>
> Return: a list of drift findings for human attention. Make no edits.

### bundles (prose only)

> Reconcile the prose of the `deploy/` bundle against its code/config and the
> published docs. Surface: the `deploy/` compose + config comments and the
> operator-facing deploy guidance in `docs/operator-guide.md`.
>
> Do not build or test here — that happens in the orchestrator. Your job is text
> coherence: the compose/config comments and the operator guide must match what
> the code and configs actually do, and every doc link must resolve into the
> published surface.
>
> Return: prose reconciled, and any link or claim that does not match the code /
> config / published surface.

### catalogs (services + images)

> Reconcile the two derive-and-verify reference catalogs against rimsky source.
> Surfaces: `docs/services/` (the bundled services —
> `${RIMSKY_REPO}/lib/services/{stores,executors,sensors,subscribers}` plus the
> `deploy/` config that wires them) and `docs/images/` (the official images —
> `${RIMSKY_REPO}/dockerfiles/` + per-service `lib/services/.../Dockerfile.*` +
> the image names in `deploy/docker-compose.yml`).
>
> For each service: what it is, the protocol(s) it implements, its config keys /
> env vars, ports, and the Dockerfile that builds it — verified against the
> service's own config struct and the `deploy/` wiring; the code wins. For each
> image: name (as tagged in deploy compose), contents (binary/service), base, and
> the Dockerfile + build context. Note that all images build from the rimsky
> checkout (no ghcr) and the dashboard builds from the `rimsky-dashboard`
> sibling. Clean end-user prose; every internal link resolves; cross-link to the
> relevant `docs/concepts/` and `docs/protocols/` pages.
>
> Return: services / images cataloged (created / refined), and any service or
> image whose config / ports / Dockerfile you could not resolve in the source
> (as `flag` entries).

The **reviewer** and **fixer** subagent templates used by the review/refine loop
live in the `/refine-docs` skill (which owns that loop). They are not duplicated
here.

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
