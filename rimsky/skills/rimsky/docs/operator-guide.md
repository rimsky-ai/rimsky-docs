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

`SweepOrphanedBlobs` runs in the foundation tick loop and reaps blob handles whose
retention window has elapsed. The blob backend itself sees only `Delete(handle)`.

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
| `POST /admin/instances/{instance}/nodes/{node_id}/invalidate` | Admin invalidate. Dispatches by node state: `parked` resumes, `fresh` invalidates, `running`/`failed` returns 409. |
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
