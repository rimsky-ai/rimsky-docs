---
sketch: last-mile-docs
date: 2026-06-10
---

# Last mile to "agent reads this, becomes a rimsky pro"

Kickoff sketch for a "last mile" brainstorm. Context: a fresh outside
review (2026-06-10) of the corpus against its own ambition found that
the corpus largely cashes the check already — the generated reference
layer is complete and genuinely generated (66 REST routes verified
against rimsky-core's action table, the full template schema, the
proto-derived wire references), the substitution grammar is fully
enumerated in `concepts/attribute.md`, the error catalog is 27 real
codes with resolutions, and the reconcile pipeline (7 lint gates,
`reconciledAgainst: v0.8.0`) is strong anti-rot machinery. The
remaining gaps are few, specific, and already known — they live in
`rimsky-docs-feedback.md` and `.build-docs/report.md`. This sketch
collects them into one bounded campaign so a spec/plan can close them
as a unit.

The unifying test for "done": three canonical agent journeys must
close **from the corpus alone**, with no recourse to rimsky-core
source — (a) write a first template with one custom executor, (b)
stand up local dev from zero, (c) debug a node stuck stale. Today (a)
closes with friction, (b) and (c) close only partially.

## Item 1 — Runnable stub executor (feedback TL;DR #2)

`docs/executors/stub/` is a 29-line README with no code. Meanwhile the
feedback file documents precisely where an implementer's time went:
the executor protocol is streaming and stateful, and reconstructing
the handshake + event sequencing from prose and symbol references is
the expensive part. Fill the slot: a minimal CI-compiled `main.go` +
pinned `go.mod` that registers, receives a dispatch, emits a named
event, and returns a terminal success — the smallest artifact that
makes the streaming lifecycle copyable rather than reconstructable.
It should ride the same vendoring/CI machinery as `docs/examples/`
(compiled at reconcile time, pinned to the reconciled release) so it
cannot rot independently.

## Item 2 — Version banners on every generated reference page
(feedback TL;DR #1)

The prose protocol guides carry a version banner; the generated pages
(`reference/template-schema.md`, `rest-api.md`, `cli.md`,
`protocols/reference/*`) do not. An agent reading a generated page has
no signal which rimsky release it reflects, while the example
`go.mod`s pin `v0.8.0` — skew between corpus snapshots is silent.
Mechanical fix: the generators emit a one-line banner sourced from
`plugin.json`'s `reconciledAgainst`, and the reference-parity lint
gains a check that the banner matches. Smallest item here; pure
generator change.

## Item 3 — Zero-to-deployed-cluster walkthrough
(`cookbook-deploy-chain-gap`, build report)

Per-service worked configs exist under `docs/reference/config/`, and
the operator guide covers every knob — but no surface carries a
copy-pasteable end-to-end "bring up control-api + store-postgres +
supervisor + one http-node template from nothing." A cold reader can
configure every piece and still not reach a running deployment without
consulting upstream. Proposal: one walkthrough (cookbook recipe or
operator-guide section — brainstorm decides where it lives) that goes
from empty host to a node driven to terminal, built on the published
images and the existing worked configs, with each step's expected
observable output stated ("you should now see X in `rimsky instance
status`"). The all-in-one image is the natural spine; a second variant
showing the three-role split would also retire the recurring
entrypoint/migrate-ownership confusion documented in rimsky-core's
gotchas.

## Item 4 — A worked debug session

The diagnostics surface is documented (the admin diagnostics routes,
the events endpoint, the error catalog, `patterns/operational-health.md`)
but nothing walks a concrete failure: "node X is stuck stale — here is
the command sequence, here is what each output means, here is the
decision tree." Proposal: one (or two) narrated debug sessions as a
pattern or cookbook entry — strongest candidates are (a) stuck-stale
node (wait-set / frame diagnosis → the wait-sets diagnostics route,
with an example payload, which the corpus currently lacks) and (b)
claim never released (orphan-reaper / heartbeat diagnosis). The error
catalog gives the per-code leaf answers; this item supplies the root
of the tree — how an agent gets from symptom to the right catalog
page.

## Explicit non-goals

- No new doc surfaces beyond filling the four gaps; the corpus shape
  (skill router → concepts / protocols / reference / cookbook /
  patterns / errors) is working and should not be reorganized.
- Containerization/orchestration manifests (docker-compose, K8s)
  remain out of scope per the existing boundary — Item 3 deploys from
  the published images by hand, it does not ship infra manifests.
- Core-side ergonomics (opaque HTTP 500s from stores, validation
  errors that don't name the active ref-validation mode) are *not*
  docs work — they are sketched in rimsky-core
  (`sketches/2026-06-10-last-mile-stability.md` there, Phase C). The
  error catalog should pick up any new error shapes that work
  produces at the next reconcile, but this campaign does not wait on
  it.

## Open questions for the brainstorm

- Item 1: is the stub executor Go-only, or does the TypeScript path
  (`@rimsky-ai/protocols` npm package) deserve a parallel stub? The
  feedback came from a Go implementer; the claude-agent executor
  suggests TS implementers are a real audience.
- Item 3: where does the walkthrough live — cookbook (problem-shaped,
  agent-facing) or operator guide (deploy-shaped)? And does it cover
  the all-in-one image only, or also the three-role split?
- Item 4: pattern doc or cookbook recipe? And should the debug
  sessions be lint-checkable in any way (e.g. symbol-existence over
  the routes and CLI verbs they cite), or are they pure prose?
- Sequencing: Items 1–2 are mechanical and could ship in a point
  reconcile; Items 3–4 are authored prose needing review rounds. One
  spec or two?
- Does the next reconcile against a post-v0.8.0 core release fold
  into this campaign, or stay a separate routine `build-docs` run?
