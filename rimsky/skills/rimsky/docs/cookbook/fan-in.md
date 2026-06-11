# Fire a node only after all its upstreams settle

## Problem

You have N upstream nodes — say 3 verifier checks — and you want a
downstream node to fire **once**, after **all N** have reached
`terminal/success`. The classic AND-join. The plain reading is "a
receiver subscribes to N upstreams' `terminal/success`; rimsky waits
for all N."

**Status note, up front: that plain reading is wrong in v0.8.0.**
Subscribing one node to N parallel siblings does **not** hold its
dispatch until all N settle. It guarantees the receiver fires at most
**once per [frame](../concepts/frame.md)**, and makes it
dispatch-eligible as soon as the **first** subscribed upstream settles
— whether the other N−1 have settled by then is a scheduling race.
There is no template-declared N-input barrier. This recipe gives the
two shapes that DO deliver "once, after everything," and spells out
what the multi-subscription topology gives instead.

## Rimsky shape

What the [wait-set](../concepts/wait-set.md) actually gates, traced to
the v0.8.0 runtime:

- **Dispatch predicate.** A pending run is dispatch-eligible iff it has
  **no undrained wait-set rows** in its frame (the supervisor's
  candidate-selection query; see [wait-set](../concepts/wait-set.md)).
- **Settlement walk is one level, insert-then-drain.** When a sender
  settles, the cascade walk visits only the **direct** subscribers
  matched by the emitted [signal](../concepts/signal.md) — no recursion
  into their subscribers. The receiver's run row is created at that
  moment, and the wait-set row inserted for it is keyed on the settling
  sender — so the **same transaction's** settled-state drain clears it.
  Net effect: a subscription edge orders the receiver after **that one
  sender's** settlement; it does not accumulate a gate across the
  sender's still-running siblings.
- **Once per frame.** A receiver that already has a run row in the
  frame is not re-seeded by later terminals in the same frame
  (self-subscriptions excepted — see
  [convergence-loop](convergence-loop.md)). So a multi-subscribed
  receiver fires exactly once per cascade wave — just not necessarily
  *after* all its upstreams.
- **Undrained rows come from recovery paths, not templates.** Wait-set
  rows that actually hold a receiver back are inserted only when a
  sender transitions back into in-flight **while the frame runs**: an
  error-policy retry, a heartbeat-lost re-enqueue, a parked wake, or an
  in-frame re-invalidation of an in-flight sender. That is a safety
  property (a receiver won't race a sender that is being re-run), not a
  construction primitive you can declare.

Two shapes give a real all-N "fires once, after everything":

1. **Serialize the upstreams; subscribe to the chain's terminal.** Run
   the checks as a chain (`check_a → check_b → check_c`) and subscribe
   the join to the **last** node's `terminal/success` only. The last
   terminal IS the "all are done" signal — the rest are its transitive
   ancestors, and each link's dispatch is correctly ordered after its
   predecessor by the settlement walk. Deterministic; costs the
   parallelism. This is the template below.
2. **Homogeneous units: use [fan-out](../concepts/fan-out.md).** When
   the N units are the same work over N partitions, declare `fan_out:`
   on one node. The parent's run settles only after **all** partition
   children resolve (state aggregation per
   [node-run](../concepts/node-run.md)), and a downstream subscriber to
   the parent's terminal fires once, after that rendezvous. This is the
   only parallel all-N barrier in v0.8.0. It requires the named claim's
   producer to support split-scope (see
   [fan-out](../concepts/fan-out.md)) — it is not a plain-YAML shape.

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

The serialized shape: one trigger, the three checks as a chain, one
joiner subscribed to the **last** check only.

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
      - { node: check_a, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          stub_probe: { type: boolean, default: true }
        additionalProperties: true

  - type: check_c
    executor: http-node
    subscribes:
      - { node: check_b, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          stub_probe: { type: boolean, default: true }
        additionalProperties: true

  - type: join
    executor: http-node
    # Subscribe to the LAST link only. check_c's terminal/success
    # already implies check_a and check_b settled (they are its
    # transitive ancestors in the cascade). Subscribing join to all
    # three would NOT add a barrier — see Gotchas.
    subscribes:
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
instance creation — the first wave runs immediately, in order:
`trigger → check_a → check_b → check_c → join`. To drive another wave
on the live instance, invalidate the trigger. There is no
`rimsky node invalidate` verb — invalidation is the admin verb and
takes the bare *node id* (it calls `POST /v1/nodes/{id}/invalidate`;
the alternative admin HTTP route keys instance + node:
`POST /v1/admin/instances/{instance}/nodes/{node_id}/invalidate`). Read
the node id off the nodes listing:

```sh
curl -s http://localhost:8080/v1/instances/<instance_id>/nodes \
  | jq '.nodes[] | {id, node_type, state}'
rimsky admin invalidate <trigger-node-id> --reason "new wave"
```

Each settlement walks the next link: `trigger`'s terminal seeds
`check_a`; `check_a`'s seeds `check_b`; and so on. `join`'s run row
does not exist until `check_c` settles, so it cannot dispatch earlier —
every node settles `fresh`, and `join` ran once, strictly after all
three checks:

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

**Multi-subscribing the join is not a barrier.** This holds for both
tempting topologies:

```yaml
# Parallel siblings (check_a/b/c each subscribe to trigger):
- type: join
  subscribes:
    - { node: check_a, type: terminal/success }
    - { node: check_b, type: terminal/success }
    - { node: check_c, type: terminal/success }
```

One trigger wave invalidates all three checks together, and they
dispatch in parallel — but `join`'s run row is created at the **first**
check's terminal, with its only wait-set row drained in that same
transaction. `join` is dispatch-eligible from that moment; whether the
supervisor claims it before or after the other checks settle is a race,
so what `join` observes from `check_b`/`check_c` is timing-dependent.
The once-per-frame guard then suppresses re-seeding at the later
checks' terminals: `join` fires exactly once per wave, and it does
**not** re-fire when the stragglers settle. Same story when the three
checks form a serial chain and `join` subscribes to all three links:
`join` becomes eligible as soon as `check_a` settles, fires once
(racing `check_b`/`check_c`), and never re-fires at their terminals.
The fix in both cases is structural: serialize and subscribe to the
last link only (the template above), or use
[fan-out](../concepts/fan-out.md) when the units are homogeneous.

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
`add_conditional_edges` is the explicit-edge-logic flavor. Rimsky has
no per-edge equivalent of `all_success` today: the all-N condition is
either structural (the chain's last terminal implies the rest) or a
counted rendezvous rimsky manages for you (fan-out's parent settles
only after all partition children resolve). The wait-set supplies the
persistence and the once-per-wave dedup; it does not supply a
declared-N barrier.
