---
error: verifier/timeout
surfaced_to: operator
---

# Verifier timeout (`verifier/timeout`)

## What it means

The `verifier-http` executor's request to the verifier endpoint exceeded its deadline (the dispatch's `attributes.timeout_ms`, default `60000`) or failed with a transport-layer `Timeout()` error. The check's verdict is unknown. The dispatch resolves to a terminal `Error{ error_class: "verifier/timeout" }`, routed through the node's `error_types:` policy.

The class is split from [`verifier/network_error`](verifier_network_error.md) deliberately: operators typically want to retry timeouts with backoff (slow upstream, transient load) while handling hard connectivity failures differently.

## When it happens

The verifier endpoint is up but slow — the check takes longer than `timeout_ms` — or the network path stalls past the deadline.

## What to do

Either raise `attributes.timeout_ms` to fit the check's real latency, or fix the upstream's slowness. A `retry` `error_types:` action with backoff on this class is the usual policy. If timeouts persist at a generous deadline, treat it as a connectivity/capacity problem on the verifier service.

## See also

- [`verifier_network_error.md`](verifier_network_error.md) — the non-timeout transport sibling.
- [`verifier_check_failed.md`](verifier_check_failed.md) — the class for when the round-trip succeeded and the check failed.
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../services/README.md`](../../services/README.md) — the `verifier-http` catalog entry.
