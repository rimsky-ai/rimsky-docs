---
error: compose_prefix_violation
surfaced_to: cli-user
---

# Compose prefix violation

## What it means

A request attempted to create a tag or instance with an identifier in the reserved `compose:` namespace from a non-privileged caller. The control-api enforces the reservation server-side at the row-create sites (`POST /v1/tags`, `POST /v1/instances`) and rejects with HTTP 400; the request persists nothing. The `compose:` prefix is the source-of-truth namespace owned by the `rimsky compose` command, not hand-manageable through the standard tag/instance verbs.

## When it happens

Any of these from a caller that is not the privileged compose path:

- `POST /v1/tags` with `tag: "compose:..."` (e.g. `rimsky tag create compose:project-alpha:foo`).
- `POST /v1/instances` with `instance_key: "compose:..."`.
- The `rimsky tag mv` / `rimsky tag rm` hand-tag verbs targeting a `compose:`-prefixed tag (the CLI also rejects these client-side as a courtesy, but the control-api would reject the underlying API call regardless).

The privileged compose path is defined by BOTH conditions holding on the same request:

1. The `X-Rimsky-Compose-Origin: 1` HTTP header (the compose engine's intent claim).
2. The authenticated identity holds the `compose:origin` permission action.

The header alone is not a trust boundary — any authenticated caller could stamp it. The `compose:origin` permission is the load-bearing check; typically only the compose CLI's privileged API key carries it. A request with the header but without the permission lands on the same reject path as an unmarked request. A request with the permission but without the header is also rejected — the prefix stays reserved until the caller explicitly declares compose-origin intent.

## What to do

Pick a different tag identifier or instance key without the `compose:` prefix. The reserved prefix exists so the compose engine can own its namespace without colliding with hand-managed entities. If you genuinely need to create a `compose:`-prefixed name, do it through `rimsky compose up` (the compose engine), not through the hand-tag or `POST /v1/instances` paths.

## See also

- [`../../concepts/tag.md`](../../concepts/tag.md)
- [`../../concepts/permission.md`](../../concepts/permission.md)
