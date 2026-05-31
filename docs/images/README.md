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

### Two naming schemes

The same images appear under two names depending on how they are built:

- **Published registry names** (`make core-images` / `make service-images`, and
  `make push-images` to `docker.io/rimskyai`): a flat, hyphenated
  `rimsky-<service>` form — `rimsky`, `rimsky-all-in-one`,
  `rimsky-store-filesystem`, `rimsky-sensor-cron`, … Each is tagged with the
  release version plus a floating `latest`. The Docker Hub org is **`rimskyai`**
  (no hyphen — Docker Hub namespaces disallow hyphens), so the published refs
  are e.g. `rimskyai/rimsky-store-filesystem`.
- **Compose-local names** (`deploy/docker-compose.yml`): a slashed
  `rimsky/<service>` form — `rimsky/control-api`, `rimsky/store-filesystem`,
  `rimsky/sensor-cron`, … These are local build tags the compose stack assigns
  to the images it builds on the spot; they are not pushed anywhere.

The tables below list both. The published set is the **15-image release set**
(4 core + 11 bundled-service). The compose stack builds a partly different set:
it splits the single `rimsky` image into per-role binaries (migrate, scheduler,
supervisor, control-api) via `Dockerfile.go-base`, and omits the all-in-one,
host-agent-proxy, conformance, verifier, and openlineage images.

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
image. The published `rimsky-host-agent-proxy` image uses it, and the compose
stack uses it to build the individual role binaries (`rimsky-migrate`,
`rimsky-scheduler`, `rimsky-supervisor`, `rimsky-control-api`) as separate
images rather than one combined `rimsky` image.

---

## Bundled-service images

Built from the per-service `Dockerfile.*` co-located under `lib/services/`, with
the repo root as context. All compile to a distroless static base except
claude-agent (Node on Wolfi).

| Published name | Compose tag | Contains | Base image | Dockerfile |
| --- | --- | --- | --- | --- |
| `rimsky-store-filesystem` | `rimsky/store-filesystem` | the `store-filesystem` ClaimProducer service | `gcr.io/distroless/static:nonroot` | `lib/services/stores/filesystem/Dockerfile.filesystem` |
| `rimsky-store-postgres` | `rimsky/store-postgres` | the `store-postgres` ClaimProducer service | `gcr.io/distroless/static:nonroot` | `lib/services/stores/postgres/Dockerfile.postgres` |
| `rimsky-sensor-cron` | `rimsky/sensor-cron` | the `sensor-cron` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-cron/Dockerfile.sensor-cron` |
| `rimsky-sensor-http` | `rimsky/sensor-http` | the `sensor-http` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-http/Dockerfile.sensor-http` |
| `rimsky-sensor-object-store` | `rimsky/sensor-object-store` | the `sensor-object-store` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-object-store/Dockerfile.sensor-object-store` |
| `rimsky-sensor-webhook` | `rimsky/sensor-webhook` | the `sensor-webhook` Publisher service | `gcr.io/distroless/static:nonroot` | `lib/services/sensors/sensor-webhook/Dockerfile.sensor-webhook` |
| `rimsky-subscriber-openlineage` | — (not in compose) | the `openlineage` subscriber binary | `gcr.io/distroless/static:nonroot` | `lib/services/subscribers/openlineage/Dockerfile.openlineage` |
| `rimsky-executor-http-node` | `rimsky/executor-http-node` | the `http-node` Executor service | `gcr.io/distroless/static:nonroot` | `lib/services/executors/http-node/Dockerfile.http-node` |
| `rimsky-executor-verifier-http` | — (not in compose) | the `verifier-http` Executor service | `gcr.io/distroless/static:nonroot` | `lib/services/executors/verifier-http/Dockerfile.verifier-http` |
| `rimsky-executor-verifier-shape-checks` | — (not in compose) | the `verifier-shape-checks` Executor service | `gcr.io/distroless/static:nonroot` | `lib/services/executors/verifier-shape-checks/Dockerfile.verifier-shape-checks` |
| `rimsky-executor-claude-agent` | `rimsky/executor-claude-agent` | the TypeScript `claude-agent` Executor service (Node 24; runtime installs the `claude` CLI globally) | `cgr.dev/chainguard/wolfi-base` (digest-pinned) | `lib/services/executors/claude-agent/Dockerfile` |

---

## Compose-only images

`deploy/docker-compose.yml` builds and tags these locally. The four role
binaries below are built from `Dockerfile.go-base` (one binary each), not from
the combined `rimsky` image.

| Compose tag | Contains | Dockerfile + build arg |
| --- | --- | --- |
| `rimsky/migrate` | the `rimsky-migrate` binary (runs DB migrations) | `dockerfiles/Dockerfile.go-base` (`BINARY: rimsky-migrate`) |
| `rimsky/scheduler` | the `rimsky-scheduler` binary | `dockerfiles/Dockerfile.go-base` (`BINARY: rimsky-scheduler`) |
| `rimsky/supervisor` | the `rimsky-supervisor` binary | `dockerfiles/Dockerfile.go-base` (`BINARY: rimsky-supervisor`) |
| `rimsky/control-api` | the `rimsky-control-api` binary | `dockerfiles/Dockerfile.go-base` (`BINARY: rimsky-control-api`) |
| `rimsky/dashboard` | the reference dashboard | `Dockerfile` in the **`rimsky-dashboard`** sibling repo (build context `${RIMSKY_DASHBOARD_REPO}`) — the only image not built from rimsky-core. |

The compose stack also runs the upstream `postgres:15` image directly (no
rimsky build) for its database and for the one-shot `init-items` step.

---

## Test-only images

| Name | Contains | Base image | Dockerfile |
| --- | --- | --- | --- |
| `stubexecutor` | the test-only stub Executor (returns `Success` for every dispatch). Built on demand by the integration harness via testcontainers `FromDockerfile`; **never published**. | `gcr.io/distroless/static:nonroot` | `lib/services/test/stubexecutor/Dockerfile.stubexecutor` |
