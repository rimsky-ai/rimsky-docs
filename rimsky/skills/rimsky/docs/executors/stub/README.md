# stub executor

The `stub` executor (`test/support/executors/stub/` in the rimsky tree) is a **test double** for rimsky's `Executor` protocol. It is the executor analog of a fake/mock: a deterministic, scripted implementation of the `Executor` gRPC service used to exercise rimsky's supervisor against canned outcomes.

It is **not** a skeleton template, not a copy-paste starting point. If you are writing a real executor, start from the [executor guide](../../protocols/executor.md) and the copyable Apache skeleton it links ([`../../examples/executor/`](../../examples/README.md)), implementing against the wire contract (`lib/protocols/proto/v1/executor.proto`). The official executors rimsky ships under `lib/services/executors/` — `http-node` (Go; HTTP-call workloads) and the `verifier-http` / `verifier-shape-checks` verifiers (AGPL), plus `claude-agent` (TypeScript; runs the Claude Code CLI end-to-end; Apache) — are runnable products to study, not starters to copy. Each carries its own README.

## Two forms, two uses

1. **In-process test double** — `test/support/executors/stub/` is a Go package, not a binary. The `stubtest/` wrapper is used by scenario tests under `test/scenarios/`: tests script per-`node_type` behavior on a shared `Stub` instance and assert on the `ExecuteRequest`s the supervisor wired through (attributes, store handles, callback URL) via `Observed()`.
2. **Standalone test binary** — `lib/services/test/stubexecutor/` is a small gRPC `stubexecutor` binary that by default returns immediate-success (`StreamClose{Success}`, `changed=false`, no attribute writeback) for every dispatch. It binds to `EXECUTOR_STUB_BIND` (default `0.0.0.0:9300`) and is built into a Docker image consumed by rimsky's services test harness. It is a test fixture, not a deploy artifact.
   <!-- @source: lib/services/test/stubexecutor/main.go -->

   **Forced-error mode:** setting `EXECUTOR_STUB_FORCE_ERROR=1` flips the binary from success-only to error-only — every dispatch emits a single terminal `StreamClose{Error}` with `error_class: "stub/forced_error"` (the `stub/` prefix follows the hierarchical signal-class convention, mirroring the in-process double's auto-prefixing). In this mode `Capabilities` also advertises `stub/forced_error` in `DeclaredErrorClasses`, so a template node can route the class through an `error_types:` policy without tripping the registration validator's vocabulary range-check. Unset (the default) keeps the success-only behavior; the harness uses forced-error mode to drive abandon-path scenarios.

   The standalone binary also registers an `ExecutorObservability` server and answers `Capabilities` with a permissive open expected-attributes schema (`{"type":"object"}`, no `properties` block — `IsPermissiveExecutorSchema` in `lib/graph/node` reads this as "open shape"). It keeps no traces, so `GetTrace` / `StreamTrace` return `Unimplemented`. The open schema matters for dispatch: the dispatch-time attribute-surface gate (`resolveAttributes` in `lib/runtime/runner_dispatch.go`) rejects an attribute-bearing node whose executor advertises *no* schema with `error_class: "executor_schema_unavailable"`. Because the standalone stub now advertises the open schema, a node carrying an `attributes:` block dispatched against it **dispatches and settles** rather than failing that gate.
   <!-- @source: lib/runtime/runner_dispatch.go::resolveAttributes -->

For conformance, `rimsky conformance executor --require-stub-mode` runs the probe against a stub-mode target as a known-good baseline for protocol-shape checks. The probe rejects non-stubbed services, preventing accidental real-LLM calls during conformance.

## Scripting DSL surface

`Stub.WhenType(type)` returns a builder producing one of the four terminal outcomes the protocol allows, each mapping onto a `StreamClose` oneof variant:
<!-- @source: test/support/executors/stub/stub.go::WhenType -->

| DSL method | Wire outcome |
|---|---|
| `.Success(result, changed, summary)` | `StreamClose{Success}` |
| `.Error(class, payload)` | `StreamClose{Error}` (use `class="executor_blocked"` for the executor-blocked path; the stub auto-prefixes single-segment classes with `stub/`, so the wire-level class becomes e.g. `stub/executor_blocked` per the hierarchical signal-class convention) |
| `.Park(reason, reasonNote, payload, resumeAt, sessionToken)` | `StreamClose{Park}` — `reason` is a `ParkReason` (`PARK_REASON_AWAIT_CALLBACK` or `PARK_REASON_SNOOZE`) |
| `.AwaitAsyncCallback(ackID, expectedMs)` | `StreamClose{AwaitAsyncCallback}` |

Plus modifiers: `.Heartbeats(n)` (emit N heartbeats before the terminal), `.Delay(d)` (sleep before each event), and `.EmitNamedEvent(name, payload)` (emit a `NamedEvent` before the terminal; calls accumulate in order).

`EnableStubMode()` short-circuits every `Execute` call to an immediate-success outcome with `attributes_delta = StubAttributesFor(node_type)`.
