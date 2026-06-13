# Fire a node only after all its upstreams settle

## Problem

You have N upstream nodes — say 3 verifier checks — and you want a
downstream node to fire **once**, after **all N** have reached
`terminal/success`. The classic AND-join: "a receiver subscribes to N
upstreams' `terminal/success`; rimsky waits for all N."

## Rimsky shape

In v0.9.0 the plain reading **works**: a multi-subscribed receiver is
held until every in-flight subscribed upstream in the same frame has
settled. The mechanism, traced through
[wait-set](../concepts/wait-set.md):

- **Dispatch predicate (v0.9.0).** A pending run is dispatch-eligible
  iff it has no undrained wait-set rows in its frame **and** no
  subscribed upstream has an in-flight run in the same frame — the
  v0.9.0 upstream-gating tighten propagation-path-independent (release
  notes: "Upstream gating tightens dispatch eligibility"). The second
  half is what makes a parallel-sibling fan-in a real barrier.
- **Once per frame.** A receiver that already has a run row in the
  frame is not re-seeded by later terminals in the same frame
  (self-subscriptions excepted — see
  [convergence-loop](convergence-loop.md)). So a multi-subscribed
  receiver fires exactly once per cascade wave — *and now also strictly
  after* all its upstreams.
- **Settlement walk is one level, insert-then-drain.** When a sender
  settles, the cascade walk visits the direct subscribers matched by
  the emitted [signal](../concepts/signal.md). The receiver's run row
  is created at that moment; the wait-set row inserted for it is keyed
  on the settling sender and cleared in the same transaction. The new
  upstream-gating predicate then keeps the receiver parked until every
  *other* in-flight subscribed upstream in the frame has also resolved.

Two equally valid shapes, then, both deliver "once, after everything":

1. **Subscribe in parallel; let the upstream-gating predicate
   rendezvous.** Each check subscribes to a trigger; the join subscribes
   to all N checks directly. v0.9.0 holds the join until every in-flight
   check resolves. Parallel and minimal. This is the new template
   below.
2. **Serialize the upstreams; subscribe to the chain's terminal.** Run
   the checks as a chain (`check_a → check_b → check_c`) and subscribe
   the join to the **last** node's `terminal/success` only. The last
   terminal IS the "all are done" signal — the rest are its transitive
   ancestors. Deterministic and works on every release; costs the
   parallelism.
3. **Homogeneous units: use [fan-out](../concepts/fan-out.md).** When
   the N units are the same work over N partitions, declare `fan_out:`
   on one node. The parent's run settles only after **all** partition
   children resolve (state aggregation per
   [node-run](../concepts/node-run.md)), and a downstream subscriber to
   the parent's terminal fires once, after that rendezvous. Requires
   the named claim's producer to support split-scope (see
   [fan-out](../concepts/fan-out.md)) — not a plain-YAML shape.

Primitives: **node-subscription** (the ordering edges), **signal**
(each upstream's `terminal/success`),
[**wait-set**](../concepts/wait-set.md) (the per-frame
dispatch-eligibility ledger).

## Template

Needs a rimsky deployment with the `http-node` executor (stub mode,
`RIMSKY_EXECUTOR_STUB_MODE=1` — that is what makes the `stub_probe`
defaults below close every dispatch with a clean success). Stand rimsky
up from the published images (see the
[operator guide](../operator-guide.md)).

The parallel shape: one trigger, three checks each subscribed to the
trigger, one joiner subscribed to **all three** checks. The v0.9.0
upstream-gating predicate is the barrier.

Save as `fan-in.yml`:

```yaml
name: fan-in-join
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: trigger
    executor: http-node
    attributes:
      schema:
        type: object
        properties:
          stub_probe: { type: boolean, default: true }
        additionalProperties: true

  - type: check_a
    executor: http-node
    subscribes:
      - { node: trigger, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          stub_probe: { type: boolean, default: true }
        additionalProperties: true

  - type: check_b
    executor: http-node
    subscribes:
      - { node: trigger, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          stub_probe: { type: boolean, default: true }
        additionalProperties: true

  - type: check_c
    executor: http-node
    subscribes:
      - { node: trigger, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          stub_probe: { type: boolean, default: true }
        additionalProperties: true

  - type: join
    executor: http-node
    # Subscribe to all three checks. v0.9.0's upstream-gating predicate
    # holds the join until every in-flight subscribed upstream in the
    # frame has settled — a real all-N barrier, not a race.
    subscribes:
      - { node: check_a, type: terminal/success }
      - { node: check_b, type: terminal/success }
      - { node: check_c, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          stub_probe: { type: boolean, default: true }
        additionalProperties: true
```

Register, deploy, instantiate:

```sh
rimsky template register fan-in.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=6b1f0c9a-4e2d-4f7b-9a3c-d5e8f1a2b3c4
```

The `deploy` step is required, not ceremonial: instance creation rejects
a template whose state is not `deployed` (409, "instance creation
requires template state 'deployed'") — `register` alone leaves it
undeployed.

`trigger` has no upstream, so it is a root and dispatches once at
instance creation — the first wave runs immediately. `trigger` settles
and seeds all three checks (`check_a`, `check_b`, `check_c`); they
dispatch in parallel. Their terminals each seed `join`'s run row, but
the upstream-gating predicate keeps `join` parked until every in-flight
check has settled. To drive another wave on the live instance,
invalidate the trigger. There is no `rimsky node invalidate` verb —
invalidation is the admin verb and takes the bare *node id* (it calls
`POST /v1/nodes/{id}/invalidate`; the alternative admin HTTP route keys
instance + node:
`POST /v1/admin/instances/{instance}/nodes/{node_id}/invalidate`). Read
the node id off the nodes listing:

```sh
curl -s http://localhost:8080/v1/instances/<instance_id>/nodes \
  | jq '.nodes[] | {id, node_type, state}'
rimsky admin invalidate <trigger-node-id> --reason "new wave"
```

`join`'s dispatch waits behind the still-in-flight checks: even though
its run row is seeded at the first check's terminal, the upstream-gating
predicate keeps it parked until none of the three checks is in-flight.
Every node settles `fresh`, and `join` ran exactly once, strictly after
all three checks:

```sh
curl -s http://localhost:8080/v1/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
# → all five nodes "fresh" once the wave resolves
```

To verify "once per wave" — count the join node's `work_started` events
on the [event log](../concepts/event-log.md) (one is appended per
dispatch; take the join's `id` from the `/nodes` listing above):

```sh
curl -s "http://localhost:8080/v1/events?instance_id=<instance_id>&node_id=<join-node-id>&kind=work_started" \
  | jq '.events | length'
# → 2 after the creation wave plus one invalidate wave — one new
#   work_started per wave
```

## Gotchas

**The upstream-gating predicate landed in v0.9.0.** Pre-v0.9.0 the
parallel-sibling shape was a race: `join`'s run row was created at the
first check's terminal and was immediately dispatch-eligible, so what
`join` saw from the still-in-flight checks was timing-dependent. v0.9.0
ties dispatch eligibility to "no subscribed upstream is in-flight in
the same frame" regardless of how staleness propagated, which makes
this recipe's multi-subscription shape a real barrier. On a v0.8.0 or
earlier deployment, use the serialized variant (chain `check_a →
check_b → check_c`, `join` subscribes to `check_c` only) or fan-out.

**Attribute reads are subscriptions too.** Every
`{{nodes.X.attribute.Y}}` `source:` directive in the join's schema
auto-subscribes the join to that upstream's attribute-changed signal
(see [node-subscription](../concepts/node-subscription.md)). If the
join reads an **early** chain link's attribute, that link's terminal
seeds the join's run row mid-chain — re-introducing the race above and
consuming the join's once-per-frame fire before the chain finishes.
Read only the last link's attributes; have each link carry forward the
values the join needs.

**`hard_dep: true` is not an N-way barrier.** A single
`hard_dep: true` source pulls that one upstream into the receiver's
frame and genuinely holds the receiver until it settles (the verified
shape: receiver subscribes to A's terminal and hard-deps B; the
receiver dispatches after both). Declaring `hard_dep` on **multiple**
upstreams that settle independently in the same frame is unsupported as
a join: the hard-dep pull at each sender's terminal re-seeds any other
hard-dep'd upstream that has already settled in that frame, re-running
it — mutual re-runs instead of a rendezvous. Keep hard-dep to one
upstream per receiver.

**Fan-in over a self-subscribing loop.** A receiver downstream of a
[convergence loop](convergence-loop.md) (`when: payload.changed` on its
own terminal, `frame: next`) re-fires once per loop iteration — each
iteration is its own frame, so the once-per-frame guard resets. Filter
with `when:` if you only want the loop's *converged* signal, or
subscribe to a downstream marker node that runs once after the loop
settles.

## Without rimsky

By hand the AND-join is a counter: each upstream finishes, increments
a shared counter, and the joiner runs when count == N. The counter is
fiddly under partial failure (a retried upstream double-counts; a
crashed worker mid-increment loses the count). Airflow's trigger
rules (`all_success`, `all_done`) and Dagster's automatic join over
parallel ops encode the count plus the persistence; LangGraph's
`add_conditional_edges` is the explicit-edge-logic flavor. In v0.9.0
rimsky's upstream-gating predicate gives the same all-N "wait for
every in-flight subscribed upstream" guarantee without a counter, plus
the once-per-frame dedup and the recovery-path safety: an in-frame
retry or heartbeat-lost re-enqueue of an already-settled upstream
re-parks `join` until that retried run settles too.
