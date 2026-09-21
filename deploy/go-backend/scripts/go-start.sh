#!/usr/bin/env bash
# scripts/go-start.sh — bring up the deployment package's stack.
#
# Refuses to start without a built image (run scripts/go-build.sh first).
# Does NOT run migrations automatically: call scripts/go-migrate.sh after
# start, then call scripts/go-status.sh to confirm /health/ready.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
export STHIRA_DEPLOY_PROJECT="${PROJECT_NAME}"
export STHIRA_DEPLOY_IMAGE="${STHIRA_DEPLOY_IMAGE:-${PROJECT_NAME}-backend:local}"

# Compose v2 with the project's `name:` directive. --project-name is
# still set explicitly so this script remains usable from directories
# other than the package root (e.g. CI runners).
COMPOSE=(docker compose --project-directory "${PACKAGE_DIR}" --project-name "${PROJECT_NAME}" -f "${PACKAGE_DIR}/docker-compose.yml")

if ! docker image inspect "${STHIRA_DEPLOY_IMAGE}" >/dev/null 2>&1; then
    echo "go-start: image ${STHIRA_DEPLOY_IMAGE} not found locally; running scripts/go-build.sh first" >&2
    "${SCRIPT_DIR}/go-build.sh"
fi

# Set explicit --scale to 1: the API binary is single-replica by design
# today; the compose file does not enable replicas either. Keeping it
# explicit prevents a stray `docker compose up --scale api=3` from racing
# migrations on bootstrap.
"${COMPOSE[@]}" up -d --scale api=1 --no-build --wait postgres api

echo "go-start: stack ${PROJECT_NAME} started. Next: ${SCRIPT_DIR}/go-migrate.sh"
