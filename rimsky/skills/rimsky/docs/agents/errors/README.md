# Error catalog

One file per consumer-observable error class — or per mechanism family, where one code path emits several related classes (e.g. the `verifier/check_failed` family, the `http-node` unexpected-status family). Each file states what the error means, when it happens, and what to do.

*Internal-correctness errors* (state-machine rejections, sweep-internal errors, advisory-lock failures) are not listed here — they are not consumer-observable.

Each page's frontmatter carries `surfaced_to:` — the audience that first observes the error and owns the response:

| Value | Who that is |
| --- | --- |
| `operator` | Whoever runs/configures the deployment and its templates; the error surfaces through the event log / `error_types:` policy chain |
| `cli-user` | A caller of the `rimsky` CLI or control API; the error comes back synchronously on the request |
| `executor` | An executor implementer; the error indicates the executor's own protocol/contract behavior needs fixing |
| `claim-producer` | A claim-producer implementer (same idea, for the ClaimProducer protocol) |
| `lifecycle-subscriber` | A lifecycle-subscriber implementer |

## Producer-declared error vocabularies

Producer-side error classes (`fs/root_unavailable`, `pg/claim_unavailable`, etc.) are not invented by rimsky — they live in the producer's own `Capabilities()` response, on `proto:claim_producer.proto::CapabilitiesResponse.declared_error_classes` (added in rimsky v0.9.0). A leaf surfaces on `google.rpc.ErrorInfo.Reason` for a faulted producer verb, OR on `Unavailable.error_class` for an acquisition refusal. The template validator's `error_types:` range-check accepts a key if it matches any declared plain leaf or matches a declared `<prefix>/*` pattern; an `error_types:` key attributable to no declared vocabulary surfaces as an advisory warning at template registration, never a hard rejection. The errors catalog mirrors what each producer declares — when a producer advertises a new leaf, it gets a page here. Mirrors `proto:executor_observability.proto::ObservabilityCapabilities.declared_error_classes` for the executor surface.

## Index

- [`acquire_unavailable.md`](acquire_unavailable.md) — claim producer's `Open` returned `Unavailable`; routed through the node's `error_types: { acquire/unavailable: ... }` chain (or a producer-declared leaf when the producer named one, e.g. `pg/claim_unavailable`).
- [`fs_root_unavailable.md`](fs_root_unavailable.md) — bundled `store-filesystem` producer rejected a verb (`Open` / `Commit` / `Abandon` / `Release`) because its configured backing root is missing or not writable; operator-misconfiguration case (`fs/root_unavailable`).
- [`orphaned_claim_lost_race.md`](orphaned_claim_lost_race.md) — supervisor lost ownership of a claim mid-execution.
- [`capability_envelope_mismatch.md`](capability_envelope_mismatch.md) — operator-declared envelope is not a subset of producer-advertised envelope.
- [`tag_shape_rejected.md`](tag_shape_rejected.md) — a tag identifier in the `sha256-<64-hex>` shape was rejected.
- [`compose_prefix_violation.md`](compose_prefix_violation.md) — `POST /v1/tags` or `POST /v1/instances` from a non-privileged caller used the reserved `compose:` prefix; rejected server-side with HTTP 400.
- [`template_not_deployed.md`](template_not_deployed.md) — instance creation against a registered-but-not-deployed template.
- [`instance_static_config_violation.md`](instance_static_config_violation.md) — `POST /v1/instances` instantiation-time static-config gate rejected the template's L1 + L2 attribute defaults against the referenced executor's `expected_attributes_schema`; HTTP 400 with a structured `validation_errors` body.
- [`stub_mode_probe_failed.md`](stub_mode_probe_failed.md) — conformance run rejected a non-stubbed executor.
- [`async_callback_wrong_key.md`](async_callback_wrong_key.md) — async-callback body was not a valid `AsyncCallbackBody` (legacy `{type: ...}` shape, or not exactly one outcome).
- [`attribute_validation_failed_at_dispatch.md`](attribute_validation_failed_at_dispatch.md) — substituted attributes failed schema validation at dispatch (`template_validation_failed`).
- [`executor_schema_unavailable.md`](executor_schema_unavailable.md) — the executor's `expected_attributes_schema` was not visible at dispatch, so the node's effective attribute schema could not be computed (`executor_schema_unavailable`).
- [`attribute_validation_failed_at_commit.md`](attribute_validation_failed_at_commit.md) — executor `attributes_delta` failed supervisor-side schema validation at commit (`attributes_schema_failed`).
- [`agent_schema_violation.md`](agent_schema_violation.md) — `claude-agent` executor-side schema validation of the agent's proposed `attributes_delta` failed past `cli.max_schema_corrections`; terminal `agent/schema_violation`.
- [`signoff_unobtained.md`](signoff_unobtained.md) — `claude-agent` sign-off gate (`cli.required_signoffs`) unmet within `cli.max_signoff_attempts`; terminal `agent/signoff_unobtained`.
- [`agent_attribute_invalid.md`](agent_attribute_invalid.md) — `claude-agent` dispatch-time attribute-contract rejection (invalid schema, bad `cwd`/`cwd_from_store`, malformed `cli.mcp_servers` / `cli.required_signoffs`); the CLI never spawned (`agent/attribute_invalid`).
- [`agent_blocked.md`](agent_blocked.md) — the agent itself called `report_blocked`; deliberate agent-initiated terminal (`agent/blocked`).
- [`agent_subprocess_exit.md`](agent_subprocess_exit.md) — the `claude-agent` CLI-run failure family: spawn failure, silence timeout, opted-out rate limit, stderr-classified exits, and the exit-without-`report_complete` fallback (`agent/cli_spawn_failed`, `agent/timeout`, `agent/rate_limited`, `agent/context_exceeded`, `agent/refused`, `agent/tool_use_failed/*`, `agent/subprocess_exit/*`).
- [`agent_internal_error.md`](agent_internal_error.md) — the `claude-agent` executor's own unhandled-exception catch-all (`agent/internal_error`).
- [`verifier_attribute_invalid.md`](verifier_attribute_invalid.md) — a bundled verifier executor's attribute bag was missing required keys or malformed; the check never ran (`verifier/attribute_invalid`).
- [`verifier_check_failed.md`](verifier_check_failed.md) — the verification itself failed: `verifier/check_failed` (untyped fallback, `verifier-http`) plus the `verifier/check_failed/*` typed leaves (upstream `class_field` token for `verifier-http`; check `kind` for `verifier-shape-checks`).
- [`verifier_network_error.md`](verifier_network_error.md) — `verifier-http` non-timeout transport failure reaching the verifier endpoint; verdict unknown (`verifier/network_error`).
- [`verifier_timeout.md`](verifier_timeout.md) — `verifier-http` request exceeded `timeout_ms` or hit a transport `Timeout()`; verdict unknown (`verifier/timeout`).
- [`http_attribute_invalid.md`](http_attribute_invalid.md) — the `http-node` executor's attribute bag violated its contract (missing `url`, unserialisable `body`, unbuildable request); no request sent (`http/attribute_invalid`).
- [`http_network_error.md`](http_network_error.md) — `http-node` non-timeout transport failure; no response arrived, upstream verdict unknown (`http/network_error`).
- [`http_timeout.md`](http_timeout.md) — `http-node` request exceeded the executor timeout or hit a transport `Timeout()`; upstream verdict unknown (`http/timeout`).
- [`http_unexpected_status.md`](http_unexpected_status.md) — `http-node` got a status outside `expect_status`: upstream-typed 4xx leaves, per-status 5xx leaves, and the taxonomy-less fallback (`http/request_invalid/*`, `http/server_error/*`, `http/expectation_mismatch`).
- [`http_response_unparseable.md`](http_response_unparseable.md) — `http-node` got an expected status but the body is not a JSON object, so no `attributes_delta` could be built (`http/response_unparseable`).
- [`http_internal_error.md`](http_internal_error.md) — the `http-node` executor's own dispatch handling failed; executor-bug catch-all (`http/internal_error`).
- [`executor_dial_failed.md`](executor_dial_failed.md) — the supervisor could not dial the executor endpoint or open the `Execute` stream (infra error; re-enqueued, emits `terminal/infra/executor_dial_failed`).
- [`build_request_failed.md`](build_request_failed.md) — constructing the `ExecuteRequest` failed, typically a producer returning non-JSON claim bytes (infra error; re-enqueued, emits `terminal/infra/build_request_failed`).
- [`stream_error.md`](stream_error.md) — the executor's `Execute` stream errored mid-dispatch before a terminal arrived (infra error; re-enqueued, emits `terminal/infra/stream_error`).
- [`stream_closed_without_terminal.md`](stream_closed_without_terminal.md) — the executor cleanly closed the `Execute` stream (EOF) without sending a terminal outcome — an executor contract violation (infra error; re-enqueued, emits `terminal/infra/stream_closed_without_terminal`).
- [`retry_loop_no_progress.md`](retry_loop_no_progress.md) — a retry policy kept firing without progress past the effective `max_retries_without_progress` cap; the runtime forced a give-up verdict (`retry_loop_no_progress`, payload carries `original_error_class`).
- [`heartbeat_missed.md`](heartbeat_missed.md) — a running node's executor heartbeats went silent past the timeout; the scheduler sweep emits the `transient/heartbeat_missed` signal, marks the node stale, and re-enqueues a recovery dispatch.
- [`unresolved_executor.md`](unresolved_executor.md) — node references an executor that is not configured in `rimsky.yml`.
- [`schedule_cron_parse_failure.md`](schedule_cron_parse_failure.md) — the `sensor-cron` publisher could not parse a supplied cron expression.
- [`schedule_dispatch_failed.md`](schedule_dispatch_failed.md) — a `sensor-cron` fire could not be delivered to the control plane (retried on the next tick).
