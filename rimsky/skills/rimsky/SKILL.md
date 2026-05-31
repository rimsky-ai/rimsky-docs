---
name: rimsky
description: >-
  Rimsky knowledge — use when designing, building, deploying, debugging, or
  evaluating fit for a workflow on rimsky, the reactive node-graph orchestration
  platform. Covers the node-graph execution model (nodes, invalidate, cascade,
  the scheduler), the full concept catalog, protocol implementation (executor,
  claim-producer, lifecycle-subscriber, publisher), the rimsky.yml / template
  schema, the REST and CLI control surfaces, cookbook and systems-design
  patterns, deployment, and an error catalog. This is a read-only reference: it
  answers questions and points you at the bundled docs on demand. It does not
  drive your design process, so it composes with your own planning workflow or
  plan mode.
---

# Rimsky

Rimsky is a project-agnostic, reactive **node-graph orchestration platform**. This
skill is a router over a bundled documentation corpus reconciled against a pinned
rimsky release (see `plugin.json` for the version). Rimsky is new enough that it is
almost certainly **not** in your training data — treat this corpus, not your
priors, as the source of truth, and read the relevant files before answering.

## The mental model (read this first)

A workflow is a **graph of nodes**. Each node computes some state from its inputs.
When something changes, an **`invalidate`** marks the affected nodes stale and
propagates along the graph's edges — that propagation is a **cascade**. The
**scheduler** then recomputes the stale nodes that have become eligible. Work that
must be exclusive, partitioned, or handed off between nodes is coordinated through
**claims** (leases over a scope) issued by a **claim producer**. The actual compute
for a node is performed by a pluggable **executor**. You declare all of this as a
**template** (a `rimsky.yml`); a running graph is an **instance**.

That paragraph is the whole shape. Everything below tells you which files to open
for a given task — you should not read the whole corpus.

## How to use this skill

- Paths below are **relative to this file**; the corpus is under `docs/`.
- Open only the slice your task needs. The concept catalog alone is ~70 files —
  the routing below tells you which handful are load-bearing for what you're doing.
- Two definitive layers never get hand-written and are the final word when prose
  and reference disagree: the generated `docs/reference/` (schema, REST, CLI) and
  `docs/protocols/reference/` (wire protocol). Trust them over any guide.

## Route by what you're doing

### 1. Deciding whether rimsky fits

Read `docs/comparison.md` (rimsky vs. queues, workflow engines, build systems) and
`docs/concepts/rimsky.md`. Good fit signals: reactive recomputation over a
dependency graph, fan-out over partitions, exclusive/leased work, event-driven
recompute, convergence loops. Poor fit: a plain linear job or a stateless request.

### 2. Designing — modeling your problem as a graph

Start with the core nouns (next section), then **match your problem against a
cookbook recipe** — these map the primitives onto real shapes:

- `docs/cookbook/queue-worker.md` — claim-as-queue, one node draining work
- `docs/cookbook/reactive-recompute.md` — cascade-driven stale-marking
- `docs/cookbook/event-driven-node.md` — a node triggered by an external signal
- `docs/cookbook/convergence-loop.md` — retry/recompute until a payload settles
- `docs/cookbook/capacity-limit.md` — bounding concurrency with claims/locks
- `docs/cookbook/claim-handoff.md` — passing a held claim down a node chain

For higher-altitude system shapes see `docs/patterns/` (`domain-stores.md`,
`operational-health.md`). Working templates to copy live in `docs/agents/examples/`
(`minimal-rimsky-yml.md`, `minimal-template-and-instance.md`,
`two-node-with-claim.md`, `holding-subgraph.md`).

### 3. Implementing — writing it / extending the platform

- **The template you write:** `docs/reference/template-schema.md` — the complete,
  generated `rimsky.yml` / template schema. This is the definitive surface for
  every key you can set on a node, claim, or graph.
- **Implementing a protocol service** (only if a bundled service doesn't suffice):
  the guides under `docs/protocols/` — `executor.md`, `claim-producer.md`,
  `lifecycle-subscriber.md`, `publisher.md` — plus the generated wire reference
  `docs/protocols/reference/` and the optional Go helper packages
  `docs/protocols/go-packages.md`. Conformance: `docs/concepts/conformance.md`.
- **Picking bundled building blocks:** `docs/stores/`, `docs/executors/`,
  `docs/blob-backends/`, `docs/mcp-servers/`.

### 4. Deploying — running it

`docs/operator-guide.md` for the operational story; `docs/services/` (the bundled
services: protocol, config, ports, image) and `docs/images/` (the official Docker
images); `docs/reference/rest-api.md` (the control-API routes + auth) and
`docs/reference/cli.md` (the `rimsky` CLI command tree).

### 5. Diagnosing — when something breaks

`docs/agents/errors/` — one file per error code (start at its `README.md`). Each
explains what raised it, what it means, and how to resolve it.

## Designing: which concepts to read

The catalog is `docs/concepts/`. Don't read it all. Start with this core set:

- `rimsky.md`, `graph.md`, `node.md`, `node-run.md` — the execution model
- `invalidate.md`, `cascade.md`, `frame.md` — how change propagates
- `template.md`, `instance.md`, `role-template.md` — declaration vs. running graph
- `claim.md`, `executor.md` — coordination and compute

Then pull in only what your problem touches:

- **Exclusive / partitioned / handed-off work:** `claim-handle.md`,
  `claim-scope.md`, `claim-producer.md`, `claim-tree.md`, `claim-lifetime.md`,
  `claim-co-holdership.md`, `named-lock.md`, `advisory-lock.md`
- **Fan-out / sub-workflows:** `fan-out.md`, `sub-graph.md`, `cascade-graph.md`,
  `backfill.md`
- **External triggers / messaging:** `sensor.md`, `signal.md`, `message.md`,
  `named-event.md`, `publisher.md`, `node-subscription.md`
- **Failure / retries / lifecycle:** `error-policy.md`, `wait-set.md`,
  `parked-state.md`, `terminal-resolution.md`, `orphan-reaper.md`
- **State / config / observability:** `attribute.md`, `tag.md`, `rimsky-yml.md`,
  `lineage.md`, `event-log.md`, `observability.md`, `write-semantics.md`,
  `atomic-staging.md`

`docs/glossary.md` is the full, generated vocabulary if you hit an unfamiliar term.

## Other indices

- `docs/agents/llms.txt` — a compact llms.txt-style index over the same corpus, for
  tooling that prefers it (this `SKILL.md` is the primary entry point for Claude
  Code; `llms.txt` is the entry point for other agents).
- `docs/agents/llms-full.txt` — the entire concept + protocol corpus concatenated
  into one file, for agents that can fetch a single large file but not crawl.

## Source of truth

These docs are derived from and verified against the rimsky source repository
(`github.com/rimsky-ai/rimsky-core`) at the release recorded in `plugin.json`. The
repository is the ultimate source of truth; this corpus is its reconciled,
agent-facing projection. If you find drift, trust the generated reference files
first, then the source repository.
