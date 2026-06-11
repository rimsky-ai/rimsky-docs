---
error: retry_loop_no_progress
surfaced_to: operator
---

# Retry loop made no progress (`retry_loop_no_progress`)

## What it means

A node's `error_types:` policy kept resolving to a retry-shaped action, the dispatch row's consecutive-retries-without-progress counter reached the effective cap, and the runtime forced the verdict to `Error{ error_class: "retry_loop_no_progress" }` instead of retrying again. The forced class then resolves through the policy chain itself — absent an explicit `error_types: { retry_loop_no_progress: ... }` entry it falls through to give-up, so the node terminal-fails rather than spinning forever. The signal payload preserves forensics: `original_error_class` and `original_payload` carry the class and payload of the error that *would* have retried.

This is the runtime's loop breaker, not a distinct fault in the work: the real problem is whatever error class kept recurring (read it from `original_error_class`).

## When it happens

Each terminal that resolves to a retry-shaped action (`retry` | `discard_claims_then_retry`) increments the counter and carries it onto the re-enqueued dispatch row; only consecutive retry resolutions carry it forward (a fresh dispatch starts at zero, and an infra re-enqueue preserves — does not bump — it). "Progress" means the run settling: any non-retry resolution (a success terminal, `pass`, or `give_up` — the resolutions that write a `settling_signal_type` on the run row) ends the chain and the counter with it; within a chain the runtime compares nothing per round — not the error class, not the payload — so a loop that alternates between different error classes, or fails with a different payload every time, still counts as no progress (see [`../../concepts/error-policy.md`](../../concepts/error-policy.md)). <!-- @source: lib/runtime/runner_error_policy.go::applyErrorPolicy -->

The exact firing condition: a cap of `N` permits `N` consecutive retries; on the terminal whose retry resolution would be the `(N+1)`-th — i.e. when the row's counter has already reached `N` — the class is rewritten to `retry_loop_no_progress` before policy lookup ("`maxRetries=100` means 100 retries permitted; the 101st is forced give_up"). <!-- @source: lib/runtime/runner_error_policy.go::shouldForceRetryLoopGiveUp --> The cap resolves in precedence order:

| Level | Source |
| --- | --- |
| 1 | Per-row dispatch-tuning override (denormalized onto the dispatch row at park time) |
| 2 | Template-spec `max_retries_without_progress` on the node |
| 3 | Deployment-level **supervisor** default — the supervisor's config carries the max-retries-without-progress default and threads it into the runner; the runner applying the cap runs inside the supervisor process <!-- @source: lib/runtime/supervisor.go::Config.MaxRetriesWithoutProgressDefault, lib/runtime/runner_terminal_park.go::resolveMaxRetriesCap --> |
| 4 | Built-in default `100` |

An override of `0` (per-row or per-node) disables the cap entirely for that node.

## What to do

Diagnose the underlying recurring error first — `original_error_class` in the payload names it; fix that and the loop disappears. If the loop is legitimate (a convergence cycle expected to take many rounds), raise the node's `max_retries_without_progress` (or set `0` to disable the cap) rather than declaring a `retry` policy on `retry_loop_no_progress` itself — re-retrying the loop breaker defeats its purpose.

## See also

- [`../../concepts/error-policy.md`](../../concepts/error-policy.md) — the `error_types:` policy chain and the cap.
- [`../../reference/template-schema.md`](../../reference/template-schema.md) — `max_retries_without_progress` on the node spec.
- [`../../cookbook/convergence-loop.md`](../../cookbook/convergence-loop.md) — the pattern that most often interacts with the cap.
