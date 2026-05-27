# stub claim-producer

The `stub` claim-producer (`stores/stub/`) is an in-memory test fixture and the only reference producer that still ships in rimsky's tree. It is a test double — deterministic, in-memory, no real backing store — used for conformance, scenario tests, and no-op smoke deployments. It is **not** a production starting point; the production reference producers (concrete-paths `filesystem`, regional / items-queue `postgres`) live in the separate `rimsky-services` repository.

## When to use it

- Conformance suites and scenario tests.
- Local development where you want claim semantics without the cost of a real Postgres or filesystem.
- Smoke-deployment compose stacks that need a working `ClaimProducer` endpoint with no domain backing.

For a custom production producer, implement `protocols/proto/v1/claim_producer.proto` — see [the claim-producer guide](../../protocols/claim-producer.md).

## Two config surfaces

There are two distinct config files, and the stub-specific knobs live in the binary's own config, not in `rimsky.yml`.

**rimsky.yml** — how rimsky dials the producer. A `claim_producers:` entry carries only `endpoint`, `protocols`, `write_semantics_allowed`, and (optionally) `observability_endpoint`. There is no `kind:` or nested `config:` block here.

```yaml
claim_producers:
  test_fixture:
    endpoint: "grpc://stub-producer:9102"
    protocols: [claim_producer]
    write_semantics_allowed: [sync, staged_async]
```

The operator-declared `write_semantics_allowed` must be a subset of the set the producer advertises in its `Capabilities` handshake; valid values are `sync`, `staged_async`, `blocking_async`, and `read_only`. A mismatch fails rimsky startup.

**The stub binary's own config** — read from `STORE_STUB_CONFIG`. This is where the stub's advertised envelope, optional pick policies, bind address/ports, and the lifecycle toggle live:

```yaml
write_semantics_allowed: [sync, staged_async]
pick_policies: {}            # optional; for tests needing pick-policy semantics
host: 0.0.0.0
grpc_port: 9102
http_port: 9112
enable_lifecycle: false      # set true to register no-op LifecycleSubscriber handlers
```

## Wire shape

The stub advertises whatever `write_semantics_allowed` its own config declares (defaulting to `[sync]` when empty). `Open`/`Commit`/`Abandon`/`Release` are no-ops on producer-side state; the only side effect is the in-memory claim ledger that enforces byte-equal scope conflicts. Pick-policy entries, when configured, exercise the shared `action` vocabulary (`on_commit` / `on_give_up`) — the same vocabulary the production stores use.

## Conformance

Point `cmd/rimsky-claim-producer-conformance` at the stub's gRPC endpoint to exercise the `ClaimProducer` contract; the same checks are available as a Go library under `protocols/conformance/claimproducer`.

## Operating

Single-process, in-memory. State does not persist across restarts. Suitable for a single-replica test deployment; multi-replica is unsupported because the in-memory ledger does not coordinate across processes.
