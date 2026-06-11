# Implementing a publisher

> **Version.** The API on this page targets the rimsky release this corpus is
> reconciled against (`reconciledAgainst` in `.claude-plugin/plugin.json`). For
> runnable, version-pinned code, copy the publisher at
> [`../examples/publisher/`](../examples/README.md).

A **publisher** is a peer service that publishes messages into rimsky. It
implements the `Publisher` protocol (four verbs: `Capabilities`, `Subscribe`,
`Unsubscribe`, `ListSubscriptions`) and emits each message by POSTing a message
envelope to the generic `POST /v1/instances/{id}/messages` endpoint — there is no
special observation-deposit route. **Sensors** (cron / http / object-store /
webhook) are one kind of publisher.

There is **no publisher SDK** — implement against the wire types in any language.
A Go service may use the `protocols` module's `publisherkit` package
([`go-packages.md`](go-packages.md)) for optional publisher-side message-emit
retry/backoff scaffolding; it is a convenience, not a requirement. Wire contract:
`lib/protocols/proto/v1/publisher.proto`; generated field/message/RPC reference at
[`reference/publisher.md`](reference/publisher.md).

The `rimsky-host-agent-proxy` transparently fronts `Publisher` alongside the
other four fronted protocols (`Executor`, `ClaimProducer`, `Validation`,
`DataProcessing`) — a late-bound publisher binary that conforms to this wire
contract works behind the proxy by construction with no proxy-specific code. See
[`README.md`](README.md#host-agent-proxy-a-transparent-forwarder) for the
implementor consequences.

<!-- @source: .ok-planner/design/concepts/publisher.md -->

## What you implement

| RPC | Required? | Purpose |
| --- | --- | --- |
| `Capabilities(Empty) → PublisherCapabilities` | **Yes** | Startup handshake: advertise supported kinds + mix-in protocols. Rimsky probes once and caches in the discovery cache. |
| `Subscribe(SubscribeRequest) → SubscribeResponse` | **Yes** | Start a publisher-subscription for one `(instance, kind)` pair; persist its inline routing fields. |
| `Unsubscribe(UnsubscribeRequest) → UnsubscribeResponse` | **Yes** | Stop a publisher-subscription. |
| `ListSubscriptions(Empty) → ListSubscriptionsResponse` | **Yes** | Reconcile-on-startup: rimsky compares its expected subscription set against the publisher's reported set. |

All four verbs are rimsky → publisher calls. The publisher's outbound message
emit goes the other direction over HTTP+JSON (see [The message envelope](#the-message-envelope)),
not through this gRPC surface.

## Boundaries

The publisher **owns**:

- Its substrate (cron clock, HTTP endpoint, object-store, webhook port, etc.) and
  the watching/firing loop over it.
- Its own per-subscription state (next fire time, body hash, watermark cursor,
  last idempotency key). Each publisher owns its own state DB.
- Persisting `target_node` and `message_kind` from `Subscribe` and copying them
  onto every emitted envelope as `target` and `kind`.
- Constructing and POSTing the message envelope at fire time, including the
  `Idempotency-Key` header.
- Its own HA posture — single-replica is the v1 contract (see [Replica contract](#replica-contract)).

The publisher does **NOT** own (rimsky's job):

- **The persisted binding state.** The `(instance, publisher, kind)` row lives in
  rimsky's `rimsky_publisher_subscriptions` ledger; the publisher holds only its
  substrate-specific state, not the binding metadata.
- **The capability check.** Rimsky validates the subscription is live before
  inserting any message (see [Rimsky-side capability check](#rimsky-side-capability-check)).
  The publisher does not enforce authorization.
- **`sender` derivation.** Rimsky derives `sender` from the subscription row's
  `publisher_name` and ignores the `sender` field on the request.
- **The message envelope's onward routing.** Once accepted, the payload flows
  through rimsky's cascade machinery unread; messages are inert in rimsky. The
  publisher does not interpret or route past the emit.
- **Replica coordination.** Rimsky does not coordinate multi-replica fan-in; HA at
  the publisher tier is the publisher's own concern.
- **Credentials, encryption, access control.** Rimsky is auth-blind;
  service-to-service auth is operator-configured at the deployment layer.

## `Capabilities` — startup handshake

Probed once per service at startup; cached in rimsky's discovery cache. The
`PublisherCapabilities` response declares:

| Field | Type | Meaning |
| --- | --- | --- |
| `supported_kinds` | `repeated PublisherKindCapability` | The kinds this binary supports. Each `PublisherKindCapability` carries a `kind` string and a `config_schema` (`bytes`) — the JSON Schema for the `resolved_config` accepted at `Subscribe` for that kind. `config_schema` is opaque to rimsky and surfaces to operator tooling. |
| `protocols` | `repeated string` | Mix-in service protocols advertised (e.g. `"publisher"`, `"validation"`). The list must include `"publisher"`. |
| `validation_supported_roles` | `repeated string` | Set when `"validation"` is in `protocols` (e.g. `["publisher"]`). **Caveat:** rimsky's startup validation-registry wiring does not yet read this field from publisher (or executor) peers — only claim-producer peers get the live `ClaimProducer.Capabilities` roles handshake; a publisher peer's supported-roles list is currently treated as empty. <!-- @source: lib/control/config/publishers.go --> |

Full field reference: [`reference/publisher.md`](reference/publisher.md).

## `Subscribe` — start a publisher-subscription

Called per `(instance, kind)` pair when the template's `publishers:` block declares
a publisher entry (resolved at instance creation). `SubscribeRequest` carries the
routing fields **inline** — there is no `on_change` / on-observation substruct.

### `SubscribeRequest`

| Field | Type | Meaning |
| --- | --- | --- |
| `publisher_subscription_id` | `string` | The subscription identity (UUIDv4). |
| `instance_id` | `string` | The instance this subscription belongs to. |
| `kind` | `string` | Which supported kind to watch under. |
| `resolved_config` | `bytes` | Per-instance config the publisher watches against (cron expression, HTTP URL, S3 bucket+prefix, …). Substituted by rimsky from the template `publishers:` block before dispatch. |
| `target_node` | `string` | Receiver node alias on the instance side. The publisher copies it onto each envelope as `target`. |
| `message_kind` | `string` | Wire-level message kind; default `"invalidate"` when empty. The publisher copies it onto each envelope as `kind`. **V1 restriction:** `POST /v1/instances/{id}/messages` rejects any `kind` other than `"invalidate"` with `400` — other kinds are V2, so `"invalidate"` is the only value that emits successfully in V1. <!-- @source: lib/control/controlapi/messages.go::handleCreateMessage --> |

`SubscribeResponse` is empty. The publisher persists `target_node` and
`message_kind` alongside its subscription state; at fire time it constructs
`{kind: message_kind, target: target_node, ...}` envelopes from these inline fields.

Rimsky calls `Subscribe` up to **3 attempts total** (the initial call plus 2
retries) with exponential backoff: ~560ms before attempt 2 and ~1.6s before
attempt 3 (200ms base × ~2.828 per attempt, ±25% jitter; the bare 200ms base is
never slept) before flipping the publisher-subscription
row to `state='failed'`. A `failed` row is operator-recoverable: the startup
resync pass re-issues `Subscribe`, transitioning `failed → active` on success.

## `Unsubscribe` — stop a publisher-subscription

Called per registered publisher-subscription at instance termination. `UnsubscribeRequest`
carries a single field `publisher_subscription_id` (`string`); `UnsubscribeResponse`
is empty. On success, rimsky transitions the row `active → stopped`.

## `ListSubscriptions` — reconcile on startup

Called at supervisor startup so rimsky can compare its expected subscription set
against what the publisher reports actually running. `ListSubscriptionsResponse`
carries `subscriptions` (`repeated PublisherSubscriptionDescriptor`); each
descriptor mirrors the subscription state:

| Field | Type | Meaning |
| --- | --- | --- |
| `publisher_subscription_id` | `string` | The subscription identity. |
| `instance_id` | `string` | The owning instance. |
| `kind` | `string` | The watched kind. |
| `resolved_config` | `bytes` | The per-instance config. |
| `target_node` | `string` | The receiver node alias. |
| `message_kind` | `string` | The wire-level message kind. |
| `started_at` | `Timestamp` | When the subscription began. |

Rimsky reconciles publisher-side state against its `rimsky_publisher_subscriptions`
row set on this pass — re-issuing `Subscribe` for any expected-but-missing binding.

## The message envelope

A publisher emits by POSTing a message envelope to the generic operator
message-emit endpoint — there is no special observation-deposit route:

```
POST /v1/instances/{instance_id}/messages
Idempotency-Key: <key>          # for at-most-once semantics
Content-Type: application/json

{
  "kind": "invalidate",
  "target": "tick",
  "payload": <raw observation bytes>,
  "sender": "sensor-cron",
  "sender_kind": "publisher",
  "publisher_subscription_id": "8a4f...uuid"
}
```

- `kind` and `target` are the persisted `message_kind` / `target_node` from
  `Subscribe`. In V1 the endpoint accepts only `kind: "invalidate"`; any other
  kind is rejected with `400` (see the [`message_kind` field](#subscriberequest)).
- `sender_kind` MUST be `"publisher"` and `publisher_subscription_id` MUST be the
  token from `Subscribe` — together they are the capability token rimsky checks.
- The request's `sender` field is **ignored** — rimsky derives `sender` from the
  subscription row's `publisher_name`. Setting it has no trust effect.
- The `payload` bytes are inert in rimsky: they flow from publisher → message
  envelope → consumer's substitution leaf without inspection.
- Send the `Idempotency-Key` header for at-most-once delivery.

## Rimsky-side capability check

Before inserting the message, rimsky validates that
`(publisher_subscription_id, instance_id, state='active')` is a live row in
`rimsky_publisher_subscriptions` — a three-way match. A subscription ID presented
against the wrong instance (cross-instance) returns **`403 Forbidden`**. A revoked
or unknown capability is likewise rejected at this boundary, not at the publisher.

## Retry & backoff on emit

A publisher POSTing to `POST /v1/instances/{id}/messages` should retry transient
failures: **5xx responses and transport/connection errors retry up to 3 attempts
total** with exponential backoff (base 200ms, geometric multiplier ~2.828:
sleep 200ms after attempt 1 and ~566ms after attempt 2, so attempt 3 issues
~766ms after the first attempt). A **4xx is terminal** — it means rimsky rejected the
capability, the route is gone, or the body is invalid (including a non-`invalidate`
`kind` in V1, which returns `400`); retrying would not help, so
abandon immediately and log it at WARN under the `publisher.message.rejected` key
so operators can spot capability-revocation / route-misconfiguration without
digging through per-publisher log noise. `2xx` is success.

This is the policy the Go `publisherkit.Send` helper implements; a non-Go publisher
implements the same retry/idempotency-header behavior directly.

## Replica contract

Single-replica is the **v1 contract**. Rimsky does not coordinate multi-replica
fan-in — at scale=N, rimsky-level behavior is the union of N independent processes.
Running two replicas of the same publisher binary pointed at the same rimsky
endpoint will **double-fire per fire window** (e.g. two cron-sensor replicas fire
twice). Operators wanting HA pick a publisher implementation that handles
coordination itself; the bundled sensors do not attempt it. See
[replica](../concepts/replica.md).

## Conformance

`rimsky conformance publisher` is a black-box conformance suite; point it at any
`Publisher` implementation to verify lifecycle + emit shape:

```sh
rimsky conformance publisher --endpoint grpc://my-publisher:9100 \
                             --kind cron \
                             --resolved-config '{"cron":"* * * * *"}' \
                             --instance-id <uuid>
```

The same checks are exposed as a Go library under
`lib/protocols/conformance/publisher`.

## Reference impls

A copyable, **Apache** skeleton you can adapt is at
[`../examples/publisher/`](../examples/README.md): it manages subscriptions
(Subscribe / Unsubscribe / ListSubscriptions) in memory and advertises a kind via
`Capabilities`. Vendored from rimsky-core's `examples/` module at the reconciled
tag.

Sensors are one kind of publisher. The four reference sensors ship under
`lib/services/sensors/` as **AGPL** runnable products — study them for patterns,
but build your own from the wire contract and the copyable skeleton above rather
than copying AGPL service code:

| Binary | Substrate |
| --- | --- |
| `sensor-cron` | Cron firing. |
| `sensor-http` | HTTP-poll with body-hash watermark. |
| `sensor-object-store` | Object-store list with `name` or `last_modified` watermark. |
| `sensor-webhook` | Inbound webhook receiver. |

Each is single-replica per the [replica contract](#replica-contract) and carries
its own README and config.

The sensors also demonstrate the **DSN-gated state-persistence pattern** an
implementor should copy: each owns a sensor-specific state schema behind a
per-binary env var (`RIMSKY_SENSOR_CRON_STATE_DSN`,
`RIMSKY_SENSOR_HTTP_STATE_DSN`, `RIMSKY_SENSOR_OBJECT_STORE_STATE_DSN`,
`RIMSKY_SENSOR_WEBHOOK_STATE_DSN`; Postgres-only DSN). With the env unset the binary runs in-memory and relies on
rimsky's startup resync (`ListSubscriptions` + `Subscribe` replay) to
reconstruct subscriptions; setting it persists subscription rows + watermarks
across sensor restarts. The state table shape is each sensor's own — it is
deliberately not shared with rimsky's persistence layer.
<!-- @source: lib/services/sensors/sensor-http/state_db.go -->

`sensor-object-store` validates backends at startup and advertises (via
`Capabilities`) **only** the registered set. The default bundled image always
registers the `memory` backend and conditionally registers a `filesystem`
backend (a directory lister) when `RIMSKY_SENSOR_OBJECT_STORE_FS_ROOT` names a
root directory — leaving the env unset omits `filesystem` from the advertised
set. <!-- @source: lib/services/sensors/sensor-object-store/main.go --> A
`Subscribe` naming an unregistered backend
(`s3` / `gcs` / `azure`) is rejected at `Subscribe` time rather than silently
no-op'ing at poll time. A deployment needing a cloud backend builds its own binary
that registers the desired lister before serving, after which the sensor advertises
and accepts it automatically. This is the general pattern for any kind-gated
publisher: advertise exactly what you can service, and reject unservable
subscriptions at `Subscribe`.

## See also

[publisher](../concepts/publisher.md) · [publisher-subscription](../concepts/publisher-subscription.md) · [sensor](../concepts/sensor.md) · [message](../concepts/message.md) · [replica](../concepts/replica.md)
