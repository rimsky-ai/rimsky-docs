# Cookbook

Patterns one level more general than a single tutorial: a problem you
recognize, the rimsky shape that solves it, a runnable walkthrough on the
published [`quickstart/`](../../quickstart/) or [`deploy/`](../../deploy/)
stack, and a "without rimsky" baseline so the trade is legible.

Each recipe states:

1. **The problem** — what you're trying to build, in plain terms.
2. **The rimsky shape** — which primitives it uses and why.
3. **A runnable walkthrough** — copy-paste steps that work on the published stack.
4. **Without rimsky** — what you'd hand-roll otherwise, for contrast.

## Status

This section is being built out — no recipes are published yet. Recipes are
authored one at a time via the docs pipeline (`/build-docs cookbook add
"<pattern name>"`), each verified runnable against the published stack before
it lands.

## Planned recipes

The patterns slated for this cookbook, drawn from rimsky's primitives and the
published stacks:

- **Build a one-node queue worker** — a Postgres claim-producer feeding a
  single-node template through an executor. Runnable on the `deploy/` stack.
- **Fan out over a group of folders with a partitioned claim** — `fan-out` with
  sub-claims over a filesystem store. Runnable on `deploy/`.
- **Trigger an instance from an external event** — wire one of the bundled
  sensors (cron / http / object-store / webhook) to publish a message that
  creates or advances an instance. Runnable on `deploy/`.
- **Cascade with a claim dependency between nodes** — a claim handed off between
  a producer node and a consumer node. Runnable on `quickstart/`.
- **Backfill / reprocess a fan-out node** — an invalidate-kind message with a
  partition-request override targeting a fan-out node. Builds on the fan-out
  recipe.
- **A bounded loop template** — a self-subscribing node that converges under a
  no-progress cap (see the [surprises page](../humans/03-surprises.md) for why
  loops are first-class and recursion is not). Runnable on `quickstart/`.
- **Hold a subgraph while a claim is staged** — held-claim resolution across a
  subgraph (generalizes the [holding-subgraph example](../agents/examples/holding-subgraph.md)).
- **Modify local files through an executor proxy** — run an executor against
  files on a developer machine via the host-agent proxy. *Note: the published
  `quickstart/` and `deploy/` stacks do not yet wire a host-agent service, so
  this recipe needs a stack change before it can be made runnable.*
