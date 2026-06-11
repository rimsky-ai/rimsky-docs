---
error: http/network_error
surfaced_to: operator
---

# HTTP-node network error (`http/network_error`)

## What it means

The bundled `http-node` executor could not complete the HTTP exchange with the configured `attributes.url` for a non-timeout transport reason (DNS failure, connection refused, TLS failure, connection reset). No response arrived, so the upstream's verdict on the request is **unknown**. The dispatch resolves to a terminal `Error{ error_class: "http/network_error" }`; the payload's `error` carries the transport error text. <!-- @source: lib/services/executors/http-node/server.go::classifyTransportErr -->

Timeout-shaped transport failures are the sibling [`http/timeout`](http_timeout.md); a response that **did** arrive with an out-of-set status is [`http_unexpected_status.md`](http_unexpected_status.md).

## When it happens

The Go HTTP client returned an error that is neither a context deadline-exceeded nor a transport-level timeout (both of those classify as `http/timeout`). Typical causes: the upstream is down, the URL's host is unresolvable, or the network path is broken.

## What to do

Transient by nature — a retry-shaped `error_types:` entry (`http/network_error: retry` with backoff) is the usual policy. If it persists, verify the upstream is reachable from the executor's network (service up, DNS, ports) and that `attributes.url` points where you think it does. Because the verdict is unknown, do not assume the upstream did or did not process the request — design the upstream call to be idempotent if retries are routed here.

## See also

- [`http_timeout.md`](http_timeout.md) — the timeout-shaped transport sibling.
- [`verifier_network_error.md`](verifier_network_error.md) — the verifier executor's equivalent class.
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
