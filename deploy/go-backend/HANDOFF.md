# Worker 3 — Go backend deployment package: handoff

This document is the operator-facing notes for the deliverable built under
`deploy/go-backend/` on branch `codex/go-deployment-package`. Worker 1
integrates these commits after its own code corrections. Operational
approval remains a separate decision owned by the deployment owner; see
`NOT_RUN.md` for items deliberately left out of scope.

- **Base SHA** (commit this package branched from on top of `CLEAN`):
  `53302b2` (`docs(plan): add deployment and scenario preparation build lanes`).
- **Branch / worktree**: `codex/go-deployment-package` at
  `/private/tmp/mz-worktrees/go-deployment-package`.
- **New commits (oldest first, on top of `53302b2`)** — Worker 1 may
  cherry-pick these individually in this order:
  - `4a825cb` — image, compose, and bounded lifecycle scaffolding
  - `c197623` — explicit migrate, backup, and isolated restore
  - `258fe97` — operator handoff and NOT_RUN register
  - `917ac82` — Dockerfile builds sthmigrate and pgdsn-env from the right module
  - `d22308b` — simplify Dockerfile to single-module build with both binaries
- **Toolchain pin** (recorded in `backend/go.mod`):
  - Go: `1.27.1` (Dockerfile `ARG GO_VERSION` defaults to the same).
  - Database image: `postgis/postgis:16-3.4` (matches the migrations'
    P3–P7 + Publication claims of PostgreSQL 15+ with PostGIS 3.x and the
    pre-P7 rehearsal's proven PostgreSQL 16 baseline).
- **Image-distroless runtime**: `gcr.io/distroless/static-debian12:nonroot`
  on UID `65532`.
- **Schema target**: the binary expects `SchemaRevision == 7`
  (`backend/internal/store/store.go:44`), which the staged migration
  files 0001–0007 collectively produce.

## What this package contains

```
deploy/go-backend/
  Dockerfile                  # multi-stage; pinned 1.27.1 builder + distroless runtime
  docker-compose.yml          # api + postgis; loopback-only API; named pgdata volume
  .env.example                # schema for the local ignore'd .env
  HANDOFF.md                  # ← this file
  NOT_RUN.md                  # what this package does NOT do (and why)
  bin/
    env-preflight.sh          # required-configuration guard
    build-image.sh            # stages a build context + builds the image
  scripts/
    go-build.sh               # compose + build the image
    go-start.sh               # bring up api + postgres containers
    go-status.sh              # container state + /health/* probes
    go-logs.sh                # tail container logs
    go-stop.sh                # stop containers (--purge for destructive cleanup)
    go-migrate.sh             # one-shot explicit migration runner
    go-backup.sh              # explicit pg_dump backup
    go-restore.sh             # explicit pg_restore into isolated target
  migrate/                    # go module — stdlib only, no third-party deps
    go.mod
    cmd/sthmigrate/           # one-shot migration runner binary
      main.go
      main_test.go
    cmd/pgdsn-env/            # tiny libpq URI -> env helper used by go-backup.sh
      main.go
```

The `migrate/` module is a standalone Go module (no third-party
dependencies; only the standard library) so adding the migration binary
to the deployment does not require any change to `backend/go.mod`. The
backend module is owned by Worker 1 in `plan/next-two-worker-1.md` and
must remain untouched by this package.

## Operator quickstart

### Prerequisites

- Docker Engine ≥ 24 with Compose v2.
- A POSIX shell (`bash`, `zsh`) — every lifecycle script uses POSIX
  features but is shellcheck-clean.
- A reachable loopback port for the API (default `8080`; override with
  `STHIRA_API_HOST_PORT`).
- `pg_dump` is NOT a host prerequisite in the docker path; the package
  runs `pg_dump` from `postgis/postgis:16-3.4` so the dump tool's
  version always matches the server. The host fallback exists for
  CI/operators without Docker.

### First-time setup

```sh
cd /path/to/MonitoringZ
git switch codex/go-deployment-package  # or use the integration branch once Worker 1 lands

cp deploy/go-backend/.env.example deploy/go-backend/.env
chmod 600 deploy/go-backend/.env
$EDITOR deploy/go-backend/.env           # set STHIRA_PG_PASSWORD and update the DSN host/port

./deploy/go-backend/scripts/go-build.sh   # stages a build context and builds the image
./deploy/go-backend/scripts/go-start.sh   # brings up postgis + api containers
./deploy/go-backend/scripts/go-migrate.sh up   # explicit one-shot migration
./deploy/go-backend/scripts/go-status.sh       # confirm /health/ready is 200
```

### Voice pipeline extension points (optional)

The API service wires its P6 voice orchestrator only when **all three**
of `STHIRA_ASR_URL`, `STHIRA_MIDDLE_URL`, `STHIRA_TTS_URL` are set
*and* the database is reachable. Tokens are optional per stage. When
unconfigured the `POST /api/v3/voice/commands` endpoint fails closed
with `503` — the documented posture when no operational voice workers
exist.

The deployment package **does not** bundle ASR, middle, or TTS workers,
model weights, or vLLM/vLLM-style inference images. Those modules are
owned by Worker 2 (`plan/next-two-worker-2.md`); only their URLs are
exposed here as the package's extension point. Until those modules
exist as deployable artifacts in this repository, leaving the worker
URLs unset is the correct default.

### Lifecycle commands

| Command                                  | Effect                                        |
| ---------------------------------------- | --------------------------------------------- |
| `scripts/go-build.sh`                    | Build the image with the pinned toolchain.    |
| `scripts/go-start.sh`                    | Bring up postgis + api. Idempotent.           |
| `scripts/go-status.sh`                   | Container state + `/health/live`, `/health/ready`, applied migrations. |
| `scripts/go-logs.sh [api\|postgres]`     | Tail container logs.                          |
| `scripts/go-stop.sh [--purge]`           | Stop containers. `--purge` requires `STHIRA_PURGE_CONFIRM=y` and also drops the `pgdata` named volume. |
| `scripts/go-migrate.sh up [N]`           | One-shot explicit migration. `N` is optional `--target` revision; defaults to head. |
| `scripts/go-migrate.sh status`           | Print applied revisions and pending files.    |
| `scripts/go-migrate.sh verify`           | Parse pending files without committing (ROLLBACK per file). |
| `scripts/go-backup.sh [--force]`         | `pg_dump` to `${STHIRA_BACKUP_DIR:-/var/backups/<project>}/<project>-<ts>.dump`. |
| `scripts/go-restore.sh --archive=PATH`   | Restore into a freshly-created `<db>_restore_<ts>-<pid>` target; refuse into active DB unless `--into-active` and `STHIRA_RESTORE_CONFIRM=y`. |

Every script is scoped by `STHIRA_DEPLOY_PROJECT` (default
`sthira-go`). The compose `name:` directive and the lifecycle scripts
agree on that name, so a stray second stack using the default cannot
silently pick up this package's volumes.

### First-startup order

1. Start the stack: `scripts/go-start.sh`
2. Migrate: `scripts/go-migrate.sh up`
3. Status: `scripts/go-status.sh`

Migration is intentionally NOT on api startup; if it were, a scale-up
or restart would race migrations. The brief required that.

### Update / upgrade order

1. Pull the new image tag (or rebuild with `scripts/go-build.sh`).
2. Stop api: `scripts/go-stop.sh` (without `--purge` — preserves `pgdata`).
3. Bring api up with the new image: `scripts/go-start.sh`.
4. Run any pending migrations: `scripts/go-migrate.sh up`.
5. Confirm readiness: `scripts/go-status.sh`.

The `pgdata` named volume is preserved across rebuild/update by
default; only explicit `--purge` drops it.

### Rollback limitations

- The package ships ONE schema revision at a time. There is no
  destructive down-migration runner; if a higher revision was applied
  and you need to roll back, the correct path is to restore the latest
  pre-upgrade `pg_dump` archive via `scripts/go-restore.sh
  --into-active --archive=…` (with `STHIRA_RESTORE_CONFIRM=y`).
- `scripts/go-stop.sh --purge` drops the volume; that action is the
  last-resort rollback and is gated on the explicit confirmation env.

### Logs

- Compose names the containers `sthira-go-api` and `sthira-go-postgres`.
- `scripts/go-logs.sh api` tails the API JSON logs (slog, level INFO+).
- Distroless has no shell inside the API container; health/readiness
  checks are done by `scripts/go-status.sh` via the host loopback.

### Secret handling

- `STHIRA_DATABASE_DSN` is the only secret. `bin/env-preflight.sh`
  refuses to proceed without it; the `.env` file (mode 600) is the
  recommended carrier because it is git-ignored.
- The compose file uses Compose's `${VAR:?message}` syntax for required
  Postgres env so missing values fail at compose time, not at runtime.
- The DSN is delivered to `psql` via individual `PG*` env vars (parsed
  by the stdlib `url` package). The password never appears in argv, so
  it is not visible via `ps`/process listings.
- Backups are written with mode 600 to `${STHIRA_BACKUP_DIR}` (mode 700).
  Backups never land inside the repository.
- Restoration targets a freshly created database whose name embeds a
  timestamp + PID, never the active DB unless `--into-active` plus
  `STHIRA_RESTORE_CONFIRM=y`.

### Cleanup

- `scripts/go-stop.sh` stops the api + postgis containers.
- `scripts/go-stop.sh --purge` stops, removes the containers, and drops
  the `sthira-go-pgdata` named volume (gated on
  `STHIRA_PURGE_CONFIRM=y`).
- The package never touches the recovery rehearsal stack
  (`backend/deploy/recovery/`) or the Python demo stack
  (`deploy/docker-compose.demo.yml`).

## Verified end-to-end during this build

The package was assembled against an environment without Docker
available. Native (host-side) verification covered every code path that
does not require a Docker engine:

| Concern                                          | Result        |
| ------------------------------------------------ | ------------- |
| Dockerfile parses with BuildKit syntax directive | PASS          |
| compose YAML is valid                            | PASS          |
| `migrate/cmd/sthmigrate` compiles                | PASS          |
| `migrate/cmd/sthmigrate` unit tests              | PASS (4 tests) |
| `migrate/cmd/pgdsn-env` compiles                 | PASS          |
| All shell scripts lint clean (`shellcheck`)      | PASS          |
| All shell scripts executable                     | PASS          |
| Migrate against fresh PG with PostGIS            | PASS (7/7)    |
| Idempotent re-apply of migrate                   | PASS          |
| `--target` flag respected                         | PASS          |
| Inconsistent-history rejection                   | PASS          |
| Two concurrent migrators against the same fresh DB | PASS (no errors) |
| Persistence across API restart                   | PASS          |
| `/health/live` always 200                        | PASS          |
| `/health/ready` 503 in foundation mode           | PASS          |
| `/health/ready` 200 with DB on schema 7          | PASS          |
| `go-backup.sh` writes archive (PG-18 host path)  | PASS          |
| Refuse to overwrite without --force              | PASS          |
| Restore into isolated database                   | PASS (data + schema) |
| Source DB unchanged after restore                | PASS          |
| Active DB unchanged on default restore          | PASS          |
| env-preflight refuses on missing/insecure env    | PASS          |

## Items marked NOT_RUN

`NOT_RUN.md` enumerates items deliberately left to deployment-owner
discretion (HA, scheduling, PITR, prod retention) plus items blocked
by the absent Docker engine in this environment (full docker-compose
`up`, end-to-end image build, distroless runtime smoke). Docker is
absent on this host — image-build / docker-composition paths are
verified statically only.

## Coordination notes for Worker 1

- **Do not change** `backend/go.mod`, `backend/internal/store/store.go`,
  or `backend/cmd/sthira/main.go`. Those are owned by Worker 1.
- This package takes the existing binary as it lands. If Worker 1's
  repairs change `SchemaRevision` from 7 to 8, **and** a new migration
  file `0008_*.sql` is added under `backend/migrations/`, then
  `./scripts/go-migrate.sh up` will pick it up automatically — the
  runner discovers files lexicographically and refuses downgrade, so a
  fresh 0008 will be applied without any package changes.
- If a migration contract needs new env vars, route them through
  `STHIRA_DATABASE_DSN` or `STHIRA_ASR_*`/`STHIRA_MIDDLE_*`/`STHIRA_TTS_*`
  as already wired in `cmd/sthira/main.go`. A `STHIRA_DATABASE_DSN_FILE`
  env would let this package adopt a docker-secrets pattern; that needs
  Worker 1 because it requires `store.Open(...)` to read a file path.
  This package documents the limitation honestly rather than inventing
  DSN-file support on the migration side and letting the API fork.
- The package intentionally uses **session-level advisory
  identifiers** (`pg_advisory_xact_lock` + a Go-level `flock` on a
  lockfile in `${TMPDIR}`). If Worker 1 chooses a different concurrency
  story for migration execution, this package's `runUp` is the only
  place to update.

## Limitations explicitly acknowledged

- Docker absent on the build host means the package's docker-based
  paths (image build, `go-start.sh`, `go-migrate.sh` via
  `docker compose run`, `go-restore.sh` orchestration) are
  **NOT_RUN** in the literal runtime sense. They have been
  semantically verified by static and structural inspection, plus
  hand-execution of the equivalent host-side commands with the same
  DSN. They are expected to behave correctly in a Docker-equipped
  environment; the assignment asks to mark this honestly rather than
  fabricate a passing run.
- The package is single-host. HA, multi-replica, blue/green, rolling,
  PITR, scheduled backups, retention enforcement and TLS termination
  upstream are deployment-owner decisions (see `NOT_RUN.md`).
- No model weights are bundled. No ASR/middle/TTS container images are
  built here. The voice pipeline stays unavailable (503) until
  Worker 2's components are wired to the corresponding URL env vars.
- The postgis metadata noise on restore (`COMMENT ON EXTENSION postgis`
  and the `spatial_ref_sys` COPY data, when the source postgis was
  installed by a different role) is filtered as a documented
  no-op. Business data, schema, and migration history are restored
  correctly. In the production stack the postgis extension is created
  by the bundled `postgres` superuser inside the official image; the
  same noise will appear and is similarly harmless.
