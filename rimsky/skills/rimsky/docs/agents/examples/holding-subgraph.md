# Holding-subgraph template demonstrating held-claim resolution

Three nodes: an acquirer and two co-holders. Both co-holders complete successfully; the held claim's automatic resolution computes the aggregate outcome (all-success → `Commit`) and fires `ClaimProducer.Commit`.

The holding subgraph is built from `holds:` edges. `holds:` is the **sole** co-holdership directive: it does BOTH jobs — it binds the upstream claim's address into the co-holder's leaf request AND adds the co-holder to the acquirer's holding subgraph. Each co-holder declares `holds: { <alias>: { from: <acquirer-type> } }`; the outer key (`<alias>`) is the local alias and the name the validator looks up on the upstream node's `stores:` block, and `from:` names the acquirer node-type directly. `HoldingSubgraphsForTemplate` iterates `n.Holds` and adds the declaring node to the `(from, alias)` subgraph. With at least one co-holder the subgraph satisfies `IsHeld()` (`len(Members) > 1`), which gates the held-claim path: the acquirer's acquire-time holder row (`insertHeldClaimHoldersAtAcquire` inserts it only when held), the claim-handle's `is_held` flag, the co-holder branch of the release path, and the aggregate auto-terminal that fires the store verb. The co-holder rows themselves are inserted per `holds:` entry at each co-holder's own acquire (`insertCoHolderClaimHoldersAtAcquire`). With no `holds:` edge the claim is non-held (subgraph size 1, only the acquirer), so the acquirer gets no acquire-time holder row and the held auto-terminal path never engages; the holder listing applies no source filter, so it returns every holder row regardless of how it was inserted. <!-- @source: lib/graph/node/inheritance.go::HoldingSubgraphsForTemplate, lib/graph/node/inheritance.go::HoldingSubgraph.IsHeld, lib/foundation/spec/graphs.go::HoldsBinding -->

The `stub` executor here is the dockerized test stub executor (a test fixture, not a published image — see [`../../executors/stub/README.md`](../../executors/stub/README.md)). It returns a canned terminal `StreamClose{Success}` (`changed=false`, no attribute writeback) for every dispatch unconditionally, ignoring the request's `attributes` bag and `node_type` — so every node in this example reaches `StreamClose{Success}`, which demonstrates the all-success path. To exercise the any-failure → `Abandon` path, you would need an executor that drives the desired outcome (e.g. `claude-agent`, or a scripted stub registered programmatically by a scenario test).

**Precondition:** a running rimsky deployment (stand one up from the published images — see the [operator guide](../../operator-guide.md)).

## 1. The template

Save as `holding.yml`:

```yaml
name: holding-subgraph
version: "1.0"
frame_resolution_mode: serial_queue
params_schema:
  type: object
  additionalProperties: true
nodes:
  - type: acquirer
    executor: stub
    stores:
      - { name: stub, selector: "snapshots/{{params.snapshot_id}}", intent: rw, alias: snapshot }
    attributes:
      schema:
        type: object
        properties:
          handle:
            type: string
            source: "{{claim.snapshot.address}}"

  - type: coholder-a
    executor: stub
    subscribes:
      - { node: acquirer, type: terminal/success }
    holds:
      snapshot: { from: acquirer }
    attributes:
      schema:
        type: object
        properties:
          read_via:
            type: string
            source: "{{claim.snapshot.address}}"

  - type: coholder-b
    executor: stub
    subscribes:
      - { node: acquirer, type: terminal/success }
    holds:
      snapshot: { from: acquirer }
    attributes:
      schema:
        type: object
        properties:
          read_via:
            type: string
            source: "{{claim.snapshot.address}}"
```

## 2. Register, deploy, instantiate

```sh
rimsky template register holding.yml
rimsky template deploy sha256-...
rimsky instance create sha256-... --params '{"snapshot_id":"snap-1"}'
```

## 3. Observe held-claim resolution

While the co-holders are running, the acquirer's claim handle is in held state. First obtain the claim handle id: the acquirer's claim acquisition appends a `lock_acquired` event to the instance's event log, and its `payload.holder_id` is the claim handle id (this event lands at acquire time, so the id is available while the run is still in flight; every holding-subgraph member references the same handle, hence the `unique`): <!-- @source: lib/runtime/runner_acquire_postcommit.go::emitLockAcquired -->

```sh
claim_handle_id=$(curl -s "http://localhost:8080/v1/events?instance_id=<instance_id>&kind=lock_acquired" \
  | jq -r '[.events[].payload.holder_id] | unique | .[0]')
```

Then list the holding-subgraph members for that handle:

```sh
curl http://localhost:8080/v1/lock-holders/$claim_handle_id/claim-holders
```

After both co-holders terminate, the held claim's automatic resolution computes the aggregate outcome (all completed → `Commit`) and fires `ClaimProducer.Commit`. Terminal does NOT delete the claim handle — it *promotes* it: every terminal flips `rimsky_claim_handles.state` and preserves the row past terminal, so an all-success commit promotes the handle to `state='committed'` (a later retention sweep reaps non-durable terminal rows). The holder rows are likewise preserved, each transitioned `active`→`completed` with `completed_at` set.

## Verification

```sh
curl -s http://localhost:8080/v1/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
```

Expected output:

```json
[
  { "node_type": "acquirer", "state": "fresh" },
  { "node_type": "coholder-a", "state": "fresh" },
  { "node_type": "coholder-b", "state": "fresh" }
]
```

Once the held claim has been resolved, the handle row is preserved (promoted to `state='committed'`, not deleted), and so are its holder rows — each transitioned to `state='completed'`. `ListByClaimHandleID` applies no state filter, so the listing returns ALL holders regardless of state. This holding subgraph has three members — the acquirer (its own holder row is inserted at acquire-time, gated on `IsHeld()`) plus coholder-a and coholder-b (each added via its `holds:` edge and inserting its own holder row at its own acquire) — so the listing returns three holders, all `completed` (the route returns `200 OK` with `{"holders": [...]}`; it does NOT return `404`):

```sh
# $claim_handle_id from step 3 (the lock_acquired event's payload.holder_id)
curl -s http://localhost:8080/v1/lock-holders/$claim_handle_id/claim-holders \
  | jq '.holders | length'
# Expected: 3
curl -s http://localhost:8080/v1/lock-holders/$claim_handle_id/claim-holders \
  | jq '[.holders[] | .state]'
# Expected: ["completed","completed","completed"]
```

## See also

- [`../../concepts/auto-terminal.md`](../../concepts/auto-terminal.md) — held-claim resolution over the holding subgraph (the mechanism this example exercises).
- [`../../concepts/claim-co-holdership.md`](../../concepts/claim-co-holdership.md) — the `holds:` directive (used here): the sole co-holdership directive. It both binds the upstream claim's address into the co-holder's leaf request AND builds the holding subgraph that drives aggregate auto-terminal.
