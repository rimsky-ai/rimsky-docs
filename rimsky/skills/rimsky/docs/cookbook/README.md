# Cookbook

Each recipe is a problem you recognize, the rimsky shape that solves it, a
copyable template you run against a rimsky deployment, and a "without
rimsky" baseline so the trade is legible. Every recipe has the same slots:

1. **Problem** — what you're trying to build, in plain terms.
2. **Rimsky shape** — which primitives it uses and why.
3. **Template** — a copyable template plus the register / deploy /
   instantiate walkthrough.
4. **Gotchas** — the sharp edges of the shape.
5. **Without rimsky** — what you'd hand-roll otherwise, for contrast.

This is a *spanning* set, not an exhaustive one: each recipe teaches a
distinct lesson about what the primitives can do, and near-duplicates are
folded into one canonical representative. The patterns combine — a queue
worker that loops is the queue recipe plus the loop recipe; a reactive
graph behind a capacity limit is two recipes composed.

## The recipes

All recipes run against a rimsky deployment — stand one up from the published images (see the [operator guide](../operator-guide.md)).

The `rimsky` CLI has **no built-in default endpoint**. Every verb resolves
the control-API endpoint as `--endpoint` flag > `RIMSKY_CONTROL_API` env >
the active context (`RIMSKY_CONTEXT`, then the config's `current_context`
set by `rimsky ctx use <name>`); with none set, the CLI errors
`no endpoint configured`. The recipes assume you have set
`RIMSKY_CONTROL_API` (or an active context) pointing at the deployment's
control API — `http://localhost:8080` in the reference deploy.

- **[A single-node queue worker](queue-worker.md)** — a claim producer as
  a work queue (the postgres `@review-queue` pick policy) drained by a
  self-subscribing node.
- **[Recompute dependents when something upstream changes](reactive-recompute.md)**
  — subscriber-driven cascade: a downstream node auto-subscribes to an
  upstream attribute and recomputes only the affected nodes on change.
- **[Fire a node only after all its upstreams settle](fan-in.md)** —
  serialize the upstreams and subscribe to the last link's terminal
  (or use fan-out for homogeneous units); subscribing one node to N
  parallel siblings is **not** a barrier — it fires once per frame, as
  soon as the first sibling settles.
- **[Cap concurrency with a counting semaphore](capacity-limit.md)** — a
  named lock as a deployment-wide capacity counter (`model-budget`,
  limit 50).
- **[Loop until the work settles](convergence-loop.md)** — a
  self-subscribing node that re-fires under a `payload.changed` gate and is
  bounded by the no-progress retry cap. With a stub-mode executor the gate
  fires every iteration, so the recipe demonstrates the loop shape and the
  runaway cap as the safety net.
- **[Drive a node from an external event](event-driven-node.md)** — an
  inbound message (operator- or publisher/sensor-emitted) delivered at a
  frame boundary and matched to a node by subscription.
- **[Hand a claim from one node to the next](claim-handoff.md)** — a claim
  co-held (`holds:`) across a chain of nodes so the whole chain is one
  all-or-nothing transaction, committed or abandoned once at the end.
- **[Call a reusable sub-graph like a function](sub-graph.md)** — a named
  sub-graph invoked via `delegate:`; the entry node absorbs into the calling
  node and the exit node's writeback carries back as the result.
- **[Run an executor on your dev machine](local-binary-executor.md)** — a
  late-bound executor name dispatched through the
  [host-agent proxy](../concepts/host-agent-proxy.md) to a binary running
  on your laptop, bound per-instance with no static deployment config.

## Instances are durable by default

Every recipe here creates an [instance](../concepts/instance.md), and the
same lifecycle rule applies to all of them: **an instance is durable by
default and never terminates on its own.** There is no auto-terminate on
drain — a loop that converges and an event-driven node that has handled
its last event settle `fresh` and keep living; a queue worker that runs
out of items settles **`failed`**: the empty queue surfaces as the
postgres store's producer-declared error class
(`pg/claim_unavailable`), which cannot be declared under an `http-node`
worker's `error_types:` (registration range-checks the keys against the
executor's declared classes), so the fail-fast `give_up` default
applies — see the [queue-worker recipe](queue-worker.md)'s gotchas.
Either way, nothing the *graph* does ends the instance.

The single self-termination path is the create-time opt-in
`terminate_after_run: true` flag. It terminates the instance after its
**next** frame ends — strict "run at most once more", never while a
node-run is parked — so it expresses the ephemeral run-once shape, not
"finish all queued work then stop". Three ways to set it:
`rimsky run --terminate-after-run` (and `rimsky run --no-keep`, which
implies it), or the create request body. The CLI `rimsky instance create`
has no flag for it, so when creating through that verb set it by POSTing
the create request directly:

```sh
curl -s -X POST http://localhost:8080/v1/instances \
  -H 'Content-Type: application/json' \
  -d '{"template":"sha256-...","terminate_after_run":true}'
```

To tear down a durable instance manually, force-terminate then delete —
two steps, because `delete` refuses a non-terminal instance:

```sh
rimsky instance kill <instance_id> --force   # marks it terminal (abandons any in-flight run)
rimsky instance delete <instance_id>          # frees the row + instance key
```

## Related surfaces

Two write-ups live in [`docs/patterns/`](../patterns/) rather than here —
they are operator/architecture patterns, not single-problem recipes:

- [Domain stores](../patterns/domain-stores.md) — holding project-specific
  state in an MCP server an agent executor consumes.
- [Operational health](../patterns/operational-health.md) — observing and
  maintaining a running deployment (lifecycle subscribers, watchdog
  graphs, diagnostics, retry-loop detection).

## Patterns that need a capability the bundled services lack

Three patterns the primitives support are **not** runnable on the bundled
producers/services as they stand, so they are not written up as recipes:

- **Fan out over a partitioned claim** (and the **backfill** that targets a
  fan-out node) requires a claim producer that advertises
  `supports_split_scope`. Neither bundled store does — the filesystem
  (`content`) and postgres (`topics-ring`) producers advertise only their
  write semantics (their `Capabilities` set `SupportsSplitScope` false) —
  so a `fan_out:` node is rejected at template registration. This recipe
  needs a split-scope-capable producer; for copyable Go starting points
  see the in-corpus
  [`atomic-staging-fs-producer`](../examples/atomic-staging-fs-producer/)
  example (single-scope) and the
  [`data-processing`](../examples/data-processing/) example (the
  candidate lifecycle a partitioning producer also needs).
- **A durable claim that outlives its holding subgraph** (the
  [asset](../concepts/asset.md) shape — `lifetime: durable` on a `stores:`
  entry) requires the producer to advertise the `data_processing` mix-in
  protocol. Neither bundled store advertises it, so the canonicalizer
  rejects a `durable` claim against them. This recipe needs a
  DataProcessing-capable producer; the in-corpus
  [`data-processing`](../examples/data-processing/) example is the
  copyable reference for the candidate-lifecycle surface.
- **Park then resume** (the
  [parked-state](../concepts/parked-state.md) hold — a node parks on
  `terminal/park/snooze` or `terminal/park/await_callback`, then resumes
  with a resume-context) needs an executor that *emits* a park outcome. The
  `http-node` stub the other recipes run on never parks — it closes every
  dispatch with success — and the only bundled park emitter is the
  `claude-agent` executor, whose park paths (rate-limit auto-park; the
  `report_park` MCP tool) fire only inside a *real* agent run against a live
  model, not deterministically from a template. So while the park/resume
  shape is fully supported by the platform, it has no stub-driven,
  copy-and-run recipe: demonstrating it requires a live `claude-agent`
  (real API key) or a custom executor that parks on demand.
