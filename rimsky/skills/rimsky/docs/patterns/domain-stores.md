---
concept: domain-stores
definition: |
  A pattern for consumer-built MCP servers that hold project-specific state and expose it as a tool catalog an agent executor consumes during a dispatch. As of v0.6.0 the reference claude-agent executor supports this directly: per-node `cli.mcp_servers` attributes wire one or more host-supplied external HTTP MCP servers into a dispatch (appended to the spawned `claude` CLI's `--mcp-config`, tools auto-allowed). The executor's own internal `rimsky-callback` MCP surface is unchanged and is how agent work re-enters the graph.
proto_symbol: (none — MCP, not a rimsky service protocol)
config_field: claude-agent per-node attributes (attributes.cli.mcp_servers, attributes.cli.*); internal rimsky-callback MCP tools
api_surface: (none)
related: [executor, attribute, named-event]
deprecated_terms: []
---

# Domain stores

> **Status (v0.6.0).** Supported, with a caveat about intent. The reference
> `claude-agent` executor **does** wire consumer-built external MCP servers into
> a dispatch — via the per-node `cli.mcp_servers` attribute (each
> `{name, url, headers?, allowed_tools?}`). Each declared server is appended to
> the spawned `claude` CLI's `--mcp-config` and its tools are auto-allowed, so
> the agent can dial it during the run. That is exactly the "domain store as a
> pluggable MCP tool catalog" shape. **The caveat:** in-code, `cli.mcp_servers`
> is framed as wiring *validator* MCP servers that the optional `cli.required_signoffs`
> sign-off gate depends on. Nothing restricts the servers to validators — any
> HTTP MCP server reachable from the executor works as a domain-state catalog —
> but the field was added for the sign-off use case, so rimsky ships no
> operator-managed *catalog* of these servers and no `ref`-based catalog
> resolution. You wire the server list per node, in the node's attributes. This
> page documents both the external-MCP surface (`cli.mcp_servers`) and the
> internal `rimsky-callback` surface, and how an agent node's outputs flow into
> the graph.

A **domain store** is a project-built MCP server holding project-specific state
— prompt context, learnings, examples, corrections, glossaries, evaluation
rubrics, partial pipeline state — exposed as a tool catalog an agent executor
consumes during a dispatch. Rimsky ships no domain store of its own, but the
reference `claude-agent` executor can dial one you build and host: list it under
the node's `cli.mcp_servers` and the agent reaches its tools during the run.

The executor exposes **two** MCP surfaces to a dispatch: its own internal
`rimsky-callback` (always present; how work re-enters the graph) and any
host-wired external servers you declare in `cli.mcp_servers` (zero or more). This
page documents both, then how an agent node's outputs flow downstream.

## Wiring an external domain store (`cli.mcp_servers`)

`@source: lib/services/executors/claude-agent/src/agent-run.ts` (host-server
wiring) and `…/src/server.ts::parseMcpServers` (attribute parsing).

A node's `attributes.cli.mcp_servers` is an array; each entry declares one HTTP
MCP server the agent should be able to reach:

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Server name. Tools surface to the agent as `mcp__<name>__<tool>`. |
| `url` | yes | HTTP MCP endpoint the executor dials. |
| `headers` | no | Flat string→string map sent on the MCP connection (e.g. an auth token). |
| `allowed_tools` | no | Restrict the agent to these tool names. Omitted ⇒ **all** of that server's tools are auto-allowed (the bare `mcp__<name>` server-prefix entry). |

Mechanics that matter:

- Each declared server is appended to the spawned `claude` CLI's `--mcp-config`,
  so the agent can actually dial it. A URL only mentioned in a prompt is
  unreachable — the CLI speaks MCP only to servers it was configured with.
- Tools are **auto-allowed**: with no `allowed_tools` the server-prefix entry
  `mcp__<name>` allows every tool that server exposes; with `allowed_tools` it
  narrows to the fully-qualified `mcp__<name>__<tool>` names.
- The same server list is re-applied on **resume** (after a park), because the
  CLI does not carry `--mcp-config` across `--resume`. A resumed dispatch still
  reaches the domain store.
- Validation is type-shape-only — rimsky never inspects the values — but a
  *present-but-malformed* entry fails the dispatch with terminal error class
  `agent/attribute_invalid` rather than being silently dropped (a dropped server
  could unwire a tool the host intended the agent to use, or a validator the
  sign-off gate depends on).

```yaml
# template node — wire an external domain-store MCP server into the dispatch
nodes:
  - type: triage
    executor: claude-agent
    attributes:
      schema:
        type: object
        properties:
          model:
            type: string
            default: "claude-sonnet-4-5"
          system_prompt:
            type: string
            default: "Use the domain store to resolve project context, then act."
          cli:
            type: object
            default:
              mcp_servers:
                - name: project-context
                  url: "http://project-context-store:8000/mcp"
                  headers:
                    Authorization: "Bearer ${PROJECT_CONTEXT_TOKEN}"
                  # allowed_tools omitted ⇒ all of this server's tools auto-allowed
```

The sign-off lineage shows in the sibling fields: `cli.required_signoffs`
(each `{public_key, path}` must be satisfied by a valid Ed25519 signature in
`report_complete`'s `signoffs` bag before the dispatch resolves to terminal
success) and `cli.max_signoff_attempts`. A validator domain store is the same
mechanism with `required_signoffs` attached; a context/state domain store is the
same mechanism without it.

## The internal callback surface (`rimsky-callback`)

Separate from any external server, the executor hosts exactly one internal HTTP
MCP server named `rimsky-callback`, reached through the `RIMSKY_CALLBACK_URL`
passed into the spawned `claude` process. This is **always** present and is how
the agent's work re-enters rimsky. Its tools:

| Tool | What it does |
| --- | --- |
| `report_complete` | Terminal success, with an optional `attributes_delta` carrying the agent's structured writeback. |
| `report_error` / `report_blocked` | Terminal failure signals. |
| `report_park` | Pause the node (await-callback or snooze). |
| `emit_named_event` | Emit a non-terminal named event (the name must be one the executor declares). |
| `attributes_read` / `attributes_set` | Read the dispatch-time attributes snapshot; persist incremental attribute writes through the supervisor. |

`rimsky-callback` is rimsky's own surface; `cli.mcp_servers` entries are *your*
surfaces. The two coexist in one dispatch.

## How a dispatch is configured

Each dispatch is configured by the node's `attributes`, not by an
operator-managed catalog. Beyond `cli.mcp_servers` above, the executor reads:

- `model`, `system_prompt`, `user_prompt` — the core agent inputs.
- An optional `cli.*` sub-object that tunes the spawned `claude` CLI:
  `cli.bare`, `cli.permission_mode`, `cli.allowed_tools`,
  `cli.disallowed_tools`, `cli.add_dirs`, `cli.max_budget_usd`,
  `cli.handle_rate_limits`, `cli.max_schema_corrections`, and the sign-off
  fields `cli.required_signoffs` / `cli.max_signoff_attempts`. Each maps to a
  `claude` flag (or a recovery behavior); rimsky never inspects the values.

See the [operator guide](../operator-guide.md) for the executor's
startup environment, and
[`docs/agents/examples/claude-agent-attribute-defaults.md`](../agents/examples/claude-agent-attribute-defaults.md)
for a worked example of how attribute defaults flow through a node.

The agent does its work, then calls `report_complete` with an
`attributes_delta` (or makes incremental `attributes_set` calls during
the run). That writeback is what the graph consumes downstream.

## Substituting an agent node's outputs into the graph

When an upstream agent node's result is needed by a downstream node,
expose it through the standard substitution grammar. Two idioms:

- **Through the agent's `report_complete`.** The agent stores its
  findings in `attributes_delta` (validated against the node's
  attributes schema at commit); a downstream node reads them with a
  `source: "{{nodes.<upstream>.attribute.<field-path>}}"` directive on
  one of its own schema properties.
- **Through named events.** If the agent emits non-terminal signals via
  `emit_named_event` (e.g. `ticket.updated`, `learning.captured`), a
  downstream node reads them with
  `source: "{{nodes.<upstream>.event.<name>.<field-path>}}"`.

Pick whichever shape the data fits — `attribute` for terminal writeback,
`event` for mid-flight signals — and the substitution layer treats
them identically once persisted. Both are members of the closed
substitution grammar; see [`concepts/attribute.md`](../concepts/attribute.md).

## Other ways to give an agent project state

Wiring a domain-store MCP server is one route; two others need no external
server at all:

- **Through the prompt and attributes.** Inject context as
  `system_prompt` / `user_prompt` text, or as substituted attribute
  values pulled from upstream nodes, claim payloads, or `params`.
- **Through the agent's own filesystem tools.** The agent runs with the
  `claude` CLI's built-in tools (`Read`, `Grep`, etc., gated by
  `cli.allowed_tools`) over its working directory and any `cli.add_dirs`
  paths — so project state staged on a shared volume is reachable.

Reach for `cli.mcp_servers` when the project state is large, dynamic, or already
exposed as a service; reach for prompt/attribute injection or filesystem tools
when it is small or static.
