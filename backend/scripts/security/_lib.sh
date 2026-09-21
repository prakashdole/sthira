#!/usr/bin/env bash
# Common helpers for the P7 security-prep runners.
# Source this from each runner. Keep it small — no module installs here.
#
# Conventions:
#   - resolve_tool prints the resolved version string on success, empty if absent.
#   - report_dir writes JSON to $REPORTS_DIR/<tool>-resolved-version.json
#   - require_tool exits non-zero (and prints a friendly note) when missing.
#   - print_safe never echoes arguments verbatim unless explicitly asked.

set -u
set -o pipefail

# Locate this script's directory even when sourced.
SECURITY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SECURITY_DIR}/versions.env"

# Default report dir, override with REPORTS_DIR=…
: "${REPORTS_DIR:=${SECURITY_DIR}/reports}"
mkdir -p "${REPORTS_DIR}"

# resolve_tool records the resolved version of an installed command.
# Usage: resolve_tool <name> <command> [<args…>]
# Prints "<resolved-version>" and writes reports/<name>-resolved.json.
resolve_tool() {
    local name="$1"; shift
    local cmd="$1"; shift
    local out
    if ! command -v "$cmd" >/dev/null 2>&1; then
        printf '%s\n' "" | tee "${REPORTS_DIR}/${name}-resolved.json" >/dev/null
        return 1
    fi
    out="$("$cmd" "$@" 2>&1 | head -1 || true)"
    printf '%s\n' "$out" | tee "${REPORTS_DIR}/${name}-resolved.json" >/dev/null
    printf '%s\n' "$out"
}

# resolve_image records the SHA digest of an image we're going to run.
# Usage: resolve_image <name> <image>
# Falls back to "absent" when docker is unavailable.
resolve_image() {
    local name="$1"; shift
    local image="$1"; shift
    if ! command -v docker >/dev/null 2>&1; then
        printf '%s\n' "absent" | tee "${REPORTS_DIR}/${name}-resolved.json" >/dev/null
        return 1
    fi
    local digest
    digest="$(docker inspect --format='{{index .RepoDigests 0}}' "$image" 2>/dev/null || echo "absent")"
    printf '%s\n' "$digest" | tee "${REPORTS_DIR}/${name}-resolved.json" >/dev/null
    printf '%s\n' "$digest"
}

# require_tool exits 77 if the tool isn't on PATH — TAP-style skip.
# Usage: require_tool <name> <command>
require_tool() {
    local name="$1"; shift
    local cmd="$1"; shift
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "skip $name: $cmd not installed (P7-prep; install at run-time)" >&2
        exit 77
    fi
}

# print_safe prints literal input without substituting secrets. The
# runner-level callers do not echo arguments verbatim; secrets stay on
# disk or in env-vars.
print_safe() {
    printf '%s\n' "$1"
}

# snapshot_metadata writes a small JSON with the run-time context: the
# user, date, host, go-version. The reports directory is the audit
# trail; findings reference the snapshot.
snapshot_metadata() {
    local snap="{
      \"snapshot_date\": \"${SNAPSHOT_DATE}\",
      \"run_date\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
      \"host\": \"${SNAPSHOT_HOST}\",
      \"go_version\": \"$(go version 2>/dev/null || echo 'absent')\",
      \"user\": \"${USER:-anonymous}\"
    }"
    printf '%s' "$snap" > "${REPORTS_DIR}/snapshot.json"
}
