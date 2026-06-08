---
error: stream_closed_without_terminal
surfaced_to: operator
---

# Stream closed without terminal

## What it means

The executor accepted the `Execute` RPC, the stream went live, the executor *cleanly* closed the stream (EOF on `Recv()`) but never sent a terminal `StreamClose` outcome (`Success`, `Error`, `Park`) or `AwaitAsyncCallback`. The supervisor has no verdict to commit. This is an **infra error**, not an application error: the supervisor moves the node `running → stale` (the awaiting-re-dispatch state — see [`../../concepts/node.md`](../../concepts/node.md)), releases its locks, and **re-enqueues the dispatch immediately**, bypassing the node's `error_types:` policy. It emits a `terminal/infra/stream_closed_without_terminal` signal (one audit-log row; observable to any sibling node whose `subscribes:` block matches `terminal/infra/*` — see [`../../concepts/node-subscription.md`](../../concepts/node-subscription.md)).

Distinct from `stream_error`: that class is a *non-EOF* transport error mid-flight (executor crash, connection reset, deadline exceeded); `stream_closed_without_terminal` is a *clean* EOF with no verdict — an executor contract violation, not a transport problem.

## When it happens

The executor returned from its `Execute` handler (the gRPC stream completed without error) without first sending a terminal outcome on the stream. The most common cause is a control-flow bug in the executor — an early return, a fall-through, or a missing send in some code path. Less commonly: a graceful shutdown of the executor process in the middle of a handler, where the handler returned cleanly without first sending an outcome.

## What to do

This is almost always a bug in the executor, not in the template or the deployment. Audit the executor's `Execute` handler: every code path must finish by sending exactly one of `StreamClose{Success | Error | Park}` or `AwaitAsyncCallback` before returning. Run the executor conformance suite (`rimsky conformance executor --endpoint <endpoint> --transport grpc`) — its scenario coverage exercises the every-path-must-terminate contract.

Note the re-enqueue is immediate and the application retry counter is not bumped on an infra round-trip, so a deterministically-misbehaving executor loops on this class until the executor is fixed — there is no give-up backstop for infra errors. The infra re-dispatch carries **no** predecessor identity (the `prior_dispatch_id` and `prior_dispatch_disposition` fields on `proto:executor.proto::ExecuteRequest` are both unset), so the re-dispatched run cannot tell from its `ExecuteRequest` that a predecessor ran or did partial work — an executor that may have done partial work before the clean close must make its dispatch handling **idempotent**.

## See also

- [`stream_error.md`](stream_error.md) — the sibling non-EOF transport class.
- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`../../concepts/signal.md`](../../concepts/signal.md)
- [`../../protocols/executor.md`](../../protocols/executor.md)
