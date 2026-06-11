# `filesystem` blob backend

**Multi-process safe:** yes, given a shared filesystem (NFS, k8s PVC,
EFS, etc.). Each rimsky process must mount the same `root` directory.

## What it is

Stores blobs as files under a configured root directory. Handles
formatted as `fs:<relpath>` where `<relpath>` is a 2-level fanout
derived from `sha256(NodeID || 0x00 || AttributeName || 0x00 || Hint)`
(the key segments are joined with NUL bytes, not a printable
delimiter, so no segment value can collide with the separator) plus a
unique suffix (`-<unix-nano>-<seq>.bin`) to avoid collisions. Atomic
writes via temp + rename + fsync.
<!-- @source: lib/foundation/persistence/blob_filesystem.go::derivePath -->
<!-- @source: lib/foundation/persistence/blob_filesystem.go::Write -->

## When to use

- Multi-process deployments where you control a shared volume.
- Payloads in the 100 KB – 100 MB range that don't justify the LO
  overhead.
- Deployments that want blob bytes outside the database backup
  perimeter (e.g. faster point-in-time recovery on Postgres).

## Configuration

```yaml
persistence:
  blob:
    backend: filesystem
    spill_threshold_bytes: 65536
    filesystem:
      root: /var/lib/rimsky/blobs
    retention:
      orphan_sweep_interval: 1h
      retention_after_unreferenced: 24h
```

## Operational notes

- The root directory must be writable by every rimsky process in the
  deployment. Permissions: rimsky creates directories with mode `0o755`
  — the root at construction, the fanout levels at write time; blob
  files are written as `0o600` temp files and atomically renamed into
  place.
  <!-- @source: lib/foundation/persistence/blob_filesystem.go::NewFilesystemBackend -->
  <!-- @source: lib/foundation/persistence/blob_filesystem.go::Write -->
- Path-escape rejection: handles like `fs:../../../etc/passwd` are
  rejected on read; the resolved path is normalized inside `root`.
  <!-- @source: lib/foundation/persistence/blob_filesystem.go::absFromHandle -->
- 2-level directory fanout: blobs under `<root>/<2-hex>/<2-hex>/<file>`
  to keep directory listings small at scale (millions of blobs).
- Backup: include the configured `root` in your backup set; rimsky
  does not duplicate bytes back into Postgres.

## Conformance

```sh
rimsky conformance blob-backend \
  --backend filesystem \
  --root /tmp/blob-conformance
```
