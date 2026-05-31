# Cookbook

Patterns one level more general than a single tutorial: a problem you
recognize, the rimsky shape that solves it, a runnable walkthrough on the
published [`deploy/`](../../deploy/) stack, and a "without rimsky" baseline
so the trade is legible.

Each recipe states:

1. **The problem** — what you're trying to build, in plain terms.
2. **The rimsky shape** — which primitives it uses and why.
3. **A runnable walkthrough** — copy-paste steps that work on the published stack.
4. **Without rimsky** — what you'd hand-roll otherwise, for contrast.

This is a *spanning* set, not an exhaustive one: each recipe teaches a
distinct lesson about what the primitives can do, and near-duplicates are
folded into one canonical representative. The patterns combine — a queue
worker that loops is the queue recipe plus the loop recipe; a reactive
graph behind a capacity limit is two recipes composed.

## The recipes

All recipes run on the published [`deploy/`](../../deploy/) stack.

- **[A single-node queue worker](queue-worker.md)** — a claim producer as
  a work queue (the postgres `@review-queue` pick policy) drained by a
  self-subscribing node.
- **[Recompute dependents when something upstream changes](reactive-recompute.md)**
  — subscriber-driven cascade: a downstream node auto-subscribes to an
  upstream attribute and recomputes only the affected nodes on change.
- **[Cap concurrency with a counting semaphore](capacity-limit.md)** — a
  named lock as a deployment-wide capacity counter (`model-budget`,
  limit 50).
- **[Loop until the work settles](convergence-loop.md)** — a
  self-subscribing node that re-fires under a `payload.changed` gate and is
  bounded by the no-progress retry cap. On `deploy/`'s stub-mode executor
  the gate fires every iteration, so the recipe demonstrates the loop shape
  and the runaway cap as the safety net.
- **[Drive a node from an external event](event-driven-node.md)** — an
  inbound message (operator- or publisher/sensor-emitted) delivered at a
  frame boundary and matched to a node by subscription.
- **[Hand a claim from one node to the next](claim-handoff.md)** — a claim
  co-held across a chain of nodes so the whole chain is one all-or-nothing
  transaction, committed or abandoned once at the end.

## Related surfaces

Two write-ups live in [`docs/patterns/`](../patterns/) rather than here —
they are operator/architecture patterns, not single-problem recipes:

- [Domain stores](../patterns/domain-stores.md) — holding project-specific
  state in an MCP server an agent executor consumes.
- [Operational health](../patterns/operational-health.md) — observing and
  maintaining a running deployment (lifecycle subscribers, watchdog
  graphs, diagnostics, retry-loop detection).

## Patterns that need a stack change first

Two patterns the primitives support are **not** runnable on the published
stacks as they stand, so they are not written up as recipes:

- **Fan out over a partitioned claim** (and the **backfill** that targets a
  fan-out node) requires a claim producer that advertises
  `supports_split_scope`. Neither bundled store does — the filesystem
  (`content`) and postgres (`topics-ring`) producers advertise only their
  write semantics — so a `fan_out:` node is rejected at template
  registration on both stacks. This recipe needs a split-scope-capable
  producer wired into a stack before it can be made runnable.
- **Modify local files through an executor proxy** (run an executor against
  files on a developer machine) requires a
  [host-agent proxy](../concepts/host-agent-proxy.md) service. Neither
  stack wires one, so this needs a stack change first.
