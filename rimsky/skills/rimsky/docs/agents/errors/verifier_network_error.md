---
error: verifier/network_error
surfaced_to: operator
---

# Verifier network error (`verifier/network_error`)

## What it means

The `verifier-http` executor could not complete the HTTP round-trip to the verifier endpoint for a transport-layer reason that is **not** a timeout — DNS failure, connection refused, connection reset, and similar. The check's verdict is unknown: this says nothing about whether the verified work is good. The dispatch resolves to a terminal `Error{ error_class: "verifier/network_error" }`, routed through the node's `error_types:` policy (it is an application-level executor `Error`, not a `terminal/infra/*` re-enqueue — the executor itself was reachable and answered).

Timeout-shaped transport failures are split out as the sibling class [`verifier/timeout`](verifier_timeout.md) so operators can attach a retry policy to timeouts without also retrying hard connectivity failures.

## When it happens

The endpoint in `attributes.url` is down, unresolvable, or unreachable from the executor's network; TLS or connection establishment failed; the connection dropped mid-request without a deadline being involved.

## What to do

Fix reachability between the `verifier-http` executor and the configured `url`: confirm the verifier service is up, the URL is correct, and no network policy blocks the route. Because the verdict is unknown, a `retry` `error_types:` action on this class is usually appropriate once connectivity is restored; the signal payload's `message` carries the underlying transport error.

## See also

- [`verifier_timeout.md`](verifier_timeout.md) — the deadline-shaped sibling.
- [`verifier_check_failed.md`](verifier_check_failed.md) — the class for when the round-trip succeeded and the check failed.
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../services/README.md`](../../services/README.md) — the `verifier-http` catalog entry.
