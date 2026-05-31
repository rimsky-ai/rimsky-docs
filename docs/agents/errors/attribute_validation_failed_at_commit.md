---
error: attribute_validation_failed_at_commit
surfaced_to: executor
---

# Attribute validation failed at commit

## What it means

The executor's terminal success record — `StreamClose{Success}` carrying `attributes_delta` (the gRPC outcome; or the `success` outcome on the async-callback body) — merged into the node's resolved attributes and failed to validate against the node's `attributes:` schema at commit. The supervisor routed the failure through `Error{ error_class: "attributes_schema_failed" }` and emitted an `attributes_schema_failed` event; the node failed per its `error_types:` policy (default give-up).

## When it happens

When an executor returns an `attributes_delta` that drifts from what the node's schema declares. Validation runs only when the success outcome reports `changed: true` with a non-empty `attributes_delta` and the node has an effective schema. Common cases: missing required fields, wrong types, fields outside the schema's allowed shape.

## What to do

The executor must produce an `attributes_delta` that, once merged onto the resolved attributes, validates against the schema. Check the merged result against the schema; correct either the delta (if the executor is wrong) or the schema (if the schema is wrong). The validation runs at commit precisely to surface this drift early — don't add per-executor schema-loosening as a workaround. The `attributes_schema_failed` event payload names the failing validation message.

## See also

- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`../../concepts/executor.md`](../../concepts/executor.md)
