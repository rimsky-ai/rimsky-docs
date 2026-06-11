---
error: http/internal_error
surfaced_to: operator
---

# HTTP-node internal error (`http/internal_error`)

## What it means

The `http-node` executor's own dispatch handling failed — `executeCore` returned an error the executor could not classify into any of its other declared classes. Emitted by the executor's HTTP+JSON bridge as a terminal `Error{ error_class: "http/internal_error" }` with an empty payload; it is the executor-bug catch-all, not a statement about the upstream or the attributes. <!-- @source: lib/services/executors/http-node/bridge.go::mountBridge -->

## What to do

This indicates a fault in the executor itself (or its transport wiring), not in the template or the upstream. Check the executor's logs for the underlying error; retrying may mask rather than fix it. If reproducible, it is a bug to fix in the executor.

## See also

- [`http_attribute_invalid.md`](http_attribute_invalid.md) — the classified attribute-contract failure (not this catch-all).
- [`../../services/README.md`](../../services/README.md) — the `http-node` catalog entry.
