# Examples

Complete, copy-pasteable, no-ellipsis examples. Each file is runnable as written. Where a precondition is required (the bundled docker-compose stack must be up, etc.), the example states the exact command at the top.

- [`minimal-rimsky-yml.md`](minimal-rimsky-yml.md) — minimal operator config.
- [`minimal-template-and-instance.md`](minimal-template-and-instance.md) — register a one-node template, create an instance, observe completion.
- [`two-node-with-claim.md`](two-node-with-claim.md) — claim dependency between two nodes.
- [`claude-agent-attribute-defaults.md`](claude-agent-attribute-defaults.md) — claude-agent executor receiving model and prompts via attribute `default:` entries.
- [`holding-subgraph.md`](holding-subgraph.md) — held-claim resolution via `holds:`.
- [`rimsky-compose-multi-template.md`](rimsky-compose-multi-template.md) — multi-template project.
- [`atomic-staging.md`](atomic-staging.md) — atomic stage / verify / commit via a held subgraph and a filesystem claim-producer (the runnable bundle in `examples/atomic-staging-fs-producer/`).
