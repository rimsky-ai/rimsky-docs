---
error: agent/schema_violation
surfaced_to: executor
---

# Agent schema violation (`agent/schema_violation`)

## What it means

The `claude-agent` reference executor ran a node, the agent emitted `report_complete`, the executor schema-validated the proposed `attributes_delta` against the node's effective schema, validation failed, the executor sent a corrective rejection back to the agent, and after `cli.max_schema_corrections` failed correction rounds the executor commits the terminal `Error{ error_class: "agent/schema_violation" }`. `agent/schema_violation` is one of `claude-agent`'s `declared_error_classes`, so the supervisor's template-policy chain routes it like any other terminal class.

This is the *executor-resident* schema-validation failure — distinct from the supervisor's commit-time schema validation (`attributes_schema_failed`). The executor checks the agent's output BEFORE committing it; the supervisor's check is defense-in-depth on what the executor actually committed. The executor's pre-commit gate is the load-bearing one for an agent-authored delta.

## Governing attributes

Set on the node template under `attributes.cli`:

| Attribute | Shape | Default | Meaning |
| --- | --- | --- | --- |
| `cli.max_schema_corrections` | integer ≥ 0 | `3` | Number of corrective `report_complete` retries the executor permits while the delta keeps failing schema validation. On exhaustion the run terminal-errors with `agent/schema_violation`. |

## When it happens

The agent's `report_complete.attributes_delta` does not validate against the node's **effective** `attributes:` schema — the composition of template-level defaults, the node's per-property `attributes:` block, and the executor's `expected_attributes_schema` (see [`../../concepts/attribute.md`](../../concepts/attribute.md) for the layering rules). Common causes:

- Required field omitted.
- Wrong type (string where number was declared, etc.).
- A field outside the schema's `additionalProperties: false` shape.

The executor rejects the round with the JSON-Schema validation message (so the agent can correct) and the agent retries. The terminal error fires only after `cli.max_schema_corrections` consecutive failed rounds.

## What to do

This is a deliberate gate, not a transient fault — retrying the same dispatch unchanged produces the same result.

- **The agent cannot produce a valid delta against the schema.** Either fix the schema (if the schema is wrong) or fix the prompt/tooling so the agent can produce conforming output. The rejection messages name the failing JSON-Schema path.
- **The correction budget is too tight.** Raise `cli.max_schema_corrections` (default `3`) if legitimate corrections need more rounds.
- **The schema is too strict to be useful.** Loosen the schema where appropriate (e.g. mark optional fields as such, broaden allowed values).

Do not paper over this with an executor-side schema bypass; the gate exists to surface drift early, parallel to the supervisor's commit-time `attributes_schema_failed` (defense-in-depth) and the dispatch-time `template_validation_failed` (input attributes).

## See also

- [`attribute_validation_failed_at_commit.md`](attribute_validation_failed_at_commit.md) — the supervisor-side commit-time class (`attributes_schema_failed`); together with `agent/schema_violation` these form the two-layer schema defense.
- [`signoff_unobtained.md`](signoff_unobtained.md) — the sibling `claude-agent` terminal gate (signatures, after schema passes).
- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`../../protocols/executor.md`](../../protocols/executor.md)
