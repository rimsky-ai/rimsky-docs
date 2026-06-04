# Hand a claim from one node to the next

## The problem

A chain of nodes operates on the *same* resource — the same file, the same
snapshot, the same staged region — which must stay locked across the whole
chain with all-or-nothing effects: if any node fails, none of the changes
commit. Exclusive access spans multiple steps, not just one.

## The rimsky shape

One node acquires a [claim](../concepts/claim.md) on a
[claim producer](../concepts/claim-producer.md); downstream nodes co-hold
the *same* claim handle via the `holds:` directive
([claim co-holdership](../concepts/claim-co-holdership.md)). The claim's
holding subgraph extends to every co-holder, and
[auto-terminal](../concepts/auto-terminal.md) resolution fires the
producer's commit or abandon verb exactly once, at the *end* of the
chain, decided by the aggregate outcome of all holders — all-success
commits, any-failure abandons. This is rimsky's atomic-by-default posture
(see [claim co-holdership](../concepts/claim-co-holdership.md)): the chain
is a transaction whose boundary is the holding subgraph.

`holds:` is the **sole** co-holdership directive. A node co-holding via
`holds:` may read the co-held address through `{{claim.<alias>.address}}`,
and the registration validator accepts that read (see Gotchas).

The downstream nodes are coupled two ways, deliberately separated:
[subscription](../concepts/node-subscription.md) decides *when* they fire
(after the acquirer settles), and `holds:` decides *what claim* they
co-hold. The address resolves into each holder through
`{{claim.<alias>.address}}`, so every node in the chain operates on the
same producer-managed region.

Primitives: **claim producer** (the filesystem `content` store),
**claim** + **claim-handle** (the shared lock), **claim co-holdership**
(`holds:`), **auto-terminal** (the end-of-chain commit/abandon),
**node-subscription** (chain ordering).

## Walkthrough

Needs a rimsky deployment whose `content` filesystem producer
(grpc://store-filesystem:9100) brokers concrete-path claims. Stand rimsky
up from the published images (see the
[operator guide](../operator-guide.md)).

Save the template as `handoff.yml`. `acquire` opens the claim; `process`
co-holds it and runs after `acquire` settles:

```yaml
name: claim-handoff
version: "1.0"
frame_resolution_mode: serial_queue
params_schema:
  type: object
  additionalProperties: true
nodes:
  - type: acquire
    executor: http-node
    stores:
      - { name: content, selector: "runs/{{params.run_id}}/output", intent: rw, alias: region }
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
          address:
            type: string
            source: "{{claim.region.address}}"

  - type: process
    executor: http-node
    subscribes:
      - { node: acquire, type: terminal/success }
    holds:
      region: { from: acquire }
    attributes:
      schema:
        type: object
        properties:
          stub_probe:
            type: boolean
            default: true
          working_on:
            type: string
            source: "{{claim.region.address}}"
```

The `content` producer is concrete-paths only, so the selector resolves to
a fixed path under the shared content root. `process` co-holds `acquire`'s
`region` claim — the same handle, not a second open — and both nodes see
the same `address`. The `holds:` outer key (`region`) is the local alias
the address binds under in `process`'s leaf request, and it MUST match the
alias `acquire` declared on its `stores:` entry (`alias: region`) — the
validator looks the claim up on the upstream by that name. Each `holds:`
entry value is `{ from: <upstream-node-type> }`, with an optional `as:`
that overrides the local alias (defaulting to the outer key) — per the
`HoldsBinding` shape in
[`reference/template-schema.md`](../reference/template-schema.md#holdsbinding).

Register, deploy, instantiate:

```sh
rimsky template register handoff.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-... --params '{"run_id":"r-1"}'
# → instance_id=01H...
```

While `process` runs, the claim handle is held by the subgraph. List its
holders:

```sh
curl -s http://localhost:8080/lock-holders/<claim_handle_id>/claim-holders | jq
```

Both dispatches close with success on the stub, so both nodes settle
`fresh`; at that point the aggregate outcome is all-success, so the held
claim auto-resolves to `Commit`. Terminal does NOT delete the claim
handle — it *promotes* it: every terminal flips
`rimsky_claim_handles.state` and preserves the row past terminal for
forensics, so an all-success commit promotes the handle to the
`committed` state (a later retention sweep reaps non-durable terminal
rows). See the `ClaimHandleState` enum in
[`reference/template-schema.md`](../reference/template-schema.md#claimhandlestate)
(`committed`: "row preserved past terminal"):

```sh
curl -s http://localhost:8080/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
# → [{"node_type":"acquire","state":"fresh"},
#    {"node_type":"process","state":"fresh"}]

# The handle row is preserved, promoted to state=committed (not deleted).
# Its holder rows are likewise preserved, transitioned to state=completed
# with completed_at set — ListByClaimHandleID returns ALL rows regardless
# of state, so the 2-party handoff still lists two holders (200, two rows).
curl -s http://localhost:8080/lock-holders/<claim_handle_id>/claim-holders \
  | jq '.holders | length'
# → 2
curl -s http://localhost:8080/lock-holders/<claim_handle_id>/claim-holders \
  | jq '[.holders[] | .state]'
# → ["completed","completed"]
```

## Gotchas

- **`holds:` + a co-held alias read validates.** The registration
  validator derives the claim aliases available to `{{claim.<alias>}}`
  reads from two sources: `stores:` (claims this node acquires, like
  `acquire`'s `region`) and `holds:` (claims this node co-holds, like
  `process`'s `region`). A node that co-holds via `holds:` and reads
  `{{claim.<alias>.address}}` — exactly the `process` node above — is
  accepted at `template register`. Undeclared aliases — neither acquired
  nor co-held — are rejected. `holds:` is the sole co-holdership directive
  (the legacy singular `inherits:` form is gone), per
  [claim co-holdership](../concepts/claim-co-holdership.md).
- **The handoff instance is durable — it does not clean itself up.** Once
  the chain commits, both nodes settle `fresh` and the instance keeps
  running (instances are durable by default; there is no auto-terminate on
  drain). To tear it down, force-terminate then delete: `rimsky instance
  kill <instance_id> --force` (marks it terminal, abandoning any in-flight
  run) followed by `rimsky instance delete <instance_id>` (frees the row).
  For a one-shot "acquire, process, commit, exit" instance that terminates
  itself after the chain settles, set `terminate_after_run: true` on the
  create request (see the [README](README.md#instances-are-durable-by-default)
  note — the flag is body-only, so create via `POST /instances`, not the
  CLI).
- **Both nodes must run the `http-node` executor in stub mode**
  (`RIMSKY_EXECUTOR_STUB_MODE=1`) with a permissive attribute schema, so
  each dispatch passes the schema gate and closes with a success — letting
  the handoff actually commit. A schema `default:` flows into the dispatch
  bag verbatim (it is never substituted).
- **Seeing the abandon path.** The all-success chain above takes the
  `Commit` branch. To watch any-failure → `Abandon` (the whole chain rolls
  back, nothing commits), drive one node to a failure outcome — e.g. point
  it at an executor that returns an error, or give the `acquire` node's
  http-node a real `url` that returns an unexpected status instead of the
  `stub_probe` short-circuit. As soon as any holder fails, auto-terminal
  resolves the held claim with the aggregate-failure outcome and the
  producer's `Abandon` verb runs (not `Commit`). The
  [holding-subgraph example](../agents/examples/holding-subgraph.md) walks
  the resolution mechanics in more depth.

## Without rimsky

By hand you would open the resource in the first step, thread its
handle/transaction through every subsequent step, and write the
commit-on-all-success / rollback-on-any-failure logic yourself — including
the awkward cases where a step crashes holding the lock, or a later step
fails and you must unwind the earlier steps' effects. Rimsky makes the
holding subgraph the transaction boundary: the producer's commit/abandon
fires exactly once at the end, the decision is the aggregate of every
holder, and a crashed holder releases through the claim-handle ledger
rather than leaking the lock.
