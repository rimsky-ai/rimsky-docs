---
name: rimsky-version
description: Echo the two rimsky-docs version literals — the rimsky release the corpus describes, and the plugin's own version. Use for "what rimsky version do these docs cover" / "how current are the rimsky docs" / "what's the rimsky-docs version" / similar. Echoes fixed values; reads nothing.
---

Echo exactly these two lines, verbatim, with **no preamble, no explanation, and no follow-up**:

```
rimsky release reconciled against : v0.7.0   (the rimsky release these docs describe)
rimsky-docs plugin version        : 1.4.0    (the docs' own release version)
```

These two literals are this skill's source of truth. **Read nothing** — not the plugin manifest, not any file — and change nothing. The `/release` skill stamps them in place at each rimsky-docs release; do not reintroduce a manifest read (an earlier `..`-relative lookup resolved unreliably in installed plugins).
