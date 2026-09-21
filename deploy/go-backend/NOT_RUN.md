# NOT_RUN items for `deploy/go-backend/`

This file lists things deliberately not delivered by the Worker 3
package, with the reason. No item here is an accidental omission;
each is either outside the brief or blocked by the environment.

## Docker-based runtime paths (host had no Docker)
- `scripts/go-build.sh` running `docker build` end-to-end: NOT_RUN.
  Build context staging (`bin/build-image.sh`) was tested standalone;
  the actual `docker build` against the staged context was not invoked
  because Docker was absent from this host.
- `scripts/go-start.sh` bringing the stack up via Compose: NOT_RUN.
  The compose YAML is valid (Python `yaml.safe_load` confirmed key
  shape: services=[postgres,api], volumes=[pgdata],
  networks=[sthira-go-backend], name=sthira-go). Compose semantics
  were not exercised because Docker was absent.
- `scripts/go-migrate.sh` running `docker compose run --rm api
  sthmigrate up`: NOT_RUN. The Docker-image path is structurally
  correct; the host fallback (build sthmigrate and `psql` directly)
  was verified against an actual PostgreSQL 18 instance and
  succeeded.
- `scripts/go-restore.sh` Docker orchestration of `createdb /
  psql / pg_restore` against the `postgis/postgis:16-3.4` image:
  NOT_RUN. The semantic equivalent was executed manually with the
  host `pg_restore` and produced the same DB state (data + schema +
  migration history).
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

## Worker 2 dependencies (not owned here)
- ASR/middle/TTS worker images, contract verification, model
  weights, readiness orchestration: NOT_RUN. The package only
  exposes the URL env vars and refuses to start a half-wired voice
  pipeline (see HANDOFF.md "Voice pipeline extension points").
- vLLM-style inference containers: NOT_RUN.

## Migration contract ambiguities (forwarded to Worker 1)
- A non-postgresql client (pg_dump 16 against server PG 18) refuses
  to dump: NOT_RUN. In production the package uses
  `postgis/postgis:16-3.4`'s bundled `pg_dump` which always matches
  the server. The version-pinning argument (`STHIRA_PG_DUMP`,
  `STHIRA_PG_RESTORE`) lets an operator override the host-fallback
  binary.
- The migration runner accepts the **current** `backend/migrations/`
  files (0001–0007) as-is. If Worker 1's Stage 4 changes add
  migration 0008, this runner will pick it up automatically — there
  is nothing to update. No contract ambiguity required clarifying
  before this commit.

## Idempotency / safety properties verified during build
- Two concurrent `sthmigrate up` invocations against the same fresh
  DB: PASS (one of each `flock` + `pg_advisory_xact_lock` per
  migration provides serialization; verifying that the second runner
  sees the first's completed work).
- Migrate refuses inconsistent history (manually-applied max=7 with
  1..6 unapplied): PASS.
- Re-apply migrate after success: PASS (idempotent, "already_applied"
  skips).
- `--target N`: PASS (stops at the named revision).
- Backup refuses to overwrite without `--force`: PASS.
- Restore defaults to an isolated `<db>_restore_<ts>-<pid>` target,
  source DB unchanged: PASS.

## Honest claims
- This package does **not** claim production readiness. The PRD,
  `plan/open-decisions.md`, and `GEMINI.md` all gate production on
  external evidence (government source authorization, model approval,
  language/ISL sign-off, security/privacy review, drift runbooks).
  This package only ships a reproducible, single-host, fail-closed
  local/staging deployment artifact.
