---
error: acquire/unavailable
surfaced_to: operator
---

# `acquire/unavailable`

## What it means

A `ClaimProducer.Open` returned the `Unavailable` outcome — the producer refused to mint the claim (resource busy, conflicting holder, capacity exhausted, producer-specific contention). The supervisor rolled the in-flight acquisition back (`Abandon` on any partial siblings — see the verb table in [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)) and routed the failure through the node's `error_types:` chain keyed on the acquisition-failure class.

The keying class is `acquire/unavailable` (synthetic) by default, OR a producer-declared leaf when the `Unavailable` carried a non-empty `error_class` field — for example a Postgres-backed producer names `pg/claim_unavailable` on an empty pick. Chain lookup (`lookupPolicy` in `lib/runtime/on_error.go`) tries the runtime-keyed class first (exact match on `pg/claim_unavailable` if the producer named one, else `acquire/unavailable`), then falls back to the synthetic family class `acquire/unavailable` passed as `PolicyFallbackClass` — `handleAcquireUnavailable` (`lib/runtime/runner_lifecycle.go`) always sets that fallback. So `error_types: { acquire/unavailable: ... }` alone DOES cover both cases — it catches the synthetic-keyed default AND catches producer-classified failures (like `pg/claim_unavailable`) when no exact entry is declared. An exact-key entry wins when both are declared: `error_types: { pg/claim_unavailable: retry, acquire/unavailable: pass }` routes the Postgres leaf through retry and any other producer's leaf (or the synthetic default) through pass. A node using claim producers without ANY `acquire/unavailable` entry gets a template-registration warning (not a rejection) — the default resolution is give-up, which is rarely what the operator wants.

## When it happens

Producer-specific contention: another holder already owns an overlapping scope, the producer's per-scope concurrency cap is saturated, the underlying backend (Postgres advisory lock, filesystem flock, etc.) reports the resource is taken. The `Unavailable` is the producer's polite refusal — not an infra error and not an executor error.

## What to do

Declare an explicit `acquire/unavailable` policy on the node so the resolution is intentional. The four actions live on `concept:error-policy`:

- `pass` — drop the dispatch silently (the common case for queue-worker patterns where the work will be re-presented on the next invalidate).
- `retry` — re-attempt the acquisition after the per-action backoff (`backoff: | base_delay_ms: | max_delay_ms: | jitter:` on the `PolicyAction` itself — see `concept:error-policy`); use for transient contention with reasonable hope of clearing.
- `discard_claims_then_retry` — release everything already acquired, then retry the whole acquisition (use when partial holds may themselves be causing the contention).
- `give_up` — terminal-fail the node (the default; rarely the right answer for a claim-producer-using node).

**The `acquire/unavailable` key is also the fallback when the producer names a leaf the template has no exact entry for.** `handleAcquireUnavailable` passes `PolicyFallbackClass: "acquire/unavailable"` to `OnError` precisely so an operator who declares only the generic family still catches producer-classified failures: `error_types: { acquire/unavailable: retry }` matches both the synthetic case AND a producer-classified `pg/claim_unavailable` failure when no exact `pg/claim_unavailable:` entry is declared. The exact-key entry (e.g. `pg/claim_unavailable:`) wins when both are declared.

Routing on the producer-declared leaf is range-checked at registration but never hard-rejected: `validateErrorTypes` (`lib/graph/node/template_validator.go`) range-checks every `error_types:` key against the **union** of (a) the node's executor's `declared_error_classes`, (b) the `declared_error_classes` of every claim producer reachable through the node's `stores:` block, and (c) the reserved `acquire/*` synthetic family plus a runtime-synthesized exempt list. A key that matches any of those passes silently. A key attributable to no declared vocabulary surfaces as an advisory **warning** (`res.Warnings`), never a hard rejection — peers MAY declare nothing, and an undeclared vocabulary must not lock operators out of routing classes the system itself emits. So `error_types: { pg/claim_unavailable: ... }` registers cleanly whenever the node's stores include a producer whose `CapabilitiesResponse.declared_error_classes` carries `pg/claim_unavailable` (the common case for a postgres-backed store); on any other node it still registers, just with a warning that the policy will only fire if some peer emits the exact class.

Two complementary surfaces for reacting to producer-declared leaves remain useful:

- **Accept the `give_up` default and observe the terminal signal.** The chain resolves give_up and emits `terminal/error/pg/claim_unavailable` on the event log — deterministic and queryable (`GET /v1/events?instance_id=<instance_id>&kind=terminal/error/pg/claim_unavailable`).
- **React in-graph with an instance-scoped wildcard subscription** on a second node: `subscribes: [{ instance: true, type: terminal/error/*, frame: in }]`. Instance-scoped wildcards are not range-checked against any peer's vocabulary — this is the tested idiom (rimsky-core's `pg_error_classes` scenario, `lib/services/test/scenarios/pg_error_classes/pg_error_classes_test.go`).

See the [queue-worker cookbook](../../cookbook/queue-worker.md) gotchas for the worked drained-queue case.

The acquisition-failure routing is not an `Error` outcome from the executor — the executor never ran. There is no `ExecutorObservability` event for this class; the audit-log entry surfaces through the standard `terminal/*` signal (see [`../../concepts/signal.md`](../../concepts/signal.md)) at the `error_types:` chain's resolution.

## See also

- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../concepts/claim-producer.md`](../../concepts/claim-producer.md)
- [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)
