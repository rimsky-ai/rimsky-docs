---
error: schedule_cron_parse_failure
surfaced_to: cli-user
---

# Schedule cron parse failure

## What it means

The cron sensor could not parse the cron expression supplied for a schedule. The expression is rejected and no schedule is registered.

## When it happens

When a cron sensor subscription resolves a `cron` config that is empty or not a valid standard cron expression. The sensor's `Subscribe` rejects the request and returns an `invalid cron "<expr>"` error to the caller, so a malformed expression in a template or subscription config fails fast at subscription time. If a previously-registered expression somehow becomes unparseable at fire time, the sensor logs `sensor-cron.cron_parse_failed` and skips the fire without advancing.

## What to do

Fix the cron expression to a valid standard 5-field form (for example `0 * * * *` for hourly). Confirm the `cron` field is present and non-empty in the schedule config, then re-submit.

## See also

- [`../../concepts/sensor.md`](../../concepts/sensor.md)
- [`../../concepts/publisher-subscription.md`](../../concepts/publisher-subscription.md)
