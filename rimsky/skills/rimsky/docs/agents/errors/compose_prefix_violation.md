---
error: compose_prefix_violation
surfaced_to: cli-user
---

# Compose prefix violation

## What it means

A `rimsky tag` verb attempted to create, move, or remove a tag whose identifier uses the reserved `compose:` prefix. The CLI rejects this client-side; `compose:` is a reserved namespace and is not hand-manageable.

## When it happens

`rimsky tag create compose:project-alpha:foo`, `rimsky tag mv compose:...`, or `rimsky tag rm compose:...` — any hand-management tag verb whose target tag begins with `compose:`.

## What to do

Pick a different tag identifier without the `compose:` prefix. The reserved prefix exists so compose-style management can own a namespace without colliding with hand-managed tags.

## See also

- [`../../concepts/tag.md`](../../concepts/tag.md)
