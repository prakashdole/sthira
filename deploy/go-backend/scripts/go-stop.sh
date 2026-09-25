#!/usr/bin/env bash
# scripts/go-stop.sh — stop the package's API and Postgres containers
# without removing the persistent volume. Rebuilds/upgrades preserve data.
# Pass `--purge` to also remove the containers and the volume
# (destructive; refuses unless explicitly opted in).

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "go-stop: docker not available; nothing to stop" >&2
    exit 0
fi

COMPOSE=(docker compose --project-directory "${PACKAGE_DIR}" --project-name "${PROJECT_NAME}" -f "${PACKAGE_DIR}/docker-compose.yml")

PURGE=0
for arg in "$@"; do
    case "${arg}" in
        --purge) PURGE=1 ;;
        -h|--help)
            cat <<USAGE
Usage: $0 [--purge]

Without flags: stop the API and Postgres containers. The pgdata volume
for project "${PROJECT_NAME}" is left intact so a subsequent
\`scripts/go-start.sh\` rebuild can read the existing schema.

--purge:    additionally remove the stopped containers and the project
            volume. DESTRUCTIVE. Refuses if STHIRA_PURGE_CONFIRM=y is
            unset.
USAGE
            exit 0
            ;;
        *) echo "go-stop: unknown flag ${arg}" >&2; exit 2 ;;
    esac
done

if (( PURGE )); then
    if [[ "${STHIRA_PURGE_CONFIRM:-}" != "y" ]]; then
        echo "go-stop: refusing --purge without STHIRA_PURGE_CONFIRM=y (would drop pgdata for ${PROJECT_NAME})" >&2
        exit 1
    fi
    "${COMPOSE[@]}" down -v --remove-orphans
    echo "go-stop: containers and pgdata volume removed for project ${PROJECT_NAME}"
else
    "${COMPOSE[@]}" stop api postgres
    echo "go-stop: containers stopped; pgdata volume preserved for project ${PROJECT_NAME}"
fi
