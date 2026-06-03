# Holding-subgraph template demonstrating held-claim resolution

Three nodes: an acquirer and two inheritors. Both inheritors complete successfully; the held claim's automatic resolution computes the aggregate outcome (all-success → `Commit`) and fires `ClaimProducer.Commit`.

The holding subgraph is built from `inherits:` edges only. Each inheritor declares `inherits: [{ claim: <alias> }]` naming an alias the acquirer holds on its `stores:` block; that directive — and only that directive — adds the inheritor to the acquirer's holding subgraph (`HoldingSubgraphsForTemplate` reads `Inherits`, never `holds:`). With at least one inheritor the subgraph satisfies `IsHeld()`, which gates every held-claim mechanism below: the acquirer's acquire-time holder row, the inheritor-release marking, and the aggregate auto-terminal that fires the store verb. A `holds:` co-holder binds the upstream claim's address into the leaf's request and inserts its own co-holder row, but on its own it does NOT make the claim held or build the holding subgraph — only `inherits:` does that. With no `inherits:` edge the claim is non-held, so the acquirer gets no acquire-time holder row and the held auto-terminal path never engages; a `holds:`-only setup would not produce this example's three-completed-holders / `Commit` outcome. (Where a claim is already held via `inherits:`, `holds:` co-holder rows are aggregated by auto-terminal alongside the inheritors — the holder listing applies no source filter.) <!-- @source: graph/node/inheritance.go::HoldingSubgraphsForTemplate, graph/node/inheritance.go::HoldingSubgraph.IsHeld, foundation/spec/template.go::InheritEntry -->

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

  - type: inheritor-a
    executor: stub
    subscribes:
      - { node: acquirer, type: terminal/success }
    inherits:
      - { claim: snapshot }
    attributes:
      schema:
        type: object
        properties:
          read_via:
            type: string
            source: "{{claim.snapshot.address}}"

  - type: inheritor-b
    executor: stub
    subscribes:
      - { node: acquirer, type: terminal/success }
    inherits:
      - { claim: snapshot }
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

While the inheritors are running, the acquirer's claim handle is in held state. List the holding-subgraph members for that handle:

```sh
curl http://localhost:8080/lock-holders/<claim_handle_id>/claim-holders
```

After both inheritors terminate, the held claim's automatic resolution computes the aggregate outcome (all completed → `Commit`) and fires `ClaimProducer.Commit`. Terminal does NOT delete the claim handle — it *promotes* it: every terminal flips `rimsky_claim_handles.state` and preserves the row past terminal, so an all-success commit promotes the handle to `state='committed'` (a later retention sweep reaps non-durable terminal rows). The holder rows are likewise preserved, each transitioned `active`→`completed` with `completed_at` set.

## Verification

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
```

Expected output:

```json
[
  { "node_type": "acquirer", "state": "fresh" },
  { "node_type": "inheritor-a", "state": "fresh" },
  { "node_type": "inheritor-b", "state": "fresh" }
]
```

Once the held claim has been resolved, the handle row is preserved (promoted to `state='committed'`, not deleted), and so are its holder rows — each transitioned to `state='completed'`. `ListByClaimHandleID` applies no state filter, so the listing returns ALL holders regardless of state. This holding subgraph has three members — the acquirer (its own holder row is inserted at acquire-time, gated on `IsHeld()`) plus inheritor-a and inheritor-b (each added via its `inherits:` edge and inserting its own holder row at its own acquire) — so the listing returns three holders, all `completed` (the route returns `200 OK` with `{"holders": [...]}`; it does NOT return `404`):

```sh
curl -s http://localhost:8080/lock-holders/<claim_handle_id>/claim-holders \
  | jq '.holders | length'
# Expected: 3
curl -s http://localhost:8080/lock-holders/<claim_handle_id>/claim-holders \
  | jq '[.holders[] | .state]'
# Expected: ["completed","completed","completed"]
```

## See also

- [`../../concepts/auto-terminal.md`](../../concepts/auto-terminal.md) — held-claim resolution over the holding subgraph (the mechanism this example exercises).
- [`../../concepts/claim-co-holdership.md`](../../concepts/claim-co-holdership.md) — the separate `holds:` directive (address binding for a downstream consumer). Contrast: `inherits:` (used here) builds the holding subgraph and drives aggregate auto-terminal; `holds:` binds the upstream claim's address into the leaf's request.
