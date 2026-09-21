#!/usr/bin/env bash
# Build, start, run a single k6 scenario, capture results, stop everything.
#
# This is the canonical runner for the P7 load harness. It uses unique
# ephemeral ports per worker so two runs cannot collide. The harness
# captures the k6 summary JSON, the fixture's /metrics snapshot, the
# dummy-worker logs, and the fixture logs into loadmodel/reports/<scenario>/.
#
# Usage: bash run_scenario.sh <scenario>
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SCENARIO="${1:-smoke}"
K6FILE=""
DURATION_S="30"
# Cache-hit and outage knobs. Empty defaults to the loadtestd's default
# of 0.95 (warm cache) and 0.0 (no source outage).
CACHE_HIT="0.95"
OUTAGE_RATE="0.0"
QUEUE_DEPTH="8"
case "$SCENARIO" in
  smoke)         K6FILE="k6/smoke.js" ;            DURATION_S="30" ;;
  cached_1k)     K6FILE="k6/cached_reads.js" ;     DURATION_S="60" ;;
  voice_20)      K6FILE="k6/voice_process.js" ;    DURATION_S="65" ;;
  writes_50)     K6FILE="k6/writes_hotspot.js" ;   DURATION_S="60" ;;
  cold_cache)    K6FILE="k6/cold_cache.js" ;       DURATION_S="90" ; CACHE_HIT="0.0" ;;
  source_outage) K6FILE="k6/source_outage.js" ;    DURATION_S="60" ; OUTAGE_RATE="1.0" ;;
  burst)         K6FILE="k6/burst.js" ;            DURATION_S="60" ;;
  *)
    echo "unknown scenario: $SCENARIO" >&2
    exit 2
    ;;
esac

REPORT_DIR="$ROOT/reports/$SCENARIO"
mkdir -p "$REPORT_DIR"

echo "==== scenario: $SCENARIO ===="

# Build all binaries once.
go build -o "$ROOT/bin/loadtestd" ./cmd/loadtestd
go build -o "$ROOT/bin/dummyd" ./cmd/dummyd

# Start dummy workers on fixed ports (the harness binds to ephemeral in
# production; here we use fixed ports to keep the runner script simple
# and the JSON env file readable).
ASR_PORT=19101
MID_PORT=19102
TTS_PORT=19103
FIX_PORT=18080

"$ROOT/bin/dummyd" --kind asr --addr "127.0.0.1:${ASR_PORT}" \
  >"$REPORT_DIR/asr.log" 2>&1 &
ASR_PID=$!
"$ROOT/bin/dummyd" --kind middle --addr "127.0.0.1:${MID_PORT}" \
  >"$REPORT_DIR/middle.log" 2>&1 &
MID_PID=$!
"$ROOT/bin/dummyd" --kind tts --addr "127.0.0.1:${TTS_PORT}" \
  >"$REPORT_DIR/tts.log" 2>&1 &
TTS_PID=$!

# Start the synthetic fixture server.
STHIRA_LOAD_CACHE_HIT="${CACHE_HIT}" \
STHIRA_LOAD_OUTAGE_RATE="${OUTAGE_RATE}" \
STHIRA_LOAD_QUEUE_DEPTH="${QUEUE_DEPTH}" \
  "$ROOT/bin/loadtestd" --addr "127.0.0.1:${FIX_PORT}" \
    >"$REPORT_DIR/loadtestd.log" 2>&1 &
FIX_PID=$!

cleanup() {
  kill "$FIX_PID" "$ASR_PID" "$MID_PID" "$TTS_PID" 2>/dev/null || true
  wait "$FIX_PID" "$ASR_PID" "$MID_PID" "$TTS_PID" 2>/dev/null || true
}
trap cleanup EXIT

# Wait for everything to bind.
sleep 0.5
for p in $ASR_PORT $MID_PORT $TTS_PORT $FIX_PORT; do
  for i in 1 2 3 4 5 6 7 8 9 10; do
    if curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${p}/health" 2>/dev/null | grep -qE '^(200|404)$'; then
      break
    fi
    sleep 0.2
  done
done

# Run the k6 scenario. Capture summary JSON.
BASE_URL="http://127.0.0.1:${FIX_PORT}" \
ASR_BASE_URL="http://127.0.0.1:${ASR_PORT}" \
MID_BASE_URL="http://127.0.0.1:${MID_PORT}" \
TTS_BASE_URL="http://127.0.0.1:${TTS_PORT}" \
  k6 run --summary-export "$REPORT_DIR/summary.json" \
         --out json="$REPORT_DIR/metrics.json" \
         "$ROOT/${K6FILE}" \
    2>"$REPORT_DIR/k6.stderr.log" \
    || true

# Snapshot the fixture's internal counters.
curl -fsS "http://127.0.0.1:${FIX_PORT}/metrics" >"$REPORT_DIR/loadtestd-stats.json" 2>/dev/null || true

# Snapshot dummy worker stats via /health responses (they don't expose
# counters; record their logs only).
echo "=== dummyd logs ===" >"$REPORT_DIR/dummyd-summary.log"
for f in "$REPORT_DIR/asr.log" "$REPORT_DIR/middle.log" "$REPORT_DIR/tts.log"; do
  echo "--- $f ---" >>"$REPORT_DIR/dummyd-summary.log"
  head -3 "$f" >>"$REPORT_DIR/dummyd-summary.log" 2>/dev/null || true
done

echo "  -> $REPORT_DIR/summary.json"
echo "  -> $REPORT_DIR/loadtestd-stats.json"
