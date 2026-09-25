# P6 worker-loss recovery is deferred

The `p7-recovery-prep` brief scopes the rehearsal to "process restart,
dependency outage, graceful drain, backup-and-restore". A later
**integration check** is required to cover what happens when a worker
that holds a websocket / long-poll / audio-streaming lease simply
disappears (P6 worker loss).

That check needs:

1. A running P6 worker with an in-flight lease against a citizen session.
2. A `kill -9` (or equivalent OOM event) on the worker process.
3. Verification that the lease is reaped within a bounded lease-TTL
   window (not the `srv.Shutdown` 10s window).
4. Verification that the citizen-side stream can be retried against the
   replacement worker, with idempotency boundaries intact.

That check is **not** in scope for `p7-recovery-prep` because:

- The P6 worker modules (`p6-asr`, `p6-tts`, `p6-middle`,
  `p6-orchestration`, `p6-context`, `p6-evaluation`) are still in
  worker-per-feature slices that have not been integration-rolled.
- A meaningful worker-loss rehearsal requires an integration package
  that itself is owned by an upstream worker.

It is recorded here so subsequent coordinators do not need to rediscover
the gap.
