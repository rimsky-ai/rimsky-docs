# Rimsky documentation

This directory hosts both the **public-documentation surface** (intended for external consumers and their coding agents) and the **internal/working surface** (engineering material that is unmaintained going forward and not intended for external citation).

## Source code

Rimsky's implementation lives in the public repository [`github.com/rimsky-ai/rimsky-core`](https://github.com/rimsky-ai/rimsky-core). The documentation here is generated and reconciled against that repository's latest release tag (currently **v0.4.1**); the prose is derived from the source and verified against it, but the source of truth is the repository, not these docs.

## Public surface

- `../README.md` — the project's framing and entry point. Start here.
- The canonical per-noun reference lives in `.ok-planner/design/concepts.md` at the repo root (auto-generated TOC over the per-concept files under `.ok-planner/design/concepts/`). Inline `@concept:` annotations in the source point at enforcement sites.
- `protocols/` — protocol-implementation guides (`ClaimProducer`, `Executor`, `LifecycleSubscriber`, `Publisher`).
- `reference/` — generated definitive references: the template / `rimsky.yml` schema (`template-schema.md`), the REST control-API routes (`rest-api.md`), and the CLI command tree (`cli.md`). Do not hand-edit.
- `services/` — catalog of the reference services rimsky ships (stores, executors, sensors, subscribers): protocol, config, ports, Dockerfile.
- `images/` — catalog of the official Docker images: name, contents, base image, build context.
- `agents/` — agent-shaped indices (`llms.txt`, `llms-full.txt`), error catalog, copy-pasteable examples.
- `glossary.md` — generated public-surface vocabulary. Do not hand-edit.
- `licensing.md` — repo licensing notice.

## Working / internal surface

- `internal/` — working engineering reference. **Unmaintained.** Not cited by the public surface.
- `specs/`, `plans/`, `history/`, `future-work/` — pipeline artifacts (specs, implementation plans, archived design docs). Ephemeral.
- `examples/` — narrative case-making material; not yet promoted to the public surface.

The public surface is fully self-contained: it cites within itself and into `protocols/proto/v1/*.proto` (the public wire contract). It does not cite, link to, or reference any file under `internal/`, `specs/`, `plans/`, `history/`, `future-work/`, or `examples/`.
