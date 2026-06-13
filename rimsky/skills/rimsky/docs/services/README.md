# Bundled services catalog

Rimsky ships a set of **reference services** — runnable implementations of its
service protocols (claim-producer, executor, publisher, lifecycle-subscriber).
They exist so an operator can stand up a working deployment without writing a
single line of protocol code, and so a service author has a worked example to
copy. Each is a standalone process with its own configuration surface, ports,
and Docker image.

This catalog answers, for each service: **what it is**, **which protocol(s) it
implements**, **how to configure it**, **which ports it listens on**, and
**which Dockerfile builds it**. For the Docker images these services produce
(names, base images, build contexts), see the [official images
catalog](../images/README.md).

## Source code

Rimsky's implementation lives in the public repository
[`github.com/rimsky-ai/rimsky-core`](https://github.com/rimsky-ai/rimsky-core).
The documentation in this directory is generated and reconciled against that
repository's latest release tag (the exact pinned version is recorded in the
plugin manifest, `.claude-plugin/plugin.json`); every fact below is
verified against the service source under `lib/services/`. The repository root
is also the build context for every image in this catalog — there is no
registry pull during a build.

## How services are wired

Reference configuration for these services lives under
[`reference/config/`](../reference/config/) — the unified `rimsky.yml` plus
per-store configs. The verifier executors, the test-only stub, and the
`openlineage` subscriber are exercised by the conformance and scenario suites
rather than wired into a default deployment. Two configuration styles appear:

- **Stores** read a YAML file whose path is named by an env var
  (`STORE_FILESYSTEM_CONFIG`, `STORE_POSTGRES_CONFIG`). The reference configs
  [`store-filesystem.yml`](../reference/config/store-filesystem.yml) and
  [`store-postgres.yml`](../reference/config/store-postgres.yml) show the
  shape. YAML values are `os.ExpandEnv`-expanded, so `${VAR}` references
  resolve at load time.
- **Executors, sensors, and the subscriber** take all their settings from
  env vars (`RIMSKY_EXECUTOR_*`, `RIMSKY_SENSOR_*`, `RIMSKY_OPENLINEAGE_*`) and
  have no service-level config file. One exception: the claude-agent executor's
  optional MCP catalog is a YAML/JSON file whose path is named by
  `RIMSKY_EXECUTOR_MCP_CATALOG` (see [claude-agent](#claude-agent)).

Sensors are clients of rimsky's control-api: they read `RIMSKY_ENDPOINT` to
reach it. The `openlineage` subscriber makes no control-api connection — it
polls the `rimsky_lineage` Postgres projection directly via
`RIMSKY_OPENLINEAGE_RIMSKY_DSN` (see [openlineage](#openlineage)).

---

## Stores

Stores implement the [ClaimProducer](../concepts/claim-producer.md) protocol —
they hand out [claims](../concepts/claim.md) over a region of state (a
filesystem path, a queue row) and enforce the claim lifecycle. See the
[ClaimProducer implementation guide](../protocols/claim-producer.md).

### store-filesystem

- **What it is:** a direct-mode store-service over a filesystem root. Each claim
  is a lock on a path under the root; an optional store-side **pick policy**
  turns a directory into a work queue.
- **Protocol:** ClaimProducer.
- **Config** (`STORE_FILESYSTEM_CONFIG` → YAML, see
  `lib/services/stores/filesystem/config-example.yml`):
  | key | meaning |
  | --- | --- |
  | `root` | **required** — filesystem root served by this store. |
  | `host` | bind host (default `0.0.0.0`). |
  | `grpc_port` | gRPC ClaimProducer port. |
  | `http_port` | HTTP/JSON bridge port. |
  | `http_bridge_url` | externally-reachable base URL of the HTTP bridge, advertised for observability. |
  | `admin_port` | admin port — **required when `pick_policies` is set**. |
  | `pick_policies` | per-selector queue config: `root`, `folder_pattern`, `on_commit`, `on_give_up`, `visibility_timeout_seconds`, `sync_strategy`. |
  | `sweep_interval_seconds` | reclaim sweep cadence (default `60`). |
- **Ports:** gRPC `9100`, HTTP `9110` (reference-config values; the Dockerfile
  `EXPOSE`s `9100 9110`). Admin port is opened only when a pick policy is
  configured.
- **Declared error classes** (advertised via
  `CapabilitiesResponse.declared_error_classes` so a template using this store
  can route the class through an `error_types:` policy and the registration
  validator accepts the name):
  | class | emitted when |
  | --- | --- |
  | `fs/root_unavailable` | the configured backing root is missing or not writable at verb time (wrong path, volume not mounted, mount gone read-only). Every producer verb (`Open` / `Release` / `Commit` / `Abandon`) refuses to attest against a vanished root, so the operator-misconfiguration case crosses the wire as the store's own class instead of an anonymous gRPC failure. |
- **Dockerfile:** `lib/services/stores/filesystem/Dockerfile.filesystem`.

### store-postgres

- **What it is:** a direct-mode store-service backed by Postgres, with store-side
  pick-policy support over operator-owned items tables. Can optionally also
  register the Executor and LifecycleSubscriber protocols in the same binary
  (the atomic-staging-with-verifier pattern).
- **Protocol:** ClaimProducer; optionally [Executor](../concepts/executor.md)
  (`enable_executor`) and [LifecycleSubscriber](../concepts/lifecycle-subscriber.md)
  (`enable_lifecycle`) as same-binary mix-ins.
- **Config** (`STORE_POSTGRES_CONFIG` → YAML, see
  `lib/services/stores/postgres/config-example.yml`):
  | key | meaning |
  | --- | --- |
  | `connection` | **required** — Postgres DSN. |
  | `write_semantics` | the single write-semantics value this store realizes on every `Open` (default `staged_async`). |
  | `pick_policies` | per-selector queue config over an operator-owned items table: `items_table` (must be a valid lowercase SQL identifier), `on_commit`, `on_give_up`, `visibility_timeout_seconds`. |
  | `host` | bind host (default `0.0.0.0`). |
  | `grpc_port` | gRPC ClaimProducer port. |
  | `http_port` | HTTP/JSON bridge port. |
  | `http_bridge_url` | externally-reachable HTTP bridge URL. |
  | `admin_port` | admin port (items seeding). |
  | `sweep_interval_seconds` | reclaim sweep cadence (default `30`). |
  | `enable_lifecycle` | also register the LifecycleSubscriber protocol. |
  | `enable_executor` | also register the Executor protocol (verifier pattern). |
- **Ports:** gRPC `9101`, HTTP `9111`, admin `9121` (reference-config values; the
  Dockerfile `EXPOSE`s `9101 9111 9121`).
- **Declared error classes** (advertised via
  `CapabilitiesResponse.declared_error_classes` — the ClaimProducer surface
  only; the Executor mix-in's vocabulary is the separate
  `ExecutorObservability.Capabilities` surface):
  | class | emitted when |
  | --- | --- |
  | `pg/claim_unavailable` | `Open` cannot grant the requested claim because the targeted scope is in-flight or exhausted (the `Unavailable.error_class` arm). |
  | `pg/swap_failed` | the atomic-staging commit step lost the swap (the staged child no longer matches the parent the producer staged against) — surfaced as the `google.rpc.ErrorInfo` Reason on the faulted verb. |
- **Dockerfile:** `lib/services/stores/postgres/Dockerfile.postgres`.
- **Note:** the items table the pick policy targets is **operator-owned**. The
  store verifies the table's schema at startup and exits if it is missing — a
  deployment must create the configured `items_table` (`topics_items` in the
  reference config) as a one-shot init step before starting the store.
  The startup check (`verifyItemsTable`, `lib/services/stores/postgres/store/store.go`)
  queries `information_schema.columns` in the connection's current schema and
  requires these columns with exactly these `data_type` values (extra columns
  are allowed):
  | column | required type |
  | --- | --- |
  | `item_id` | `text` |
  | `payload` | `jsonb` |
  | `state` | `text` |
  | `claim_token` | `text` |
  | `claimed_at` | `timestamp with time zone` |
  | `enqueued_at` | `timestamp with time zone` |
  | `priority` | `integer` |
  | `sequence` | `bigint` |
  A working init DDL (the shape the store's own tests create — constraints,
  defaults, and indexes are not verified at startup but match how the store
  reads the table):
  ```sql
  CREATE TABLE topics_items (
      item_id     TEXT PRIMARY KEY,
      payload     JSONB NOT NULL,
      state       TEXT NOT NULL DEFAULT 'available',
      claim_token TEXT,
      claimed_at  TIMESTAMPTZ,
      enqueued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      priority    INTEGER NOT NULL DEFAULT 0,
      sequence    BIGSERIAL
  );
  CREATE INDEX topics_items_available_idx   ON topics_items (priority DESC, sequence) WHERE state = 'available';
  CREATE INDEX topics_items_in_progress_idx ON topics_items (claim_token) WHERE state = 'in_progress';
  ```

---

## Executors

Executors implement the [Executor](../concepts/executor.md) protocol — they run
a single cell's work in response to an `Execute` dispatch and stream back
`ExecuteEvent`s ending in a terminal `StreamClose`. See the [Executor
implementation guide](../protocols/executor.md).

### http-node

- **What it is:** the reference node executor for the `http.request@1` node
  type. Performs an outbound HTTP request driven by the dispatch attributes and
  emits the response as the terminal `Success.attributes_delta`.
- **Protocol:** Executor, plus the read-only `ExecutorObservability` mix-in. The
  mix-in is registered on the gRPC server and additionally mirrored onto the
  HTTP/JSON bridge listener under `/observability/v1/*` — both transports serve
  it (`lib/services/executors/http-node/main.go`).
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_EXECUTOR_HTTP_NODE_HOST` | `0.0.0.0` | bind host for both transports. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_PORT` | `9091` | gRPC port. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_HTTP_PORT` | gRPC + 1 (`9092`) | HTTP/JSON bridge port. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_HTTP_BRIDGE_URL` | — | externally-reachable bridge URL advertised for observability. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_TIMEOUT_MS` | `60000` | per-request upstream timeout. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_MAX_BODY_BYTES` | `10485760` | response-body size cap. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_ERROR_CLASS_FIELD` | `error_class` | JSON field read from a parseable 4xx upstream error body to populate the emitted `error_class`. A per-node `attributes.error_class_field` overrides this for an individual dispatch; when the field is absent from the body, classification falls back to the stable `http/request_invalid/_unspecified` leaf. |
  | `RIMSKY_EXECUTOR_STUB_MODE` | `0` | `1` short-circuits the network path (returns canned success). |
- **Ports:** gRPC `9091`, HTTP `9092` (Dockerfile `EXPOSE 9091 9092`).
- **Rate-limit handling:** an upstream `429 Too Many Requests` that is **not**
  listed in the dispatch's `expect_status` resolves to a terminal `StreamClose
  Park` outcome with `reason = PARK_REASON_SNOOZE` and a `resume_at` populated
  from the upstream's `Retry-After` header per RFC 9110 §10.2.3 (delta-seconds,
  e.g. `7` → `now + 7s`; HTTP-date → that instant; absent or malformed header →
  `now + 30s`). The supervisor's existing parked-node auto-wake sweep
  re-dispatches the node at `resume_at` (`resume_reason = "deadline_elapsed"`).
  A 429 the template **does** list in `expect_status` is a normal success per
  the operator's declared contract.
- **Dockerfile:** `lib/services/executors/http-node/Dockerfile.http-node`.

### claude-agent

- **What it is:** a TypeScript reference executor that runs agentic cells by
  spawning the Claude Code CLI as a subprocess, handing it an internal MCP
  callback URL, and relaying the subprocess's structured outcome back. Always
  uses the async-handoff pattern (`StreamClose{AwaitAsyncCallback}` then a later
  HTTP callback).
- **Protocol:** Executor, plus the full `ExecutorObservability` mix-in
  (`Capabilities` + `GetTrace` + `StreamTrace`, backed by an in-process trace
  ledger). Both
  services are registered on the gRPC server and mirrored onto the HTTP/JSON
  bridge listener — Executor as the bridge's dispatch routes, observability
  under `/observability/v1/*` (`src/server.ts`, `src/http-bridge.ts`).
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_EXECUTOR_HOST` | `0.0.0.0` | bind host. |
  | `RIMSKY_EXECUTOR_PORT_GRPC` | `9090` | gRPC bind port. |
  | `RIMSKY_EXECUTOR_PORT_HTTP` | `9190` | HTTP bridge bind port. |
  | `RIMSKY_EXECUTOR_STUB_MODE` | unset | `1` short-circuits the agent runtime (canned success, no CLI spawn). |
  | `RIMSKY_EXECUTOR_SILENCE_MS` | `120000` | silence-detection timeout. |
  | `RIMSKY_EXECUTOR_CALLBACK_HOST` | `127.0.0.1` | host for the internal MCP callback URL. |
  | `RIMSKY_EXECUTOR_CLAUDE_BINARY` | unset | path to the Claude CLI binary. Empty → bare `claude` PATH lookup. Read once at startup and injected into **both** transports (gRPC and HTTP) so the override applies uniformly. |
  | `RIMSKY_OBS_IDLE_TIMEOUT_MS` | `300000` | idle-close timeout for an observability `StreamTrace` (gRPC) / trace-stream (HTTP) with no events — prevents a stream for an unknown `dispatch_id` from pinning server resources indefinitely. |
  | `RIMSKY_EXECUTOR_OBSERVABILITY_HTTP_BRIDGE_URL` | — | externally-reachable bridge URL (reference-config value). |
  | `RIMSKY_EXECUTOR_MCP_CATALOG` | unset | Path to a YAML/JSON catalog file (one parser handles both — YAML is a JSON superset) of named MCP servers a dispatch can reference by `{ ref: "<name>" }` in its `cli.mcp_servers` attribute. Each entry declares a `transport` (`http` / `stdio` / `module` / `http-loopback`). Parsed **once at startup** and shared by both transports; a malformed catalog fails startup loudly. |
  | `RIMSKY_EXECUTOR_MCP_ALLOW_INLINE` | `0` (deny) | per-deployment policy. When unset / falsy, inline `{ name, url }` MCP servers in `cli.mcp_servers` are rejected and only catalog `{ ref: "<name>" }` references are accepted — the catalog is the authoritative server source. Set to `1` / `true` / `yes` to opt back into inline servers. |
  | `RIMSKY_DISPATCH_MAX_USD` | unset | deployment-wide spend cap, passed to the CLI as `--max-budget-usd`. The **fallback** when a dispatch carries no per-template `attributes.cli.max_budget_usd` — the per-template value wins when set. Unset → no cap. |
  | `RIMSKY_EXECUTOR_DECLARED_EVENTS` | unset (empty list) | comma-separated names of events agents may emit via the `emit_named_event` MCP tool (whitespace trimmed, empty segments dropped). The resolved list is advertised as `Capabilities.declared_events` (so rimsky's registration-time `subscribes:` cross-check sees the names) and gates `emit_named_event` — an undeclared name is rejected. |
  | `ANTHROPIC_API_KEY` / `CLAUDE_CODE_OAUTH_TOKEN` | unset | one is required in non-stub mode (API key wins). |
- **Ports:** gRPC `9090`, HTTP `9190` (Dockerfile `EXPOSE 9090 9190`).
- **Dockerfile:** `lib/services/executors/claude-agent/Dockerfile` (Node 24 on a
  digest-pinned Chainguard/Wolfi base; the runtime stage installs a
  version-pinned `@anthropic-ai/claude-code` CLI globally).
- **Note:** depends on the `@rimsky-ai/protocols` wire bindings, resolved in-tree
  via an npm `file:` link to `lib/protocols` (the Go workspace build, not the
  published npm tarball). The same `lib/protocols` is what gets published to npm
  as `@rimsky-ai/protocols` for external consumers. This is the lone
  Apache-2.0-licensed bundled service; its own package
  `@rimsky/executor-claude-agent` is `"private": true` and ships only as the
  Docker image — never to npm.

### verifier-http

- **What it is:** a reference verifier executor that POSTs a payload to a URL and
  checks the response status — the executor half of the
  atomic-staging-with-verifier pattern.
- **Protocol:** Executor, plus the minimum `ExecutorObservability` mix-in (a
  `Capabilities` RPC advertising the error vocabulary and a permissive-open
  attribute schema; no trace store — `supports_trace_get` /
  `supports_trace_stream` are false).
- **Attributes read:** `url` (**required**), `body` (sent as the POST body),
  `expected_status` (default `[200]`), `timeout_ms`, `class_field` (default
  `class`), `stub_probe`.
- **Declared error classes** (advertised via
  `ObservabilityCapabilities.DeclaredErrorClasses`):
  | class | emitted when |
  | --- | --- |
  | `verifier/attribute_invalid` | the attribute bag is missing required keys, malformed, or non-JSON-serializable. |
  | `verifier/network_error` | transport failure dialing the endpoint (DNS, refused, reset) that is not a timeout. |
  | `verifier/timeout` | request deadline exceeded or transport timeout. |
  | `verifier/check_failed` | the endpoint responded outside `expected_status` and its 4xx/5xx body carried no parseable `class_field` token — the stable fallback so `verifier/check_failed/*` policies still match taxonomy-less upstreams. |
  | `verifier/check_failed/*` | typed leaves populated from the upstream's `class_field` JSON token (per-node `attributes.class_field`, default `class`); the suffix is the upstream's verbatim class string. Mirrors http-node's `http/request_invalid/*` discipline. |
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_EXECUTOR_VERIFIER_HTTP_HOST` | `0.0.0.0` | bind host. |
  | `RIMSKY_EXECUTOR_VERIFIER_HTTP_PORT` | `9096` | gRPC port. |
  | `RIMSKY_EXECUTOR_STUB_MODE` | `0` | `1` enables stub mode. |
- **Ports:** gRPC `9096` (default; the Dockerfile declares no `EXPOSE`).
- **Dockerfile:** `lib/services/executors/verifier-http/Dockerfile.verifier-http`.
- **Note:** not wired into a default deployment; exercised by the
  conformance and scenario suites.

### verifier-shape-checks

- **What it is:** a protocol-shape reference verifier executor. Runs an array of
  shape-check primitives (e.g. `no_nulls`, `pk_unique`, `row_count_absolute`)
  over an in-memory `rows` payload and emits a Success or Error terminal based
  on aggregate pass/fail. Beyond the Executor protocol it also advertises
  template-registration-time validation, so the control-api can cross-check the
  resolved attribute schema when a template using it is registered.
- **Protocol:** Executor + the `Validation` mix-in (`role=executor` only) + the
  minimum `ExecutorObservability` mix-in (`Capabilities` advertising the
  vocabulary and a permissive-open attribute schema; no trace store —
  `supports_trace_get` / `supports_trace_stream` are false).
- **Attributes read:** `checks` (**required**, non-empty array of
  `{kind, config, severity?}`), `rows` (array of objects to verify; missing
  rows is acceptable — checks like `row_count_absolute` fire on empty input as
  a pass/fail signal), `stub_probe`.
- **Registered check kinds:** `no_nulls`, `nullable_fields_present`,
  `pk_unique`, `row_count_ratio`, `row_count_absolute`, `value_in_set`,
  `regex_match`, `numeric_range`. The runtime dispatcher rejects any other
  `kind` at dispatch; the `Validation` mix-in warns
  (`unknown_check_kind`) at template registration.
- **Per-check severity:** each `checks[i]` may carry a `severity` of `error`
  (default) or `warning`. An **error-severity** failure blocks the commit and
  the dispatch terminates with `verifier/check_failed/<first-blocking-kind>`. A
  **warning-severity** failure is non-blocking — the dispatch still
  succeeds, and every warning is surfaced under
  `Success.attributes_delta.verifier_warnings` (plus a
  `verifier_warning_count` count) so the operator sees the soft signal. An
  unknown severity string is rejected with `verifier/attribute_invalid` rather
  than silently coerced.
- **Declared error classes** (advertised via
  `ObservabilityCapabilities.DeclaredErrorClasses`):
  | class | emitted when |
  | --- | --- |
  | `verifier/attribute_invalid` | the attribute bag is missing required keys (`checks` absent or non-array), an entry is malformed, or a `severity` value is unknown. |
  | `verifier/check_failed/*` | one or more error-severity shape checks failed; the suffix is the failing check's `kind` (e.g. `verifier/check_failed/pk_unique`, `verifier/check_failed/row_count_absolute`). Patterns ending in `*` are prefix patterns; the validator recognises `verifier/check_failed/*` as a wildcard for `error_types:` matching. |
- **Validation findings** (emitted by the `Validation` mix-in at template
  registration when `role=executor`):
  | class | severity | emitted when |
  | --- | --- | --- |
  | `unsupported_role` | error | `role` is anything other than `executor`. |
  | `missing_context` | error | `ValidateRequest.context.executor` is unset. |
  | `invalid_attribute` | error | `attributes_schema` is not valid JSON. |
  | `missing_checks` | error | the merged effective attribute schema declares no `checks` property via either `default:` or `source:` (and the property is not `readOnly`). |
  | `empty_checks` | error | a static `default:` for `checks` is present but empty. |
  | `malformed_check` | error | a `checks[i]` entry is not an object. |
  | `missing_check_kind` | error | a `checks[i]` entry has no `kind`. |
  | `unknown_check_kind` | warning | a `checks[i].kind` is not a registered shape check; the runtime will reject it at dispatch, but registration only warns so the template can still deploy alongside other valid checks. |
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_EXECUTOR_VERIFIER_SHAPE_CHECKS_HOST` | `0.0.0.0` | bind host. |
  | `RIMSKY_EXECUTOR_VERIFIER_SHAPE_CHECKS_PORT` | `9095` | gRPC port. |
  | `RIMSKY_EXECUTOR_STUB_MODE` | `0` | `1` enables stub mode. |
- **Ports:** gRPC `9095` (default; the Dockerfile declares no `EXPOSE`).
- **Dockerfile:** `lib/services/executors/verifier-shape-checks/Dockerfile.verifier-shape-checks`.
- **Note:** not wired into a default deployment; exercised by the
  conformance and scenario suites.

---

## Sensors (Publishers)

Sensors are [sensors](../concepts/sensor.md) that implement the
[Publisher](../concepts/publisher.md) protocol: they observe an external signal
and POST message envelopes to control-api's
`POST /v1/instances/{id}/messages` endpoint with
`sender_kind: "publisher"`. See the [Publisher implementation
guide](../protocols/publisher.md). The v1 contract is single-replica per sensor
binary. All sensors share two env vars:

- `RIMSKY_ENDPOINT` — base URL of the rimsky control-api (default
  `http://localhost:8080`).
- `RIMSKY_SENSOR_<NAME>_HOST` / `_PORT` — gRPC bind host/port for the
  Publisher service.

### sensor-cron

- **What it is:** a cron sensor — fires observations on a cron schedule. Replaces
  the retired internal scheduler cron-fire path.
- **Protocol:** Publisher.
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_ENDPOINT` | `http://localhost:8080` | base URL of the rimsky control-api. |
  | `RIMSKY_SENSOR_CRON_HOST` | `0.0.0.0` | gRPC bind host. |
  | `RIMSKY_SENSOR_CRON_PORT` | `9081` | gRPC bind port. |
  | `RIMSKY_SENSOR_CRON_STATE_DSN` | unset (in-memory) | optional **Postgres** DSN for persisting active cron subscriptions and `next_fire_at` watermarks across restarts. |

  The `STATE_DSN` durability property: a restarted binary fires on the
  originally-scheduled window rather than recomputing
  `sched.Next(restartTime)` and skipping the in-flight window. When unset the
  sensor runs in-memory and resyncs via `Publisher.Subscribe` replay at
  control-api startup. SQLite is not supported (the state schema uses
  Postgres-only `now()` and `TIMESTAMPTZ`).
- **Ports:** gRPC `9081` (Dockerfile `EXPOSE 9081`).
- **Dockerfile:** `lib/services/sensors/sensor-cron/Dockerfile.sensor-cron`.

### sensor-http

- **What it is:** an HTTP-poll sensor — polls a URL on an interval and emits an
  observation when the response body changes (body-hash watermark).
- **Protocol:** Publisher.
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_ENDPOINT` | `http://localhost:8080` | base URL of the rimsky control-api. |
  | `RIMSKY_SENSOR_HTTP_HOST` | `0.0.0.0` | gRPC bind host. |
  | `RIMSKY_SENSOR_HTTP_PORT` | `9082` | gRPC bind port. |
  | `RIMSKY_SENSOR_HTTP_STATE_DSN` | unset (in-memory) | optional **Postgres** DSN for persisting subscriptions and body-hash watermarks across restarts. |

  When `STATE_DSN` is unset the sensor runs in-memory and resyncs via
  `Publisher.Subscribe` replay at control-api startup. SQLite is not
  supported (the state schema uses Postgres-only `now()` and `TIMESTAMPTZ`).
- **Ports:** gRPC `9082` (Dockerfile `EXPOSE 9082`).
- **Dockerfile:** `lib/services/sensors/sensor-http/Dockerfile.sensor-http`.

### sensor-object-store

- **What it is:** an object-store sensor — polls a bucket+prefix on an interval
  and emits one observation per new object.
- **Protocol:** Publisher.
- **Backends:** the sensor advertises (via `Publisher.Capabilities`) and accepts
  (via `Publisher.Subscribe`) **exactly** the backends it has registered at
  startup — it does not advertise a backend it cannot serve. The default bundled
  binary registers two SDK-free backends: the in-memory `memory` backend
  (always), and the `filesystem` backend (only when
  `RIMSKY_SENSOR_OBJECT_STORE_FS_ROOT` is set — unset omits it from
  `Capabilities` and `Subscribe` rejects it). A subscription naming
  `s3` / `gcs` / `azure` is **rejected at `Subscribe`**, not silently no-op'd at
  poll time. S3/GCS/Azure are deliberately not built into this binary (keeps the
  cloud SDKs out of the default `go.mod`); a deployment that needs one builds its
  own binary that constructs the desired object lister and registers it via
  `SetBackend` under a backend name (e.g. `s3`) before the service starts, after
  which the sensor advertises and accepts that backend automatically.
- **Filesystem backend semantics:** the env var names a base directory treated
  as the object-store root; buckets map to first-level subdirectories under it,
  and every regular file under `<root>/<bucket>/` is one object (subdirectories
  walked recursively, symlinks not followed). The object `Name` is the path
  relative to the bucket root, forward-slash separated regardless of OS. The
  ETag is an FNV-64a hash of the file contents, folded into the
  rimsky-side idempotency key — so a file mutated in place (same name, new
  contents) re-emits; whether a name-equal object counts as "new" is still the
  subscription's `watermark_field` (`name` or `last_modified`) decision. A
  missing bucket directory lists as empty (no error); other listing errors skip
  the poll without advancing the watermark.
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_ENDPOINT` | `http://localhost:8080` | base URL of the rimsky control-api. |
  | `RIMSKY_SENSOR_OBJECT_STORE_HOST` | `0.0.0.0` | gRPC bind host. |
  | `RIMSKY_SENSOR_OBJECT_STORE_PORT` | `9083` | gRPC bind port. |
  | `RIMSKY_SENSOR_OBJECT_STORE_FS_ROOT` | unset (backend omitted) | optional — registers the `filesystem` backend rooted at the given directory (see backend semantics above). |
  | `RIMSKY_SENSOR_OBJECT_STORE_STATE_DSN` | unset (in-memory) | optional **Postgres** DSN for persistent subscription/watermark state. |
- **Ports:** gRPC `9083` (Dockerfile `EXPOSE 9083`).
- **Dockerfile:** `lib/services/sensors/sensor-object-store/Dockerfile.sensor-object-store`.

### sensor-webhook

- **What it is:** a webhook sensor — runs an inbound HTTP server; each
  subscription registers a route under its `path_prefix`, and inbound POSTs are
  forwarded to rimsky as publisher messages. The webhook HTTP surface is on a
  separate port from the gRPC port so it can be exposed publicly while the gRPC
  port stays private.
- **Protocol:** Publisher.
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_ENDPOINT` | `http://localhost:8080` | base URL of the rimsky control-api. |
  | `RIMSKY_SENSOR_WEBHOOK_HOST` | `0.0.0.0` | bind host. |
  | `RIMSKY_SENSOR_WEBHOOK_PORT` | `9084` | gRPC bind port. |
  | `RIMSKY_SENSOR_WEBHOOK_HTTP_PORT` | `9184` | inbound-webhook HTTP port (the publicly exposable surface; the gRPC port stays private). |
  | `RIMSKY_SENSOR_WEBHOOK_STATE_DSN` | unset (in-memory) | optional **Postgres** DSN for persistent subscription state. |
- **Ports:** gRPC `9084`, inbound-webhook HTTP `9184` (Dockerfile
  `EXPOSE 9084 9184`).
- **Dockerfile:** `lib/services/sensors/sensor-webhook/Dockerfile.sensor-webhook`.

---

## Subscribers

### openlineage

- **What it is:** a standalone OpenLineage subscriber. It polls rimsky's lineage
  projection (`rimsky_lineage`) for new rows since a stored cursor and emits
  [OpenLineage](https://openlineage.io/) 1.x JSON events to a configured backend
  (Marquez, DataHub, …). The transport is polling, not live lifecycle events.
- **Protocol:** none on the wire — it is a passive reader of the
  [lineage](../concepts/lineage.md) projection rather than a
  [LifecycleSubscriber](../concepts/lifecycle-subscriber.md) listener. (See the
  [LifecycleSubscriber guide](../protocols/lifecycle-subscriber.md) for the
  event-driven alternative rimsky also offers.)
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_OPENLINEAGE_RIMSKY_DSN` | — | **required** — Postgres DSN of the rimsky lineage projection. |
  | `RIMSKY_OPENLINEAGE_STATE_DSN` | rimsky DSN | DSN for the subscriber's own cursor state. |
  | `RIMSKY_OPENLINEAGE_BACKEND_URL` | — | OpenLineage HTTP receiver; events are POSTed to `{url}/api/v1/lineage`. Empty → events are not POSTed (no-op); the subscriber still polls `rimsky_lineage` and advances its cursor. |
  | `RIMSKY_OPENLINEAGE_NAMESPACE` | `rimsky` | OpenLineage namespace stamped on every event. |
  | `RIMSKY_OPENLINEAGE_POLL_INTERVAL` | `5s` | projection poll cadence. |
  | `RIMSKY_OPENLINEAGE_BATCH_SIZE` | `200` | max rows processed per poll. |
- **Ports:** none — outbound HTTP only (Postgres in, OpenLineage HTTP out). The
  Dockerfile `EXPOSE`s no port.
- **Dockerfile:** `lib/services/subscribers/openlineage/Dockerfile.openlineage`.

---

## Test-only services

### stubexecutor

- **What it is:** a **test-only** Executor that returns `Success` for every
  dispatch (or, with `EXECUTOR_STUB_FORCE_ERROR=1`, a single terminal `Error`
  with `error_class: "stub/forced_error"` — used by held-subgraph abandon-case
  scenarios). The integration harness builds it on demand (testcontainers
  `FromDockerfile`) and registers it as a peer executor so tests about stores,
  subscribers, and observability can complete the claim loop without a real
  executor. It is **never published as a product image**.
- **Protocol:** [Executor](../concepts/executor.md) (`Execute` emits a single
  terminal `StreamClose` — `Success` by default, or `Error{stub/forced_error}`
  when `EXECUTOR_STUB_FORCE_ERROR=1`; zero heartbeats, no attribute writeback)
  plus the read-only `ExecutorObservability` mix-in.
- **Observability / attribute schema:** `Capabilities` answers with a
  **permissive open** [expected-attributes schema](../concepts/attribute.md)
  `{"type":"object"}` (no `properties` block → read as "open shape"). This
  lets a node that carries an `attributes:` block dispatch against the stub
  and settle: the dispatch-time attribute-surface gate rejects any
  attribute-bearing node whose executor advertises **no** schema with
  `executor_schema_unavailable`, and a permissive open schema clears that
  gate. In `EXECUTOR_STUB_FORCE_ERROR=1` mode, `Capabilities` additionally
  advertises `stub/forced_error` in `DeclaredErrorClasses` so a template can
  route that class through an `error_types:` policy without the registration
  validator rejecting the unknown class. The stub keeps **no traces** —
  `GetTrace` and `StreamTrace` return gRPC `Unimplemented`, and `Capabilities`
  reports `supports_trace_get: false` / `supports_trace_stream: false`. It
  carries no `Validation` mix-in.
- **Config** (env): `EXECUTOR_STUB_BIND` — gRPC bind address (default
  `0.0.0.0:9300`); `EXECUTOR_STUB_FORCE_ERROR` — `1` flips the stub from
  success-only to error-only (single terminal `Error` per dispatch).
- **Ports:** gRPC `9300` (Dockerfile `EXPOSE 9300`).
- **Dockerfile:** `lib/services/test/stubexecutor/Dockerfile.stubexecutor`.

### overlapproducer

- **What it is:** a **test-only** ClaimProducer that advertises a non-trivial
  `ScopesConflict` predicate (prefix-containment over selector strings) plus
  `SplitScope`. Exists so the `S-claimproducer-scopesconflict-wired` scenario
  can drive a real rimsky stack against a producer whose overlapping scopes are
  not byte-equal — the case rimsky's default byte-equal conflict check cannot
  detect. The integration harness builds it on demand (testcontainers
  `FromDockerfile`) and registers it as a peer claim-producer on the shared
  docker network. It is **never published as a product image**.
- **Protocol:** [ClaimProducer](../concepts/claim-producer.md), advertising
  `supports_scopes_conflict` and `supports_split_scope`.
- **Config** (env): `OVERLAP_PRODUCER_BIND` — gRPC bind address (default
  `0.0.0.0:9400`).
- **Ports:** gRPC `9400` (Dockerfile `EXPOSE 9400`).
- **Dockerfile:** `lib/services/test/overlapproducer/Dockerfile.overlapproducer`.
