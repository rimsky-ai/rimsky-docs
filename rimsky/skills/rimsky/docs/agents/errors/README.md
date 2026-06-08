# Error catalog

One file per consumer-observable error. Each file states what the error means, when it happens, and what to do.

*Internal-correctness errors* (state-machine rejections, sweep-internal errors, advisory-lock failures) are not listed here — they are not consumer-observable.

## Index

- [`acquire_unavailable.md`](acquire_unavailable.md) — claim producer's `Open` returned `Unavailable`; routed through the node's `error_types: { acquire/unavailable: ... }` chain (or a producer-declared leaf when the producer named one, e.g. `pg/claim_unavailable`).
- [`orphaned_claim_lost_race.md`](orphaned_claim_lost_race.md) — supervisor lost ownership of a claim mid-execution.
- [`capability_envelope_mismatch.md`](capability_envelope_mismatch.md) — operator-declared envelope is not a subset of producer-advertised envelope.
- [`tag_shape_rejected.md`](tag_shape_rejected.md) — a tag identifier in the `sha256-<64-hex>` shape was rejected.
- [`compose_prefix_violation.md`](compose_prefix_violation.md) — `POST /tags` or `POST /instances` from a non-privileged caller used the reserved `compose:` prefix; rejected server-side with HTTP 400.
- [`template_not_deployed.md`](template_not_deployed.md) — instance creation against a registered-but-not-deployed template.
- [`instance_static_config_violation.md`](instance_static_config_violation.md) — `POST /instances` instantiation-time static-config gate rejected the template's L1 + L2 attribute defaults against the referenced executor's `expected_attributes_schema`; HTTP 400 with a structured `validation_errors` body.
- [`stub_mode_probe_failed.md`](stub_mode_probe_failed.md) — conformance run rejected a non-stubbed executor.
- [`async_callback_wrong_key.md`](async_callback_wrong_key.md) — async-callback body was not a valid `AsyncCallbackBody` (legacy `{type: ...}` shape, or not exactly one outcome).
- [`attribute_validation_failed_at_dispatch.md`](attribute_validation_failed_at_dispatch.md) — substituted attributes failed schema validation at dispatch (`template_validation_failed`).
- [`executor_schema_unavailable.md`](executor_schema_unavailable.md) — the executor's `expected_attributes_schema` was not visible at dispatch, so the node's effective attribute schema could not be computed (`executor_schema_unavailable`).
- [`attribute_validation_failed_at_commit.md`](attribute_validation_failed_at_commit.md) — executor `attributes_delta` failed supervisor-side schema validation at commit (`attributes_schema_failed`).
- [`agent_schema_violation.md`](agent_schema_violation.md) — `claude-agent` executor-side schema validation of the agent's proposed `attributes_delta` failed past `cli.max_schema_corrections`; terminal `agent/schema_violation`.
- [`signoff_unobtained.md`](signoff_unobtained.md) — `claude-agent` sign-off gate (`cli.required_signoffs`) unmet within `cli.max_signoff_attempts`; terminal `agent/signoff_unobtained`.
- [`executor_dial_failed.md`](executor_dial_failed.md) — the supervisor could not dial the executor endpoint or open the `Execute` stream (infra error; re-enqueued, emits `terminal/infra/executor_dial_failed`).
- [`build_request_failed.md`](build_request_failed.md) — constructing the `ExecuteRequest` failed, typically a producer returning non-JSON claim bytes (infra error; re-enqueued, emits `terminal/infra/build_request_failed`).
- [`stream_error.md`](stream_error.md) — the executor's `Execute` stream errored mid-dispatch before a terminal arrived (infra error; re-enqueued, emits `terminal/infra/stream_error`).
- [`stream_closed_without_terminal.md`](stream_closed_without_terminal.md) — the executor cleanly closed the `Execute` stream (EOF) without sending a terminal outcome — an executor contract violation (infra error; re-enqueued, emits `terminal/infra/stream_closed_without_terminal`).
- [`heartbeat_lost.md`](heartbeat_lost.md) — supervisor or executor heartbeat went silent past the timeout.
- [`unresolved_executor.md`](unresolved_executor.md) — node references an executor that is not configured in `rimsky.yml`.
- [`schedule_cron_parse_failure.md`](schedule_cron_parse_failure.md) — the `sensor-cron` publisher could not parse a supplied cron expression.
- [`schedule_dispatch_failed.md`](schedule_dispatch_failed.md) — a `sensor-cron` fire could not be delivered to the control plane (retried on the next tick).
