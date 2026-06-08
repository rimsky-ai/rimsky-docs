# Examples

Complete, copy-pasteable, no-ellipsis examples. Each file is runnable as written. Where a precondition is required (a running rimsky deployment, etc.), the example states it at the top.

- [`minimal-rimsky-yml.md`](minimal-rimsky-yml.md) — minimal operator config.
- [`minimal-template-and-instance.md`](minimal-template-and-instance.md) — register a one-node template, create an instance, observe completion.
- [`two-node-with-claim.md`](two-node-with-claim.md) — claim dependency between two nodes.
- [`claude-agent-attribute-defaults.md`](claude-agent-attribute-defaults.md) — claude-agent executor receiving model and prompts via attribute `default:` entries.
- [`holding-subgraph.md`](holding-subgraph.md) — held-claim resolution over a holding subgraph built with `holds:`.

For runnable Go server skeletons (one per protocol — executor, claim-producer, lifecycle-subscriber, publisher, validation, data-processing, atomic-staging-fs-producer) and a runnable `rimsky-compose.yml` + referenced `TemplateSpec` files, see the corpus-level [`../../examples/`](../../examples/README.md).
