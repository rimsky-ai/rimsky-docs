# Cold Read for Documentation

The documentation surface of Cold Read. The [manifesto](cold-read-manifesto.md)
makes the case once for any artifact an agent reads cold; the
[code style guide](cold-read-style-guide.md) is the rules for source. This is the
rules for **documentation** — prose an agent consumes to act *elsewhere* (build a
template, implement a protocol, operate a system), with no prior context and a
context budget spent per task.

A doc is **done** when a fresh agent with no prior context can perform a
representative task from it — plus the files it links — without guessing.

## What docs change vs. code

The shared philosophy is identical (cold read, no memory, context-per-task, the
inverted cost model). Two structural differences drive the doc-specific rules:

1. **A doc describes a source of truth; code *is* its own truth.** So docs have an
   **accuracy lifecycle** — a claim can be wrong, and drifts as the source moves.
   Code cold-read has no analogue; a function is not "wrong about" an external
   system. → *Accuracy* below is first-class.
2. **A doc is consumed to act elsewhere; code is modified in place.** So the unit
   of value is "can an agent do the task from this," and the acceptance test is a
   **cold read** by a no-context agent, not a code review. → *Validation* below is
   the gate.

## Principles

1. **Assertion-first.** Lead with the fact; cut motivation, analogy, and "before
   we get into…" ramps. *Bad:* "To understand X, picture a…" *Good:* "X is Y; it
   does Z." The reader extracts more from fewer tokens.
2. **Self-contained chunks.** Each section opens with a one-clause orientation and
   links rather than assuming the previous section was read — agents retrieve by
   chunk, not front-to-back. (The doc analogue of code's locality-of-behavior.)
3. **Tables for facts, prose for reasoning.** Enumerables — fields, options, enum
   values, error classes, endpoints — go in tables (a reader pattern-matches a
   row; a maintainer diffs a cell). The *why*, the *when-A-vs-B*, the
   *gotcha-and-its-cause* stay tight prose: that reasoning is the highest-value
   content and the part not in the agent's training — never compress it into a
   table or cut it for length. Nothing for motivation.
4. **Boundaries — what it is AND what it is not.** Every concept/component states
   what it owns and, explicitly, what it does **not**. This kills the most
   expensive agent error: assuming a thing does something adjacent. (The doc
   analogue of code's `@agent-contract` "does NOT handle…".)
5. **Canonical terms, exact symbols.** One name per thing, repeated verbatim — no
   synonyms for variety. A field is its exact key; an API its exact signature; an
   error its exact code. Never paraphrase a name. Repetition is a feature: it lets
   an agent — and a linter — match "same thing" across files.
6. **Source-anchored, by the surface's form.** Provenance must be traceable for
   re-verification. A derived guide or generated reference carries an inline
   `@source:` to the file/symbol it reconciles against; a wholesale mirror (e.g. a
   concept catalog copied near-verbatim from a design source) records that
   contract once in the maintenance tooling, not per-file. Name the
   source-of-truth once per surface.

## The layered corpus

Organize the corpus in layers, each cold-readable for one task, navigated by
links — the doc analogue of code's feature-isolation and shallow dependencies:

- **Entry / router** — a thin, always-available map: what the corpus is, the
  mental model in a screen, and *task → which file*. (For an installable skill
  this is `SKILL.md`; the cross-tool equivalent is an `llms.txt`.)
- **Guides** — dense behavioral prose: how to do a thing, its boundaries, the
  reasoning, the gotchas.
- **Reference** — mechanical, generated-from-source: exact fields, signatures,
  routes. Guides link here for shapes; they do not restate them.
- **Concepts** — one self-contained file per load-bearing noun, on a fixed
  template.

**One fact, one home; link, don't restate.** A fact duplicated across layers is a
fact that drifts and a maintenance multiplier. The reference owns wire shapes; the
concept owns a noun's semantics; the guide owns the how — each links the others.
(The doc analogue of tracked-duplication: prefer a link to a copy; where a copy is
unavoidable, anchor it.)

## Accuracy: shrink the pass-surface

A doc's hardest property is staying *true* to its source as the source moves. The
efficient model is not "re-read everything, repeatedly" — it is to shrink the
surface that needs a human/agent pass, cheapest ring first:

1. **Generate — accurate by construction.** Anything derivable mechanically from
   source (schemas, signatures, route tables, enums) is generated and never
   hand-verified; regenerate and it is correct. Push as much across this line as
   possible — every generated fact is one you never re-verify.
2. **Machine-checkable invariants.** Over the hand-written remainder, enforce
   deterministically (milliseconds, no pass): links resolve, cross-references
   resolve, generated surfaces match source (parity), **every exact symbol a doc
   names exists in source** (the #1 hallucination class), templates/frontmatter
   conform, the entry index is valid. (See *What a docs linter enforces*.)
3. **Cold-read triage.** A no-context cold read cannot confirm truth, but every
   "I had to guess" / "term used but not defined" / "this looks inconsistent" is a
   precise pointer at where the doc is incomplete *or wrong* — it focuses the
   expensive ring instead of re-verifying uniformly.
4. **Targeted reconciliation passes.** Only the residue rings 1–3 cannot reach —
   genuine prose judgments and behavioral claims against the running system. This
   is where a multi-pass review loop earns its cost, now over a fraction of the
   corpus rather than all of it.

The structured style is what makes rings 2–3 possible: atomic, source-anchored,
exact-symbol claims are checkable in isolation and triageable precisely. Flowing
prose can only be accuracy-checked by re-reading it — which is why, without the
style, accuracy reduces to brute-force passes.

## Validation: the cold read

A doc is accepted when a **fresh agent with no prior context** can do a
representative task from it (plus the files it links) without guessing.

To run one: dispatch a no-context agent; give it only the target doc and a
concrete task ("implement X", "answer Y", "configure Z"); have it report —

- **Extractable** — what came directly and unambiguously.
- **Guessed / missing** — what it had to infer, or could not find.
- **Ambiguous** — anything it could read two ways.
- **Sent elsewhere** — which link it had to follow to proceed.
- **Verdict** — could it act from this doc plus the files it links?

The doc's defects are whatever the cold reader had to guess or look elsewhere for.
Distinguish *by-design delegations* (field shapes → the generated reference) from
*real gaps* (a term used but never defined). The cold read tests **usability**,
not accuracy — pair it with the accuracy rings above.

## What a docs linter enforces

The machine-checkable ring, as invariant classes a project's linter implements.
Mechanical correctness only — word choice and usability are the cold read's job.

| Class | Checks |
| --- | --- |
| Link validity | Every relative link/image target resolves on disk. |
| Cross-reference resolution | Every in-corpus reference token (concept slugs, etc.) resolves to a real doc. |
| Source parity | Each generated surface byte-matches what regenerating from source would produce. |
| Symbol existence | Every exact symbol a doc names — type, field, enum value, route, error class, config key — exists in the source it is reconciled against. |
| Template / frontmatter conformance | Each doc-type carries its template's sections and required frontmatter keys. |
| Entry / index validity | The router and index files are well-formed and their links resolve. |

## Templates

- **Concept:** What it is · Purpose · Boundaries (owns / does NOT own / adjacent) ·
  Invariants.
- **Guide:** what you implement (+ an operation/RPC table) · Boundaries · per-op
  spec (request/response as tables, semantics as prose) · reasoning sections ·
  conformance · reference impls · see also.
- **Recipe / cookbook:** Problem · Shape (primitives + why) · Template (copyable) ·
  Gotchas · the "without it" baseline.
- **Catalog:** tables first; prose only for cross-cutting rules.
- **Pattern:** assertion-first — lead with the real surface; a status note if the
  pattern is aspirational; what's provided vs. **NOT**; how to build it. Not a
  magazine article.

## Quick reference

- Lead with the fact. No ramp.
- Table the enumerables; prose the reasoning; nothing for motivation.
- Say what it is **and** is not.
- One name per thing; exact symbols.
- One fact, one home; link, don't restate.
- Generate what you can; machine-check the rest; cold-read the residue; pass over
  only what's left.
- Done = a no-context agent does the task without guessing.
