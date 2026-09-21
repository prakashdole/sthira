#!/usr/bin/env bash
# Bounded k6 smoke against the ACTUAL sthira Go binary running on a
# uniquely-owned migrated PostgreSQL. Records summary, key checks and
# backend stderr into plan/evidence/loadrun/.
#
# Usage: scripts/run_k6_smoke_real.sh
#
# Prereqs:
#   - PostgreSQL reachable on localhost
#   - Go 1.27.1 toolchain
#   - k6 binary installed
#
# Behavior:
#   1. Creates a uniquely-named database (sthira_p7k6_<ts>)
#   2. Applies all migrations in order with ON_ERROR_STOP=1
#   3. Builds /tmp/sthira_<ts>
#   4. Starts the binary with STHIRA_DATABASE_DSN pointing at the new DB
#   5. Runs the bounded k6 smoke (25 VUs, 20s)
#   6. Stops the binary, drops the database, captures results
#
# NEVER scale past 50 VUs in this script. Larger loads need
# controlled hardware, which is NOT_RUN in this revision.

set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

TS=$(date +%s)
DB="sthira_p7k6_${TS}"
ADDR="127.0.0.1:18443"
LOG_DIR="$ROOT/plan/evidence/loadrun"
mkdir -p "$LOG_DIR"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

cleanup() {
  log "stopping sthira (pid $STHIRA_PID)"
  kill "$STHIRA_PID" 2>/dev/null || true
  wait "$STHIRA_PID" 2>/dev/null || true
  log "dropping test database $DB"
  PGDATABASE=postgres psql -h localhost -tAc "DROP DATABASE IF EXISTS ${DB}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

log "creating test database $DB"
PGDATABASE=postgres psql -h localhost -tAc "CREATE DATABASE ${DB}" >/dev/null

log "applying migrations (ordered, ON_ERROR_STOP=1)"
for f in backend/migrations/*.sql; do
  PGDATABASE="$DB" psql -h localhost -v ON_ERROR_STOP=1 -q -f "$f" >/dev/null
done

REV=$(PGDATABASE="$DB" psql -h localhost -tAc "SELECT MAX(revision) FROM schema_migrations")
log "schema_migrations.revision=$REV"

log "building sthira"
(cd backend && go build -o /tmp/sthira_"$TS" ./cmd/sthira)
STHIRA_BIN=/tmp/sthira_"$TS"

log "starting $STHIRA_BIN on $ADDR"
STHIRA_ADDR="$ADDR" \
STHIRA_DATABASE_DSN="postgres://@localhost/${DB}?sslmode=disable" \
  "$STHIRA_BIN" >"$LOG_DIR/sthira_${TS}.log" 2>&1 &
STHIRA_PID=$!

# Wait for /health/ready.
for i in $(seq 1 20); do
  if curl -fsS "http://${ADDR}/health/ready" >/dev/null 2>&1; then
    log "ready after ${i} attempts"
    break
  fi
  sleep 0.2
done

if ! curl -fsS "http://${ADDR}/health/ready" >/dev/null 2>&1; then
  log "ERROR: sthira did not become ready"
  exit 1
fi

log "running k6 smoke_real.js (25 VUs, 20s)"
set +e
BASE_URL="http://${ADDR}" \
  k6 run --summary-export "$LOG_DIR/smoke_real_${TS}_summary.json" \
         "$ROOT/loadmodel/k6/smoke_real.js" \
         2>"$LOG_DIR/k6_${TS}.stderr.log" \
         >"$LOG_DIR/k6_${TS}.stdout.log"
K6_STATUS=$?
set -e

log "k6 exit status: $K6_STATUS"

# Latest summary alias.
cp -f "$LOG_DIR/smoke_real_${TS}_summary.json" "$LOG_DIR/smoke_real_summary.json"
cp -f "$LOG_DIR/sthira_${TS}.log" "$LOG_DIR/sthira.log"
cp -f "$LOG_DIR/k6_${TS}.stdout.log" "$LOG_DIR/k6.stdout.log"

if [ $K6_STATUS -ne 0 ]; then
  log "ERROR: k6 run failed (status $K6_STATUS); see $LOG_DIR/k6_${TS}.stderr.log"
  exit $K6_STATUS
fi

log "evidence:"
log "  $LOG_DIR/smoke_real_summary.json"
log "  $LOG_DIR/sthira.log"
log "  $LOG_DIR/k6.stdout.log"
exit 0