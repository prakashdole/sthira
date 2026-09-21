#!/usr/bin/env bash
# scripts/go-build.sh — build the package's image and tag it so the rest of
# the lifecycle scripts reference it. Idempotent: re-runs do not delete
# anything, just refresh the tag.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
export STHIRA_DEPLOY_PROJECT="${PROJECT_NAME}"
export STHIRA_DEPLOY_IMAGE="${STHIRA_DEPLOY_IMAGE:-${PROJECT_NAME}-backend:local}"

"${PACKAGE_DIR}/bin/build-image.sh"
