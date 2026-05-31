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
- **The dashboard is the one exception.** The `rimsky/dashboard` image builds
  from the separate `rimsky-dashboard` sibling repository, not from rimsky-core.

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
| `rimsky` | All four role binaries (`rimsky-scheduler`, `rimsky-supervisor`, `rimsky-control-api`, `rimsky-migrate`) plus the `rimsky` CLI and `rimsky-entrypoint` PID-1, under one image; role chosen by container command, persistence backend (postgres\|sqlite) by config. | `gcr.io/distroless/static-debian12:nonroot` | `dockerfiles/Dockerfile.rimsky` |
| `rimsky-all-in-one` | The `rimsky` image with zero-config SQLite defaults baked in; `rimsky-entrypoint` runs migrate then spawns scheduler + supervisor + control-api. **Development only.** | `rimsky:latest` (the image above, via the `RIMSKY_BASE` arg) | `dockerfiles/Dockerfile.all-in-one` |
| `rimsky-host-agent-proxy` | The late-bound host-agent proxy service (a single binary built via the `BINARY` arg). | `gcr.io/distroless/static:nonroot` | `dockerfiles/Dockerfile.go-base` (`--build-arg BINARY=rimsky-host-agent-proxy`) |
| `rimsky-conformance` | Every protocol conformance runner in one image; pick one via `rimsky conformance <protocol>`. Probes an external impl against the rimsky protocol. | `gcr.io/distroless/static-debian12:nonroot` | `dockerfiles/Dockerfile.conformance` |

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
| `rimsky-executor-claude-agent` | the TypeScript `claude-agent` Executor service (Node 24; runtime installs the `claude` CLI globally) | `cgr.dev/chainguard/wolfi-base` (digest-pinned) | `lib/services/executors/claude-agent/Dockerfile` |

---

## Other build targets

- **Dashboard** — the reference dashboard image builds from the separate
  `rimsky-dashboard` sibling repository (build context
  `${RIMSKY_DASHBOARD_REPO}`), the only image not built from rimsky-core. It is
  not part of the published release set.
- **Standalone role binaries** — `Dockerfile.go-base` can build any single role
  binary (`rimsky-migrate`, `rimsky-scheduler`, `rimsky-supervisor`,
  `rimsky-control-api`) as its own image; the release ships these four inside
  the combined `rimsky` image instead.
- A deployment also runs the upstream `postgres:15` image directly for its
  database (no rimsky build).

---

## Test-only images

| Name | Contains | Base image | Dockerfile |
| --- | --- | --- | --- |
| `stubexecutor` | the test-only stub Executor (returns `Success` for every dispatch). Built on demand by the integration harness via testcontainers `FromDockerfile`; **never published**. | `gcr.io/distroless/static:nonroot` | `lib/services/test/stubexecutor/Dockerfile.stubexecutor` |
