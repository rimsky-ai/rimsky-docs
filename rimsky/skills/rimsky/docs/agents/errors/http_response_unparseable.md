---
error: http/response_unparseable
surfaced_to: operator
---

# HTTP-node response unparseable (`http/response_unparseable`)

## What it means

The bundled `http-node` executor got a response whose status **is** in the `attributes.expect_status` set — the request succeeded at the HTTP level — but the response body could not be turned into the node's `attributes_delta`: the body is malformed JSON, or parses to something other than a JSON object (an array or scalar violates the attribute-bag shape). The dispatch resolves to a terminal `Error{ error_class: "http/response_unparseable" }`; the payload's `error` carries the parse error. <!-- @source: lib/services/executors/http-node/server.go::executeCore, ::buildAttributesDelta -->

This is an upstream **contract** violation, not an upstream failure verdict — the status said success. Out-of-set statuses are [`http_unexpected_status.md`](http_unexpected_status.md).

## What to do

Deterministic for a given upstream response — fix the upstream to return a JSON object body on success, or front it with a shim that does. Retrying helps only if the upstream's malformed output is itself intermittent.

## See also

- [`http_unexpected_status.md`](http_unexpected_status.md) — the out-of-set-status family.
- [`attribute_validation_failed_at_commit.md`](attribute_validation_failed_at_commit.md) — the supervisor-side failure when a well-formed delta fails the node's attribute schema.
- [`../../concepts/attribute.md`](../../concepts/attribute.md)
