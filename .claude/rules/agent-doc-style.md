# Agent-doc style

How the **published doc corpus** (`rimsky/skills/rimsky/docs/`) is written. It is
read and maintained by coding agents; optimize for an agent to **consume** and to
**maintain**. Human readability is secondary — drier and denser is fine; cryptic
is not. This is the cold-read principle (see `cold-read/`) applied to prose: a doc
must be usable by an agent with **no prior context**.

Scope: the published corpus only. Not the Go code, not the skills, not commit
messages, and not agent↔human chat (that is `citation-grammar.md`).

## Required moves

1. **Assertion-first.** Lead with the fact; cut motivation, analogy, and
   "before we get into…" ramps.
   - Bad: "To understand executors, it helps to first picture a worker that…"
   - Good: "An executor runs one node's work. It implements `Executor.Execute`."
2. **Tables / definition-lists for enumerables.** Fields, options, enum values,
   error classes, ports → a table with stable columns. A reader pattern-matches a
   row; a maintainer diffs a cell. Reserve prose for reasoning.
3. **State boundaries — what it is AND what it is not.** Every concept/service
   names what it owns and, explicitly, what it does **not** own. This kills the
   most expensive agent error: assuming a thing does something adjacent. Say the
   non-goals and the "does NOT".
4. **Reasoning stays prose — protect it.** The *why*, the *when-A-vs-B*, the
   *gotcha-and-its-cause* is the highest-value content and the part not in an
   agent's training. Never compress it into a table or cut it for brevity.
   Tables for facts; tight prose for reasoning; nothing for motivation.
5. **One template per doc type** (below). Same slots, same order, every time.
6. **Canonical terminology, verbatim.** One name per thing, repeated exactly — no
   synonyms for variety. Match the source's vocabulary.
7. **Self-contained chunks.** Each section opens with a one-clause orientation and
   links rather than assuming the reader saw the previous section. Agents retrieve
   by chunk, not front-to-back.
8. **Source-anchored — by the surface's own form.** Every doc's provenance must be
   traceable for re-verification, but the form differs by surface:
   - **Derived guides + generated reference** (`protocols/`, `reference/`): an
     inline `@source:` anchor to the rimsky file/symbol they reconcile against.
   - **Concept catalog** (`concepts/`): deliberately *self-contained* — no inline
     code citations (it mirrors rimsky's design concepts wholesale; that contract
     is recorded in the build-docs ownership table, not per-file). Do not add
     inline `@source:` here.
   The source-of-truth is named once per surface — inline for per-file
   projections, in the ownership table for wholesale mirrors.
9. **Exact, copyable symbols.** A field is its exact key; an RPC its exact
   signature; an error its exact class string. Never paraphrase a name.

## Templates

- **Concept** (`concepts/`): `What it is` · `Purpose` · `Boundaries` (owns / does
  NOT own / adjacent) · `Invariants`. Already the house shape — keep it.
- **Protocol guide** (`protocols/*.md`): one-line `What you implement` + the RPC
  table · `Boundaries` (service owns vs. rimsky owns) · per-method spec (request /
  response fields as tables, semantics as prose) · reasoning sections (async,
  resume, error handling — prose) · `Conformance` · `Reference impls` · `See also`.
  Do **not** restate field-level wire shapes (proto field numbers, JSON encoding)
  — link the generated `reference/`; the HTTP+JSON encoding convention lives once
  in `protocols/README`.
- **Cookbook** (`cookbook/*.md`): `Problem` · `Rimsky shape` (primitives + why) ·
  `Template` (copyable) · `Gotchas` · `Without rimsky`.
- **Catalog** (`services/`, `images/`, `reference/`): tables first; prose only for
  cross-cutting rules.
- **Pattern** (`patterns/`): assertion-first — lead with the real surface, not the
  motivation. If the pattern is aspirational or only partially supported, say so
  up front (a status note). Then: what rimsky provides, what it does **not**, and
  how to build it. Not a magazine article.

## Acceptance test — the cold read

A doc is done when a **fresh agent with no prior context** can perform a
representative task using only that doc (plus the files it links), with no
guessing. Validate ports by dispatching such an agent: give it the doc and a task
("implement / answer X"), and have it report friction, gaps, and ambiguity. The
doc's defects are whatever the cold reader had to guess or look elsewhere for.
