# Implementing an executor

This guide is for developers implementing an executor — in any language — and wiring it into a Rimsky deployment. The wire contracts live at `protocols/proto/v1/executor.proto` (the required dispatch protocol) and `protocols/proto/v1/executor_observability.proto` (the optional read-only observability protocol); the mechanically-generated field/message/RPC references are at [`reference/executor.md`](reference/executor.md) and [`reference/executor-observability.md`](reference/executor-observability.md). This guide is the practical companion. There is no executor SDK; a Go service may use the `protocols` module's `serverkit` helpers ([`go-packages.md`](go-packages.md)) for the gRPC + HTTP/JSON bridge scaffolding, or implement straight against the wire types in any language.

<!-- @source: ../../.ok-planner/design/concepts/executor.md -->
> The protocol-level term for the service that runs a node's work. Implements the dispatch protocol `Executor` (one method, `Execute`) and optionally the paired read-only `ExecutorObservability` protocol (`Capabilities`, `GetTrace`, `StreamTrace`). Out-of-process; supervisors dispatch to executors over gRPC, with an HTTP+JSON bridge available for non-Go services.

> **Auth-blind advisory.** Rimsky has no machinery for credentials, encryption, or access control. Encrypt sensitive bytes before handing them to Rimsky if you need protection. Service-to-service auth is operator-configured at the deployment layer (mTLS, IAM).

---

## 1. The wire contract

The executor surface is split across two service definitions. The required dispatch protocol carries one method:

```protobuf
service Executor {
  rpc Execute(ExecuteRequest) returns (stream ExecuteEvent);
}
```

Source: `protocols/proto/v1/executor.proto`.

The optional read-only observability protocol carries three:

```protobuf
service ExecutorObservability {
  rpc Capabilities(ExecutorCapabilitiesRequest) returns (ObservabilityCapabilities);
  rpc GetTrace(GetTraceRequest) returns (Trace);
  rpc StreamTrace(StreamTraceRequest) returns (stream TraceEvent);
}
```

Source: `protocols/proto/v1/executor_observability.proto`.

Rimsky's supervisor dials the executor at dispatch time and streams events back via `Execute`. Dashboards (and other read-only consumers) dial the executor's observability service to pull or stream per-dispatch traces. Services MUST implement `Executor`; `ExecutorObservability` is opt-in but recommended for any executor whose dispatches are interesting to humans.

`Execute` is the load-bearing method. The response stream is a sequence of `ExecuteEvent` records — each a `oneof` of `Heartbeat`, `NamedEvent`, or `StreamClose`. The stream carries zero or more `Heartbeat` and `NamedEvent` records followed by **exactly one** terminal `StreamClose`, and the executor MUST close the stream immediately after emitting it. A stream that closes without a `StreamClose` is treated by the supervisor as an infrastructure error.

`StreamClose` is itself a `oneof outcome` of exactly one of: `Success`, `Error`, `Park`, `AwaitAsyncCallback`.

## 2. The methods

### `Executor.Execute(ExecuteRequest) → stream<ExecuteEvent>`

Dispatch a node. The salient `ExecuteRequest` fields (full reference in [`reference/executor.md`](reference/executor.md)):

- **node identity** — `node_id`, `instance_id`, `node_type` name which dispatch this is.
- **`attributes`** (`Struct`) — already substituted. The executor sees resolved values; the `{{...}}` directives have been resolved by the supervisor. This is the only template-author-supplied input surface; the historical `userdata` field was collapsed into `attributes` and is reserved on the wire. `attributes_schema` carries the declared JSON Schema for reference.
- **`stores`** (`map<string, StoreHandle>`) — one entry per store the node references, keyed by store-config name. Each `StoreHandle` carries the producer-supplied `handle` (the `Address` bytes returned by `ClaimProducer.Open`, wrapped as a `Struct`), a `kind` string, and `candidate_handle` bytes for `DataProcessing` fan-out leaves. All inert to rimsky.
- **`callback_url`** / **`cancel_token`** — the HTTP+JSON callback base URL for async handoff and incremental attribute writes, and the bearer token the supervisor watches for cancellation (also used on those callbacks).
- **`dispatch_id`** — the supervisor-side `rimsky_node_runs.id`; key per-dispatch traces/state on it.
- **`resume_context`** — populated on resume of a parked node (see §4a); absent on a fresh dispatch.
- **`prior_dispatch_id`** / **`prior_dispatch_disposition`** — set when this dispatch supersedes a prior failed / stale / recalculated dispatch for the same `(run_scope_id, node_id)`. The disposition enum (`PRIOR_HEARTBEAT_STALE`, `PRIOR_RETRY_AFTER_ERROR`, `PRIOR_RECALCULATE`; `PRIOR_NONE` when unset) tells a session-keeping executor *why* it is taking over, so it can recover or hand off work-in-progress.

Stream back any number of `Heartbeat` / `NamedEvent` records, then exactly one `StreamClose`:

- **`Heartbeat`** — keep-alive while work continues. Carries `timestamp_ms` and a free-text `note`.
- **`NamedEvent`** — non-terminal domain-shaped signal. Two fields: `string name` (must appear in the executor's `ObservabilityCapabilities.declared_events`) plus `bytes payload` (opaque to Rimsky; available to substitution as `nodes.<emitter>.event.<name>.<path>`). Zero or more emissions per run; does not close the stream.
- **`StreamClose{Success}`** — terminal success. Three fields:
    - `bool changed` — producer-declared verdict on whether this run produced a different value than the previous run. A `false` value halts cascade propagation at this node.
    - `string change_summary` — free-text summary of the change (audit-log only; not parsed by Rimsky).
    - `Struct attributes_delta` — terminal-final attribute writeback (validated against the node's attributes schema). May be empty when the executor used the incremental-callback path during the run.
- **`StreamClose{Error}`** — terminal application-level error. Two fields: `string error_class` (an executor-defined classifier) plus an opaque `Struct payload`. Use `error_class: "executor_blocked"` for the "I produced output but explicitly chose not to claim success" path (low-confidence outputs routed to human review). The executor does NOT pick the resolution: the supervisor's policy chain in the template maps `(error_class, retry_counter)` to one of `retry`, `give_up`, or `pass`. If you advertise `declared_error_classes` in your observability capabilities, the supervisor validates operator `error_types:` keys against it. Cascade coupling on failure is declared receiver-side on the impactee node.
- **`StreamClose{Park}`** — terminal: pause this run until externally resumed. Fields: `ParkReason reason` (a closed enum — `PARK_REASON_AWAIT_CALLBACK` (the zero value; won't auto-resume) or `PARK_REASON_SNOOZE`), `bytes payload` (opaque; passed back as `ResumeContext.payload`), optional `google.protobuf.Timestamp resume_at` (absent means signal-based-only), optional `string session_token` (opaque; passed back as `ResumeContext.session_token`), and free-form `reason_note` / `reason_label` annotations (inert). Resume happens via time elapsed, an admin invalidate, or an in-graph `on_event` invalidate. See [parked-state](../concepts/parked-state.md).
- **`StreamClose{AwaitAsyncCallback}`** — terminal handoff: I'll POST the final outcome later via callback (see §4). Carries an `async_ack_id` the executor echoes on the callback, plus an optional `expected_completion_ms` hint.

### `ExecutorObservability.StreamTrace(StreamTraceRequest) → stream<TraceEvent>`

Streaming trace of executor activity for observability dashboards. Keyed by `dispatch_id`.

### `ExecutorObservability.GetTrace(GetTraceRequest) → Trace`

Pull a previously-streamed trace by `dispatch_id`. Useful for replaying past invocations from dashboards.

### `ExecutorObservability.Capabilities(ExecutorCapabilitiesRequest) → ObservabilityCapabilities`

Startup handshake for the observability protocol. Declares:

- `supports_trace_get` / `supports_trace_stream` — which read-side RPCs the executor honors.
- `retention_after_terminal_seconds` — the per-dispatch trace retention window.
- `custom_ui` (`CustomUI`) — an optional dashboard-embeddable UI (`ui_url`, `embed_mode`, `dispatch_url_template`).
- `http_bridge_url` — when non-empty, the base URL the executor serves the HTTP+JSON observability bridge on for browser clients; empty means gRPC-only.
- `expected_attributes_schema` — JSON Schema bytes for the accepted attribute shape; empty means accept-any. Output properties are marked `readOnly: true`.
- `declared_events` — event names the executor may emit via `NamedEvent`; empty means none.
- `declared_error_classes` — error-class paths the executor may emit on `Error.error_class`. Patterns ending in `*` are prefix leaves (e.g. `http/server_error/*`); empty means "skip the validator's range-check for this executor."

Probed once per service at process startup.

`expected_attributes_schema` is merged with the template's L1 defaults and L2 per-node declaration to form the effective attribute schema; Rimsky validates the post-substitution bag at dispatch and the post-write-back bag at commit. Schema-validation failures route through `Error { error_class: "template_validation_failed" }`. `declared_events` is cross-validated against any `subscribes: [{node: <sender>, type: event/<name>}]` entries in registering templates; references to undeclared events reject the registration.

## 3. The attribute surface

<!-- @source: ../../.ok-planner/design/concepts/attribute.md -->
> The single template-author-supplied input surface for an executor dispatch. Attributes are declared as a JSON Schema on the template node, substituted (`{{...}}` directives resolved) by the supervisor, validated against the schema at dispatch and again at terminal write-back, and delivered to the executor verbatim. There is no peer "opaque" surface — the historical `userdata` field was collapsed into `attributes` (see `_retired/userdata.md` for the migration record).

This means:

- Every key the executor consumes — model name, system prompts, transport config, etc. — appears under `attributes` and is governed by the node's attributes schema.
- The executor declares the shape it accepts via `expected_attributes_schema` on the `ObservabilityCapabilities` response; Rimsky enforces compatibility at template registration and the substituted bag at dispatch.
- Static configuration (constants the template author wants to hand the executor) lives in attribute `default:` entries; dynamic configuration (values pulled from other nodes or params) lives in `source:` entries. Both surface to the executor identically.
- Encrypted attribute values stay encrypted in transit. Decryption is the executor's responsibility at point of use.

## 4. The async-callback path

For executors whose work outlives a streaming RPC (background jobs, async LLM calls, long-running batch processes), close the `Execute` stream with `StreamClose{AwaitAsyncCallback}` carrying an `async_ack_id`. Later, when the work completes, POST the final outcome back to the supervisor.

The callback body is the `AsyncCallbackBody` message marshalled as JSON: an optional `events` array (a `NamedEvent` stream replayed before the verdict) plus exactly one `outcome` oneof variant — `success`, `error`, or `park`:

```
POST ${callback_url}/v1/callback/{async_ack_id}
Content-Type: application/json

{
  "events": [
    { "name": "phase_complete", "payload": "..." }
  ],
  "success": { "changed": true, "attributes_delta": { ... } }
}
```

Events from the array are persisted and processed before the terminal, so an `on_event` handler can fire mid-flight.

Important wire details:

- The callback path is `${callback_url}/v1/callback/{async_ack_id}` — `callback_url` from the `ExecuteRequest` plus the `async_ack_id` you echoed.
- The body MUST parse as `AsyncCallbackBody`. The pre-2026 legacy `{ "type": "complete" | "blocked" | "errored" }` shape is **no longer accepted**; a body that fails to parse as this message receives HTTP 400.
- `AwaitAsyncCallback` is not a valid `outcome` here — the callback *is* the second half of the async path, so chaining another async handoff is forbidden.
- The same `cancel_token` from the `ExecuteRequest` is the bearer token on this callback (and on incremental attribute writes).

The `protocols` module's `serverkit` package provides Go scaffolding for this bridge; for a non-Go executor, marshal the `AsyncCallbackBody` shape directly.

## 4a. Resume context

When the supervisor resumes a parked node, the dispatch's `ExecuteRequest.resume_context` is populated with three fields:

- `bytes payload` — the original `Park.payload`.
- `string session_token` — the original `session_token` (executor-side correlation identifier).
- `string resume_reason` — `"deadline_elapsed"` (time-based via `resume_at`) or `"external_invalidate"` (admin or in-graph invalidate).

When `resume_context` is empty, this is a fresh dispatch. Executors that do not implement parking can ignore the field.

## 5. Conformance

The `cmd/rimsky-executor-conformance` binary exercises an executor against the wire-protocol contract. Run it pointing at your executor endpoint:

```
rimsky-executor-conformance --endpoint <your-executor-host:port> --transport grpc
```

For LLM-calling executors, run with `--require-stub-mode`. The conformance harness probes the executor for stub mode at startup; non-stubbed services are rejected. This prevents accidental real-LLM calls during conformance.

## 6. Reference impls

The in-tree reference executor is the stub at `executors/stub/` — a test double (Meszaros sense) for scenario tests, conformance, and no-op smoke deployments. **Not a skeleton template** — see [`../executors/stub/README.md`](../executors/stub/README.md).

The production-shaped reference executors — `http-node` (Go; HTTP-call workloads) and `claude-agent` (TypeScript; runs the Claude Code CLI, demonstrates the async-callback path end-to-end) — live in the separate `rimsky-services` repository (`pkg:github.com/fallguyconsulting/rimsky-services/executors/...`). They are illustrative, not part of rimsky's tree.

## See also

- [executor](../concepts/executor.md)
- [node](../concepts/node.md)
- [attribute](../concepts/attribute.md)
