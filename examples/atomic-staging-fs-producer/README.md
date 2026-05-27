# atomic-staging-fs-producer

A reference ClaimProducer implementing the atomic-staging pattern
(see [`docs/agents/examples/atomic-staging.md`](../../docs/agents/examples/atomic-staging.md))
over a POSIX filesystem
substrate. Open creates a per-(scope, claim_id) staging directory;
Commit fires a two-rename atomic swap into the canonical location;
Abandon drops the staging directory; Release is a no-op for `r`
intent.

## Build

From the `examples/` module root (the directory holding `go.mod`):

```sh
go build -o atomic-staging ./atomic-staging-fs-producer/cmd
```

## Run

```sh
RIMSKY_ATOMIC_STAGING_ROOT=/var/lib/atomic-staging \
RIMSKY_LISTEN_ADDR=:8090 \
RIMSKY_SWEEP_INTERVAL=5m \
RIMSKY_SWEEP_TTL=24h \
./atomic-staging
```

`RIMSKY_ATOMIC_STAGING_ROOT` is required (the binary exits non-zero if
unset). `RIMSKY_LISTEN_ADDR` defaults to `:8090`. `RIMSKY_SWEEP_INTERVAL`
and `RIMSKY_SWEEP_TTL` are durations; when unset (or non-positive) the
sweep loop falls back to its built-in defaults of `5m` and `24h`
respectively.

## Layout

- `cmd/` — gRPC server entry point + sweep loop wiring.
- `server/` — gRPC `ClaimProducerServer` adapter.
- `store/` — four-verb logic over the filesystem.
- `sweep/` — periodic reaper for leaked staging directories.
- `template.yaml` — worked-example template using the producer.

## Conformance

Once the binary is running locally, exercise the rimsky-side protocol
expectations with the ClaimProducer conformance probe. The probe ships
in the rimsky repository (not in this examples module), so run it from a
rimsky checkout:

```sh
# from your local rimsky checkout
go run ./cmd/rimsky-claim-producer-conformance \
   --endpoint grpc://localhost:8090
```

The `--endpoint` value is a single URL carrying the `grpc://` scheme;
there is no separate `--transport` flag. Pass `--timeout` to adjust the
per-check deadline (default 10s) and `--check-observability` to also
probe the ClaimProducerObservability surface.

## Side-table caveats

The `producer_state.jsonl` side-table is a single-writer-per-root
artifact: `appendEntry` and `removeEntry` both read the full file,
modify, and write back via a `.tmp` + `os.Rename` swap (atomic on the
same filesystem). Concurrency is serialised by the coarse `Store.mu`
mutex, which is sufficient as long as **only one process** operates on
a given `<root>`. Running two instances of the binary against the same
root would race on the side-table writes; the second writer's `readAll`
could see a half-written file (parse error) or stomp the first writer's
update. The reference deployment assumes one producer process per
filesystem root; production deployments should enforce this via
filesystem locks or run a single replica per substrate.

## See also

- [`docs/agents/examples/atomic-staging.md`](../../docs/agents/examples/atomic-staging.md) — pattern doc.
- [`docs/concepts/atomic-staging.md`](../../docs/concepts/atomic-staging.md) — the pattern as a concept.
- [`docs/concepts/claim-producer.md`](../../docs/concepts/claim-producer.md) — protocol surface.
- [`docs/concepts/auto-terminal.md`](../../docs/concepts/auto-terminal.md) — holding-subgraph resolution (Commit/Abandon on aggregate outcome).
- [`docs/concepts/claim-co-holdership.md`](../../docs/concepts/claim-co-holdership.md) — how verifiers co-hold the acquirer's claim.
