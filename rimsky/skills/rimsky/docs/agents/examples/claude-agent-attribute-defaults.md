# Attribute defaults are inert in Rimsky

A single-node template whose `attributes:` schema declares one property as a static-default value (a literal `{{...}}` string). Rimsky must NOT substitute the default's value. The executor receives it verbatim. The verification observes that Rimsky's `attributes_substituted` event lists only properties with `source:` directives — static-default properties are not substitution targets.

This example demonstrates the structural-inertness discipline (see `concept:inertness`) that replaced the retired `userdata` concept under the 2026-05-21 userdata-collapse spec. Pre-collapse, the same demonstration used a `userdata:` block. Post-collapse, the analogous surface is a `default:` value on the unified attribute schema.

**Precondition:** a running rimsky deployment (stand one up from the published images — see the [operator guide](../../operator-guide.md)).

The bundled `executor-stub` runs in stub mode (`RIMSKY_EXECUTOR_STUB_MODE=1`). The stub ignores attribute values for behavior selection — its job here is simply to receive the dispatch and close the stream with a terminal `StreamClose{Success}`. The proof that Rimsky did not substitute the static default is upstream of the executor: the `attributes_substituted` event in the events log records the fields Rimsky resolved at dispatch.

## 1. The template

Save as `attribute-defaults-demo.yml`:

```yaml
name: attribute-defaults-demo
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: summarize
    executor: stub
    attributes:
      schema:
        type: object
        properties:
          prompt:
            type: string
            default: |
              Summarize the following document.
              Use Markdown formatting where appropriate. Substitute literal text
              like {{nodes.upstream.attribute.value}} into the output if it appears in the
              source, but do not expect Rimsky to have substituted it on input.
          model:
            type: string
            default: "claude-sonnet-4-6"
        additionalProperties: true
```

The `{{nodes.upstream.attribute.value}}` literal in `properties.prompt.default` is intentional — Rimsky does not substitute `default:` values, so the executor sees the literal text in the resolved attribute bag.

## 2. Register, deploy, instantiate

```sh
rimsky template register attribute-defaults-demo.yml
rimsky template deploy sha256-...
rimsky instance create sha256-...
```

## 3. Inspect the dispatch event log

After the instance settles, fetch the per-instance event log and look at the `attributes_substituted` event for the `summarize` node. That event names exactly the attribute schema fields whose `source:` directives Rimsky resolved at dispatch — it does not list static-default properties because static defaults are not substitution targets.

```sh
curl "http://localhost:8080/events?instance_id=<instance_id>"
```

Expected: events of kind `attributes_substituted` list only schema fields with `source:` directives. In this template no field has a `source:`, so `substituted_fields` is empty. The static-default properties (`prompt`, `model`) flow into the dispatch bag from the schema's `default:` keyword verbatim — never substituted, but persisted alongside source-resolved values in `rimsky_node_attributes.data`.

## Verification

```sh
curl -s "http://localhost:8080/events?instance_id=<instance_id>" \
  | jq -r '[.events[] | select(.kind=="attributes_substituted") | .payload.substituted_fields[]] | length'
```

Expected output: `0` (no schema field had a `source:` directive in this template, so substitution touched nothing — and since static-default values are not substitution candidates, the `{{nodes.upstream.attribute.value}}` literal in `properties.prompt.default` was never even a candidate for substitution).

## The `claude-agent` `cli.*` attribute reference

The demonstration above uses the `stub` executor (it only proves inertness). The
real `claude-agent` executor reads a fixed set of per-node attributes to tune the
spawned Claude Code CLI. They live under the node's `attributes.cli` object; the
defaults below are the executor's own (from its
`expected_attributes_schema`, which rimsky merges into the node's effective
attribute schema at template registration). `model` / `system_prompt` /
`user_prompt` are read at the top level of `attributes`, **not** under `cli`.

| Attribute | Shape | Default | Notes |
| --- | --- | --- | --- |
| `model` | string | `claude-sonnet-4-5` | Top-level, not under `cli`. |
| `cli.bare` | boolean | unset | |
| `cli.permission_mode` | enum `default` \| `acceptEdits` \| `bypassPermissions` \| `plan` | `bypassPermissions` (executor default) | |
| `cli.allowed_tools` | array of string | unset | Unioned with the auto-allows from `cli.mcp_servers` (below). |
| `cli.disallowed_tools` | array of string | unset | |
| `cli.add_dirs` | array of string | unset | |
| `cli.max_budget_usd` | string | unset | |
| `cli.handle_rate_limits` | boolean | `true` | Explicit `false` disables rate-limit auto-park. |
| `cli.max_schema_corrections` | integer ≥ 0 | `3` | Corrective `report_complete` retries on schema-validation failure; exhaustion ⇒ terminal `agent/schema_violation`. |
| `cli.mcp_servers` | array of `{ name, url, headers?, allowed_tools? }` | unset | Host-wired external HTTP MCP servers (see below). |
| `cli.required_signoffs` | array of `{ public_key, path? }` | unset (no gate) | Arms the Ed25519 sign-off gate. |
| `cli.max_signoff_attempts` | integer ≥ 0 | `3` | Sign-off correction budget; exhaustion ⇒ terminal `agent/signoff_unobtained`. |

The schema is open (`additionalProperties: true`): author-declared extension
attributes used purely for inter-node dataflow flow through untouched; the
executor reads only the keys above. `model` carries a schema `default:` —
inert per the demonstration above (rimsky does not substitute it; the schema's
`default:` keyword fills it into the dispatch bag verbatim).

### `cli.mcp_servers` — host-wired external MCP servers

Each entry wires one external HTTP MCP server into the spawned CLI:

| Field | Shape | Required | Meaning |
| --- | --- | --- | --- |
| `name` | string, non-empty | yes | MCP server name; becomes the `mcp__<name>` tool prefix. |
| `url` | string, non-empty | yes | HTTP MCP endpoint the CLI dials. |
| `headers` | object (string → string) | no | Per-request headers (e.g. auth). |
| `allowed_tools` | array of string | no | Tool names to allow; absent ⇒ all of the server's tools are auto-allowed. |

Each declared server is appended to the spawned CLI's `--mcp-config` so the
agent can actually dial it (a URL only mentioned in a prompt is unreachable —
the CLI speaks MCP only to servers it was configured with). Its tools are
auto-allowed into `--allowedTools`: with no per-server `allowed_tools`, the bare
`mcp__<name>` prefix allows all of that server's tools; an explicit
`allowed_tools` narrows it to the fully-qualified `mcp__<name>__<tool>` names.
The same list is re-applied on resume (`--allowedTools` is process-local
invocation config, not session state, so a resumed dispatch that omitted it
could not reach the validators). A present-but-malformed entry (missing or
empty `name`/`url`) is rejected as `agent/attribute_invalid`, never silently
dropped.

### The sign-off gate — `cli.required_signoffs` / `cli.max_signoff_attempts`

A non-empty `cli.required_signoffs` arms an Ed25519 sign-off gate: terminal
success is withheld until the agent's `report_complete` carries valid signatures
satisfying every required entry. Each entry is `{ public_key, path? }` — one
valid signature over the value at `path` (dotted path into `attributes_delta`;
omitted ⇒ the whole delta), bound to the dispatch's `dispatch_id`. `public_key`
is mandatory and non-empty. The gate runs after schema validation in
`report_complete`; each unmet round is rejected back to the agent (so it can
re-obtain signatures and retry) until `cli.max_signoff_attempts` (default `3`) is
exhausted, at which point the run terminal-errors with `agent/signoff_unobtained`
(see [`../errors/signoff_unobtained.md`](../errors/signoff_unobtained.md)). The
signer is typically — but not necessarily — a `cli.mcp_servers` entry the agent
dials for signatures. A malformed `required_signoffs` entry (missing
`public_key`) is rejected as `agent/attribute_invalid`, never silently dropped.

These `cli.*` keys validate against the `claude-agent` template schema (rimsky's
`lib/foundation/spec/` template validation merges the executor's
`expected_attributes_schema` at registration).

## See also

- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`../../protocols/executor.md`](../../protocols/executor.md)
- [`../errors/signoff_unobtained.md`](../errors/signoff_unobtained.md)
