---
name: rimsky-version
description: >-
  Report the two versions the rimsky docs carry — the rimsky release the docs
  were built/reconciled against, and the rimsky-docs plugin's own version. Use
  when asked "what version of rimsky do these docs cover", "how current are the
  rimsky docs", "what's the rimsky-docs version", or to check whether the bundled
  corpus matches a given rimsky release. Read-only.
---

# rimsky-version — which rimsky the docs cover, and which docs

The rimsky docs carry **two independent version numbers**, and you usually need
both:

- **rimsky release reconciled against** (`reconciledAgainst` in the plugin
  manifest) — the rimsky release this corpus was built/reconciled against. This
  is the rimsky the docs describe; treat the corpus as accurate for that release.
- **rimsky-docs plugin version** (`version` in the plugin manifest) — the docs
  plugin's *own* release version. It advances every time the docs themselves are
  re-released (a corrected guide, a new recipe, a tooling fix), **independent of
  any rimsky release**.

They move on different clocks and are deliberately separate fields — neither is
pinned to the other.

## Procedure

1. Read the plugin manifest. It lives at the plugin root, two directories above
   this skill:

   `${CLAUDE_SKILL_DIR}/../../.claude-plugin/plugin.json`

   (If that variable is not expanded in your environment, the manifest is the
   `.claude-plugin/plugin.json` at the root of this installed `rimsky` plugin —
   the directory two levels up from this skill's own folder.)

2. Take the `version` and `reconciledAgainst` string fields.

3. Print both, labeled with their meaning:

   ```
   rimsky release reconciled against : <reconciledAgainst>   (the rimsky release these docs describe)
   rimsky-docs plugin version        : <version>             (the docs' own release version)
   ```

4. If `reconciledAgainst` is absent or empty, say so plainly — no build has
   stamped a rimsky release into this manifest — and do not guess a value.

Report only what the manifest says. Read nothing else, and change nothing.
