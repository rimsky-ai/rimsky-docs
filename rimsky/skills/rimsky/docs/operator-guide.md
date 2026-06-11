# Operator guide

Operator-visible knobs that span multiple concepts. Per-concept references are in
`docs/concepts/`; protocol implementation is in `docs/protocols/`.

## Configuration root: `RIMSKY_CONFIG`

Each rimsky binary reads its deployment-shape config from `RIMSKY_CONFIG` (default
`/etc/rimsky/rimsky.yml`). The unified file declares persistence, named-locks,
claim-producers, executors, and publishers. A reference config ships at
[`reference/config/rimsky.yml`](reference/config/rimsky.yml).

Per-process tuning (concurrency, callback host, heartbeat) lives in each process's
own YAML or env vars and is read separately (e.g. `RIMSKY_SUPERVISOR_CONFIG` for
`rimsky-supervisor`).

The whole file is `os.ExpandEnv`-expanded at load (Go `${VAR}` syntax — no
bash `${VAR:-default}` defaults; an unset variable expands to empty).

### rimsky.yml: the full operator key set

Every key the loader accepts (loader in-tree at `lib/control/config`).
Unknown `protocols:` values, unknown `max_park_duration` keys, unknown
`write_semantics_allowed` values, and unknown `templates.ref_validation_mode`
values are all rejected at startup with a precise error — never silently
ignored.

| Key | Type / values | Default | Notes |
| --- | --- | --- | --- |
| `persistence.driver` | `postgres` \| `sqlite` | — (required) | Selects the unified persistence driver. |
| `persistence.postgres.dsn` | string | — | Postgres connection string. |
| `persistence.postgres.max_open_conns` | int | driver default | Pool size cap. |
| `persistence.postgres.max_idle_conns` | int | driver default | Idle pool size. |
| `persistence.postgres.conn_max_lifetime` | Go duration | driver default | Per-connection max lifetime. |
| `persistence.sqlite.path` | string | — | SQLite database file path. |
| `persistence.blob.*` | block | `backend: inline` | See [Persistence: blob backend](#persistence-blob-backend). |
| `claim_producers.<name>.endpoint` | gRPC endpoint | — (required) | `grpc://host:port` or bare `host:port`; `http://`, `https://`, `tcp://`, `unix://` are rejected at startup. |
| `claim_producers.<name>.protocols` | list | `[claim_producer]` | Must include `claim_producer`. Valid values: `claim_producer`, `executor`, `publisher`, `lifecycle_subscriber`, `validation`, `data_processing`. |
| `claim_producers.<name>.write_semantics_allowed` | list | — (required, non-empty) | Operator-permitted subset of the producer-advertised set. Values: `sync`, `staged_async`, `blocking_async`, `read_only`. To learn a producer's advertised set, declare a guess and read the startup mismatch error — it names both sets (`operator declared [...], producer advertised [...]`); for the bundled postgres store the advertised set is exactly its own config's `write_semantics` value (default `staged_async` when omitted). |
| `claim_producers.<name>.observability_endpoint` | endpoint | reuse `endpoint` | Override for the observability handshake only. |
| `named_locks.<name>.limit` | int | — | Counting-semaphore limit for the named lock. |
| `executors.<name>.transport` | `grpc` \| `http` | — (required) | `grpc`: `endpoint` is the gRPC dispatch target (`host:port`). `http`: the supervisor drives the executor through its HTTP+JSON bridge instead — `endpoint` is the bridge **base URL** (e.g. `http://http-node:9092` — http-node's bridge defaults to its gRPC port + 1); the supervisor POSTs `<endpoint>/v1/Execute` and reads an NDJSON `ExecuteEvent` stream. Use `http` only for bridge-only peers; the reference stack uses `grpc`. |
| `executors.<name>.endpoint` | endpoint | — (required) | e.g. `claude-agent:9090` (grpc) or `http://http-node:9092` (http). |
| `executors.<name>.tls` | `off` \| `optional` \| `required` | empty (optional) | Recorded but **not yet enforced** — pre-v1, all executor connections are insecure plaintext regardless of this value; TLS enforcement is post-v1. Omitting it is fine. Unquoted `off` parses as the string `"off"` (the loader's YAML library does not treat `off` as a boolean); no quoting needed. |
| `executors.<name>.protocols` | list | `[executor]` | Must include `executor`. |
| `executors.<name>.observability_endpoint` | endpoint | reuse `endpoint` | |
| `publishers.<name>.endpoint` | endpoint | — | Publisher gRPC endpoint. |
| `publishers.<name>.protocols` | list | `[publisher]` | Must include `publisher`. |
| `publishers.<name>.observability_endpoint` | endpoint | reuse `endpoint` | |
| `max_park_duration.<reason>` | Go duration | none | See [Parked-duration caps](#parked-duration-caps). Keys: `await_callback`, `snooze` only. |
| `retention.*` | block | retention ON | See [Scheduler retention](#scheduler-retention). |
| `late_bind_service_proxies` | map protocol → service name | `{}` | e.g. `{executor: host-agent-proxy, claim_producer: host-agent-proxy}`. Routes late-bound dispatches for that protocol through the named proxy peer (the host-agent-proxy architecture). Empty map = no late-bind behavior. |
| `templates.ref_validation_mode` | `all` \| `available` \| `none` | `all` | See [Template registration and reference validation](#template-registration-and-reference-validation). Env `RIMSKY_REF_VALIDATION_MODE` overrides the YAML value. |

Retired keys the loader **rejects** with a pointer to the replacement: the
top-level `stores:` block (use `claim_producers:`), the per-producer
single-value `write_semantics:` shortcut and the `write_semantics_envelope:`
alias (both: use `write_semantics_allowed: [...]`).

### Startup peer handshake

All three core processes (scheduler, supervisor, control-api) dial every
`claim_producers:` entry at startup and run the `Capabilities` handshake,
bounded at 30s per peer. An unreachable peer, a handshake timeout, or a
`write_semantics_allowed` set that is not a subset of the producer-advertised
set fails the process at startup — the three processes stay in lock-step on
the operator-declared topology (the subset-mismatch error names both sets, so
it is also how an operator discovers what a producer advertises).

The per-protocol mix-in dials happen only in the **supervisor and
control-api** — the scheduler runs nothing beyond the claim-producer
`Capabilities` handshake. Peers declaring `lifecycle_subscriber`, `publisher`,
or `data_processing` in `protocols:` get an additional per-protocol gRPC
client constructed under the same 30s bound, but that construction issues
**no RPC** — the connection comes up lazily, so an unreachable mix-in peer
surfaces on first use, not at startup. The exception is `validation`: since
v0.8.0, a claim-producer peer that declares the `validation` mix-in incurs
**one extra `Capabilities` RPC at startup** (the control-api and supervisor
re-handshake to learn the peer's live `validation_supported_roles`) — an
unreachable validation peer fails startup, and one that accepts the
connection but never answers blocks that process's startup until the 30s
bound trips.

`executors:` and `publishers:` entries are **not** handshaked at startup and
an unreachable one does not fail startup. Executor dispatch connections open
lazily from the supervisor's client pool at first dispatch. Publisher clients
are constructed at control-api startup (no RPC); the control-api then runs a
**best-effort** background subscription resync against each publisher
(re-issue `Subscribe` for rows rimsky holds as active but the publisher
dropped, e.g. after a publisher restart; tear down orphans) — an unreachable
publisher is logged and skipped, never a startup failure. Bring-up ordering:
only `claim_producers:` (plus `validation`-mix-in peers) must be live before
the three core processes start; executors and publishers may come up
afterward — but under the default `templates.ref_validation_mode: all` they
must be visible before `template register` runs.

## Control API: the `/v1/` route prefix

Every control-API route lives under a `/v1/` version prefix as of v0.8.0 —
including `GET /v1/health`. There are no unversioned routes; pre-v0.8.0 bare
paths (`/health`, `/instances`, `/templates`, …) return 404. The supervisor's
async-callback listener is a separate HTTP server and is unaffected (executors
keep POSTing to `${callback_url}/v1/callback/{async_ack_id}`).

## Control API: auth

Every control-API route except `GET /v1/health` is auth-gated: requests carry
`Authorization: Bearer <key>` and each route checks a per-action permission
(admin and diagnostics routes included; publisher/sensor POSTs to
`/v1/instances/{id}/messages` are gated by the `message:send` action, so in
an authenticated deployment publishers need a key too). The `/metrics`
listeners are separate ports and are never auth-gated.

The default is **anonymous mode**: while the API-key ledger has zero active
keys, every request — including requests with no `Authorization` header — is
admitted as a synthetic admin identity (wildcard `*` permission), and the
control-api logs a WARN banner at startup and every 5 minutes. The mode is
**data-derived, not config-derived**: there is no `rimsky.yml` or env knob;
it flips off automatically the moment the first key is minted and cannot be
re-entered without explicitly revoking the last key. Mint the first admin key
with `rimsky auth init` (POSTs `/v1/auth/keys` against the anonymous
deployment; refuses if a key already exists), further keys with
`rimsky auth create-key`. The CLI sends the key from `RIMSKY_API_KEY` (or
`--key`). See [`concepts/anonymous-mode.md`](concepts/anonymous-mode.md) and
[`concepts/api-key.md`](concepts/api-key.md).

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

The control-API liveness route is `GET /v1/health` (not auth-gated, so
load-balancer and k8s probes need no Bearer token). It is versioned like
every other control-API route — bare `/health` returns 404. There is no
shipped dashboard health route in the published images.

What the snapshot actually checks: the handler answers `status: "ok"` iff the
control-api can query its persistence layer (it lists registered supervisors
and counts nodes by state in one transaction); a DB failure is the only
non-200 path. The response also carries the registered supervisors (id,
accepted executors, concurrency, active node count, last heartbeat) and a
node-state rollup for eyeballing. It does **not** probe peers — a deployment
with every executor, store, and publisher down still reports `ok`. Wire it as
a probe for the control-api process + its database only; peer health is the
observability surface's job.

## Template registration and reference validation

Template registration validates every reference the template carries — executors,
claim producers (stores), named locks — against the running control-plane. Under
the default mode the referenced services must be visible (provisioned and
handshaked) at the moment `template register` runs; references whose targets are
not yet visible cause the registration to be refused with a 400.

The strictness is operator-controlled via `templates.ref_validation_mode` in
`rimsky.yml` (or the `RIMSKY_REF_VALIDATION_MODE` env-var override):

| Value | Behavior |
| --- | --- |
| `all` (default) | Every reference must be visible at registration; refuse otherwise. |
| `available` | Skip refs whose target services are not yet provisioned (validate the ones that are visible). This is the previously-implicit always-on soft-fail heuristic, now explicit. |
| `none` | Skip all reference validation at registration; rely entirely on the mandatory instantiation-time gate. |

Bring-up order under the default `all`: provision services (start executors,
stores, lock providers) before `template register`. If your deployment pipeline
cannot guarantee that ordering — for example, the operator wants to register
templates first and wire executors after — set
`templates.ref_validation_mode: available` (or `none`) in `rimsky.yml`.

A relaxed registration mode does **not** weaken instance creation:
`POST /v1/instances` runs a mandatory static-config gate regardless of how
registration was configured. See
[`agents/errors/instance_static_config_violation.md`](agents/errors/instance_static_config_violation.md)
for the instantiation-time gate and
[`agents/errors/executor_schema_unavailable.md`](agents/errors/executor_schema_unavailable.md)
for the dispatch-time analog.

## Instance lifecycle: durable by default

Instances are **durable by default**. There is no auto-terminate-on-drain: an
instance that runs out of work stays alive (paused-idle) and resumes when new
input arrives. The only built-in self-termination path is the create-time opt-in
`terminate_after_run` flag on `POST /v1/instances`:

```json
{ "template": "<id>", "instance_key": "...", "terminate_after_run": true }
```

When `terminate_after_run` is true the instance self-terminates after its **next
frame ends** — strict "run at most once more" semantics. It is an opt-in on the
create request only; an idempotent re-create (same `template` + `instance_key`)
returns the existing instance and **ignores** the flag, exactly as `paused` does.
Operators who relied on instances cleaning themselves up on drain must now either
set `terminate_after_run` at create, or force-terminate explicitly via
`POST /v1/instances/{idOrKey}/terminate`.

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
value is NULL. Omit the block for no deployment-level cap. The scheduler
tick's parked-node sweep enforces the cap.

When the cap fires, the node is **not** resumed: the sweep emits an Error
terminal verdict with error class `park_timeout` (signal
`terminal/error/park_timeout`), the node transitions parked → `failed`, any
claims still held by the run get an auto-terminal `Abandon`, and the node-run
row is removed. A template can route the outcome through its error policy
with an `error_types: { park_timeout: ... }` override; otherwise remediate
the failed node with `rimsky admin reset` (below).

## claude-agent: configuration

The `claude-agent` reference executor is configured two ways: process environment
at startup, and per-node attributes at dispatch time. It has no separate YAML
config file — operator-managed knobs are all environment variables, including the
startup MCP-server catalog (`RIMSKY_EXECUTOR_MCP_CATALOG`, see
[MCP wiring](#mcp-wiring) below).

### Startup environment

Set on the `claude-agent` executor process. The full env-var table is in the
[bundled services catalog](services/README.md#claude-agent); the operator-relevant
subset:

| Variable | Default | Meaning |
| --- | --- | --- |
| `ANTHROPIC_API_KEY` / `CLAUDE_CODE_OAUTH_TOKEN` | — | At least one is required in non-stub mode; the executor refuses to start without one. In API-key mode the key is written to a 0600 temp file behind an `apiKeyHelper` and never enters the spawned `claude` process's environment. |
| `RIMSKY_EXECUTOR_STUB_MODE` | unset | `=1` ⇒ stub mode: the executor spawns no `claude` subprocess and returns a canned completion. The cookbook recipes run claude-agent in stub mode. |
| `RIMSKY_EXECUTOR_HOST` | `0.0.0.0` | Bind address for the gRPC executor and HTTP+JSON bridge. |
| `RIMSKY_EXECUTOR_PORT_GRPC` | `9090` | gRPC executor port. |
| `RIMSKY_EXECUTOR_PORT_HTTP` | `9190` | HTTP+JSON bridge port. |
| `RIMSKY_EXECUTOR_SILENCE_MS` | `120000` | How long the subprocess may produce no stdout before the silence-tracker acts. |
| `RIMSKY_EXECUTOR_MCP_CATALOG` | unset | Path to a YAML/JSON catalog of named MCP servers (the operator-managed authoritative source). Parsed **once at startup**; a malformed catalog fails startup loudly. See [MCP wiring](#mcp-wiring). |
| `RIMSKY_EXECUTOR_MCP_ALLOW_INLINE` | `0` (deny) | Per-deployment policy. When unset / falsy, inline `{name, url}` servers in `cli.mcp_servers` are rejected and only catalog `{ref: "<name>"}` references are accepted. Set to `1` / `true` / `yes` to opt back into inline servers. |
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
| `cli.mcp_servers` |
| `cli.required_signoffs` |
| `cli.max_signoff_attempts` |

The full expected-attributes schema is defined by the claude-agent executor itself
(in-tree at `lib/services/executors/claude-agent/`); see
[`docs/agents/examples/claude-agent-attribute-defaults.md`](agents/examples/claude-agent-attribute-defaults.md)
for a worked example of how attribute defaults flow through it.

### MCP wiring

Every dispatch always gets the executor's internal `rimsky-callback` (an HTTP MCP
server it hosts), through which the agent reports terminal outcomes
(`report_complete`, `report_error`, `report_blocked`, `report_park`), emits named
events, and reads/writes node attributes. That server is non-removable.

Beyond `rimsky-callback`, a node wires **host-declared MCP servers** per dispatch
via the `cli.mcp_servers` attribute. There are two declaration shapes; which one
is accepted is gated by the deployment's `RIMSKY_EXECUTOR_MCP_ALLOW_INLINE`
policy (default: deny):

- **Catalog reference (recommended): `{ref: "<name>"}`.** Resolves against the
  startup catalog loaded from `RIMSKY_EXECUTOR_MCP_CATALOG` (a YAML/JSON file
  mapping name → entry). Each catalog entry declares a `transport`: `http`
  (remote streamable-HTTP `{url, headers?, allowed_tools?}`), `stdio` (local
  subprocess `{command, args?, env?, allowed_tools?}`), or `module` /
  `http-loopback` (in-tree MCP module `{module, allowed_tools?}` fronted on a
  per-dispatch loopback HTTP listener). A `{ref: ...}` to an unknown catalog
  name fails the dispatch at resolution.
- **Inline declaration: `{name, url, headers?, allowed_tools?}`** — only accepted
  when `RIMSKY_EXECUTOR_MCP_ALLOW_INLINE=1`. The intended model is that the
  catalog is the authoritative server source; inline is an explicit opt-out for
  ad-hoc deployments.

Secrets in catalog `http.headers` use a `${env:VAR_NAME}` reference syntax. The
unresolved reference is what the supervisor persists and traces; resolution
against the executor's process environment happens **only at spawn time**, when
assembling the transient per-dispatch `--mcp-config`. An unset variable resolves
to the empty string (the downstream server fails loud on a missing credential).

Resolved entries are appended to the spawned CLI's `--mcp-config` so the agent
can dial them. Each server's tools are auto-allowed — a bare `mcp__<name>` allows
all of that server's tools, or an explicit `allowed_tools` narrows it to
fully-qualified `mcp__<name>__<tool>` names. The same list is re-applied on
resume (the CLI does not carry `--mcp-config` across `--resume`).

`cli.mcp_servers` pairs with the **sign-off gate** (`cli.required_signoffs`): each
`{public_key, path?}` entry must be satisfied by a valid Ed25519 signature in
`report_complete`'s `signoffs` bag before the dispatch can resolve to terminal
success. An unmet gate triggers a corrective `report_complete` retry; after
`cli.max_signoff_attempts` (default 3) the run terminal-errors with
`agent/signoff_unobtained`. The signers are typically — but not necessarily — the
host-wired validator servers.

## Per-process binding

The control-api binds its HTTP server with `RIMSKY_CONTROL_API_HOST` (default
`127.0.0.1`) and `RIMSKY_CONTROL_API_PORT` (default `8080`). The
`rimsky-all-in-one` image overrides the host to `0.0.0.0` so a published port
reaches it out of the box. The scheduler and supervisor have no public HTTP
surface; the supervisor's callback listener is configured via the
`supervisor-config.yml` `callback:` block (see
[`reference/config/supervisor-config.yml`](reference/config/supervisor-config.yml)).

The full supervisor-tuning key set (`RIMSKY_SUPERVISOR_CONFIG` YAML, also
`os.ExpandEnv`-expanded at load):

| Key | Default | Meaning |
| --- | --- | --- |
| `supervisor_id` | `<hostname>-<pid>` | Unique supervisor identity; stamped on heartbeats and audit rows. |
| `concurrency` | `4` (values < 1 are re-defaulted) | Max concurrent node dispatches. |
| `heartbeat_interval_ms` | `5000` (values < 100 re-defaulted) | Supervisor heartbeat cadence. |
| `claim_poll_interval_ms` | `1000` (values < 50 re-defaulted) | Claim-queue poll cadence. |
| `callback.host` | `0.0.0.0` | Async-callback listener bind address. |
| `callback.port` | `0` (OS-assigned) | Async-callback listener port. |
| `callback.advertise_host` | empty | Hostname executors dial back to; env `RIMSKY_SUPERVISOR_CALLBACK_ADVERTISE_HOST` wins over the YAML value. Empty or loopback → executors on other hosts/containers cannot reach the supervisor (the supervisor warns at startup). |
| `callback.advertise_port` | `0` (falls back to the bound listener port) | Port embedded in the `callback_url`; env `RIMSKY_SUPERVISOR_CALLBACK_ADVERTISE_PORT` wins over the YAML value. |

The scheduler has no YAML tuning file; its knobs are env vars:

| Variable | Default | Meaning |
| --- | --- | --- |
| `RIMSKY_SCHEDULER_TICK_MS` | `1500` (values ≤ 0 re-defaulted) | Scheduler tick cadence — the period of the ready sweeps, retention sweeps, and the parked-node sweep. |
| `RIMSKY_HEARTBEAT_TIMEOUT_MS` | `15000` (values ≤ 0 re-defaulted) | Supervisor-heartbeat staleness bound used by the scheduler's orphan sweeps. |
| `RIMSKY_SCHEDULER_ID` | `scheduler-<hostname>` (fallback `scheduler-default`) | Scheduler identity stamped on audit-log rows and orphan-claim attribution; set it only when the hostname-derived default is unstable. |

All three role binaries (`rimsky-scheduler`, `rimsky-supervisor`,
`rimsky-control-api`) also read `RIMSKY_LOG_LEVEL` (`debug` \| `info` \|
`warn` \| `error`; default `info` — unknown values fall back to `info`) and
`RIMSKY_LOG_BINARY` (an optional structured `binary` slog field, used by the
unified image to tag interleaved role logs).

## Observability: Prometheus metrics

Each rimsky binary can expose a `/metrics` endpoint.

| Binary | Settings |
| --- | --- |
| `rimsky-control-api` | `RIMSKY_METRICS_PORT` (0 = disabled, default). Bound to `RIMSKY_CONTROL_API_HOST`. |
| `rimsky-scheduler` | `RIMSKY_METRICS_PORT` and `RIMSKY_METRICS_HOST` (default `127.0.0.1`). |
| `rimsky-supervisor` | Same as scheduler. |

The control-api and supervisor also re-probe their declared peers'
observability endpoints in the background; the cadence is
`RIMSKY_OBSERVABILITY_REFRESH_INTERVAL` (Go duration syntax, default `60s`).

The full metric set (@source: `lib/control/observability/metrics.go`):

| Metric | Type | Labels | Measures |
| --- | --- | --- | --- |
| `rimsky_dispatches_total` | counter | `executor`, `terminal_class` | Dispatches by executor and terminal class. |
| `rimsky_terminal_verdicts_total` | counter | `terminal_class`, `error_class` | Terminal verdicts (e.g. `error_class="park_timeout"`). |
| `rimsky_invalidates_total` | counter | `source_kind` | Invalidates fired (e.g. `source_kind="admin"`). |
| `rimsky_claim_acquisitions_total` | counter | `producer`, `intent` | Claim acquisitions. |
| `rimsky_named_events_total` | counter | `executor`, `event_name` | NamedEvent emissions persisted. |
| `rimsky_nodes_by_state` | gauge | `state` | Node count per state (`fresh`/`stale`/`running`/`failed`/`parked`). |
| `rimsky_parked_nodes_by_reason` | gauge | `reason` | Parked nodes per `parked_reason` (`await_callback`/`snooze`). |
| `rimsky_held_frames` | gauge | — | Frames with at least one parked node. |
| `rimsky_node_runs_pending` | gauge | — | `rimsky_node_runs` rows in pending phase awaiting dispatch (queue depth). |
| `rimsky_dispatch_latency_seconds` | histogram | `executor` | Dispatch start → terminal wall-clock latency. |
| `rimsky_claim_acquisition_latency_seconds` | histogram | `producer` | Claim-acquisition transaction latency. |
| `rimsky_frame_duration_seconds` | histogram | — | Frame start → terminal duration. |
| `rimsky_parked_duration_on_resume_seconds` | histogram | — | Time spent parked, sampled at resume. |

The gauges are refreshed by a background poller (5s cadence) against live
persistence state, so they reflect the database, not in-process counters.

## Diagnostic endpoints

The control API exposes:

| Method + path | Purpose |
| --- | --- |
| `GET /v1/admin/diagnostics/held-frames` | Frames currently held. |
| `GET /v1/admin/diagnostics/parked-nodes` | Parked nodes; optional `?reason=<name>` filter. `GET /v1/diagnostics/parked` is a read-only alias on the same handler group. |
| `GET /v1/admin/diagnostics/wait-sets` | Wait-sets currently registered. |
| `POST /v1/admin/instances/{instance}/nodes/{node_id}/invalidate` | Admin invalidate. Dispatches by node state: `parked` resumes; `fresh`/`stale`/`failed` invalidate; `running` returns 409. `POST /v1/nodes/{id}/invalidate` is the non-admin form of the same action. Node states are defined in [`concepts/node.md`](concepts/node.md) (the parked state in detail in [`concepts/parked-state.md`](concepts/parked-state.md)). |
| `POST /v1/admin/lineage/prune` | Prune lineage records. |
| `GET /v1/events` | Paginated read of the append-only event log. Filters: `instance_id`, `node_id`, `kind` (an operational kind or a signal type-path — `terminal/*`, `transient/*`, `attribute/*/changed`, `event/*`, `message/*`; an unknown value is a 400 naming the valid set), `since` / `until` (RFC3339), `limit` (default 100), `cursor`. Returns `{events, next_cursor}`. |

`/v1/admin/instances/{instance}/nodes/{node_id}/invalidate` is the general-purpose
admin invalidation surface for any node state. Distinguish it from
`rimsky admin reset` (`POST /v1/nodes/{id}/reset`), the failed-node
remediation: valid **only** from state `failed` (any other state returns
409), it clears the run's error bookkeeping (`action_index`, `retry_counter`,
`error_class`) and re-enqueues the node through the frame model so the next
scheduler tick re-runs it. Invalidate marks work stale or resumes a park;
reset un-fails.

There is no scheduled-node
`force-fire` route — template-level schedules were retired; cron firing now lives
in the standalone `sensor-cron` publisher service, which sources its own messages.
The cron **expression** is not declared in `rimsky.yml`: it lives in the
template's `publishers:` block as that entry's per-instance config (a JSON
object with a required `cron` key, e.g. `{"cron": "*/5 * * * *"}`). At
instance creation the control-api resolves that config and calls the
publisher's `Subscribe` RPC (`kind: cron`, the resolved config as
`resolved_config`); `sensor-cron` parses the expression and POSTs a message
envelope to `/v1/instances/{id}/messages` on each fire. So a working cron
flow needs three pieces: the `publishers: sensor-cron` entry in `rimsky.yml`,
a running `sensor-cron` service, and the template's `publishers:` entry
carrying the expression. See [`protocols/publisher.md`](protocols/publisher.md)
and [`concepts/publisher-subscription.md`](concepts/publisher-subscription.md).

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

## Upgrading from v0.7.0

**All control-API routes moved under the `/v1/` prefix in v0.8.0** — including
the health route (`/health` → `/v1/health`). Any client, probe, dashboard, or
script pointed at a bare pre-v0.8.0 path gets a 404 with no other diagnostic.
Repoint:

- Liveness probes: `GET /v1/health`.
- Publishers and message emitters: `POST /v1/instances/{id}/messages`.
- Admin/diagnostic tooling: prefix every path (`/v1/admin/...`).

The supervisor's async-callback server is unaffected (`callback_url` already
carried `/v1/callback/...`), as are the `/metrics` listeners (separate ports,
unversioned).

**Validation-mix-in peers are re-handshaked at startup since v0.8.0.** A
claim-producer peer whose `protocols` list includes `validation` now gets an
extra `Capabilities` RPC at process startup to learn its live
`validation_supported_roles` — and a failure of that handshake **fails
startup**. A configuration that booted under v0.7.0 with an unreachable
validation peer will refuse to start under v0.8.0; bring validation-mix-in
producers up before the core processes. (Details in the startup-handshake
section above.)

**`rimsky agent` grew daemon management in v0.8.0.** `rimsky agent start`
daemonizes the host-agent by default (use `--foreground` to keep it attached),
performs a synchronous readiness handshake (a misconfigured `--proxy` URL
exits non-zero instead of forking silently), and writes `agent.pid` /
`agent.status` under `--state-dir` (default `~/.rimsky`). `rimsky agent
status` reports the live proxy-stream state, not just pid liveness; `rimsky
agent stop` SIGTERMs the daemon. See [`reference/cli.md`](reference/cli.md).

## Upgrading from v0.6.0

**`templates.ref_validation_mode` default flipped to `all` in v0.7.0.** Under
v0.6.0 the registration-time reference check was the implicit always-on
soft-fail heuristic — equivalent to mode `available` today. Deployments that
relied on registering templates before provisioning their referenced executors
now refuse to register under the v0.7.0 default. Two options:

- Reorder bring-up: provision services before `template register`. The intended
  order under `all`.
- Set `templates.ref_validation_mode: available` (or `none`) in `rimsky.yml` to
  preserve the v0.6.0 behavior.

See [Template registration and reference validation](#template-registration-and-reference-validation)
for the full mode table.

## Pre-v1 caveats

- No Helm chart or Kubernetes manifests ship yet. Deploy from the published
  images (`rimskyai/rimsky*`); a reference config lives at
  [`reference/config/rimsky.yml`](reference/config/rimsky.yml).
- The unified image (`rimsky-all-in-one`, built `FROM` the multi-role `rimsky`
  image) defaults to SQLite at `/var/lib/rimsky/state.db`. Replicas > 1 break
  (independent SQLite databases). Run the combined `rimsky` image per role with
  the postgres driver for multi-replica deployments.
- Pre-v1 has no backwards-compat guarantees on schema or wire shapes.
  Migrations may drop and recreate tables.
