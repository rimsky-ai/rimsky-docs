---
error: verifier/check_failed
surfaced_to: operator
---

# Verifier check failed (`verifier/check_failed`, `verifier/check_failed/*`)

## What it means

A bundled verifier executor ran its check and the check itself failed — the work being verified did not pass. The dispatch resolves to a terminal `Error` whose class is in the `verifier/check_failed` family, routable through the node's `error_types:` policy like any other declared class. Two class shapes exist in the family:

- **`verifier/check_failed/<suffix>`** — the typed leaf. For `verifier-http`, the suffix is the upstream's verbatim class string, read from the failing response body's JSON `class` field (override the field name per dispatch via `attributes.class_field`); the same token is echoed on the signal payload as `upstream_class`. The class-field parse runs on **every** out-of-set status — it is not gated on 4xx/5xx, so with `expected_status: [201]` an actual `200` still has its body's `class` field parsed and emits the typed leaf when the field is a non-empty string. <!-- @source: lib/services/executors/verifier-http/executor.go::executeCore, ::extractClassField --> For `verifier-shape-checks`, the suffix is the failing check's `kind` (e.g. `verifier/check_failed/pk_unique`, `verifier/check_failed/row_count_absolute`). Both executors advertise the family as the `verifier/check_failed/*` wildcard in `declared_error_classes`, so a template can route `error_types: { verifier/check_failed/<leaf>: ... }` without the registration-time declared-class check rejecting it.
- **`verifier/check_failed`** (exact, `verifier-http` only) — the stable fallback when the upstream's failure body carries no parseable class token (empty body, non-JSON-object body, or the class field missing / not a non-empty string). It exists so a `verifier/check_failed`-keyed policy still matches taxonomy-less upstreams instead of collapsing to a catch-all.

This family means the *verification* failed, not the verifier. Transport problems reaching the verifier endpoint are the sibling classes [`verifier/network_error`](verifier_network_error.md) / [`verifier/timeout`](verifier_timeout.md); a malformed attribute bag is [`verifier/attribute_invalid`](verifier_attribute_invalid.md).

## When it happens

- `verifier-http`: the configured endpoint (`attributes.url`) responded with a status outside the operator's `attributes.expected_status` set (default `[200]`).
- `verifier-shape-checks`: one or more of the shape checks declared in `attributes.checks` failed against the supplied rows.

For `verifier-http` the signal payload carries `actual_status`, `expected_status`, and `body_preview` (the response body truncated to 512 bytes), plus `upstream_class` when a typed leaf fired, so the operator sees what the upstream actually returned. <!-- @source: lib/services/executors/verifier-http/executor.go::executeCore -->

## What to do

Mechanically the family means only "status outside `expected_status`" — read `actual_status` before deciding which side to fix:

- **Data-shaped failure (typically 4xx, or any taxonomy the upstream emits via the class field):** the upstream evaluated the work and rejected it. Inspect the upstream check (the `verifier-http` target service's verdict, or the data the shape checks ran over) and fix the producing side.
- **Availability-shaped failure (5xx — the verifier endpoint itself is unhealthy):** the upstream could not evaluate the work, so the verdict on the data is effectively unknown. Check the target service's health first; fixing the producing side is misdirected effort. If 5xx rounds are expected to be transient, route them to a retry via a typed leaf the upstream emits on its 5xx body (or accept that the untyped `verifier/check_failed` fallback conflates the two shapes and key the retry there deliberately).

The boundary with the transport siblings stays crisp: `verifier/check_failed` always means an HTTP response **arrived** with an out-of-set status; [`verifier/network_error`](verifier_network_error.md) / [`verifier/timeout`](verifier_timeout.md) mean no response arrived at all.

For policy routing, key `error_types:` on the typed leaf when the upstream taxonomy is meaningful (e.g. retry one class, give up on another) and keep a `verifier/check_failed` entry as the fallback for untyped failures. The typical home for this class is the abandon branch of the atomic-staging-with-verifier pattern: a failed check abandons the staged claim rather than committing it.

## See also

- [`verifier_network_error.md`](verifier_network_error.md) · [`verifier_timeout.md`](verifier_timeout.md) — the sibling transport classes (the verifier itself was unreachable).
- [`verifier_attribute_invalid.md`](verifier_attribute_invalid.md) — the sibling attribute-contract class.
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../concepts/signal.md`](../../concepts/signal.md)
- [`../../services/README.md`](../../services/README.md) — the `verifier-http` / `verifier-shape-checks` catalog entries.
