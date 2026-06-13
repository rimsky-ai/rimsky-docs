---
error: transient/heartbeat_missed
surfaced_to: operator
---

# Heartbeat missed (`transient/heartbeat_missed`)

## What it means

The scheduler's stale-heartbeat sweep found a `running` node whose `last_heartbeat` is older than the heartbeat timeout, appended a `transient/heartbeat_missed` signal to the event log (its own transaction — an append failure is only logged and does not block recovery), and then, in one transaction, transitioned the node `running` → `stale` (transition reason `heartbeat_lost`), retired the zombie dispatch row, and re-enqueued a recovery dispatch. <!-- @source: lib/runtime/conductor.go::SweepStaleHeartbeats -->

The signal payload carries `last_heartbeat_at`, `dispatch_id` (the dispatch whose heartbeats went silent), and `threshold_ms` (the timeout that fired). Read it from the event log:

```sh
curl "http://localhost:8080/v1/events?instance_id=<instance_id>&kind=transient/heartbeat_missed"
```

What this is **not**:

- Not an `error_class` and not routed through the node's `error_types:` policy chain — it is an audit-only signal plus an automatic recovery re-enqueue. There is nothing to declare in a template for it.
- Not the fixed-string `heartbeat_lost` audit row — that event was retired alongside the signal-taxonomy decoupling and is gone. (The `heartbeat_lost` `OperationalKind` still exists in `events.proto` but nothing emits it; the string survives only as the `running → stale` state-transition reason.)
- Not the orphaned-claim sweep — claim-ownership loss at the supervisor level (cutoff 5 × the supervisor `heartbeat_interval`) is a separate mechanism; see [`orphaned_claim_lost_race.md`](orphaned_claim_lost_race.md).

## When it happens

The executor stopped sending `Heartbeat` events on the `Execute` stream for longer than the timeout: process crash, network partition, host overload, or — most commonly — an executor that is silent on heartbeats while doing slow synchronous work. The supervisor stamps the node's `last_heartbeat` on every `Heartbeat` event it receives; <!-- @source: lib/runtime/runner_dispatch.go --> the scheduler sweeps on its tick with the cutoff from `RIMSKY_HEARTBEAT_TIMEOUT_MS` (default `15000`). <!-- @source: cmd/rimsky-scheduler/main.go -->

## What to do

Recovery is automatic: the node is re-enqueued in the same transaction, and the recovery dispatch carries the retired dispatch's id on `ExecuteRequest.prior_dispatch_id` (disposition `heartbeat_stale`), so a recovery-aware executor can resume rather than redo the work.

To stop it recurring:

- **Executors:** send `Heartbeat` events on a regular cadence, independent of the work loop. If the work is genuinely synchronous and blocking (a long C call, a slow LLM call), interpose a goroutine / async task that pumps heartbeats while the work runs.
- **Operators:** if legitimate long-running dispatches are being swept, raise `RIMSKY_HEARTBEAT_TIMEOUT_MS` on the scheduler — or fix the executor to heartbeat, which is the better repair.

## See also

- [`../../concepts/signal.md`](../../concepts/signal.md) — the signal taxonomy (`transient/*` vs `terminal/*`).
- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`orphaned_claim_lost_race.md`](orphaned_claim_lost_race.md) — the supervisor-level claim-ownership sweep.
