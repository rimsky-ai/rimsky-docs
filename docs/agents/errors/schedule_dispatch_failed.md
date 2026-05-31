---
error: schedule_dispatch_failed
surfaced_to: operator
---

# Schedule dispatch failed

## What it means

A scheduled fire from the cron sensor could not be delivered: posting the observation message to the control plane failed. The fire is not lost — the sensor does not advance `next_fire_at`, so the same fire is retried on the next tick.

## When it happens

When the cron sensor's fire path cannot post its message envelope (for example the control plane is unreachable, returns an error, or rejects the message). The sensor logs `sensor-cron.message_post_failed` and leaves the watch's `next_fire_at` unchanged so the next tick retries the same fire window. Because the idempotency key is `subscription_id + fire-window`, a retry within the same window dedupes server-side rather than double-firing.

## What to do

Check that the control plane is reachable from the sensor and that the target node and message kind are valid. Inspect the sensor logs for the `sensor-cron.message_post_failed` entry and its `error` field. Once the underlying delivery problem clears, the next tick delivers the pending fire automatically.

## See also

- [`../../concepts/sensor.md`](../../concepts/sensor.md)
- [`../../concepts/publisher-subscription.md`](../../concepts/publisher-subscription.md)
