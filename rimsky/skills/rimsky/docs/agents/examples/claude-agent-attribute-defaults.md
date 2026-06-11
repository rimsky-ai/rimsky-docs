# Attribute defaults are inert in Rimsky

A single-node template whose `attributes:` schema declares one property as a static-default value (a literal `{{...}}` string). Rimsky must NOT substitute the default's value. The executor receives it verbatim. The verification reads the node's persisted attribute bag back and observes the `{{...}}` literal survived dispatch unsubstituted.

This example demonstrates the structural-inertness discipline (see `concept:inertness`) that replaced the retired `userdata` concept under the 2026-05-21 userdata-collapse spec. Pre-collapse, the same demonstration used a `userdata:` block. Post-collapse, the analogous surface is a `default:` value on the unified attribute schema.

**Precondition:** a running rimsky deployment (stand one up from the published images — see the [operator guide](../../operator-guide.md)).

The `stub` executor here is the dockerized test stub executor (a test fixture, not a published image — see [`../../executors/stub/README.md`](../../executors/stub/README.md)). It returns a canned terminal `StreamClose{Success}` (`changed=false`, no attribute writeback) for every dispatch unconditionally, ignoring the request's `attributes` bag and `node_type`. (The `RIMSKY_EXECUTOR_STUB_MODE=1` env var is a separate mechanism on the `http-node` and verifier executors that short-circuits their network paths for testing; the test stub executor does not read it.) Its job here is simply to receive the dispatch and close the stream with a terminal `StreamClose{Success}`. The proof that Rimsky did not substitute the static default is the persisted attribute bag itself: the value dispatched for `prompt` (readable back via `GET /v1/nodes/{id}`, `latest_attributes`) still contains the `{{...}}` literal.

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

After the instance settles, fetch the per-instance event log and look at the `attributes_substituted` event for the `summarize` node. The event's `payload.substituted_fields` records the field names present in the resolved dispatch bag — source-resolved AND default-filled alike — so for this template it lists `prompt` and `model` even though neither was substituted. <!-- @source: lib/runtime/runner_dispatch.go::fieldNames over the resolved dispatch bag --> The event proves a dispatch bag was resolved; it does NOT distinguish defaults from substitutions, which is why the inertness proof in the Verification section reads the persisted *value*, not the event's field list.

```sh
curl "http://localhost:8080/v1/events?instance_id=<instance_id>"
```

Expected: one event of kind `attributes_substituted` for the `summarize` node whose `payload.substituted_fields` contains `prompt` and `model`. The static-default properties flow into the dispatch bag from the schema's `default:` keyword verbatim — never substituted — and are persisted alongside any source-resolved values in `rimsky_node_attributes.data`.

## Verification

The inertness proof: read the node's persisted attribute bag back and confirm the `{{nodes.upstream.attribute.value}}` literal in `prompt` survived dispatch unsubstituted.

```sh
node_id=$(curl -s "http://localhost:8080/v1/instances/<instance_id>/nodes" \
  | jq -r '.nodes[] | select(.node_type=="summarize") | .id')
curl -s "http://localhost:8080/v1/nodes/$node_id" \
  | jq '.latest_attributes.prompt | contains("{{nodes.upstream.attribute.value}}")'
```

Expected output: `true` (the default's value reached the dispatch bag verbatim — Rimsky applied no substitution pass to it; `latest_attributes` is the node's most-recent resolved attribute bag, returned only on the `GET /v1/nodes/{id}` detail read).

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
| `cwd_from_store` | string | `""` | Top-level. Names a store-config entry; the CLI's cwd becomes that store handle's `address` (the filesystem store fills it with an absolute path). Validated as an existing directory before spawn; mismatch ⇒ `agent/attribute_invalid`. |
| `cwd` | string | `""` | Top-level. Raw static-cwd override of last resort; lower priority than `cwd_from_store`. Empty = no override. |
| `cli.bare` | boolean | unset | |
| `cli.permission_mode` | enum `default` \| `acceptEdits` \| `bypassPermissions` \| `plan` | `bypassPermissions` (executor default) | |
| `cli.allowed_tools` | array of string | unset | Unioned with the auto-allows from `cli.mcp_servers` (below). |
| `cli.disallowed_tools` | array of string | unset | |
| `cli.add_dirs` | array of string | unset | |
| `cli.max_budget_usd` | string | unset | |
| `cli.handle_rate_limits` | boolean | `true` | Explicit `false` disables rate-limit auto-park. |
| `cli.max_schema_corrections` | integer ≥ 0 | `3` | Corrective `report_complete` retries on schema-validation failure; exhaustion ⇒ terminal `agent/schema_violation`. |
| `cli.mcp_servers` | array; each entry `{ ref }` (catalog reference) or inline `{ name, url, headers?, allowed_tools? }` | unset | Host-wired MCP servers (see below). Inline entries are accepted only when the executor's `RIMSKY_EXECUTOR_MCP_ALLOW_INLINE` policy is on. |
| `cli.required_signoffs` | array of `{ public_key, path? }` | unset (no gate) | Arms the Ed25519 sign-off gate. |
| `cli.max_signoff_attempts` | integer ≥ 0 | `3` | Sign-off correction budget; exhaustion ⇒ terminal `agent/signoff_unobtained`. |

The schema is open (`additionalProperties: true`): author-declared extension
attributes used purely for inter-node dataflow flow through untouched; the
executor reads only the keys above. `model` carries a schema `default:` —
inert per the demonstration above (rimsky does not substitute it; the schema's
`default:` keyword fills it into the dispatch bag verbatim).

### `cli.mcp_servers` — host-wired MCP servers

Each entry takes one of two shapes:

| Shape | Fields | Accepted when |
| --- | --- | --- |
| Catalog reference | `{ ref }` — `ref` is a non-empty server name in the executor's startup catalog | Always (the default mechanism). The catalog file is loaded once at executor startup from `env:RIMSKY_EXECUTOR_MCP_CATALOG`; each catalog entry declares a `transport` (`http` / `stdio` / `module` / `http-loopback`). An unknown `ref` is rejected as `agent/attribute_invalid`. |
| Inline server | `{ name, url, headers?, allowed_tools? }` — `name` and `url` non-empty; `headers` is string→string; `allowed_tools` is an array of tool names | Only when the executor's `env:RIMSKY_EXECUTOR_MCP_ALLOW_INLINE` policy is truthy (default off in a catalog deployment) — otherwise the entry is rejected as `agent/attribute_invalid` with a message citing `allow_inline`. |

Each resolved server is appended to the spawned CLI's `--mcp-config` so the
agent can actually dial it (a URL only mentioned in a prompt is unreachable —
the CLI speaks MCP only to servers it was configured with). Its tools are
auto-allowed into `--allowedTools`: with no per-server `allowed_tools`, the bare
`mcp__<name>` prefix allows all of that server's tools; an explicit
`allowed_tools` narrows it to the fully-qualified `mcp__<name>__<tool>` names
(for a catalog reference, `<name>` is the `ref`). The same list is re-applied on
resume (`--allowedTools` is process-local invocation config, not session state,
so a resumed dispatch that omitted it could not reach the validators). A
present-but-malformed or forbidden entry (missing/empty `name`/`url`, unknown
`ref`, inline-when-disallowed) is rejected as `agent/attribute_invalid`, never
silently dropped — a dropped server could unwire a validator the sign-off gate
depends on. Header values in `http` server entries (inline or catalog) may use
`${env:VAR}` references; they resolve from the executor's environment at spawn,
so the secret never sits in the template or the persisted attribute bag.

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
