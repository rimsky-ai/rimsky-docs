# Recompute dependents when something upstream changes

## Problem

When an upstream value changes, the downstream should recompute — and
*only* the parts that actually depend on the change, not the whole graph. A
value derives from another: a classification depends on a fetched document,
a rollup depends on its inputs, a report depends on the data it summarizes.

## Rimsky shape

In rimsky the downstream node owns the coupling. It declares a
[subscription](../concepts/node-subscription.md) on the upstream node's
signals; when the upstream settles, the [cascade](../concepts/cascade.md)
marks the subscriber stale and the scheduler recomputes it. The edge is
**subscriber-driven**: the sender does not push a value or name its
downstreams — the receiver declares what it reacts to, and its match is
the gate (see [cascade](../concepts/cascade.md)).

The cleanest spelling: when a downstream node reads an upstream
attribute through `{{nodes.<upstream>.attribute.<field>}}`, rimsky
*auto-subscribes* it to that attribute's
[`changed` signal](../concepts/signal.md). Read access and reactive
coupling come from the same substitution directive — there are no orphan
reads. The mechanism is invalidate-then-pull: the upstream transition does
not carry a value along the edge; it invalidates the downstream, which
then pulls the latest persisted value at dispatch.

Primitives: **node-subscription** (the explicit + auto-subscribed edge),
**signal** (`attribute/<key>/changed`, `terminal/success`), **cascade**
(the downstream walk), **attribute** (the value that flows by pull).

## Template

Needs a rimsky deployment with the `http-node` executor. Stand rimsky up
from the published images (see the [operator guide](../operator-guide.md)).

Save the template as `recompute.yml`. `classify` reads `fetch`'s output,
which auto-subscribes it to `fetch`'s `summary` attribute:

```yaml
name: reactive-recompute
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: fetch
    executor: http-node
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
          summary:
            type: object
            additionalProperties: true

  - type: classify
    executor: http-node
    # Reading fetch.summary auto-subscribes classify to
    # fetch's `attribute/summary/changed` signal; the explicit
    # entry makes the coupling obvious to a reader. The `?` makes the
    # read lenient — when the upstream value is absent it resolves to
    # null instead of failing with template_resolution_failed, which is
    # the right spelling for an executor that may not write `summary`
    # (the stub returns {stub: true}, leaving `summary` unwritten).
    subscribes:
      - { node: fetch, type: terminal/success }
    attributes:
      schema:
        type: object
        properties:
          stub_probe:
            type: boolean
            default: true
          label:
            type: object
            source: "{{nodes.fetch.attribute.summary?}}"
```

Register, deploy, instantiate:

```sh
rimsky template register recompute.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=01H...
```

Both nodes settle `fresh` on the first frame — `fetch` runs, then
`classify` fires off the subscription and pulls `fetch`'s `summary`
attribute at dispatch (lenient `?`, so the absent value resolves to null):

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
# → [{"node_type":"fetch","state":"fresh"},
#    {"node_type":"classify","state":"fresh"}]
```

The wiring is the lesson: `{{nodes.fetch.attribute.summary}}`
auto-subscribes `classify` to `fetch`'s `attribute/summary/changed`
signal, and the explicit `terminal/success` entry makes the coupling
visible. After the first frame settles, both nodes are `fresh` but the
instance stays alive — instances are durable by default, so it keeps
running and remains invalidatable indefinitely (there is no auto-terminate
on drain). That durability is what makes the next step land on a live
instance. Now invalidate just the upstream and watch the recompute
propagate. Take the `fetch` node's `id` from the `/nodes` listing (each
entry carries an `id` field alongside `node_type`/`state`):

```sh
rimsky admin invalidate <fetch-node-id> --reason "upstream changed"
```

`fetch` re-runs in a new frame; its `terminal/success` matches
`classify`'s subscription, so `classify` recomputes too — and nothing else
does, because nothing else subscribed. Confirm the recompute settled by
re-reading the nodes — both are back to `fresh` (a node settles `fresh`
on a successful run; it is `stale` or `running` while a frame is in
flight):

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
# → [{"node_type":"fetch","state":"fresh"},
#    {"node_type":"classify","state":"fresh"}]
```

## Gotchas

**The stub advertises a permissive schema.** In stub mode
(`RIMSKY_EXECUTOR_STUB_MODE=1`) `http-node` advertises a permissive
attribute schema, so a node that declares an `attributes:` block dispatches
cleanly (no `executor_schema_unavailable`) and closes with a success.

**Use the lenient `?` read when the upstream may not write the value.** The
`?` in `{{nodes.fetch.attribute.summary?}}` makes the read resolve to null
when the upstream value is absent instead of failing with
`template_resolution_failed` — the right spelling for an executor that may
not write `summary` (the stub returns `{stub: true}`, leaving `summary`
unwritten).

## Without rimsky

By hand you would maintain a dependency graph and a dirty-tracking pass:
when A changes, figure out the transitive closure of things that read A,
mark them stale, and schedule them in topological order without
re-running unaffected nodes or double-running shared dependents. You would
also decide where the edge lives — usually in the sender ("after A, run
B"), which couples A to every consumer and breaks the moment a new
consumer appears. Rimsky inverts that: the edge lives on the receiver and
falls out of the read itself, so adding a consumer is a one-node change
and the recompute reaches exactly the affected nodes.
