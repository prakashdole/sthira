# Why true PostgreSQL HA failover is `NOT_RUN` at P7-prep

A backup-and-restore rehearsal is **not** the same as a database
failover. The clear distinction matters because both appear in incident
postmortems, but they have different recovery-time budgets and different
operational ownership.

## What we did

`scripts/recovery/run-backup-restore.sh` and the corresponding Go
integration test in `internal/store/recovery_integration_test.go` prove:

- schema revision, audit-chain head, idempotency replay state,
  capacity, FK relationships all survive a `pg_dump -Fc` → fresh DB →
  `pg_restore` round-trip.
- the backend process reconnects to the freshly restored DB and reports
  `/health/ready=200`.

That is a backup-and-restore exercise; the worst case is "promote the
backup", and the recovery time is bounded by dump size + restore time +
process restart.

## What we did NOT do

A real HA failover requires at minimum:

1. A streaming replica attached to the primary.
2. `pg_stat_replication` showing the replica is caught up.
3. A `pg_ctl promote` on the replica after the primary is unreachable,
   plus a clients-point-at-the-new-leader flow (DNS, pgbouncer, or
   connection-pool ha-mode).
4. Verification that production-path latency (writes, reads, snapshots)
   holds within the agreed budget.

None of those run at P7-prep because:

- No replica topology is provided at the P7-prep moment.
- Promoting a replica on the team's shared PostgreSQL instance is
  explicitly out of scope and would risk the broader environment.
- Coordinator-owned provisioning decisions are required to enable it
  (see plan/open-decisions.md → O04: hosting/budget; O09: availability
  and restore budgets).

## Distinct evidence we WILL keep recording

| Failure class          | How we measure                       | How it differs from failover |
| ---                    | ---                                  | ---                          |
| Backup restoration     | `scripts/recovery/run-backup-restore.sh` dump+restore timings | full file copy; expected RTO ~ minutes to ~hour on representative volumes |
| Process restart        | `scripts/recovery/run-graceful-drain.sh` drain timings | same data, same cluster, same path; expected RTO ~10s (ShutdownTimeout) |
| Dep outage (proxy for replica unavailability) | `scripts/recovery/run-dep-outage.sh` DB stop + restart | shared cluster still in scope of one Postgres instance, no replica |
| True HA failover       | NOT_RUN                              | requires a promoted replica; out of scope at P7-prep |

## What we would do if a replica were available

If the coordinator allocated a single-replica disposable topology:

1. Apply `backend/migrations/*.sql` to the replica lag (slot
   subscriber) instead of starting cold.
2. Set up a `p1.failover.sh` driver that:
   - captures write traffic on the primary,
   - `pg_ctl promote`s the replica,
   - captures write traffic on the new primary,
   - confirms no committed-write loss and an acceptably bounded
     committed-after-down (window depends on replica lag).
3. Wire that into a recurring runbook, owned by operations, gated on
   stakeholder sign-off on real downtime.

Each of those steps would be its own script + its own narrowly-scoped
test (still gated on `STHIRA_RUN_HA=1`); they do not live at P7-prep.
