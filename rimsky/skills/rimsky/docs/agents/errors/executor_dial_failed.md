---
error: executor_dial_failed
surfaced_to: operator
---

# Executor dial failed

## What it means

The supervisor could not establish (or could not open the `Execute` stream on) the gRPC connection to the executor's configured endpoint. The dispatch never reached the executor. This is an **infra error**, not an application error: the supervisor moves the node `running → stale`, releases its locks, and **re-enqueues the dispatch immediately** — it does not route through the node's `error_types:` policy and does not consume an application retry. It emits a `terminal/infra/executor_dial_failed` signal (one audit-log row; observable to any subscriber matching `terminal/infra/*`).

## When it happens

The executor's `endpoint` in `rimsky.yml` is wrong, the executor process is down or still starting, a TLS handshake failed, or a network partition sits between the supervisor and the executor. The class covers both the connection-pool `GetOrCreate` failure and an error returned from the `Execute` RPC call itself.

## What to do

Because the dispatch re-enqueues immediately and the application retry counter is **not** bumped on an infra round-trip, a persistently-unreachable executor produces a tight re-dispatch loop on this class until the endpoint becomes reachable — there is no give-up backstop for infra errors. Fix the underlying reachability: confirm the executor's `endpoint`, `transport`, and `tls` settings in `rimsky.yml` match where the executor is actually listening, confirm the executor process is up and its gRPC server is bound, and check for a network/TLS problem between supervisor and executor. The signal payload carries the underlying dial error under `details.error`. Once the executor is reachable the next re-enqueue dispatches normally.

## See also

- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`../../concepts/signal.md`](../../concepts/signal.md)
- [`../../concepts/rimsky-yml.md`](../../concepts/rimsky-yml.md)
