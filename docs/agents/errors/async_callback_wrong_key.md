---
error: async_callback_wrong_key
surfaced_to: executor
---

# Async-callback body rejected

## What it means

An executor POSTed to the supervisor's async-callback URL (`POST {callback_url}/v1/callback/{async_ack_id}`) with a body that does not parse as an `AsyncCallbackBody`. The supervisor rejects the body with HTTP 400.

The body must carry exactly one outcome — `success`, `error`, or `park` — plus an optional `events` array replayed before the outcome verdict. The legacy discriminator shape `{type: "complete"|"blocked"|"errored"}` is no longer accepted and is rejected with HTTP 400.

## When it happens

When implementing an executor that uses the async-handoff path (it returned `AwaitAsyncCallback` on the gRPC stream, then reports the terminal outcome later via this HTTP+JSON callback). Common mistakes:

- Posting the legacy `{type: ...}` discriminator body instead of the outcome oneof.
- Setting zero or more than one of `success` / `error` / `park` — exactly one is required.
- Sending malformed JSON.

## What to do

Shape the body as an `AsyncCallbackBody` with exactly one outcome variant set. The HTTP+JSON body mirrors the gRPC `StreamClose` outcome oneof:

- `success` — optional `changed`, `change_summary`, and `attributes_delta`.
- `error` — `error_class` plus optional `payload`.
- `park` — `reason` (snake_case `ParkReason`), with optional `reason_note`, `reason_label`, `payload`, `resume_at`, `session_token`.

The claude-agent reference executor (in rimsky-core under `lib/services/executors/claude-agent/`) and its test suite cover the exact wire shape; align with that.

## See also

- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`../../protocols/executor.md`](../../protocols/executor.md)
