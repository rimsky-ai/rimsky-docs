---
error: instance_static_config_violation
surfaced_to: cli-user
---

# Instance static-config validation failed

## What it means

`POST /v1/instances` ran the instantiation-time static-config gate against the named template; one or more nodes' statically-knowable attribute values (template `defaults:` ∪ per-node `default:` literals) failed validation against the referenced executor's `expected_attributes_schema` (the per-executor JSON-Schema slice of the node's effective attribute schema — see [`../../concepts/attribute.md`](../../concepts/attribute.md)). The instance was not created. The response is **always** HTTP 400 with both `error` and `validation_errors[]` present (the gate stops at the first violation, so `validation_errors[]` carries exactly one entry):

```json
{
  "error": "template validation failed",
  "validation_errors": [
    {
      "path": "nodes[<node-type>].attributes",
      "msg": "<JSON-Schema violation message>"
    }
  ]
}
```

The wrapped error is `ErrTemplateValidation` (same sentinel as the registration-time validation chain), but the 400 status distinguishes this *instance-time* failure from registration-time validation (which returns 409 for template state-machine conflicts).

## When it happens

`POST /v1/instances` is the **mandatory** static-config gate. Registration-time validation may have been deferred under a relaxed mode (operator-wide `cfg:templates.ref_validation_mode` or its `env:RIMSKY_REF_VALIDATION_MODE` override, set to `available` or `none`) — for example because the executor was not yet provisioned, or the operator wanted to register the template before wiring the executor. Whatever a relaxed mode skipped is NOT skipped forever — by the time `POST /v1/instances` runs, the template is deployed and the referenced services exist, so the gate validates the statically-knowable subset every time, regardless of how registration was configured.

The gate validates ONLY the statically-knowable subset: composed L1 template defaults ∪ L2 node-declared `default:` literals. Substitution-sourced values (`source:`-bound and `{{...}}` directive values) are knowable only once a node acquires its inputs, so they continue to validate at dispatch (the **validate-twice** rule — see the *Invariants* section of [`../../concepts/attribute.md`](../../concepts/attribute.md)).

Common causes:

- Template registered under `cfg:templates.ref_validation_mode: available` or `none` (or its `env:RIMSKY_REF_VALIDATION_MODE` override) against an executor that was not yet provisioned, and the static config does not in fact validate.
- The executor's schema changed after the template was registered (operator upgraded the executor); the template's static config now violates the new schema.
- A `default:` literal was added to a node attribute that does not validate against the executor's schema.

## What to do

Read the `validation_errors[]` array — each entry names the offending node and the violated JSON-Schema constraint (e.g. `minimum`, `required`, `type`). Either:

- Fix the template's `default:` literal so it conforms to the executor's schema, then re-register and re-deploy.
- Fix the executor's `expected_attributes_schema` if the schema is wrong (then redeploy the executor; the next `POST /v1/instances` will re-validate).

Do not work around this by registering against a different (looser) schema and hoping dispatch-time validation catches drift — the gate exists precisely to catch the static drift early, before per-dispatch failures accumulate.

## See also

- [`../../concepts/template.md`](../../concepts/template.md)
- [`../../concepts/instance.md`](../../concepts/instance.md)
- [`../../operator-guide.md#template-registration-and-reference-validation`](../../operator-guide.md#template-registration-and-reference-validation) — the operator-side `templates.ref_validation_mode` knob and `available` / `none` relaxation modes.
- [`../../concepts/attribute.md`](../../concepts/attribute.md)
- [`attribute_validation_failed_at_dispatch.md`](attribute_validation_failed_at_dispatch.md) — the dispatch-time pass for substitution-sourced values.
