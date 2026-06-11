# Run an executor on your dev machine

## Problem

A node has to run code that lives only on your laptop — touch the local
filesystem, hit a service behind your VPN, exercise an unfinished
executor you're iterating on — so the deployed `rimsky` cluster cannot
dispatch to a long-lived in-cluster endpoint. You want the cluster to
treat your local binary as the executor for one specific instance,
without static deployment config and without redeploying anything when
the binary changes.

## Rimsky shape

The **[host-agent proxy](../concepts/host-agent-proxy.md)** is a rimsky
service that fronts every supervisor-facing protocol
([executor](../concepts/executor.md),
[claim-producer](../concepts/claim-producer.md), and at registration
others) and routes each dispatch to whichever connected
[host-agent](../concepts/host-agent.md) holds the binding for the
instance's owner. The host-agent is a daemon bundled into the `rimsky`
CLI; it dials the proxy outbound from your machine, authenticates with
your [api-key](../concepts/api-key.md), and `exec`s local binaries on
demand. The template marks the service name as **late-bound**
(`late_bind_services: [<name>]`) so registration skips the
endpoint-existence and Capabilities checks for it; the actual binary is
named per-instance via a [service
binding](../concepts/instance.md), with the spawn keyed by `(run-scope,
binding-name)` (see `concept:host-agent-proxy`).

The chain on every dispatch: supervisor → proxy (Executor.Execute carrying
the `x-rimsky-service-name` header) → connected agent → spawned local
binary's gRPC server → StreamClose travels back the same way. The
callback URL the spawned binary receives is rewritten to the agent's
local HTTP listener, so a binary running on your laptop can post async
callbacks to itself rather than dialing the supervisor across your VPN.

Primitives: **host-agent-proxy** (the supervisor-facing executor entry
in `rimsky.yml`), **host-agent** (the per-machine daemon and api-key
holder), **service binding** (the per-instance `{ name → binary path }`
map), `late_bind_services` (the template's "this name resolves at
instance creation" declaration).

## Template

Needs a rimsky deployment plus the `rimsky-host-agent-proxy` image
running alongside it (one of the four core images; see the
[official images catalog](../images/README.md)). Wire the proxy as a
named executor in `rimsky.yml` — its gRPC endpoint, with `protocols`
listing every supervisor-facing protocol you want late-bound through
it. The name on the entry (`codegen` below) is the service name templates
and bindings will use; one binary, registered once per protocol:

```yaml
# rimsky.yml — relevant fragment
executors:
  codegen:
    transport: grpc
    endpoint: "rimsky-host-agent-proxy:9090"
    tls: off
    # lifecycle_subscriber is load-bearing: the proxy fills its
    # per-instance binding cache from the OnInstanceCreated lifecycle
    # hook, and rimsky dials lifecycle hooks only at peers whose
    # protocols list declares lifecycle_subscriber. Without it the
    # proxy is silently skipped from the fan-out and every dispatch
    # fails with binding_not_found ("instance ... not found" — the
    # cache never learned the instance). (Alternative:
    # set RIMSKY_CONTROL_API_URL on the proxy so it fetches bindings
    # from the control API instead.)
    protocols: [executor, lifecycle_subscriber]
```

You also need an api-key the agent will authenticate with. Mint one
(`rimsky auth login` walks the flow); the agent reads it from the CLI's
active-context config.

Save the template as `local-codegen.yml`. The node names `codegen` as its
executor; the top-level `late_bind_services` declares `codegen` is
late-bound so registration does not try to resolve the executor's
Capabilities yet (the spawned binary's Capabilities are the runtime
authority for late-bound nodes):

```yaml
name: local-codegen
version: "1.0"
frame_resolution_mode: serial_queue
# Declaring codegen as late-bound bypasses the existence + schema checks
# at registration. The actual binary is named per-instance via a
# service binding on `rimsky run --service` (below).
late_bind_services: [codegen]
nodes:
  - type: worker
    # Same name as the rimsky.yml executors entry; the template never
    # names your laptop or the binary path. The node carries no
    # attributes block — late-bound dispatch skips the dispatch-time
    # executor_schema_unavailable gate because the spawned binary's
    # Capabilities are the authority.
    executor: codegen
```

Register, deploy, and instantiate the template with the binding to your
local binary. `rimsky run --service` auto-starts the host-agent if its
pid-file is not live, then submits the create with the binding embedded:

```sh
# `rimsky run` does register + deploy + create in one shot. Substitute
# the path to whatever binary serves the Executor protocol on the
# RIMSKY_AGENT_PORT the agent assigns at exec time — e.g. a local
# build of the http-node executor, or your own work-in-progress
# executor binary.
rimsky run local-codegen.yml --service codegen=$(which my-local-executor)
# → rimsky agent started (pid 12345)
# → instance_id=6b1f0c9a-4e2d-4f7b-9a3c-d5e8f1a2b3c4
```

The first `--service` flag triggers the auto-start check — a pid-file
under `~/.rimsky/agent.pid` plus a signal-0 liveness probe. With the
agent up the proxy sees one connected agent for your api-key, the create
request carries `service_bindings: { codegen: { path: "..." } }`, and
the supervisor dispatch resolves `codegen` through the proxy → agent →
binary chain. Watch the worker reach `fresh`:

```sh
curl -s http://localhost:8080/v1/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state}'
# → {"node_type":"worker","state":"fresh"}
```

Iterate by rebuilding the binary and re-running `rimsky run` — the
existing agent picks up the new path on the next instance's binding (the
spawn is keyed by `(run-scope, binding-name)`, so a fresh instance gets
a fresh spawn). When you are done, stop the agent (`rimsky agent stop`).

## Gotchas

- **The binary must read `RIMSKY_AGENT_PORT`.** When the agent spawns
  your binary it picks a free port, sets `RIMSKY_AGENT_PORT` in the
  child's environment, and poll-dials `127.0.0.1:<port>` until the
  child's gRPC server is up. A binary that ignores the env var or binds
  elsewhere fails readiness and the dispatch returns `spawn_failed`.
- **The proxy's `rimsky.yml` entry repeats per protocol.** The proxy is
  one binary on one gRPC port; if you want it to front *both* the
  executor and claim-producer protocols for late-bound services, declare
  it twice in `rimsky.yml` (once under `executors:`, once under
  `claim_producers:`) with the same endpoint. In v0.8.0 the proxy
  fronts all five supervisor-facing protocols — executor,
  claim-producer, publisher, validation, data-processing — by one
  uniform spawn/forward mechanism; none ships as a registered-but-
  unimplemented stub, so a late-bound binary speaking any of them works
  behind the proxy by construction (see
  [`host-agent-proxy`](../concepts/host-agent-proxy.md)). `blob-backend`
  is intentionally excluded — it is an in-process Go interface, not a
  gRPC wire protocol.
- **Bindings are scoped to the instance, not the template.** Two
  instances of the same template can bind `codegen` to different
  binaries; each gets its own spawn. The template never names a path —
  that is the point of `late_bind_services`.
- **The agent has no persistent state.** Authentication is the api-key in
  your CLI config (`rimsky auth login`); on a stream close the agent
  reaps every child it spawned after a grace period
  (`RIMSKY_AGENT_REAP_GRACE_SEC`, default 30s).
- **The instance is durable — it does not clean itself up.** Once the
  worker settles `fresh` the instance keeps living (instances are durable
  by default; there is no auto-terminate on drain). To tear it down,
  force-terminate then delete (`rimsky instance kill <id> --force`
  followed by `rimsky instance delete <id>`). For a one-shot iteration
  loop, `rimsky run --no-keep` is the whole story: it *implies*
  `terminate_after_run: true` on the create (so the instance
  self-terminates once its nodes settle), polls until terminal, and
  deletes the instance + template — no extra flag or body field needed.
  (`rimsky run --terminate-after-run` sets the flag without the
  poll-and-delete cleanup.) See the
  [README](README.md#instances-are-durable-by-default).

## Without rimsky

By hand you would stand up a reverse tunnel from your laptop to the
cluster (an SSH reverse forward, an `ngrok`-style egress), register a
permanent executor endpoint pointing at the tunnel in the deployment
config, wire a per-developer routing rule so other people's instances
don't pick up your binary, and tear all of it down each time you switch
branches. Rimsky moves the dev-machine endpoint behind a single proxy
service the deployment already knows about, routes by your api-key
rather than DNS, and binds the binary per-instance — so iterating is
"rebuild + `rimsky run`", and nothing about the deployed cluster
changes between iterations.
