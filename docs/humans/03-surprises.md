# 3. The surprises

If you are pattern-matching rimsky against a DAG scheduler, a workflow
engine, or an agent framework, a handful of things will not be where
you expect them. This page collects the ones worth knowing before you
commit a design to the platform.

## Loops are first-class — recursion is not

This is the surprise most readers do not expect, and the two halves
point in opposite directions, so hold them apart.

**A node can drive its own loop.** Self-subscription is first-class: a
node subscribes to *its own* terminal signal and re-fires. This is the
"drain my own queue" and "converge until done" idiom — exactly what an
agent needs when it iterates toward a goal (reason, act, observe,
decide whether to go again). There are two equally valid spellings,
and which you pick is an editorial choice, not a platform constraint:

- **`frame: next`** opens a fresh [frame](../concepts/frame.md) for the
  same node on every matching commit — one frame per iteration, with
  clean `frame.start` / `frame.end` markers you can observe per loop.
- **`frame: in`** keeps the iteration inside the current frame — one
  long-running frame; the supervisor picks up each new pending run as
  it lands.

The canonical form of the self-edge is a subscription back to the
node's own type on `terminal/success`, typically gated on
`when: payload.changed` so the loop stops when a run produces no change.
See [node-subscription](../concepts/node-subscription.md) for both
shapes. Because the loop is just a subscription, there is no special
"loop node" type — the [cascade](../concepts/cascade.md) already does
the right thing.

**Runaway loops are bounded, not banned.** Every dispatch tracks a
consecutive-retries-without-progress counter. When it exceeds the
effective cap (a per-node `max_retries_without_progress` or the
deployment default), the runtime forces a `retry_loop_no_progress`
error, which routes through your [error policy](../concepts/error-policy.md).
A converging loop refreshes progress every iteration and never trips
it; a wedged loop trips it and surfaces.

**Recursion is explicitly rejected.** A
[sub-graph](../concepts/sub-graph.md) that
[delegates](../concepts/delegation.md) to itself — directly or through
a cycle — is rejected at template registration as
`subgraph_recursion_unsupported`. Composition is one-way: a node
delegates *into* a sub-graph, the sub-graph cannot delegate back into
its own call chain. So the iterative shapes you want live at the
node-self-subscription level (loops), and the compositional shape
(delegation) is acyclic by construction. If you came expecting
recursive sub-graph calls, this is the line that will catch you.

## The platform never reads your bytes

Rimsky treats six byte streams as inert: attribute values, claim scope,
claim payload, blob content, named-event payload, and message payload. It
does not log them, normalize them, index them, attach them to traces,
or put them in error messages — it validates only against the schema
gates. [Inertness](../concepts/inertness.md) is not minimalism; it is
what keeps rimsky out of your domain. Three blessed invariants lock
this in source. The practical consequence: if you expected the
orchestrator to inspect a payload and route on its meaning, it will
not. Routing is the subscriber's CEL predicate over a *signal*, not the
platform reading your bytes.

## The subscriber decides, not the sender

In most DAG tools, an edge means "when A finishes, run B." In rimsky the
edge is owned by the receiver: B declares what signals it reacts to, and
**B's match is the gate**. A node that wants to act on every upstream
error subscribes broadly to `terminal/error/*`; a node that wants to
ignore errors simply does not subscribe to them. There is no sender-side
"fire downstream on failure" switch — propagation is subscriber-driven.
See [cascade](../concepts/cascade.md) and
[node-subscription](../concepts/node-subscription.md).

## No special node types for "smart" behavior

Patterns that look like they would need a bespoke node type are all
expressed through the ordinary executor and subscription surfaces:

- **Agent self-blocks** — an agent emits a structured failure and a
  downstream node routes on the failure class — is just a subscription
  on `terminal/error/<class>`.
- **Confidence-driven branching** — fire only the matching subscriber
  — is a [named event](../concepts/named-event.md) plus CEL `when:`
  predicates.
- **Deterministic transformation** runs inside an
  [executor](../concepts/executor.md) like any other work; a node with
  no executor declared is a *native node* whose value is whatever the
  upstream nodes wrote.

There is no special-cased "deterministic node" type because cascade,
substitution, retry policy, and claim semantics are already correct for
pure code.

## Atomic by default, not best-effort

A DAG scheduler where every step's effects are independently persistent
leaves half-applied state behind on failure. Rimsky's
[held subgraph](../concepts/claim-co-holdership.md) +
[atomic-staging](../concepts/atomic-staging.md) make all-or-nothing the
default: a chain of nodes co-holds one claim, and the producer's
commit/abandon verbs run exactly once at subgraph completion, decided
by the [aggregate outcome](../concepts/auto-terminal.md) of every
co-holder. Walked through end to end on [the next
page](04-worked-example.md).

## An LLM can operate the platform

The [control-api](../concepts/control-api.md) hosts a coextensive MCP
skin: every operator verb is also an MCP tool at `POST /mcp`. The same
[API-key](../concepts/api-key.md) auth, permission grammar, dry-run, and
audit apply whether the caller is a human with `curl` or an LLM-driven
supervisor. Rimsky expects to be operated by an agent, not only by a
person.

## What rimsky deliberately isn't

The platform stays useful by being explicit about what it is not.
Stream processing (windowing, watermarks, exactly-once stream
semantics), per-key state stores, streaming-batch unification,
cluster resource scheduling, a semantic layer or SQL transformation
language, in-flight workflow migration, bundled agent-pattern
libraries, and a SQL data platform are all out of scope by design — each
gets pushed to a consumer-side service or an adjacent system rather than
growing a rimsky primitive. The full list with rationale is in the
project [README](https://github.com/fallguyconsulting/rimsky/blob/main/README.md#5-what-rimsky-deliberately-isnt)
and the [ecosystem comparison](../comparison.md).

Next: [a worked example, end to end](04-worked-example.md).
