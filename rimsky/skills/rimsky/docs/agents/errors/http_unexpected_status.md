---
error: http/expectation_mismatch
surfaced_to: operator
---

# HTTP-node unexpected status (`http/request_invalid/*`, `http/server_error/*`, `http/expectation_mismatch`)

## What it means

The bundled `http-node` executor got an HTTP response whose status is outside the node's `attributes.expect_status` set (default: every 2xx code — `[200, 201, 202, 203, 204, 205, 206, 207, 208, 226]`). One mechanism, three class shapes, picked by status range and body taxonomy — except `429`, which is carved out before classification (below): <!-- @source: lib/services/executors/http-node/server.go::classifyUnexpectedStatus -->

| Class | Fires when | Suffix source |
| --- | --- | --- |
| `http/request_invalid/<class>` | Status 4xx AND the body is a JSON object whose configured error-class field is a non-empty string | The upstream's verbatim class token, read from the body field named by `attributes.error_class_field` (default `error_class`, executor-wide override `RIMSKY_EXECUTOR_HTTP_NODE_ERROR_CLASS_FIELD`) |
| `http/request_invalid/_unspecified` | Status 4xx AND the body is a JSON object but the error-class field is missing or empty | Fixed leaf — the upstream rejected the request without supplying taxonomy |
| `http/server_error/<status>` | Status 5xx | The numeric status code (e.g. `http/server_error/503`) |
| `http/expectation_mismatch` | Any other out-of-set status: not 5xx, and either not 4xx or the 4xx body is empty / not a JSON object | Fixed leaf — fallback when no upstream taxonomy is parseable |

**429 carve-out — auto-park, never an error.** An unexpected `429 Too Many Requests` is diverted to a terminal `Park` (`PARK_REASON_SNOOZE`) **before** the unexpected-status classification runs, so a 429 never produces any of the three classes above and never reaches the node's `error_types:` policy chain. <!-- @source: lib/services/executors/http-node/server.go::executeCore (429 branch), ::parseRetryAfter, ::sendParked --> The park's `resume_at` is computed from the response's `Retry-After` header (RFC 9110 delta-seconds or HTTP-date; an empty, malformed, or negative-delta-seconds header falls back to now + 30s — a finite future `resume_at` is required for auto-wake; a parseable HTTP-date that is not in the future resumes immediately at now, the upstream having explicitly cleared the wait), and the supervisor's parked-node sweep wakes the node at `resume_at` and re-dispatches it (`resume_reason = "deadline_elapsed"`). A 429 that `expect_status` explicitly accepts is a normal success per the declared contract — only an *unexpected* 429 parks. This carve-out is `http-node`-specific; the verifier executor has no 429 special case.

`http/request_invalid/*` and `http/server_error/*` are advertised as prefix patterns in `declared_error_classes`, so a template can key `error_types:` on a specific leaf without the registration-time declared-class check rejecting it. The payload's `error` carries `status=<code>, body=<body truncated to 512 bytes>`.

A response that never arrived is the transport siblings [`http/network_error`](http_network_error.md) / [`http/timeout`](http_timeout.md); a 2xx whose body is unusable is [`http/response_unparseable`](http_response_unparseable.md).

## What to do

Read the status range before deciding which side to fix:

- **`http/request_invalid/*` (4xx):** the upstream evaluated and rejected the request — deterministic; retrying unchanged reproduces it. Fix the request the node sends (`url`, `method`, `headers`, `body`, or the upstream's expectations). Key `error_types:` on the typed leaf when the upstream taxonomy is meaningful.
- **`http/server_error/*` (5xx):** the upstream itself is unhealthy — availability-shaped and often transient. Check the target service; a retry-with-backoff policy on `http/server_error/*` (or a specific leaf like `http/server_error/503`) is the usual routing.
- **`http/expectation_mismatch`:** check whether `expect_status` actually matches the upstream's contract (e.g. an upstream that legitimately returns `302`) before treating it as a fault.
- **Rate-limited (429):** you will not see it in this family — the node auto-parks and self-resumes (carve-out above). If a node parks on 429 repeatedly, slow the dispatch cadence or raise the upstream's limit; no `error_types:` routing applies.

## See also

- [`http_response_unparseable.md`](http_response_unparseable.md) — the 2xx-but-unusable-body sibling.
- [`verifier_check_failed.md`](verifier_check_failed.md) — the verifier executor's analogous status-outside-expected family (and the same configured-error-class-field discipline).
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../services/README.md`](../../services/README.md) — the `http-node` catalog entry.
