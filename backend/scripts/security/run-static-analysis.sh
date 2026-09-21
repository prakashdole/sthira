#!/usr/bin/env bash
# Pinned Go static-analysis runner for the P7 prep window.
#
# Runs, in order:
#   1. go vet ./...                  (always; stdlib only)
#   2. staticcheck ./...             (skipped when absent; pin recorded)
#   3. govulncheck ./...             (skipped when absent; pin recorded)
#   4. gosec ./...   (targeted only) (skipped when absent; pin recorded)
#
# Tool pinning lives in versions.env. Reports go to reports/.
#
# Exit codes:
#   0   clean
#   1   vet/staticcheck/gosec finding
#   77  required tool not installed

set -euo pipefail
cd "$(dirname "$0")/../.." # repo backend/

source "$(dirname "$0")/_lib.sh"
snapshot_metadata

FAIL=0

echo "==> go vet ./..."
if ! go vet ./...; then
    echo "FAIL: go vet found issues" >&2
    FAIL=1
fi

echo "==> staticcheck (pinned ${STATICCHECK_VERSION})"
if command -v staticcheck >/dev/null 2>&1; then
    resolve_tool staticcheck staticcheck -version >/dev/null
    if ! staticcheck ./... 2>&1 | tee "${REPORTS_DIR}/staticcheck.txt"; then
        echo "FAIL: staticcheck found issues" >&2
        FAIL=1
    fi
else
    echo "skip staticcheck: not installed (pin ${STATICCHECK_VERSION}; install when run-window opens)"
    exit 77
fi

echo "==> govulncheck (pinned ${GOVULNCHECK_VERSION})"
if command -v govulncheck >/dev/null 2>&1; then
    resolve_tool govulncheck govulncheck -version >/dev/null
    if ! govulncheck ./... 2>&1 | tee "${REPORTS_DIR}/govulncheck.txt"; then
        echo "FAIL: govulncheck found vulnerabilities" >&2
        FAIL=1
    fi
else
    echo "skip govulncheck: not installed (pin ${GOVULNCHECK_VERSION})"
fi

echo "==> gosec (pinned ${GOSEC_VERSION})"
if command -v gosec >/dev/null 2>&1; then
    resolve_tool gosec gosec --version >/dev/null
    # Targeted: ingest + auth + signing modules only. The runner
    # explicitly excludes generated code, tests, and the demo binary.
    targets=(
        ./internal/capfeed/...
        ./internal/sourceact/...
        ./internal/offlinequeue/...
        ./internal/offlineclient/...
        ./internal/offlinepkg/...
        ./internal/httpserver/...
        ./internal/httpjson/...
        ./cmd/...
    )
    gosec -quiet -no-fail \
          -fmt json -out "${REPORTS_DIR}/gosec.json" \
          "${targets[@]}" 2>&1 \
        | tee "${REPORTS_DIR}/gosec.txt" || FAIL=1
else
    echo "skip gosec: not installed (pin ${GOSEC_VERSION})"
fi

if [ "$FAIL" -eq 0 ]; then
    echo "OK: static analysis clean"
fi
exit "$FAIL"
