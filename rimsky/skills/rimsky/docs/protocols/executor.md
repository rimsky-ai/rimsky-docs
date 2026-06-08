# Implementing an executor

> **Version.** The API on this page targets the rimsky release this corpus is
> reconciled against (`reconciledAgainst` in `.claude-plugin/plugin.json`). For
> runnable, version-pinned code, copy the executor at
> [`../examples/executor/`](../examples/README.md) — its `go.mod` states the
> exact `lib/protocols` tag.

An **executor** runs one node's work. It implements the dispatch protocol
`Executor` (one method, `Execute`) and, optionally, the read-only
`ExecutorObservability` protocol. It is out-of-process: the supervisor dials it
over gRPC at dispatch time, with an HTTP+JSON bridge available for non-Go
services.

There is **no executor SDK** — implement against the wire types in any language. A
Go service may use the `protocols` module's `serverkit` package
([`go-packages.md`](go-packages.md)) for **generic gRPC server-lifecycle helpers**
(`serverkit.Listen` / `serverkit.RunGRPC` / `serverkit.GracefulStop`) to stand up
the `Executor` gRPC server. `serverkit` does **not** carry an executor-specific
helper: there is no dispatch handler, no async-callback POST client, and no
incremental-attribute-write helper in it (its HTTP+JSON bridges cover the
claim-producer / lifecycle-subscriber surfaces, not the executor). The dispatch
handler, the async-callback POST, and incremental attribute writes are yours to
write straight against the wire contract — the in-tree `http-node` executor does
exactly that with `genv1.RegisterExecutorServer` plus a plain `http.Client`. Wire
contracts: `lib/protocols/proto/v1/executor.proto` (dispatch, required) and
`lib/protocols/proto/v1/executor_observability.proto` (observability, optional);
generated field/message/RPC references at
[`reference/executor.md`](reference/executor.md) and
[`reference/executor-observability.md`](reference/executor-observability.md).

<!-- @source: ../../.ok-planner/design/concepts/executor.md -->

## What you implement

| RPC | Service | Required? | Purpose |
| --- | --- | --- | --- |
| `Execute(ExecuteRequest) → stream<ExecuteEvent>` | `Executor` | **Yes** | Run one dispatch; stream events ending in exactly one terminal. |
| `Capabilities(ExecutorCapabilitiesRequest) → ObservabilityCapabilities` | `ExecutorObservability` | No | Startup handshake: declare accepted schema, events, error classes, trace support. |
| `GetTrace(GetTraceRequest) → Trace` | `ExecutorObservability` | No | Pull a past dispatch's trace, keyed by `dispatch_id`. |
| `StreamTrace(StreamTraceRequest) → stream<TraceEvent>` | `ExecutorObservability` | No | Live trace of a dispatch, keyed by `dispatch_id`. |

`ExecutorObservability` is opt-in but recommended for any executor whose
dispatches are interesting to humans — dashboards dial it to pull or stream
per-dispatch traces.

## Boundaries

The executor **owns**:

- Running the dispatch and streaming `ExecuteEvent`s.
- Classifying its own outcome — `Success` / `Error` / `Park` / `AwaitAsyncCallback`.
- Declaring its accepted attribute shape, emittable events, and error classes via
  `Capabilities`.

The executor does **NOT** own (rimsky's job):

- **The resolution of an error.** The executor emits an `error_class`; the
  supervisor's template policy chain maps `(error_class, retry_counter)` to
  `retry` / `give_up` / `pass`. The executor never decides retry-vs-give-up.
- **Attribute substitution.** `{{...}}` directives are resolved by the supervisor
  *before* dispatch; the executor receives resolved values.
- **Cascade coupling.** Whether a downstream node reacts to this node's success or
  failure is declared receiver-side, not by the executor.
- **Credentials, encryption, access control.** Rimsky is auth-blind. Encrypt
  sensitive bytes before handing them to rimsky; service-to-service auth is
  operator-configured at the deployment layer (mTLS, IAM). Encrypted attribute
  values stay encrypted in transit — decrypting at point of use is the executor's
  job.

## `Execute` — the dispatch

The supervisor dials the executor and streams events back. The response stream is
zero or more `Heartbeat` / `NamedEvent` records (in any order) followed by
**exactly one** terminal `StreamClose`; the executor MUST close the stream
immediately after emitting it. A stream that closes without a `StreamClose` is an infrastructure
error to the supervisor.

### `ExecuteRequest` (salient fields)

Full field reference: [`reference/executor.md`](reference/executor.md).

| Field | Type | Meaning |
| --- | --- | --- |
| `node_id`, `instance_id`, `node_type` | string | Which dispatch this is. |
| `attributes` | `Struct` | The **only** template-author input surface, already substituted. `attributes_schema` carries the declared JSON Schema. The historical `userdata` field was collapsed into `attributes` and is reserved on the wire. |
| `stores` | `map<string, StoreHandle>` | One entry per referenced store, keyed by store-config name. Each `StoreHandle` carries the producer's `handle` (the `Address` bytes from `ClaimProducer.Open`, wrapped as a `Struct`), a `kind` string, and `candidate_handle` bytes for `DataProcessing` fan-out leaves. All inert to rimsky. |
| `callback_url`, `cancel_token` | string | HTTP+JSON base URL for async handoff and incremental attribute writes; the bearer token the supervisor watches for cancellation (also used on those callbacks). |
| `dispatch_id` | string | The `rimsky_node_runs.id`; key per-dispatch traces/state on it. |
| `resume_context` | `ResumeContext` | Populated on resume of a parked node (see [Resume context](#resume-context)); absent on a fresh dispatch. |
| `prior_dispatch_id`, `prior_dispatch_disposition` | string, enum | Set when this dispatch supersedes a prior failed / stale / recalculated one for the same `(run_scope_id, node_id)`. Disposition (`PRIOR_HEARTBEAT_STALE` / `PRIOR_RETRY_AFTER_ERROR` / `PRIOR_RECALCULATE`; `PRIOR_NONE` when unset) tells a session-keeping executor *why* it is taking over. |
| `run_scope_id` | string | The run-scope this dispatch lives in. Opaque to in-process executors. Only meaningful to the host-agent-proxy, which keys per-run-scope spawn isolation on it (one spawned child per `(run_scope_id, binding)`, reaped at run-scope termination) so concurrent run-scopes of a fanned-out instance get distinct late-bound children. Ignore unless you are writing a forwarder. |

### `ExecuteEvent` records

| Record | Terminal? | Fields | Notes |
| --- | --- | --- | --- |
| `Heartbeat` | no | `timestamp_ms`, `note` | Keep-alive while work continues. |
| `NamedEvent` | no | `name`, `payload` (bytes) | Non-terminal domain signal. `name` must appear in `Capabilities.declared_events`; `payload` is opaque to rimsky, reachable in substitution as `nodes.<emitter>.event.<name>.<path>`. Zero or more per run. |
| `StreamClose` | **yes — exactly one** | `oneof outcome` | One of the four outcomes below. |

`StreamClose.outcome` is exactly one of:

| Outcome | Fields | Meaning |
| --- | --- | --- |
| `Success` | `bool changed`, `string change_summary`, `Struct attributes_delta` | Terminal success. `changed = false` halts cascade propagation at this node. `change_summary` is audit-only (rimsky does not parse it). `attributes_delta` is the terminal write-back (validated against the node's attributes schema); may be empty if the incremental-callback path was used during the run. |
| `Error` | `string error_class`, `Struct payload` | Terminal application-level error. `error_class` is executor-defined; `payload` is opaque. The **supervisor**, not the executor, maps it to a resolution (see [Boundaries](#boundaries)). Use `error_class: "executor_blocked"` for "I produced output but explicitly declined to claim success" — low-confidence outputs routed to human review. |
| `Park` | `ParkReason reason`, `bytes payload`, `Timestamp resume_at?`, `string session_token?`, `reason_note` / `reason_label?` | Pause this run until resumed. `reason` ∈ `PARK_REASON_AWAIT_CALLBACK` (the zero value; will not auto-resume) / `PARK_REASON_SNOOZE`. `payload` and `session_token` are echoed back in `ResumeContext`. `resume_at` absent ⇒ signal-based resume only. See [parked-state](../concepts/parked-state.md). |
| `AwaitAsyncCallback` | `string async_ack_id`, `int expected_completion_ms?` | Terminal handoff: the outcome arrives later via HTTP callback (see [Async callback](#async-callback)). Echo `async_ack_id` on the callback. |

`AwaitAsyncCallback` vs `Park{PARK_REASON_AWAIT_CALLBACK}`: with `AwaitAsyncCallback`
the **executor** finishes the run by POSTing the outcome (see
[Async callback](#async-callback)); with `Park` the **node** is suspended until an
external resume — elapsed time, an admin invalidate, or an in-graph `on_event`
invalidate — after which the supervisor re-dispatches it with `resume_context`.

## The attribute surface

<!-- @source: ../../.ok-planner/design/concepts/attribute.md -->

`attributes` is the single template-author-supplied input to a dispatch. There is
no peer "opaque" surface — the historical `userdata` field was collapsed into it.

- Every key the executor consumes — model name, system prompts, transport config —
  appears under `attributes` and is governed by the node's attributes schema.
- The executor declares the shape it accepts via `expected_attributes_schema` on
  the `Capabilities` response. Rimsky merges that with the template's **L1
  defaults** and **L2 per-node declaration** into the effective schema, then
  validates the post-substitution bag at dispatch and the post-write-back bag at
  commit. Validation failures route through
  `Error { error_class: "template_validation_failed" }`.
- Static configuration (constants the template author hands the executor) lives in
  attribute `default:` entries; dynamic configuration (values pulled from other
  nodes or params) lives in `source:` entries. Both reach the executor identically.

## `Capabilities` — startup handshake

Probed once per service at startup. The `ObservabilityCapabilities` response
declares:

| Field | Meaning |
| --- | --- |
| `supports_trace_get` / `supports_trace_stream` | Which read-side RPCs the executor honors. |
| `retention_after_terminal_seconds` | Per-dispatch trace retention window. |
| `custom_ui` (`CustomUI`) | Optional dashboard-embeddable UI (`ui_url`, `embed_mode`, `dispatch_url_template`). |
| `http_bridge_url` | Non-empty ⇒ base URL of the HTTP+JSON observability bridge for browser clients; empty ⇒ gRPC-only. |
| `expected_attributes_schema` | JSON Schema for the accepted attribute shape; empty ⇒ accept-any. Output properties are marked `readOnly: true`. |
| `declared_events` | Event names the executor may emit via `NamedEvent`; empty ⇒ none. Cross-validated against template `subscribes: [{type: event/<name>}]` entries — references to undeclared events reject the registration. |
| `declared_error_classes` | Error-class paths the executor may emit on `Error.error_class`. Patterns ending in `*` are prefix leaves (e.g. `http/server_error/*`); empty ⇒ skip the validator's range-check. Validated against operator `error_types:` keys. |

## Async callback

For work that outlives a streaming RPC — background jobs, async LLM calls, long
batches — close `Execute` with `StreamClose{AwaitAsyncCallback}` carrying an
`async_ack_id`, then POST the outcome back when the work completes:

```
POST ${callback_url}/v1/callback/{async_ack_id}
Content-Type: application/json

{
  "events": [ { "name": "phase_complete", "payload": "..." } ],
  "success": { "changed": true, "attributes_delta": { ... } }
}
```

The body is the `AsyncCallbackBody` message marshalled as JSON: an optional
`events` array (a `NamedEvent` stream replayed *before* the verdict, so an
`on_event` handler can fire mid-flight) plus exactly one `outcome` oneof —
`success`, `error`, or `park`.

Wire details that bite:

- The path is `${callback_url}/v1/callback/{async_ack_id}` — `callback_url` from
  the `ExecuteRequest` plus the `async_ack_id` you echoed.
- The body MUST parse as `AsyncCallbackBody`. The pre-2026 legacy
  `{ "type": "complete" | "blocked" | "errored" }` shape is **rejected** (HTTP 400).
- `AwaitAsyncCallback` is **not** a valid `outcome` here — the callback is the
  second half of the async path, so chaining another handoff is forbidden.
- The bearer token is the same `cancel_token` from the `ExecuteRequest` (also used
  on incremental attribute writes).

There is no Go helper for this POST — a Go executor marshals `AsyncCallbackBody`
and POSTs it with a plain `http.Client` (the in-tree `http-node` executor does
exactly that), the same as a non-Go executor marshalling the shape directly.

**Incremental attribute writes.** An executor may also write attributes *mid-run*,
before the terminal, against the same `callback_url` base with the `cancel_token`
as bearer (the route and body are in [`reference/executor.md`](reference/executor.md)).
This too is a plain POST you write yourself — no `serverkit` helper. A run that wrote
incrementally may then close `Success` with an empty `attributes_delta`.

## Resume context

When the supervisor resumes a parked node, `ExecuteRequest.resume_context` carries:

| Field | Meaning |
| --- | --- |
| `bytes payload` | The original `Park.payload`. |
| `string session_token` | The original `session_token` (executor-side correlation identifier). |
| `string resume_reason` | `"deadline_elapsed"` (time-based via `resume_at`) or `"external_invalidate"` (admin or in-graph invalidate). |

Empty `resume_context` ⇒ this is a fresh dispatch. Executors that do not implement
parking can ignore the field.

## Conformance

`rimsky conformance executor` exercises an executor against the wire-protocol
contract:

```
rimsky conformance executor --endpoint <your-executor-host:port> --transport grpc
```

For LLM-calling executors, add `--require-stub-mode`: the harness probes for stub
mode at startup and rejects non-stubbed services, preventing accidental real-LLM
calls during conformance. The same checks are exposed as a Go library under
`lib/protocols/conformance/executor`.

## Reference impls

- **Copyable skeleton (Apache)** — a minimal executor you can copy and adapt:
  [`../examples/executor/`](../examples/README.md). It registers, answers the
  `Capabilities` schema gate, and returns terminal success, with the
  `StreamClose` oneof constructions spelled out. Vendored from rimsky-core's
  Apache-licensed `examples/` module at the reconciled tag — the one
  protocol-speaking executor here meant to be built on.
- **Test double** — the stub at `test/support/executors/stub/`, for scenario tests
  and conformance. **Not a skeleton template** — see
  [`../executors/stub/README.md`](../executors/stub/README.md).
- **Official services** — the executors rimsky ships under
  `lib/services/executors/`: `http-node` (Go; HTTP-call workloads),
  `verifier-http`, and `verifier-shape-checks` are **AGPL** runnable products;
  `claude-agent` (TypeScript; runs the Claude Code CLI, demonstrates the
  async-callback path end-to-end) is independently **Apache**. Study them for
  protocol patterns, but do not copy the AGPL services into a non-AGPL project —
  build from the wire contract and the copyable skeleton above.

The `claude-agent` reference executor loads the wire contract via the published
`@rimsky-ai/protocols` npm package (`@grpc/proto-loader` + the package's
`protoDir`/`protoPath` helpers — see [`README.md`](README.md)) — the same package
any TypeScript executor author would use. <!-- @source: lib/services/executors/claude-agent/src/proto-loader.ts -->

## See also

[executor](../concepts/executor.md) · [node](../concepts/node.md) · [attribute](../concepts/attribute.md) · [parked-state](../concepts/parked-state.md)
