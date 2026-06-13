# The protocols module & implementation guides

> **Version.** These guides target the rimsky release this corpus is reconciled
> against (`reconciledAgainst` in `.claude-plugin/plugin.json`). Runnable,
> version-pinned server skeletons for each protocol live under
> [`../examples/`](../examples/README.md), each with a `go.mod` stating the exact
> `lib/protocols` tag.

rimsky's protocols are language-neutral gRPC contracts. You implement a service against the wire protocol in whatever language you like — there is no required SDK.

For Go projects, rimsky publishes its **`protocols` module** as a convenience. It is the single public Go module: it carries the wire contract (the protobuf-generated types) and a handful of **optional helper packages** you can lean on if you want them, or ignore entirely and code straight to the wire types:

- **Contract ergonomics** — `claimproducer`, `lifecycle`: hand-written Go types over the wire contract.
- **Optional helpers** — `serverkit` (gRPC + HTTP/JSON bridge scaffolding), `publisherkit` (publisher-side retry/backoff), `action` (claim-producer pick-policy vocabulary).
- **Conformance library** — `conformance/`: the executable contract spec, invocable from your own Go tests as well as the `rimsky conformance <protocol>` CLI subcommands.

None of these are required. They exist so a Go service doesn't have to re-derive the boilerplate.

For JS/TS / non-Go consumers, rimsky publishes **`@rimsky-ai/protocols`** (npm, Apache-2.0) — the parallel convenience. <!-- @source: lib/protocols/package.json --> It ships the raw `.proto` files (under `proto/v1/`) plus two path helpers — `protoDir` (the include directory) and `protoPath(file)` (one named proto) — exported from `index.js` / `index.d.ts`. <!-- @source: lib/protocols/index.js --> Feed those to `@grpc/proto-loader`; you do **not** need a repo checkout or `protoc`. Install the package alongside the gRPC toolchain it needs:

```sh
npm install @rimsky-ai/protocols @grpc/proto-loader @grpc/grpc-js
```

The load pattern. The proto package namespace is `rimsky.v1`, so every service constructor hangs off `pkg.rimsky.v1`:

```ts
import * as protoLoader from "@grpc/proto-loader";
import * as grpc from "@grpc/grpc-js";
import { protoDir, protoPath } from "@rimsky-ai/protocols";

const definition = protoLoader.loadSync(
  [protoPath("executor.proto"), protoPath("executor_observability.proto")],
  { keepCase: true, longs: String, enums: String, defaults: true, oneofs: true,
    includeDirs: [protoDir] }, // resolves cross-proto imports
);
const pkg = grpc.loadPackageDefinition(definition) as any;
const { Executor } = pkg.rimsky.v1; // the Executor service constructor
```

`protoDir` / `protoPath` are ESM (the package is `"type": "module"`). CommonJS consumers that skip the ESM helper resolve a proto directly via the `"./proto/*"` subpath export, e.g. `require.resolve("@rimsky-ai/protocols/proto/v1/executor.proto")`.

## References

- [`go-packages.md`](go-packages.md) — generated reference for the module's hand-written Go packages (the helpers above).
- [`reference/`](reference/) — generated reference for the wire contract itself (services, messages, fields, enums).

## Implementation guides

These cover the gap between *understanding the concepts* and *implementing a custom service against the wire protocol in your language of choice*.

- [ClaimProducer](claim-producer.md) — implement the producer protocol: `Capabilities`, `Open`, `Commit`, `Abandon`, `Release`, plus the optional capability-gated `SplitScope` and `ScopesConflict`. An optional read-only observability protocol `ClaimProducerObservability` (`Capabilities`, `GetClaim`, `StreamClaim`, `ListClaims`, `GetAdminView`) sits alongside it; its wire contract lives in the generated reference ([`reference/claim-producer-observability.md`](reference/claim-producer-observability.md)).
- [Executor](executor.md) — implement the dispatch protocol `Executor` (`Execute`) and the optional read-only observability protocol `ExecutorObservability` (`Capabilities`, `GetTrace`, `StreamTrace`).
- [LifecycleSubscriber](lifecycle-subscriber.md) — implement the lifecycle protocol: seven hooks — six template/instance state-transition hooks plus `OnRunScopeTerminal`.
- [Publisher](publisher.md) — implement the publisher protocol: `Capabilities`, `Subscribe`, `Unsubscribe`, `ListSubscriptions`. Sensors (cron / http / object-store / webhook) are one kind of Publisher.

Two further protocols have **no separate prose guide**: `DataProcessing` (for producers that materialize partitioned content) and `Validation` (template-registration-time validation). Like `LifecycleSubscriber`, both are **mix-ins** — a service advertises them alongside its primary protocol via its `protocols` capability list (e.g. `data_processing`, `validation`) rather than implementing them standalone — but their wire contracts live only in the generated reference ([`reference/data-processing.md`](reference/data-processing.md), [`reference/validation.md`](reference/validation.md)).

For the `validation` mix-in, the role list rimsky honors is the **live** `validation_supported_roles` from the peer's primary-protocol `Capabilities` handshake, not anything in `rimsky.yml`. When a peer advertises `validation`, rimsky runs a fresh capability handshake at startup to learn the roles — one extra RPC per validation-mix-in peer; a failure fails startup. The handshake is uniform across all three peer kinds: `ClaimProducer.Capabilities` for claim-producers, `ExecutorObservability.Capabilities` for executors (off `observability_endpoint:` when configured, otherwise off the dispatch endpoint), and `Publisher.Capabilities` for publishers. <!-- @source: lib/control/config/publishers.go -->

One proto file is not a service contract at all: `events.proto` defines the typed **operational event-log** vocabulary — the `OperationalKind` enum (the canonical operational kind discriminators: `auth.*` audit kinds, node-run lifecycle, lock and claim lifecycle, attribute-substitution/validation, breakpoints, message-bus activity (`OPERATIONAL_KIND_MESSAGE_EMITTED` / `OPERATIONAL_KIND_MESSAGE_RECEIVED` — the operational-side audit of message activity, distinct from the signal-class `message/...` topology), fan-out/sub-graph dispatch, parked-node lifecycle) plus a typed `Event` message with per-kind payloads for consumers who want type-checked event streams. Signal-class kinds (`terminal/...`, `transient/...`, `attribute/...`, `event/...`, `message/...`) are **not** in this enum — they carry the parsed signal type-path as the kind value and live in the signal taxonomy ([signal](../concepts/signal.md)). You never implement `events.proto`; consume it when reading the event log ([event-log](../concepts/event-log.md)). Full enum and payload reference: [`reference/events.md`](reference/events.md). <!-- @source: lib/protocols/proto/v1/events.proto -->

The proto definitions live in the rimsky repo at `lib/protocols/proto/v1/`; the generated wire reference is published here under [`reference/`](reference/). For static codegen, generate language bindings with `protoc` (the rimsky build uses `make proto-gen`). **JS/TS and any `@grpc/proto-loader` consumer should prefer the `@rimsky-ai/protocols` npm package** ([above](#references)) — it ships the same `.proto` files with no repo checkout and no `protoc`.

## HTTP+JSON encoding

Non-Go services reach the protocols over the HTTP+JSON bridge, which uses **canonical protobuf-JSON**: `bytes` fields are base64-encoded strings, field names are `lowerCamelCase`, and a `oneof` renders as the set variant's name used as the JSON key (e.g. a `StreamClose` carrying the `success` variant → `{ "success": { ... } }`). A `google.protobuf.Struct` is a plain JSON object. The per-message field reference is under [`reference/`](reference/).

## Host-agent-proxy: a transparent forwarder

The `rimsky-host-agent-proxy` binary fronts every fronted rimsky service protocol — `Executor`, `ClaimProducer`, `Publisher`, `Validation`, `DataProcessing` — by one uniform spawn/forward mechanism. Each presents exactly the fronted service's protocol on the supervisor-facing side; none ships as a registered-but-unimplemented stub. A late-bound binary that conforms to its own protocol therefore works behind the proxy by construction — there is no separate proxy conformance suite, and **no protocol surface is excluded** (the `BlobBackend` is the only intentional exclusion, because it is an in-process Go interface, not a gRPC wire protocol).

For an implementor, three consequences matter:

- **Don't read `run_scope_id`.** It exists on `ExecuteRequest` and `OpenRequest` solely so the proxy can key per-run-scope spawn isolation (one spawned child per `(run_scope_id, binding)`, reaped at run-scope termination). Concurrent run-scopes of one fanned-out instance therefore get distinct, isolated children. An in-process executor or claim-producer ignores the field.
- **Don't read `Binding` overrides.** A binding may carry per-binding exec() overrides (`args`, `env`, `cwd`, `ready_timeout_seconds`). All four are additive — absent means today's default (no extra args, inherited env, the instance-level cwd, and the global Spawn ready-timeout). They are consumed by the agent at spawn time, not by your service.
- **`DispatchFrame.rpc_method` is the proxy's internal routing key**, not part of your wire surface. For `Publisher`, `Validation`, and `DataProcessing` — which expose multiple unary RPCs whose request messages are distinct types — the proxy carries the RPC name on the wire so the agent can match it against the child service descriptor (the analogue of `claim_producer_verb` for the claim-producer path). You do not handle frames; your child binary just serves the protocol as normal.
