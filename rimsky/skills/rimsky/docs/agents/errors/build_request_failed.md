---
error: build_request_failed
surfaced_to: operator
---

# Build request failed

## What it means

The supervisor dialed the executor successfully but could not assemble the `ExecuteRequest` to send it. Constructing the request failed before the executor was contacted — typically while building the per-store `StoreHandle` entries from the acquired claims. This is an **infra error**, not an application error: the supervisor moves the node `running → stale`, releases its locks, and **re-enqueues the dispatch immediately**, bypassing the node's `error_types:` policy. It emits a `terminal/infra/build_request_failed` signal (one audit-log row; observable to any `terminal/infra/*` subscriber).

## When it happens

The dominant cause is a claim producer that returned `Address` or `Payload` bytes that do not round-trip as JSON. The supervisor projects those producer-supplied bytes into the wire `StoreHandle.handle` struct verbatim (per the wire-encoding invariant it must not mangle, log, or normalize them); if the bytes are not JSON-decodable it refuses to dispatch rather than ship a silently-corrupted handle. This is a **producer-side contract violation**: the producer's `Open`/`Commit` must return JSON-encodable address and payload bytes.

## What to do

This is almost always a bug in the claim producer, not in the template or the deployment. Fix the producer so its claim `Address` and `Payload` bytes are valid JSON — do not work around it by loosening the supervisor's handle construction. The signal payload carries the underlying error (the failing field and decode error) under `details.error`. Note the re-enqueue is immediate and the application retry counter is not bumped, so a deterministically-failing producer loops on this class until the producer is fixed — there is no give-up backstop for infra errors. Run the claim-producer conformance suite against the producer to confirm its byte shapes.

## See also

- [`../../concepts/claim-producer.md`](../../concepts/claim-producer.md)
- [`../../concepts/claim-handle.md`](../../concepts/claim-handle.md)
- [`../../concepts/signal.md`](../../concepts/signal.md)
- [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)
