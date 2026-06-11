---
error: agent/internal_error
surfaced_to: operator
---

# claude-agent internal error (`agent/internal_error`)

## What it means

The `claude-agent` executor's own dispatch handling threw an exception that is not a classified configuration error — the catch-all for executor bugs and unexpected runtime failures, emitted as a terminal `Error{ error_class: "agent/internal_error" }` with the exception text on the payload's `error`. <!-- @source: lib/services/executors/claude-agent/src/server.ts, lib/services/executors/claude-agent/src/http-bridge.ts -->

It is a statement about the **executor**, not the agent's work, the attributes, or the CLI subprocess: classified configuration faults surface as [`agent/attribute_invalid`](agent_attribute_invalid.md), and subprocess faults as the [`agent_subprocess_exit.md`](agent_subprocess_exit.md) family.

## What to do

Check the executor's logs for the underlying exception. If reproducible, it is an executor bug to fix; a retry policy may mask it. Verify the executor image/version before suspecting the template.

## See also

- [`agent_attribute_invalid.md`](agent_attribute_invalid.md) · [`agent_subprocess_exit.md`](agent_subprocess_exit.md) — the classified siblings.
- [`http_internal_error.md`](http_internal_error.md) — the `http-node` equivalent catch-all.
