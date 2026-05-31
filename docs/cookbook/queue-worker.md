# A single-node queue worker

## The problem

You have a stream of work items — review tasks, scrape targets, documents
to classify — landing in a table. You want one worker that pulls the next
item, processes it, and comes back for more, with no two workers grabbing
the same item and no item lost if a worker dies mid-flight.

## The rimsky shape

A queue is just a [claim producer](../concepts/claim-producer.md) whose
selector resolves to "the next available row" instead of a fixed address.
The bundled postgres store (`deploy/`'s `topics-ring` producer) does this
with a **pick policy**: the `@review-queue` selector runs `FOR UPDATE SKIP
LOCKED` over an items table and hands the picked row's payload back as the
[claim](../concepts/claim.md). One node holds that claim for the duration
of its run; the [claim handle ledger](../concepts/claim-handle.md) — not
the store — is the authority on who holds what, so the pick is exclusive
across the whole deployment.

To keep pulling, the node subscribes to *its own* terminal signal — the
"drain my own queue" idiom (see
[node-subscription](../concepts/node-subscription.md)). Each successful
run re-fires the node, which opens the claim again and gets the next item.
The `@review-queue` policy is configured `on_commit: recycle`, so a
committed item goes back to the tail of the ring rather than being
consumed — useful for a demo that never runs dry; switch to `pop` for a
drain-once queue.

Primitives: **claim producer** (the postgres pick policy), **claim**
(the picked item), **self-subscription** (the loop), **frame** (one
cascade resolution per pulled item).

## Walkthrough

Runs on the published [`deploy/`](../../deploy/) stack, which ships the
`topics-ring` postgres producer with the `@review-queue` pick policy
already configured and the `topics_items` table already created.

Bring the stack up:

```sh
docker compose -f deploy/docker-compose.yml up -d
```

Seed a few items directly into the store's admin endpoint (port `9121`,
exposed by the compose file). Operators talk to the store directly for
seeding — never through rimsky:

```sh
curl -s -X POST http://localhost:9121/admin/items/@review-queue \
  -H 'Content-Type: application/json' \
  -d '{"items":[
        {"payload":{"doc":"alpha"}},
        {"payload":{"doc":"beta"}},
        {"payload":{"doc":"gamma"}}
      ]}'
# → {"inserted":3}
```

Save the template as `queue-worker.yml`. The worker pulls one item per
run and re-fires on success:

```yaml
name: queue-worker
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: worker
    executor: http-node
    stores:
      - { name: topics-ring, selector: "@review-queue", intent: rw, alias: item }
    subscribes:
      - { node: worker, type: terminal/success, frame: next }
    attributes:
      schema:
        type: object
        properties:
          # stub_probe short-circuits the bundled http-node stub before its
          # transport-config check, so a node that makes no real HTTP call
          # still closes with a clean success. A schema `default:` flows into
          # the dispatch bag verbatim (it is never substituted).
          stub_probe:
            type: boolean
            default: true
          picked:
            type: string
            source: "{{claim.item.payload.doc}}"
```

The `subscribes:` entry is the self-edge: on every `terminal/success` the
worker opens a fresh [frame](../concepts/frame.md) (`frame: next`) and
runs again, pulling the next item off the ring. The `http-node` executor
runs in stub mode in the published stack (`RIMSKY_EXECUTOR_STUB_MODE=1`):
with `stub_probe: true` in the dispatch bag it short-circuits its network
path and closes the stream with a clean `StreamClose{Success}` on every
dispatch — exactly the loop driver we want. It advertises a permissive
attribute schema, so the `picked` source-bound attribute and the rest of
the bag pass the dispatch-time schema gate.

Register, deploy, instantiate:

```sh
rimsky template register queue-worker.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=01H...
```

Watch the worker cycle through items (each run records the item it picked
in the `picked` attribute):

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state}'
# → {"node_type":"worker","state":"running"}   # while a pull is in flight
# → {"node_type":"worker","state":"fresh"}      # between iterations
```

The single `worker` node is `running` (or `stale`) while a frame is in
flight and settles back to `fresh` between iterations. Because
`@review-queue` recycles on commit, the worker keeps finding work and the
loop continues frame after frame. To stop it, terminate the instance:

```sh
rimsky instance delete <instance_id>
```

> **Want a drain-once queue instead of a ring?** That is a store-config
> change (`on_commit: pop`), not a template change — and the published
> `deploy/` stack ships the recycle policy. Re-point the `store-postgres`
> config to a `pop` policy to exercise the drain-once shape.

## Without rimsky

By hand you would write the worker loop yourself: a `SELECT ... FOR UPDATE
SKIP LOCKED`, a visibility-timeout sweep to recover items abandoned by a
crashed worker, an at-least-once vs at-most-once decision baked into your
transaction boundaries, and a back-off loop polling for new work. Every
new worker type re-implements the same dequeue/ack/recover plumbing.
Rimsky moves the dequeue, the exclusivity, and the crash-recovery into the
claim-producer protocol, so the template carries only "what to do with an
item" and the loop is one `subscribes:` line.
