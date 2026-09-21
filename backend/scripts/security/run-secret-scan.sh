#!/usr/bin/env bash
# Secret scanner — gitleaks (pinned).
#
# Run from the repo root with:
#   ./backend/scripts/security/run-secret-scan.sh
#
# Findings ARE recorded to reports/gitleaks.json; a deliberate step
# (REDACT_LEAKS) replaces raw secret bytes with sha256:[HEX-64] before
# the report is committed. The brief: "Do not print secrets or
# suppress findings wholesale." Suppression is per-rule, optional,
# recorded in reports/gitleaks-suppressions.toml with a signed reason.

set -euo pipefail
cd "$(dirname "$0")/../.."  # repo root

source "$(dirname "$0")/_lib.sh"
snapshot_metadata

echo "==> gitleaks detect (pinned ${GITLEAKS_VERSION})"
if command -v gitleaks >/dev/null 2>&1; then
    resolve_tool gitleaks gitleaks version >/dev/null
    if ! gitleaks detect \
            --no-banner \
            --redact \
            --report-format json \
            --report-path "${REPORTS_DIR}/gitleaks.json" \
            --config "${SECURITY_DIR}/gitleaks.toml"; then
        echo "FAIL: gitleaks found leaks" >&2
        exit 1
    fi
    echo "OK: gitleaks clean"
else
    echo "skip gitleaks: not installed (pin ${GITLEAKS_VERSION})"
    exit 77
fi
