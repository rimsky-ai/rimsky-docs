---
concept: domain-stores
definition: |
  A would-be pattern for consumer-built MCP servers that hold project-specific state and expose it as a tool catalog an agent executor consumes during a dispatch. Not wired in v0.4.0: the reference claude-agent executor consumes no external MCP servers. The real, narrower surface is the executor's own internal rimsky-callback MCP tools plus per-node attributes.
proto_symbol: (none — MCP, not a rimsky service protocol)
config_field: claude-agent per-node attributes (attributes.cli.*); internal rimsky-callback MCP tools
api_surface: (none)
related: [executor, attribute, named-event]
deprecated_terms: []
---

# Domain stores

> **Status (v0.4.0).** The reference `claude-agent` executor does **not**
> consume consumer-built MCP servers. It wires exactly one MCP server
> into each dispatch — its own internal `rimsky-callback` — and reads its
> per-dispatch configuration from the node's attributes. There is no
> operator catalog of external MCP servers, no `ref`-based catalog
> resolution, and no way for a template to register an additional MCP
> server for a dispatch to reach. This page describes the real surface
> the executor offers today and how an agent node's outputs flow into the
> graph. The broader "domain store as a pluggable MCP tool catalog"
> pattern is aspirational and not implemented.

Rimsky ships no domain store, and the reference `claude-agent` executor cannot
dial a consumer-built MCP server. What it *does* expose today is one internal MCP
surface plus per-node attributes — this page documents that real surface and how
an agent node's outputs flow into the graph.

The aspirational pattern (per the status note above): a **domain store** would be
a project-built MCP server holding project-specific state — prompt context,
learnings, examples, corrections, glossaries, evaluation rubrics, partial pipeline
state — exposed as a tool catalog an agent executor consumes during a dispatch.
Rimsky neither ships one nor wires one in.

## What the executor actually exposes

The `claude-agent` reference executor (in-tree at
`lib/services/executors/claude-agent/`) gives a dispatch exactly one MCP
surface: an internal HTTP MCP server named `rimsky-callback` that the
executor hosts itself. The agent reaches it through the
`RIMSKY_CALLBACK_URL` passed into the spawned `claude` process. Its tools:

| Tool | What it does |
| --- | --- |
| `report_complete` | Terminal success, with an optional `attributes_delta` carrying the agent's structured writeback. |
| `report_error` / `report_blocked` | Terminal failure signals. |
| `report_park` | Pause the node (await-callback or snooze). |
| `emit_named_event` | Emit a non-terminal named event (the name must be one the executor declares). |
| `attributes_read` / `attributes_set` | Read the dispatch-time attributes snapshot; persist incremental attribute writes through the supervisor. |

That callback surface is how the agent's work re-enters rimsky. There is no
second, consumer-supplied MCP server in the picture.

## How a dispatch is configured

Each dispatch is configured by the node's `attributes`, not by an
operator-managed catalog. The executor reads:

- `model`, `system_prompt`, `user_prompt` — the core agent inputs.
- An optional `cli.*` sub-object that tunes the spawned `claude` CLI:
  `cli.bare`, `cli.permission_mode`, `cli.allowed_tools`,
  `cli.disallowed_tools`, `cli.add_dirs`, `cli.max_budget_usd`,
  `cli.handle_rate_limits`, `cli.max_schema_corrections`. Each maps to a
  `claude` flag (or a recovery behavior); rimsky never inspects the
  values.

See the [operator guide](../operator-guide.md) for the executor's
startup environment, and
[`docs/agents/examples/claude-agent-attribute-defaults.md`](../agents/examples/claude-agent-attribute-defaults.md)
for a worked example of how attribute defaults flow through a node.

```yaml
# template node — claude-agent inputs via attribute defaults
nodes:
  - type: ingest
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
            default: "Summarize the input and report your findings."
          cli:
            type: object
            default:
              allowed_tools: ["Read", "Grep"]
```

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

## If you need external state today

Because the reference executor cannot dial a consumer MCP server, the
practical ways to give an agent project-specific state in v0.4.0 are:

- **Through the prompt and attributes.** Inject context as
  `system_prompt` / `user_prompt` text, or as substituted attribute
  values pulled from upstream nodes, claim payloads, or `params`.
- **Through the agent's own filesystem tools.** The agent runs with the
  `claude` CLI's built-in tools (`Read`, `Grep`, etc., gated by
  `cli.allowed_tools`) over its working directory and any `cli.add_dirs`
  paths — so project state staged on a shared volume is reachable.

Bringing a true external MCP tool catalog into a dispatch would require
new wiring in the reference executor that does not exist as of v0.4.0.
