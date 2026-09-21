#!/usr/bin/env bash
# scripts/go-status.sh — print container state and reachability for the
# API and the Postgres container. Probes /health/live and /health/ready on
# the loopback API port. The Postgres container is left alone (no public
# port is published by this package), so its state is reported via
# `docker compose ps` only.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
HOST_PORT="${STHIRA_API_HOST_PORT:-8080}"

echo "=== container state ==="
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    COMPOSE=(docker compose --project-directory "${PACKAGE_DIR}" --project-name "${PROJECT_NAME}" -f "${PACKAGE_DIR}/docker-compose.yml")
    "${COMPOSE[@]}" ps || true
else
    echo "docker not available; container states unknown"
fi

echo
echo "=== loopback API probes ==="
probe() {
    local path="$1"
    local body
    body="$(curl -fsS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${HOST_PORT}${path}" 2>/dev/null || echo "ERR")"
    echo "GET ${path}  -> ${body}"
}
probe /health/live
probe /health/ready

echo
echo "=== migration state (via sthmigrate status, host-run if no docker) ==="
"${SCRIPT_DIR}/go-migrate.sh" status || true
