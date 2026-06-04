# Two-node template with a claim dependency

Two nodes; the first declares a claim on the bundled stub claim producer; the second subscribes to the first via `subscribes:` and consumes the captured address through `{{nodes.<source>.attribute.<field>}}` (which also auto-subscribes the receiver to the sender's `attribute` topic).

**Precondition:** a running rimsky deployment (stand one up from the published images — see the [operator guide](../../operator-guide.md)).

The `stub` executor here is the dockerized test stub executor (a test fixture, not a published image — see [`../../executors/stub/README.md`](../../executors/stub/README.md)). It returns a canned terminal `StreamClose{Success}` (`changed=false`, no attribute writeback) for every dispatch unconditionally, ignoring the request's `attributes` bag and `node_type`. This example demonstrates the all-success path: both nodes commit, the claim is auto-`Commit`ted at the acquirer's terminal.

## 1. The template

Save as `two-node.yml`:

```yaml
name: two-node-claim
version: "1.0"
frame_resolution_mode: serial_queue
params_schema:
  type: object
  additionalProperties: true
nodes:
  - type: acquirer
    executor: stub
    stores:
      - { name: stub, selector: "items/{{params.item_id}}", intent: rw, alias: workspace }
    attributes:
      schema:
        type: object
        properties:
          captured_address:
            type: string
            source: "{{claim.workspace.address}}"

  - type: consumer
    executor: stub
    # The `{{nodes.acquirer.attribute.captured_address}}` substitution
    # below auto-subscribes consumer to acquirer's `attribute` topic;
    # the explicit `subscribes:` entry below makes that coupling
    # obvious to readers.
    subscribes:
      - { node: acquirer, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          relayed_address:
            type: string
            source: "{{nodes.acquirer.attribute.captured_address}}"
```

The `stub` claim producer must be configured under `claim_producers:` in `rimsky.yml` (see the [minimal `rimsky.yml`](minimal-rimsky-yml.md) for the shape).

## 2. Register, deploy, instantiate

```sh
rimsky template register two-node.yml
# returns: template_hash=sha256-..., tags=

rimsky template deploy sha256-...

rimsky instance create sha256-... \
    --params '{"item_id":"42"}'
# returns: instance_id=..., template_hash=sha256-..., node_count=2
```

## 3. Observe both nodes settle

```sh
curl http://localhost:8080/instances/<instance_id>/nodes
```

Expected output: both nodes in `fresh` state.

## Verification

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
```

Expected output:

```json
[
  { "node_type": "acquirer", "state": "fresh" },
  { "node_type": "consumer", "state": "fresh" }
]
```

The acquirer's claim was committed at its terminal (success → `Commit`); the claim-holders listing for that handle is empty:

```sh
# This claim is NON-held: neither node declares `holds:`,
# so no `rimsky_claim_holders` rows are ever inserted — that is why the
# listing is empty, not because anything was deleted. (The handle row
# itself is promoted to state='committed' at the terminal, not deleted.)
curl -s http://localhost:8080/lock-holders/<claim_handle_id>/claim-holders | jq '.holders | length'
# Expected: 0
```
