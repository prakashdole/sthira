# NOT_RUN items for `deploy/go-backend/`

This file lists things deliberately not delivered or executed in the current
environment for Task R06, with the reason. No item here is an accidental omission;
each is either outside the brief or blocked by the environment.

## Docker-based runtime paths (host has CONTAINER_RUNTIME=NOT_RUN)
- `scripts/go-build.sh` running `docker build` end-to-end: NOT_RUN.
  Build context staging (`bin/build-image.sh`) was executed and verified standalone;
  the actual `docker build` against the staged context was not invoked
  because Docker was absent from this macOS host (`docker: command not found`).
- `scripts/go-start.sh` bringing the stack up via Compose: NOT_RUN.
  The compose YAML is valid (Python `yaml.safe_load` confirmed key
  shape: services=[postgres,api,migrate], volumes=[pgdata],
  networks=[sthira-go-backend], project isolation verified). Compose semantics
  were not exercised because Docker was absent.
- `scripts/go-migrate.sh` running `docker compose run --rm migrate up`: NOT_RUN.
  The dedicated migration runner image path (`target: migrate` on `postgis/postgis:18-3.6`
  with bundled `psql`, `sthmigrate`, `pgdsn-env`, and `/migrations`) is structurally
  verified. The host fallback (`sthmigrate` compiled stdlib-only) passes all unit tests
  and handles schema 9 migrations.
- `scripts/go-restore.sh` Docker orchestration of `createdb / psql / pg_restore`
  against `postgis/postgis:18-3.6`: NOT_RUN. The semantic equivalent was verified
  statically and supports host fallback when host tools are present.
- Healthcheck-while-in-container: NOT_RUN. Distroless has no shell,
  so container-side healthchecks are intentionally disabled in
  `deploy/go-backend/docker-compose.yml`; `scripts/go-status.sh`
  probes `/health/*` from the host loopback instead. This is by
  design — see `Dockerfile` and `docker-compose.yml` for the rationale.

## Deployment-owner decisions (out of scope)
- **HA / failover**: NOT_RUN. The package is single-host. Streaming
  replicas, `pg_basebackup`, `pg_ctl promote`, Patroni, and any
  true-failover semantics remain a deployment-owner decision. See
  `backend/deploy/recovery/NOT_RUN.failover.md` for the rehearsal
  package's stance and `plan/open-decisions.md`.
- **PITR / WAL archiving**: NOT_RUN. Point-in-time recovery is a
  deployment-owner decision. The package's `go-backup.sh` produces
  logical dumps; it does not configure `archive_mode`/`archive_command`
  on the running PostgreSQL cluster.
- **Backup scheduling**: NOT_RUN. The package only ships one-shot
  backup/restore commands. Cron, systemd timers, or a backup orchestrator
  are deployment-owner choices.
- **Retention and rotation**: NOT_RUN. The backup directory
  (`${STHIRA_BACKUP_DIR}`) is the operator's responsibility to prune.
  The package never auto-deletes backups (see HANDOFF.md
  "Secret handling" / "Cleanup").
- **Public TLS / domain / DNS**: NOT_RUN. The compose file binds the
  API to the host loopback only. Production deployments must place a
  TLS-terminating reverse proxy (Caddy / nginx / envoy / cloud LB) in
  front of `127.0.0.1:8080`; certificate procurement and renewal are
  not implemented here.
- **Hardware sizing and capacity claims**: NOT_RUN. The compose file
  carries bounded CPU/memory defaults for a *single-host local*
  deployment; no claim is made about user capacity, throughput, or
  p99 latency. Those depend on hardware, traffic shape, and the
  cost/benefit trade-off the deployment owner accepts.
- **Real-time observability stack**: NOT_RUN. The compose file
  exposes JSON to stdout/stderr (`slog`); the lifecycle scripts can
  be pointed at any log shipper. A Prometheus exporter, trace
  collection, or audit pipeline is intentionally not built.

## Inference dependencies (R02 / Worker 2)
- ASR/middle/TTS worker images, contract verification, model
  weights, readiness orchestration: NOT_RUN. The package only
  exposes the URL env vars and refuses to start a half-wired voice
  pipeline (see HANDOFF.md "Voice pipeline extension points").
- vLLM-style inference containers: NOT_RUN.

## Migration contracts and schema alignment
- Under R00, the required schema readiness is revision 9 (staged
  `backend/migrations/0001_p3_foundation.sql` through `0009_p6_template_binding.sql`).
- The database image is pinned to `postgis/postgis:18-3.6` (overridable via `STHIRA_PG_IMAGE`).
- The migration runner `sthmigrate` supports `up [target]`, `status`, and `verify`.

## Idempotency / safety properties verified during build
- Unit test coverage for `sthmigrate` (`main_test.go`): discovery, ordering,
  duplicate revision rejection, revision splitting, DSN redaction, `PG*` env
  merging, and `newPsqlRunner`.
- Unit test coverage for `pgdsn-env` (`main_test.go`): URL parsing, port, credentials,
  and query options extraction into libpq environment variables.
- Shell script static verification: all scripts pass `bash -n` and `shellcheck`.
- Build context staging verification: `bin/build-image.sh` stages backend (305 files)
  and migrate (4 files) into an isolated scratch tree without polluting repository root.
- Compose specification validation: verified via Python `yaml.safe_load` for services,
  ports (host loopback binding `127.0.0.1:${STHIRA_API_HOST_PORT:-8080}:8080`),
  internal address `0.0.0.0:8080`, project isolation (no fixed container/volume names),
  and PostgreSQL 18-3.6 pinning.

## Honest claims
- This package does **not** claim live production readiness or container runtime execution
  on hosts where Docker Engine is not installed.
- R07 local demonstration may use documented local processes (`cd backend && go run ./cmd/sthira`)
  without requiring Docker on the critical path.
