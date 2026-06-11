# Blob backends

When an attribute, parked-state payload, or named-event payload exceeds
the inline-spill threshold (default 64 KiB), rimsky stores the bytes in
a configured `BlobBackend` rather than inline in `rimsky_node_attributes`
/ `rimsky_node_runs` / `rimsky_node_events`.

Rimsky ships four reference backends:

| Backend | Default? | Multi-process safe? | Handle prefix | Dev-only gate |
| --- | --- | --- | --- | --- |
| [`inline`](./inline.md) | **yes** | yes (no spill, no shared state) | — (never produces handles) | — |
| [`pg-largeobject`](./pg-largeobject.md) | no | yes (state in the same Postgres) | `pglo:` | — |
| [`filesystem`](./filesystem.md) | no | yes, given a shared `root` volume | `fs:` | — |
| [`memory`](./memory.md) | no | **NO** (in-process map) | `mem:` | rejected at startup unless `RIMSKY_PROCESS_ROLE=unified` |

`inline` is degenerate — it stores everything in the existing columns and
never spills. `pg-largeobject` keeps blobs inside the Postgres instance
every process already talks to (via the LO API); `filesystem` writes
files under a configured root that every process must mount; `memory`
is for the unified single-process dev setup only — the startup
validator (`ValidateBlobConfig`) enforces the gate.
<!-- @source: lib/foundation/persistence/blob_config.go::ValidateBlobConfig -->

Configuration: the `persistence.blob.backend` key plus per-backend sub-blocks
(`spill_threshold_bytes`, `filesystem.root`, `pg_largeobject`, `retention`).
The schema is parsed in `lib/control/config/` (`persistence.blob` block),
which validates the config and constructs the backend at startup, and
defaults to `inline` with a 64 KiB notional threshold when the key is
absent.
<!-- @source: lib/control/config/blob.go::OpenBlobBackend -->
<!-- @source: lib/foundation/persistence/blob_config.go::DefaultBlobConfig -->

## Cross-backend invariants

- **Blob content is inert in Rimsky** (`@blessed-invariant 21`): bytes
  are read only via `walkPath` substitution and the persistence-layer
  fetch on attribute read. Rimsky never logs, formats with `%v`,
  validates beyond schema gates, transforms, normalizes, hashes,
  indexes, pattern-matches, attaches to traces, or includes blob bytes
  in error messages.
- Handles are self-describing: each backend prefixes with its name
  (`pglo:`, `fs:`, `mem:`) so a future migration tool can route by
  prefix.
- The spill decision: spill only when a backend is configured, the
  threshold is > 0, the backend is not `inline`, and the payload
  exceeds the threshold.
  <!-- @source: lib/foundation/persistence/blob_spill.go::ShouldSpillBlob -->
- Orphan reaping: when an attribute's `value_handle` is overwritten or
  the row is deleted, the old handle is queued in
  `rimsky_blob_orphans`; the `SweepOrphanedBlobs` sweep deletes the
  bytes from the backend after `retention.retention_after_unreferenced`
  has passed (default 24h).
  <!-- @source: lib/foundation/persistence/blob_spill.go::QueueBlobOrphan -->

## Conformance

`rimsky conformance blob-backend --backend <name> [args]` runs the
in-process conformance suite (checks include round-trip 1KB + 10MB,
range read, delete-then-read, idempotent delete, and concurrent writes).
The same checks are exposed as a Go library under
`lib/protocols/conformance/blobbackend`.
