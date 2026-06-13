# Zero to deployed

End-to-end walkthrough: from an empty host with Docker to one rimsky node
driven to terminal. Every step states what you should see — if the observable
output does not match, stop and resolve before continuing.

This is a **journey walkthrough**, not a primitive pattern. It exists to
ground the rest of the cookbook in a runnable deployment.

## Problem

You have a host with Docker and nothing else. You want a running rimsky
deployment, a registered template that references a real bundled executor,
and a created instance whose single node reaches `succeeded` — confirming
the whole stack (control-api, supervisor, scheduler, store, executor) is
end-to-end live.

## Prerequisites

| Requirement | Verified by |
| --- | --- |
| Docker Engine ≥ 24 with the daemon running | `docker info` returns without error. |
| Outbound network to `docker.io/rimskyai` | `docker pull rimskyai/rimsky-all-in-one:v0.9.0` succeeds. |
| `curl` and `jq` on the host | `command -v curl jq`. |
| The `rimsky` CLI binary, matching the deployment version | `rimsky version` reports `rimsky v0.9.0`. The CLI ships from `make cli` on the same source tree — there is no published CLI image; install it from the GitHub Releases asset for `v0.9.0`. |
| No process already bound to `:8080` on the host | `lsof -iTCP:8080 -sTCP:LISTEN` returns no rows. |

The pinned images this journey uses:

| Image | Tag | Role |
| --- | --- | --- |
| `rimskyai/rimsky-all-in-one` | `v0.9.0` | All three role processes plus SQLite, in one container, no command — dev only. |
| `rimskyai/rimsky` | `v0.9.0` | The four role binaries plus `rimsky-entrypoint` PID-1, used in the split variant. |
| `rimskyai/rimsky-executor-http-node` | `v0.9.0` | The bundled `http-node` executor, here in stub mode so no upstream is required. |
| `postgres` | `16` | Postgres backing store for the split variant. |

See the [images catalog](../images/README.md) for the full set; the
zero-config SQLite defaults baked into the all-in-one image are in
[`reference/config/rimsky.yml`](../reference/config/rimsky.yml).

## Spine: the all-in-one container

The fastest path. One container, SQLite at `/var/lib/rimsky/state.db`,
all three role processes inside one OS process via `lib/control/launch`
(new in v0.9.0). Development only — production uses the split variant
below.

### Step 1 — Pull and start the all-in-one container

The all-in-one image ships with `executors: {}` baked in (see
[`reference/config/rimsky.yml`](../reference/config/rimsky.yml)). To drive a
real node to terminal we mount an override `rimsky.yml` that declares the
bundled `http-node` executor and run that executor as a sidecar container on a
shared docker network. Save this as `rimsky.yml` in the working directory:

```yaml
persistence:
  driver: sqlite
  sqlite:
    path: /var/lib/rimsky/state.db

executors:
  http-node:
    transport: grpc
    endpoint: rimsky-http-node:9091
    tls: off

claim_producers: {}
named_locks: {}
```

Pull the images and bring up the network:

```sh
docker pull rimskyai/rimsky-all-in-one:v0.9.0
docker pull rimskyai/rimsky-executor-http-node:v0.9.0
# Expected output (each pull): status lines ending in
#   Status: Downloaded newer image for <image>:v0.9.0

docker network create rimsky-net
```

Start the `http-node` executor sidecar first (so it is reachable when the
control-api inside the all-in-one container runs its executor handshake at
startup):

```sh
docker run -d --name rimsky-http-node --network rimsky-net \
  --network-alias rimsky-http-node \
  -e RIMSKY_EXECUTOR_STUB_MODE=1 \
  rimskyai/rimsky-executor-http-node:v0.9.0
# Expected log line in `docker logs rimsky-http-node`:
#   ... msg="http-node starting" grpc_port=9091 http_port=9092 stub_mode=true
```

Now start the all-in-one container as a foreground process in a second
terminal (or background it with `-d`):

```sh
docker run --rm \
  --name rimsky --network rimsky-net \
  -p 8080:8080 \
  -v $(pwd)/rimsky-state:/var/lib/rimsky \
  -v $(pwd)/rimsky.yml:/etc/rimsky/rimsky.yml:ro \
  rimskyai/rimsky-all-in-one:v0.9.0
```

Expected log lines (in order; the exact timestamps differ):

```
... msg="running migrations"
... msg="migrations complete" applied=9
... msg="scheduler started" binary=scheduler
... msg="supervisor started" binary=supervisor
... msg="control api listening" addr=[::]:8080 binary=control-api
```

If the migrate step exits before the role startup lines appear, the SQLite
path under `/var/lib/rimsky/` is unwritable — adjust the bind mount.

### Step 2 — Wait for the control-API to report healthy

```sh
until curl -sf http://localhost:8080/v1/health >/dev/null; do sleep 1; done
curl -s http://localhost:8080/v1/health | jq .status
# Expected output:
#   "ok"
```

The `/v1/health` route is unauth-gated and answers `ok` iff the control-api
can query its persistence layer (see the
[operator-guide health section](../operator-guide.md#deployment-the-entrypoint)).
A non-200 here means the SQLite store is unreachable.

### Step 3 — Point the CLI at the deployment

```sh
export RIMSKY_CONTROL_API=http://localhost:8080
rimsky health
# Expected output (three lines):
#   status: ok
#   endpoint: http://localhost:8080
#   supervisors: 1
```

Anonymous mode is the default in the all-in-one image, so no API key is
needed — see [`concepts/anonymous-mode.md`](../concepts/anonymous-mode.md).

### Step 4 — Register a single-node template

Save this template as `hello-template.yml`. It references the bundled
`http-node` executor by the in-rimsky name `http-node`, runs in stub mode
(no upstream URL is contacted), and carries one node that returns success
on the first dispatch:

```yaml
name: hello
version: "1.0"
frame_resolution_mode: coalesce
nodes:
  - type: greeter
    executor: http-node
    attributes:
      schema:
        type: object
        properties:
          # stub_probe short-circuits the http-node stub before the
          # transport-config check, so no upstream is needed.
          stub_probe:
            type: boolean
            default: true
          url:
            type: string
            default: "http://stub.invalid/"
          method:
            type: string
            default: "GET"
```

`frame_resolution_mode` (`coalesce` or `serial_queue`) is required by the
v0.9.0 template schema; omitting it fails registration with a 400 naming the
field as required.

The mounted `rimsky.yml` (Step 1) wires the `http-node` executor, so the
default `ref_validation_mode: all` accepts this template at registration with
no relaxation:

```sh
rimsky template register hello-template.yml
# Expected output (two lines):
#   template_hash: sha256-<64-hex>
#   tags:
```

### Step 5 — Deploy and instantiate

`template deploy` flips the template to deployable; `instance create` (or
`instance start`, a CLI alias) mints an instance that runs through the
mandatory instantiation-time static-config gate. Because `http-node` is
declared in the mounted `rimsky.yml`, the gate accepts.

```sh
rimsky template deploy sha256-<64-hex>
# Expected output:
#   sha256-<64-hex> deployed

rimsky instance create sha256-<64-hex>
# Expected output (three lines):
#   instance_id: <uuid>
#   template_hash: sha256-<64-hex>
#   node_count: 1
```

### Step 6 — Watch the node settle

The instance `state` stays `running` even after the node fires
`terminal/success` (instances are durable — see Gotchas). Terminal-state is
observed on the event log, not on the instance row. Poll the event log
directly:

```sh
INST=<instance_id>
until [ "$(curl -s "http://localhost:8080/v1/events?instance_id=$INST&kind=terminal/success" | jq '.events | length')" = "1" ]; do sleep 1; done

rimsky instance status $INST
# Expected output (Recent events / Pending breakpoint hits sections also
# emitted; elided here):
#   id:            <instance_id>
#   state:         running
#   template_hash: sha256-...
#
#   Nodes:
#   ID      TYPE     STATE  ERROR_CLASS  RETRIES  LAST_HEARTBEAT
#   <uuid>  greeter  fresh               0
```

The node's `STATE` reaches `fresh` after `terminal/success` fires — the
instance row remains `running`. Confirm via the event log:

```sh
curl -s "http://localhost:8080/v1/events?instance_id=$INST&kind=terminal/success" \
  | jq '.events | length'
# Expected output: 1
```

Tear down when done:

```sh
docker rm -f rimsky rimsky-http-node
docker network rm rimsky-net
```

The SQLite file persists under `./rimsky-state/` so a re-run resumes the
same state.

## Variant: the three-role split

The production topology — one container per role, against shared
Postgres. The v0.9.0 split harness (`lib/services/test/harness/rimsky_split.go`)
boots this exact shape end-to-end, and the migrate-once contract is
first-class: the control-api role owns migrations, scheduler and
supervisor wait for the schema.

The split lets you declare a real `rimsky.yml` (executors and stores
wired in) without depending on `ref_validation_mode` relaxation.

### Files

Create `rimsky.yml` in the working directory:

```yaml
persistence:
  driver: postgres
  postgres:
    dsn: ${RIMSKY_PG_DSN}

executors:
  http-node:
    transport: grpc
    endpoint: rimsky-http-node:9091
    tls: off

claim_producers: {}
named_locks: {}
```

Create `supervisor-config.yml` (the supervisor advertises its in-network
hostname so executors on the shared network can POST async callbacks
back; see the
[operator-guide](../operator-guide.md#per-process-binding) on
`callback.advertise_host`):

```yaml
concurrency: 4
heartbeat_interval_ms: 5000
claim_poll_interval_ms: 1000
callback:
  host: 0.0.0.0
  port: 9100
  advertise_host: rimsky-supervisor
```

### Bring up the topology

A docker network shared by every container, Postgres first, then the
`http-node` executor (so it is reachable when control-api runs its executor
handshake at startup — otherwise control-api logs
`observability.handshake.executor.unreachable` and does not retry, and
template registration fails with
`executor "http-node" expected_attributes_schema is not visible at registration`),
then control-api (which migrates), then scheduler + supervisor:

```sh
docker network create rimsky-net

docker run -d --name pg --network rimsky-net \
  -e POSTGRES_USER=rimsky -e POSTGRES_PASSWORD=rimsky -e POSTGRES_DB=rimsky \
  postgres:16
# Wait for Postgres to be ready
until docker exec pg pg_isready -U rimsky >/dev/null 2>&1; do sleep 1; done
# Expected: returns within ~5s; if not, `docker logs pg`.

# 1. Bundled http-node executor (stub mode) BEFORE control-api so the
#    executor handshake at control-api startup succeeds.
docker run -d --name rimsky-http-node --network rimsky-net \
  --network-alias rimsky-http-node \
  -e RIMSKY_EXECUTOR_STUB_MODE=1 \
  -e RIMSKY_EXECUTOR_HTTP_NODE_PORT=9091 \
  rimskyai/rimsky-executor-http-node:v0.9.0
# Expected log line in `docker logs rimsky-http-node`:
#   ... msg="http-node starting" grpc_port=9091 http_port=9092 stub_mode=true

# 2. control-api — migrate-once owner, runs the executor handshake at startup.
docker run -d --name rimsky-control-api --network rimsky-net \
  --network-alias rimsky \
  -p 8080:8080 \
  -e RIMSKY_CONFIG=/etc/rimsky/rimsky.yml \
  -e RIMSKY_PG_DSN='postgres://rimsky:rimsky@pg:5432/rimsky?sslmode=disable' \
  -e RIMSKY_CONTROL_API_HOST=0.0.0.0 \
  -e RIMSKY_CONTROL_API_PORT=8080 \
  -v $(pwd)/rimsky.yml:/etc/rimsky/rimsky.yml:ro \
  rimskyai/rimsky:v0.9.0 rimsky-control-api
# Expected log lines in `docker logs rimsky-control-api`:
#   ... msg="migrations complete" path=...
#   ... msg="control api listening" addr=[::]:8080 binary=control-api

until curl -sf http://localhost:8080/v1/health >/dev/null; do sleep 1; done
# Expected: command returns within ~10s.

# 3. Scheduler + supervisor against the migrated store.
docker run -d --name rimsky-scheduler --network rimsky-net \
  -e RIMSKY_CONFIG=/etc/rimsky/rimsky.yml \
  -e RIMSKY_PG_DSN='postgres://rimsky:rimsky@pg:5432/rimsky?sslmode=disable' \
  -v $(pwd)/rimsky.yml:/etc/rimsky/rimsky.yml:ro \
  rimskyai/rimsky:v0.9.0 rimsky-scheduler
# Expected log line: ... msg="scheduler started" binary=scheduler

docker run -d --name rimsky-supervisor --network rimsky-net \
  --network-alias rimsky-supervisor \
  -e RIMSKY_CONFIG=/etc/rimsky/rimsky.yml \
  -e RIMSKY_PG_DSN='postgres://rimsky:rimsky@pg:5432/rimsky?sslmode=disable' \
  -e RIMSKY_SUPERVISOR_CONFIG=/etc/rimsky/supervisor-config.yml \
  -e RIMSKY_SUPERVISOR_CALLBACK_ADVERTISE_HOST=rimsky-supervisor \
  -v $(pwd)/rimsky.yml:/etc/rimsky/rimsky.yml:ro \
  -v $(pwd)/supervisor-config.yml:/etc/rimsky/supervisor-config.yml:ro \
  rimskyai/rimsky:v0.9.0 rimsky-supervisor
# Expected log line: ... msg="supervisor started" binary=supervisor
```

Confirm the supervisor row is visible to the control-api:

```sh
curl -s http://localhost:8080/v1/health | jq '.supervisors | length'
# Expected output: 1
```

### Register, deploy, instantiate

The executor was up before control-api started, so the executor handshake
succeeded and the default `ref_validation_mode: all` accepts the registration
without relaxation:

```sh
export RIMSKY_CONTROL_API=http://localhost:8080
rimsky template register hello-template.yml
# Expected output (two lines):
#   template_hash: sha256-<64-hex>
#   tags:

rimsky template deploy sha256-<64-hex>
# Expected output:
#   sha256-<64-hex> deployed

rimsky instance create sha256-<64-hex>
# Expected output (three lines):
#   instance_id: <uuid>
#   template_hash: sha256-<64-hex>
#   node_count: 1
```

Watch it settle. Instances are durable — the instance `state` stays
`running` even after the node fires `terminal/success`. Terminal-state is
observed on the event log:

```sh
INST=<instance_id>
until [ "$(curl -s "http://localhost:8080/v1/events?instance_id=$INST&kind=terminal/success" | jq '.events | length')" = "1" ]; do sleep 1; done

rimsky instance status $INST
# Expected output (Recent events / Pending breakpoint hits sections also
# emitted; elided here):
#   id:            <uuid>
#   state:         running
#   template_hash: sha256-...
#
#   Nodes:
#   ID      TYPE     STATE  ERROR_CLASS  RETRIES  LAST_HEARTBEAT
#   <uuid>  greeter  fresh               0

curl -s "http://localhost:8080/v1/events?instance_id=$INST&kind=terminal/success" \
  | jq '.events | length'
# Expected output: 1
```

### Tear down the split

```sh
docker rm -f rimsky-supervisor rimsky-scheduler rimsky-http-node rimsky-control-api pg
docker network rm rimsky-net
```

## Gotchas

- **Run the all-in-one image only for local dev.** It is built `FROM` the
  `rimsky:v0.9.0` image with zero-config SQLite defaults baked in — the
  [operator guide](../operator-guide.md#pre-v1-caveats) names it
  development-only and the README of the
  [images catalog](../images/README.md) labels it the same way. Production
  deployments use the split variant against Postgres.
- **The single-process all-in-one path is new in v0.9.0.** All three roles
  run in one OS process via `lib/control/launch`, not three child processes —
  `ps -ef` inside the container shows one process, not four. The memory
  blob backend (`persistence.blob.backend: memory`) becomes legal only
  here, because the per-role processes cannot share an in-process map.
- **The split variant's container start order is load-bearing on TWO
  axes.** (1) The `http-node` executor must be reachable *before*
  control-api starts, because control-api runs its executor handshake at
  startup, logs `observability.handshake.executor.unreachable` if the
  endpoint is unreachable, and does not retry — subsequent template
  registrations then fail with
  `executor "http-node" expected_attributes_schema is not visible at registration`.
  (2) control-api owns migrations: scheduler and supervisor will refuse to
  start cleanly against an unmigrated store. The
  [v0.9.0 split harness](https://github.com/rimsky-ai/rimsky-core/blob/v0.9.0/lib/services/test/harness/rimsky_split.go)
  enforces the migrate-first invariant — it boots control-api before
  scheduler and supervisor and waits for `/v1/health` 200.
- **`RIMSKY_CONTROL_API_HOST` defaults differ between images.** The
  combined `rimsky` image keeps the upstream default `127.0.0.1` — set
  `0.0.0.0` explicitly (as in the split commands above) or the published
  port is unreachable from the host. The `rimsky-all-in-one` image
  overrides this for you (see the
  [operator-guide](../operator-guide.md#per-process-binding)).
- **The `http-node` stub mode is a real executor.** It runs the
  [executor protocol](../protocols/executor.md), validates the dispatch
  attribute bag, and closes the stream with `StreamClose{Success}` — it
  is not a no-op. A template attribute schema that violates the
  http-node attribute contract (missing `url`, invalid `method`) still
  fails with [`http_attribute_invalid`](../agents/errors/http_attribute_invalid.md)
  in stub mode, just as it would live.
- **Instances are durable by default.** The `greeter` instance above
  stays `running` even after the node fires `terminal/success` — it stays
  alive, ready to fire again if invalidated. The instance `state` row does
  **not** flip to `succeeded`; observe terminal-state on the event log
  (`GET /v1/events?kind=terminal/success`), not on the instance row. To
  clean up, `rimsky instance kill --force <id>` then `rimsky instance
  delete <id>`. The cookbook
  [README](README.md#instances-are-durable-by-default) covers this in
  detail.
