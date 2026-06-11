# Call a reusable sub-graph like a function

## Problem

A multi-step routine — fetch, validate, transform — recurs in several
places in a graph, or you want to package an internal DAG behind one node
so the outer graph reads as a single step. You want to invoke it the way
you call a function: one site names it, the routine runs its own steps, and
one result flows back. Duplicating the steps inline at each call site is
the thing to avoid.

## Rimsky shape

A [sub-graph](../concepts/sub-graph.md) is a named graph with declared
`entry:` and `exit:` nodes; a node in another graph invokes it by carrying
`delegate: <graph-name>` instead of `executor:`. A template declares its
graphs under a top-level `graphs:` block: exactly one graph named `main`
(the top-level graph) plus zero or more named sub-graphs.

Identity is asymmetric ([delegation](../concepts/delegation.md)):

- **The entry node is absorbed into the calling node.** At
  canonicalization the calling node's persisted row inherits the entry
  node's `executor` (and the entry's internal `stores:` / `holds:` /
  `attributes:`) merged with what the calling node declared. The entry
  node still gets its own `rimsky_nodes` row at provisioning, but it
  never dispatches as a standalone child — for dispatch it *is* the
  calling node, same run (the internal-cascade dispatch set filters the
  entry out; the calling node's executor invocation does the entry's
  work). This is why the calling node carries `delegate:` and never
  `executor:`: the executor comes from the absorbed entry.
- **The exit node is not absorbed.** It keeps its own row, runs inside a
  sub-graph [run-scope](../concepts/run-scope.md), and at its terminal the
  *carry-rule* copies its writeback onto the calling node's run in the same
  transaction. That is the function's "return value" — the exit's
  attributes surface as the calling node's attributes.

So from outside, `delegate:` reads as a function call (entry IS the caller,
exit's writeback IS the return); inside, the sub-graph's nodes structure
their own DAG, chained by [subscription](../concepts/node-subscription.md)
the same as any graph.

Primitives: **sub-graph** (`graphs:` + `entry:` / `exit:`), **delegation**
(`delegate:` — entry absorption + exit carry-rule), **node-subscription**
(internal chaining), **run-scope** (the per-invocation sub-graph scope).

## Template

Needs a rimsky deployment with the `http-node` executor (stub mode,
`RIMSKY_EXECUTOR_STUB_MODE=1`). Stand rimsky up from the published images
(see the [operator guide](../operator-guide.md)). Sub-graphs are a
canonicalization feature — no special service is required; any executor
works.

Save the template as `subgraph.yml`. The `main` graph's `run-pipeline`
node delegates to the `pipeline` sub-graph, which runs `prepare` →
`process` → `finalize` (a three-step internal pipeline). Internal nodes
reference each other in `subscribes:` by their `type` value:

```yaml
name: subgraph-demo
version: "1.0"
frame_resolution_mode: serial_queue
graphs:
  - name: main
    nodes:
      - type: run-pipeline
        # delegate: replaces executor: — the entry node's executor is
        # absorbed onto this node at canonicalization. Declaring both
        # delegate: and executor: on one node is rejected (mutually
        # exclusive): "delegate and executor are mutually exclusive".
        delegate: pipeline

  - name: pipeline
    entry: prepare
    exit: finalize
    nodes:
      - type: prepare
        executor: http-node
        attributes:
          schema:
            type: object
            properties:
              # stub_probe short-circuits the bundled http-node stub before
              # its transport-config check; a schema `default:` flows into
              # the dispatch bag verbatim (it is never substituted).
              stub_probe:
                type: boolean
                default: true
      - type: process
        executor: http-node
        # Intermediate internal node: fires after prepare settles, feeds
        # finalize. It keeps its own rimsky_nodes row (only the entry is
        # absorbed). An internal node may reference only other nodes in
        # the same sub-graph (or the entry alias) — a reference to an
        # outer-graph node is rejected (subgraph_internal_references_outer).
        subscribes:
          - { node: prepare, type: terminal/success }
        attributes:
          schema:
            type: object
            properties:
              stub_probe:
                type: boolean
                default: true
      - type: finalize
        executor: http-node
        # Internal chaining: finalize fires after process settles. As the
        # exit node it is not absorbed — it keeps its own row, and its
        # writeback carries up to run-pipeline at its terminal.
        subscribes:
          - { node: process, type: terminal/success }
        attributes:
          schema:
            type: object
            properties:
              stub_probe:
                type: boolean
                default: true
              result:
                type: string
                default: "done"
```

The `main` graph has exactly one node, the delegating caller; the
`pipeline` sub-graph declares its `entry:` (`prepare`, absorbed onto
`run-pipeline`), one intermediate node (`process`), and its `exit:`
(`finalize`, whose writeback carries up). `entry` and `exit` are distinct,
and every internal node is reachable from `entry` and feeds `exit`.

Register, deploy, instantiate:

```sh
rimsky template register subgraph.yml
# → template_hash=sha256-...
rimsky template deploy sha256-...
rimsky instance create sha256-...
# → instance_id=6b1f0c9a-4e2d-4f7b-9a3c-d5e8f1a2b3c4
```

At instance creation `run-pipeline` dispatches as the absorbed `prepare`
entry (it has no upstream, so it is a root). When it settles success, the
sub-graph's internal cascade fires `process`, then `finalize` after
`process` settles; at `finalize`'s terminal the carry-rule copies its
`result` writeback onto `run-pipeline`'s run. List the nodes — every
declared node has its own `rimsky_nodes` row, including the absorbed entry
(`flatten` keeps the entry in the provisioned node set; only the internal-
cascade *dispatch* set filters it out so it never runs as a standalone
child). The absorbed entry's row exists but never dispatches on its own —
its work is the calling node's executor invocation:

```sh
curl -s http://localhost:8080/v1/instances/<instance_id>/nodes \
  | jq '[.nodes[] | {node_type, state}]'
# → [{"node_type":"run-pipeline","state":"fresh"},  # caller; dispatches (absorbs prepare)
#    {"node_type":"prepare","state":"fresh"},       # entry row; never dispatches standalone
#    {"node_type":"process","state":"fresh"},        # internal node; dispatches as a child
#    {"node_type":"finalize","state":"fresh"}]       # exit; dispatches, writeback carries up
```

`run-pipeline` carries the exit's `result` writeback, so a downstream node
in `main` could read it through `{{nodes.run-pipeline.attribute.result}}`
exactly as if `run-pipeline` had run a plain executor.

## Gotchas

- **`graphs:` and the legacy flat `nodes:` are mutually exclusive.** A
  template using `graphs:` must put *every* node inside a graph (including
  the single `main` graph). Declaring a top-level `nodes:` list *and* a
  non-empty `graphs:` is rejected at canonicalization. The other recipes in
  this cookbook use the flat `nodes:` form; a sub-graph template cannot.
- **A sub-graph must declare both `entry:` and `exit:`, and they must
  differ.** A missing `entry:` → `subgraph_missing_entry`; a missing
  `exit:` → `subgraph_missing_exit` (they are two distinct classes, not one
  combined check); `entry == exit` → `subgraph_entry_equals_exit`. Every
  internal node must be reachable from `entry` and feed `exit`
  (`subgraph_disconnected_internal_node`).
- **Recursion is rejected.** A sub-graph that delegates to itself, directly
  or through a cycle, is rejected at registration as
  `subgraph_recursion_unsupported`. Sub-graphs nest (an internal node may
  `delegate:` to a *different* sub-graph) but may not recurse. (Self-*loops*
  are a different shape and are first-class — see
  [the loop recipe](convergence-loop.md) — but they are a node subscribing
  to its own signal, not a sub-graph invoking itself.)
- **The exit's writeback is the return value.** Downstream consumers read
  the sub-graph's result off the *calling* node
  (`{{nodes.<caller>.attribute.<field>}}`), not off the exit node — the
  carry-rule lands the exit's writeback on the caller's run. The exit's own
  attribute row stays empty.
- **The stub advertises a permissive schema.** In stub mode
  (`RIMSKY_EXECUTOR_STUB_MODE=1`) `http-node` advertises a permissive
  attribute schema, so each internal node's `attributes:` block dispatches
  cleanly (no `executor_schema_unavailable`) and closes with a success.

## Without rimsky

By hand you would extract the routine into a function or a service call and
thread its inputs and outputs through every call site yourself — plus the
bookkeeping to make each internal step independently observable, resumable
after a crash, and aggregated into one outcome at the boundary. Inlining
the steps at each site instead duplicates them and drifts. Rimsky makes the
routine a first-class graph: `delegate:` is the call, the entry absorbs into
the caller so there is no wrapper node, the exit's writeback is the return
value via a uniform carry-rule, and each internal step is its own audited,
claim-and-lock-aware dispatch — composition without a second runtime.
