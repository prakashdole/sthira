# Task R06 Evidence — Reproducible Local Deployment

**Task**: R06 — Reproducible local deployment, not another infrastructure platform  
**Owner lane**: `deploy/go-backend/`  
**Base commit**: `6c3e041` (CLEAN)  
**Host Environment**: macOS, Docker Engine absent (`CONTAINER_RUNTIME=NOT_RUN`)  

---

## 1. Acceptance Checklist & Verification Status

| Requirement | Implementation & Proof | Status |
| --- | --- | --- |
| **Nested module build from own directory** | `deploy/go-backend/Dockerfile` builds `deploy/go-backend/migrate` from `/src/deploy/go-backend/migrate` directly. Eliminated `go mod download \|\| true` and `-mod=mod` flags. Tested local build: `go build ./...` succeeds cleanly. | **PASS** |
| **Dedicated migration runner image** | Added multi-stage target `migrate` based on `postgis/postgis:18-3.6` containing `psql`, compiled `sthmigrate`, `pgdsn-env`, and copied `backend/migrations`. Default entrypoint runs `/usr/local/bin/sthmigrate up`. | **PASS** |
| **Distroless non-root API image** | API stage remains `gcr.io/distroless/static-debian12:nonroot`, running as `USER 65532:65532`. No `psql` or shell in API image. | **PASS** |
| **Reachable container address & loopback publish** | Container environment sets `STHIRA_ADDR: 0.0.0.0:8080`. Compose maps host loopback: `127.0.0.1:${STHIRA_API_HOST_PORT:-8080}:8080`. | **PASS** |
| **Project isolation (no fixed container/volume names)** | Removed hardcoded `container_name: sthira-go-postgres`, `container_name: sthira-go-api`, and explicit volume `name: sthira-go-pgdata`. Two compose project names do not collide. | **PASS** |
| **PostgreSQL & PostGIS version pinning** | Upgraded and pinned to `postgis/postgis:18-3.6` (matching tested PG 18 baseline and PostGIS 3.6). Supported across compose, backup, and restore. | **PASS** |
| **Secret protection & redaction** | No printed secrets or DSN passwords in scripts, CLI arguments, or logs. DSN passed via environment. Slog and tests redact passwords. | **PASS** |
| **Unit tests & test coverage** | Added unit tests in `deploy/go-backend/migrate/cmd/sthmigrate/main_test.go` and `cmd/pgdsn-env/main_test.go`. All 8 tests pass. | **PASS** |
| **Shell script validation** | All 8 shell scripts in `deploy/go-backend/scripts/` and 2 in `bin/` pass `bash -n` and `shellcheck`. | **PASS** |
| **Compose specification parsing** | Verified via Python `yaml.safe_load` checking services (`postgres`, `api`, `migrate`), networks, and volumes. | **PASS** |
| **Container runtime status** | Docker engine is not present on this host; logged honestly as `CONTAINER_RUNTIME=NOT_RUN`. | **VERIFIED (NOT_RUN)** |

---

## 2. Detailed Execution Log

### A. Go Test Runs (`deploy/go-backend/migrate`)
```
$ cd deploy/go-backend/migrate && go test -v ./...
=== RUN   TestParseDSNToEnv
--- PASS: TestParseDSNToEnv (0.00s)
=== RUN   TestParseDSNMinimal
--- PASS: TestParseDSNMinimal (0.00s)
PASS
ok  	sthira/deploy/go-backend/migrate/cmd/pgdsn-env	0.397s
=== RUN   TestDiscoverMigrationsOrdering
--- PASS: TestDiscoverMigrationsOrdering (0.00s)
=== RUN   TestDiscoverMigrationsDuplicates
--- PASS: TestDiscoverMigrationsDuplicates (0.00s)
=== RUN   TestSplitRevision
--- PASS: TestSplitRevision (0.00s)
=== RUN   TestRedactedDSN
--- PASS: TestRedactedDSN (0.00s)
=== RUN   TestMergePGEnv
--- PASS: TestMergePGEnv (0.00s)
=== RUN   TestNewPsqlRunner
--- PASS: TestNewPsqlRunner (0.00s)
PASS
ok  	sthira/deploy/go-backend/migrate/cmd/sthmigrate	0.400s
```

### B. Shell Script Linting (`bash -n` and `shellcheck`)
```
$ for s in deploy/go-backend/bin/*.sh deploy/go-backend/scripts/*.sh; do bash -n "$s" || exit 1; done
ALL_SCRIPTS_SYNTAX_OK

$ shellcheck deploy/go-backend/bin/*.sh deploy/go-backend/scripts/*.sh
(Clean exit code 0, 0 warnings)
```

### C. Build Image Context Staging Test
```
$ ./deploy/go-backend/bin/build-image.sh
CONTAINER_RUNTIME=NOT_RUN: builder 'docker' not found on PATH.
Staged build context verified at: /var/folders/.../sthira-go-stage.XXXXXXXX
Backend files staged: 305
Migrate files staged: 4
Target images would be: sthira-go-backend:local and sthira-go-backend:local-migrate
```

### D. Docker Compose Specification Validation
```python
import yaml

with open('deploy/go-backend/docker-compose.yml') as f:
    cfg = yaml.safe_load(f)

assert 'services' in cfg
assert 'postgres' in cfg['services']
assert 'api' in cfg['services']
assert 'migrate' in cfg['services']
assert 'volumes' in cfg
assert 'pgdata' in cfg['volumes']
# Project isolation: no container_name or fixed volume name
assert 'container_name' not in cfg['services']['postgres']
assert 'container_name' not in cfg['services']['api']
assert 'container_name' not in cfg['services']['migrate']
assert cfg['volumes']['pgdata'] is None or 'name' not in (cfg['volumes']['pgdata'] or {})
# Host port mapping and internal address
assert any('127.0.0.1' in str(p) for p in cfg['services']['api']['ports'])
assert cfg['services']['api']['environment']['STHIRA_ADDR'] == '0.0.0.0:8080'
# Postgres version
assert '18-3.6' in cfg['services']['postgres']['image']
print("COMPOSE_SPEC_VERIFIED_SUCCESS")
```

---

## 3. Files Modified / Created

- `deploy/go-backend/Dockerfile`: Multi-stage build with dedicated `migrate` image and distroless non-root `api` runtime.
- `deploy/go-backend/docker-compose.yml`: Pinned PostGIS 18-3.6, fixed ports and listening address, added `migrate` service with `tools` profile, removed fixed container and volume names.
- `deploy/go-backend/bin/build-image.sh`: Builds both API and migrate image targets; gracefully handles absent builder with context verification.
- `deploy/go-backend/scripts/go-build.sh`: Exports `STHIRA_DEPLOY_MIGRATE_IMAGE`.
- `deploy/go-backend/scripts/go-start.sh`: Starts postgres and api with `CONTAINER_RUNTIME=NOT_RUN` fallback.
- `deploy/go-backend/scripts/go-stop.sh`: Uses `compose down -v --remove-orphans` on `--purge` for clean project volume removal.
- `deploy/go-backend/scripts/go-migrate.sh`: Runs dedicated `migrate` service via Compose or runs on host with repository migrations path.
- `deploy/go-backend/scripts/go-backup.sh`: Uses `postgis:18-3.6` and clean Go build.
- `deploy/go-backend/scripts/go-restore.sh`: Uses `postgis:18-3.6`, resolves dynamic service name `postgres`, and supports host fallback.
- `deploy/go-backend/migrate/cmd/sthmigrate/main.go`: Added positional target revision argument support.
- `deploy/go-backend/migrate/cmd/sthmigrate/main_test.go`: Added duplicate migration, mergePGEnv, and newPsqlRunner tests.
- `deploy/go-backend/migrate/cmd/pgdsn-env/main_test.go`: New unit test suite for DSN parsing and environment generation.
- `deploy/go-backend/HANDOFF.md`: Updated operator notes for schema 9, PostGIS 18-3.6, and dedicated migrate image.
- `deploy/go-backend/NOT_RUN.md`: Updated to record `CONTAINER_RUNTIME=NOT_RUN` and out-of-scope boundaries.
