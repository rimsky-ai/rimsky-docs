---
error: attribute_validation_failed_at_dispatch
surfaced_to: cli-user
---

# Attribute validation failed at dispatch

## What it means

After resolving `{{...}}` substitution directives, the resulting input attributes did not validate against the node's effective `attributes:` schema (composition violations, type mismatches, override-vs-schema conflicts, or a dispatch-bag failure against the executor's advertised `expected_attributes_schema`). The supervisor routed the failure through `Error{ error_class: "template_validation_failed" }` and emitted a `template_validation_failed` event; the dispatch was rejected and the node did not run.

## When it happens

Most commonly when an upstream node's committed attributes don't match the consuming node's expected types — a substitution returns a value whose JSON shape doesn't fit the schema. Also: a `{{params.<key>}}` reference resolved to an unexpected type, a template default that conflicts with the schema, or a value the executor's advertised `expected_attributes_schema` rejects.

(A *missing* required source — a strict directive that resolved to nothing — is a different class, `template_resolution_failed`, the canonical retry-after-cascade case. An executor whose `expected_attributes_schema` is not yet visible at dispatch surfaces as its own class, [`executor_schema_unavailable`](executor_schema_unavailable.md).)

## What to do

Check the executor trace and the named-source values (`{{nodes.<source>.attribute.<field>}}` resolutions, `{{params.<key>}}` resolutions) against the node's `attributes:` schema. The error message will name the failing JSON-Schema path. Either correct the upstream value, adjust the schema, or fix the substitution-source declaration.

## See also

- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`../../concepts/node.md`](../../concepts/node.md)
- [`executor_schema_unavailable.md`](executor_schema_unavailable.md) — the sibling dispatch-time class for when the executor's schema is not yet visible.
