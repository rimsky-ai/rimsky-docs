# Troubleshoot a stuck-stale node

Diagnose a node sitting at `Status: stale` instead of progressing to a
terminal. This is a **diagnosis session**: a symptom, the diagnostic command
sequence, a decision tree against the responses, and which error-catalog
page resolves each leaf.

## Symptom

You created an instance some time ago; the control-api accepts the create
but `rimsky instance status` keeps reporting `stale`:

```text
$ rimsky instance status $INST
id:            1c5b4f9a-...
state:         running
template_hash: sha256-...

Nodes:
  ID       TYPE    STATE  ERROR_CLASS  RETRIES  LAST_HEARTBEAT
  greeter  worker  stale               0
...
```

`stale` means: scheduled, eligible for dispatch, **not running**, **not
parked**, **not terminal**. The scheduler keeps re-considering it; nothing
moves.

## Why a node stays stale

A `stale` node is one whose work is logically eligible to fire but cannot
yet — typically because **something upstream is gating it** (an unfired
wait-set edge, a missing schema, a saturated lock, no claim available),
or **dispatch keeps failing and re-queuing it** (executor unreachable,
attribute validation failure, heartbeat sweep). Stale is *not* failed
and *not* parked; the [node concept](../concepts/node.md) distinguishes
the five states.

The first useful framing is "what is it waiting on?" The
[wait-set diagnostic](../patterns/operational-health.md) is the rimsky-native
answer.

## Command sequence

### Step 1 — Get the frame id for the stale node

The wait-set surface is keyed by frame. Pull the latest node-run row to
find the frame the stale node is currently scheduled in:

```sh
INST=<instance_id>
curl -s "http://localhost:8080/v1/instances/$INST/nodes" | \
  jq -r '.nodes[] | select(.state == "stale") | "\(.id) \(.frame_id // "(no frame)")"'
# Example output:
#   8d2f1c9e-...  3b6e0a44-...
```

If the `frame_id` is `(no frame)`, the node has never been dispatched
into a frame at all — see decision tree leaf D5 below.

### Step 2 — Inspect the wait-set for that frame

The
[`GET /v1/admin/diagnostics/wait-sets`](../reference/rest-api.md) endpoint
**requires** a `frame=<uuid>` query parameter (the v0.9.0 handler returns
HTTP 400 without it). Optional `receiver_run=<uuid>` filters to one
receiver:

```sh
FRAME=3b6e0a44-...
curl -s "http://localhost:8080/v1/admin/diagnostics/wait-sets?frame=$FRAME" | jq
```

Example response when something is gating dispatch:

```json
{
  "wait_set": [
    {
      "frame_id": "3b6e0a44-...",
      "receiver_run_id": "9d1f...-greeter",
      "sender_run_id":   "5a3c...-upstream",
      "topic_kind":      "terminal",
      "subscription_scope": "in",
      "topic_filter":    {"type": "terminal/success"}
    }
  ]
}
```

Each row is a sender-run-to-receiver-run edge: the receiver is waiting
for a `topic_kind`/`topic_filter` event from the sender. An empty
`wait_set` means the frame is **not** held by an unfired subscription.

### Step 3 — Pull recent terminal/transient signals for the stale node

If no wait-set row gates it, dispatch was attempted but did not settle.
The event log carries every `terminal/*` and `transient/*` signal the
node emitted:

The `kind=` query parameter does an **exact-match** on the event row, not a taxonomy-prefix glob (the v0.9.0 `kind=terminal/*` request returns an empty list, not "every terminal" — confirmed against `lib/foundation/persistence/sqlite/events.go`). Fetch the unfiltered node-scoped window and select prefixes client-side with `jq`:

```sh
NODE=8d2f1c9e-...
curl -s "http://localhost:8080/v1/events?instance_id=$INST&node_id=$NODE" | \
  jq '.events[] | select(.kind | startswith("terminal/")) | {at: .occurred_at, kind: .kind, error_class: .payload.error_class}'

curl -s "http://localhost:8080/v1/events?instance_id=$INST&node_id=$NODE&kind=transient/heartbeat_missed" | \
  jq '.events[] | {at: .occurred_at, dispatch_id: .payload.dispatch_id}'
```

The `transient/heartbeat_missed` form keeps `kind=` because it names a single concrete event class — that exact-match works.

If the node has any `transient/heartbeat_missed` events, you are in
leaf D3.

### Step 4 — Confirm executor reachability

If neither wait-set nor heartbeat-missed signals fire, the supervisor
may be unable to dial the executor (a separate infra path). The control
API surfaces this via the event log too, as
`terminal/infra/executor_dial_failed`:

```sh
curl -s "http://localhost:8080/v1/events?instance_id=$INST&node_id=$NODE" | \
  jq '.events[] | select(.kind | startswith("terminal/infra/")) | {at: .occurred_at, kind: .kind, error_class: .payload.error_class}'
```

A row here is leaf D4.

## Decision tree

Read each branch from the responses above:

```
Is the node sitting in a non-empty wait-set?  (Step 2 returned ≥1 row)
├── Yes — the receiver is gated on a sender. WHAT SHAPE?
│   ├── D1. topic_kind=terminal, sender is in-flight ──→ legitimate gating
│   ├── D2. topic_kind=terminal, sender ALREADY succeeded ──→ wait-set leak
│   └── D5. topic_kind=message, no message ever arrived ──→ event source down
└── No — dispatch was attempted. WHAT DID IT EMIT?
    ├── D3. ≥1 transient/heartbeat_missed for this node ──→ heartbeat loss
    ├── D4. ≥1 terminal/infra/executor_dial_failed       ──→ executor unreachable
    ├── D6. ≥1 terminal/error/acquire/unavailable        ──→ claim contention
    └── D7. No relevant events for the node              ──→ scheduler not picking it up
```

### D1 — Legitimate upstream gating

`topic_kind: terminal` with a `sender_run_id` whose run is still in flight
is **expected**: the v0.9.0 upstream-gating change tightened dispatch
eligibility so a stale receiver waits while any subscribed upstream is
in-flight in the same frame (see the v0.9.0 release notes' "Upstream
gating tightens dispatch eligibility"). Resolution: let it run, or inspect
the sender's own diagnostics if it is itself stuck.

Resolution page: this is normal — no error class.

### D2 — Wait-set leak (receiver gated on a settled sender)

If the sender_run row already carries `state: succeeded` but the receiver
is still gated, you have hit the cross-supervisor sub-claim settlement
stall class fixed in v0.9.0. The fix re-stamps the sub-claim holder
inside leaf settlement so the parent's CAS-guarded handle release
succeeds. If you see this on v0.9.0+ it is a real bug — file it.

Workaround: `POST /v1/admin/instances/{instance}/nodes/{node_id}/invalidate`
against the receiver clears the wait-set and re-fires.

Resolution page: open an issue against rimsky-core; the v0.9.0 release
notes name the fix under "Fixes".

### D3 — Heartbeat lost

The supervisor's executor stopped sending `Heartbeat` events. The
scheduler sweeps the running row to `stale` and re-enqueues a recovery
dispatch. Recurring sweeps indicate the executor cannot heartbeat —
process crash, network partition, host overload, or synchronous work that
blocks the heartbeat goroutine.

Resolution page:
[`agents/errors/heartbeat_missed.md`](../agents/errors/heartbeat_missed.md) —
covers the signal payload (`last_heartbeat_at`, `dispatch_id`,
`threshold_ms`), the automatic recovery re-enqueue, and how to fix the
executor or raise `RIMSKY_HEARTBEAT_TIMEOUT_MS`.

### D4 — Executor dial failed

The supervisor could not dial the executor endpoint or open the
`Execute` stream. The infra-error class re-enqueues the dispatch, so the
node stays `stale` between attempts.

Resolution page:
[`agents/errors/executor_dial_failed.md`](../agents/errors/executor_dial_failed.md) —
covers the wire shape, the re-enqueue contract, and the typical causes
(executor down, TLS peer rejection, DNS or network partition).

In v0.9.0 a peer-TLS mismatch is itself a loud failure: `tls: required`
against a plaintext peer surfaces the peer name and the configured mode
at startup or first dial. Check the supervisor log for `tls` lines.

### D5 — No event source ever arrived

A `topic_kind: message` row gating the receiver, with the
`subscription_scope` matching a publisher or operator-sent message, means
the receiver is waiting for an external event that never arrived. The
two-step subscription model (mounting → active) introduced in v0.9.0
means the subscription itself might be `state: mounting`:

```sh
curl -s "http://localhost:8080/v1/instances/$INST" | \
  jq '.subscriptions[] | {kind, publisher_name, state, failure_reason}'
# Expected for an active subscription:
#   {"kind":"cron","publisher_name":"sensor-cron","state":"active","failure_reason":null}
# A "mounting" or "failed" state stranded the wait:
#   {"kind":"cron","publisher_name":"sensor-cron","state":"failed","failure_reason":"dial tcp ...: connection refused"}
```

Resolution path: fix the publisher peer (start the container, repair the
DSN, etc.). The retry-forever reconciler will move `mounting → active`
once the peer is reachable.

### D6 — Claim acquisition unavailable

The node uses a `stores:` entry and the producer is refusing the
`Open` — saturated capacity, conflicting holder, an empty postgres queue
(`pg/claim_unavailable`). Without an `error_types:` entry the default
`give_up` resolution fires `terminal/error/<class>` and the node
**settles failed**, not stuck-stale — so if you see this and the node is
still `stale` rather than `failed`, the dispatch is queued behind a
retry policy and will move next tick.

Resolution page:
[`agents/errors/acquire_unavailable.md`](../agents/errors/acquire_unavailable.md) —
covers the synthetic vs producer-declared class shapes and the
`error_types:` chain.

### D7 — Scheduler not picking the node up

No events at all for the node, no wait-set, no dispatch failures. The
scheduler is not dispatching it. Three productive checks:

1. The instance is not paused (`rimsky instance get $INST` reports
   `paused: false`). A paused instance stops dispatch globally — see
   `route:POST /v1/instances/{idOrKey}/resume`.
2. The supervisor row exists and is heartbeating
   (`curl http://localhost:8080/v1/health | jq '.supervisors'`). A
   supervisor whose `last_heartbeat` is older than
   `RIMSKY_HEARTBEAT_TIMEOUT_MS` (default 15s) is treated as gone.
3. The node's executor reference is resolvable in `rimsky.yml`. If it is
   not, instance creation should have rejected the template — confirm
   the registered template matches what is deployed
   (`rimsky template get sha256-...`).

Resolution pages:
- [`agents/errors/unresolved_executor.md`](../agents/errors/unresolved_executor.md) —
  executor name not in `rimsky.yml`.
- [`agents/errors/template_not_deployed.md`](../agents/errors/template_not_deployed.md) —
  template registered but not deployed.

## Force progress (last resort)

If the diagnosis names the cause and you want to retire the stale node
to clear an instance for re-run, use the admin invalidate to mark it
stale and re-fire:

```sh
curl -s -X POST \
  "http://localhost:8080/v1/admin/instances/$INST/nodes/$NODE/invalidate" \
  | jq
# Expected: {"instance_id":"<uuid>","node_id":"<uuid>","status":"accepted"}.
# The next scheduler tick re-dispatches the node.
```

Per the
[admin diagnostics route table](../reference/rest-api.md), invalidate on
a `running` node returns HTTP 409 (the in-flight work cannot be
preempted); `parked` resumes; `stale` / `fresh` / `failed` re-fire.

## See also

- [Operational health pattern](../patterns/operational-health.md) — the
  steady-state operator playbook for diagnostics endpoints.
- [`concepts/wait-set.md`](../concepts/wait-set.md) — the ledger schema
  behind the diagnostic.
- [`concepts/frame.md`](../concepts/frame.md) — what holding a frame
  means.
- [`agents/errors/README.md`](../agents/errors/README.md) — the full
  error catalog index.
