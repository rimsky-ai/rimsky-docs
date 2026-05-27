# The protocols module & implementation guides

rimsky's protocols are language-neutral gRPC contracts. You implement a service against the wire protocol in whatever language you like — there is no required SDK.

For Go projects, rimsky publishes its **`protocols` module** as a convenience. It is the single public Go module: it carries the wire contract (the protobuf-generated types) and a handful of **optional helper packages** you can lean on if you want them, or ignore entirely and code straight to the wire types:

- **Contract ergonomics** — `claimproducer`, `lifecycle`: hand-written Go types over the wire contract.
- **Optional helpers** — `serverkit` (gRPC + HTTP/JSON bridge scaffolding), `publisherkit` (publisher-side retry/backoff), `action` (claim-producer pick-policy vocabulary).
- **Conformance library** — `conformance/`: the executable contract spec, invocable from your own Go tests as well as the `rimsky-*-conformance` CLIs.

None of these are required. They exist so a Go service doesn't have to re-derive the boilerplate.

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

The proto definitions live in the rimsky repo at `protocols/proto/v1/`; the generated wire reference is published here under [`reference/`](reference/). Generate language bindings with `protoc` (the rimsky build uses `make proto-gen`).
