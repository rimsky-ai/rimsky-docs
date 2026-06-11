---
error: verifier/attribute_invalid
surfaced_to: operator
---

# Verifier attribute invalid (`verifier/attribute_invalid`)

## What it means

A bundled verifier executor (`verifier-http` or `verifier-shape-checks`) received a dispatch whose attribute bag does not satisfy the verifier's attribute contract. The check never ran; the dispatch resolves to a terminal `Error{ error_class: "verifier/attribute_invalid" }`, routable through the node's `error_types:` policy. The signal payload carries a `message` naming the offending key.

## When it happens

This is a deterministic template/configuration error, not a transient fault — retrying the same dispatch unchanged produces the same result.

- `verifier-http`: `attributes.url` missing or empty; `attributes.body` not JSON-serialisable; or the assembled request is unbuildable (malformed URL).
- `verifier-shape-checks`: `attributes.checks` missing or not an array; a `rows[*]` entry not an object; or a similarly malformed required key.

## What to do

Fix the node's `attributes:` so the dispatch bag satisfies the verifier's contract. For `verifier-http` the read keys are `url` (required, non-empty), `body` (sent verbatim as the POST body), `expected_status` (default `[200]`), `timeout_ms` (default `60000`), and `class_field` (default `class`). For `verifier-shape-checks`, supply a valid `checks` array and object-shaped rows. The error `message` names the failing key.

## See also

- [`verifier_check_failed.md`](verifier_check_failed.md) — the class for when the attributes were fine and the check itself failed.
- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../services/README.md`](../../services/README.md) — the `verifier-http` / `verifier-shape-checks` catalog entries.
