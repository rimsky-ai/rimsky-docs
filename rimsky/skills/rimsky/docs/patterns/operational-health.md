---
concept: operational-health
definition: |
  How operators observe and maintain a running rimsky deployment: lifecycle subscribers for audit and per-tenant rollups, retry-loop detection, parked-node watchdogs, frame-stuck warnings.
proto_symbol: (none)
config_field: (none)
api_surface: GET /v1/admin/diagnostics/held-frames, GET /v1/admin/diagnostics/parked-nodes, GET /v1/admin/diagnostics/wait-sets, POST /v1/admin/instances/{instance}/nodes/{node_id}/invalidate, GET /v1/events, GET /metrics
related: [lifecycle-subscriber, parked-state, frame, error-policy]
deprecated_terms: []
---

# Operational health

> **Status (v0.9.0).** Fully supported. The lifecycle-subscriber protocol, the
> `/v1/admin/diagnostics/*` JSON endpoints, the admin-invalidate route, the
> Prometheus `/metrics` surface, and the `sensor-cron` publisher all ship and
> are exercised by the bundled stack. The control-API mounts every route
> (including health) under a `/v1/` prefix, and event-log kinds are a typed
> `OperationalKind` proto enum so `GET /v1/events` rejects unknown `?kind=`
> values with a 400 instead of returning empty. v0.9.0 adds two pieces that
> matter here: the `work_completed` event now fires at terminal application
> (pairs with the existing `work_started`, also tx-atomic on
> heartbeat-loss-abandoned dispatch) and a labelled
> `rimsky_named_lock_acquisitions_total` counter is distinct from
> producer-claim acquisitions. A polished dashboard / lineage-query UI is not
> yet shipped (the observability backplane is in place) — operators compose
> Prometheus + their own dashboards over the JSON and metrics surfaces.

Rimsky exposes operator health signals as JSON over HTTP plus Prometheus metrics.
This page maps those surfaces — lifecycle subscribers, watchdog graphs,
control-API diagnostics, admin invalidate — and the patterns that compose them
into observability and remediation.

## Surfaces

### Lifecycle-subscriber services

A lifecycle subscriber is a peer declared in `rimsky.yml`'s
`claim_producers:` or `executors:` block — the only two blocks rimsky
dials — with `lifecycle_subscriber` alongside its primary protocol in
its `protocols` list, e.g. `protocols: [claim_producer,
lifecycle_subscriber]`. Declaring the protocol is necessary but not
sufficient: lifecycle fan-out is scoped per template to the peers that
template's nodes reference (a node's `stores:` alias or `executor:`
field). A declared subscriber that no node references receives
**nothing** — there is no global broadcast and no standalone
`subscribers:` block. To wire a domain-shaped audit log, declare the
audit service under `claim_producers:` with the `lifecycle_subscriber`
mix-in and reference it as a store on at least one node of every
template whose lifecycle it should audit; for those templates the audit
trail then composes for free.

A referenced subscriber receives the seven lifecycle events at the
points they fire: the four template events (registered / deployed /
undeployed / deregistered), the two instance events (created /
terminated), and the run-scope-terminal event (carrying the run-scope
id and a terminal reason). Template and instance events fire from the
control-api. The run-scope-terminal event fires from whichever process
closes the scope: the control-api for the instance's main run-scope
(instance delete and the terminator worker), the supervisor for
sub-graph and fanout-partition scope closes.
The events are persistently idempotent:
rimsky tracks a per-`(service, event)` ledger and fires each pair
exactly once, so a re-fire is a no-op on the consumer side.

### Watchdog graphs

A graph instance can run forever and check itself: a scheduled node
that polls a metric, queries the control API, and emits a terminal
signal that a downstream remediation node subscribes to when an alarm
condition fires. This pattern lets operators express incident response
as a graph rather than as ad hoc scripts.

The primitives that compose the watchdog:
- The standalone **`sensor-cron` publisher** fires the cadence. It is a
  Publisher-protocol service that evaluates cron expressions and POSTs a
  message envelope into the control API (`POST /v1/instances/{id}/messages`,
  `sender_kind=publisher`) on each tick; the watchdog node subscribes to
  that message. (Template-level `cron:` node fields and the admin
  force-fire route were retired — cron firing lives only in `sensor-cron`
  now.)
- A receiver-side **node subscription** chains the watchdog into a
  remediation graph: the remediation node subscribes to the watchdog
  node's settling signal — `{ node: <watchdog-type>, type: terminal/success,
  when: <CEL over the payload>, frame: next }` — and fires only when the
  watchdog's payload signals an alarm. (This replaces the retired
  send-side `on_executor_complete: { invalidate: ... }` form; propagation
  is now driven by subscriber matches against the emitted signal, not by
  the sender. See [`concepts/node-subscription.md`](../concepts/node-subscription.md).)

### Control-API polling

The control API exposes JSON endpoints suitable for polling from dashboards or
external monitors:

| Endpoint | Returns |
| --- | --- |
| `GET /v1/admin/diagnostics/held-frames` | Frames with at least one parked node. Normal during agent-driven work; persistent holds may indicate stuck reviews. |
| `GET /v1/admin/diagnostics/parked-nodes` | Parked nodes with reasons and resume timestamps. Optional `?reason=<name>` filter. (Also mounted as `GET /v1/diagnostics/parked`.) |
| `GET /v1/admin/diagnostics/wait-sets` | Wait-set rows currently blocking dispatch — what each frame is waiting on (sender run, topic, drained state). Useful for diagnosing "the cascade looks like it should fire but the node isn't running." |
| `GET /v1/events` | Paginated read of the append-only event log; filterable by `instance_id`, `node_id`, `kind`, `since`, `until`. `?kind=` is validated against the typed `OperationalKind` enum — a snake_case operational name (e.g. `state_transition`, `claim_acquired`) or a canonical slash-delimited signal type-path (`terminal/*`, `transient/*`, …); an unknown kind returns 400 listing the valid values. |
| `GET /metrics` | Prometheus text format on the per-process `RIMSKY_METRICS_PORT` (default disabled; in single-process all-in-one mode each role gets its own port, overridable per role with `RIMSKY_METRICS_PORT_<ROLE>`). **Not** under the control API's `/v1/` tree — it is its own listener with a bare `/metrics` path. Covers dispatches by terminal class, claim acquisitions by producer, named-lock acquisitions by lock name and intent (`rimsky_named_lock_acquisitions_total` — labelled distinct from producer-claim acquisitions), node-state gauges, parked-by-reason gauges, dispatch-latency histograms, and held-frame counts. |

### Admin invalidate

`POST /v1/admin/instances/{instance}/nodes/{node_id}/invalidate` is the operator
escape hatch. It dispatches by node state:

| Node state | Effect |
| --- | --- |
| `parked` | Resume with `resume_reason: "external_invalidate"`. |
| `fresh` / `stale` / `failed` | Standard invalidate (state → stale; cascade picks up next scheduler tick). |
| `running` | 409 Conflict. |

Use it when something has wedged on a signal that didn't arrive (a
review never came back, a webhook never fired). For all other states,
use the normal cascade or template-driven mechanisms.

## Patterns

### Single dashboard for held + parked + retries

Compose `held-frames`, `parked-nodes`, and the
`rimsky_dispatches_total` metric (filtered by terminal class) into one
operator dashboard. Persistent growth in any of the three signals
something to investigate.

### Per-tenant SLA observability

Templates can be tagged with the consumer they belong to. A
lifecycle-subscriber service (wired as above — declared in
`claim_producers:`/`executors:` and referenced by a node of each
template it should observe) receives that tag at template-deployed time
and can surface per-tenant rollups in the operator's monitoring stack.

### Detect retry loops with no progress

The `max_retries_without_progress` cap (default 100; 0 disables;
configurable per-node) forces `error_class: "retry_loop_no_progress"`
when the same `settling_signal_type` is observed N times in a row.
Combined with the
`rimsky_terminal_verdicts_total{error_class="retry_loop_no_progress"}`
metric, retry loops surface in alerts before they exhaust budget.

### Graceful degradation

A graph that depends on an external service can wrap the dependency in
a node whose `error_types:` chain resolves the relevant error class
to `pass` — the run settles fresh and downstream subscribers wired to
`type: terminal/error/*` for that node can react (e.g., dispatch a
"degraded mode" sibling). The graph keeps moving; the operator sees
degraded-mode telemetry and can intervene.

## What rimsky does not provide

Rimsky's surfaces are JSON over HTTP plus Prometheus metrics. Any
higher-level dashboarding (alerting rules, log aggregation, paging)
lives in the operator's existing observability stack — Grafana,
Datadog, PagerDuty, or whatever the project standardizes on.
Rimsky's role is to expose the underlying signals; the operator's
role is to compose them.
