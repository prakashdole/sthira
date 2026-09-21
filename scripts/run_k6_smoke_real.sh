#!/usr/bin/env bash
# Bounded k6 smoke against the ACTUAL sthira Go binary running on a
# uniquely-owned migrated PostgreSQL. Records summary, key checks and
# backend stderr into plan/evidence/loadrun/.
#
# Usage: scripts/run_k6_smoke_real.sh [--business-flow]
#
# Behavior:
#   1. Creates a uniquely-named database (sthira_p7k6_<ts>)
#   2. Applies all migrations in order with ON_ERROR_STOP=1
#   3. Builds /tmp/sthira_<ts>
#   4. Allocates a free loopback port and starts the binary bound
#      to it
#   5. Waits for /health/ready with bounded retries
#   6. Runs the bounded k6 smoke (smoke_real.js, 25 VUs, 20s) which
#      probes /health/* and exercises two typed handlers with the
#      expected 4xx outcome
#   7. Optionally runs smoke_business.js (--business-flow) which
#      exercises a successful place-resolve round trip
#   8. Stops the binary, drops the database, captures results
#
# Exit codes:
#   0   k6 passed all thresholds (no 5xx, no connection failures)
#   1   k6 returned non-zero (broken response or connection drop)
#   2   sthira failed to become ready
#   3   migration failed
#
# NEVER scale past 50 VUs. Larger loads need controlled hardware,
# which is NOT_RUN in this revision.

set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

BUSINESS_FLOW=0
for arg in "$@"; do
  case "$arg" in
    --business-flow) BUSINESS_FLOW=1 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

TS=$(date +%s)
DB="sthira_p7k6_${TS}"

# Allocate a free loopback port to avoid colliding with team
# services. The chosen port range (45000-65000) excludes common
# dev ports.
ADDR="127.0.0.1:$(python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()")"

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

# Detect whether PostgreSQL is reachable on localhost.
if ! PGDATABASE=postgres psql -h localhost -tAc "SELECT 1" >/dev/null 2>&1; then
  log "ERROR: PostgreSQL not reachable on localhost; aborting"
  exit 1
fi

log "creating test database $DB"
PGDATABASE=postgres psql -h localhost -tAc "CREATE DATABASE ${DB}" >/dev/null

log "applying migrations (ordered, ON_ERROR_STOP=1)"
for f in backend/migrations/*.sql; do
  if ! PGDATABASE="$DB" psql -h localhost -v ON_ERROR_STOP=1 -q -f "$f" >/dev/null; then
    log "ERROR: migration failed: $f"
    exit 3
  fi
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

# Wait for /health/ready with bounded retries.
ready=0
for i in $(seq 1 50); do
  if curl -fsS "http://${ADDR}/health/ready" >/dev/null 2>&1; then
    log "ready after ${i} attempts"
    ready=1
    break
  fi
  sleep 0.2
done

if [ "$ready" -ne 1 ]; then
  log "ERROR: sthira did not become ready; see $LOG_DIR/sthira_${TS}.log"
  exit 2
fi

run_k6() {
  local script="$1"
  local label="$2"
  log "running k6 $script (label=$label)"
  set +e
  BASE_URL="http://${ADDR}" \
    k6 run --summary-export "$LOG_DIR/${label}_${TS}_summary.json" \
           "$ROOT/loadmodel/k6/$script" \
           2>"$LOG_DIR/${label}_${TS}.stderr.log" \
           >"$LOG_DIR/${label}_${TS}.stdout.log"
  local k6_status=$?
  set -e
  log "$label k6 exit status: $k6_status"
  cp -f "$LOG_DIR/${label}_${TS}_summary.json" "$LOG_DIR/${label}_summary.json"
  cp -f "$LOG_DIR/${label}_${TS}.stdout.log" "$LOG_DIR/${label}.stdout.log"
  cp -f "$LOG_DIR/${label}_${TS}.stderr.log" "$LOG_DIR/${label}.stderr.log"
  if [ "$k6_status" -ne 0 ]; then
    log "ERROR: k6 $script failed (status $k6_status); see $LOG_DIR/${label}_${TS}.stderr.log"
    return "$k6_status"
  fi
  return 0
}

cp -f "$LOG_DIR/sthira_${TS}.log" "$LOG_DIR/sthira.log"

if ! run_k6 smoke_real.js smoke_real; then
  exit 1
fi

if [ "$BUSINESS_FLOW" -eq 1 ]; then
  if ! run_k6 smoke_business.js smoke_business; then
    exit 1
  fi
fi

log "evidence:"
log "  $LOG_DIR/smoke_real_summary.json"
log "  $LOG_DIR/sthira.log"
log "  $LOG_DIR/k6.stdout.log"
exit 0
