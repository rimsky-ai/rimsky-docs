# Official images catalog

This catalog lists the Docker images rimsky builds. For each image: its **name**
(as tagged), **what it contains** (which binary or service), its **base image**,
and the **Dockerfile + build context** that produces it.

For the configuration surface of the services these images run, see the
[bundled services catalog](../services/README.md).

## How images are built

A few facts hold for every image below:

- **Everything builds from the rimsky checkout.** There is no `ghcr`/registry
  pull during a build — the source is compiled from the local tree.
- **The build context is the rimsky-core repo root.** Even the per-service
  Dockerfiles co-located under `lib/services/.../` use the repo root as their
  context, so the Go build can reach `lib/protocols`, `lib/services`, and
  `go.work` (the bundled services compile against the in-tree protocols module
  via the Go workspace — no published-tag pin).

### Image names

Published images use a flat, hyphenated `rimsky-<service>` form — `rimsky`,
`rimsky-all-in-one`, `rimsky-store-filesystem`, `rimsky-sensor-cron`, … built
by `make core-images` / `make service-images` and pushed to
`docker.io/rimskyai` by `make push-images`. Each is tagged with the release
version plus a floating `latest`. The Docker Hub org is **`rimskyai`** (no
hyphen — Docker Hub namespaces disallow hyphens), so the published refs are
e.g. `rimskyai/rimsky-store-filesystem:<version>`.

The published set is the **15-image release set** (4 core + 11
bundled-service). The single combined `rimsky` image carries the four role
binaries (migrate, scheduler, supervisor, control-api), selected by container
command; the role binaries are not published as separate images.

---

## Core images

Built from `dockerfiles/` with the repo root as context.

| Published name | Contains | Base image | Dockerfile |
| --- | --- | --- | --- |
| `rimsky` | All four role binaries (`rimsky-scheduler`, `rimsky-supervisor`, `rimsky-control-api`, `rimsky-migrate`) plus the `rimsky` CLI and `rimsky-entrypoint` PID-1, under one image; role selected by the container `command:` (see the entrypoint note below), persistence backend (postgres\|sqlite) by config. The no-command path runs all three roles **in a single process** via `lib/control/launch` (not three child processes), and stamps `RIMSKY_PROCESS_ROLE=unified` on its environment — the marker that gates `persistence.blob.backend: memory`. | `gcr.io/distroless/static-debian12:nonroot` | `dockerfiles/Dockerfile.rimsky` |
| `rimsky-all-in-one` | The `rimsky` image with zero-config SQLite defaults baked in (`/etc/rimsky/rimsky.yml` + `/etc/rimsky/supervisor-config.yml`) and run with no command, so `rimsky-entrypoint` migrates then runs all three roles — `rimsky-scheduler` + `rimsky-supervisor` + `rimsky-control-api` — **in a single process** under one PID-1. Binds `control-api` on `0.0.0.0:8080` (the image's only `EXPOSE`) and the supervisor's async-callback HTTP listener on `0.0.0.0:9100`, advertised to executors as `127.0.0.1` (override via `RIMSKY_SUPERVISOR_CALLBACK_ADVERTISE_HOST` when executors run outside the container). **Development only** — anonymous mode (no API keys) exposes an unauthed admin API on all interfaces. | `rimsky:<version>` (the image above — `make core-images` pins the same-release tag via the `RIMSKY_BASE` build arg; the Dockerfile's arg default is `rimsky:latest`) | `dockerfiles/Dockerfile.all-in-one` |
| `rimsky-host-agent-proxy` | The late-bound host-agent proxy service (a single binary built via the `BINARY` arg). | `gcr.io/distroless/static:nonroot` | `dockerfiles/Dockerfile.go-base` (`--build-arg BINARY=rimsky-host-agent-proxy`) |
| `rimsky-conformance` | Every protocol conformance runner in one image; pick one via `rimsky conformance <protocol>`. Probes an external impl against the rimsky protocol. | `gcr.io/distroless/static-debian12:nonroot` | `dockerfiles/Dockerfile.conformance` |

### The `rimsky` entrypoint — role selection and migrate

`rimsky-entrypoint` is the `rimsky` image's PID-1. It reads its single command
argument to decide which roles to run, and it **validates** that argument:

| Container `command:` | What runs |
| --- | --- |
| *(none)* | all three roles — `rimsky-scheduler` + `rimsky-supervisor` + `rimsky-control-api` — **in a single process** via `lib/control/launch` (not three spawned children). This path alone stamps `RIMSKY_PROCESS_ROLE=unified` on the environment — the marker that gates `persistence.blob.backend: memory` (the in-process map can only be shared when roles share the process). |
| one recognized role — `[rimsky-scheduler]` \| `[rimsky-supervisor]` \| `[rimsky-control-api]` | the entrypoint **spawns** that one role binary as a child (one role per container). Spawned children do **not** inherit `RIMSKY_PROCESS_ROLE=unified` — the marker belongs exclusively to the single-process path. |
| anything else — an unknown role, `rimsky-migrate`, or more than one argument | none — the entrypoint logs the error and **exits non-zero** (exit code 2). |

`rimsky-migrate` is not a selectable role: migrate is a one-shot init step the
entrypoint runs **synchronously before** the roles start, but only when the
invocation owns it — the no-command (all-in-one) path always migrates, and a
single-role container migrates only when its role is `rimsky-control-api`. So a
three-container split deployment migrates exactly once instead of racing three
runs or never running. Override with `RIMSKY_ENTRYPOINT_MIGRATE`: `=1` forces
migrate (e.g. a dedicated one-shot init container), `=0` skips it. Any other
non-empty value (`true`, `yes`, a typo) is a startup error — the entrypoint
exits non-zero naming the value.

### `Dockerfile.go-base` — the single-binary builder

`Dockerfile.go-base` is a parameterized builder: pass `--build-arg BINARY=<cmd>`
and it compiles `./cmd/<cmd>` into a distroless `gcr.io/distroless/static:nonroot`
image. The published `rimsky-host-agent-proxy` image uses it. It can also build
any single role binary (`rimsky-migrate`, `rimsky-scheduler`,
`rimsky-supervisor`, `rimsky-control-api`) as a standalone image, though the
release ships those four inside the combined `rimsky` image.

---

## Bundled-service images

Built from the per-service `Dockerfile.*` co-located under `lib/services/`, with
the repo root as context. All compile to a distroless static base except
claude-agent (Node on Wolfi).

| Published name | Contains | Base image | Dockerfile |
| --- | --- | --- | --- |
| `rimsky-store-filesystem` | the `store-filesystem` ClaimProducer service | `gcr.io/distroless/static:nonroot` | `lib/services/stores/filesystem/Dockerfile.filesystem` |
| `rimsky-store-postgres` | the `store-postgres` ClaimProducer service | `gcr.io/distroless/static:nonroot` | `lib/services/stores/postgres/Dockerfile.postgres` |
| `rimsky-sensor-cron` | the `sensor-cron` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-cron/Dockerfile.sensor-cron` |
| `rimsky-sensor-http` | the `sensor-http` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-http/Dockerfile.sensor-http` |
| `rimsky-sensor-object-store` | the `sensor-object-store` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-object-store/Dockerfile.sensor-object-store` |
| `rimsky-sensor-webhook` | the `sensor-webhook` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-webhook/Dockerfile.sensor-webhook` |
| `rimsky-subscriber-openlineage` | the `openlineage` subscriber binary | `gcr.io/distroless/static:nonroot` | `lib/services/subscribers/openlineage/Dockerfile.openlineage` |
| `rimsky-executor-http-node` | the `http-node` Executor service | `gcr.io/distroless/static:nonroot` | `lib/services/executors/http-node/Dockerfile.http-node` |
| `rimsky-executor-verifier-http` | the `verifier-http` Executor service | `gcr.io/distroless/static:nonroot` | `lib/services/executors/verifier-http/Dockerfile.verifier-http` |
| `rimsky-executor-verifier-shape-checks` | the `verifier-shape-checks` Executor service | `gcr.io/distroless/static:nonroot` | `lib/services/executors/verifier-shape-checks/Dockerfile.verifier-shape-checks` |
| `rimsky-executor-claude-agent` | the TypeScript `claude-agent` Executor service (Node 24; the runtime stage installs a version-pinned `@anthropic-ai/claude-code` CLI globally) | `cgr.dev/chainguard/wolfi-base` (digest-pinned) | `lib/services/executors/claude-agent/Dockerfile` |

---

## Other build targets

- **Standalone role binaries** — `Dockerfile.go-base` can build any single role
  binary (`rimsky-migrate`, `rimsky-scheduler`, `rimsky-supervisor`,
  `rimsky-control-api`) as its own image; the release ships these four inside
  the combined `rimsky` image instead.
- A deployment also runs the upstream Postgres image directly for its
  database (no rimsky build).

---

## Test-only images

| Name | Contains | Base image | Dockerfile |
| --- | --- | --- | --- |
| `stubexecutor` | the test-only stub Executor (returns `Success` for every dispatch, or a single terminal `Error` when `EXECUTOR_STUB_FORCE_ERROR=1`). Built on demand by the integration harness via testcontainers `FromDockerfile`; **never published**. | `gcr.io/distroless/static:nonroot` | `lib/services/test/stubexecutor/Dockerfile.stubexecutor` |
| `overlapproducer` | the test-only overlap ClaimProducer (advertises a prefix-containment `ScopesConflict` predicate + `SplitScope`, so the `S-claimproducer-scopesconflict-wired` scenario can exercise non-byte-equal overlap detection). Built on demand by the integration harness via testcontainers `FromDockerfile`; **never published**. | `gcr.io/distroless/static:nonroot` | `lib/services/test/overlapproducer/Dockerfile.overlapproducer` |
