#!/usr/bin/env bash
# SBOM generator for the P7 prep window.
#
# This backend currently ships as a Go module; **no Docker image is
# built yet** (the binary at cmd/sthira/main.go has a Dockerfile
# follow-up under P8/P9, not P7). Therefore the SBOM evidence at
# P7-prep is a Go-modules SPDX/CycloneDX manifest derived from
# `go mod graph` and any installed SBOM-tool output for the source tree.
#
# The runner is idempotent: run once, get a stable hash; run twice,
# get the same content. Snapshots live in reports/sbom/.

set -euo pipefail
cd "$(dirname "$0")/../.."  # repo root

source "$(dirname "$0")/_lib.sh"
snapshot_metadata

OUT="${REPORTS_DIR}/sbom"
mkdir -p "$OUT"

echo "==> go mod graph (Go modules dependency lock)"
go mod graph 2>&1 | sort > "${OUT}/go-mod-graph.txt"
sha256sum "${OUT}/go-mod-graph.txt" \
  | tee "${OUT}/go-mod-graph.txt.sha256" >/dev/null

echo "==> syft SBOM (CycloneDX JSON), if installed"
if command -v syft >/dev/null 2>&1; then
    resolve_tool syft syft version >/dev/null
    syft scan dir:. \
        --output cyclonedx-json="${OUT}/syft.cdx.json" \
                 spdx-json="${OUT}/syft.spdx.json" \
        --quiet \
        || { echo "FAIL: syft exited non-zero" >&2; exit 1; }
    sha256sum "${OUT}"/*.json \
      | tee "${OUT}/syft.sha256" >/dev/null
else
    echo "skip syft: not installed (pin ${SYFT_VERSION}); the Go module
lockfile above is the prep-stage source of truth"
fi

echo "==> trivy SBOM on the source tree"
if command -v trivy >/dev/null 2>&1; then
    resolve_tool trivy trivy --version >/dev/null
    trivy fs \
        --format cyclonedx \
        --output "${OUT}/trivy.cdx.json" \
        --quiet . \
        || { echo "FAIL: trivy SBOM failed" >&2; exit 1; }
    sha256sum "${OUT}/trivy.cdx.json" \
      | tee "${OUT}/trivy.sha256" >/dev/null
else
    echo "skip trivy: not installed (pin ${TRIVY_VERSION})"
fi

echo "==> summary"
ls -la "${OUT}" | tee "${OUT}/sbom-summary.txt"
