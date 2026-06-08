# Feedback for rimsky-docs: executor protocol — version-label the pages, and ship a pinned, compile-checked stub-executor example

**From:** a downstream project implementing a custom `claude-agent`-typed executor (a test-harness stub) against `github.com/rimsky-ai/rimsky-core/lib/protocols`.
**Docs corpus consulted:** rimsky-docs **1.3.0** (`skills/rimsky/docs/...`).
**Module pinned by our build:** `lib/protocols **v0.6.0**`.

## TL;DR (two asks, ranked)

1. **Version-label every protocol doc page** — state which `lib/protocols` version the page's API/code reflects. This is the higher-value fix.
2. **Fill the existing `docs/executors/stub/` slot with a runnable, version-pinned, CI-compiled minimal executor** (`main.go` + `go.mod` that build as-is).

## What happened

We built a minimal `claude-agent`-typed executor whose only job is to register, receive a dispatch, call out over MCP, and return terminal success — i.e. a stub for an end-to-end test harness. The implementer consulted the rimsky skill heavily (the skill was invoked repeatedly and most executor-related pages were read) and used the real module (`genv1.RegisterExecutorServer`, `genv1.ExecuteRequest`/`ExecuteEvent`/`OpenResponse`/`StreamClose`/`Error`, `serverkit`). Despite using the docs *correctly*, assembling a compiling executor took many build-debug cycles. Two doc-side causes:

### 1. Version skew with no signal

The docs corpus is rimsky **1.3.0**; our build pins `lib/protocols **v0.6.0**`. These are different version lines, and **no page states which module version its API description targets.** Code written faithfully to the prose can fail to compile against the pinned generated API, and the reader has no way to tell whether a mismatch is their error or doc/version drift. "Consult the latest docs" actively misleads a reader on an older (or simply different) pin.

This is the single most disorienting part, and it is cheap to fix: a one-line version banner per page (e.g. *"API on this page reflects `lib/protocols vX.Y.Z`; for another pin, read the generated package at your tag"*).

### 2. The executor protocol is streaming/stateful, but the docs are prose-only

`protocols/executor.md` (v1.3.0) is **231 lines with zero fenced Go** — it describes the API in prose ("do that with `genv1.RegisterExecutorServer` plus a plain `http.Client`"). But the executor surface is not a single unary `Dispatch`; it's a **streaming, stateful lifecycle** (`ExecuteRequest` → a stream of `ExecuteEvent` → `OpenResponse` / `StreamClose` / `Error`). Reconstructing that handshake and event sequencing from prose + symbol references is exactly where the time went — a place where a runnable skeleton is worth far more than prose.

There is already a slot for this — `docs/executors/stub/` — but it currently contains only a **29-line `README.md` with no code.** The intent seems present; the artifact is missing.

## The example we'd want (and why this form)

A minimal **stub executor** is the canonical "I just need something that speaks the protocol" case — test harnesses, local development, conformance smoke tests. Concretely:

- A complete, compiles-as-is `main.go` + `go.mod` under `docs/executors/stub/` that: dials/registers with the supervisor, accepts a dispatch, emits the minimal valid event sequence, and returns terminal success.
- **`go.mod` pinned to a stated version**, with the page's version banner matching.
- **CI-compiled** against that pin so it cannot rot. (Rot-avoidance is presumably why the docs ship prose today; a compile job in CI removes that reason without giving up runnable code.)
- Bonus: a short note pointing readers at the **module's own godoc / generated package at their pinned tag** as the version-exact authority that complements the conceptual docs — the one source that is both authoritative and guaranteed to match what the reader compiles against.

## Why version-labeling leads

A single newest-version example, unlabeled, just relocates the skew: a reader on an older pin still can't trust it. Labeling lets a reader map doc → pin and decide whether to trust the code or fall back to the generated package at their tag. The example is most useful *once the version it targets is explicit.*

## Impact

A capable implementer, using the skill as intended, spent a long build-debug loop on a streaming executor because the authoritative code shape wasn't runnable and wasn't version-anchored. Both fixes are low-maintenance (a version banner; one CI-checked example) and would turn "speak the executor protocol" from a multi-hour synthesis into a copy-pin-adapt.
