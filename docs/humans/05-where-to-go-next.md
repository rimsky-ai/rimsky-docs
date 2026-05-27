# 5. Where to go next

You have the shape of rimsky, the vocabulary, the surprises, and one
pattern walked end to end. Where you go from here depends on what you
are trying to do.

## If you are evaluating

- **[Ecosystem comparison](../comparison.md)** — an honest read on where
  rimsky overlaps with Airflow, Dagster, Prefect, and Temporal, and
  where it diverges. Start here if you are deciding whether rimsky fits
  a workload you already understand.
- **[Roadmap](../roadmap.md)** — what is actively being designed, what
  is on the horizon, and what rimsky has deliberately chosen not to
  become. Rimsky is pre-v1; the roadmap is a direction statement, not a
  contract.
- **Project [README](https://github.com/fallguyconsulting/rimsky/blob/main/README.md)**
  — the canonical framing this tour is built from.

## If you are building

Point your coding agent at the agent-oriented surface and let it walk
you through depth:

- **[`../agents/llms.txt`](../agents/llms.txt)** — the canonical
  agent-oriented manifest. It indexes every public surface: concepts,
  protocols, examples, and the error catalog. This is the handoff point
  from human-shaped docs to agent-shaped docs.
- **[Runnable examples](../agents/examples/README.md)** — complete,
  copy-pasteable, no-ellipsis templates you can run against the bundled
  docker-compose stack. The [holding-subgraph
  example](../agents/examples/holding-subgraph.md) is the runnable
  companion to [the worked example](04-worked-example.md).
- **[Error catalog](../agents/errors/README.md)** — the error classes
  rimsky emits and what they mean.

## If you are implementing a service

Rimsky brings the control plane; you bring the work. Services speak
gRPC and implement one or more rimsky protocols:

- **[Protocol guides](../protocols/README.md)** — how to implement each
  protocol, with starter shapes:
  [Executor](../protocols/executor.md),
  [ClaimProducer](../protocols/claim-producer.md),
  [LifecycleSubscriber](../protocols/lifecycle-subscriber.md),
  [Publisher](../protocols/publisher.md).
- **[Go packages](../protocols/go-packages.md)** — the `protocols`
  module is the single public Go module (the wire contract plus optional
  helper packages). There is no separate Go SDK.

## If you are operating a deployment

- **[Operator guide](../operator-guide.md)** — the operator-visible
  knobs that span multiple concepts: config root, blob backends,
  metrics, diagnostic endpoints, conformance binaries, pre-v1 caveats.

## When you want the precise definition

- **[Concept catalog / glossary](../glossary.md)** — every noun rimsky
  traffics in, one entry each. The per-concept pages under
  [`../concepts/`](../concepts/) carry the full definition, purpose,
  boundaries, and invariants for each.

## The cookbook

For patterns one level more general than a single tutorial — "build a
one-node queue worker," "fan out over a group of folders with a
partitioned claim," "modify local files through an executor proxy" — see
the **[cookbook](../cookbook/README.md)**. Each recipe states the
problem, the rimsky shape, a runnable walkthrough on the published
stack, and a "without rimsky" baseline.

---

That is the tour. Re-read [the surprises](03-surprises.md) before you
commit a design — the loop-yes / recursion-no split and the
subscriber-decides model are the two places a mental model carried over
from another orchestrator will most often steer you wrong.
