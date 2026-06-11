---
error: http/attribute_invalid
surfaced_to: operator
---

# HTTP-node attribute invalid (`http/attribute_invalid`)

## What it means

The bundled `http-node` executor received a dispatch whose attribute bag does not satisfy its attribute contract. No request was sent to the upstream; the dispatch resolves to a terminal `Error{ error_class: "http/attribute_invalid" }`, routable through the node's `error_types:` policy. The signal payload carries an `error` message naming the defect. <!-- @source: lib/services/executors/http-node/server.go::executeCore -->

## When it happens

This is a deterministic template/configuration error, not a transient fault — retrying the same dispatch unchanged produces the same result.

- `attributes.url` missing or empty.
- `attributes.body` present but not JSON-serialisable (a structured body is JSON-marshalled before send).
- The assembled request is unbuildable (malformed URL / request construction failure).
- In stub mode: `attributes.stub_response` present but not a JSON object.

## What to do

Fix the node's `attributes:` so the dispatch bag satisfies the contract. The read keys are `url` (required, non-empty), `method` (default `GET`), `headers` (string → string), `body` (string sent verbatim; structured value JSON-marshalled), `expect_status` (default: every 2xx code), and `error_class_field` (default `error_class`). The payload's `error` message names the failing key.

## See also

- [`http_unexpected_status.md`](http_unexpected_status.md) — the classes for when the attributes were fine and the upstream returned an out-of-set status.
- [`verifier_attribute_invalid.md`](verifier_attribute_invalid.md) — the verifier executors' equivalent class.
- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`../../services/README.md`](../../services/README.md) — the `http-node` catalog entry.
