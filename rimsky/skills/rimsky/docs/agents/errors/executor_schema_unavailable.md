---
error: executor_schema_unavailable
surfaced_to: operator
---

# Executor schema unavailable

## What it means

At dispatch the supervisor needs the executor's advertised `expected_attributes_schema` to compute the node's effective `attributes:` schema, but that schema is not visible: the executor handshake has not completed, the discovery cache is empty, or the executor advertises no schema while the node carries an `attributes:` block. The supervisor refuses to dispatch on an unknown contract. It emits an `executor_schema_unavailable` event and routes the failure through `Error{ error_class: "executor_schema_unavailable" }`; the node fails per its `error_types:` policy (default give-up).

This is a distinct class from `template_validation_failed` — that one means the bag was validated and *failed*; this one means the executor's contract was *not yet known*, so validation could not run at all. The split exists so an operator can set a different policy (e.g. retry-after-handshake-completes) for a transient visibility gap versus a genuine schema violation.

## When it happens

The node references an executor (`executor:` is non-empty) and declares an `attributes:` block, but the discovery cache holds no `expected_attributes_schema` for that executor at dispatch time. Most commonly: the executor is unreachable or has not completed its first observability handshake yet, so the discovery cache never populated. Also fires when the executor genuinely advertises no attribute schema but the node carries an `attributes:` block — there is no contract to validate against, and the supervisor will not guess one.

(A node with no `executor:` or no `attributes:` block is not gated — the schema-visibility check only applies when both are present.)

## What to do

Confirm the executor is reachable and has completed its handshake: the supervisor populates the discovery cache from the executor's `ExecutorObservability.Capabilities` advertisement. If the executor is up but advertises no schema, either give it an `expected_attributes_schema` (so the node's `attributes:` block has a contract to compose against) or remove the node's `attributes:` block if it genuinely carries no attributes. If the gap is transient (executor still starting), set an `error_types:` policy on the node to retry the class so the dispatch re-attempts once the handshake lands, rather than failing give-up on the first miss.

## See also

- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`../../concepts/discovery-cache.md`](../../concepts/discovery-cache.md)
- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`attribute_validation_failed_at_dispatch.md`](attribute_validation_failed_at_dispatch.md)
- [`../../operator-guide.md#template-registration-and-reference-validation`](../../operator-guide.md#template-registration-and-reference-validation) — the registration-time `templates.ref_validation_mode` knob; the registration-time analog of this dispatch-time gap.
