# A single-node queue worker

## Problem

One worker drains a stream of work items — review tasks, scrape targets,
documents to classify — landing in a table: pull the next item, process
it, come back for more. No two workers grab the same item; no item is lost
if a worker dies mid-flight.

## Rimsky shape

A queue is just a [claim producer](../concepts/claim-producer.md) whose
selector resolves to "the next available row" instead of a fixed address.
The bundled postgres store (the `topics-ring` producer) does this
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

## Template

**Assumes running.** This recipe references three services by the names
in the template below; each must already be running and registered
under that name. The corpus does not ship a copy-pasteable deploy
chain — wire these up per their config docs:

- A **control-api** process (rimsky's HTTP API on `:8080`) — see
  [`reference/config/rimsky.yml`](../reference/config/rimsky.yml) and
  [`services/README.md`](../services/README.md).
- A **`store-postgres` claim-producer registered as `topics-ring`**,
  configured with the `@review-queue` pick policy against a
  `topics_items` table — see
  [`reference/config/store-postgres.yml`](../reference/config/store-postgres.yml).
- An **`http-node` executor in stub mode**
  (`RIMSKY_EXECUTOR_STUB_MODE=1`) registered under the executor name
  `http-node` — see [`services/README.md`](../services/README.md).

Seed a few items into the store's admin endpoint (port `9121` in the
reference config). Operators seed the store directly, never through rimsky:

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
    # Pre-dispatch acquisition failures route through error_types:,
    # keyed on the EXACT error class. This entry catches the synthetic
    # acquire/unavailable class (a durable-conflict on the claim scope,
    # or a producer that names no class on its Unavailable response);
    # without it, those failures take the fail-fast default (give_up)
    # rather than pass. It does NOT catch the drained queue: the postgres store names its
    # own class (pg/claim_unavailable) on the empty-pick Unavailable,
    # and that key cannot be declared here — registration range-checks
    # error_types: keys against the executor's declared_error_classes,
    # and http-node declares only http/*. See Gotchas for what an empty
    # queue actually does.
    error_types:
      acquire/unavailable:
        policy:
          - { action: pass }
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
runs again, pulling the next item off the ring.

Register, deploy, instantiate:

```sh
rimsky template register queue-worker.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=6b1f0c9a-4e2d-4f7b-9a3c-d5e8f1a2b3c4
```

Watch the worker cycle through items (each run records the item it picked
in the `picked` attribute):

```sh
curl -s http://localhost:8080/v1/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state}'
# → {"node_type":"worker","state":"running"}   # while a pull is in flight
# → {"node_type":"worker","state":"fresh"}      # between iterations
```

The single `worker` node is `running` (or `stale`) while a frame is in
flight and settles back to `fresh` between iterations. Because
`@review-queue` recycles on commit, the worker keeps finding work and the
loop continues frame after frame. The instance is durable — it never
terminates on its own (instances are durable by default; there is no
auto-terminate on drain). To stop it, force-terminate then delete:

```sh
rimsky instance kill <instance_id> --force   # marks it terminal, abandons the in-flight run
rimsky instance delete <instance_id>          # frees the row (refused until terminal)
```

A plain `rimsky instance delete` on a still-running instance is **refused**
("instance is not in terminal state") — `kill --force` is what makes it
terminal first.

## Gotchas

- **The `http-node` executor must run in stub mode**
  (`RIMSKY_EXECUTOR_STUB_MODE=1`). With `stub_probe: true` in the dispatch
  bag it short-circuits its network path before the transport-config check
  and closes the stream with a clean `StreamClose{Success}` on every
  dispatch — exactly the loop driver we want. A schema `default:` flows
  into the dispatch bag verbatim (it is never substituted). The stub
  advertises a permissive attribute schema, so the `picked` source-bound
  attribute and the rest of the bag pass the dispatch-time schema gate.
- **Operators seed the store directly, never through rimsky** — the seed
  `curl` hits the store's admin endpoint (port `9121`), not the rimsky API.
- **`@review-queue` recycles on commit.** A committed item goes back to
  the tail of the ring rather than being consumed, so the demo never runs
  dry. To get a drain-once queue instead of a ring, change the store config
  (`on_commit: pop`), not the template — the reference config ships the
  `recycle` policy. Re-point the `store-postgres` config to a `pop` policy
  to exercise the drain-once shape.
- **An empty queue is an *error class*, not a quiet no-op — and you
  cannot route it to `pass` on this worker.** With a `pop` policy the
  ring eventually empties; the next pull's `Open` returns
  `Available: false` with the producer-declared class
  `pg/claim_unavailable`, and rimsky keys the node's `error_types:`
  chain on that **exact** class (a producer-declared class overrides the
  synthetic `acquire/unavailable`, so the template's
  `acquire/unavailable: pass` entry never fires for this store's empty
  queue — it covers only durable-conflicts and class-less producers).
  But `error_types: { pg/claim_unavailable: ... }` is **rejected at
  registration**: the validator range-checks `error_types:` keys against
  the node's *executor's* `declared_error_classes`, http-node declares
  only `http/*`, and `pg/claim_unavailable` is not in the
  runtime-synthesized exempt set (`acquire/*` plus a fixed list). The
  producer-declared class is owned by the *store*, not the worker's
  executor, so no `error_types:` entry on this node can both register
  and match it. The drain therefore takes the
  [error-policy](../concepts/error-policy.md) fail-fast default:
  `give_up`, and the worker settles **`failed`**. The drain is still
  deterministic and observable — `give_up` emits
  `terminal/error/pg/claim_unavailable` (not `terminal/success`), so the
  self-edge stops matching and the loop ends cleanly, with the
  producer-declared class on the event log as the drain marker. Watch
  for it directly
  (`GET /v1/events?instance_id=<instance_id>&kind=terminal/error/pg/claim_unavailable`),
  or react in-graph with a second node carrying an instance-scoped
  wildcard subscription
  (`subscribes: [{ instance: true, type: terminal/error/*, frame: in }]`)
  — instance-scoped wildcards are not range-checked against any
  executor's vocabulary, which is exactly how rimsky-core's own
  `pg_error_classes` scenario observes this class.
- **A draining worker does NOT free its own instance.** When the queue
  drains, the `worker` node settles (`failed` via the `give_up` default
  above) and the self-edge stops firing — but the instance keeps living (durable
  by default). Nothing terminates the instance. For an ephemeral "run one
  more frame, then exit" worker, set `terminate_after_run: true` at
  create time: the instance self-terminates after its *next* frame ends
  (strict "run at most once more"), so this is the run-*one*-item shape,
  not drain-the-whole-queue. Set it with `rimsky run
  --terminate-after-run` (implied by `rimsky run --no-keep`), or — since
  the CLI `instance create` has no flag for it — `POST /v1/instances`
  with `{"template":"sha256-...","terminate_after_run":true}`. See the
  [README](README.md#instances-are-durable-by-default).

## Without rimsky

By hand you would write the worker loop yourself: a `SELECT ... FOR UPDATE
SKIP LOCKED`, a visibility-timeout sweep to recover items abandoned by a
crashed worker, an at-least-once vs at-most-once decision baked into your
transaction boundaries, and a back-off loop polling for new work. Every
new worker type re-implements the same dequeue/ack/recover plumbing.
Rimsky moves the dequeue, the exclusivity, and the crash-recovery into the
claim-producer protocol, so the template carries only "what to do with an
item" and the loop is one `subscribes:` line.
