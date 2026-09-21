#!/usr/bin/env bash
# Native Go fuzz runner — wrapper around `go test -fuzz`.
#
# Each fuzz target listed in FUZZ_TARGETS runs for FUZZ_TIME (per
# target). The targets are limited to the *input parsing* and
# *boundary enforcement* packages — anything else (DB, fmt) is not
# relevant to the P7 prep surface and grows the corpus needlessly.
#
# The fuzz harnesses are the ones already shipped by the package
# owners (capfeed, httpjson, offlinepkg). This runner just exercises
# them with bounded time so CI costs stay predictable.

set -euo pipefail
cd "$(dirname "$0")/../.."

source "$(dirname "$0")/_lib.sh"
snapshot_metadata

FUZZ_TIME="${FUZZ_TIME:-30s}"
FUZZ_TARGETS_DEFAULT=(
    "internal/capfeed"
    "internal/httpjson"
    "internal/offlinepkg"
)

# Allow callers to override the target list with FUZZ_TARGETS env var
# (space-separated package paths). Defaults iterate over the three
# known fuzz-bearing packages.
if [ -n "${FUZZ_TARGETS:-}" ]; then
    IFS=' ' read -r -a TARGETS <<< "$FUZZ_TARGETS"
else
    TARGETS=("${FUZZ_TARGETS_DEFAULT[@]}")
fi

for pkg in "${TARGETS[@]}"; do
    echo "==> go test -fuzz -run=^$ -fuzztime=${FUZZ_TIME} ./${pkg}/..."
    if ! go test -run='^$' -fuzz=. -fuzztime="${FUZZ_TIME}" "./${pkg}/..."; then
        echo "FAIL: fuzz found a crash in ${pkg}" >&2
        exit 1
    fi
done

echo "OK: fuzz clean (time=${FUZZ_TIME} per target)"
