# stub claim-producer

The `stub` claim-producer (`test/support/stores/stub/` in the rimsky tree) is an in-memory **test double** — deterministic, no real backing store — used for conformance and scenario tests. It is **not** a production starting point, and it is **not** a standalone deployable: it is a Go package consumed in-process by rimsky's own test fixtures and test harness, not a binary you wire into a deployment.

For a production producer, the in-tree reference producers are the concrete-paths `filesystem` store and the regional / items-queue `postgres` store under `lib/services/stores/{filesystem,postgres}`. To build your own, implement `lib/protocols/proto/v1/claim_producer.proto` — see [the claim-producer guide](../../protocols/claim-producer.md).

## When the stub is used

- ClaimProducer conformance (`rimsky conformance claim-producer`) and scenario tests, as a known-good in-memory target for claim semantics.
- DataProcessing self-tests — the stub also implements `SplitScope` and `ScopesConflict` (always advertised on the `ClaimProducer` surface via `SupportsSplitScope` / `SupportsScopesConflict`) and `DataProcessing` (registered and advertised only when `EnableDataProcessing` is true), so it doubles as the fan-out / partition test target.

## Configuration

The stub is configured by a Go `server.Config` struct, not a YAML file or an environment variable. The shape (see `test/support/stores/stub/config-example.yml` and `server/server.go`):
<!-- @source: test/support/stores/stub/server/server.go::Config -->

- `Substrate` — the in-memory store config, including the write-semantics envelope and any optional `pick_policies` for tests that need pick-policy semantics.
- `EnableLifecycle` — when true, registers no-op `LifecycleSubscriber` handlers alongside `ClaimProducer`.
- `EnableDataProcessing` — when true, registers the `DataProcessing` service and flips `Capabilities` to advertise `data_processing`.

The example config carries the in-memory knobs (`write_semantics`, `pick_policies`, `host`, `grpc_port`, `http_port`); the lifecycle and data-processing toggles are set by the fixture that starts the server.

## Wire shape

`Open` / `Commit` / `Abandon` / `Release` are no-ops on producer-side state; the only side effect is the in-memory claim ledger that enforces byte-equal scope conflicts. Pick-policy entries, when configured, exercise the shared `action` vocabulary (`on_commit` / `on_give_up`) — the same vocabulary the production stores use.

## Conformance

Point `rimsky conformance claim-producer` at a running stub's gRPC endpoint to exercise the `ClaimProducer` contract; the same checks are available as a Go library under `lib/protocols/conformance/claimproducer`.
