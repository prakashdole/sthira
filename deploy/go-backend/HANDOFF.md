# Task R06 — Go backend deployment package: handoff

This document contains operator-facing notes for the local/staging deployment
package built under `deploy/go-backend/`.

- **Toolchain pin** (recorded in `backend/go.mod`):
  - Go: `1.27.1` (Dockerfile `ARG GO_VERSION` defaults to the same).
  - Database image: `postgis/postgis:18-3.6` (overridable via `STHIRA_PG_IMAGE`).
- **Images**:
  - API runtime: `gcr.io/distroless/static-debian12:nonroot` on UID `65532`.
  - Migration runner: dedicated image target based on `postgis/postgis:18-3.6`
    bundled with `psql`, `sthmigrate`, `pgdsn-env`, and `/migrations` copied from
    `backend/migrations/`.
- **Schema target**: the binary expects `SchemaRevision == 9`
  (`backend/internal/store/store.go`), which the staged migration
  files 0001–0009 collectively produce.
- **Project isolation**: all fixed container names and hardcoded volume names
  have been removed. Multi-project invocations via `STHIRA_DEPLOY_PROJECT`
  (or `docker compose -p <project>`) run cleanly in isolated namespaces.

## What this package contains

```
deploy/go-backend/
  Dockerfile                  # multi-stage: build -> migrate (with psql) -> api (distroless)
  docker-compose.yml          # api + postgres + migrate; loopback-only API; project-isolated pgdata
  .env.example                # schema for the local ignored .env
  HANDOFF.md                  # ← this file
  NOT_RUN.md                  # what this package does NOT do (and why)
  bin/
    env-preflight.sh          # required-configuration guard
    build-image.sh            # stages build context + builds api and migrate images
  scripts/
    go-build.sh               # stages and builds deployment images
    go-start.sh               # bring up api + postgres containers
    go-status.sh              # container state + /health/* probes
    go-logs.sh                # tail container logs
    go-stop.sh                # stop containers (--purge for destructive project-volume cleanup)
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
      main_test.go
```

The `migrate/` module is a standalone Go module (`sthira/deploy/go-backend/migrate`)
with no third-party dependencies (stdlib only). It compiles cleanly from its own
directory without `go mod download || true` or `-mod=mod` flags.

## Operator quickstart

### Prerequisites

- Docker Engine ≥ 24 with Compose v2 (when running containerized).
- A POSIX shell (`bash`, `zsh`) — every lifecycle script is shellcheck-clean.
- A reachable loopback port for the API (default `8080`; override with
  `STHIRA_API_HOST_PORT`).
- When Docker is unavailable, the lifecycle scripts provide documented host
  fallbacks (Go + host PostgreSQL tools).

### First-time setup

```sh
cd /path/to/MonitoringZ

cp deploy/go-backend/.env.example deploy/go-backend/.env
chmod 600 deploy/go-backend/.env
$EDITOR deploy/go-backend/.env           # set STHIRA_PG_PASSWORD and update the DSN host/port

./deploy/go-backend/scripts/go-build.sh   # stages context and builds api + migrate images
./deploy/go-backend/scripts/go-start.sh   # brings up postgres + api containers
./deploy/go-backend/scripts/go-migrate.sh up   # explicit one-shot migration through revision 9
./deploy/go-backend/scripts/go-status.sh       # confirm /health/ready is 200
```

### Voice pipeline extension points (optional)

The API service wires its voice orchestrator only when **all three**
of `STHIRA_ASR_URL`, `STHIRA_MIDDLE_URL`, `STHIRA_TTS_URL` are set
*and* the database is reachable. Tokens are optional per stage. When
unconfigured the `POST /api/v3/voice/commands` endpoint fails closed
with `503` — the documented posture when no operational voice workers
exist.

The deployment package **does not** bundle ASR, middle, or TTS workers,
model weights, or vLLM/vLLM-style inference images. Those modules are
owned by R02; only their URLs are exposed here as the package's extension point.

### Lifecycle commands

| Command                                  | Effect                                        |
| ---------------------------------------- | --------------------------------------------- |
| `scripts/go-build.sh`                    | Build API and migrate images with pinned toolchain. |
| `scripts/go-start.sh`                    | Bring up postgres + api. Idempotent.          |
| `scripts/go-status.sh`                   | Container state + `/health/live`, `/health/ready`, applied migrations. |
| `scripts/go-logs.sh [api\|postgres]`     | Tail container logs.                          |
| `scripts/go-stop.sh [--purge]`           | Stop containers. `--purge` requires `STHIRA_PURGE_CONFIRM=y` and removes project containers and volume via `docker compose down -v`. |
| `scripts/go-migrate.sh up [N]`           | One-shot explicit migration. `N` is optional target revision (defaults to head: 9). |
| `scripts/go-migrate.sh status`           | Print applied revisions and pending files.    |
| `scripts/go-migrate.sh verify`           | Parse pending files without committing (ROLLBACK per file). |
| `scripts/go-backup.sh [--force]`         | `pg_dump` to `${STHIRA_BACKUP_DIR:-/var/backups/<project>}/<project>-<ts>.dump`. |
| `scripts/go-restore.sh --archive=PATH`   | Restore into a freshly-created `<db>_restore_<ts>-<pid>` target; refuse into active DB unless `--into-active` and `STHIRA_RESTORE_CONFIRM=y`. |

Every script is scoped by `STHIRA_DEPLOY_PROJECT` (default `sthira-go`).

### Startup and migration order

1. Start the stack: `scripts/go-start.sh`
2. Migrate: `scripts/go-migrate.sh up`
3. Status: `scripts/go-status.sh`

Migration is intentionally NOT run on API startup; if it were, a scale-up
or restart would race migrations.

### Secret handling

- `STHIRA_DATABASE_DSN` is the only database connection secret. `bin/env-preflight.sh`
  refuses to proceed without it; the `.env` file (mode 600) is the recommended
  carrier because it is git-ignored.
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

- `scripts/go-stop.sh` stops the api + postgres containers.
- `scripts/go-stop.sh --purge` runs `docker compose down -v --remove-orphans`,
  removing project-scoped containers and volumes (gated on `STHIRA_PURGE_CONFIRM=y`).
