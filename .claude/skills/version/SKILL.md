---
name: version
description: "ONLY activated by the explicit /version slash command. Never auto-triggered by conversation content. Project-local maintenance skill for rimsky-docs: report the two versions this repo carries — rimsky-docs' own plugin release version, and the rimsky release the corpus is reconciled against."
---

# /version — the two versions rimsky-docs carries

rimsky-docs carries **two independent versions**, and a consumer needs both:

- **Plugin version** (`version` in the manifest) — rimsky-docs' *own* release
  semver. `/release` bumps it whenever the docs themselves change — a new
  cookbook recipe, a corrected guide, a tooling fix — **independent of any
  rimsky release**. This is the number a marketplace compares to decide whether
  an update is available, so a doc-only improvement still ships.
- **Reconciled against** (`reconciledAgainst` in the manifest) — the rimsky
  release the corpus *documents*. `/build-docs` stamps it when it reconciles the
  corpus against a rimsky release. A doc-only improvement leaves it unchanged.

They move on different clocks: the plugin version advances every release; the
reconciled-against version advances only when a `/build-docs` run tracks a new
rimsky release. Pinning one to the other (an earlier mistake) is wrong — that is
exactly why they are two fields.

Both live in the plugin manifest, `rimsky/.claude-plugin/plugin.json`.

This is a **read-only** maintenance tool for the repo author. It lives in
`.claude/skills/` (not the distributed `skills/`), reads the manifest, and
prints. It makes **no** changes and runs **no** git commands.

## Procedure

1. Read `rimsky/.claude-plugin/plugin.json`. Take `version` and
   `reconciledAgainst`.
2. Print both, labeled, with the one-line meaning of each:

   ```
   rimsky-docs plugin version : <version>            (this repo's own release; bumped by /release)
   reconciled against rimsky  : <reconciledAgainst>  (the rimsky release the corpus documents; stamped by /build-docs)
   ```

3. If `reconciledAgainst` is absent or empty, say so plainly: no `/build-docs`
   run has stamped a rimsky release yet (or the manifest predates the field). Do
   **not** guess a value.

Report only what the manifest says. Do not read git tags, do not infer drift,
and do not edit anything. If a value is not what the user expects, that is their
cue to investigate (a missing `/release` bump, or a `/build-docs` run that did
not stamp `reconciledAgainst`) — not for this skill to "fix."
