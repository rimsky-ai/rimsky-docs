---
name: rimsky-version
description: >-
  Report the two versions the rimsky docs carry — the rimsky release the docs
  were built/reconciled against, and the rimsky-docs plugin's own version. Use
  when asked "what version of rimsky do these docs cover", "how current are the
  rimsky docs", "what's the rimsky-docs version", or to check whether the bundled
  corpus matches a given rimsky release. Reports fixed values; reads nothing and
  changes nothing.
---

# rimsky-version — which rimsky the docs cover, and which docs

The rimsky docs carry **two independent version numbers**, and you usually need
both:

- **rimsky release reconciled against** — the rimsky release this corpus was
  built/reconciled against. This is the rimsky the docs describe; treat the
  corpus as accurate for that release.
- **rimsky-docs plugin version** — the docs plugin's *own* release version. It
  advances every time the docs themselves are re-released (a corrected guide, a
  new recipe, a tooling fix), **independent of any rimsky release**.

They move on different clocks and are deliberately separate numbers — neither is
pinned to the other.

## What to report

Print exactly these two lines, verbatim:

```
rimsky release reconciled against : v0.6.0   (the rimsky release these docs describe)
rimsky-docs plugin version        : 1.3.0    (the docs' own release version)
```

Report only these two values. **Read nothing** — not the plugin manifest, not
any file — and change nothing. The two values above are this skill's source of
truth; they are updated in place at each rimsky-docs release.

<!-- VERSION-LITERALS: the two values above are stamped by the /release skill at
     release time (plugin version) and reflect the rimsky release the corpus is
     reconciled against (reconciledAgainst). Keep this file's body the only place
     they live for the skill; do not reintroduce a manifest read. -->
