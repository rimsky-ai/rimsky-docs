# Cap concurrency with a counting semaphore

## The problem

Something downstream has a hard ceiling: an API that allows 50 in-flight
calls, a model budget, a database that falls over past N concurrent
writers. You want many nodes (or many instances) to share that ceiling and
block when it is reached — without coordinating it by hand in every node.

## The rimsky shape

A [named lock](../concepts/named-lock.md) is a producer-independent
capacity counter. The operator declares it once in `rimsky.yml` with a
limit; templates reference it by name on a node's `locks:` block. At
dispatch the node must acquire a slot; when `limit` slots are held, the
next acquirer waits. It materializes as a `named`-kind row in the
[claim-handle ledger](../concepts/claim-handle.md) — the same ledger that
holds scope claims, so capacity counting and claim conflict are walked in
one deterministic order and cannot deadlock against each other.

A named lock is **deployment-scoped, not data-scoped**: it is the right
tool when the constraint has nothing to do with which rows you touch
("at most 50 model calls anywhere") and the wrong tool when the constraint
is "don't let two nodes write the same file" — that is a scope
[claim](../concepts/claim.md), not a lock.

Primitives: **named lock** (the counting semaphore), **claim-handle**
(where the count lives), **node** (the holder for the duration of its
run).

## Walkthrough

Runs on the published [`deploy/`](../../deploy/) stack, which declares two
named locks in `rimsky.yml`:

```yaml
named_locks:
  "topics-ring:concurrent-claims": { limit: 5 }
  model-budget:                    { limit: 50 }
```

Bring the stack up:

```sh
docker compose -f deploy/docker-compose.yml up -d
```

Save the template as `budgeted.yml`. The node holds a `model-budget` slot
for the duration of its run:

```yaml
name: budgeted-work
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: call-model
    executor: claude-agent
    locks:
      - { name: model-budget }
    attributes:
      schema:
        type: object
        additionalProperties: true
```

Register, deploy, instantiate:

```sh
rimsky template register budgeted.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=01H...
```

The node acquires one of the 50 `model-budget` slots, runs, and releases
it at terminal. With the limit at 50 a single instance never blocks; the
cap bites when more than 50 holders are live at once — across *every*
instance and node that references the lock, since the count is
deployment-wide. You can watch held capacity on the diagnostics surface:

```sh
curl -s http://localhost:8080/admin/diagnostics/held-frames | jq
```

A node waiting on a saturated lock stays `stale` with its frame
[held](../concepts/frame.md) — its dispatch waits in the queue until a
slot frees, then dispatches with no polling and no retry storm.

> **Mutex is just `limit: 1`.** A whole-job mutex is a counting semaphore
> with a single slot. Declare one in `rimsky.yml` alongside the existing
> locks (the `deploy/` stack ships `topics-ring:concurrent-claims:
> { limit: 5 }` and `model-budget: { limit: 50 }`) — e.g.
> `single-writer: { limit: 1 }` — and reference it the same way:
> `locks: [{ name: single-writer }]`.

## Without rimsky

By hand you would reach for a distributed semaphore — a Redis token
bucket, a database advisory lock, a leased-counter table — and wire every
worker to acquire before work and release after, with a lease timeout so a
crashed holder does not leak a slot forever. You would also have to order
that acquisition against any *other* lock or row-lock the worker takes, or
risk a classic lock-ordering deadlock. Rimsky folds the counter into the
same ledger and acquisition order as scope claims, so the deadlock-free
ordering is the platform's job and the template carries one `locks:` line.
