# Rimsky documentation

This directory is the **agent-facing documentation corpus** for rimsky. Every
file here is part of the published surface — it is meant to be read and cited by
coding agents. There is no separate internal/working surface in this tree.

## Source code

Rimsky's implementation lives in the public repository [`github.com/rimsky-ai/rimsky-core`](https://github.com/rimsky-ai/rimsky-core). The documentation here is generated and reconciled against that repository's pinned release tag — the exact pinned version is recorded in the plugin manifest (`.claude-plugin/plugin.json`), the single source of the reconciled-against release. The prose is derived from the source and verified against it, but the source of truth is the repository, not these docs. The `.proto` wire-IDL files live in the rimsky repository under `lib/protocols/proto/v1/`; they are **not** shipped on this docs surface (this surface carries generated `reference/` projections of them instead).

## Entry points

- `../SKILL.md` — the Claude Code entry point: a router over this corpus.
- `agents/llms.txt` and `agents/llms-full.txt` — the entry point for other agents.

## How concepts are named

The canonical per-noun reference ships on this surface:

- `concepts/` — one file per concept (~70 files), each with `What it is` /
  `Purpose` / `Boundaries` / `Invariants`. This is the durable design catalog.
- `glossary.md` — generated one-line index over the concept vocabulary. Do not
  hand-edit.

Inline `@concept:` annotations in the rimsky source point at the code sites that
enforce each concept; the definitions they refer to are the `concepts/` files
here.

## The surface

Guides (hand-maintained):

- `comparison.md` — rimsky vs. other orchestrators; fit evaluation.
- `roadmap.md` — what shipped, what's in active design, declared non-goals.
- `operator-guide.md` — cross-cutting operator knobs (config, blob backend,
  metrics, diagnostic endpoints).
- `licensing.md` — repo licensing notice.

Concept catalog:

- `concepts/` — per-noun reference (see above).
- `glossary.md` — generated vocabulary index. Do not hand-edit.

Protocol implementation:

- `protocols/` — protocol-implementation guides (`claim-producer`, `executor`,
  `lifecycle-subscriber`, `publisher`) plus `go-packages.md`.
- `protocols/reference/` — generated wire-contract reference (per-protocol proto
  projections). Definitive; do not hand-edit.

Generated reference:

- `reference/` — generated definitive references: the template / `rimsky.yml`
  schema (`template-schema.md`), the REST control-API routes (`rest-api.md`),
  the CLI command tree (`cli.md`), and a reference config under `reference/config/`.
  Do not hand-edit.

Bundled building blocks (catalogs):

- `services/` — catalog of the reference services rimsky ships (stores, executors,
  sensors, subscribers): protocol, config, ports, Dockerfile.
- `images/` — catalog of the official Docker images: name, contents, base image,
  build context.
- `stores/`, `executors/`, `blob-backends/`, `mcp-servers/` — per-building-block
  pages for selecting a bundled store, executor, blob backend, or MCP server.

Design and recipe material:

- `cookbook/` — copyable recipes mapping primitives onto real shapes
  (queue-worker, reactive-recompute, event-driven-node, convergence-loop,
  capacity-limit, claim-handoff, sub-graph).
- `patterns/` — higher-altitude system shapes (`domain-stores`,
  `operational-health`).

Agent-shaped indices:

- `agents/` — `llms.txt`, `llms-full.txt`, an error catalog under `agents/errors/`,
  and copy-pasteable templates under `agents/examples/`.

The corpus is self-contained: it cites only within itself and into the rimsky
repository. The two generated layers — `reference/` and `protocols/reference/` —
are the final word when prose and reference disagree.
