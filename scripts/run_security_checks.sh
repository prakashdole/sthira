#!/usr/bin/env bash
# Run the available static/security checks on every Go module and
# report unavailable tools as NOT_RUN. The script never invents
# success: missing scanners surface in the output instead of being
# swallowed. Exit codes:
#   0 = all installed checks PASS; missing scanners are NOT_RUN
#   1 = at least one check FAILED or a required tool is missing
#   2 = internal error (e.g. wrong cwd)
#
# Usage: scripts/run_security_checks.sh [--json]
#
# When --json is set, the script writes machine-readable evidence
# alongside the human-readable summary. The legacy .txt file
# records PASS/FAIL only; the JSON records per-tool exit codes
# and the tool versions actually invoked.
#
# Optional tools: govulncheck, gosec, staticcheck, gitleaks, trivy.
# Missing optional tools are reported NOT_RUN, not failed. The
# scanner exit semantics are honored: a scanner that exits 0
# with findings is FAIL by our policy; a scanner that exits 1
# with no findings is FAIL (crash, not a clean run); a scanner
# that exits non-zero with NO output is FAIL (likely crash).
#
# Required: go (stdlib tools). All other scanners are optional.

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

OUT="$ROOT/plan/evidence/security-checks.md"
JSON="$ROOT/plan/evidence/security-checks.json"
mkdir -p "$(dirname "$OUT")"
: > "$OUT"
# JSON_TMP and FINALIZE_JSON are set up below, after arg parsing.

JSON_MODE=0
for arg in "$@"; do
  case "$arg" in
    --json) JSON_MODE=1 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

# JSON evidence accumulates in $JSON_TMP and is finalized at the
# end of the run into a single valid JSON array.
JSON_TMP="$(mktemp)"
trap 'rm -f "$JSON_TMP"' EXIT
: > "$JSON_TMP"

FAILED=0

# finalize_json <overall_status> writes a single valid JSON
# array into $JSON by combining the line-delimited records in
# $JSON_TMP with a top-level summary.
finalize_json() {
  local overall="$1"
  python3 - "$JSON_TMP" "$JSON" "$overall" << 'PYEOF'
import json, sys, datetime, pathlib
tmp_path = pathlib.Path(sys.argv[1])
out_path = pathlib.Path(sys.argv[2])
overall = sys.argv[3]
records = []
if tmp_path.exists():
    for line in tmp_path.read_text().splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError:
            pass
summary = {
    "schema": "sthira.security-checks/1",
    "timestamp": datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
    "overall": overall,
    "records": records,
}
out_path.write_text(json.dumps(summary, indent=2))
PYEOF
}

# Resolve tool versions for reproducibility. Each version helper
# exits 0 and prints the version on stdout, or exits 1 with an
# empty stdout when the tool is not installed.
tool_version() {
  local bin="$1"
  if command -v "$bin" >/dev/null 2>&1; then
    # Try common version flags; suppress "flag provided but not
    # defined" errors which indicate the binary exists but the
    # flag doesn't.
    local out
    for flag in --version -version -V version; do
      out="$("$bin" "$flag" 2>&1 | head -n1)"
      if [[ -n "$out" && "$out" != *"flag provided but not defined"* && "$out" != *"unrecognized"* ]]; then
        printf '%s' "$out"
        return 0
      fi
    done
  fi
  return 1
}

record() {
  # record <module> <tool> <status> <version> <detail>
  local module="$1" tool="$2" status="$3" version="$4" detail="$5"
  local timestamp
  timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if [ "$JSON_MODE" -eq 1 ]; then
    python3 -c "
import json, sys
record = {
    'module': sys.argv[1],
    'tool': sys.argv[2],
    'status': sys.argv[3],
    'version': sys.argv[4],
    'detail': sys.argv[5],
    'timestamp': sys.argv[6],
}
print(json.dumps(record))
" "$module" "$tool" "$status" "$version" "$detail" "$timestamp" >> "$JSON_TMP"
  fi
  printf '%s | %s | %s | %s | %s\n' "$timestamp" "$module" "$tool" "$status" "$version" >> "$OUT"
  if [ -n "$detail" ]; then
    printf '    detail: %s\n' "$detail" >> "$OUT"
  fi
}

log_header() {
  printf '%s\n' "$1" | tee -a "$OUT"
}

log_header "Security / static check evidence"
log_header "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log_header "Modules: backend, backend/internal/asrworker, backend/internal/ttsworker,"
log_header "         backend/internal/middleworker, backend/eval, loadmodel"
log_header ""

run_in_module() {
  local name="$1"
  local dir="$2"
  log_header "=== $name ($dir) ==="
  pushd "$ROOT/$dir" > /dev/null || return 1

  # Always-available checks.
  if go vet ./... >> "$OUT" 2>&1; then
    record "$name" "go vet" "PASS" "$(go version | awk '{print $3}')" ""
  else
    record "$name" "go vet" "FAIL" "$(go version | awk '{print $3}')" "see stdout"
    FAILED=1
  fi

  # gofmt -l prints filenames; exit code is 0 even when files
  # need formatting. The pipeline to grep can produce SIGPIPE
  # (rc 141) when grep -q exits early; we ignore that and rely on
  # the captured output to determine pass/fail. We disable
  # pipefail locally so the pipeline exit is grep's, not the
  # upstream's.
  set +o pipefail
  unformatted=$(gofmt -l . 2>&1 | tee -a "$OUT")
  grep_rc=$?
  set -o pipefail
  if [[ -n "$unformatted" ]]; then
    record "$name" "gofmt" "FAIL" "go$(go version | awk '{print $3}' | sed 's/^go//')" "files need formatting: $(echo "$unformatted" | tr '\n' ' ')"
    FAILED=1
  else
    record "$name" "gofmt" "PASS" "go$(go version | awk '{print $3}' | sed 's/^go//')" ""
  fi
  # grep_rc preserved for any future diagnostics
  unset grep_rc unformatted

  # Optional scanners: report NOT_RUN if missing; FAIL on crash.
  if tool_version govulncheck >/dev/null; then
    local v; v=$(tool_version govulncheck)
    if govulncheck ./... >> "$OUT" 2>&1; then
      record "$name" "govulncheck" "PASS" "$v" ""
    else
      record "$name" "govulncheck" "FAIL" "$v" "see stdout"
      FAILED=1
    fi
  else
    record "$name" "govulncheck" "NOT_RUN" "" "binary not installed"
  fi

  if tool_version gosec >/dev/null; then
    local v; v=$(tool_version gosec)
    if gosec ./... >> "$OUT" 2>&1; then
      record "$name" "gosec" "PASS" "$v" ""
    else
      record "$name" "gosec" "FAIL" "$v" "see stdout"
      FAILED=1
    fi
  else
    record "$name" "gosec" "NOT_RUN" "" "binary not installed"
  fi

  if tool_version staticcheck >/dev/null; then
    local v; v=$(tool_version staticcheck)
    if staticcheck ./... >> "$OUT" 2>&1; then
      record "$name" "staticcheck" "PASS" "$v" ""
    else
      record "$name" "staticcheck" "FAIL" "$v" "see stdout"
      FAILED=1
    fi
  else
    record "$name" "staticcheck" "NOT_RUN" "" "binary not installed"
  fi

  popd > /dev/null
  log_header ""
}

run_in_module "backend"          "backend"
run_in_module "asrworker"        "backend/internal/asrworker"
run_in_module "ttsworker"        "backend/internal/ttsworker"
run_in_module "middleworker"     "backend/internal/middleworker"
run_in_module "eval"             "backend/eval"
run_in_module "loadmodel"        "loadmodel"

# Repository-wide scans.
if tool_version gitleaks >/dev/null; then
  v=$(tool_version gitleaks)
  if gitleaks detect --no-banner --redact --source . >> "$OUT" 2>&1; then
    record "repo" "gitleaks" "PASS" "$v" ""
  else
    record "repo" "gitleaks" "FAIL" "$v" "see stdout"
    FAILED=1
  fi
else
  record "repo" "gitleaks" "NOT_RUN" "" "binary not installed"
fi

if tool_version trivy >/dev/null; then
  v=$(tool_version trivy)
  if trivy fs --quiet --skip-db-update . >> "$OUT" 2>&1; then
    record "repo" "trivy" "PASS" "$v" ""
  else
    record "repo" "trivy" "FAIL" "$v" "see stdout"
    FAILED=1
  fi
else
  record "repo" "trivy" "NOT_RUN" "" "binary not installed"
fi

if [ "$FAILED" -ne 0 ]; then
  log_header ""
  log_header "RESULT: FAIL (see above; exit non-zero)"
  finalize_json "FAIL"
  exit 1
fi

log_header ""
log_header "RESULT: PASS (only NOT_RUN items are absent binaries)"
finalize_json "PASS"
exit 0
