---
error: http/timeout
surfaced_to: operator
---

# HTTP-node timeout (`http/timeout`)

## What it means

The bundled `http-node` executor's request to `attributes.url` exceeded the executor's HTTP timeout (`RIMSKY_EXECUTOR_HTTP_NODE_TIMEOUT_MS`, default `60000`) or hit a transport-level `net.Error` with `Timeout() == true`. No usable response arrived, so the upstream's verdict is **unknown** — the request may or may not have been processed. The dispatch resolves to a terminal `Error{ error_class: "http/timeout" }`; the payload's `error` carries the timeout error text. <!-- @source: lib/services/executors/http-node/server.go::classifyTransportErr, lib/services/executors/http-node/config.go::LoadConfig -->

## When it happens

The upstream is slow or hung past the deadline, or the network path stalls. Distinguished from [`http/network_error`](http_network_error.md) (non-timeout transport failure) so a policy can back off on timeouts but alert on hard connectivity failures.

## What to do

Retry with backoff is the usual policy. If legitimate upstream work takes longer than the deadline, raise `RIMSKY_EXECUTOR_HTTP_NODE_TIMEOUT_MS` on the executor. Because the verdict is unknown, make the upstream call idempotent before routing retries here.

## See also

- [`http_network_error.md`](http_network_error.md) — the non-timeout transport sibling.
- [`verifier_timeout.md`](verifier_timeout.md) — the verifier executor's equivalent class.
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
