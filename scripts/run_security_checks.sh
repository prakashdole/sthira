#!/usr/bin/env bash
# Run the available static/security checks on every Go module and
# report unavailable tools as NOT_RUN. The script never invents
# success: missing scanners surface in the output instead of being
# swallowed. Always exit non-zero if any check fails or a required
# tool is missing.
#
# Usage: scripts/run_security_checks.sh
#
# Required: go (stdlib tools).
# Optional: govulncheck, gosec, staticcheck, gitleaks, trivy.
# Missing optional tools are reported NOT_RUN, not failed.

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

OUT="$ROOT/plan/evidence/security-checks.txt"
mkdir -p "$(dirname "$OUT")"
: > "$OUT"

log() { printf '%s\n' "$*" | tee -a "$OUT"; }

log "Security / static check evidence"
log "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "Modules: backend, backend/internal/asrworker, backend/internal/ttsworker,"
log "         backend/internal/middleworker, backend/eval, loadmodel"
log ""

run_in_module() {
  local name="$1"
  local dir="$2"
  log "=== $name ($dir) ==="
  pushd "$ROOT/$dir" > /dev/null || return 1

  # Always-available checks.
  if go vet ./... >> "$OUT" 2>&1; then
    log "go vet ./... PASS"
  else
    log "go vet ./... FAIL"
    FAILED=1
  fi

  if gofmt -l . 2>&1 | tee -a "$OUT" | grep -q .; then
    log "gofmt -l . FAIL (above files need formatting)"
    FAILED=1
  else
    log "gofmt -l . PASS"
  fi

  # Optional scanners: report NOT_RUN if missing.
  if command -v govulncheck >/dev/null 2>&1; then
    if govulncheck ./... >> "$OUT" 2>&1; then
      log "govulncheck ./... PASS"
    else
      log "govulncheck ./... FAIL"
      FAILED=1
    fi
  else
    log "govulncheck NOT_RUN (binary not installed)"
  fi

  if command -v gosec >/dev/null 2>&1; then
    if gosec ./... >> "$OUT" 2>&1; then
      log "gosec ./... PASS"
    else
      log "gosec ./... FAIL"
      FAILED=1
    fi
  else
    log "gosec NOT_RUN (binary not installed)"
  fi

  if command -v staticcheck >/dev/null 2>&1; then
    if staticcheck ./... >> "$OUT" 2>&1; then
      log "staticcheck ./... PASS"
    else
      log "staticcheck ./... FAIL"
      FAILED=1
    fi
  else
    log "staticcheck NOT_RUN (binary not installed)"
  fi

  popd > /dev/null
  log ""
}

FAILED=0

run_in_module "backend"          "backend"
run_in_module "asrworker"        "backend/internal/asrworker"
run_in_module "ttsworker"        "backend/internal/ttsworker"
run_in_module "middleworker"     "backend/internal/middleworker"
run_in_module "eval"             "backend/eval"
run_in_module "loadmodel"        "loadmodel"

if command -v gitleaks >/dev/null 2>&1; then
  if gitleaks detect --no-banner --redact --source . >> "$OUT" 2>&1; then
    log "gitleaks detect PASS"
  else
    log "gitleaks detect FAIL"
    FAILED=1
  fi
else
  log "gitleaks NOT_RUN (binary not installed)"
fi

if command -v trivy >/dev/null 2>&1; then
  if trivy fs --quiet --skip-db-update . >> "$OUT" 2>&1; then
    log "trivy fs PASS"
  else
    log "trivy fs FAIL"
    FAILED=1
  fi
else
  log "trivy fs NOT_RUN (binary not installed)"
fi

if [ "$FAILED" -ne 0 ]; then
  log ""
  log "RESULT: FAIL (see above; exit non-zero)"
  exit 1
fi

log ""
log "RESULT: PASS (only NOT_RUN items are absent binaries)"
exit 0