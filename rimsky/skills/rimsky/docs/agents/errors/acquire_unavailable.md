---
error: acquire/unavailable
surfaced_to: operator
---

# `acquire/unavailable`

## What it means

A `ClaimProducer.Open` returned the `Unavailable` outcome — the producer refused to mint the claim (resource busy, conflicting holder, capacity exhausted, producer-specific contention). The supervisor rolled the in-flight acquisition back (`Abandon` on any partial siblings — see the verb table in [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)) and routed the failure through the node's `error_types:` chain keyed on the acquisition-failure class.

The keying class is `acquire/unavailable` (synthetic) by default, OR a producer-declared leaf when the `Unavailable` carried a non-empty `error_class` field — for example a Postgres-backed producer may declare `pg/claim_unavailable`. Chain lookup is exact-match on the runtime-keyed class: when the producer named a leaf (`pg/claim_unavailable`), only an `error_types: { pg/claim_unavailable: ... }` entry matches; when it did not, only `error_types: { acquire/unavailable: ... }` matches. To catch both cases under one standing policy, declare both keys. A node using claim producers without ANY `acquire/unavailable` entry gets a template-registration warning (not a rejection) — the default resolution is give-up, which is rarely what the operator wants.

## When it happens

Producer-specific contention: another holder already owns an overlapping scope, the producer's per-scope concurrency cap is saturated, the underlying backend (Postgres advisory lock, filesystem flock, etc.) reports the resource is taken. The `Unavailable` is the producer's polite refusal — not an infra error and not an executor error.

## What to do

Declare an explicit `acquire/unavailable` policy on the node so the resolution is intentional. The four actions live on `concept:error-policy`:

- `pass` — drop the dispatch silently (the common case for queue-worker patterns where the work will be re-presented on the next invalidate).
- `retry` — re-attempt the acquisition after the per-action backoff (`backoff: | base_delay_ms: | max_delay_ms: | jitter:` on the `PolicyAction` itself — see `concept:error-policy`); use for transient contention with reasonable hope of clearing.
- `discard_claims_then_retry` — release everything already acquired, then retry the whole acquisition (use when partial holds may themselves be causing the contention).
- `give_up` — terminal-fail the node (the default; rarely the right answer for a claim-producer-using node).

If the producer declares a leaf like `pg/claim_unavailable`, an operator routes on that exact class (`error_types: { pg/claim_unavailable: ... }`) for producer-specific handling. Declare both `pg/claim_unavailable` and `acquire/unavailable` on the same node to cover both the producer-named-a-leaf and the synthetic-fallback cases under one standing policy.

The acquisition-failure routing is not an `Error` outcome from the executor — the executor never ran. There is no `ExecutorObservability` event for this class; the audit-log entry surfaces through the standard `terminal/*` signal (see [`../../concepts/signal.md`](../../concepts/signal.md)) at the `error_types:` chain's resolution.

## See also

- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../concepts/claim-producer.md`](../../concepts/claim-producer.md)
- [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)
