---
error: fs/root_unavailable
surfaced_to: operator
---

# Filesystem store root unavailable (`fs/root_unavailable`)

## What it means

The bundled `store-filesystem` claim producer rejected a `ClaimProducer` verb (`Open` / `Commit` / `Abandon` / `Release`) because its configured backing root is missing or not writable at verb time. The store's `checkRootAvailable` gate runs at the head of every claim-mutating verb and short-circuits with a classed `fs/root_unavailable` error — the rejection is loud, not silent: no claim-handle mutation is acked against a root the store could not actually honor. <!-- @source: lib/services/stores/filesystem/store/store.go::checkRootAvailable, lib/services/stores/filesystem/store/errors.go::RootUnavailableClass -->

This is the operator-misconfiguration case: the path in the producer's `root:` config is wrong, the volume backing it is unmounted, or the mount went read-only. It is NOT a busy-resource refusal (that is the `Unavailable` outcome on `Open`, surfaced as `acquire/unavailable` — see [`acquire_unavailable.md`](acquire_unavailable.md)); `fs/root_unavailable` is a faulted RPC, not an `Unavailable` outcome.

The class crosses the wire on every faulted verb. The store's gRPC server boundary maps the in-process `*ClassedError` into a `codes.Internal` status carrying a `google.rpc.ErrorInfo` detail (`Reason = "fs/root_unavailable"`, `Domain = "rimsky.store-filesystem"`); rimsky's claim-producer client decodes the detail into `peer.ProducerCallError.ErrorClass` and routes the supervisor's error-policy chain on that class. <!-- @source: lib/services/stores/filesystem/server/server.go::classedStatus, lib/runtime/peer/errors.go::extractErrorClass -->

The class is advertised on `CapabilitiesResponse.declared_error_classes` (the producer-declared error vocabulary added in v0.9.0 — see `proto:claim_producer.proto::CapabilitiesResponse.declared_error_classes`), so the template validator's range-check accepts `error_types: { fs/root_unavailable: ... }` on nodes whose claim references the filesystem store. <!-- @source: lib/services/stores/filesystem/server/server.go::producerDeclaredErrorClasses -->

## When it happens

On any `ClaimProducer` verb against `store-filesystem` when `os.Stat(root)` or `syscall.Access(root, W_OK)` fails. Typical causes:

| Cause | Symptom |
| --- | --- |
| Operator typo in the store's `root:` config | Verb fails immediately at first dispatch with `root … is not accessible`. |
| Volume backing the root never mounted, or unmounted at runtime | The path stat fails (ENOENT) or returns the mount point's stat (root contents gone). |
| Mount went read-only (filesystem error, quota, kernel remount) | The stat succeeds; the writability probe fails with EROFS / EACCES. |
| Containerised store missing its volume mount entirely | First verb fails; never recovers without operator action. |

The verb path is independent of which verb fires — `Open`, `Commit`, `Abandon`, and `Release` all gate on `checkRootAvailable`. A run that opened a claim while the root was healthy and lost it before terminal observes the class on the release-path verb (`Commit` for success, `Abandon` for failure), not on `Open`.

## What to do

This is operator-misconfiguration. Routing it through `error_types: { fs/root_unavailable: { policy: [{ action: retry, ... }] } }` is reasonable only if you expect the root to come back without intervention (a flapping NFS mount); the durable case wants `give_up` so the failure surfaces on the event log and is investigated.

Fix the root cause:

- Check the store-service's logs and `root:` config. The error message names the path and the underlying OS error (`is not accessible: stat /...: no such file or directory`, or `is not writable: …: permission denied`).
- For containerised deployments, confirm the volume mount is present and writable inside the store-service container (`docker exec <store> ls -ld <root>`). The `store-filesystem` image refuses to start with an empty `root:` config but does NOT pre-flight the path's existence — a missing mount surfaces here on the first verb.
- For network-attached storage, confirm the mount is healthy from the store-service host and that no quota / read-only remount has fired.

For API-triggered verbs (control-api claim endpoints), the class + message surface synchronously on the HTTP response so a caller sees the store's own diagnosis instead of an anonymous 500.

## See also

- [`acquire_unavailable.md`](acquire_unavailable.md) — the busy-resource sibling: an `Unavailable` outcome on `Open`, not a faulted verb.
- [`../../concepts/claim-producer.md`](../../concepts/claim-producer.md)
- [`../../protocols/claim-producer.md`](../../protocols/claim-producer.md)
- [`../../services/README.md`](../../services/README.md) — the `store-filesystem` config keys (including `root:`).
