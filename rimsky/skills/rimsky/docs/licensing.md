# Rimsky Licensing — Operator FAQ

Rimsky is multi-licensed along **one bright line**, applied per-file: two
surfaces, three license options. **Apache 2.0 covers exactly the implement/link
surface** — the wire-protocol contract, the TypeScript executor reference impl,
and the docs; it carries no copyleft and no commercial track. **Everything else
rimsky ships is the dual-licensed orchestrator layer:** AGPL-3.0-or-later by
default (no action required), OR a Fall Guy Consulting commercial license as an
alternative over the same code. That layer is the `rimsky` CLI, the Go reference
executors and stores, the sensors and subscribers, conformance, and the deploy
artifacts (`dockerfiles/`). These are real, runnable artifacts — using them is
using rimsky, and that use carries the AGPL's copyleft unless you hold the
commercial license.

The boundary is mechanical, not prose: `licensing.yml` is the source of truth,
and `make license-lint` verifies it on every CI run. `COPYRIGHT` carries the
formal per-layer notice; `COPYING.md` is the plain-language compliance guide.
Classification is longest-prefix-match, so a specific subdirectory can override
its parent's bucket.

## The surface → license map

| Surface | Path | License |
| --- | --- | --- |
| Wire-protocol contract (IDL + generated bindings + protocol-kit / conformance libraries) | `lib/protocols/` | Apache 2.0 |
| TypeScript executor reference impl | `lib/services/executors/claude-agent/` | Apache 2.0 |
| Documentation (cold-read style guide) | `cold-read/` | Apache 2.0 |
| `rimsky` CLI (all role binaries; conformance subcommands) | `cmd/` | AGPL-3.0-or-later / commercial |
| Go reference executors (`http-node`, the verifiers) | `lib/services/executors/` (except `claude-agent/`) | AGPL-3.0-or-later / commercial |
| Reference stores | `lib/services/stores/` (`filesystem`, `postgres`) | AGPL-3.0-or-later / commercial |
| Sensors | `lib/services/sensors/` | AGPL-3.0-or-later / commercial |
| Subscribers | `lib/services/subscribers/` | AGPL-3.0-or-later / commercial |
| Orchestrator core (foundation, graph, control, runtime) | `lib/foundation/`, `lib/graph/`, `lib/control/`, `lib/runtime/` | AGPL-3.0-or-later / commercial |
| Tests + dev tooling | `test/`, `tools/` | AGPL-3.0-or-later / commercial |

The Apache surface is **exactly those three entries** — the wire contract, the
TypeScript executor, and the docs. Nothing else rimsky ships is Apache. There is
no permissive "stub" carve-out: the test-support stub doubles live under the AGPL
trees and are AGPL like the rest of `test/`.

The Apache code forms a single closed island: because `lib/protocols/` imports
nothing internal (the protocols-purity depguard), no Apache-licensed file can
depend on an AGPL-licensed one. That is the whole point of drawing the line at
the wire contract.

## What each license requires

**Apache 2.0** — permissive, no copyleft. Use, modify, and redistribute under
the Apache terms, including in closed-source products. Preserve the license and
copyright notices and the `NOTICE` file.

**AGPL-3.0-or-later** — strong copyleft, and the default; no action is required
to use rimsky under it. If you modify rimsky and convey it, **or make it
available to users over a network**, you must offer those users the
corresponding source under the AGPL. The network-service clause is §13 — the
clause that distinguishes the AGPL from the GPL.

**Fall Guy Consulting commercial license** — an alternative to the AGPL, by
separate agreement, over the same AGPL-licensed code. For organizations that
want to use, modify, or distribute the orchestrator and services without the
AGPL's §5 (copyleft) or §13 (network-service source disclosure) obligations. It
does not change the Apache terms on `lib/protocols/`. Contact
`licensing@fallguyconsulting.com`.

## Can I run Rimsky internally without disclosing source?

Yes. AGPL §13 only triggers on offering rimsky as a network service to *third
parties*. Internal use within a single legal entity — even when serving
thousands of users via internal tools — is unaffected. You may modify rimsky and
run it on your own infrastructure for your own operational purposes without
publishing those modifications.

## Can I offer Rimsky as a hosted service to my customers under AGPL?

Yes, but you must publish your modifications and let users (downstream customers)
download the corresponding source for the AGPL-licensed code — the orchestrator,
the CLI, and the reference services. This is what AGPL §13 requires. The
Apache-licensed portions (the wire contract, the TypeScript executor, the docs)
carry no such obligation.

If you would prefer to operate a hosted service without that publication
obligation, the commercial license track removes it.

## I'm a SaaS company embedding Rimsky inside my product. Do I need a commercial license?

If your product is offered over a network and includes a modified rimsky, AGPL
§13 reaches it: your users have the right to receive the corresponding source for
the AGPL portion. The cleanest answer is the commercial license, which removes
that obligation. The alternative — publishing your modifications under AGPL — is
also legally valid but typically not what proprietary product companies prefer.

If you embed only **unmodified** rimsky and your modifications sit in your own
executors, stores, or product-side code, AGPL §13 still requires you to offer
rimsky's source to users on request, but unmodified-source-as-shipped already
satisfies that — point them at the upstream repository. Note that the reference
executors and stores rimsky ships are themselves AGPL; to keep your own
extensions outside the AGPL, write them as independent processes that speak the
Apache-licensed wire protocol (see below), not as forks of the shipped services.

## Can I link my proprietary code to unmodified Rimsky?

If "linking" means "import the AGPL-licensed Go packages into my proprietary
application and produce a single binary," the FSF reads that as creating a
derivative work, and the combined binary would have to be AGPL-compatible. The
commercial license sidesteps the question.

If "linking" means "run rimsky as a separate process and talk to it over its
wire protocol," there is no derivative-work relationship: your process is
independent, and the wire protocol is Apache-licensed. This is the recommended
embedding pattern. It is the *only* part of rimsky a consumer is ever required
to implement or link against — which is exactly why it, and nothing else, is
permissive.

Separately from your own code: the rimsky orchestrator/CLI process you run is
itself AGPL-3.0-or-later. Running it as a separate process keeps your app,
executors, and stores outside the AGPL, but the rimsky process carries its own
§13 obligation — if you expose that process as a network service to third
parties, §13 obligates you to offer rimsky's source to those users (for an
unmodified build, satisfied by pointing them at the upstream repository).
Purely internal use within one legal entity triggers no §13 obligation, and the
commercial license removes it entirely.

## Can I write a closed-source executor, store, or CLI that talks to Rimsky?

Yes. Executors, stores, and other services are independent processes that speak
the wire protocol; the protocol is Apache-licensed. There is no license
relationship between your service and the rimsky orchestrator beyond the wire.
Build whatever you want against `lib/protocols/`; you owe nothing back.
(Contributions back are welcomed under the CLA in `CLA.md`, but are not
required.) Be aware that the reference executors and stores rimsky *ships* —
`http-node`, the verifiers, `filesystem`, `postgres` — are AGPL; a closed-source
implementation must be your own, talking over the wire, not a fork of those.

## Can I fork Rimsky and offer hosted Rimsky-as-a-Service?

Under AGPL, yes — provided you publish your modifications under AGPL and make the
corresponding source available to your users. We discourage it (this is what the
AGPL is designed to deter, and what the commercial license is designed to
capture) but do not forbid it.

The trademark policy in `TRADEMARKS.md` prohibits calling the fork "Rimsky" or
"Rimsky-<suffix>". Choose a distinct project name (acceptable: "FooEngine, based
on Rimsky"). Hosted services may not market under the Rimsky name — "Hosted
FooEngine, powered by Rimsky" is acceptable, "Hosted Rimsky" is not. A software
license is not a trademark license; the trademark grant is governed by
`TRADEMARKS.md` independently of the Apache/AGPL grant on the code.

The "Rimsky-compatible" label is available if your fork passes the upstream
conformance suite for the protocol version it claims to implement. Conformance
is now built into the AGPL `rimsky` binary as `rimsky conformance <protocol>`
subcommands (`executor`, `claim-producer`, `publisher`, `validation`,
`data-processing`, `blob-backend`, `probe`); the underlying runner libraries
live in `lib/protocols/conformance/` and are themselves Apache 2.0, so running
them against your implementation carries no copyleft obligation.

## Packages and images

Rimsky publishes exactly **one** npm package:

| Artifact | Identifier | License | Distribution |
| --- | --- | --- | --- |
| Wire-contract bundle (`.proto` files + `index.js` / `index.d.ts` for `@grpc/proto-loader` consumers) | `@rimsky-ai/protocols` (scope `@rimsky-ai`; published, public) | Apache 2.0 | npm, via the Makefile `publish-protocols` target — **the only rimsky package a consumer installs from npm** |
| TypeScript executor reference impl | `@rimsky/executor-claude-agent` (scope `@rimsky`; **PRIVATE — `"private": true`, NOT published to npm**) | Apache 2.0 | **Not** on npm — ships only as the `rimsky-executor-claude-agent` Docker image |

The two scopes are intentional, not a typo: `@rimsky-ai/protocols` is the lone
published package; `@rimsky/executor-claude-agent` is marked `"private": true`
and never reaches npm. The TypeScript executor is Apache-licensed source, but you
consume it as the `rimsky-executor-claude-agent` Docker image, not via
`npm install`. There is no other rimsky npm package.

---

For commercial license inquiries, contact `licensing@fallguyconsulting.com`. The
authoritative per-file map is `licensing.yml`; the formal notice is `COPYRIGHT`;
the plain-language compliance guide is `COPYING.md`.
