# Control-API MCP surface

The rimsky control-API exposes its operational surface as an MCP (Model Context Protocol) tool catalog. This is **not** a standalone sidecar binary — it is a JSON-RPC protocol skin mounted directly inside the control-API process at `POST /mcp` (wired in `lib/control/controlapi/mcp_route.go`; the catalog and JSON-RPC envelope live in `lib/control/controlapi/mcp/`). An agentic client speaks MCP to the same process and port that serves the HTTP API, and every tool re-enters the control-API's own routing and auth pipeline.

## When to use it

- You want an LLM agent to drive rimsky operationally (register a template, create an instance, inspect held frames, resume a parked node) without writing a custom MCP server per integration.
- You want the agent to see exactly the operations its API key is granted — `tools/list` is filtered per-key.

## How it is mounted

`POST /mcp` is gated by the `mcp:read` umbrella action, which the bundled `read-only` role covers via wildcard `*:read` (the `debug-operator` and `agent-supervisor` bundled roles also carry `*:read`). `initialize` and `tools/list` run inside the MCP server and never reach a tool. A `tools/call` re-enters the chi router through the catalog, so the call picks up the **per-tool action gate** there — a mutating tool still requires the matching write permission (e.g. `instance_create` requires `instance:create`).

There is no separate MCP config block, no `CONTROL_API_URL` / `CONTROL_API_TOKEN` / `BIND_ADDR` / `PORT` environment surface, and no second process to deploy. The MCP surface inherits the control-API's bind address, TLS, and auth.

## Auth

Same model as the HTTP API. A request carries `Authorization: Bearer <plaintext API key>`; the auth middleware resolves it to an identity with a set of permission grants (or a synthetic anonymous identity when the deployment runs in anonymous mode). Two consequences for MCP:

- `tools/list` returns only the tools whose backing action the key is granted (`auth.CheckGrant` per tool). An agent never sees tools it cannot call.
- `tools/call` is gated again at dispatch by the per-tool action; a permission denial surfaces as a JSON-RPC error.

## Wire protocol

MCP **Streamable HTTP** transport over `GET`+`POST /mcp`, advertising `protocolVersion` `2025-06-18` and `serverInfo` `{name: "rimsky-control-api", version: "v1"}`. This is the transport the default MCP `type: http` client speaks; the server implements just enough of it to **connect and control** (server-initiated push is V2 — see [Streaming](#streaming)).

`POST /mcp` carries one JSON-RPC 2.0 message per request. Five methods:

- `initialize` — returns `protocolVersion`, `capabilities` (`tools: {}`, `resources: {subscribe: false, listChanged: false}`), and `serverInfo`. The response also sets a fresh `Mcp-Session-Id` header; the client echoes it on subsequent requests and binds its `GET` stream to it. v1 holds no per-session state — the id is opaque and unvalidated beyond connect.
- `tools/list` — the per-key-filtered tool catalog.
- `tools/call` — dispatches by tool name; arguments are JSON-Schema validated; the result is returned as an MCP `content` text block wrapping the control-API JSON response.
- `resources/list` / `resources/read` — the read-only resource catalog (empty on tools-only deployments).

A JSON-RPC **notification** (an id-less message, e.g. `notifications/initialized`, the post-`initialize` handshake step) is consumed with `202 Accepted` and an empty body — never a JSON-RPC reply. Replying to a notification is a JSON-RPC 2.0 violation, and the `type: http` client treats a spurious reply as a handshake failure. Any unknown method on an `id`-bearing request returns `-32601`.

Error codes follow JSON-RPC: `-32700` parse error, `-32600` invalid request, `-32601` method/tool not found, `-32602` invalid params, `-32603` internal error.

## Streaming

`GET /mcp` opens the MCP Streamable HTTP **server-to-client stream** — a `text/event-stream` (SSE) the default `type: http` client probes on connect. In v1 the stream is **idle**: it flushes `200` + the SSE headers immediately so the probe succeeds, then emits only periodic keep-alive comments (`: keep-alive`, ~25s) to hold the connection open. It pushes **no** MCP messages. (An earlier build answered this probe with `405`, failing the client's connect; the idle stream replaces that.)

Server-initiated push — `resources/subscribe` + `notifications/resources/updated`, live event streaming — is **out of v1 scope**: connect-and-control only, live push is V2. The `GET` stream exists to satisfy the transport probe, not to deliver data. If a request carries an `Mcp-Session-Id` header the stream echoes it back; otherwise the stream is unbound. Any HTTP method other than `GET`/`POST` returns `405` with `Allow: GET, POST`.

## Tool catalog

Tools are declared in the canonical action registry (`lib/control/controlapi/actions.go`); each maps to one control-API action and its HTTP route(s). The catalog is the source of truth — the list below is grouped for orientation, not hand-maintained field-by-field.

- **Instances:** `instance_list`, `instance_get`, `instance_create`, `instance_terminate` (graceful, `DELETE /instances/{idOrKey}`), `instance_kill` (force-terminate — mark terminal and abandon in-flight node-runs, `POST /instances/{idOrKey}/terminate`), `instance_pause`, `instance_resume`.
- **Breakpoints:** `breakpoint_list`, `breakpoint_create`, `breakpoint_resume_hit`, `breakpoint_delete`.
- **Templates:** `template_list`, `template_get`, `template_validate` (read action; `POST /templates/validate`, gated by `template:validate`), `template_register`, `template_deploy`, `template_undeploy`, `template_deregister`.
- **Tags:** `tag_list`, `tag_create`, `tag_set`, `tag_delete`.
- **Nodes:** `node_list`, `node_get`, `node_invalidate` (resumes a parked node or marks a node stale and re-fires; backed by `POST /nodes/{id}/invalidate` and the admin route), `node_reset` (reset a failed node back to stale).
- **Messages:** `message_send`, `message_list`, `message_get`.
- **Events:** `event_list`.
- **Audit:** `audit_list` — read the auth audit log (`GET /audit`). Gated by `audit:read`, granted separately from `event:read` because actor identity / IP / user-agent / action are sensitive.
- **Lineage:** `lineage_get`, `lineage_prune`.
- **Backfills:** `backfill_create`, `backfill_list`, `backfill_get`, `backfill_partitions`, `backfill_cancel`.
- **Assets:** `asset_list`, `asset_get`, `asset_versions`, `asset_materialization_history`, `asset_materialize`, `asset_delete`.
- **Diagnostics:** `parked_node_list`, `waitset_list`, `claim_holders_list`, `held_frames_list`.
- **Auth (self-administration):** `auth_list`, `auth_get`, `auth_status`, `auth_create_key`, `auth_revoke_key`, `auth_rotate_key`.

`instance_create` accepts `{template, instance_key?, params?, attribute_overrides?, frame_delivery_mode?}` (`frame_delivery_mode` ∈ `serial_queue` / `coalesce`). `attribute_overrides` mirrors the control-API's per-instance overrides surface (`{by_executor, by_node, by_match}`); `by_match` is an ordered list of `{matcher, overlay}` entries keyed on a content predicate (`node_type`, `executor`, `graph`, `child_key`, `attrs.<path>`) — see the [attribute concept](../../concepts/attribute.md).

## Dry-run

Dry-run is a per-request modifier, not a separate tool or grant: the auth middleware reads a `?dry_run=true` query flag and tags the request as `dry_run` mode. A mutating handler in that mode runs full validation, returns a synthetic `{ dry_run: true, would_have_X: ... }` envelope, and skips the state mutation. Reads run normally (there is nothing to skip). Because a `tools/call` re-enters the chi router, an MCP tool inherits the same behavior when its forwarded request carries the flag — the call still requires the matching write grant, it just doesn't commit.

Every request (including dry-run) is recorded in the durable audit log, which captures actor identity, IP, user-agent, the action, the resolved mode, and the protocol skin (`mcp` for tool calls vs the HTTP API otherwise). Read the audit log with the `audit_list` tool (`GET /audit`, gated by `audit:read`).

## Security

- The MCP surface does not add its own auth layer: it is gated by the control-API's API-key permission model, the same as every HTTP route.
- `tools/call` mutations require the corresponding write grant; a read-only key sees and can invoke only read tools.
- Tool arguments are JSON-Schema validated before dispatch; malformed params return JSON-RPC `-32602`.
