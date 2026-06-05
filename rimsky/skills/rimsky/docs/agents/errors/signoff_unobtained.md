---
error: signoff_unobtained
surfaced_to: executor
---

# Sign-off gate unmet (`agent/signoff_unobtained`)

## What it means

The `claude-agent` reference executor ran a node configured with a sign-off gate (`cli.required_signoffs`) and the agent never produced a terminal `report_complete` whose output carried the required valid Ed25519 signatures within the gate's attempt budget. The dispatch resolves to a terminal `Error{ error_class: "agent/signoff_unobtained" }` — the gRPC `StreamClose{Error}` outcome, or the `error` outcome on the async-callback body. `agent/signoff_unobtained` is one of `claude-agent`'s `declared_error_classes`, so the supervisor's template policy chain maps it like any other terminal `error_class`.

The gate is a security control: when a host wires `cli.required_signoffs`, output cannot resolve to terminal success until an out-of-band validator has signed the bound value. This error is what fires when that condition is not met — it fails loud rather than letting unsigned output through.

## Governing attributes

Set on the node template under `attributes.cli`:

| Attribute | Shape | Default | Meaning |
| --- | --- | --- | --- |
| `cli.required_signoffs` | array of `{ public_key, path? }` | none (no gate) | Each entry requires one valid Ed25519 signature over the value at `path` (dotted path into `attributes_delta`; omitted ⇒ the whole delta), bound to the dispatch. A non-empty list arms the gate. `public_key` is mandatory and non-empty. |
| `cli.max_signoff_attempts` | integer ≥ 0 | `3` | Corrective `report_complete` retries allowed while the gate is unmet, mirroring `cli.max_schema_corrections`. On exhaustion the run terminal-errors with `agent/signoff_unobtained`. |

The validator the agent obtains signatures from is typically a host-wired external MCP server declared under `cli.mcp_servers` (so the agent can actually dial it), but the gate itself only checks signatures — it does not require the signer to be an `mcp_servers` entry.

## When it happens

The gate runs in the executor's `report_complete` handler, as the second sequential layer **after** schema validation passes (get the shape right, then get it signed). Two paths produce `agent/signoff_unobtained`:

1. **Budget exhausted.** Each `report_complete` whose `signoffs` bag fails to satisfy every required `{ public_key, path }` is rejected back to the agent (with the unmet `path:reason` list) so the agent can re-obtain signatures and retry. After more than `cli.max_signoff_attempts` failed rounds the run commits the terminal error. An entry is unmet as `missing` when the signature bag is empty, else `invalid` (no signature in the bag Ed25519-verifies under that key, or the configured `public_key` cannot be parsed).
2. **Gate armed but unbindable.** A non-empty `cli.required_signoffs` with an empty dispatch `dispatch_id` cannot be bound or verified (signatures bind to the dispatch id; an empty binding id has no honest signature). This is treated as a configuration/usage error and terminal-errors immediately with `agent/signoff_unobtained` rather than running silently ungated.

The gate's `required` list (the `{ public_key, path }` entries) is taken from the dispatch-time `cli.required_signoffs`, never re-read from `attributes_delta` — so a gated agent cannot weaken its own gate by emitting a `cli.required_signoffs` override in its delta. Each signature itself binds to `domain ‖ dispatch_id ‖ canonical(value-at-path)`: the `dispatch_id` makes a signature valid for only the one real dispatch (anti-replay), and the signed bytes cover the gate's bound value-at-path drawn from `attributes_delta` (not the `required` list).

A separate failure mode is **not** this error: a `cli.required_signoffs` (or `cli.mcp_servers`) entry that is *present but malformed* — e.g. an entry missing `public_key` — is rejected up front as `agent/attribute_invalid` (it never reaches the gate), because silently dropping a malformed gate entry would disable the gate. See [`attribute_validation_failed_at_commit.md`](attribute_validation_failed_at_commit.md) for the related attribute-shape failure mode.

## What to do

This is a deliberate gate, not a transient fault — retrying the same dispatch unchanged produces the same result. Resolve by addressing whichever condition is true:

- **The agent could not obtain valid sign-offs.** Confirm the validator the agent dials (its `cli.mcp_servers` entry) is reachable and is producing signatures over the same value the gate checks — `domain ‖ dispatch_id ‖ canonical(value-at-path)`, where the domain is `rimsky/claude-agent/signoff/v1`, `dispatch_id` is the raw `ExecuteRequest.dispatch_id`, and `path` is the gate's configured path. A `path`/value mismatch yields `invalid`; an unreachable validator yields `missing`.
- **The budget is too tight for the workflow.** Raise `cli.max_signoff_attempts` (default `3`) if legitimate sign-off rounds need more correction passes.
- **The key or path is misconfigured.** A `public_key` that cannot be parsed is always `invalid`. Verify the configured PEM and the `path` match the validator's signing scheme.
- **The gate is unintended.** If the node should not be gated, remove `cli.required_signoffs` from the template.
- **`dispatch_id` was empty.** A gated dispatch with no `dispatch_id` is a usage error — ensure the caller supplies a non-empty `dispatch_id` on the `Execute` request.

The `claude-agent` reference executor (in rimsky-core under `lib/services/executors/claude-agent/`, sign-off gate in `src/signoff.ts` + the `report_complete` handler in `src/agent-run.ts`) and its test suite (`src/signoff-gate.e2e.test.ts`) cover the exact gate semantics and the terminal-error wire shape; align with that.

## See also

- [`../../concepts/signal.md`](../../concepts/signal.md)
- [`../../concepts/executor.md`](../../concepts/executor.md)
- [`../../protocols/executor.md`](../../protocols/executor.md)
