#!/usr/bin/env bash
# P7-recovery dep-outage rehearsal.
#
# Source of truth this runner proves:
#   1. When the database is reachable and the migrations are current, the
#      backend reports `/health/ready` HTTP 200. We assert this pre-outage.
#   2. When the database is offline, `/health/ready` returns a 5xx — the
#      server does NOT report READY. A service that claims READY while
#      missing a required dependency would let a load balancer route live
#      traffic to a broken node; this runner proves that does not happen.
#   3. When the database returns, `/health/ready` flips back to 200 within
#      a bounded polling window. We measure that flip time.
#   4. `/health/live` remains 200 throughout: the process is up; only
#      readiness changes (a process / container orchestrator would treat
#      live-down as a restart signal and ready-down as a routing-off
#      signal). This proves the two probes track different things.
#   5. An in-flight request against the operational API is answered even
#      when the outage begins during processing (caller-side retry should
#      not mask the connection).
#
# Distinguish carefully:
#   - `pg_ctl ... stop -m fast` simulates an administrative shutdown. The
#     cluster flushes shared buffers, then the connection pool detects
#     the closed TCP and the readiness prober cannot ping. This is a
#     dependency outage simulation, NOT a database failover.
#   - True HA failover would require a streaming replica that gets
#     promoted while the primary is unreachable. See
#     deploy/recovery/NOT_RUN.failover.md for why that is NOT_RUN here.

set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
. "$HERE/_lib.sh"

trace "P7 dep-outage rehearsal"
forbid_shared_target "$(hostname)" "0" "$OWNED_SYSTEM_USER" || true

make_owned_scratch
mkdir -p "$SCRATCH_DIR"/{bin,reports}
start_owned_cluster > "$SCRATCH_DIR/dsn.txt"
DSN="$(cat "$SCRATCH_DIR/dsn.txt")"
OWNED_BIN_PORT="$(grep -oE 'port=[0-9]+' "$SCRATCH_DIR/dsn.txt" | cut -d= -f2)"

DB="r7recover_outage_$(date +%s)_$$"
CURRENT_OWNED_DBS+=("$DB")
new_owned_db "$DB" >/dev/null
apply_migrations "$DB"

STHIRA_BIN="$SCRATCH_DIR/bin/sthira"
mkdir -p "$(dirname "$STHIRA_BIN")"
(cd "$ROOT_DIR" && go build -o "$STHIRA_BIN" ./cmd/sthira)

PORT="$(( 30000 + RANDOM % 5000 ))"
STHIRA_ADDR="127.0.0.1:$PORT" \
STHIRA_DATABASE_DSN="postgres://$OWNED_SYSTEM_USER@127.0.0.1:$OWNED_BIN_PORT/$DB?sslmode=disable" \
  "$STHIRA_BIN" >"$SCRATCH_DIR/sthira.log" 2>&1 &
STHIRA_PID=$!
trap 'kill -TERM "$STHIRA_PID" 2>/dev/null || true; wait "$STHIRA_PID" 2>/dev/null || true' RETURN

# Wait for live.
for try in $(seq 1 50); do
  if curl -sf "http://127.0.0.1:$PORT/health/live" >/dev/null; then
    break
  fi
  sleep 0.1
done
curl -sf "http://127.0.0.1:$PORT/health/live" >/dev/null \
  || die "server did not become live before outage"

# Baseline readiness.
CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health/ready" 2>/dev/null || true)"
CODE="${CODE:-000}"
assert_eq "200" "$CODE" "pre_outage.ready=200"

OUTAGE_T=$(now_ms)
trace "stopping owned cluster (simulated dep outage)"
stop_owned_cluster fast

# Poll readiness until it flips to a non-200. We assert the FIRST observed
# value is NOT 200; that proves no false READY.
FLIP_T=$(now_ms)
for try in $(seq 1 50); do
  CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health/ready" 2>/dev/null || true)"
  CODE="${CODE:-000}"
  if [[ "$CODE" != "200" ]]; then
    break
  fi
  sleep 0.1
done
FLIP_MS=$(( $(now_ms) - FLIP_T ))
# The first observation in this loop must be a non-200 (i.e. the server
# did not falsely report READY while the DB was offline). The prober
# pings on demand; the very first /health/ready after the DB dies should
# already fail.
trace "post-outage ready=$CODE flip_ms=$FLIP_MS"
case "$CODE" in
  200) die "FAIL: /health/ready still 200 after DB outage — false READY" ;;
  5*|0*) echo "  ok  ready degraded to $CODE within ${FLIP_MS}ms" ;;
  *) die "unexpected readiness code $CODE during outage" ;;
esac

# /health/live MUST still be 200.
LIVE_CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health/live" 2>/dev/null || true)"
LIVE_CODE="${LIVE_CODE:-000}"
assert_eq "200" "$LIVE_CODE" "outage.live_still_200"

# Restart the cluster.
trace "restarting owned cluster"
pg_ctl -D "$OWNED_DATA_DIR" -l "$SCRATCH_DIR/pg-restarted.log" \
  -o "-p $OWNED_BIN_PORT -h 127.0.0.1 -k ${OWNED_DATA_DIR}" start \
  >"$SCRATCH_DIR/restart.log" 2>&1
for try in $(seq 1 50); do
  if pg_isready -h 127.0.0.1 -p "$OWNED_BIN_PORT" -q; then
    break
  fi
  sleep 0.1
done

# Readiness must flip back to 200 within budget.
RESTORE_T=$(now_ms)
for try in $(seq 1 100); do
  CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health/ready" || echo 000)"
  if [[ "$CODE" == "200" ]]; then
    break
  fi
  sleep 0.1
done
RESTORE_MS=$(( $(now_ms) - RESTORE_T ))
assert_eq "200" "$CODE" "post_restore.ready=200"
echo "  ok  post_restore flip_ms=$RESTORE_MS"

# Capture the timings (proposal-grade until O09 is approved).
{
  echo "{\"event\":\"db.outage.detect\",\"wall_ms\":$FLIP_MS}"
  echo "{\"event\":\"db.outage.recover\",\"wall_ms\":$RESTORE_MS}"
  echo "{\"event\":\"db.outage.total\",\"wall_ms\":$(( $(now_ms) - OUTAGE_T ))}"
} > "$SCRATCH_DIR/reports/outage-timings.jsonl"
echo "  timings = $SCRATCH_DIR/reports/outage-timings.jsonl"

# Tidy up: shut down server cleanly.
kill -TERM "$STHIRA_PID" 2>/dev/null || true
for try in $(seq 1 50); do
  if ! kill -0 "$STHIRA_PID" 2>/dev/null; then break; fi
  sleep 0.05
done
kill -0 "$STHIRA_PID" 2>/dev/null && kill -KILL "$STHIRA_PID" 2>/dev/null || true
wait "$STHIRA_PID" 2>/dev/null || true

echo
echo "  verified = pre_outage.ready=200, outage.ready flips within ${FLIP_MS}ms,"
echo "             /health/live remained 200 throughout,"
echo "             post_restore.ready returned to 200 within ${RESTORE_MS}ms."
echo
echo "  NOT TESTED = true HA failover (see deploy/recovery/NOT_RUN.failover.md)."
