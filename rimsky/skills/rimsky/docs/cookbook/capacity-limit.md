# Cap concurrency with a counting semaphore

## The problem

Many nodes (or many instances) share a hard downstream ceiling — an API
that allows 50 in-flight calls, a model budget, a database that falls over
past N concurrent writers — and must block when it is reached, without
coordinating it by hand in every node.

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

Needs a rimsky deployment whose `rimsky.yml` declares the named lock:

```yaml
named_locks:
  "topics-ring:concurrent-claims": { limit: 5 }
  model-budget:                    { limit: 50 }
```

Stand rimsky up from the published images (see the
[operator guide](../operator-guide.md)); the
[reference config](../reference/config/rimsky.yml) shows the `named_locks`
block.

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
cap bites when more than 50 holders are live at once.

To observe capacity, watch the `rimsky_claim_acquisitions_total{producer,
intent}` Prometheus counter on `GET /metrics`, or query the live holders
directly — a held slot is a `named`-kind row in the
[claim-handle ledger](../concepts/claim-handle.md). The admin
diagnostics surfaces do **not** help here: `held-frames` reports only
frames with a [parked](../concepts/parked-state.md) node, and a node
blocked on a saturated named lock does not park — its per-candidate
acquisition tx rolls back and it stays `stale` in the queue (see the
gotcha below), so it never appears as a held frame.

## Gotchas

- **The count is deployment-wide.** The cap bites across *every* instance
  and node that references the lock, not per-instance — that is the point
  of a named lock, but it means a saturated lock blocks unrelated
  instances too.
- **A waiting node stays `stale`.** A node waiting on a saturated lock
  does not park — it stays `stale` and its dispatch row waits `pending`
  in the queue until a slot frees, then dispatches with no polling and no
  retry storm. (It never appears as a held frame; see the diagnostics note
  above.)
- **Mutex is just `limit: 1`.** A whole-job mutex is a counting semaphore
  with a single slot. Declare one in `rimsky.yml` alongside the existing
  locks (the reference config ships `topics-ring:concurrent-claims:
  { limit: 5 }` and `model-budget: { limit: 50 }`) — e.g.
  `single-writer: { limit: 1 }` — and reference it the same way:
  `locks: [{ name: single-writer }]`.

## Without rimsky

By hand you would reach for a distributed semaphore — a Redis token
bucket, a database advisory lock, a leased-counter table — and wire every
worker to acquire before work and release after, with a lease timeout so a
crashed holder does not leak a slot forever. You would also have to order
that acquisition against any *other* lock or row-lock the worker takes, or
risk a classic lock-ordering deadlock. Rimsky folds the counter into the
same ledger and acquisition order as scope claims, so the deadlock-free
ordering is the platform's job and the template carries one `locks:` line.
