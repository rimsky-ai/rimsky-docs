# Fire a node only after all its upstreams settle

## Problem

You have N upstream nodes — say 9 verifier checks running in parallel —
and you want a downstream node to fire **once**, after **all N** have
reached `terminal/success`. The classic AND-join. The plain reading is
"a receiver subscribes to N upstreams' `terminal/success`; rimsky waits
for all N." The [wait-set](../concepts/wait-set.md) is built to support
exactly this, but only under one topology.

## Rimsky shape

A node fans in by subscribing to N upstream `terminal/success` signals.
The dispatch predicate is **"stale AND no undrained wait-set rows in the
current [frame](../concepts/frame.md)."** When an upstream transitions
out of a settled state, a row is inserted for every subscriber; when the
upstream resolves (`fresh` / `failed` / `parked`), the row drains. A
receiver becomes dispatch-eligible only when every row in its wait-set
has been drained.

The mechanism is **dynamic, not static.** Rimsky doesn't carry a
pre-declared input count for a `subscribes:` list — there is **no
static N-input barrier** today. It only knows about inputs that are
*currently* stale in the *current* frame. So the AND-join fires when
all the rows the receiver *happens to have* are drained — which
behaves like an all-N barrier only when all N upstreams are stale
*simultaneously* in the same frame.

Two topologies make that hold:

1. **Parallel fan-out then fan-in.** One trigger invalidates all N
   upstreams in the same cascade wave. All N transition stale together;
   the receiver accumulates N wait-set rows; one fire when all N drain.
2. **Subscribe to the chain's terminal only.** If the upstreams are a
   serial chain (`A → B → C → ...`), the last node's terminal IS the
   "all are done" signal — the rest are transitive ancestors.

The topology that **does not work** is **subscribing directly to
multiple nodes of a serial chain**: each goes stale one at a time, the
receiver sees only the row that's currently in flight, and fires once
per chain link. See Gotchas.

Primitives: **node-subscription** (the fan-in edges), **signal** (each
upstream's `terminal/success`), [**wait-set**](../concepts/wait-set.md)
(the dynamic per-frame dispatch-eligibility ledger).

## Template

Needs a rimsky deployment with the `http-node` executor. Stand rimsky
up from the published images (see the
[operator guide](../operator-guide.md)).

The parallel fan-in shape: one trigger, N sibling workers, one joiner
subscribed to all N.

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
    # All three checks go stale in one cascade wave (they all subscribe
    # to `trigger`). The join's wait-set accumulates three rows; one
    # fire when all three drain.
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

Register, deploy, instantiate, invalidate the trigger:

```sh
rimsky template register fan-in.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=01H...
rimsky node invalidate <instance_id> trigger
```

The cascade walks `trigger`'s `terminal/success` → inserts wait-set
rows for `check_a`, `check_b`, `check_c`. All three dispatch (claim
availability permitting). As each settles, the corresponding row for
`join` drains. `join` becomes dispatch-eligible only after all three
drain.

## Gotchas

**The serial-chain anti-pattern.** A 9-way fan-in *looks* like a
barrier but isn't, if the upstreams form a serial chain. Take a
verifier chain where each check subscribes to the prior one:

```yaml
- type: check_2
  subscribes: [{ node: check_1, type: terminal/success }]
- type: check_3
  subscribes: [{ node: check_2, type: terminal/success }]
- type: gate
  subscribes:
    - { node: check_1, type: terminal/success }
    - { node: check_2, type: terminal/success }
    - { node: check_3, type: terminal/success }
```

A single `invalidate(check_1)` cascades the chain in one frame — but
each check goes stale one at a time, not all at once. `gate` fires
when `check_1` settles (the only row currently in its wait-set);
fires again when `check_2` settles; fires again when `check_3`
settles. **`gate` fires three times, not once.** The fix is to
subscribe `gate` to `check_3` only — `check_3`'s terminal already
implies `check_1` and `check_2` succeeded (they're transitive
ancestors of `check_3` in the cascade).

**Fan-in over a self-subscribing loop.** A receiver that fans in N
upstreams and one of those upstreams is a
[convergence loop](convergence-loop.md) (`when: payload.changed` on
its own terminal) re-fires on *every* loop iteration. Filter with
`when:` if you only want the loop's *converged* signal, or subscribe
to a downstream marker node that runs once after the loop settles.

## Without rimsky

By hand the AND-join is a counter: each upstream finishes, increments
a shared counter, and the joiner runs when count == N. The counter is
fiddly under partial failure (a retried upstream double-counts; a
crashed worker mid-increment loses the count). Airflow's trigger
rules (`all_success`, `all_done`) and Dagster's automatic join over
parallel ops encode the count plus the persistence; LangGraph's
`add_conditional_edges` is the explicit-edge-logic flavor. Rimsky's
wait-set is the same persistence-plus-bookkeeping, but the
all-N-arrived condition is topological — derived from "everyone is
stale in this frame" — rather than configurable per-edge.
