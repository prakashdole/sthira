#!/usr/bin/env bash
# scripts/go-logs.sh — tail logs for the package's containers. Defaults to
# both api and postgres; pass a service name (or several) to filter.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "go-logs: docker not available" >&2
    exit 1
fi

COMPOSE=(docker compose --project-directory "${PACKAGE_DIR}" --project-name "${PROJECT_NAME}" -f "${PACKAGE_DIR}/docker-compose.yml")

if [[ "$#" -eq 0 ]]; then
    SERVICES=(api postgres)
else
    SERVICES=("$@")
fi

"${COMPOSE[@]}" logs -f "${SERVICES[@]}"
