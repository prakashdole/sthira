#!/usr/bin/env bash
# Strix execution-stub — purpose-built NO-OP for P7-prep.
#
# Strix requires a model/spend/target decision (O15). The current
# repo state (per plan/open-decisions.md O15) is OPEN. Therefore
# this script DOES NOT execute against any system.
#
# What this script DOES do:
#   - validates that the operator understood the stop conditions;
#   - writes a deterministic record to reports/strix-configured.json
#     so reviewers can reproduce or revoke the configuration without
#     invoking paid inference.
#
# To run a real Strix session, the operator must:
#   1. resolve O15 (model pin, spend cap, scoping);
#   2. update strix_run.env with the chosen values;
#   3. set RUN_STRIX_FOR_REAL=1 only on a disposable staging host.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/_lib.sh"
snapshot_metadata

: "${STRIX_REPO:?missing STRIX_REPO}"
: "${STRIX_VERSION:?missing STRIX_VERSION}"

OUT="${REPORTS_DIR}"
mkdir -p "$OUT"

# The "configuration" is just a documented record; the runner does
# not connect to anything.
if [ "${RUN_STRIX_FOR_REAL:-}" != "1" ]; then
    cat > "${OUT}/strix-configured.json" <<EOF
{
  "executed": false,
  "reason": "O15 open; Strix execution stays blocked until model/spend/target decision",
  "snapshot_date": "${SNAPSHOT_DATE}",
  "repo": "${STRIX_REPO}",
  "version": "${STRIX_VERSION}",
  "host": "${SNAPSHOT_HOST}",
  "reviewers_required": ["security", "product"]
}
EOF
    echo "Strix configuration recorded at ${OUT}/strix-configured.json"
    echo "Real execution requires RUN_STRIX_FOR_REAL=1 and a resolved O15"
    exit 0
fi

# Past this point the operator has resolved O15 and explicitly opted
# in. The runner still halts if the configured target host does not
# match the explicit allowlist.
: "${STRIX_TARGET_URL:?set to a disposable, allowlisted staging URL}"
echo "FAIL: live Strix execution not yet wired in the P7-prep directory" >&2
exit 1
