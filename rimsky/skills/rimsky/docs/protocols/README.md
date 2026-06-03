# The protocols module & implementation guides

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

- [ClaimProducer](claim-producer.md) — implement the producer protocol: `Capabilities`, `Open`, `Commit`, `Abandon`, `Release`, plus the optional capability-gated `SplitScope` and `ScopesConflict`.
- [Executor](executor.md) — implement the dispatch protocol `Executor` (`Execute`) and the optional read-only observability protocol `ExecutorObservability` (`Capabilities`, `GetTrace`, `StreamTrace`).
- [LifecycleSubscriber](lifecycle-subscriber.md) — implement the lifecycle protocol: seven hooks — six template/instance state-transition hooks plus `OnRunScopeTerminal`.
- [Publisher](publisher.md) — implement the publisher protocol: `Capabilities`, `Subscribe`, `Unsubscribe`, `ListSubscriptions`. Sensors (cron / http / object-store / webhook) are one kind of Publisher.

Two further protocols are **mix-ins** a service advertises alongside its primary protocol rather than implements standalone: `DataProcessing` (for producers that materialize partitioned content) and `Validation` (template-registration-time validation). They have no separate prose guide; their wire contracts are in the generated reference ([`reference/data-processing.md`](reference/data-processing.md), [`reference/validation.md`](reference/validation.md)). A service advertises them via its `protocols` capability list (e.g. `data_processing`, `validation`).

The proto definitions live in the rimsky repo at `lib/protocols/proto/v1/`; the generated wire reference is published here under [`reference/`](reference/). For static codegen, generate language bindings with `protoc` (the rimsky build uses `make proto-gen`). **JS/TS and any `@grpc/proto-loader` consumer should prefer the `@rimsky-ai/protocols` npm package** ([above](#references)) — it ships the same `.proto` files with no repo checkout and no `protoc`.

## HTTP+JSON encoding

Non-Go services reach the protocols over the HTTP+JSON bridge, which uses **canonical protobuf-JSON**: `bytes` fields are base64-encoded strings, field names are `lowerCamelCase`, and a `oneof` renders as the set variant's name used as the JSON key (e.g. a `StreamClose` carrying the `success` variant → `{ "success": { ... } }`). A `google.protobuf.Struct` is a plain JSON object. The per-message field reference is under [`reference/`](reference/).
