# Drive a node from an external event

## The problem

Work should start when something happens outside rimsky: a file lands in a
bucket, a webhook fires, a clock ticks, an operator hits a button. You want
a long-lived instance that sits idle until the event arrives, then advances
the right node — without polling and without the event source needing to
understand your graph.

## The rimsky shape

External events enter rimsky as [messages](../concepts/message.md): a
boundary-crossing envelope POSTed to the instance's message-emit endpoint.
A message creates (or joins) a [frame](../concepts/frame.md) and is
matched against the instance's [subscriptions](../concepts/node-subscription.md)
at the frame boundary; matched nodes are marked stale and dispatched. The
node reads the event's payload through the substitution leaf
`{{trigger.message.payload.<field>}}`.

Two senders fire the same endpoint identically:

- **Operators** POST with `sender_kind: "operator"` — the building block,
  available on both published stacks.
- **[Publishers](../concepts/publisher.md)** (of which
  [sensors](../concepts/sensor.md) are one class) POST with
  `sender_kind: "publisher"` after rimsky has registered a
  publisher-subscription for them at instance creation. The `deploy/`
  stack ships four bundled sensors (cron / http / object-store / webhook),
  each watching an external substrate and emitting the *same* message
  envelope when it changes.

The payload is [inert](../concepts/inertness.md): rimsky never reads it; it
flows untouched from sender to the consuming node's substitution leaf.
Routing is the subscriber's CEL predicate over a signal, never the
platform inspecting your bytes.

Primitives: **message** (the envelope + the messages endpoint),
**node-subscription** (matching the message to a node), **frame** (the
message-delivery frame), **publisher / sensor** (the bundled event sources).

## Walkthrough

Runs on the published [`deploy/`](../../deploy/) stack, using the
`http-node` executor (stub mode, `RIMSKY_EXECUTOR_STUB_MODE=1`). Bring it
up:

```sh
docker compose -f deploy/docker-compose.yml up -d
```

Save the template as `event-driven.yml`. The `react` node subscribes to
inbound invalidate messages targeted at itself and reads the payload:

```yaml
name: event-driven
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: react
    executor: http-node
    subscribes:
      - { node: react, type: "message/invalidate/operator/react", frame: next }
    attributes:
      schema:
        type: object
        properties:
          # stub_probe short-circuits the bundled http-node stub before its
          # transport-config check; a schema `default:` flows into the
          # dispatch bag verbatim (it is never substituted).
          stub_probe:
            type: boolean
            default: true
          received:
            type: string
            source: '{{trigger.message.payload.note | "no-note"}}'
```

The subscription `type:` is a [signal](../concepts/signal.md) type-path:
`message/<kind>/<sender_kind>/<target>`. Here it matches an
`invalidate`-kind message from an `operator` targeting the `react` node.
The substitution falls back to `"no-note"` when no payload field is
present (the `<directive> | <literal>` fallback grammar; the literal must
be a double-quoted JSON string, so the YAML scalar is single-quoted to
keep both the directive and the literal valid).

Register, deploy, instantiate:

```sh
rimsky template register event-driven.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=01H...
```

`react` has no upstream subscription (its only `subscribes:` entry names
itself) and no upstream-node attribute reference, so rimsky treats it as a
**root** and dispatches it once at instance creation. With no trigger
message bound to that first frame, `{{trigger.message.payload.note}}` is
absent and the `| "no-note"` fallback fires — the node runs once and
settles `fresh` with `received: "no-note"`, then waits for an event:

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state}'
# → {"node_type":"react","state":"fresh"}
```

Now fire an event into the instance. There is no `messages create` CLI
verb — the message-emit endpoint is plain HTTP, the same surface a
publisher uses:

```sh
curl -s -X POST http://localhost:8080/instances/<instance_id>/messages \
  -H 'Content-Type: application/json' \
  -d '{"kind":"invalidate","target":"react","payload":{"note":"file-landed"}}'
# → {"message_id":"..."}
```

The message lands in the message ledger, is delivered at the next frame
boundary, matches `react`'s subscription, and dispatches the node — which
pulls `"file-landed"` into its `received` attribute. Confirm: `react`
settles back to `fresh` once the delivery frame resolves (it reads `stale`
or `running` while the frame is in flight):

```sh
rimsky messages tail --instance <instance_id>
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '.nodes[] | {node_type, state}'
# → {"node_type":"react","state":"fresh"}
```

> **Wiring a real sensor.** To have a *sensor* fire this node instead of an
> operator, declare the publisher on the template's `publishers:` block
> (e.g. `kind: cron`, `target_node: react`). The `deploy/` stack ships four
> bundled sensors (cron / http / object-store / webhook); at instance
> creation rimsky calls the matching sensor's subscribe verb, and the
> sensor then POSTs the same envelope with `sender_kind: "publisher"` on
> each fire. The node's subscription `type:` changes its third segment to
> `publisher` accordingly. The envelope shape and the consuming node are
> otherwise identical to the operator path above — which is the whole point
> of the unified messages endpoint.

## Without rimsky

By hand you would stand up a listener per event source, a durable inbox so
events are not lost between arrival and processing, a dispatcher that maps
each event to the right handler, and idempotency keys so a redelivered
webhook does not double-fire. Each new source — cron, bucket, webhook —
gets its own bespoke deposit path. Rimsky gives every source one envelope
shape and one endpoint, persists the message on receipt, matches it to
nodes by subscription, and carries a universal idempotency-key header — so
adding a source is a config entry, not a new ingestion pipeline.
