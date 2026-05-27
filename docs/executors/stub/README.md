# stub executor

The `stub` executor (`executors/stub/`) is a **test double** for rimsky's `Executor` protocol and the only executor that ships in rimsky's tree. It is the executor analog of a fake/mock: a deterministic, scripted implementation of the `Executor` gRPC service used to exercise rimsky's supervisor against canned outcomes.

It is **not** a skeleton template, not a copy-paste starting point. If you are writing a real executor, start from the [executor guide](../../protocols/executor.md) and implement against the wire contract. The production-shaped reference executors (`http-node`, `claude-agent`) live in the separate `rimsky-services` repository.

## Three primary uses

1. **`executors/stub/cmd/`** — a standalone gRPC binary (`rimsky-executor-stub`) used by the quickstart and smoke-deployment compose stacks as a no-op executor. It runs in stub mode (the binary calls `EnableStubMode()` at startup), returning immediate-success outcomes keyed by `node_type` (via `StubAttributesFor`), letting an end-to-end stack exercise the supervisor + control-api + persistence path without doing real executor-side work.
2. **`executors/stub/stubtest/`** — the in-process wrapper used by scenario tests under `test/scenarios/`. Tests script per-`node_type` behavior on a shared `Stub` instance and assert on the `ExecuteRequest`s the supervisor wired through (attributes, store handles, callback URL) via `Observed()`.
3. **`rimsky-executor-conformance --require-stub-mode`** — the conformance probe runs against a stub-mode target as a known-good baseline for protocol-shape checks. The probe rejects non-stubbed services, preventing accidental real-LLM calls during conformance.

## Scripting DSL surface

`Stub.WhenType(type)` returns a builder producing one of the four terminal outcomes the protocol allows, each mapping onto a `StreamClose` oneof variant:

| DSL method | Wire outcome |
|---|---|
| `.Success(result, changed, summary)` | `StreamClose{Success}` |
| `.Error(class, payload)` | `StreamClose{Error}` (use `class="executor_blocked"` for the executor-blocked path; the stub auto-prefixes single-segment classes with `stub/`, so the wire-level class becomes e.g. `stub/executor_blocked` per the hierarchical `concept:signal` convention) |
| `.Park(reason, reasonNote, payload, resumeAt, sessionToken)` | `StreamClose{Park}` — `reason` is a `ParkReason` (`PARK_REASON_AWAIT_CALLBACK` or `PARK_REASON_SNOOZE`) |
| `.AwaitAsyncCallback(ackID, expectedMs)` | `StreamClose{AwaitAsyncCallback}` |

Plus modifiers: `.Heartbeats(n)` (emit N heartbeats before the terminal), `.Delay(d)` (sleep before each event), and `.EmitNamedEvent(name, payload)` (emit a `NamedEvent` before the terminal; calls accumulate in order).

`EnableStubMode()` short-circuits every `Execute` call to an immediate-success outcome with `attributes_delta = StubAttributesFor(node_type)`.

## Operating

The standalone binary is reachable over gRPC; operators wire its endpoint into rimsky's `executors:` block in `rimsky.yml`. It holds no database and no persistent state.
