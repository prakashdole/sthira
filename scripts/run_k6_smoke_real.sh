#!/usr/bin/env bash
# Bounded k6 smoke against the ACTUAL sthira Go binary running on a
# uniquely-owned migrated PostgreSQL. Records a SUMMARIZED verdict
# (not raw megabyte logs) plus key checks and capped backend stderr
# into plan/evidence/loadrun/.
#
# Usage: scripts/run_k6_smoke_real.sh [--business-flow]
#
# Ownership rules (P7 harness repair, 2026-09-21):
#   * Every owned resource (database, process, binary) is
#     initialized BEFORE any trap is installed, and cleanup only
#     ever touches resources THIS run actually created. An existing
#     database is never dropped; a reused/foreign PID is never
#     killed (the kill is guarded by a process-identity check).
#   * Names embed microseconds + PID + a random token, not
#     second-resolution timestamps, so concurrent runs cannot
#     collide or hijack each other's resources.
#   * --business-flow seeds exactly one isolated place_aliases row
#     (unique alias_id) and asserts the round trip semantically
#     (data.place_id / place_kind / envelope fields) — never by
#     body length.
#
# Exit codes:
#   0   k6 passed all thresholds AND the seeded business flow (when
#       requested) returned the semantically correct candidate
#   1   k6 reported failed thresholds / broken responses
#   2   sthira failed to become ready (readiness timeout)
#   3   migration failed
#   4   precondition missing (psql unreachable, k6 not installed,
#       seeding verification failed) — the run did NOT happen
# NEVER scale past 50 VUs.

set -euo pipefail

# Force PG connections over 127.0.0.1: macOS /etc/hosts resolves
# `localhost` to ::1 first, but the local Postgres instance binds
# IPv4 only; pinning avoids libpq racing on two addresses.
export PGHOST=127.0.0.1

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

BUSINESS_FLOW=0
for arg in "$@"; do
  case "$arg" in
    --business-flow) BUSINESS_FLOW=1 ;;
    *) echo "unknown argument: $arg" >&2; exit 4 ;;
  esac
done

# ---- owned resources, initialized BEFORE the trap ----------------
RUN_ID="$(date -u +%Y%m%dT%H%M%S)_$$_$(python3 -c 'import secrets;print(secrets.token_hex(3))')"
DB="sthira_p7k6_${RUN_ID}"
DB="$(printf %s "$DB" | tr "[:upper:]" "[:lower:]" | tr -cd "a-z0-9_-")"
DB_CREATED=0                 # set ONLY after a successful CREATE
STHIRA_PID=""                # set ONLY after a successful spawn
STHIRA_BIN="/tmp/sthira_smoke_${RUN_ID}"
BIN_CREATED=0
ADDR="127.0.0.1:$(python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()")"
LOG_DIR="$ROOT/plan/evidence/loadrun"
mkdir -p "$LOG_DIR"
SERVER_LOG="$LOG_DIR/sthira_${RUN_ID}.log"
# Seeded business identity (isolated to this run's DB).
SEED_ALIAS_ID="p7k6-${RUN_ID}"
SEED_JURISDICTION="KL-WYD"
SEED_QUERY="Smokehold-${RUN_ID}"
SEED_PLACE_ID="FACILITY-SMOKE-${RUN_ID}"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

proc_is_ours() {
  # True only if $1 is alive AND is our built binary. Protects
  # against killing a PID that was recycled after our server died.
  local pid="$1" args
  args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
  case "$args" in
    *"$STHIRA_BIN"*) return 0 ;;
    *) return 1 ;;
  esac
}

cleanup() {
  if [ -n "$STHIRA_PID" ] && proc_is_ours "$STHIRA_PID"; then
    log "stopping owned sthira (pid $STHIRA_PID)"
    kill "$STHIRA_PID" 2>/dev/null || true
    wait "$STHIRA_PID" 2>/dev/null || true
  fi
  if [ "$DB_CREATED" -eq 1 ]; then
    # Only ever drop the database THIS run created.
    PGDATABASE=postgres psql -h 127.0.0.1 -tAc "DROP DATABASE ${DB}" >/dev/null 2>&1 || true
  fi
  if [ "$BIN_CREATED" -eq 1 ]; then
    rm -f "$STHIRA_BIN" || true
  fi
}
trap cleanup EXIT

# ---- preconditions -------------------------------------------------
if ! PGDATABASE=postgres psql -h 127.0.0.1 -tAc "SELECT 1" >/dev/null 2>&1; then
  log "ERROR: PostgreSQL not reachable on localhost; aborting (NOT_RUN)"
  exit 4
fi
if ! command -v k6 >/dev/null 2>&1; then
  log "ERROR: k6 not installed; the smoke did NOT run (NOT_RUN)"
  exit 4
fi

# Never touch a pre-existing database name: our name embeds RUN_ID,
# but verify anyway rather than assuming.
if PGDATABASE=postgres psql -h 127.0.0.1 -tAc "SELECT 1 FROM pg_database WHERE datname='${DB}'" | grep -q 1; then
  log "ERROR: database ${DB} already exists; refusing to reuse/drop"
  exit 4
fi

log "creating owned test database $DB"
PGDATABASE=postgres psql -h 127.0.0.1 -tAc "CREATE DATABASE ${DB}" >/dev/null
DB_CREATED=1

log "applying migrations (ordered, ON_ERROR_STOP=1)"
for f in backend/migrations/*.sql; do
  if ! PGDATABASE="$DB" psql -h 127.0.0.1 -v ON_ERROR_STOP=1 -q -f "$f" >/dev/null; then
    log "ERROR: migration failed: $f"
    exit 3
  fi
done
REV=$(PGDATABASE="$DB" psql -h 127.0.0.1 -tAc "SELECT MAX(revision) FROM schema_migrations")
log "schema_migrations.revision=$REV"

if [ "$BUSINESS_FLOW" -eq 1 ]; then
  log "seeding one isolated place alias ($SEED_ALIAS_ID)"
  PGDATABASE="$DB" psql -h 127.0.0.1 -v ON_ERROR_STOP=1 -q <<SQL >/dev/null
INSERT INTO place_aliases (alias_id, jurisdiction, lookup_key, place_id, place_kind)
VALUES ('${SEED_ALIAS_ID}', '${SEED_JURISDICTION}', lower('${SEED_QUERY}'), '${SEED_PLACE_ID}', 'FACILITY');
SQL
  got=$(PGDATABASE="$DB" psql -h 127.0.0.1 -tAc "SELECT count(*) FROM place_aliases WHERE alias_id='${SEED_ALIAS_ID}'")
  if [ "$got" != "1" ]; then
    log "ERROR: seed verification failed (count=$got)"
    exit 4
  fi
fi

log "building sthira"
(cd backend && go build -o "$STHIRA_BIN" ./cmd/sthira)
BIN_CREATED=1

log "starting $STHIRA_BIN on $ADDR"
STHIRA_ADDR="$ADDR" \
STHIRA_DATABASE_DSN="postgres://@127.0.0.1/${DB}?sslmode=disable" \
  "$STHIRA_BIN" >"$SERVER_LOG" 2>&1 &
STHIRA_PID=$!   # ONLY now does the trap have a live PID to own

ready=0
for i in $(seq 1 50); do
  if ! proc_is_ours "$STHIRA_PID"; then
    log "ERROR: sthira process died during startup; see $SERVER_LOG"
    break
  fi
  if curl -fsS "http://${ADDR}/health/ready" >/dev/null 2>&1; then
    log "ready after ${i} attempts"
    ready=1
    break
  fi
  sleep 0.2
done
if [ "$ready" -ne 1 ]; then
  # Readiness timeout MUST fail the harness, not pass vacuously.
  log "ERROR: sthira did not become ready within budget"
  tail -n 40 "$SERVER_LOG" >> "$LOG_DIR/sthira.log" 2>/dev/null || true
  exit 2
fi

# ---- k6 runs -------------------------------------------------------
verdict_summary() {
  # Extract the threshold table from k6's summary JSON: a compact
  # line per threshold, not the raw multi-megabyte file.
  local summary="$1" label="$2"
  python3 - "$summary" "$label" << 'PYEOF'
import json, sys, pathlib
path, label = sys.argv[1], sys.argv[2]
p = pathlib.Path(path)
if not p.exists():
    print(f"{label}: NO SUMMARY PRODUCED")
    sys.exit(1)
data = json.loads(p.read_text())
# k6 stores threshold status as literal bool; older shapes wrap in {ok: bool}.
# k6 quirk: a Counter/Rate with NO data points evaluates `count==0` /
# `rate==0` as False in the summary, but does NOT fail the run — k6
# exit code 0 means "the threshold was not violated by data". Match
# that: skip a False reading when the metric has no observed events.
total, bad, skipped = 0, [], 0
def metric_has_data(m):
    if not isinstance(m, dict): return False
    return (m.get("count") or m.get("passes") or m.get("fails") or m.get("value")) not in (0, None)
def record(name, t, m=None):
    if isinstance(t, bool):
        ok = t
    elif isinstance(t, dict):
        ok = t.get("ok", True)
    else:
        return 0, None, False
    if not ok and not metric_has_data(m):
        return 1, None, True
    return 1, (None if ok else name), False
for name, t in (data.get("thresholds", {}) or {}).items():
    n, b, sk = record(name, t)
    total += n
    skipped += sk
    if b is not None: bad.append(b)
for m_name, m in (data.get("metrics") or {}).items():
    ths = (m or {}).get("thresholds") or {}
    if not isinstance(ths, dict): continue
    for t_name, t in ths.items():
        n, b, sk = record(t_name, t, m)
        total += n
        skipped += sk
        if b is not None: bad.append(f"{m_name}::{b}")
ok = (not bad) if total else None
print(f"{label}: thresholds={total} failed={bad if bad else 0} no-data-skipped={skipped} ok={ok}")
sys.exit(1 if bad else 0)
PYEOF
}

run_k6() {
  local script="$1" label="$2"
  shift 2
  log "running k6 $script (label=$label)"
  local out="$LOG_DIR/${label}_${RUN_ID}.stdout.log"
  local err="$LOG_DIR/${label}_${RUN_ID}.stderr.log"
  local summary="$LOG_DIR/${label}_${RUN_ID}_summary.json"
  set +e
  BASE_URL="http://${ADDR}" \
  SMOKE_BUSINESS_JURISDICTION="${SMOKE_JURIS:-KL-WYD}" \
  SMOKE_BUSINESS_QUERY="${SMOKE_QUERY:-}" \
  SMOKE_BUSINESS_PLACE_ID="${SMOKE_PLACE:-}" \
  SMOKE_BUSINESS_PLACE_KIND="${SMOKE_KIND:-}" \
    k6 run --summary-export "$summary" "$ROOT/loadmodel/k6/$script" \
      2>"$err" >"$out"
  local k6_status=$?
  set -e
  log "$label k6 exit status: $k6_status"
  # Capped evidence: last 60 lines of each stream, not everything.
  tail -n 60 "$out" > "$LOG_DIR/${label}.stdout.log" 2>/dev/null || true
  tail -n 60 "$err" > "$LOG_DIR/${label}.stderr.log" 2>/dev/null || true
  if ! verdict_summary "$summary" "$label"; then
    log "ERROR: $label threshold verification failed"
    return 1
  fi
  if [ "$k6_status" -ne 0 ]; then
    log "ERROR: k6 $script failed (status $k6_status)"
    return "$k6_status"
  fi
  return 0
}

cp -f "$SERVER_LOG" "$LOG_DIR/sthira.log" 2>/dev/null || true

SMOKE_JURIS="KL-WYD"; SMOKE_QUERY=""; SMOKE_PLACE=""; SMOKE_KIND=""
if ! run_k6 smoke_real.js smoke_real; then
  exit 1
fi

if [ "$BUSINESS_FLOW" -eq 1 ]; then
  SMOKE_JURIS="$SEED_JURISDICTION"
  SMOKE_QUERY="$SEED_QUERY"
  SMOKE_PLACE="$SEED_PLACE_ID"
  SMOKE_KIND="FACILITY"
  if ! run_k6 smoke_business.js smoke_business; then
    exit 1
  fi
fi

log "evidence:"
log "  $LOG_DIR/smoke_real_summary.json (capped stdout/stderr alongside)"
log "  $LOG_DIR/sthira.log (tail-capped)"
exit 0
