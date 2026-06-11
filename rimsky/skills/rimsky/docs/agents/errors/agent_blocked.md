---
error: agent/blocked
surfaced_to: operator
---

# claude-agent blocked (`agent/blocked`)

## What it means

The agent itself reported that it cannot proceed: it called the per-dispatch callback MCP tool `mcp__rimsky-callback__report_blocked`, and the `claude-agent` executor mapped that to a terminal `Error{ error_class: "agent/blocked" }` whose payload carries the agent's `reason` and `context`. <!-- @source: lib/services/executors/claude-agent/src/agent-run.ts::onBlocked -->

This is a deliberate, agent-initiated outcome — not a crash, not a subprocess fault, and not the reserved core class `executor_blocked`. The executor declares it in `declared_error_classes`, so the template's `error_types:` chain routes it like any other class.

## When it happens

The agent decided the dispatched work is not completable as posed: a missing precondition, an instruction conflict, an environment it cannot act in. Whether that judgment is right is the agent's call; the executor just relays it.

## What to do

Read `reason` / `context` from the signal payload — they are the agent's own statement of what blocked it. Typical fixes: supply the missing input upstream, adjust the node's prompt attributes, or wire the missing capability (e.g. an MCP server under `cli.mcp_servers`). A retry policy on `agent/blocked` is usually wasted rounds unless something upstream changed between attempts.

## See also

- [`agent_subprocess_exit.md`](agent_subprocess_exit.md) — the involuntary CLI-run failure family.
- [`../../protocols/executor.md`](../../protocols/executor.md)
- [`../../concepts/error-policy.md`](../../concepts/error-policy.md)
