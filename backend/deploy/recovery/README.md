# P7 deployment-rehearsal artifacts (worker `p7-recovery-prep`)

These files are the deploy-side complement to `backend/scripts/recovery/`.
The scripts prove the data plane (backup, restore, outage, drain); this
directory records how those proofs could be wired against a real
disposable deployment, while documenting what does and does not exist at
P7-prep time.

## Files

```
deploy/recovery/
  README.md                  # this file
  NOT_RUN.failover.md        # why true HA failover is NOT_RUN at P7-prep
  worker-loss-recovery.md    # P6 worker-loss recovery is deferred
  compose.recovery.yml       # disposable single-host rehearsal stack
  cloud-init.example         # how to bootstrap the disposable host
```

## What lives here vs. what lives in `scripts/recovery/`

| Concern                     | scripts/recovery/                | deploy/recovery/                  |
| ---                         | ---                              | ---                                |
| Backups                     | `run-backup-restore.sh`          | `compose.recovery.yml`             |
| Restore verification        | `run-backup-restore.sh`          | `compose.recovery.yml`             |
| Dependency outage           | `run-dep-outage.sh`              | `NOT_RUN.failover.md`              |
| Graceful drain              | `run-graceful-drain.sh`          | `cloud-init.example`               |
| Time-measurement record     | `timings.jsonl` (each runner)    | roll-up notes in `README.md`       |
| True failover (HA)          | n/a                              | `NOT_RUN.failover.md`              |
| Cross-process / worker-loss | `crash_process_test.go` (P4)     | `worker-loss-recovery.md`          |

## Why this directory is small

The brief warned against spending cycles producing a deploy artifact the
team cannot run today. We document the shape (`compose.recovery.yml`,
`cloud-init.example`) but mark anything that requires an actual cluster
(NOT_RUN). The scripts that exist already prove the data plane on a
locally owned cluster; promotion to an actually disposable environment
is a deploy/allocation step the coordinator owns.

## Promotion checklist (P7-prep → P7 body)

- A disposable staging environment is provisioned (regulator-or-isolated
  cloud project; not the team's primary project).
- A streaming-replica topology is available; `pg_basebackup` and
  `pg_ctl promote` exercises are runnable. Without them, true failover
  remains NOT_RUN.
- `compose.recovery.yml` is invoked end-to-end against the disposable
  environment, with timings attached to a service-owned dashboard
  (separate from this repo).
- P6 worker-loss benchmarks complete and join these timings.
- O09 recovery budgets approve the proposal stage into the gate.
- Then this directory and `scripts/recovery/` graduate from `prep` to a
  scheduled run, not a one-shot rehearsal.
