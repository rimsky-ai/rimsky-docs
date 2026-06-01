# Implementing a lifecycle subscriber

A **lifecycle subscriber** reacts to template, instance, and run-scope state
transitions in rimsky. It implements the opt-in `LifecycleSubscriber` protocol:
seven RPCs, each `On…(…Request) → LifecycleAck`. It is a mix-in — a service
advertises it alongside its primary protocol (typically `ClaimProducer`), never
standalone.

There is **no subscriber SDK** — implement against the wire types in any language.
A Go service may use the `protocols` module's `lifecycle` package (hand-written
types over the contract; [`go-packages.md`](go-packages.md)), or code straight to
the wire types. Wire contract: `lib/protocols/proto/v1/lifecycle.proto`; generated
field/message/RPC reference at [`reference/lifecycle.md`](reference/lifecycle.md).

<!-- @source: ../../.ok-planner/design/concepts/lifecycle-subscriber.md -->

**Auth-blind advisory.** Rimsky has no machinery for credentials, encryption, or
access control. Service-to-service auth is operator-configured at the deployment
layer.

## What you implement

All seven RPCs return `LifecycleAck` — no return data, just an acknowledgement
that the subscriber processed the event. Return success from any method the binary
doesn't react to; a binary that reacts to no event simply doesn't implement the
service.

| RPC | Required? | Purpose |
| --- | --- | --- |
| `OnTemplateRegistered(OnTemplateRegisteredRequest) → LifecycleAck` | No | A template hash is added to the registry. Provision idempotent per-template infrastructure (e.g. allocate an empty queue, prepare a sub-bucket). |
| `OnTemplateDeployed(OnTemplateDeployedRequest) → LifecycleAck` | No | A registered template moves to `deployed`. Warm caches, mark resources ready for instance traffic. |
| `OnTemplateUndeployed(OnTemplateUndeployedRequest) → LifecycleAck` | No | A deployed template moves to `undeployed`. Drain caches, mark resources for graceful winding-down. |
| `OnTemplateDeregistered(OnTemplateDeregisteredRequest) → LifecycleAck` | No | A template is removed from the registry. Delete provisioned per-template infrastructure. |
| `OnInstanceCreated(OnInstanceCreatedRequest) → LifecycleAck` | No | An operator (or compose up) creates a new instance against a deployed template. |
| `OnInstanceTerminated(OnInstanceTerminatedRequest) → LifecycleAck` | No | An instance moves to its terminal state — completed all frames or deleted by an operator. |
| `OnRunScopeTerminal(OnRunScopeTerminalRequest) → LifecycleAck` | No | A run-scope reaches terminal state. Unlike the other six, fires from control-api **or** the supervisor (see [Firing sites](#firing-sites-and-synchronous-fan-out)). |

Every RPC is opt-in; "Required?" is No for all seven because the protocol itself
is opt-in. A subscribed service should still return success from the hooks it
ignores.

## Boundaries

The subscriber **owns**:

- Reacting to each event it cares about — provisioning, cache warming/draining,
  teardown, notifications.
- Handling replays correctly (see [Idempotency](#idempotency)) — its own internal
  effects (allocating a queue, sending a notification) are not idempotent by
  default.
- Acknowledging fast (see [Firing sites](#firing-sites-and-synchronous-fan-out)) —
  pushing slow work into its own internal queue.
- Gating whether it actually *registers* the handlers, via its own startup-config
  flag (separate from rimsky.yml; see [Opting in](#opting-in)).

The subscriber does **NOT** own (rimsky's job):

- **The state transitions themselves.** Template/instance transitions happen in
  control-api; run-scope-terminal transitions happen in control-api (main scopes)
  or the supervisor (sub-graph and fan-out-partition scopes). The subscriber only
  observes; it never drives the transition.
- **Idempotency at the rimsky boundary.** Rimsky keys each event by
  `(service-name, event-type, object-id)` and makes replays no-ops on its own side.
  The subscriber does not signal rimsky to dedupe; it must dedupe its *own* effects.
- **Fan-out ordering and fan-out membership.** Rimsky decides which subscribed
  services receive an event and in what order; the subscriber cannot rely on
  inter-service or inter-event ordering.
- **Error propagation.** A subscriber error is logged but does not block fan-out to
  remaining subscribers and does not roll back the transition.
- **Delivery retries.** Rimsky's replay behavior (retries, restarts,
  operator-driven backfill) is rimsky-side; the subscriber does not request
  redelivery.

## Opting in

`LifecycleSubscriber` is a per-service configuration choice, gated across two
distinct surfaces in two different files.

1. **rimsky.yml — tells rimsky to fan out to the service.** Add
   `lifecycle_subscriber` to the service's `protocols: [...]` list:

   ```yaml
   claim_producers:
     my-store:
       endpoint: "grpc://my-store:9100"
       protocols: [claim_producer, lifecycle_subscriber]
       write_semantics_allowed: [sync]
   ```

   Without that entry, the service is silently skipped during fan-out — there is
   no error; non-subscription is the default.

2. **The producer binary's own config — tells the binary to register the handlers.**
   A producer that ships a no-op `LifecycleSubscriber` may gate registration behind
   its own startup-config flag. The in-tree stub store
   (`test/support/stores/stub/`) registers its handlers only when its server config
   sets `enable_lifecycle: true`, not from rimsky.yml — so operators can turn the
   handlers on without forking the binary.

   ```yaml
   # the producer binary's own config
   enable_lifecycle: true
   ```

The flag is per-service, not per-protocol. A service implementing both
`ClaimProducer` and `LifecycleSubscriber` lists both protocols; the gRPC server
registers handlers for both. With both surfaces set, rimsky fans out the seven
lifecycle methods at the matching state transitions.

## Per-hook request payloads

Full field reference (types, field numbers): [`reference/lifecycle.md`](reference/lifecycle.md).

`OnTemplateRegisteredRequest` carries the template hash plus `spec` (`bytes`) — the
template's canonical JCS spec bytes, deterministically re-hashable.
`OnTemplateDeployedRequest` carries the template hash plus `tags`. The undeploy and
deregister requests carry only the `template_hash`.

`OnInstanceCreatedRequest` salient fields:

| Field | Meaning |
| --- | --- |
| `instance_id`, `template_hash`, `instance_key` | Which instance, against which template. |
| `params` | The instance params (`bytes`). |
| `service_bindings` | Per-instance late-bound service catalog (opaque JSONB `bytes`); empty when the instance has no late-bound services. Consumed by the host-agent-proxy to populate its binding cache. |
| `owner_api_key_id` | The api-key whose authenticated request created the instance (empty string for anonymous-mode-created instances). Consumed by the host-agent-proxy to route dispatches to the right user's agent. |

`OnInstanceTerminatedRequest` carries `instance_id`, `template_hash`, and
`terminated_at_unix_ms` (`int64`).

`OnRunScopeTerminalRequest` salient fields:

| Field | Meaning |
| --- | --- |
| `run_scope_id` | The terminating run-scope. |
| `terminal_reason` | Why it terminated. |
| `instance_id` | The owning instance of the terminating run-scope. Populated at every firing site (control-api for main scopes; the supervisor for sub-graph and fan-out-partition scopes). The host-agent-proxy keys lazily-spawned children by instance id (its v1 dispatch-observable scope), so the reap path matches on `instance_id` rather than `run_scope_id`. Empty only for legacy callers that predate this field. |

## Firing sites and synchronous fan-out

Lifecycle events fire **synchronously** from the rimsky-side process that owns the
triggering transition. The six template/instance events fire from control-api, so
a slow subscriber slows the control-api response on the triggering operation (e.g.
a slow `OnTemplateDeployed` makes `POST /templates/{id}/deploy` slow).
`OnRunScopeTerminal` fires from control-api (main scopes, polling-driven) or the
**supervisor** (sub-graph and fan-out-partition scopes, synchronous, in-transaction);
a slow subscriber there holds up the firing process's path.

- **Be fast.** Subscribers should acknowledge within hundreds of milliseconds. Push
  slow work into the subscriber's own internal queue.
- **Don't depend on inter-event ordering.** The firing process fans out to
  subscribed services in a fixed but unspecified order; an `OnTemplateDeployed`
  notification from service A may arrive before or after service B's notification.
- **Failures don't block other subscribers.** A subscriber returning an error is
  logged but does not block fan-out to remaining subscribers.

## Idempotency

Rimsky tracks idempotency at its own boundary: each event is keyed by
`(service-name, event-type, object-id)`. Replays — caused by retries, restarts, or
operator-driven backfill — are no-ops at the rimsky side. For `OnRunScopeTerminal`,
idempotency is preserved across **both** firing sites (control-api and the
supervisor) via the same ledger, keyed `scope_kind="run_scope"`,
`state="run_scope_terminal"`.

That is the rimsky-side guarantee; the subscriber must still handle replays
correctly because its own internal effects (e.g. allocating a queue, sending a
notification) may not be idempotent by default. The recommended pattern is to treat
each handler as if it could be invoked multiple times for the same
`(event-type, object-id)` and short-circuit early.

## Reference impl

There is no standalone reference `LifecycleSubscriber` binary in the tree —
lifecycle handlers ride inside producer binaries. The in-tree example is the stub
store (`test/support/stores/stub/`), whose server registers `LifecycleSubscriber`
handlers when its own config sets `enable_lifecycle: true`. The in-tree OpenLineage
subscriber at `lib/services/subscribers/openlineage/` is a *polling* reader of the
lineage projection, **not** a `LifecycleSubscriber` implementation — it is a
different integration shape.

## See also

[lifecycle-subscriber](../concepts/lifecycle-subscriber.md) · [template](../concepts/template.md) · [instance](../concepts/instance.md) · [claim-producer](../concepts/claim-producer.md)
