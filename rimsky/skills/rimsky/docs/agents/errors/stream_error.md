---
error: stream_error
surfaced_to: operator
---

# Stream error

## What it means

The executor accepted the `Execute` RPC and the stream went live, but a `Recv()` on the executor's gRPC stream returned an error before a terminal `StreamClose` arrived. The dispatch was interrupted mid-flight. This is an **infra error**, not an application error: the supervisor moves the node `running → stale`, releases its locks, and **re-enqueues the dispatch immediately**, bypassing the node's `error_types:` policy. It emits a `terminal/infra/stream_error` signal (one audit-log row; observable to any `terminal/infra/*` subscriber).

Distinct from `stream_closed_without_terminal`: that class is a *clean* EOF with no terminal verdict; `stream_error` is a *non-EOF* transport error on the stream (executor crash mid-stream, connection reset, deadline exceeded).

## When it happens

The executor crashed or exited while streaming, the connection was reset, the RPC deadline elapsed, or a network blip dropped the stream after the executor had started but before it sent its `StreamClose`. The supervisor re-enqueues this as a genuinely **fresh** dispatch: unlike an application-level retry (which stamps `prior_dispatch_id` + `prior_dispatch_disposition`), the infra re-enqueue carries **no** predecessor identity — both fields are unset — so the re-dispatched run cannot tell from its `ExecuteRequest` that a predecessor ran or did partial work.

## What to do

Investigate executor stability: check the executor's logs for a crash or panic at the corresponding dispatch, confirm it sends a terminal `StreamClose{Success|Error|Park}` (or `AwaitAsyncCallback`) on every path rather than dropping the stream, and confirm the RPC deadline is long enough for the work. The signal payload carries the underlying transport error under `details.error`. Note the re-enqueue is immediate and the application retry counter is not bumped on an infra round-trip, so an executor that crashes deterministically on a given dispatch loops on this class — there is no give-up backstop for infra errors. Because the infra re-dispatch carries no predecessor identity, an executor that may have done partial work before the stream broke must make its dispatch handling **idempotent** — re-running the same dispatch must be safe — since it is given no signal that it is taking over from an interrupted predecessor.

## See also

- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`../../concepts/signal.md`](../../concepts/signal.md)
- [`../../protocols/executor.md`](../../protocols/executor.md)
