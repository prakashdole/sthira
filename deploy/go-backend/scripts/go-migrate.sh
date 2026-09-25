#!/usr/bin/env bash
# scripts/go-migrate.sh — explicit one-shot migration runner for the Go
# backend deployment package.
#
# Strategy:
#   1. Decide whether to run sthmigrate in the package's dedicated migration
#      image (`docker compose run --rm migrate up`) or directly on the
#      host (when Docker is unavailable, as a CI/operator workflow).
#      Either way, the migration is a single, atomic, advisory-lock-
#      protected pass.
#   2. Never auto-migrate on API startup. The api service has no `command:`
#      override that runs migrations, and Compose does NOT call this
#      script implicitly.
#
# Usage:
#   go-migrate.sh up        # apply all pending migrations
#   go-migrate.sh up 9      # apply up to and including revision 9
#   go-migrate.sh status    # print applied revisions
#   go-migrate.sh verify    # parse pending files without committing

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${PACKAGE_DIR}/../.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
export STHIRA_DEPLOY_PROJECT="${PROJECT_NAME}"
export STHIRA_DEPLOY_IMAGE="${STHIRA_DEPLOY_IMAGE:-${PROJECT_NAME}-backend:local}"
export STHIRA_DEPLOY_MIGRATE_IMAGE="${STHIRA_DEPLOY_MIGRATE_IMAGE:-${PROJECT_NAME}-backend:local-migrate}"

SUBCMD="${1:-up}"
shift || true

run_via_docker() {
    COMPOSE=(docker compose --project-directory "${PACKAGE_DIR}" --project-name "${PROJECT_NAME}" -f "${PACKAGE_DIR}/docker-compose.yml")
    # Use `run --rm` so the container is removed after the migrate. `--no-deps`
    # avoids racing the api service's startup; `--no-deps` on `run` is the
    # default for `compose run` in Compose v2.
    "${COMPOSE[@]}" run --rm --no-deps \
        -e "STHIRA_DATABASE_DSN=${STHIRA_DATABASE_DSN}" \
        migrate "${SUBCMD}" "$@"
}

run_on_host() {
    # Used when Docker is unavailable. Requires `psql` on PATH and Go
    # for the binary build (or a pre-built sthmigrate at STHIRA_MIGRATE_BIN).
    local bin="${STHIRA_MIGRATE_BIN:-}"
    if [[ -z "${bin}" || ! -x "${bin}" ]]; then
        if ! command -v psql >/dev/null 2>&1; then
            echo "go-migrate: docker unavailable AND host has no psql; cannot migrate" >&2
            exit 1
        fi
        if ! command -v go >/dev/null 2>&1; then
            echo "go-migrate: docker unavailable, host lacks psql AND go; cannot migrate" >&2
            exit 1
        fi
        bin="$(mktemp -t sthmigrate.XXXXXX)"
        trap 'rm -f "${bin}"' EXIT
        (cd "${PACKAGE_DIR}/migrate" && go build -o "${bin}" ./cmd/sthmigrate)
    fi
    local dir_arg=()
    if [[ ! " $* " =~ " -migrations " && ! " $* " =~ " --migrations " ]]; then
        dir_arg=(--migrations "${REPO_ROOT}/backend/migrations")
    fi
    STHIRA_DATABASE_DSN="${STHIRA_DATABASE_DSN}" "${bin}" "${SUBCMD}" "${dir_arg[@]}" "$@"
}

# Prefer Docker when available and the migration image exists locally.
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    if docker image inspect "${STHIRA_DEPLOY_MIGRATE_IMAGE}" >/dev/null 2>&1; then
        run_via_docker "$@"
        exit
    fi
    echo "go-migrate: image ${STHIRA_DEPLOY_MIGRATE_IMAGE} not built; please run scripts/go-build.sh" >&2
    exit 1
fi

echo "go-migrate: docker not available; falling back to host-run sthmigrate" >&2
run_on_host "$@"
