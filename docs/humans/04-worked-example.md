# 4. A worked example, end to end

This walks one shape all the way through: a **held subgraph** that
stages work behind a claim and either commits it atomically or rolls it
all back. It is the pattern an agent needs when it has to touch a
system coherently — stage, verify, then promote-or-discard — and it is
where rimsky's [atomic-staging](../concepts/atomic-staging.md) and
[claim co-holdership](../concepts/claim-co-holdership.md) primitives
earn their keep.

The shapes below are illustrative — they show the primitives in play
and use current template vocabulary. For copy-pasteable, runnable
templates against the bundled docker-compose stack, use the
[runnable examples](../agents/examples/README.md): in particular
[holding-subgraph](../agents/examples/holding-subgraph.md) and
[atomic-staging](../agents/examples/atomic-staging.md).

## The shape

Three nodes against one claim:

1. **acquirer** opens a staging area on a
   [claim producer](../concepts/claim-producer.md) (a filesystem
   directory, a Postgres staging schema, an Iceberg branch — the
   producer decides what "staging" means).
2. **verifier** co-holds the acquirer's claim and runs checks against
   the staged contents *before* anything is promoted.
3. The held claim resolves once, at the end of the holding subgraph:
   all-success fires the producer's `Commit` (the atomic swap);
   any-failure fires `Abandon` (drop the staging).

The acquirer **acquires** the claim with `claims:`; the verifier
**co-holds** it with `holds:` plus a `from:` pointer back to the
acquirer. That `from:` is what extends the holding subgraph over both
runs, so [auto-terminal](../concepts/auto-terminal.md) waits for every
co-holder before deciding.

## The template

```yaml
name: staged-write
version: "1.0"
frame_resolution_mode: serial_queue
params_schema:
  type: object
  additionalProperties: true
nodes:
  - type: acquirer
    executor: stager
    claims:
      - { name: fs, selector: "snapshots/{{params.snapshot_id}}", intent: rw, alias: staging }
    attributes:
      schema:
        type: object
        properties:
          staging_path:
            type: string
            source: "{{claim.staging.address}}"

  - type: verifier
    executor: checker
    subscribes:
      - { node: acquirer, type: terminal/success }
    holds:
      - { from: acquirer, claim: staging, alias: staging }
    attributes:
      schema:
        type: object
        properties:
          checked_path:
            type: string
            source: "{{claim.staging.address}}"
```

Two things to notice. First, the verifier reads the *same* staging
location the acquirer opened — via `{{claim.staging.address}}` on the
co-held claim — so it inspects exactly what was staged, not a fresh
acquisition that might see a different snapshot. Second, the verifier's
`subscribes:` entry uses a canonical [signal](../concepts/signal.md)
type-path (`terminal/success`), which is the
[subscriber-side gate](../concepts/node-subscription.md): the verifier
fires because *it* declared the match, not because the acquirer pushed
to it.

## What happens at runtime

1. You register the template (content-addressed — you get a
   `sha256-...` [hash](../concepts/template.md)), deploy it, and create
   an [instance](../concepts/instance.md) with params.
2. The scheduler dispatches **acquirer**. Its claim opens; the producer
   returns a staging address, which lands in the acquirer's attributes.
   The acquirer's executor writes its work product into the staging
   area.
3. Acquirer settles with `terminal/success`. The
   [cascade](../concepts/cascade.md) walks the subscription edge and
   marks **verifier** stale within this [frame](../concepts/frame.md).
4. The scheduler dispatches **verifier**, which co-holds the still-open
   claim and runs its checks against the staged contents. The claim
   stays open the whole time because a co-holder exists.
5. When both runs are non-active, [auto-terminal](../concepts/auto-terminal.md)
   computes the aggregate outcome over the holding subgraph: both
   succeeded → `Commit` (the producer's atomic swap fires exactly once);
   either failed → `Abandon` (staging dropped). The
   [claim handle](../concepts/claim-handle.md) is then released.

## Observing it

While the verifier runs, the acquirer's claim is held. After both nodes
settle, the held claim resolves and the handle is deleted. You watch
node states and held-claim resolution through the
[control-api](../concepts/control-api.md) — the same verbs are
available as MCP tools at `POST /mcp`, so an LLM-driven operator
observes it identically. The runnable
[holding-subgraph example](../agents/examples/holding-subgraph.md) shows
the exact `curl` calls and expected JSON.

## Why this is the point

A DAG scheduler where each step's effects are independently persistent
would leave a half-written staging area behind if the verifier failed.
Here, the producer's `Commit` and `Abandon` verbs are the only ways
state becomes visible or gets cleaned up, and exactly one of them fires,
once, decided by the aggregate outcome. All-or-nothing is the default
mode — not something you assemble out of compensating steps. To grow
this into fan-out over many partitions, see
[fan-out](../concepts/fan-out.md); to make the staged result durable and
addressable afterward, see [assets](../concepts/asset.md) and
[lineage](../concepts/lineage.md).

Next: [where to go next](05-where-to-go-next.md).
