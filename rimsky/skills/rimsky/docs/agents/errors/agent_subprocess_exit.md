---
error: agent/subprocess_exit/before_complete
surfaced_to: operator
---

# claude-agent CLI-run failures (`agent/cli_spawn_failed`, `agent/timeout`, `agent/rate_limited`, `agent/context_exceeded`, `agent/refused`, `agent/tool_use_failed/*`, `agent/subprocess_exit/*`)

## What it means

The `claude-agent` executor's spawned Claude Code CLI subprocess failed, stalled, or exited without delivering a terminal `report_complete` callback. One mechanism family — the CLI process lifecycle and its exit classification — emitting one of seven declared classes (table below), each a terminal `Error{ error_class }` routable through the node's `error_types:` policy. <!-- @source: lib/services/executors/claude-agent/src/agent-run.ts, lib/services/executors/claude-agent/src/error-classify.ts::classifyAgentError -->

Not in this family: dispatch-time attribute rejection ([`agent_attribute_invalid.md`](agent_attribute_invalid.md)), the agent voluntarily reporting blocked ([`agent_blocked.md`](agent_blocked.md)), the schema and sign-off gates ([`agent_schema_violation.md`](agent_schema_violation.md), [`signoff_unobtained.md`](signoff_unobtained.md)), and the executor's own catch-all ([`agent_internal_error.md`](agent_internal_error.md)).

## The classes

| Class | Fires when | Payload keys |
| --- | --- | --- |
| `agent/cli_spawn_failed` | Spawning the CLI threw (missing binary, permissions, OS resource exhaustion) — the run never started | `error` |
| `agent/timeout` | The subprocess produced no stdout for longer than the silence timeout (`RIMSKY_EXECUTOR_SILENCE_MS`, default `120000`); the executor SIGTERMs then SIGKILLs it | `silence_duration_ms` |
| `agent/rate_limited` | Non-zero exit whose stderr matches a rate-limit signature (`rate_limit_error`, "rate limit", a 429) **and** `cli.handle_rate_limits` is explicitly `false`. With the default (`true`) the same detection emits a `Park` (`reason: snooze`, woken at the detected `resume_at`) instead of this error | `exitCode`, `signal`, `resume_at` (ISO 8601 or null) |
| `agent/context_exceeded` | Non-zero exit whose stderr carries a context-window signature (`context_length_exceeded`, "context window", "maximum context", "prompt is too long") | `exitCode`, `signal` |
| `agent/tool_use_failed/<tool>` | Non-zero exit whose stderr carries `tool_use_failed` / "tool execution failed"; `<tool>` is the tool name parsed from stderr, `unknown` when unparseable. Advertised as the `agent/tool_use_failed/*` prefix pattern | `exitCode`, `signal` |
| `agent/refused` | Non-zero exit whose stderr carries a refusal signature (`(refusal)`, "refused by the model", "declined to respond") | `exitCode`, `signal` |
| `agent/subprocess_exit/before_complete` | The subprocess exited without any recognized signature and without calling `report_complete` — the fallback leaf (and the family's only leaf; advertised as the `agent/subprocess_exit/*` prefix pattern) | `exitCode`, `signal` (+ `retry_attempted` / `retry_failed` on the recovery path) |

Classification order matters: rate-limit detection runs first, then (for a **clean** exit-0 without a callback) a recovery resume, then the stderr-signature classification, then the `before_complete` fallback.

## The clean-exit recovery path

A subprocess that exits `0` without ever calling `mcp__rimsky-callback__report_complete` is not failed immediately: the executor resumes the same CLI session with a one-shot reminder prompt (the agent's full context and the callback MCP server are still intact). Only if the resumed session also ends without a terminal callback — or the resume itself throws — does the run settle as `agent/subprocess_exit/before_complete`, with `retry_attempted: true` or `retry_failed: <error>` on the payload. A `report_complete` that lands during the resume always wins over the fallback.

## What to do

Key on the leaf, not the family:

- `cli_spawn_failed` — fix the executor host (CLI binary present, permissions, resources); transient under load.
- `timeout` — raise `RIMSKY_EXECUTOR_SILENCE_MS` if the work is legitimately quiet for long stretches; otherwise treat as a hung run.
- `rate_limited` — prefer the default auto-park (`cli.handle_rate_limits: true`); only key a policy on this class when you have deliberately opted out and want `error_types:` to own the backoff.
- `context_exceeded` — shrink the prompt/inputs or split the node; retrying unchanged reproduces it.
- `tool_use_failed/<tool>` — pivot policy on the offending tool; check the tool's availability/permissions on the executor host.
- `refused` / `subprocess_exit/before_complete` — read the run's stderr in the executor logs; a retry sometimes lands (non-deterministic agent behavior), but recurring instances usually mean the prompt or environment needs repair.

## See also

- [`agent_internal_error.md`](agent_internal_error.md) — the executor's own unhandled-exception catch-all.
- [`../examples/claude-agent-attribute-defaults.md`](../examples/claude-agent-attribute-defaults.md) — the `cli.*` attribute reference (`cli.handle_rate_limits` among them).
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
- [`../../protocols/executor.md`](../../protocols/executor.md)
