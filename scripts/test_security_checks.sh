#!/usr/bin/env bash
# test_security_checks.sh — deterministic self-test for the P7
# security harness (prompt item 6: "Test the harness without
# expensive installation"; exit-semantics and failure propagation
# must be proven, not assumed).
#
# It builds a THROWAWAY repo skeleton in a temp dir (tiny valid Go
# modules) so nothing here touches the real tree, then runs the
# real scripts/run_security_checks.sh against synthetic scanners
# placed on PATH with exactly the documented exit behaviours:
#
#   scenario A: no optional scanners            -> exit 4 INCOMPLETE
#   scenario B: trivy rc0 + "Total: 1" table    -> findings -> exit 1
#             gosec rc1                         -> findings -> exit 1
#             staticcheck rc2                   -> CRASH, never PASS
#             govulncheck rc0                   -> PASS
#             gitleaks rc1                      -> findings
#             spdx valid fixture present        -> PASS
#   scenario C: all clean (rc0 / Total: 0)      -> exit 0 PASS
#
# Exit: 0 iff every assertion holds.

set -euo pipefail

SRC_ROOT=$(cd "$(dirname "$0")/.." && pwd)
FAILS=0
note() { printf 'test_security: %s\n' "$*"; }
assert() { # assert <desc> <expected> <actual>
  if [ "$2" = "$3" ]; then
    note "PASS  $1"
  else
    note "FAIL  $1 (want $2, got $3)"
    FAILS=$((FAILS + 1))
  fi
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# ---- throwaway repo skeleton --------------------------------------
mk_module() {
  local dir="$1"
  mkdir -p "$WORK/$dir"
  cat > "$WORK/$dir/go.mod" <<EOF
module example.com/sthira/smoketest/$(basename "$dir")

go 1.23
EOF
  cat > "$WORK/$dir/main.go" <<'EOF'
package main

func main() {}
EOF
}
mkdir -p "$WORK/scripts" "$WORK/plan/evidence"
cp "$SRC_ROOT/scripts/run_security_checks.sh" "$WORK/scripts/"
mk_module backend
mk_module backend/internal/asrworker
mk_module backend/internal/ttsworker
mk_module backend/internal/middleworker
mk_module backend/eval
mk_module loadmodel
chmod +x "$WORK/scripts/run_security_checks.sh"

# Minimal structurally-valid SBOM fixture (no invented real deps:
# one synthetic local package, license NOASSERTION is honest).
cat > "$WORK/plan/evidence/sbom-smoke.spdx.json" <<'EOF'
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "smoke-harness-fixture",
  "documentNamespace": "https://example.invalid/sbom/smoke",
  "packages": [
    {
      "SPDXID": "SPDXRef-Package-fake",
      "name": "fake-package",
      "versionInfo": "0.0.0",
      "licenseDeclared": "NOASSERTION",
      "downloadLocation": "NOASSERTION"
    }
  ]
}
EOF

# ---- fake tool factory ---------------------------------------------
FAKEBIN="$WORK/fakebin"
mkdir -p "$FAKEBIN"
make_fake() { # make_fake <name> <rc> <stdout-text>
  local name="$1" rc="$2" text="$3"
  cat > "$FAKEBIN/$name" <<EOF
#!/usr/bin/env bash
if [ "\${1:-}" = "--version" ] || [ "\${1:-}" = "version" ]; then echo "$name fake 0.0-test"; exit 0; fi
echo "$text"
exit $rc
EOF
  chmod +x "$FAKEBIN/$name"
}

run_harness() { # run_harness <tag> -> sets RC + JSON paths
  local tag="$1"
  set +e
  PATH="$FAKEBIN:$PATH" \
  STHIRA_SECURITY_MD="$WORK/out_$tag.md" \
  STHIRA_SECURITY_JSON="$WORK/out_$tag.json" \
    bash "$WORK/scripts/run_security_checks.sh" --json > "$WORK/log_$tag.txt" 2>&1
  RC=$?
  set -e
}

assert_json() { # assert_json <file> <python-expression on records/summary> <desc>
  local file="$1" expr="$2" desc="$3" got
  got="$(python3 - "$file" <<PYEOF
import json, sys
d = json.load(open(sys.argv[1]))
records = d.get("records", [])
overall = d.get("overall", "")
ok = bool(${expr})
print("yes" if ok else "no")
PYEOF
)"
  assert "$desc" "yes" "$got"
}

# ---- scenario A: no scanners present -> INCOMPLETE, never PASS -----
rm -f "$FAKEBIN"/* 2>/dev/null || true
# Move the SBOM away so its NOT_RUN also lands in the aggregate.
mv "$WORK/plan/evidence/sbom-smoke.spdx.json" "$WORK/sbom-held.json"
run_harness A
assert "A: required-checks-missing exits INCOMPLETE(4)" "4" "$RC"
assert_json "$WORK/out_A.json" "overall == 'INCOMPLETE'" "A: overall INCOMPLETE not PASS"
assert_json "$WORK/out_A.json" "any(r['status']=='NOT_RUN' and r['tool']=='govulncheck' for r in records)" "A: govulncheck NOT_RUN recorded"
assert_json "$WORK/out_A.json" "any(r['status']=='NOT_RUN' and r['tool']=='trivy' for r in records)" "A: trivy NOT_RUN recorded"
assert_json "$WORK/out_A.json" "overall != 'PASS'" "A: never reported clean PASS"

# ---- scenario B: mixed semantics -> FAIL with per-kind details -----
mv "$WORK/sbom-held.json" "$WORK/plan/evidence/sbom-smoke.spdx.json"
make_fake govulncheck 0 "no vulnerabilities"
make_fake gosec 1 "gosec found issues"
make_fake staticcheck 2 "staticcheck: boom (usage crash)"
make_fake gitleaks 1 "leak found"
# trivy exits 0 ALWAYS; findings live in the table only.
make_fake trivy 0 "Scan Results
════════════════
Total: 1 (UNKNOWN: 0, LOW: 0, MEDIUM: 1, HIGH: 0, CRITICAL: 0)"
run_harness B
assert "B: findings exit FAIL(1)" "1" "$RC"
assert_json "$WORK/out_B.json" "overall == 'FAIL'" "B: overall FAIL"
assert_json "$WORK/out_B.json" "any(r['tool']=='trivy' and r['status']=='FAIL' and 'findings' in r['detail'] for r in records)" "B: trivy rc0-with-findings is FAIL (exit-behavior proof)"
assert_json "$WORK/out_B.json" "any(r['tool']=='gosec' and r['status']=='FAIL' and 'findings' in r['detail'] for r in records)" "B: gosec rc1 = findings FAIL"
assert_json "$WORK/out_B.json" "any(r['tool']=='staticcheck' and r['status']=='FAIL' and 'crash' in r['detail'].lower() for r in records)" "B: staticcheck rc2 = CRASH FAIL (never silent)"
assert_json "$WORK/out_B.json" "any(r['tool']=='govulncheck' and r['status']=='PASS' for r in records)" "B: clean govulncheck still PASS"
assert_json "$WORK/out_B.json" "any(r['tool']=='spdx-validate' and r['status']=='PASS' for r in records)" "B: valid SPDX fixture passes"

# ---- scenario C: everything clean -> PASS ---------------------------
make_fake gosec 0 "no issues"
make_fake staticcheck 0 ""
make_fake gitleaks 0 "no leaks"
make_fake trivy 0 "Scan Results
Total: 0 (UNKNOWN: 0, LOW: 0, MEDIUM: 0, HIGH: 0, CRITICAL: 0)"
run_harness C
assert "C: all clean exits 0" "0" "$RC"
assert_json "$WORK/out_C.json" "overall == 'PASS'" "C: overall PASS only when everything ran"

# ---- scenario D: SBOM fixture malformed -> FAIL (no invented pass) --
cat > "$WORK/plan/evidence/sbom-smoke.spdx.json" <<'EOF'
{"spdxVersion": "SPDX-2.3", "packages": [{"SPDXID": "SPDXRef-P", "name": "x"}]}
EOF
run_harness D
assert "D: invalid SPDX metadata fails the run" "1" "$RC"
assert_json "$WORK/out_D.json" "any(r['tool']=='spdx-validate' and r['status']=='FAIL' for r in records)" "D: spdx-validate FAIL recorded"

if [ "$FAILS" -ne 0 ]; then
  note "RESULT: FAILED ($FAILS assertions)"
  exit 1
fi
note "RESULT: all harness assertions pass"
