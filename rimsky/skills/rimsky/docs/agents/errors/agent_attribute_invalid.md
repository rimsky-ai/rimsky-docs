---
error: agent/attribute_invalid
surfaced_to: operator
---

# claude-agent attribute invalid (`agent/attribute_invalid`)

## What it means

The `claude-agent` reference executor rejected a dispatch up front because the node's attribute bag violates the executor's attribute contract. The CLI subprocess was never spawned (or, for host-server wiring failures, never reached the work); the dispatch resolves to a terminal `Error{ error_class: "agent/attribute_invalid" }`, routable through the node's `error_types:` policy. The payload carries the defect as `reason`, `error`, or `errors` (an array of validation strings) depending on the trigger site.

This is the fail-loud guard for security-relevant configuration: a present-but-malformed `cli.mcp_servers` or `cli.required_signoffs` entry is **rejected, never silently dropped** — a dropped server could unwire a validator the sign-off gate depends on. <!-- @source: lib/services/executors/claude-agent/src/cli-config-error.ts -->

## When it happens

Deterministic template/configuration errors — retrying the same dispatch unchanged reproduces them: <!-- @source: lib/services/executors/claude-agent/src/agent-run.ts, lib/services/executors/claude-agent/src/server.ts::parseMcpServers, ::parseRequiredSignoffs -->

| Trigger | Condition |
| --- | --- |
| Malformed attribute bag | The top-level attributes fail the executor's malformed-shape check at dispatch entry |
| Invalid `attributes.schema` | The node's effective attribute schema does not compile (AJV rejects it); payload `errors` lists the compile errors |
| `cwd_from_store` / `cwd` invalid | `cwd_from_store` names a store handle absent from the dispatch's `stores`, or the resolved path (or the raw `cwd` override) does not exist or is not a directory |
| Malformed `cli.mcp_servers` | Not an array; an entry not an object; a catalog `ref` missing/empty or unknown in the startup catalog; an inline entry missing a non-empty `name`/`url`; an inline entry when `RIMSKY_EXECUTOR_MCP_ALLOW_INLINE` is off |
| Malformed `cli.required_signoffs` | Not an array; an entry not an object; an entry missing a non-empty `public_key` |
| `stub_response` malformed | In stub mode, `stub_response` present but not a JSON object |

## What to do

Fix the template's `attributes:` (or the executor's MCP catalog / inline policy) — the payload message names the offending key. The full `cli.*` attribute reference, including the two `cli.mcp_servers` entry shapes and the sign-off gate's entry shape, is in [`../examples/claude-agent-attribute-defaults.md`](../examples/claude-agent-attribute-defaults.md).

What this is **not**: a well-formed gate whose signatures are never satisfied is [`signoff_unobtained.md`](signoff_unobtained.md); an agent-proposed `attributes_delta` failing schema validation is [`agent_schema_violation.md`](agent_schema_violation.md); supervisor-side commit validation is [`attribute_validation_failed_at_commit.md`](attribute_validation_failed_at_commit.md).

## See also

- [`../examples/claude-agent-attribute-defaults.md`](../examples/claude-agent-attribute-defaults.md) — the `cli.*` attribute reference.
- [`signoff_unobtained.md`](signoff_unobtained.md) · [`agent_schema_violation.md`](agent_schema_violation.md) — the adjacent claude-agent gate failures.
- [`../../concepts/attribute.md`](../../concepts/attribute.md)
