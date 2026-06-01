# Loop until the work settles

## Problem

You want a node that re-runs itself until it converges, with a safety net
so a wedged loop surfaces instead of spinning forever. The work is
iterative: an agent reasons, acts, observes, and decides whether to go
again; a refinement pass runs until the output stops changing; a poller
checks an external system until a condition holds.

## Rimsky shape

A loop in rimsky is a node that subscribes to *its own* terminal signal —
self-subscription is first-class (loops are first-class; recursion is
not). The canonical self-edge is a subscription back to the node's own
type on `terminal/success`, gated on `when: payload.changed` so the loop
*stops* the moment a run produces no change. Because the loop is just a
[subscription](../concepts/node-subscription.md), there is no special
"loop node" type — the [cascade](../concepts/cascade.md) already does the
right thing.

The `when: payload.changed` gate is the loop's *natural* stop condition:
when a run commits with `changed: false`, the subscription does not match
and the loop ends. Whether the loop converges is therefore the executor's
call — the platform re-fires for as long as the executor keeps reporting
`changed: true`.

A separate, adjacent guard catches a different failure mode: the
**error-driven** retry loop. Every dispatch tracks a
consecutive-retries-without-progress counter that increments on
error-policy *retries* (not on successful re-fires); when it exceeds the
effective cap (`max_retries_without_progress`, default 100 from the
deployment), the runtime forces a `retry_loop_no_progress`
[error](../concepts/error-policy.md). That cap bounds a node that keeps
*erroring and retrying* without progress; it does **not** bound a
`terminal/success` self-edge — a success loop is stopped only by the
`changed: false` gate (or by terminating the instance). Set
`max_retries_without_progress: 0` on a node you intend to retry
indefinitely (a watchdog or external poller).

Two equally valid spellings, an editorial choice not a platform
constraint: `frame: next` opens a fresh [frame](../concepts/frame.md) per
iteration (clean `frame.start` / `frame.end` markers you can observe per
loop); `frame: in` keeps the iteration inside one long-running frame.

Primitives: **self-subscription** (the loop edge), **signal**
(`terminal/success` + the `payload.changed` CEL gate),
**error-policy** (the no-progress cap on the adjacent retry-loop shape).

## Template

Needs a rimsky deployment with the `http-node` executor. Stand rimsky up
from the published images (see the [operator guide](../operator-guide.md)).

Save the template as `loop.yml`:

```yaml
name: convergence-loop
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: refine
    executor: http-node
    # The self-edge: re-fire on every successful run that reported
    # a change. When a run settles with payload.changed == false,
    # the subscription does not match and the loop stops.
    subscribes:
      - { node: refine, type: terminal/success, when: "payload.changed", frame: next }
    # Bounds the *error-driven* retry shape (consecutive error-policy
    # retries with no progress). It does NOT bound this success self-edge;
    # see the prose below.
    max_retries_without_progress: 20
    attributes:
      schema:
        type: object
        properties:
          # stub_probe short-circuits the bundled http-node stub before its
          # transport-config check; a schema `default:` flows into the
          # dispatch bag verbatim (it is never substituted).
          stub_probe:
            type: boolean
            default: true
        additionalProperties: true
```

Register, deploy, instantiate:

```sh
rimsky template register loop.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=01H...
```

The loop's mechanism is the self-edge: each run that commits with
`changed: true` matches the `when: payload.changed` gate and re-fires the
node in a new frame; the first run that settles with `changed: false`
fails the gate, the subscription does not match, and the loop stops. A loop
is one `subscribes:` line, no special node type, and its stop condition is
the `when: payload.changed` gate the executor drives.

## Gotchas

**The stub always reports `changed: true`.** In stub mode
(`RIMSKY_EXECUTOR_STUB_MODE=1`) `http-node` never emits the `changed:
false` that would let the loop converge, so the `when: payload.changed`
self-edge fires on *every* iteration. What you can demonstrate end-to-end
with the stub is the loop *shape*: the node re-fires itself frame after
frame, each iteration a clean `terminal/success`. Watch a few iterations go
by — the `refine` node alternates between `running` (a frame in flight) and
`fresh` (settled, waiting for the next self-fire), and a fresh frame opens
each cycle:

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state}'
# → {"node_type":"refine","state":"running"}   # iteration in flight
# → {"node_type":"refine","state":"fresh"}      # between iterations
```

Because every iteration is a clean `terminal/success` rather than an
error-policy retry, the `retry_loop_no_progress` cap never increments — a
success self-edge is not a retry loop. On the always-`changed` stub this
loop therefore runs indefinitely; stop it by terminating the instance:

```sh
rimsky instance delete <instance_id>
```

**The runaway guard fires on retry loops, not success loops.** The
`retry_loop_no_progress` cap is the adjacent guard for a *different* shape —
the error-driven retry loop. Point a node at an executor that *errors* and
routes that error class to a `retry` action, and the
consecutive-retries-without-progress counter climbs each retry. After
`max_retries_without_progress` (20 here) retries with no
`settling_signal_type` change, the runtime forces a `retry_loop_no_progress`
[error](../concepts/error-policy.md): the node settles `failed` carrying
that error class, and the
`rimsky_terminal_verdicts_total{error_class="retry_loop_no_progress"}`
metric increments — a wedged retry loop surfaces instead of silently
burning budget.

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state, current_error_class}'
# → {"node_type":"refine","state":"failed",
#    "current_error_class":"retry_loop_no_progress"}
```

**Natural convergence needs a real executor.** Convergence is the
executor's call: the loop stops when a run reports `changed: false`. To
see that branch, point `refine` at a real executor that reports
`changed: true` while it has more work to do and flips to `changed:
false` once its work settles. With such an executor the node runs each
iteration, then on the no-change run the `when: payload.changed` self-edge
stops matching and the node settles `fresh`:

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state}'
# → {"node_type":"refine","state":"fresh"}
```

## Without rimsky

By hand you would write the loop construct yourself — a `while` with a
convergence check — plus a runaway guard (max iterations, a wall clock, a
no-progress detector) and the bookkeeping to make each iteration
observable and resumable after a crash. Rimsky makes the iteration a
scheduled, audited, claim-and-lock-aware unit: each turn is its own
dispatch with its own frame markers, the convergence test is one CEL
predicate over the executor's `changed` flag, and the
consecutive-retries-without-progress cap is a platform invariant that
backstops the error-driven retry shape for free.
