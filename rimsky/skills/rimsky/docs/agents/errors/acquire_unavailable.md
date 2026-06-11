---
error: acquire/unavailable
surfaced_to: operator
---

# `acquire/unavailable`

## What it means

A `ClaimProducer.Open` returned the `Unavailable` outcome — the producer refused to mint the claim (resource busy, conflicting holder, capacity exhausted, producer-specific contention). The supervisor rolled the in-flight acquisition back (`Abandon` on any partial siblings — see the verb table in [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)) and routed the failure through the node's `error_types:` chain keyed on the acquisition-failure class.

The keying class is `acquire/unavailable` (synthetic) by default, OR a producer-declared leaf when the `Unavailable` carried a non-empty `error_class` field — for example a Postgres-backed producer names `pg/claim_unavailable` on an empty pick. Chain lookup is exact-match on the runtime-keyed class (`lookupPolicy` in `lib/runtime/on_error.go`): when the producer named a leaf (`pg/claim_unavailable`), only an `error_types: { pg/claim_unavailable: ... }` entry matches; when it did not, only `error_types: { acquire/unavailable: ... }` matches. There is no single `error_types:` key that covers both cases — and a producer-declared leaf is usually **not registrable** on the node at all (see "What to do"). A node using claim producers without ANY `acquire/unavailable` entry gets a template-registration warning (not a rejection) — the default resolution is give-up, which is rarely what the operator wants.

## When it happens

Producer-specific contention: another holder already owns an overlapping scope, the producer's per-scope concurrency cap is saturated, the underlying backend (Postgres advisory lock, filesystem flock, etc.) reports the resource is taken. The `Unavailable` is the producer's polite refusal — not an infra error and not an executor error.

## What to do

Declare an explicit `acquire/unavailable` policy on the node so the resolution is intentional. The four actions live on `concept:error-policy`:

- `pass` — drop the dispatch silently (the common case for queue-worker patterns where the work will be re-presented on the next invalidate).
- `retry` — re-attempt the acquisition after the per-action backoff (`backoff: | base_delay_ms: | max_delay_ms: | jitter:` on the `PolicyAction` itself — see `concept:error-policy`); use for transient contention with reasonable hope of clearing.
- `discard_claims_then_retry` — release everything already acquired, then retry the whole acquisition (use when partial holds may themselves be causing the contention).
- `give_up` — terminal-fail the node (the default; rarely the right answer for a claim-producer-using node).

**The `acquire/unavailable` key covers only the synthetic case** — durable conflicts and producers that name no class on their `Unavailable` response. When the producer DOES name a leaf (`pg/claim_unavailable`), the runtime keys the chain on that exact leaf, so the `acquire/unavailable:` entry never fires for it.

Routing on the producer-declared leaf itself is constrained at registration: `validateErrorTypes` (`lib/graph/node/template_validator.go`) range-checks every `error_types:` key against the node's **executor's** `declared_error_classes` whenever the executor advertises a non-empty set (the common case — http-node declares only `http/*`, claude-agent its own set). Only the `acquire/*` prefix plus a fixed runtime-synthesized exempt list passes regardless; `pg/claim_unavailable` is owned by the *store*, not the executor, so the key is rejected. `error_types: { pg/claim_unavailable: ... }` registers only when the node's executor happens to declare that class (rare). For everyone else the workable surfaces are:

- **Accept the `give_up` default and observe the terminal signal.** The chain resolves give_up and emits `terminal/error/pg/claim_unavailable` on the event log — deterministic and queryable (`GET /v1/events?instance_id=<instance_id>&kind=terminal/error/pg/claim_unavailable`).
- **React in-graph with an instance-scoped wildcard subscription** on a second node: `subscribes: [{ instance: true, type: terminal/error/*, frame: in }]`. Instance-scoped wildcards are not range-checked against any executor's vocabulary — this is the tested idiom (rimsky-core's `pg_error_classes` scenario, `lib/services/test/scenarios/pg_error_classes/pg_error_classes_test.go`).

See the [queue-worker cookbook](../../cookbook/queue-worker.md) gotchas for the worked drained-queue case.

The acquisition-failure routing is not an `Error` outcome from the executor — the executor never ran. There is no `ExecutorObservability` event for this class; the audit-log entry surfaces through the standard `terminal/*` signal (see [`../../concepts/signal.md`](../../concepts/signal.md)) at the `error_types:` chain's resolution.

## See also

- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../concepts/claim-producer.md`](../../concepts/claim-producer.md)
- [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)
