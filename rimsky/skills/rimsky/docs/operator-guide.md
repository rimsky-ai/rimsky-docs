# Operator guide

Operator-visible knobs that span multiple concepts. Per-concept references are in
`docs/concepts/`; protocol implementation is in `docs/protocols/`.

## Configuration root: `RIMSKY_CONFIG`

Each rimsky binary reads its deployment-shape config from `RIMSKY_CONFIG` (default
`/etc/rimsky/rimsky.yml`). The unified file declares persistence, named-locks,
claim-producers, and executors. A reference config ships at
[`reference/config/rimsky.yml`](reference/config/rimsky.yml).

Per-process tuning (concurrency, callback host, heartbeat) lives in each process's
own YAML or env vars and is read separately (e.g. `RIMSKY_SUPERVISOR_CONFIG` for
`rimsky-supervisor`).

## Deployment: the entrypoint

The distributed `rimsky` image runs `rimsky-entrypoint` as PID 1. It selects which
role processes to spawn from its single command argument and validates it:

| Container `command:` | Spawns | Migrate? |
| --- | --- | --- |
| (none) | All three roles (scheduler + supervisor + control-api) — the all-in-one stack. | Yes. |
| `[rimsky-scheduler]` | Only the scheduler. | No. |
| `[rimsky-supervisor]` | Only the supervisor. | No. |
| `[rimsky-control-api]` | Only the control-api. | **Yes** — the designated migrate owner in a split. |
| anything else (unknown role, `rimsky-migrate`, or >1 arg) | Nothing — exits non-zero with an error naming the valid roles. | — |

DB migration runs synchronously **exactly once** across a deployment, before any
role spawns: the no-arg all-in-one path always migrates; in a three-container
split only the `rimsky-control-api` role owns it, so three role containers do not
race three concurrent migrations (and none is skipped). Override with
`RIMSKY_ENTRYPOINT_MIGRATE`: `=1` forces migrate (e.g. a dedicated one-shot init
container), `=0` skips it.

The control-API liveness route is `GET /health` (a shallow snapshot). There is no
shipped dashboard health route in the published images.

## Instance lifecycle: durable by default

Instances are **durable by default**. There is no auto-terminate-on-drain: an
instance that runs out of work stays alive (paused-idle) and resumes when new
input arrives. The only built-in self-termination path is the create-time opt-in
`terminate_after_run` flag on `POST /instances`:

```json
{ "template": "<id>", "instance_key": "...", "terminate_after_run": true }
```

When `terminate_after_run` is true the instance self-terminates after its **next
frame ends** — strict "run at most once more" semantics. It is an opt-in on the
create request only; an idempotent re-create (same `template` + `instance_key`)
returns the existing instance and **ignores** the flag, exactly as `paused` does.
Operators who relied on instances cleaning themselves up on drain must now either
set `terminate_after_run` at create, or force-terminate explicitly via
`POST /instances/{id}/terminate`.

## Persistence: blob backend

The `persistence.blob` block selects how attribute values, parked-state payloads,
and named-event payloads are stored when they exceed the inline-spill threshold:

```yaml
persistence:
  driver: postgres
  postgres:
    dsn: ...
  blob:
    backend: pg-largeobject  # inline | pg-largeobject | filesystem | memory
    spill_threshold_bytes: 65536
    filesystem:
      root: /var/lib/rimsky/blobs
    pg_largeobject:
      schema: public
    retention:
      orphan_sweep_interval: 1h
      retention_after_unreferenced: 24h
```

| `backend` | What it does | When to use |
| --- | --- | --- |
| `inline` | No spill; large attribute values stay in the attribute table inline. | The default. Small-attribute workloads. |
| `pg-largeobject` | Postgres-large-object backend. Uses the same DSN as the persistence driver. | Multi-host deployments. |
| `filesystem` | Files written under `filesystem.root`. | Multi-host deployments — requires a shared volume. |
| `memory` | In-process map. | Dev-only (see below). |

`memory` is rejected at startup unless `RIMSKY_PROCESS_ROLE=unified` (set by
`rimsky-entrypoint`). The per-process binaries (`rimsky-scheduler`,
`rimsky-supervisor`, `rimsky-control-api`) cannot share state through an
in-process map, so the gate prevents accidental misconfiguration.

`SweepOrphanedBlobs` runs in the scheduler tick loop (throttled to
`OrphanBlobSweepInterval`, default 1h) and reaps blob handles whose retention
window has elapsed. The blob backend itself sees only `Delete(handle)`.

## Scheduler retention

The top-level `retention:` block tunes the scheduler tick's trailing-window
retention sweeps — the reaping of stale run-tree trace frames, lineage records,
released claim handles, and message idempotencies. It is distinct from
`persistence.blob.retention` (above), which governs only orphaned blob handles.

```yaml
retention:
  recent_frames_kept: 100
  trace_trailing: 720h
  lineage_trailing: 720h
  claim_handles_trailing: 720h
  message_idempotencies_trailing: 24h
```

| Key | Type | Default | Reaps |
| --- | --- | --- | --- |
| `recent_frames_kept` | int | 100 | Most-recent run-tree trace frames kept per run-tree regardless of age. |
| `trace_trailing` | duration | `720h` (30d) | Run-tree trace frames older than the window (beyond `recent_frames_kept`). |
| `lineage_trailing` | duration | `720h` (30d) | Lineage records older than the window. |
| `claim_handles_trailing` | duration | `720h` (30d) | Released claim handles older than the window. |
| `message_idempotencies_trailing` | duration | `24h` | `rimsky_message_idempotencies` rows older than the window. |

Retention is ON by default. The whole block — and any individual key — may be
omitted and the documented default applies; the scheduler tick reaps stale rows
out of the box. An explicit `0s` on one key **disables that one sweep** (it is
NOT re-defaulted — e.g. `claim_handles_trailing: 0s` keeps released handles
forever). A negative value is a startup error. Durations use Go syntax (`720h`,
`24h`, `90m`).

## Parked-duration caps

The top-level `max_park_duration:` block sets a deployment-wide fallback cap on
how long a node may stay parked, keyed by the stored park reason. The park-reason
enum is closed: only `await_callback` and `snooze` are accepted; any other key is
a startup error naming the valid set.

```yaml
max_park_duration:
  await_callback: 168h   # 7d
  snooze: 1h
```

The per-row `rimsky_node_runs.max_park_duration_seconds` always wins when set;
these keys are the deployment-level defaults that fire only when the per-row
value is NULL. Omit the block for no deployment-level cap. The supervisor's
parked-node sweep enforces the cap.

## claude-agent: configuration

The `claude-agent` reference executor is configured two ways: process environment
at startup, and per-node attributes at dispatch time. It has no separate config
file or catalog of external MCP servers.

### Startup environment

Set on the `claude-agent` executor process.

| Variable | Default | Meaning |
| --- | --- | --- |
| `ANTHROPIC_API_KEY` / `CLAUDE_CODE_OAUTH_TOKEN` | — | At least one is required in non-stub mode; the executor refuses to start without one. In API-key mode the key is written to a 0600 temp file behind an `apiKeyHelper` and never enters the spawned `claude` process's environment. |
| `RIMSKY_EXECUTOR_STUB_MODE` | unset | `=1` ⇒ stub mode: the executor spawns no `claude` subprocess and returns a canned completion. The cookbook recipes run claude-agent in stub mode. |
| `RIMSKY_EXECUTOR_HOST` | — | Bind address for the gRPC executor and HTTP+JSON bridge. |
| `RIMSKY_EXECUTOR_PORT_GRPC` | `9090` | gRPC executor port. |
| `RIMSKY_EXECUTOR_PORT_HTTP` | `9190` | HTTP+JSON bridge port. |
| `RIMSKY_EXECUTOR_SILENCE_MS` | `120000` | How long the subprocess may produce no stdout before the silence-tracker acts. |
| `RIMSKY_DISPATCH_MAX_USD` | — | Deployment-wide spend cap forwarded to the CLI as `--max-budget-usd`. A per-node `cli.max_budget_usd` attribute wins over this when set. |

### Per-node attributes

Templates drive each dispatch through the node's `attributes`, not through an
operator catalog. The executor reads `model`, `system_prompt`, `user_prompt`, and
an optional `cli.*` sub-object that tunes the spawned `claude` CLI. Each `cli.*`
key maps to a `claude` CLI flag (or a recovery behavior); rimsky never inspects the
values.

| `cli.*` attribute |
| --- |
| `cli.bare` |
| `cli.permission_mode` |
| `cli.allowed_tools` |
| `cli.disallowed_tools` |
| `cli.add_dirs` |
| `cli.max_budget_usd` |
| `cli.handle_rate_limits` |
| `cli.max_schema_corrections` |

The full expected-attributes schema is defined by the claude-agent executor itself
(in-tree at `lib/services/executors/claude-agent/`); see
[`docs/agents/examples/claude-agent-attribute-defaults.md`](agents/examples/claude-agent-attribute-defaults.md)
for a worked example of how attribute defaults flow through it.

### MCP wiring

The executor wires exactly one MCP server into each dispatch: its own internal
`rimsky-callback` (an HTTP MCP server it hosts), through which the agent reports
terminal outcomes (`report_complete`, `report_error`, `report_blocked`,
`report_park`), emits named events, and reads/writes node attributes. There is no
operator-configured catalog of external MCP servers, and templates cannot register
additional MCP servers for a dispatch to reach.

## Observability: Prometheus metrics

Each rimsky binary can expose a `/metrics` endpoint.

| Binary | Settings |
| --- | --- |
| `rimsky-control-api` | `RIMSKY_METRICS_PORT` (0 = disabled, default). Bound to the same host as the control API. |
| `rimsky-scheduler` | `RIMSKY_METRICS_PORT` and `RIMSKY_METRICS_HOST` (default `127.0.0.1`). |
| `rimsky-supervisor` | Same as scheduler. |

The metric set is documented in `lib/control/observability/metrics.go`. Counters
cover dispatches, terminal verdicts, invalidates, claim acquisitions. Gauges cover
nodes-by-state, parked-by-reason, held frames, dispatch queue depth. Histograms
cover dispatch latency, claim acquisition latency, frame duration, and
parked-duration-on-resume.

## Diagnostic endpoints

The control API exposes:

| Method + path | Purpose |
| --- | --- |
| `GET /admin/diagnostics/held-frames` | Frames currently held. |
| `GET /admin/diagnostics/parked-nodes` | Parked nodes; optional `?reason=<name>` filter. |
| `GET /admin/diagnostics/wait-sets` | Wait-sets currently registered. |
| `POST /admin/instances/{instance}/nodes/{node_id}/invalidate` | Admin invalidate. Dispatches by node state: `parked` resumes; `fresh`/`stale`/`failed` invalidate; `running` returns 409. |
| `POST /admin/lineage/prune` | Prune lineage records. |

`/admin/instances/{instance}/nodes/{node_id}/invalidate` is the general-purpose
admin invalidation surface for any node state. There is no scheduled-node
`force-fire` route — template-level schedules were retired; cron firing now lives
in the standalone `sensor-cron` publisher service, which sources its own messages.

## Conformance probes

The conformance probes are subcommands of the `rimsky` CLI —
`rimsky conformance <protocol> ...`. They were folded in from the former standalone
`cmd/rimsky-*-conformance` binaries; the underlying runners remain importable as Go
libraries under `lib/protocols/conformance/...`.

| Subcommand | Exercises |
| --- | --- |
| `rimsky conformance executor` | An executor against the protocol. Stub mode is mandatory for LLM-calling executors (`--require-stub-mode`). |
| `rimsky conformance claim-producer` | A claim-producer. |
| `rimsky conformance publisher` | A publisher (`--kind`). |
| `rimsky conformance validation` | The Validation mix-in. |
| `rimsky conformance data-processing` | The DataProcessing mix-in. |
| `rimsky conformance blob-backend` | A blob backend against the `BlobBackend` interface (in-process; pass `--backend <name>` plus the backend's required config). |
| `rimsky conformance probe` | The protocol-agnostic stub-mode probe. |

Each exits 0 on all checks passing.

## Pre-v1 caveats

- No Helm chart or Kubernetes manifests ship yet. Deploy from the published
  images (`rimskyai/rimsky*`); a reference config lives at
  [`reference/config/rimsky.yml`](reference/config/rimsky.yml), and a chart is
  on the roadmap but not published.
- The unified image (`rimsky-all-in-one`, built `FROM` the multi-role `rimsky`
  image) defaults to SQLite at `/var/lib/rimsky/state.db`. Replicas > 1 break
  (independent SQLite databases). Run the combined `rimsky` image per role with
  the postgres driver for multi-replica deployments.
- Pre-v1 has no backwards-compat guarantees on schema or wire shapes.
  Migrations may drop and recreate tables.
