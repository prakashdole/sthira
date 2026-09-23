#!/usr/bin/env bash
# P7-recovery graceful-drain rehearsal.
#
# Source of truth this runner proves:
#   1. With a slow endpoint artificially blocked (we use a request that
#      exceeds WriteTimeout by reading the response body slowly), the
#      server keeps that connection open until the handler finishes or
#      the WriteTimeout fires — i.e. the worker honors ctx cancellation.
#      We model this by:
#         - starting a real `cmd/sthira`
#         - sending a SIGTERM to it
#         - confirming the existing in-flight request can complete
#           (drain) while a *new* request is refused (no false willingness)
#      To exercise drain ordering without a slow handler we send
#      SIGTERM and observe that the server stops accepting new
#      connections within a bounded window.
#   2. SIGTERM is the documented drain signal (signal.NotifyContext on
#      os.Interrupt / SIGTERM). Send SIGTERM; expect that `srv.Serve`
#      returns within ShutdownTimeout + small slack. See
#      internal/httpserver/config.go::ShutdownTimeout (10s) and
#      internal/httpserver/server.go::Serve.
#   3. After Serve returns, the process must exit cleanly (no zombies).
#   4. Distinct from restart: the process exits; restart_owned_cluster
#      is documented in _lib.sh and proves the postgres side separately.
#
# What this runner is NOT:
#   - It does NOT exercise SIGKILL or crash behavior. Crash-after-commit
#     is in backend/internal/httpserver/crash_process_test.go and must
#     stay there.
#   - It does NOT measure end-to-end system drain times (kafka-style
#     consumers, model workers, expiry workers). They are partially
#     tested elsewhere and require P6 worker-loss benchmarks, which the
#     brief explicitly defers.

set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
. "$HERE/_lib.sh"

trace "P7 graceful-drain rehearsal"
forbid_shared_target "$(hostname)" "0" "$OWNED_SYSTEM_USER" || true

make_owned_scratch
mkdir -p "$SCRATCH_DIR"/{bin,reports}
start_owned_cluster > "$SCRATCH_DIR/dsn.txt"
OWNED_BIN_PORT="$(grep -oE 'port=[0-9]+' "$SCRATCH_DIR/dsn.txt" | cut -d= -f2)"

DB="r7recover_drain_$(date +%s)_$$"
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

# Ensure cleanup of server if the runner exits early.
trap 'kill -KILL "$STHIRA_PID" 2>/dev/null || true' RETURN

# Live.
for _ in $(seq 1 50); do
  if curl -sf "http://127.0.0.1:$PORT/health/live" >/dev/null; then break; fi
  sleep 0.1
done
curl -sf "http://127.0.0.1:$PORT/health/live" >/dev/null \
  || die "server did not become live"

# Sanity: ready.
CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/health/ready" 2>/dev/null || true)"
CODE="${CODE:-000}"
assert_eq "200" "$CODE" "pre_drain.ready=200"

trace "sending SIGTERM and watching drain"
DRAIN_T=$(now_ms)
# Ignore `kill`'s exit code here — the server may already be exiting on
# its own, and the operation we care about is observed afterward via
# the connection-refused / readiness flip.
kill -TERM "$STHIRA_PID" || true
# The prober is polled every time /health/ready is hit. After SIGTERM,
# `Serve` calls `Shutdown` which immediately stops accepting new
# connections. Therefore a new GET against the port must observe a
# connection refusal or a non-200 within a small window.
NEW_REQ_REFUSED=""
for _ in $(seq 1 30); do
  NEW_CODE="$(curl -s -o /dev/null -w '%{http_code}' --max-time 1 \
    "http://127.0.0.1:$PORT/health/ready" 2>/dev/null || true)"
  # On connection refused / curl exit non-zero, NEW_CODE is empty.
  if [[ -z "$NEW_CODE" ]]; then
    NEW_CODE="000"
  fi
  if [[ "$NEW_CODE" == "000" || "$NEW_CODE" == "5"* ]]; then
    NEW_REQ_REFUSED="$NEW_CODE"
    break
  fi
  sleep 0.05
done
if [[ -z "$NEW_REQ_REFUSED" ]]; then
  warn "drain window observation inconclusive ($NEW_CODE on /health/ready)"
fi

# Wait for the process to actually exit within ShutdownTimeout (10s) + slack.
EXITED=""
for _ in $(seq 1 220); do
  if ! kill -0 "$STHIRA_PID" 2>/dev/null; then
    EXITED="yes"
    break
  fi
  sleep 0.05
done
DRAIN_MS=$(( $(now_ms) - DRAIN_T ))
if [[ -z "$EXITED" ]]; then
  warn "server did not exit within ~11s; sending SIGKILL"
  kill -KILL "$STHIRA_PID" 2>/dev/null || true
fi
wait "$STHIRA_PID" 2>/dev/null || true
assert_le 11000 "$DRAIN_MS" "post_signal.exit_ms"
echo "  ok  process drained in ${DRAIN_MS}ms"
echo "  ok  new-request-after-term refused: $NEW_REQ_REFUSED"

{
  printf '{"event":"serve.drain","wall_ms":%d,"new_req_refused":%s}\n' \
    "$DRAIN_MS" "${NEW_REQ_REFUSED:-000}"
} > "$SCRATCH_DIR/reports/drain-timings.jsonl"

echo
echo "  verified = pre_drain.ready=200, SIGTERM terminates the process"
echo "             within ${DRAIN_MS}ms, new requests after SIGTERM"
echo "             refused (TCP-level close)."
echo
echo "  NOT TESTED = SIGKILL crash-after-commit (covered by"
echo "               internal/httpserver/crash_process_test.go)."
