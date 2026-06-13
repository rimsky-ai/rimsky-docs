# Troubleshoot an unreleased claim

Diagnose a claim that was opened by a node-run, never released, and is
now blocking every subsequent node that wants the same scope. This is a
**diagnosis session**: a symptom, the diagnostic command sequence, a
decision tree against the responses, and the error-catalog page that
resolves each leaf.

## Symptom

A holding subgraph (a chain of nodes that share a claim through `holds:`,
see the [claim-handoff recipe](claim-handoff.md)) is not advancing.
`rimsky instance status` reports the lead node as `succeeded`, but the
next node in the chain is `stale` and never moves. New instances
referencing the same scope refuse to acquire, with
`acquire/unavailable` on the event log.

```text
$ rimsky instance status $INST
id:            1c5b4f9a-...
state:         running
template_hash: ...

Nodes:
  ID        TYPE    STATE  ERROR_CLASS  RETRIES  LAST_HEARTBEAT
  acquirer  worker  fresh               0
  holder    worker  stale               0                            ← never dispatches
  releaser  worker  fresh               0
...
```

The claim handle is logically held by `holder` but the holder is not
running and nothing is releasing it.

## Why a claim stays held

[Claim handles](../concepts/claim-handle.md) are released on three paths:

1. **Normal terminal application** — the holding subgraph's last node
   commits or abandons; the [held-claim auto-terminal](../concepts/auto-terminal.md)
   fires once every node in the subgraph completes.
2. **Heartbeat-based abandonment** — the
   [orphan-reaper](../concepts/orphan-reaper.md) sweep finds a
   claim-handle row whose claimant supervisor has not heartbeat past
   `5 × heartbeat_timeout` and marks the row abandoned, releasing the
   underlying producer claim.
3. **Operator force-release** — none in v0.9.0; the model is auto-only.

When the holding subgraph stalls (the *holder* node cannot dispatch),
path 1 never fires. Path 2 fires only if the **supervisor** is gone, not
if the supervisor is fine but the **executor** is silent.

The two-side diagnosis is "is the holding subgraph held in a held frame,
and if so, who is failing to make progress on it?"

## Command sequence

### Step 1 — Pull held-frames

The [held-frames diagnostic](../reference/rest-api.md) lists every frame
that has at least one parked or pending-acquisition node:

```sh
curl -s http://localhost:8080/v1/admin/diagnostics/held-frames | jq
```

Example response when one frame is held:

```json
{
  "frames": [
    {
      "frame_id": "3b6e0a44-...",
      "instance_id": "1c5b4f9a-...",
      "node_ids": ["8d2f1c9e-..."],
      "held_since": "2026-06-13T14:22:01.123456789Z",
      "node_states": [
        {
          "node_id": "8d2f1c9e-...",
          "state": "parked",
          "reason": "await_callback"
        }
      ]
    }
  ]
}
```

`held_since` older than a few minutes on a node that should be making
quick progress is the smoking gun.

### Step 2 — Pull the claim-handle row for the held scope

The
[`route:GET /v1/lock-holders/{claim_handle_id}/claim-holders`](../reference/rest-api.md)
route lists every co-holder row for a claim handle. Find the handle from
the holding subgraph's claim alias (the template's `stores:` block):

```sh
HANDLE=<claim_handle_id>
curl -s "http://localhost:8080/v1/lock-holders/$HANDLE/claim-holders" | jq
```

Each row is one co-holder of the claim-handle and carries `id`,
`claim_handle_id`, `holder_run_id`, `state` (`active`, `completed`, or
`failed`), and `completed_at` (set only on terminal states). The
`holder_run_id` is the `rimsky_node_runs.id` of the run that holds the
claim through that row. The claim-handle row itself — which owns the
`holder_supervisor_id` binding — is not surfaced over HTTP in v0.9.0;
trace the run via `/v1/events?node_id=$NODE` (Step 4) to find which
supervisor most recently dispatched it.

### Step 3 — Compare against the live supervisor set

```sh
curl -s http://localhost:8080/v1/health | jq '.supervisors[] | {id, last_heartbeat_at, active_node_count}'
# Example:
#   {"id":"supervisor-abc","last_heartbeat_at":"2026-06-13T14:22:00Z","active_node_count":1}
#   {"id":"supervisor-def","last_heartbeat_at":"2026-06-13T14:18:55Z","active_node_count":0}
```

If a supervisor's `last_heartbeat_at` is recent, the supervisor is
alive — path 2 will not fire on its claims. If the only supervisor
whose `accepted_executors` covers the holder's node-type has a
heartbeat older than `5 × heartbeat_timeout` (default 75s), the
orphan-reaper should be moving — see leaf D2. (The
`/v1/lock-holders/{claim_handle_id}/claim-holders` rows do not name the
holding supervisor; the holder ↔ supervisor binding is on the
`claim_handle` itself and is not currently surfaced over HTTP — match
by claim-handle ownership through the scheduler logs if you need the
exact mapping.)

### Step 4 — Pull terminal/transient signals for the holder run

```sh
NODE=8d2f1c9e-...
curl -s "http://localhost:8080/v1/events?instance_id=$INST&node_id=$NODE" | \
  jq '.events[] | {at: .occurred_at, kind: .kind, error_class: .payload.error_class}'
```

A `terminal/error/<class>` row tells you what closed the dispatch; a
`transient/heartbeat_missed` row tells you the scheduler swept it stale
without a terminal. The classes that route to leaves below: `fs/root_unavailable`,
`pg/claim_unavailable`, `executor_blocked`, `agent/blocked`.

### Step 5 — If a producer is the source, check the producer's logs

The two bundled stores classify their own failures: the postgres store
names `pg/claim_unavailable`, and the filesystem store names
`fs/root_unavailable` (new in v0.9.0). When you see one of those classes
on the event log, the *producer* knows why — check its container logs:

```sh
docker logs rimsky-store-filesystem 2>&1 | grep -E 'fs/root_unavailable|backing root'
# Example:
#   ... msg="backing root not writable" path=/var/lib/rimsky/fs class=fs/root_unavailable err="mkdir /var/lib/rimsky/fs: read-only file system"
```

## Decision tree

```
Is the holder run in a held frame?  (Step 1 surfaced a frame, Step 2 a holder row)
├── Yes — claim is held; WHY IS THE HOLDER NOT MAKING PROGRESS?
│   ├── D1. Step 1 shows node_states[].reason = await_callback (parked)  ──→ legitimate park; await callback
│   ├── D2. Every live supervisor's last_heartbeat_at is older than       ──→ orphan-reaper should fire; if it isn't, path-2 bug
│   │       5 × heartbeat_timeout (Step 3)
│   └── D3. Step 4 shows terminal/error/<class> on the holder run         ──→ the dispatch failed but the claim is still held
└── No — the frame is NOT held; the holder is stuck-stale.
    ├── D4. Step 4 shows terminal/error/fs/root_unavailable       ──→ filesystem store backing root unavailable (v0.9.0 NEW)
    ├── D5. Step 4 shows terminal/error/pg/claim_unavailable      ──→ postgres store says the scope is unavailable
    ├── D6. Step 4 shows terminal/error/executor_blocked          ──→ executor returned a Blocked terminal
    └── D7. No signals at all                                     ──→ go to troubleshoot-stuck-stale.md
```

### D1 — Legitimate park; await the callback

A node in the holding subgraph parked on `await_callback` or `snooze` and
is waiting for an external resume. This is normal for human-in-the-loop
flows (the [park-then-resume](../concepts/parked-state.md) shape).
Resolution: deliver the resume — POST the callback URL, or admin-invalidate
to force-resume if the external system will not respond.

Resolution path:
`POST /v1/admin/instances/{instance}/nodes/{node_id}/invalidate` against
a parked node resumes it (the v0.9.0 unified-handler shape; see
[admin diagnostics route](../reference/rest-api.md)).

### D2 — Orphan reaper should fire but is not

The holder's supervisor is gone (no recent heartbeat) but the claim is
still held — the orphan-reaper sweep is either not running or is being
prevented from reaping. The reaper runs on the scheduler tick loop;
confirm the scheduler is healthy:

```sh
docker logs rimsky-scheduler 2>&1 | grep -E 'orphan_reaper|scheduler tick' | tail -10
# Expected: periodic tick lines.
```

If the scheduler is silent, restart it. If it is running but not
reaping, file the issue — this should not happen on v0.9.0; the claim
spine rewrite collapsed the per-driver guards to one written site.

Resolution page:
[`agents/errors/orphaned_claim_lost_race.md`](../agents/errors/orphaned_claim_lost_race.md) —
covers the supervisor-level claim-ownership sweep (cutoff
`5 × heartbeat_timeout`).

### D3 — Dispatch failed but the claim is still held

A `terminal/error/<class>` settled the holder dispatch, yet the
claim-handle row is still occupied. This is the cross-supervisor sub-claim
settlement stall fixed in v0.9.0 (`lib/runtime/`): the leaf-acquirer now
re-stamps the sub-claim holder so the parent's CAS-guarded handle
release succeeds. On v0.9.0+ this should not recur; if it does, file it
against rimsky-core.

Workaround: admin-invalidate the holder so the cascade re-runs the
holding subgraph from the top — the new run will acquire fresh and
abandon the stale handle through the normal terminal path.

### D4 — `fs/root_unavailable` (filesystem store, NEW in v0.9.0)

The filesystem claim-producer (`rimsky-store-filesystem:v0.9.0`) returned
a classed error `fs/root_unavailable` on the `Open` (or any other claim
verb): the configured backing root is missing or not writable at verb
time. Causes the v0.9.0 release notes call out: the path is wrong, the
volume is not mounted, the mount went read-only.

Before v0.9.0 the failure surfaced as an anonymous `500`; now it crosses
the HTTP boundary as `502 Bad Gateway` (producer failed) carrying
`producer_name`, `error_class: "fs/root_unavailable"`, and a `message`.

What to fix:

1. Confirm the filesystem store's container has the backing root
   mounted and writable:

   ```sh
   docker exec rimsky-store-filesystem ls -ld /var/lib/rimsky/fs
   # Expected: a writable directory owned by the nonroot user.
   ```

2. If the mount went read-only (kernel auto-remount on disk error,
   permission flip), repair and restart the producer container.
3. Confirm the producer's reachability with a direct GET to its admin
   surface (see [`services/README.md`](../services/README.md)) — it
   should report ready.

Resolution page:
[`agents/errors/fs_root_unavailable.md`](../agents/errors/fs_root_unavailable.md) —
covers the verb gate (`checkRootAvailable` runs at the head of every
`Open` / `Commit` / `Abandon` / `Release`), the typical operator-misconfig
causes (typo, missing mount, EROFS, EACCES), and the recommended
`error_types: { fs/root_unavailable: ... }` routing.

### D5 — `pg/claim_unavailable` (postgres store, empty queue)

The postgres claim-producer returned `Unavailable` with the
producer-declared class `pg/claim_unavailable` on the `Open` — the
queue is drained, or every row in the requested scope is already held.
The producer is healthy; rimsky is doing the right thing.

Resolution page:
[`agents/errors/acquire_unavailable.md`](../agents/errors/acquire_unavailable.md) —
covers the synthetic vs producer-declared class shapes and the
`error_types:` chain. The [queue-worker recipe](queue-worker.md) gotchas
walk the drained-queue case for the http-node worker (the
worker's executor cannot declare `pg/*` in `error_types:`, so the
fail-fast `give_up` default fires — observable as
`terminal/error/pg/claim_unavailable` on the event log).

### D6 — `executor_blocked` (executor-initiated)

An executor returned a `StreamClose.Error` with a class containing
`executor_blocked` (or a typed variant like `stub/executor_blocked` or
`agent/blocked`). The executor itself decided the work could not
proceed and named the reason in the payload. The claim release path
depends on whether the holding subgraph itself terminates — a
`give_up` resolution closes the subgraph and releases; a `pass`
resolution drops the dispatch silently and the subgraph keeps the
claim.

Resolution pages:
- [`agents/errors/agent_blocked.md`](../agents/errors/agent_blocked.md) —
  the `claude-agent` `report_blocked` path; deliberate
  agent-initiated terminal.
- Generic `executor_blocked` from a custom executor: route via
  `error_types: { executor_blocked: { policy: [pass|give_up|...] } }`
  on the node and resolve in-graph.

### D7 — No relevant signals

The holder never dispatched at all; you have a stuck-stale node masked
as an unreleased claim. Follow the
[stuck-stale walkthrough](troubleshoot-stuck-stale.md) on the holder node — the
decision tree there will route to wait-set, heartbeat-missed,
executor_dial_failed, etc.

## Force release (last resort)

There is no operator-facing claim-release endpoint in v0.9.0. The two
ways to clear a stranded claim:

1. **Admin-invalidate the holder.** Restarts the dispatch; on
   completion (success or fail), the held-claim auto-terminal releases.

   ```sh
   curl -s -X POST \
     "http://localhost:8080/v1/admin/instances/$INST/nodes/$NODE/invalidate"
   # Expected: 200 with the new run id.
   ```

2. **Force-terminate the instance.** Marks the instance terminal and
   abandons every in-flight node-run, which cascades to claim release
   via the abandonment path.

   ```sh
   rimsky instance kill $INST --force
   # Expected: "instance marked terminal".
   rimsky instance delete $INST
   # Expected: "instance deleted".
   ```

Restart should not recreate the stranded claim — the underlying
backing-root or executor problem must be fixed first, or the next
instance will park on the same scope.

## See also

- [Operational health pattern](../patterns/operational-health.md) — the
  steady-state operator playbook.
- [`concepts/claim-handle.md`](../concepts/claim-handle.md) — the
  authoritative ledger.
- [`concepts/orphan-reaper.md`](../concepts/orphan-reaper.md) — the
  abandonment-by-heartbeat path.
- [`concepts/auto-terminal.md`](../concepts/auto-terminal.md) — the
  held-claim release-on-subgraph-completion path.
- [Claim handoff recipe](claim-handoff.md) — the `holds:` chain shape
  this walkthrough debugs.
- [Stuck-stale walkthrough](troubleshoot-stuck-stale.md) — when leaf D7
  routes you back there.
- [`agents/errors/README.md`](../agents/errors/README.md) — the full
  error catalog index.
