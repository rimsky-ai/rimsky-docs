# Implementing a lifecycle subscriber

This guide is for developers implementing a lifecycle subscriber — a service that wants to react to template and instance state transitions in Rimsky. The wire contract lives at `lib/protocols/proto/v1/lifecycle.proto`; this guide is the practical companion.

<!-- @source: ../../.ok-planner/design/concepts/lifecycle-subscriber.md -->
> An opt-in protocol for services that want to react to template, instance, and run-scope state transitions. Seven methods: `OnTemplateRegistered`, `OnTemplateDeployed`, `OnTemplateUndeployed`, `OnTemplateDeregistered`, `OnInstanceCreated`, `OnInstanceTerminated`, `OnRunScopeTerminal`. Template/instance events fire synchronously from the control-api process; the run-scope-terminal event fires from the rimsky-side process that owns the transition (control-api for main scopes, the supervisor for sub-graph and fan-out-partition scopes).

> **Auth-blind advisory.** Rimsky has no machinery for credentials, encryption, or access control. Service-to-service auth is operator-configured at the deployment layer.

---

## 1. The wire contract

```protobuf
service LifecycleSubscriber {
  rpc OnTemplateRegistered(OnTemplateRegisteredRequest)     returns (LifecycleAck);
  rpc OnTemplateDeployed(OnTemplateDeployedRequest)         returns (LifecycleAck);
  rpc OnTemplateUndeployed(OnTemplateUndeployedRequest)     returns (LifecycleAck);
  rpc OnTemplateDeregistered(OnTemplateDeregisteredRequest) returns (LifecycleAck);
  rpc OnInstanceCreated(OnInstanceCreatedRequest)           returns (LifecycleAck);
  rpc OnInstanceTerminated(OnInstanceTerminatedRequest)     returns (LifecycleAck);
  rpc OnRunScopeTerminal(OnRunScopeTerminalRequest)         returns (LifecycleAck);
}
```

The mechanically-generated message/field reference is at [`reference/lifecycle.md`](reference/lifecycle.md). For Go services, the `protocols` module's `lifecycle` package provides hand-written types over this contract ([`go-packages.md`](go-packages.md)); using it is optional.

All seven methods return `LifecycleAck` — there's no return data, just an acknowledgement that the subscriber processed the event. The implementer pattern is to return success from any method the binary doesn't react to; a binary that reacts to no event simply doesn't implement the service.

## 2. Opting in

`LifecycleSubscriber` is a mix-in protocol; opting in is a per-service configuration choice. Add `lifecycle_subscriber` to the service's `protocols: [...]` list in `rimsky.yml`:

```yaml
claim_producers:
  my-store:
    endpoint: "grpc://my-store:9100"
    protocols: [claim_producer, lifecycle_subscriber]
    write_semantics_allowed: [sync]
```

Without that entry, the service is silently skipped during fan-out — there's no error, non-subscription is the default.

The flag is per-service, not per-protocol. A service that implements both `ClaimProducer` and `LifecycleSubscriber` lists both protocols; the gRPC server registers handlers for both.

There are two distinct surfaces here: the rimsky.yml `protocols: [...]` list (above) is what tells rimsky to *fan out* to the service. Separately, a producer binary that ships a no-op `LifecycleSubscriber` may gate whether it actually *registers* the handlers behind its own startup-config flag — the in-tree stub store (`test/support/stores/stub/`) registers its handlers only when its own server config sets `enable_lifecycle: true`, not from rimsky.yml — so operators can turn the handlers on without forking the binary.

## 3. The seven events

### `OnTemplateRegistered`

Fired when a template hash is added to the registry. Common subscriber response: provision idempotent infrastructure tied to the template (e.g. allocate an empty queue, prepare a sub-bucket).

### `OnTemplateDeployed`

Fired when a registered template moves to `deployed`. Templates must be deployed before instances can be created against them. Common subscriber response: warm caches, mark resources ready for instance traffic.

### `OnTemplateUndeployed`

Fired when a deployed template moves to `undeployed`. Existing instances continue; new instances cannot be created against this template. Common subscriber response: drain caches, mark resources for graceful winding-down.

### `OnTemplateDeregistered`

Fired when a template is removed from the registry. Common subscriber response: delete provisioned infrastructure tied to the template.

### `OnInstanceCreated`

Fired when an operator (or compose up) creates a new instance against a deployed template.

### `OnInstanceTerminated`

Fired when an instance moves to its terminal state — completed all frames or was deleted by an operator.

### `OnRunScopeTerminal`

Fired when a run-scope reaches terminal state. `OnRunScopeTerminalRequest` carries `run_scope_id`, `terminal_reason`, and the owning `instance_id`. Unlike the other six, this event fires from whichever rimsky-side process owns the transition: control-api for main scopes (polling-driven), and the **supervisor** for sub-graph and fan-out-partition scopes (synchronous, in-transaction). Idempotency is preserved across both firing sites via the same ledger (keyed `scope_kind="run_scope"`, `state="run_scope_terminal"`).

## 4. Idempotency

<!-- @source: ../../.ok-planner/design/concepts/lifecycle-subscriber.md -->
> An opt-in protocol for services that want to react to template, instance, and run-scope state transitions. Seven methods: `OnTemplateRegistered`, `OnTemplateDeployed`, `OnTemplateUndeployed`, `OnTemplateDeregistered`, `OnInstanceCreated`, `OnInstanceTerminated`, `OnRunScopeTerminal`. Template/instance events fire synchronously from the control-api process; the run-scope-terminal event fires from the rimsky-side process that owns the transition (control-api for main scopes, the supervisor for sub-graph and fan-out-partition scopes).

Rimsky tracks idempotency at its own boundary: each event is keyed by `(service-name, event-type, object-id)`. Replays — caused by retries, restarts, or operator-driven backfill — are no-ops at the rimsky side.

That's the rimsky-side guarantee; the subscriber must still handle replays correctly because its own internal effects (e.g. allocating a queue, sending a notification) may not be idempotent by default. The recommended pattern is to treat each handler as if it could be invoked multiple times for the same `(event-type, object-id)` and short-circuit early.

## 5. Synchronous fan-out

Lifecycle events are fired synchronously from the rimsky-side process that owns the triggering transition. The six template/instance events fire from control-api, so a slow subscriber slows the control-api response on the triggering operation (e.g. a slow `OnTemplateDeployed` makes `POST /templates/{id}/deploy` slow). `OnRunScopeTerminal` fires from control-api (main scopes) or the supervisor (sub-graph and fan-out-partition scopes); a slow subscriber there holds up the firing process's path.

Implications:

- **Be fast.** Subscribers should acknowledge within hundreds of milliseconds. Push slow work into the subscriber's own internal queue.
- **Don't depend on inter-event ordering.** The firing process fans out to subscribed services in a fixed but unspecified order; an `OnTemplateDeployed` notification from service A may arrive before or after service B's notification.
- **Failures don't block other subscribers.** A subscriber returning an error is logged but does not block fan-out to remaining subscribers.

## 6. Reference impl

There's no standalone reference `LifecycleSubscriber` binary in the tree — lifecycle handlers ride inside producer binaries. The in-tree example is the stub store (`test/support/stores/stub/`), whose server registers `LifecycleSubscriber` handlers when its own config sets `enable_lifecycle: true`. (The in-tree OpenLineage subscriber at `lib/services/subscribers/openlineage/` is a *polling* reader of the lineage projection, not a `LifecycleSubscriber` implementation — it is a different integration shape.)

Two config surfaces, in two different files:

```yaml
# rimsky.yml — tells rimsky to fan out lifecycle events to this peer
claim_producers:
  my-store:
    endpoint: "grpc://my-store:9100"
    protocols: [claim_producer, lifecycle_subscriber]
    write_semantics_allowed: [sync]
```

```yaml
# the producer binary's own config — tells the binary to register the handlers
enable_lifecycle: true
```

With both set, the service's gRPC server registers both `ClaimProducer` and `LifecycleSubscriber` handlers, and rimsky fans out the seven lifecycle methods at the matching state transitions.

## See also

- [lifecycle-subscriber](../concepts/lifecycle-subscriber.md)
- [template](../concepts/template.md)
- [instance](../concepts/instance.md)
- [claim-producer](../concepts/claim-producer.md)
