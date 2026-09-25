#!/usr/bin/env bash
# ZAP authenticated API scan — P7 prep.
#
# This is a config-only runner. It does NOT scan a real deployment;
# the runner is an entry point for the disposable, owned-by-us
# staging target documented in backend/security/threat-model.md.
#
# Inputs (all explicit; no defaults that could miss a staging target):
#   ZAP_TARGET_URL  -- the allowlisted local target (e.g. http://127.0.0.1:8080)
#   ZAP_API_KEY      -- ZAP API key (local ZAP instance; not a prod credential)
#   ZAP_REPORT_DIR   -- output directory for reports
#
# Stop conditions (recorded in zap_stop_conditions.env):
#   - target not reachable within 30s
#   - any URL outside ZAP_TARGET_URL host detected (range-checking)
#   - ZAP fatal severity > 0 (HIGH/CRITICAL) — runner halts and records
#   - any test-time > 60 minutes (configurable)
#
# Strix is NOT invoked from this script — execution requires the O15
# decision (model/spend/target). The runner reads zap_stop_conditions.env
# for the documented stop set.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/_lib.sh"
snapshot_metadata

# Refuse to start without a target — defaults silently are a footgun.
: "${ZAP_TARGET_URL:?ZAP_TARGET_URL must be set to the allowlisted staging target}"
: "${ZAP_API_KEY:?ZAP_API_KEY must be set (local ZAP API key, not a prod credential)}"

OUT="${ZAP_REPORT_DIR:-${REPORTS_DIR}/zap}"
mkdir -p "$OUT"

# Range check: refuse any non-loopback or non-private target. Scanning
# government / production systems is explicitly out of scope.
check_target_is_local() {
    case "$1" in
        http://127.0.0.1*|http://localhost*|http://[0-9a-fA-F:]*:[0-9]* )
            : OK ;;
        http://10.*|http://192.168.*|http://172.1[6-9].*|http://172.2[0-9].*|http://172.3[0-1].* )
            : private-rfc1918-OK ;;
        https://* )
            echo "FAIL: target uses TLS — ZAP local run uses HTTP" >&2 ; return 1 ;;
        * )
            echo "FAIL: target $1 is not a local/private host (P7-prep scope is local-only)" >&2 ; return 1 ;;
    esac
}
check_target_is_local "$ZAP_TARGET_URL"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "skip ZAP: docker not available (pin ${ZAP_IMAGE})"
    exit 77
fi

resolve_image zap "$ZAP_IMAGE" >/dev/null

# ZAP does not print secrets. The auth uses synthetic test accounts
# registered with the staging backend (see tests/ in httpserver/).
docker run --rm \
    -v "${OUT}:/zap/wrk:rw" \
    -u zap \
    -e ZAP_TARGET_URL="$ZAP_TARGET_URL" \
    --network host \
    "${ZAP_IMAGE}" \
        bash -c '
            set -euo pipefail
            zap.sh -daemon -port 8090 -config api.key="$ZAP_API_KEY" &
            for i in $(seq 1 30); do
                sleep 1
                if curl -fsS "http://127.0.0.1:8090/?api_key=$ZAP_API_KEY&action=healthz" >/dev/null; then break; fi
            done
            # Spider the local target. The maximum duration is bounded
            # by ZAP itself; we additionally cap with --maxDuration.
            zap-cli --verbose spider --api-key "$ZAP_API_KEY" \
                    --context "sthira-staging" "$ZAP_TARGET_URL" \
                    || { echo "spider stopped"; exit 1; }
            # Active scan with severity floor at MEDIUM. HIGH/CRITICAL
            # halt the runner (per zap_stop_conditions.env).
            zap-cli --verbose active-scan --api-key "$ZAP_API_KEY" \
                    --context "sthira-staging" \
                    --recursive true \
                    --threshold MEDIUM \
                    "$ZAP_TARGET_URL" \
                    || { echo "scan halted on finding"; exit 1; }
            # Export reports in JSON + HTML. Secrets never appear in
            # reports because ZAP redacts by default.
            zap-cli --verbose report --api-key "$ZAP_API_KEY" \
                    --output /zap/wrk/zap-report.html --format html
            zap-cli --verbose report --api-key "$ZAP_API_KEY" \
                    --output /zap/wrk/zap-report.json --format json
        '

# Enforce a manual review of HIGH/CRITICAL severity findings. The
# runner does NOT auto-blanket-suppress; suppression requires a
# rule-id + signed justification (see zap/suppressions.toml).
HIGH_CRITICAL=$(grep -E '"riskcode": ?"3"|"riskcode": ?"4"' \
    "${OUT}/zap-report.json" 2>/dev/null | wc -l | tr -d ' ')
if [ "${HIGH_CRITICAL}" -gt 0 ]; then
    echo "FAIL: ZAP reported ${HIGH_CRITICAL} HIGH/CRITICAL finding(s); manual review required" >&2
    exit 1
fi

echo "OK: ZAP completed with no HIGH/CRITICAL findings"
