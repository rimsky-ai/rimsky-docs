# Implementing a claim producer

> **Version.** The API on this page targets the rimsky release this corpus is
> reconciled against (`reconciledAgainst` in `.claude-plugin/plugin.json`). For
> runnable, version-pinned code, copy the producer at
> [`../examples/claimproducer/`](../examples/README.md).

A **claim producer** is an out-of-process service that produces claim handles for
rimsky's lock-and-claim primitives. It implements the `ClaimProducer` protocol over
gRPC — five required methods (`Capabilities`, `Open`, `Commit`, `Abandon`,
`Release`) plus two optional capability-gated methods (`SplitScope`,
`ScopesConflict`). It owns the underlying state (filesystem paths, postgres rows,
in-memory map) and the per-verb semantics; rimsky owns the lock-state bookkeeping.

There is **no required SDK** — implement against the wire types in any language. A
Go service may use the `protocols` module's `claimproducer` package for
hand-written types over the wire contract
([`go-packages.md`](go-packages.md)). Wire contract:
`lib/protocols/proto/v1/claim_producer.proto`; generated field/message/RPC
reference at [`reference/claim-producer.md`](reference/claim-producer.md) (do not
hand-track field numbers here).

A note on terminology: the protocol-level term is **claim producer** (the service
implementing `ClaimProducer` over gRPC). The colloquial term **store** survives at
the bundled-services layer for data-backed reference impls (filesystem store,
postgres store, stub store).

<!-- @source: .ok-planner/design/concepts/claim-producer.md -->

## What you implement

| RPC | Required? | Purpose |
| --- | --- | --- |
| `Capabilities(CapabilitiesRequest) → CapabilitiesResponse` | **Yes** | Startup handshake: declare allowed write semantics, optional-RPC flags, mix-in protocols, validation roles. |
| `Open(OpenRequest) → OpenResponse` | **Yes** | Acquire a claim inside rimsky's atomic acquisition transaction; return `Acquired` or `Unavailable`. |
| `Commit(CommitRequest) → CommitResponse` | **Yes** | Terminal: the consumer of the claim succeeded. Idempotent in `claim_id`. |
| `Abandon(AbandonRequest) → AbandonResponse` | **Yes** | Terminal: the consumer of the claim failed. Idempotent in `claim_id`. |
| `Release(ReleaseRequest) → ReleaseResponse` | **Yes** | Tear down producer-side state for a committed-durable claim at instance termination or explicit asset release. Idempotent in `claim_id`. |
| `SplitScope(SplitScopeRequest) → SplitScopeResponse` | No — gated on `supports_split_scope` | Partition an already-`Open`'d parent scope into `SubScopeDescriptor` rows for fan-out. |
| `ScopesConflict(ClaimScopesConflictRequest) → ScopesConflictResponse` | No — gated on `supports_scopes_conflict` | Decide whether two non-byte-equal `claim_scope` values serialize against each other. |

`OpenResponse` is a `oneof` carrying either an `Acquired` message (with `address`,
`payload`, `claim_scope`, and `realized_write_semantics`) or an `Unavailable`
message (no claim available right now; rimsky retries on the next scheduler tick).
Each terminal verb has its own request/response pair so the producer can tag
idempotency keys and structured per-verb fields without overloading a shared
envelope.

> **Auth-blind advisory.** Rimsky has no machinery for credentials, encryption, or
> access control. Encrypt sensitive bytes before handing them to rimsky if you need
> protection. Service-to-service auth is operator-configured at the deployment
> layer (mTLS, IAM).

## Boundaries

The producer **owns**:

- The underlying state — filesystem paths, postgres rows, in-memory map.
- The per-verb semantics (what `Commit` / `Abandon` / `Release` actually do).
- Its own TTL / sweep for orphan reclamation (obligation 1 below).
- Canonicalizing `claim_scope` bytes so that acquisitions which should conflict are
  byte-equal (obligation 5 below).
- Resume detection — recognizing a repeated `claim_id` against its own state.

The producer does **NOT** own (rimsky's job):

- **Lock-state bookkeeping, the orphan reaper, the state machine, and verb
  dispatch.** Rimsky drives these.
- **The conflict predicate over claim content.** Rimsky compares `claim_scope`
  byte-for-byte by default — no glob, no range-match, no canonicalization on the
  rimsky side. The producer canonicalizes; rimsky only compares (unless the
  producer implements the optional `ScopesConflict` RPC).
- **Interpreting claim content.** `address`, `payload`, and `claim_scope` from
  `OpenResponse.acquired` are opaque to rimsky (see [Inertness](#inertness)).
- **Producer-side orphan cleanup.** When rimsky's orphan reaper deletes a stale
  claim-handle row, it does **not** fire any producer verb — the producer's own TTL
  handles that case (obligation 4 below).
- **Credentials, encryption, access control.** Rimsky is auth-blind (see advisory
  above).

## Five obligations every implementation must meet

1. **Run your own TTL / sweep for orphan reclamation.** Rimsky-side reaping handles
   its own claim-handle records. The producer-side state lives outside rimsky's
   view; the producer must run its own TTL sweep so partial commits don't leak.
   Recommended TTL: strictly greater than `5 × heartbeat_interval` (rimsky's
   orphan-reap window) so a healthy producer doesn't strip a claim out from under a
   live supervisor.
2. **Record `claim_id` before any state mutation in `Open`.** `Open` is invoked
   inside rimsky's atomic acquisition transaction. If the producer mutates state
   but rimsky's transaction rolls back, the producer is left with orphan state.
   Recording `claim_id` first lets the producer's TTL sweep identify and clean up
   orphans.
3. **All terminal verbs must be idempotent in `claim_id`.** `Commit(claim_id)`,
   `Abandon(claim_id)`, `Release(claim_id)` must be safe to retry. Rimsky may retry
   on transient gRPC failures.
4. **Do not depend on rimsky calling `Abandon` for orphan cleanup.** When rimsky's
   orphan reaper deletes a stale claim-handle row, it does not fire any producer
   verb. The producer's own TTL handles that case.
5. **Canonicalize `claim_scope` bytes.** Two `Open` calls that should conflict must
   produce byte-equal `claim_scope` values. Rimsky's foundation compares
   byte-for-byte by default; no glob, no range-match, no canonicalization on the
   rimsky side. (If you need non-byte-equal values to conflict, implement the
   optional `ScopesConflict` RPC.)

## `Open` — acquire a claim

<!-- @source: .ok-planner/design/concepts/claim.md -->

`Open` is invoked inside rimsky's atomic acquisition transaction. A claim is a
node-declared assertion that the node will read or read-write a producer-defined
slice of state for the duration of its run; it binds an alias, an intent (`r` or
`rw`), a producer name, and a selector. Return one `oneof` variant in
`OpenResponse`.

### `OpenRequest`

| Field | Meaning |
| --- | --- |
| `claim_id` | Rimsky-generated UUID; record it first (obligation 2). |
| `producer_name`, `selector`, `intent`, `alias` | The resolved claim spec. `selector` is post-substitution; the producer parses it. |
| `template_id`, `instance_id` | Opaque to rimsky, provided for namespace routing or trace correlation. `template_id` (the wire-protocol field name) carries the content hash; `instance_id` is the instance UUID. |
| `run_scope_id` | Opaque per-run-scope identifier; ignore unless you are the host-agent-proxy (which keys per-run-scope spawn isolation on it so two concurrent run-scopes of the same instance get distinct late-bound child processes). A normal in-process producer does not need to read it. |

### `OpenResponse` (`oneof`)

| Variant | Fields | Meaning |
| --- | --- | --- |
| `Acquired` | `address`, `payload`, `claim_scope`, `realized_write_semantics` | The claim was acquired. `address` = producer-supplied bytes the executor uses to access claimed state. `payload` = producer-supplied data captured at acquisition (e.g. a picked queue item's user data). `claim_scope` = canonicalized scope bytes; two acquisitions that should conflict must produce byte-equal `claim_scope`. `realized_write_semantics` = the per-claim value; must be a member of the envelope returned by `Capabilities`. |
| `Unavailable` | optional `error_class` | No claim available right now (e.g. an empty items-table queue). Rimsky retries on the next scheduler tick. Producer-side faults flow as gRPC error status codes, not as an `Unavailable` response. Set `error_class` to a member of your producer's declared error vocabulary (e.g. `"pg/claim_unavailable"`) so rimsky's acquisition-failure routing keys the operator's `error_types:` chain on it; leave it empty to preserve the historical synthetic class `acquire/unavailable`. |

Resume detection is the producer's responsibility. If `Open` arrives with a
`claim_id` the producer has seen before, the producer recognizes the resume
internally (lookup by `claim_id` against its own state). There is no `resumed` flag
on `Open`.

## `Commit` — consumer succeeded

`CommitRequest` carries `claim_id`, `claim_scope`, `address`. Signals that the
consumer of the claim succeeded. Producer disposition is per-producer config.
Examples: for `rw` claims on `staged_*` mode, atomically publish the staging area's
contents into live state; for `sync`-mode `rw` claims, producer-side no-op (writes
already live); for pick-policy claims, apply the configured commit-default action.

The bundled postgres store demonstrates the SQL-substrate shape: for a
`staged_async` claim whose selector names a schema, `Open` reserves a per-claim
staging schema and returns its name as the claim's `address`; the executor
writes into the staging schema; `Commit` performs the swap (drop the canonical
schema, rename staging into its place) inside one store-side transaction;
`Abandon` discards the staging schema; `Release` reaps any residual staging from
an interrupted claim. The `claim_scope` stays the canonical selector so
byte-equality conflict detection is unaffected. A swap that would lose external
dependencies surfaces the declared error class `pg/swap_failed` and leaves the
staging intact (the claim's recorded state stays `OPEN`). The lifecycle engages
only for schema-shaped selectors; opaque (path-shaped) scope-bytes claims keep
the verbatim selector-echo at `Open` and no-op terminals. This is not a
wire-protocol change — every producer is free to implement the same atomic-staging
shape against whatever substrate (Iceberg manifest pointer, S3 prefix swap, etc.)
makes sense.

The atomic-staging pattern composes with the held-claim sign-off gate
([atomic-staging](../concepts/atomic-staging.md),
[write-semantics](../concepts/write-semantics.md)): when verifier nodes co-hold
the staging claim, the supervisor's aggregation fires `Commit` (atomic swap) on
all-success and `Abandon` (drop staging) on any-failure. The gate binds the
run's **effective bound attributes** (terminal-final delta merged with any
incremental writebacks, last-write-wins) so the bound output equals the
persisted output. From the producer's perspective the verbs are
indistinguishable from a non-held terminal; the held resolution is rimsky-side
machinery.

Idempotent in `claim_id` (obligation 3). `CommitResponse` carries two optional
fields, both inert to rimsky:

| Field | Meaning |
| --- | --- |
| `version_id` | Declared for `DataProcessing`-capable producers, but in v0.8.0 rimsky does **not** read it off the wire: the runtime's `Commit` client discards the response body, and the persisted `rimsky_claim_handles.version_id` is sourced from the `CommitCandidate` step instead. Setting it has no effect today; leave it empty. |
| `producer_metadata` | Declared for a future fan-out writeback surface, but in v0.8.0 rimsky does **not** read it off the wire — the same discarded `Commit` response. Setting it has no effect today; leave it empty. |

## `Abandon` — consumer failed

`AbandonRequest` carries `claim_id`, `claim_scope`, `address` (where `address` may
be empty — the producer identifies its own state by `claim_id`). Signals that the
consumer of the claim failed. Producer disposition is per-producer config.
Examples: for `staged_*` `rw` claims, discard the staging area without publishing;
for pick-policy claims, apply the configured abandon-default action; for `sync`
`rw` claims it is degenerate — direct writes cannot be undone, so document this as
an honest limitation in your producer's README.

Fires for **every** non-held claim whose consumer failed, including `r`-intent
(read-only) claims — the run-terminal path has no intent or write-semantics
gating, only the success/failure binary
(see [Verb firing at terminal](#verb-firing-at-terminal)). For a read claim the
producer-side disposition is typically a no-op or read-state teardown. Idempotent
in `claim_id`. `AbandonResponse` has no fields.
<!-- @source: lib/runtime/terminal_decision.go::fireProducerVerb -->

## `Release` — release a committed-durable claim

`ReleaseRequest` carries `claim_id`, `claim_scope`, `address`. `Release` is **not**
a run-terminal verb — it never fires when a node run completes (that path fires
`Commit` / `Abandon` only). It fires against committed-durable claim-handle rows
(`state = 'committed'` AND `lifetime = 'durable'` — the asset surface) in exactly
two places:

- **Instance termination** — the runtime walks the instance's committed-durable
  rows and calls `Release` on each, sequentially. A failed `Release` does not
  block termination; the rimsky-side row is preserved for retry and the failure
  is reported per-claim to the operator.
  <!-- @source: lib/runtime/instance_termination.go::ReleaseHeldDurableClaims -->
- **Explicit asset release** — the operator
  `DELETE /v1/instances/{id}/assets/{alias}` handler.

The rimsky-side row is deleted only on `Release` success. Producer disposition:
tear down whatever state backs the durable claim (residual staging, snapshot,
MVCC transaction). Idempotent in `claim_id`. `ReleaseResponse` has no fields.
See [claim-lifetime](../concepts/claim-lifetime.md) for the durable-vs-subgraph
lifecycle.

## `Capabilities` — startup handshake

`CapabilitiesRequest` has no fields. Probed once per service at process startup;
cached for the process's lifetime. `CapabilitiesResponse` returns:

| Field | Meaning |
| --- | --- |
| `write_semantics_allowed` | The set of `WriteSemantics` values this producer may return from `Open`. |
| `supports_split_scope` / `supports_scopes_conflict` | The optional-RPC flags. Advertise `true` only when you implement the matching RPC. |
| `protocols` | The mix-in service protocols this binary also speaks alongside `claim_producer` (e.g. `data_processing`, `validation`, `lifecycle_subscriber`). A binary that implements `LifecycleSubscriber` lists `lifecycle_subscriber` here. |
| `validation_supported_roles` | When `validation` is in `protocols`, the role discriminators this service will validate (`executor` / `claim_producer` / `lifecycle_subscriber` / `sensor`). For a claim-producer peer advertising the `validation` mix-in, rimsky learns the **live** list by running a fresh `ClaimProducer.Capabilities` handshake at startup (the operator-declared YAML carries only the write-semantics envelope, never roles); a handshake failure fails rimsky startup. <!-- @source: lib/control/config/publishers.go --> |

The valid `WriteSemantics` values are `sync`, `staged_async`, `blocking_async`, and
`read_only` (the proto enum `WRITE_SEMANTICS_*`; `WRITE_SEMANTICS_UNKNOWN` is the
zero value and MUST NOT be returned). What each value means for concurrent claims
on byte-equal claim scope — and the `intent` it fits — is in
[write-semantics](../concepts/write-semantics.md).

The operator declares a subset envelope per service in `rimsky.yml` under
`write_semantics_allowed:`. The capability handshake validates operator-declared ⊆
producer-declared; a mismatch fails rimsky startup.

## Optional RPCs

| RPC | Gate flag | When to implement |
| --- | --- | --- |
| `SplitScope` | `supports_split_scope = true` | Required for templates that fan out against your producer. Rimsky calls it inside the acquisition transaction to partition the already-`Open`'d parent scope into `SubScopeDescriptor` rows that seed the sub-claims. The shape of `partition_request` bytes is yours to define. |
| `ScopesConflict` | `supports_scopes_conflict = true` | When two non-byte-equal `claim_scope` values should still serialize against each other (range overlap, hash-bucket collision). When not advertised, rimsky falls back to the byte-equal default (`@blessed-invariant 4b`). |

## Byte-equal-scope uniformity

Across the lifetime of the producer process, two `Open` calls returning byte-equal
`claim_scope` MUST return the same `realized_write_semantics`. Rimsky relies on this
for the conflict predicate; producers enforce.

In practice: if your producer supports both `staged_async` and `sync` modes, it
must consistently return the same one for any given canonical scope. The simplest
path is "one mode per producer process" (single-element envelope); supporting
per-claim variation requires per-scope state.

## Verb firing at terminal

<!-- @source: lib/runtime/runner_terminal_release.go::releaseClaim -->
<!-- @source: lib/runtime/terminal_decision.go::fireProducerVerb -->

When a node run reaches terminal, rimsky fires exactly one producer verb per
non-held claim: `Commit(claim_id)` on success, `Abandon(claim_id)` on failure.
This holds for **every** claim shape — there is no intent or write-semantics
gating, and no "no verb" branch. An `r`-intent `sync` claim gets the same
`Commit` / `Abandon` pair as a `staged_async` `rw` claim; what differs is the
producer-side disposition (often a no-op for read claims), which is per-producer
config — rimsky carries only the success/failure binary. `Release` never fires
on the run-terminal path; it is reserved for committed-durable rows (see
[`Release`](#release--release-a-committed-durable-claim)).

After the verb, rimsky promotes the claim-handle row's `state` column to
`committed` / `abandoned` rather than deleting it ("Promote-not-delete"). The
row is preserved past terminal for forensics and asset-presentation queries;
the retention sweep reaps non-durable terminal rows at the configured trailing
window, while committed-durable rows persist as the asset surface until
`Release` ([claim-lifetime](../concepts/claim-lifetime.md)).
<!-- @source: lib/runtime/terminal_decision.go::promoteHandleState -->

For held claims, rimsky fires exactly one automatic resolution at holding-subgraph
completion. Aggregate outcome — all-completed → `Commit`; any-failed → `Abandon` —
drives the verb. From the producer's perspective, the call is indistinguishable
from a non-held terminal.

## Inertness

<!-- @source: .ok-planner/design/concepts/claim-handle.md -->

What rimsky won't do with claim content. The persistent claim-handle row asserts
"holder H has acquired scope S for purpose P" — it carries the rimsky-generated
`claim_id`, holder identity, scope bytes, producer-returned address and payload, the
realized write semantics, and a held flag.

`address`, `payload`, and `claim_scope` from `OpenResponse.acquired` are opaque to
rimsky. Rimsky reads claim content by named-field path only at substitution-leaf
extraction; it never logs, validates, transforms, normalizes, decrypts, hashes,
indexes, pattern-matches, attaches to traces, or includes the bytes in errors.

This means: encrypted fields stay encrypted in transit. Operators who want fields
out of rimsky's address space encrypt before passing.

## Atomicity is decoupled

Rimsky opens a transaction, claims the worker-request, INSERTs claim-handle rows,
RPCs `ClaimProducer.Open` (the producer runs in its own transaction), UPDATEs
claim-handle addresses, INSERTs claim-holders, COMMITs.

Failures on either side are recovered separately:

- Rimsky-side failures are recovered via rimsky's orphan reaper.
- Producer-side failures are recovered via the producer's own TTL/sweep
  (obligation 1).

The two transactions are decoupled. A failure on one side does not roll back the
other.

## Conformance

`rimsky conformance claim-producer` exercises a producer against the wire-protocol
contract. Run it pointing at your producer endpoint to verify the verbs behave
correctly:

```
rimsky conformance claim-producer --endpoint grpc://your-producer:9101
```

The same checks are exposed as a Go library under
`lib/protocols/conformance/claimproducer` so you can invoke them from your own
tests.

## Reference impls

- **Copyable skeleton (Apache)** — a minimal read-only producer you can copy and
  adapt: [`../examples/claimproducer/`](../examples/README.md). It shows the
  `OpenResponse` oneof (Acquired vs. Unavailable) and the Commit / Abandon /
  Release lifecycle. Vendored from rimsky-core's Apache `examples/` module at the
  reconciled tag.
- **Test double** — the stub at `test/support/stores/stub/`, an in-memory fixture
  (see [`../stores/stub/README.md`](../stores/stub/README.md)). It is a test double,
  not a production starting point.
- **Official services (AGPL)** — the concrete-paths `filesystem` store and the
  regional-access / items-queue / atomic-staging-schema `postgres` store, under
  `lib/services/stores/{filesystem,postgres}`. The postgres store demonstrates
  the SQL-substrate atomic-staging lifecycle (per-claim staging schema reserved
  at `Open`, atomic swap on `Commit`, drop on `Abandon`) plus the `pg/swap_failed`
  declared-error-class emit site. These are AGPL runnable products; study their
  `config-example.yml` and server packages for patterns, but build your own
  producer from the Apache wire contract and the copyable skeleton above rather
  than copying AGPL service code.

## See also

[claim-producer](../concepts/claim-producer.md) · [claim](../concepts/claim.md) · [claim-handle](../concepts/claim-handle.md) · [claim-scope](../concepts/claim-scope.md) · [write-semantics](../concepts/write-semantics.md) · [atomic-staging](../concepts/atomic-staging.md)
