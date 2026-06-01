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

The corpus is packaged as an installable Claude Code skill. This repo is a
**plugin marketplace** (`.claude-plugin/marketplace.json`) shipping one plugin,
`rimsky`, whose skill bundles the whole corpus under `rimsky/skills/rimsky/`.
An agent reaches the corpus through one of two entry points — `SKILL.md` (the
Claude Code skill router) and `rimsky/skills/rimsky/docs/agents/llms.txt` (the
llms.txt entry for other agents) — and the plugin version
(`rimsky/.claude-plugin/plugin.json`) tracks the rimsky release this run
reconciles against. The skill router and that version stamp are reconciled
surfaces too (see the ownership table and the `skill-packaging` surface), not
set-and-forget files.

The skill is the orchestrator. It does not write docs or run builds itself —
it dispatches one subagent per surface, calls the repo's Go binaries for
mechanical generation, runs the tooling tests, gates on the lint suite, runs a
review-and-fix loop, and reports. A run is not done until the build, tests, and
lints are all green. Verifying the docs against rimsky's *runtime behavior* is a
separate, optional pass (step 3b) — this is a documentation project, not a test
harness for rimsky.

## The source of truth

The source of truth is the **latest public release** of rimsky — the official
repo `https://github.com/rimsky-ai/rimsky-core` at its newest release tag, not
an unreleased `dev` working tree. Preflight (step 0) resolves that tag and
acquires a read-only checkout of it; `${RIMSKY_REPO}` points there. Three kinds
of source feed the published surface; every surface reconciles against one of
them:

- **Design concept catalog** — `${RIMSKY_REPO}/.ok-planner/design/concepts/*.md`.
  One file per noun in rimsky, carrying the full architectural story. Feeds
  `rimsky/skills/rimsky/docs/concepts/`.
- **Code, protos, and config schema** — `${RIMSKY_REPO}/lib/protocols/proto/v1/*.proto`,
  the protocols module's hand-written Go packages
  (`${RIMSKY_REPO}/lib/protocols/{claimproducer,lifecycle,action,serverkit,publisherkit,conformance}`),
  rimsky's code-side READMEs (`lib/services/executors/`, etc.), the `rimsky.yml`
  config schema, and the control-API routes. Feeds the protocol / store /
  executor / blob-backend / mcp-server reference and the operator material.
  These docs are **generated from the code**.
- **Runtime behavior** — optionally checked by standing up rimsky from the
  published images and exercising the documented behavior (step 3b). Not a
  gate; its value is catching rimsky-core bugs, which are recorded in the bug
  list, not fixed here.

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
  its own `Dockerfile.*`. There is **no separate `rimsky-services` repo**; the
  bundled services build from the rimsky checkout (no ghcr pull at build time).
  The standalone stub executor (`lib/services/test/stubexecutor/`) is
  Execute-only — it advertises no attribute schema, so a node with an
  `attributes:` block on it fails dispatch with `executor_schema_unavailable`;
  the bundled `http-node`/`claude-agent` executors (stub mode) *do* advertise a
  schema. The stub claim-producer is an in-process test double at
  `test/support/stores/stub/` with **no** standalone binary, so a stub-based
  deployment uses the filesystem store as its claim-producer.
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
`../rimsky-dashboard`, override `RIMSKY_DASHBOARD_REPO`); it builds from source
and is not part of the published image set.

## What this skill owns

| Surface | Source | Ownership |
|---|---|---|
| `rimsky/skills/rimsky/docs/concepts/*.md` | design concept catalog | Skill-owned. Near-verbatim copy of `design/concepts/*.md`, minus design-process scaffolding (tensions/discovery refs). Overwrite. |
| `rimsky/skills/rimsky/docs/protocols/*.md` guides, `rimsky/skills/rimsky/docs/stores/`, `rimsky/skills/rimsky/docs/executors/`, `rimsky/skills/rimsky/docs/blob-backends/`, `rimsky/skills/rimsky/docs/mcp-servers/` | code + protos + rimsky READMEs + the generated references | Agent-written explanation layer; refine against source. |
| `rimsky/skills/rimsky/docs/protocols/reference/*.md` | rimsky `lib/protocols/proto/v1/*.proto` | Mechanical (`rimsky-docs-proto`); never hand-edit. |
| `rimsky/skills/rimsky/docs/protocols/go-packages.md` | rimsky `lib/protocols/` hand-written Go packages (godoc) | Mechanical (`rimsky-docs-gopkg`); never hand-edit. |
| `rimsky/skills/rimsky/docs/reference/template-schema.md` | rimsky `lib/foundation/spec/` structs | Mechanical (`rimsky-docs-template-ref`); never hand-edit. |
| `rimsky/skills/rimsky/docs/reference/rest-api.md` | rimsky `lib/control/controlapi/actions.go` | Mechanical (`rimsky-docs-rest-ref`); never hand-edit. |
| `rimsky/skills/rimsky/docs/reference/cli.md` | the `rimsky` CLI's `help` output | Mechanical (`rimsky-docs-cli-ref`); never hand-edit. |
| `rimsky/skills/rimsky/docs/operator-guide.md`, `rimsky/skills/rimsky/docs/comparison.md`, `rimsky/skills/rimsky/docs/roadmap.md`, `rimsky/skills/rimsky/docs/licensing.md` | code + design | Generated/derived; refine. |
| `rimsky/skills/rimsky/docs/glossary.md` | rimsky's `concepts.md` catalog | Mechanical; published verbatim by the glossary binary. Never hand-edit. |
| `rimsky/skills/rimsky/docs/agents/llms-full.txt`, root `llms.txt` / `llms-full.txt` | the Go binaries | Mechanical; regenerate, never hand-edit. |
| `rimsky/skills/rimsky/docs/cookbook/*.md` | rimsky's primitives + concepts | Derived: the minimal canonical set of patterns rimsky's primitives span, reconciled against the concepts. Each recipe is shape → primitives → a copyable template → gotchas, runnable against a rimsky deployment. Skill-owned set — add canonical patterns, merge/retire redundant ones, refine the rest. |
| `rimsky/skills/rimsky/docs/agents/llms.txt` | the files it links to | Curated index. Flag drift; do not overwrite. |
| `rimsky/skills/rimsky/docs/agents/errors/`, `rimsky/skills/rimsky/docs/agents/examples/` | code error classes + template/instance examples | Mostly human. Flag drift/additions; do not overwrite. |
| `rimsky/skills/rimsky/docs/services/` | rimsky `lib/services/` + the reference config | Derive-and-verify catalog of the bundled services (config / ports / protocols / image). Refine against source. |
| `rimsky/skills/rimsky/docs/images/` | rimsky `dockerfiles/` + per-service Dockerfiles | Derive-and-verify catalog of the published images. Refine against source. |
| `rimsky/skills/rimsky/SKILL.md` | the corpus it routes to | Skill-owned router. Keep the mental model, the fit→design→implement→deploy→diagnose routing, and the concept-triage current as concepts / protocols / recipes are added or removed; every path it links must resolve. Refine, don't flatten. |
| `rimsky/.claude-plugin/plugin.json` | the reconciled rimsky release | Set `version` to the release tag this run reconciled against ("the skill is the version"). |
| `.claude-plugin/marketplace.json` | the plugin set | Stable. Flag drift only (e.g. a renamed plugin or changed `source`); do not churn. |
| `rimsky/skills/rimsky/docs/reference/config/` | `rimsky.yml` schema + the bundled services | Worked example configs (the unified `rimsky.yml`, the store and supervisor configs). Refine against the schema; keep them valid and copyable, with no removed-stack hostnames. |

"Refine" on a hand-shaped surface means: **bring the facts, vocabulary, and
links current against source, in the agent-doc style**
(`.claude/rules/agent-doc-style.md`) — update what is stale, keep the reasoning
prose intact, and move drifted prose toward the style (tables for enumerables,
explicit boundaries) without losing information. Generated surfaces have no such
constraint; they are fully skill-owned.

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
  helper packages — the *generated* references under `rimsky/skills/rimsky/docs/protocols/reference/`
  (wire) and `rimsky/skills/rimsky/docs/protocols/go-packages.md` (Go packages) are mechanical, step 2)
- cookbook
- catalogs (the `rimsky/skills/rimsky/docs/services/` and `rimsky/skills/rimsky/docs/images/` derive-and-verify
  catalogs — the *generated* references under `rimsky/skills/rimsky/docs/reference/` are mechanical,
  step 2)
- agents-index (llms.txt + errors/ + examples/ drift flagging)
- config-examples (the worked configs under `docs/reference/config/` — verify
  against the schema and the services they configure)
- skill-packaging (the `SKILL.md` router, the two-entry-point parity between
  `SKILL.md` and `docs/agents/llms.txt`, and the `plugin.json` version stamp)

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

`rimsky-docs-llms-full` writes `rimsky/skills/rimsky/docs/agents/llms-full.txt` + the repo-root copy
(concatenating rimsky's design concept bodies and the published `rimsky/skills/rimsky/docs/protocols/`
guides). `rimsky-docs-glossary` publishes `rimsky/skills/rimsky/docs/glossary.md` as a verbatim copy
of rimsky's `.ok-planner/design/concepts.md` catalog. `rimsky-docs-proto`
generates the wire-protocol reference under `rimsky/skills/rimsky/docs/protocols/reference/` from
rimsky's `lib/protocols/proto/v1/*.proto` (services, messages, fields, enums + their
proto comments). `rimsky-docs-gopkg` generates `rimsky/skills/rimsky/docs/protocols/go-packages.md`
from the godoc of the protocols module's hand-written Go packages
(`claimproducer`, `lifecycle`, `action`, `serverkit`, `publisherkit`,
`conformance`) — the optional helpers a Go service may import; the `proto/`
bindings are excluded (they are the wire reference's job).

The three `rimsky/skills/rimsky/docs/reference/` generators are the **definitive specifics layer** an
agent reads to build a template / deployment plan: `rimsky-docs-template-ref`
writes `rimsky/skills/rimsky/docs/reference/template-schema.md` (the complete template / `rimsky.yml`
schema, from the spec structs under `lib/foundation/spec/`); `rimsky-docs-rest-ref`
writes `rimsky/skills/rimsky/docs/reference/rest-api.md` (every control-API route + method + per-action
auth gate, from the action registry `lib/control/controlapi/actions.go`); and
`rimsky-docs-cli-ref` writes `rimsky/skills/rimsky/docs/reference/cli.md` from the `rimsky` CLI's own
`help` output (it shells `go run ./cmd/rimsky` inside `${RIMSKY_REPO}`, so that
checkout must build — slower than the others).

All of these are mechanical — never hand-edit them.

### 3. Verify docs against code — the gate

The gate is **docs-vs-code**: build and test the tooling, then lint (step 4).
Run it every pass; a run is not done until it is green.

```bash
cd cmd && go build ./... && go test ./...
```

This proves the published surface is internally consistent and faithful to the
rimsky *source* — proto symbols, Go symbols, config keys, routes, and links all
resolve. It does **not** run rimsky.

### 3b. Verify docs against behavior — optional bug-finding pass

Separately and optionally, stand up rimsky from the **published images**
(`rimskyai/rimsky*:<release>`; see the operator guide) and exercise the
behavior the docs describe — run a cookbook recipe end to end, drive a
template, watch the diagnostics. This is **not a gate**, and this repo ships no
stack to run; its value is that it surfaces **rimsky-core bugs**, which are
recorded in the bug list (`.build-docs/` run scratch, promoted to rimsky-core
when curated) — *not* fixed here. Skip it freely; a green gate (step 3 +
step 4) is what a run requires.

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
any red test or lint result from steps 3–4 as seed issues. See `/refine-docs` for
the loop mechanics and the journal / report format — including the reviewer and
fixer subagent templates, which now live there.

### 6. Report

`/refine-docs` (step 5) has already rendered the **convergence summary** and the
**attention table** (judgment calls · source-side conflicts · declined
additions) into `.build-docs/report.md` from the run journal. Prepend the run
header and print the whole thing:

- the ref reconciled against (release tag, or `local: <branch>@<sha>` override);
- the final build / test / lint results (plus any step-3b behavior findings);
- the per-surface change summary: created / refined / removed.

Do not commit — this repo's rules forbid commits unless the user explicitly
asks.

## Surface subagent templates

Dispatch each as an `Agent` (general-purpose). Every prompt is self-contained:
the subagent has not seen this conversation. Always pass the absolute
`RIMSKY_REPO` path and the repo root, and instruct the subagent to write every
hand-shaped surface in the **agent-doc style** (`.claude/rules/agent-doc-style.md`):
assertion-first, tables for enumerables, explicit boundaries (what it is and is
not), reasoning kept as tight prose, source-anchored.

### concepts

> Reconcile the published per-concept pages `rimsky/skills/rimsky/docs/concepts/*.md` against
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
>   `concept:<slug>` links (those resolve within `rimsky/skills/rimsky/docs/concepts/`). Do NOT
>   rewrite or "improve" the prose, and do NOT author new frontmatter fields.
>
> For every concept in the source: ensure `rimsky/skills/rimsky/docs/concepts/<slug>.md` exists
> (create if missing, refresh from source if present). Remove published pages
> whose source concept no longer exists. Keep the set internally consistent:
> every surviving `concept:<slug>` link must point at a concept that still has a
> published page.
>
> The glossary (`rimsky/skills/rimsky/docs/glossary.md`) is NOT your job — it is published verbatim
> from rimsky's `concepts.md` by the glossary binary in the mechanical step.
>
> Return: pages created / refreshed / removed, and any surviving
> `concept:<slug>` link that does not resolve to a published concept.

### code-reference

> Reconcile the prose **guides** for the code-facing surfaces against rimsky's
> code, protos, and config schema. Surfaces: `rimsky/skills/rimsky/docs/protocols/*.md` (the
> "Implementing an X" guides), `rimsky/skills/rimsky/docs/stores/`, `rimsky/skills/rimsky/docs/executors/`,
> `rimsky/skills/rimsky/docs/blob-backends/`, `rimsky/skills/rimsky/docs/mcp-servers/`.
>
> You write the *explanations*, not the reference. Two machine-readable
> references are generated mechanically and you must NOT hand-write or duplicate
> them — point at them instead: the wire reference under
> `rimsky/skills/rimsky/docs/protocols/reference/` (`rimsky-docs-proto`, from the `.proto` files), and
> `rimsky/skills/rimsky/docs/protocols/go-packages.md` (`rimsky-docs-gopkg`, the godoc of the
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
> narrative belongs in `rimsky/skills/rimsky/docs/protocols/` guides, cross-linking into
> `rimsky/skills/rimsky/docs/protocols/go-packages.md` rather than restating signatures.
>
> Sources (rimsky v0.4.0 — the Go code lives under `lib/`):
> `${RIMSKY_REPO}/lib/protocols/proto/v1/*.proto` and the generated
> `rimsky/skills/rimsky/docs/protocols/reference/`; the protocols module's hand-written Go packages at
> `${RIMSKY_REPO}/lib/protocols/{claimproducer,lifecycle,action,serverkit,publisherkit,conformance}`
> and the generated `rimsky/skills/rimsky/docs/protocols/go-packages.md`; rimsky's code-side READMEs
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

> Reconcile `rimsky/skills/rimsky/docs/cookbook/` to the **minimal canonical set of patterns rimsky's
> primitives span** — patterns one level more general than tutorials (e.g. "build
> a one-node queue worker", "fan out over a group of folders with a partitioned
> claim", "modify local files via an executor proxy", "a loop that retries until
> a payload settles"). The cookbook is a *derived* surface: its membership comes
> from what rimsky can actually do, not from ad-hoc ideas.
>
> Work it as coverage → parsimony, not intake:
> 1. **Enumerate broadly.** From the published concepts (`rimsky/skills/rimsky/docs/concepts/`) and
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
>    subsumed, why), and any combination the bundled producers/services can't
>    support is a `flag`.
> 4. **Reconcile the cookbook to that set.** Create canonical patterns that are
>    missing; merge/retire entries that are redundant or no longer canonical;
>    refine the survivors. Structure each: problem statement, the rimsky shape
>    (which primitives and why), a walkthrough — the copyable template plus the
>    register/deploy/instantiate steps, runnable against a rimsky deployment —
>    and a "without rimsky" baseline. Optimize for what an agent copies: the
>    exact template and the gotchas, not a turnkey stack.
>
> Return: the canonical pattern set (created / refined / merged / retired, with a
> one-line rationale each), the `decision` entries for merges and drops, and any
> pattern whose template you could not validate against the schema (as `flag`
> entries).

### agents-index

> Audit the curated agent surface for drift; do not overwrite it.
>
> Read `rimsky/skills/rimsky/docs/agents/llms.txt` and every file it links to; verify the links
> resolve and the index still describes what those files contain. Read
> `rimsky/skills/rimsky/docs/agents/errors/` against rimsky's actual error classes and
> `rimsky/skills/rimsky/docs/agents/examples/` (template/instance walkthroughs) against rimsky's
> current behavior. These surfaces are human-curated — your job is to flag drift
> (dead links, missing error classes newly present in rimsky, walkthroughs that
> no longer match rimsky), not to rewrite.
>
> Do not touch `rimsky/skills/rimsky/docs/agents/llms-full.txt` — it is mechanically regenerated.
>
> Return: a list of drift findings for human attention. Make no edits.

### config-examples

> Reconcile the worked example configs under
> `rimsky/skills/rimsky/docs/reference/config/` (the unified `rimsky.yml`, the
> store configs, the supervisor config) and the operator-facing guidance in
> `rimsky/skills/rimsky/docs/operator-guide.md` against rimsky's config schema
> and the bundled services.
>
> Do not build or run rimsky here. Your job is correctness: every config key is
> a real key the loader accepts, the values are valid and copyable, the configs
> match the published image names / ports, and every doc link resolves into the
> published surface. Where a config references a removed compose-stack hostname
> or port, neutralize it to a deployment-agnostic example.
>
> Return: configs and guidance reconciled, and any config key or value that does
> not match the schema or the services (as `flag` entries).

### catalogs (services + images)

> Reconcile the two derive-and-verify reference catalogs against rimsky source.
> Surfaces: `rimsky/skills/rimsky/docs/services/` (the bundled services —
> `${RIMSKY_REPO}/lib/services/{stores,executors,sensors,subscribers}` plus the
> reference config that wires them) and `rimsky/skills/rimsky/docs/images/` (the published images —
> `${RIMSKY_REPO}/dockerfiles/` + per-service `lib/services/.../Dockerfile.*` +
> the published image names under `docker.io/rimskyai`).
>
> For each service: what it is, the protocol(s) it implements, its config keys /
> env vars, ports, and the Dockerfile that builds it — verified against the
> service's own config struct and the reference config; the code wins. For each
> image: published name, contents (binary/service), base, and
> the Dockerfile + build context. Note that all images build from the rimsky
> checkout (no ghcr) and the dashboard builds from the `rimsky-dashboard`
> sibling. Clean end-user prose; every internal link resolves; cross-link to the
> relevant `rimsky/skills/rimsky/docs/concepts/` and `rimsky/skills/rimsky/docs/protocols/` pages.
>
> Return: services / images cataloged (created / refined), and any service or
> image whose config / ports / Dockerfile you could not resolve in the source
> (as `flag` entries).

### skill-packaging

> Reconcile the skill packaging that wraps the corpus. Surfaces:
> `rimsky/skills/rimsky/SKILL.md` (the skill router), `.claude-plugin/marketplace.json`
> (marketplace manifest), and `rimsky/.claude-plugin/plugin.json` (plugin manifest).
>
> The corpus lives at `rimsky/skills/rimsky/docs/`. The marketplace is the repo;
> it lists exactly one plugin, `rimsky`, whose single skill bundles that corpus.
> An installed skill can only read files under its own directory, so everything
> the router points at must live under `rimsky/skills/rimsky/`.
>
> 1. **`SKILL.md` router.** It is a *router*, not a copy of the corpus: a mental
>    model, then task-based routing (fit → design → implement → deploy →
>    diagnose) into specific corpus files. Reconcile it against the corpus as it
>    now stands — every relative path it names must resolve under
>    `rimsky/skills/rimsky/docs/`; new concept clusters, protocols, or cookbook
>    recipes that change the routing should be reflected; retired files must not
>    be linked. Keep the `description` frontmatter breadth-advertising (design,
>    implementation, deployment, debugging, fit) and keyword-front so other
>    agents' workflows and plan mode pull it in; leave it model- and
>    user-invocable (no `disable-model-invocation`) so it composes. Refine; do
>    not flatten the routing.
> 2. **Two-entry parity.** `SKILL.md` (Claude Code) and
>    `rimsky/skills/rimsky/docs/agents/llms.txt` (other agents) are the two entry
>    points into the same corpus. Flag drift between what they advertise (a
>    surface present in one entry's map but absent from the other).
> 3. **`plugin.json` version.** Set `version` to the rimsky release tag this run
>    reconciled against — the skill *is* that version. (`marketplace.json` is
>    stable; flag drift, don't churn it.)
>
> Return: router / manifest changes made, any router link that does not resolve
> under the corpus, any entry-point parity drift (as `flag` entries), and the
> version the stamp was set to.

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
