#!/usr/bin/env bash
# Run the available static/security checks on every Go module and
# report unavailable tools as NOT_RUN. The script never invents
# success. Exit codes (2026-09-21 semantics repair):
#   0 = every check that is REQUIRED to run was run and PASSED
#   1 = at least one executed check FAILED
#   2 = internal error (e.g. wrong cwd, bad usage)
#   4 = INCOMPLETE: nothing failed, but one or more REQUIRED
#       scanner checks were NOT_RUN — that is NOT a clean pass and
#       must never be reported as PASS.
#
# Usage: scripts/run_security_checks.sh [--json]
#
# Evidence paths can be redirected (used by the deterministic
# self-test): STHIRA_SECURITY_MD / STHIRA_SECURITY_JSON.
#
# Scanner exit semantics (policy per tool, verified against each
# tool's documented behaviour):
#   govulncheck  rc0 clean | rc3 vulnerabilities found (FAIL) |
#                other rc = tool error/crash (FAIL, distinct detail)
#   gosec        rc0 clean | rc1 issues found (FAIL) | other = crash
#   staticcheck  rc0 clean | rc1 findings (FAIL) | other = crash
#   gitleaks     rc0 clean | rc1 leaks found (FAIL) | other = crash
#   trivy        ALWAYS rc0 without --exit-code: findings are only
#                in the output table ("Total: N"), so output is
#                parsed; a real tool error yields rc>=1 (crash).
# A tool that crashes never counts as PASS. Version + resolved path
# are recorded per invocation for provenance.

set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

OUT="${STHIRA_SECURITY_MD:-$ROOT/plan/evidence/security-checks.md}"
JSON="${STHIRA_SECURITY_JSON:-$ROOT/plan/evidence/security-checks.json}"
mkdir -p "$(dirname "$OUT")" "$(dirname "$JSON")"
: > "$OUT"

JSON_MODE=0
for arg in "$@"; do
  case "$arg" in
    --json) JSON_MODE=1 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done
# Evidence is always machine-readable for CI.
JSON_MODE=1

JSON_TMP="$(mktemp)"
trap 'rm -f "$JSON_TMP"' EXIT
: > "$JSON_TMP"

FAILED=0
NOT_RUN_REQUIRED=0

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
    "schema": "sthira.security-checks/2",
    "timestamp": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "overall": overall,
    "records": records,
}
out_path.write_text(json.dumps(summary, indent=2))
PYEOF
}

tool_path() { command -v "$1" 2>/dev/null || echo ""; }

tool_version() {
  local bin="$1"
  local path
  path="$(tool_path "$bin")"
  [ -z "$path" ] && return 1
  local out
  for flag in --version -version -V version; do
    out="$("$bin" "$flag" 2>&1 | head -n1 || true)"
    if [ -n "$out" ] && [[ "$out" != *"flag provided but not defined"* && "$out" != *"unrecognized"* ]]; then
      printf '%s (%s)' "$out" "$path"
      return 0
    fi
  done
  printf '%s' "$path"
  return 0
}

record() {
  local module="$1" tool="$2" status="$3" version="$4" detail="$5"
  local timestamp
  timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if [ "$JSON_MODE" -eq 1 ]; then
    python3 -c "
import json, sys
print(json.dumps({
    'module': sys.argv[1], 'tool': sys.argv[2], 'status': sys.argv[3],
    'version': sys.argv[4], 'detail': sys.argv[5], 'timestamp': sys.argv[6],
}))
" "$module" "$tool" "$status" "$version" "$detail" "$timestamp" >> "$JSON_TMP"
  fi
  printf '%s | %s | %s | %s | %s\n' "$timestamp" "$module" "$tool" "$status" "$version" >> "$OUT"
  if [ -n "$detail" ]; then
    printf '    detail: %s\n' "$detail" >> "$OUT"
  fi
}

# run_scanner <module-label> <tool> <clean_rc_set semantics: args...>
# Implements the per-tool exit semantics documented at the top.
run_scanner() {
  local label="$1" tool="$2"; shift 2
  local v path out rc
  v="$(tool_version "$tool")" || { record "$label" "$tool" "NOT_RUN" "" "binary not installed"; return 2; }
  path="$(tool_path "$tool")"
  out="$("$@" 2>&1)"; rc=$?
  printf '%s\n' "$out" >> "$OUT"
  case "$tool:$rc" in
    trivy:0)
      # Trivy exits 0 even WITH findings; parse the summary table.
      if printf '%s' "$out" | grep -qE 'TOTAL:[[:space:]]*[1-9][0-9]*|Total:[[:space:]]*[1-9][0-9]*[[:space:]]*\('; then
        record "$label" "$tool" "FAIL" "$v" "findings present in output (trivy rc0 with Total>0)"
        return 1
      fi
      record "$label" "$tool" "PASS" "$v" "rc0, no findings table entries; path=$path"
      return 0 ;;
    govulncheck:0|gosec:0|staticcheck:0|gitleaks:0)
      record "$label" "$tool" "PASS" "$v" "path=$path"
      return 0 ;;
    govulncheck:3|gosec:1|staticcheck:1|gitleaks:1)
      record "$label" "$tool" "FAIL" "$v" "scanner reported findings (documented exit $rc); path=$path"
      return 1 ;;
    trivy:*)
      # Non-zero for trivy fs means tool/db error (findings never
      # change rc without --exit-code): crash, not a pass.
      record "$label" "$tool" "FAIL" "$v" "tool error/crash rc=$rc (not a clean scan); path=$path"
      return 1 ;;
    *)
      record "$label" "$tool" "FAIL" "$v" "unexpected exit rc=$rc = crash, never PASS; path=$path"
      return 1 ;;
  esac
}

log_header() { printf '%s\n' "$1" | tee -a "$OUT"; }

log_header "Security / static check evidence"
log_header "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log_header "Modules: backend, backend/internal/asrworker, backend/internal/ttsworker,"
log_header "         backend/internal/middleworker, backend/eval, loadmodel"
log_header ""

run_in_module() {
  local name="$1" dir="$2"
  log_header "=== $name ($dir) ==="
  pushd "$ROOT/$dir" > /dev/null || return 1

  if go vet ./... >> "$OUT" 2>&1; then
    record "$name" "go vet" "PASS" "$(go version | awk '{print $3}')" ""
  else
    record "$name" "go vet" "FAIL" "$(go version | awk '{print $3}')" "see stdout"
    FAILED=1
  fi

  unformatted="$(gofmt -l . 2>&1 || true)"
  if [ -n "$unformatted" ]; then
    record "$name" "gofmt" "FAIL" "$(go version | awk '{print $3}')" "files need formatting: $(echo "$unformatted" | tr '\n' ' ')"
    FAILED=1
  else
    record "$name" "gofmt" "PASS" "$(go version | awk '{print $3}')" ""
  fi

  # REQUIRED scanner set for the P7 gate: every NOT_RUN here makes
  # the overall result INCOMPLETE (exit 4), never a clean PASS.
  local rc
  rc=0; run_scanner "$name" govulncheck govulncheck ./... || rc=$?
  [ "$rc" -eq 1 ] && FAILED=1
  [ "$rc" -eq 2 ] && NOT_RUN_REQUIRED=1
  rc=0; run_scanner "$name" gosec gosec ./... || rc=$?
  [ "$rc" -eq 1 ] && FAILED=1
  [ "$rc" -eq 2 ] && NOT_RUN_REQUIRED=1
  rc=0; run_scanner "$name" staticcheck staticcheck ./... || rc=$?
  [ "$rc" -eq 1 ] && FAILED=1
  [ "$rc" -eq 2 ] && NOT_RUN_REQUIRED=1

  popd > /dev/null
  log_header ""
}

run_in_module "backend"          "backend"
run_in_module "asrworker"        "backend/internal/asrworker"
run_in_module "ttsworker"        "backend/internal/ttsworker"
run_in_module "middleworker"     "backend/internal/middleworker"
run_in_module "eval"             "backend/eval"
run_in_module "loadmodel"        "loadmodel"

rc=0; run_scanner "repo" gitleaks gitleaks detect --no-banner --redact --source . || rc=$?
[ "$rc" -eq 1 ] && FAILED=1
[ "$rc" -eq 2 ] && NOT_RUN_REQUIRED=1
# Trivy WITHOUT --exit-code always returns 0; run_scanner parses the
# findings table and treats rc!=0 as a crash.
rc=0; run_scanner "repo" trivy trivy fs --quiet --skip-db-update . || rc=$?
[ "$rc" -eq 1 ] && FAILED=1
[ "$rc" -eq 2 ] && NOT_RUN_REQUIRED=1

# ---- SPDX evidence validation (no invented dependencies/licenses) --
SPDX_FILES=(plan/evidence/sbom-*.spdx.json)
spdx_present=0
for f in "${SPDX_FILES[@]}"; do [ -f "$f" ] && spdx_present=1 && break; done
if [ "$spdx_present" -eq 1 ]; then
  if python3 - "${SPDX_FILES[@]}" << 'PYEOF'
import json, sys
bad = []
for path in sys.argv[1:]:
    try:
        d = json.load(open(path))
    except Exception as e:
        bad.append(f"{path}: unreadable JSON: {e}")
        continue
    for field in ("spdxVersion", "dataLicense", "SPDXID", "name", "documentNamespace"):
        if not d.get(field):
            bad.append(f"{path}: missing {field}")
    if not isinstance(d.get("packages"), list):
        bad.append(f"{path}: packages not a list")
    else:
        for i, p in enumerate(d["packages"]):
            for field in ("SPDXID", "name"):
                if not p.get(field):
                    bad.append(f"{path}: package[{i}] missing {field}")
            lic = p.get("licenseDeclared", "")
            # NOASSERTION is an honest statement; an EMPTY or
            # fabricated-appearing license is not.
            if lic is None or lic == "":
                bad.append(f"{path}: package[{i}] licenseDeclared empty (invent nothing)")
if bad:
    print("\n".join(bad))
    sys.exit(1)
print("spdx metadata structurally valid")
PYEOF
  then
    record "repo" "spdx-validate" "PASS" "python3" "validated ${#SPDX_FILES[@]} SBOM documents"
  else
    record "repo" "spdx-validate" "FAIL" "python3" "SPDX metadata invalid (see above)"
    FAILED=1
  fi
else
  record "repo" "spdx-validate" "NOT_RUN" "" "no sbom-*.spdx.json evidence present; run scripts/gen_spdx.py first"
  NOT_RUN_REQUIRED=1
fi

# ---- verdict -------------------------------------------------------
log_header ""
if [ "$FAILED" -ne 0 ]; then
  log_header "RESULT: FAIL (at least one executed check failed)"
  finalize_json "FAIL"
  exit 1
fi
if [ "$NOT_RUN_REQUIRED" -ne 0 ]; then
  # Honest aggregation: required scanners absent means the audit
  # is INCOMPLETE — never an overall clean PASS.
  log_header "RESULT: INCOMPLETE (required scanner checks were NOT_RUN; nothing failed)"
  finalize_json "INCOMPLETE"
  exit 4
fi
log_header "RESULT: PASS (every required check ran and passed)"
finalize_json "PASS"
exit 0
