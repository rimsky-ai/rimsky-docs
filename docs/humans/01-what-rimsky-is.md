# 1. What rimsky is

Rimsky is a reactive node-graph orchestration platform for agentic
workloads. You declare work as a graph of typed nodes connected by
subscriptions. When a node's value changes, the **cascade** marks its
dependents stale and the scheduler dispatches them. The platform is
domain-agnostic: it owns control flow, concurrency, and persistence,
and leaves the work itself to out-of-process services that you bring.

A node's executor — anything reachable over the executor gRPC protocol
— runs the work. Node output ripples downstream through the cascade,
which decides which dependents become stale and recompute. Concurrency
across shared state goes through **claims** acquired against named
scopes via the claim-producer protocol. **Templates** are
content-addressed specifications; **instances** bind a template to
runtime parameters. A **frame** is one cascade resolution. **Held
subgraphs** let multiple nodes share an acquired claim and resolve it
atomically when the subgraph completes.

## What it was built for

Rimsky is an agent orchestrator that can also implement
data-processing patterns — not a data orchestrator that happens to
handle agents. The primitives look superficially like a
data-engineering toolkit (assets, partitions, lineage, backfills, typed
attributes), but that surface exists to give agentic work durable
handles, not because data transformation is the headline.

It was designed against five patterns. If your problem looks like one
of them, keep reading:

- **Watching external state and reacting.** [Sensors](../concepts/sensor.md)
  observe external systems — S3 prefixes, HTTP endpoints, cron
  schedules, inbound webhooks — and emit messages into the graph. The
  reactive logic lives in the graph itself, not in code you write
  around a workflow engine. You declare the subscription; the cascade
  does the routing.
- **Stateful agentic workloads at platform scale.** An LLM agent is an
  [executor](../concepts/executor.md): a service that takes inputs and
  produces outputs and named events along the way. Rimsky operates
  above the single-agent layer — it coordinates many agents and
  templates against shared infrastructure, rather than orchestrating
  one agent's tool calls.
- **Subgraphs that succeed or fail atomically.** [Held
  claims](../concepts/claim-co-holdership.md) combined with held
  subgraphs let a chain of agents and deterministic nodes do N steps
  and either all commit or all roll back. The
  [atomic-staging](../concepts/atomic-staging.md) primitive makes
  all-or-nothing the default.
- **Coordinating across shared state.** [Claim
  producers](../concepts/claim-producer.md) expose a uniform
  acquisition interface against arbitrary backing systems — a
  filesystem, a Postgres table, a vector store, a custom service. The
  platform compares scope bytes through the producer's conflict matrix;
  it does not know what "row 42" means.
- **Data operations as a service to agentic work.** When agents
  produce, transform, or materialize data, rimsky's typed-attribute
  system, [partitions](../concepts/fan-out.md), and
  [lineage](../concepts/lineage.md) support that work. They are plumbing
  for the agent's job, not the headline.

The thread across all five: rimsky was built when an agentic workflow
needed platform-grade orchestration and the available platforms were
either too task-shaped (single-DAG schedulers, no shared-state
coordination), too data-shaped (transformation-first platforms with
opinions about how data moves), or too agent-shaped (single-agent
frameworks with no coordination layer) to fit.

## How it runs

Three runtime processes — **scheduler**, **supervisor**, and
**control-api** — communicate only through Postgres. The
[control-api](../concepts/control-api.md) hosts both the operator
HTTP+JSON surface and a coextensive MCP skin, so an LLM-driven operator
can drive the platform on the same verbs a human would.

Rimsky is pre-v1. Wire protocols, YAML config shapes, and persistence
schemas may change between versions. The safety properties —
deterministic-sorted-order multi-lock acquisition, verify-before-run,
claimant-guarded release, unified terminal-decision, auto-terminal
aggregate-outcome — are stable.

For a side-by-side read against Airflow, Dagster, Prefect, and Temporal,
see [the ecosystem comparison](../comparison.md). For what rimsky
deliberately is *not*, see [the surprises](03-surprises.md) and
[the comparison's scope boundaries](../comparison.md).

Next: [the mental model and vocabulary](02-mental-model.md).
