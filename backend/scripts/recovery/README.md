# P7 recovery — backup, restore, dependency-outage, graceful-drain

This directory is the **preparation** layer for P7 lifecycle
rehearsals. It contains:

- `versions.env` — pinned tooling versions and snapshot date.
- `_lib.sh` — shared helpers + the safety invariants:
  - every database this directory touches must be prefixed with
    `r7recover_` or already in `CURRENT_OWNED_DBS`;
  - the default `forbid_shared_target` refuses to talk to the team's
    shared PostgreSQL instance (port 5432, default-user). Operators
    must set `RECOVERY_ALLOW_SHARED=1` to override (never on CI);
  - every runner brings up a uniquely-named `initdb` cluster on a
    high-numbered port (24000-27999 random) and tears it down on
    EXIT (success, error, or signal);
  - `drop_owned_db` refuses anything not `r7recover_*`-prefixed.
- `run-backup-restore.sh` — full source-→-pg_dump-→-separate-DB
  verification + reconnect to a real `cmd/sthira` + measure
  timings.
- `run-dep-outage.sh` — owned cluster ↔ real server; proves a
  dependency outage flips `/health/ready` and re-flips when the
  cluster returns; measures both windows.
- `run-graceful-drain.sh` — SIGTERM against a running `cmd/sthira`;
  proves the process exits within `ShutdownTimeout + slack` and
  refuses new connections in the drain window.

The narrowly-scoped Go integration test lives next to the code it
exercises:

- `internal/store/recovery_integration_test.go` — gated on
  `recoverytest` build tag + `STHIRA_RUN_RECOVERY=1`; runs the same
  DB-level invariants through a real `pg_dump` + `pg_restore`,
  without spawning `cmd/sthira` (the runners do that).

## Why this directory is small

The brief forbids overlapping scripts and unverifiable runbooks. We
contribute three disjoint rehearsals, each of which produces real
timings and verifiable artifacts. Anything that cannot be proven on
a locally owned cluster today (e.g. true HA failover, P6 worker-loss
recovery) is documented under `backend/deploy/recovery/` as
`NOT_RUN.failover.md` and `worker-loss-recovery.md`.

## Usage

```sh
# 1. Backup + restore + reconnect real server.
bash scripts/recovery/run-backup-restore.sh

# 2. Dep outage + restart.
bash scripts/recovery/run-dep-outage.sh

# 3. SIGTERM drain.
bash scripts/recovery/run-graceful-drain.sh

# 4. (Optional) the Go integration counterpart against an environment
#    that already has a shared DSN. NEVER set this on CI without a
#    coordinator allocation:
#    STHIRA_TEST_DSN='host=127.0.0.1 port=25432 user=foo dbname=postgres' \
#      STHIRA_RUN_RECOVERY=1 \
#      go test -tags recoverytest ./internal/store/...
```

## Cleanup guarantees

Every runner traps EXIT (and INT/HUP/TERM) and:

1. Stops only the cluster it started (via `pg_ctl ... stop -m fast`
   on its own `OWNED_DATA_DIR`; never on the team's path).
2. Drops only `r7recover_*`-prefixed databases it created (and only
   in the allowlist `CURRENT_OWNED_DBS`).
3. Removes its own `SCRATCH_DIR` under `$RECOVERY_TMP`.

If a tool fails (pg_dump nonzero, restore errored, server SIGKILLed),
the same trap path runs; cleanup is independent of the run's success.

## Hard limits (refusing to cross them)

- **Never** connect to a database that is not `r7recover_*`-prefixed.
- **Never** accept port 5432 as the default for this directory's
  runners (the team's PostgreSQL instance runs there).
- **Never** `pg_ctl stop -m immediate` without
  `RECOVERY_ALLOW_IMMEDIATE=1` set; `immediate` does not flush shared
  buffers and would not represent a graceful failover.
- **Never** relabel restart as HA — restart keeps the same data; HA
  promotes a replica. See `deploy/recovery/NOT_RUN.failover.md`.
