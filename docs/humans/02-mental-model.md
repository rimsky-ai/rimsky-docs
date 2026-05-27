# 2. Mental model and vocabulary

Rimsky has a small set of load-bearing nouns. Learn these and the rest
of the system reads cleanly. Each links to its full definition in the
[concept catalog](../glossary.md).

## The graph and its pieces

- **[Node](../concepts/node.md).** One declarative unit of work in a
  template's graph. A node either declares an `executor:` (out-of-process
  work) or `delegate:` (a sub-graph it dispatches as its execution unit).
  A node with neither is a *native node*: the cascade synthesizes its
  completion once its claims are acquired, carrying downstream whatever
  the upstream nodes wrote.
- **[Subscription](../concepts/node-subscription.md).** How a node says
  what it reacts to. Each entry declares a `type:` — a canonical signal
  type-path under `terminal/*`, `transient/*`, `attribute/*`, `event/*`,
  or `message/*` — and an optional CEL `when:` predicate over the signal
  payload. **The subscriber's match is the gate**: senders emit signals,
  receivers decide whether to fire.
- **[Attribute](../concepts/attribute.md).** The typed inputs, outputs,
  and configuration of a node, declared by JSON Schema in the template.
  `blob` and `table` are the blessed typed attributes today (geo is on
  the way).
- **[Template](../concepts/template.md) and
  [instance](../concepts/instance.md).** A template is a
  content-addressed specification of a graph — keyed by a hash over its
  canonicalized bytes, so two equal templates compare equal and none can
  be silently mutated. An instance binds a template to runtime
  parameters and carries live execution state. A
  [tag](../concepts/tag.md) is a movable string alias pointing at a
  template hash, for when a stable name needs to track the current
  version.

## How work moves

- **[Cascade](../concepts/cascade.md).** The engine that turns one
  node-state transition into the set of downstream transitions. It is
  the killer primitive: it makes a graph reactive without making the
  executor responsible for routing. Reactivity to external change
  (sensors emitting messages) is the same machinery as reactivity to
  internal change.
- **[Frame](../concepts/frame.md).** One cascade resolution. A frame
  begins when a node receives an [invalidate](../concepts/invalidate.md)
  or when pending boundary-crossing messages get delivered; it ends when
  no run for the instance remains stale or running. At most one frame
  runs per instance at a time.
- **Node state.** A node-run is in one of five states: `fresh`,
  `stale`, `running`, `failed`, or `parked`. State lives on the
  [node-run](../concepts/node-run.md) — the per-node execution record
  within a frame.
- **[Parked](../concepts/parked-state.md).** The state a node enters
  when its executor emits a park outcome — waiting for a callback, a
  snooze, or human review. Parking is how long-running and
  human-in-the-loop work waits without holding a dispatch slot.

## Concurrency and shared state

- **[Claim](../concepts/claim.md).** A node's request to access a
  producer-managed resource, declared as a scope and resolved at runtime
  by the producer. The platform serializes conflicting acquisitions
  through the producer's conflict matrix.
- **[Claim producer](../concepts/claim-producer.md).** An out-of-process
  gRPC service implementing four verbs (open / commit / abandon /
  release) plus a capabilities handshake. It owns what a scope *means*;
  rimsky only compares scope bytes.
- **[Named lock](../concepts/named-lock.md).** A producer-independent
  capacity counter declared in operator config (`mutex` or `counting`).
  This is deployment-level capacity gating — "at most N runs of this
  template concurrently."
- **[Held subgraph](../concepts/claim-co-holdership.md).** Multiple
  nodes share one acquired claim via a `holds:` directive. The claim
  resolves once at the end of the holding subgraph: aggregate success
  commits, any failure abandons. This is the
  [atomic-staging](../concepts/atomic-staging.md) machinery — stage,
  then promote-or-discard, as a first-class shape.
- **[Fan-out](../concepts/fan-out.md).** A node partitions a held claim
  into sub-claims at runtime via the producer's split-scope verb. Each
  sub-claim gets its own run, tracked through a parent/child
  [run tree](../concepts/run-scope.md).

## Services and durable handles

- **[Service protocols](../concepts/service.md).** Out-of-process gRPC
  services implement one or more rimsky protocols: claim producer,
  [executor](../concepts/executor.md),
  [lifecycle subscriber](../concepts/lifecycle-subscriber.md),
  [publisher](../concepts/publisher.md) (of which
  [sensors](../concepts/sensor.md) are one class), and the optional
  [validation](../concepts/validation.md) mix-in. The executor's scope
  is broader than the name suggests: anything that takes inputs and
  produces outputs — an agent, a CI pipeline, a webhook dispatcher, a
  Lambda, a Python transformation — can be an executor.
- **[Asset](../concepts/asset.md) and
  [lineage](../concepts/lineage.md).** Durable-lifetime claims surface
  as assets — addressable resources with stable identity and provenance.
  Two [lineage record](../concepts/lineage-record.md) kinds (`leaf_run`
  for computational provenance, `claim_terminal` for data-promotion
  provenance) feed the control-api's lineage endpoints. This is how
  agentic work gets durable handles and audit trails.
- **[Control-api](../concepts/control-api.md) and
  [API keys](../concepts/api-key.md).** Every operator action is also
  an MCP tool at `POST /mcp`. Auth is per-key bearer tokens with JSONB
  permission grants, a verb-noun action grammar with wildcards, per-handler
  dry-run, and structured audit. The bootstrap path is
  implicit-anonymous-admin until the first real key is minted.

## A note on naming

There is no separate "Go SDK." The `protocols` module is the single
public Go module — the wire contract plus optional helper packages
(implementer scaffolding, the action vocabulary, the conformance
library). See [the Go packages reference](../protocols/go-packages.md).

Next: [the surprises](03-surprises.md).
