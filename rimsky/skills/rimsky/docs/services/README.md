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
- **Executors, sensors, and the subscriber** are configured entirely through
  env vars (`RIMSKY_EXECUTOR_*`, `RIMSKY_SENSOR_*`, `RIMSKY_OPENLINEAGE_*`).
  There is no config file.

Sensors and the subscriber are clients of rimsky's control-api: they read
`RIMSKY_ENDPOINT` (sensors) or a Postgres DSN (subscriber) to reach it.

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
- **Dockerfile:** `lib/services/stores/postgres/Dockerfile.postgres`.
- **Note:** the items table the pick policy targets is **operator-owned**. The
  store verifies the table's schema at startup and exits if it is missing — a
  deployment must create `topics_items` (a one-shot init step) before starting
  the store.

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
- **Protocol:** Executor (plus the read-only `ExecutorObservability` mix-in over
  the HTTP bridge).
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_EXECUTOR_HTTP_NODE_HOST` | `0.0.0.0` | bind host for both transports. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_PORT` | `9091` | gRPC port. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_HTTP_PORT` | gRPC + 1 (`9092`) | HTTP/JSON bridge port. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_HTTP_BRIDGE_URL` | — | externally-reachable bridge URL advertised for observability. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_TIMEOUT_MS` | `60000` | per-request upstream timeout. |
  | `RIMSKY_EXECUTOR_HTTP_NODE_MAX_BODY_BYTES` | `10485760` | response-body size cap. |
  | `RIMSKY_EXECUTOR_STUB_MODE` | `0` | `1` short-circuits the network path (returns canned success). |
- **Ports:** gRPC `9091`, HTTP `9092` (Dockerfile `EXPOSE 9091 9092`).
- **Dockerfile:** `lib/services/executors/http-node/Dockerfile.http-node`.

### claude-agent

- **What it is:** a TypeScript reference executor that runs agentic cells by
  spawning the Claude Code CLI as a subprocess, handing it an internal MCP
  callback URL, and relaying the subprocess's structured outcome back. Always
  uses the async-handoff pattern (`StreamClose{AwaitAsyncCallback}` then a later
  HTTP callback).
- **Protocol:** Executor (with an HTTP/JSON bridge for HTTP-preferring callers).
- **Config** (env):
  | var | default | meaning |
  | --- | --- | --- |
  | `RIMSKY_EXECUTOR_HOST` | `0.0.0.0` | bind host. |
  | `RIMSKY_EXECUTOR_PORT_GRPC` | `9090` | gRPC bind port. |
  | `RIMSKY_EXECUTOR_PORT_HTTP` | `9190` | HTTP bridge bind port. |
  | `RIMSKY_EXECUTOR_STUB_MODE` | unset | `1` short-circuits the agent runtime (canned success, no CLI spawn). |
  | `RIMSKY_EXECUTOR_SILENCE_MS` | `120000` | silence-detection timeout. |
  | `RIMSKY_EXECUTOR_CALLBACK_HOST` | `127.0.0.1` | host for the internal MCP callback URL. |
  | `RIMSKY_EXECUTOR_OBSERVABILITY_HTTP_BRIDGE_URL` | — | externally-reachable bridge URL (reference-config value). |
  | `ANTHROPIC_API_KEY` / `CLAUDE_CODE_OAUTH_TOKEN` | unset | one is required in non-stub mode (API key wins). |
- **Ports:** gRPC `9090`, HTTP `9190` (Dockerfile `EXPOSE 9090 9190`).
- **Dockerfile:** `lib/services/executors/claude-agent/Dockerfile` (Node 24 on a
  Chainguard/Wolfi base; the runtime image installs the `claude` CLI globally).
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
- **Protocol:** Executor.
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

- **What it is:** a protocol-shape reference verifier executor. Beyond the
  Executor protocol it also advertises template-registration-time validation, so
  the control-api can cross-check the resolved attribute schema when a template
  using it is registered.
- **Protocol:** Executor + the `Validation` mix-in.
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
`POST /instances/{instance_id}/messages` endpoint with
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
- **Config** (env): `RIMSKY_ENDPOINT` (default `http://localhost:8080`),
  `RIMSKY_SENSOR_CRON_HOST` (`0.0.0.0`), `RIMSKY_SENSOR_CRON_PORT` (`9081`).
- **Ports:** gRPC `9081` (Dockerfile `EXPOSE 9081`).
- **Dockerfile:** `lib/services/sensors/sensor-cron/Dockerfile.sensor-cron`.

### sensor-http

- **What it is:** an HTTP-poll sensor — polls a URL on an interval and emits an
  observation when the response body changes (body-hash watermark).
- **Protocol:** Publisher.
- **Config** (env): `RIMSKY_ENDPOINT`, `RIMSKY_SENSOR_HTTP_HOST` (`0.0.0.0`),
  `RIMSKY_SENSOR_HTTP_PORT` (`9082`), and optional
  `RIMSKY_SENSOR_HTTP_STATE_DSN` — a **Postgres** DSN for persisting
  subscriptions and watermarks across restarts. When unset the sensor runs
  in-memory and resyncs via `Publisher.Subscribe` replay at supervisor startup.
- **Ports:** gRPC `9082` (Dockerfile `EXPOSE 9082`).
- **Dockerfile:** `lib/services/sensors/sensor-http/Dockerfile.sensor-http`.

### sensor-object-store

- **What it is:** an object-store sensor — polls a bucket+prefix on an interval
  and emits one observation per new object.
- **Protocol:** Publisher.
- **Backends:** the sensor advertises (via `Publisher.Capabilities`) and accepts
  (via `Publisher.Subscribe`) **exactly** the backends it has registered at
  startup — it does not advertise a backend it cannot serve. The default bundled
  binary registers only the in-memory `memory` backend, so a subscription naming
  `s3` / `gcs` / `azure` is **rejected at `Subscribe`**, not silently no-op'd at
  poll time. S3/GCS/Azure are deliberately not built into this binary (keeps the
  cloud SDKs out of the default `go.mod`); a deployment that needs one builds its
  own binary that constructs the desired object lister and registers it under a
  backend name (e.g. `s3`) before the service starts, after which the sensor
  advertises and accepts that backend automatically.
- **Config** (env): `RIMSKY_ENDPOINT`, `RIMSKY_SENSOR_OBJECT_STORE_HOST`
  (`0.0.0.0`), `RIMSKY_SENSOR_OBJECT_STORE_PORT` (`9083`), and optional
  `RIMSKY_SENSOR_OBJECT_STORE_STATE_DSN` (Postgres) for persistent state.
- **Ports:** gRPC `9083` (Dockerfile `EXPOSE 9083`).
- **Dockerfile:** `lib/services/sensors/sensor-object-store/Dockerfile.sensor-object-store`.

### sensor-webhook

- **What it is:** a webhook sensor — runs an inbound HTTP server; each
  subscription registers a route under its `path_prefix`, and inbound POSTs are
  forwarded to rimsky as publisher messages. The webhook HTTP surface is on a
  separate port from the gRPC port so it can be exposed publicly while the gRPC
  port stays private.
- **Protocol:** Publisher.
- **Config** (env): `RIMSKY_ENDPOINT`, `RIMSKY_SENSOR_WEBHOOK_HOST`
  (`0.0.0.0`), `RIMSKY_SENSOR_WEBHOOK_PORT` (gRPC, `9084`),
  `RIMSKY_SENSOR_WEBHOOK_HTTP_PORT` (inbound webhooks, `9184`), and optional
  `RIMSKY_SENSOR_WEBHOOK_STATE_DSN` (Postgres) for persistent state.
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
  dispatch. The integration harness builds it on demand (testcontainers
  `FromDockerfile`) and registers it as a peer executor so tests about stores,
  subscribers, and observability can complete the claim loop without a real
  executor. It is **never published as a product image**.
- **Protocol:** [Executor](../concepts/executor.md) (`Execute` emits a single
  terminal `StreamClose{Success}` — zero heartbeats, no attribute writeback)
  plus the read-only `ExecutorObservability` mix-in.
- **Observability / attribute schema:** `Capabilities` answers with a
  **permissive open** [expected-attributes schema](../concepts/attribute.md)
  `{"type":"object"}` (no `properties` block → read as "open shape"). This
  lets a node that carries an `attributes:` block dispatch against the stub
  and settle: the dispatch-time attribute-surface gate rejects any
  attribute-bearing node whose executor advertises **no** schema with
  `executor_schema_unavailable`, and a permissive open schema clears that
  gate. The stub keeps **no traces** — `GetTrace` and `StreamTrace` return
  gRPC `Unimplemented`, and `Capabilities` reports
  `supports_trace_get: false` / `supports_trace_stream: false`. It carries no
  `Validation` mix-in.
- **Config** (env): `EXECUTOR_STUB_BIND` — gRPC bind address (default
  `0.0.0.0:9300`).
- **Ports:** gRPC `9300` (Dockerfile `EXPOSE 9300`).
- **Dockerfile:** `lib/services/test/stubexecutor/Dockerfile.stubexecutor`.
