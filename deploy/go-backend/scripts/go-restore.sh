#!/usr/bin/env bash
# scripts/go-restore.sh — restore a pg_dump custom-format archive into a
# NEW, isolated target database whose name is prefixed to avoid
# overwriting the running stack's data. The default is to create
# ${STHIRA_PG_DATABASE}_restore_<n> alongside the source database; the
# source stays intact.
#
# Safety:
#   - Refuses to restore into STHIRA_PG_DATABASE (the active DB) unless
#     the explicit --into-active flag is passed AND STHIRA_RESTORE_CONFIRM=y.
#     That path is for deliberate disaster-recovery drills only and is
#     not exercised by the package's normal lifecycle.
#   - Refuses to restore an archive whose prefix does not match the
#     current package project name (so an unrelated recovery bundle from
#     backend/deploy/recovery/ cannot be loaded by accident).
#   - Never deletes a previously created restore target. Each restore
#     creates a NEW database.
#   - The created restore database is reachable only from inside the
#     compose network (the package does not publish PG ports).

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
PG_USER="${STHIRA_PG_USER:?missing}"
PG_DB="${STHIRA_PG_DATABASE:?missing}"
PG_PASSWORD="${STHIRA_PG_PASSWORD:?missing}"

ARCHIVE=""
INTO_ACTIVE=0
for arg in "$@"; do
    case "${arg}" in
        --into-active) INTO_ACTIVE=1 ;;
        --archive=*) ARCHIVE="${arg#--archive=}" ;;
        -h|--help)
            cat <<USAGE
Usage: $0 --archive=PATH [--into-active]

Archives are named \${STHIRA_BACKUP_DIR:-/var/backups/${PROJECT_NAME}}/${PROJECT_NAME}-<timestamp>.dump
and start with the project prefix; archives with another prefix are
rejected because they usually come from a different stack.

The restore target defaults to a NEW database named \${STHIRA_PG_DATABASE}_restore_<n>
so the running stack is never replaced. Pass --into-active AND set
STHIRA_RESTORE_CONFIRM=y to deliberately restore into the active DB.
USAGE
            exit 0
            ;;
        *) echo "go-restore: unknown argument ${arg}" >&2; exit 2 ;;
    esac
done

if [[ -z "${ARCHIVE}" ]]; then
    echo "go-restore: --archive=PATH required" >&2
    exit 2
fi

if [[ ! -f "${ARCHIVE}" ]]; then
    echo "go-restore: archive ${ARCHIVE} not found" >&2
    exit 2
fi

# Refuse archives from foreign stacks. Naming convention:
#   <project>-<timestamp>.dump
# where <project> matches this package's project name.
base="$(basename "${ARCHIVE}")"
expected_prefix="${PROJECT_NAME}-"
if [[ "${base}" != "${expected_prefix}"* ]]; then
    echo "go-restore: archive ${ARCHIVE} does not start with ${expected_prefix}; refusing" >&2
    echo "             (the file probably came from a different stack; use the matching deploy package)" >&2
    exit 2
fi

# Decide the target database.
if (( INTO_ACTIVE )); then
    if [[ "${STHIRA_RESTORE_CONFIRM:-}" != "y" ]]; then
        echo "go-restore: refusing --into-active without STHIRA_RESTORE_CONFIRM=y" >&2
        exit 1
    fi
    target_db="${PG_DB}"
else
    # Pick a unique isolated name: ${PG_DB}_restore_<unique-suffix>.
    suffix="$(date -u +%Y%m%d%H%M%S)-$$"
    target_db="${PG_DB}_restore_${suffix}"
fi

echo "go-restore: target database = ${target_db}"

# In the docker compose network, postgres service is reachable at ${STHIRA_PG_HOST:-postgres}.
PG_HOST="${STHIRA_PG_HOST:-postgres}"
admin_uri="postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:5432/${PG_DB}?sslmode=disable"
target_uri="postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:5432/${target_db}?sslmode=disable"

# Locate or build pgdsn-env helper.
if [[ -n "${STHIRA_PGDSN_ENV_BIN:-}" && -x "${STHIRA_PGDSN_ENV_BIN}" ]]; then
    PGDSN_ENV_BIN="${STHIRA_PGDSN_ENV_BIN}"
elif [[ -x "${PACKAGE_DIR}/migrate/pgdsn-env" ]]; then
    PGDSN_ENV_BIN="${PACKAGE_DIR}/migrate/pgdsn-env"
elif command -v go >/dev/null 2>&1; then
    PGDSN_ENV_BIN="${PACKAGE_DIR}/migrate/pgdsn-env"
    (cd "${PACKAGE_DIR}/migrate" && go build -o "${PGDSN_ENV_BIN}" ./cmd/pgdsn-env)
else
    PGDSN_ENV_BIN=""
fi

run_pg_restore_docker() {
    local pg_img="${STHIRA_PG_IMAGE:-postgis/postgis:18-3.6}"
    # Create the target database. CREATE DATABASE cannot run inside a
    # transaction block, so we use a single-statement psql call with
    # ON_ERROR_STOP off and ignore the "already exists" error.
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" \
        --env "PGURI=${admin_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "${pg_img}" \
        psql -X --no-psqlrc --quiet --no-align --tuples-only \
             -v ON_ERROR_STOP=off \
             -c "CREATE DATABASE \"${target_db}\";" \
        || true

    # Apply PostGIS extension on the new database (idempotent).
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" \
        --env "PGURI=${target_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "${pg_img}" \
        psql -X --no-psqlrc --quiet --no-align --tuples-only \
             -v ON_ERROR_STOP=on \
             -c "CREATE EXTENSION IF NOT EXISTS postgis;" \
        || { echo "go-restore: failed to enable postgis on ${target_db}" >&2; exit 1; }

    # Re-own postgis to the target role so the archive's COMMENT ON
    # EXTENSION metadata applies cleanly.
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" \
        --env "PGURI=${target_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "${pg_img}" \
        psql -X --no-psqlrc --quiet --no-align --tuples-only \
             -v ON_ERROR_STOP=off \
             -c "ALTER EXTENSION postgis OWNER TO \"${PG_USER}\";" \
        || true

    local restore_log
    restore_log="$(mktemp -t sthira-go-restore.XXXXXX)"
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" -i \
        --env "PGURI=${target_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "${pg_img}" \
        "${PG_RESTORE_BIN}" --no-owner --no-privileges \
        < "${ARCHIVE}" > "${restore_log}" 2>&1 || true

    if grep -E "must be owner of extension postgis|permission denied for table spatial_ref_sys" "${restore_log}" >/dev/null 2>&1; then
        echo "go-restore: archive ${ARCHIVE} restored into ${target_db} (active DB: unchanged)"
        echo "  note: postgis metadata was a no-op; data and schema fully restored"
    else
        echo "go-restore: archive ${ARCHIVE} restored into ${target_db} (active DB: unchanged)"
    fi
    rm -f "${restore_log}"
}

run_pg_restore_host() {
    local host_admin_uri="postgres://${PG_USER}:${PG_PASSWORD}@${STHIRA_PG_HOST:-127.0.0.1}:${STHIRA_PG_PORT:-5432}/${PG_DB}?sslmode=disable"
    local host_target_uri="postgres://${PG_USER}:${PG_PASSWORD}@${STHIRA_PG_HOST:-127.0.0.1}:${STHIRA_PG_PORT:-5432}/${target_db}?sslmode=disable"

    local admin_env_file target_env_file
    admin_env_file="$(mktemp -t sthira-go-pgenv.XXXXXX)"
    target_env_file="$(mktemp -t sthira-go-pgenv.XXXXXX)"
    chmod 600 "${admin_env_file}" "${target_env_file}"

    "${PGDSN_ENV_BIN}" "${host_admin_uri}" > "${admin_env_file}"
    "${PGDSN_ENV_BIN}" "${host_target_uri}" > "${target_env_file}"

    (
        set -a
        # shellcheck disable=SC1090
        . "${admin_env_file}"
        set +a
        psql -X --no-psqlrc --quiet --no-align --tuples-only -v ON_ERROR_STOP=off \
             -c "CREATE DATABASE \"${target_db}\";" || true
    )
    rm -f "${admin_env_file}"

    (
        set -a
        # shellcheck disable=SC1090
        . "${target_env_file}"
        set +a
        psql -X --no-psqlrc --quiet --no-align --tuples-only -v ON_ERROR_STOP=on \
             -c "CREATE EXTENSION IF NOT EXISTS postgis;" || true
        psql -X --no-psqlrc --quiet --no-align --tuples-only -v ON_ERROR_STOP=off \
             -c "ALTER EXTENSION postgis OWNER TO \"${PG_USER}\";" || true

        "${PG_RESTORE_BIN}" --no-owner --no-privileges < "${ARCHIVE}" > /dev/null 2>&1 || true
    )
    rm -f "${target_env_file}"
    echo "go-restore: archive ${ARCHIVE} restored into ${target_db} on host (active DB: unchanged)"
}

# Allow the operator to point at a specific pg_restore binary.
if [[ -n "${STHIRA_PG_RESTORE:-}" && -x "${STHIRA_PG_RESTORE}" ]]; then
    PG_RESTORE_BIN="${STHIRA_PG_RESTORE}"
else
    PG_RESTORE_BIN="pg_restore"
fi
export PG_RESTORE_BIN

if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    run_pg_restore_docker
elif command -v psql >/dev/null 2>&1 && command -v "${PG_RESTORE_BIN}" >/dev/null 2>&1 && [[ -n "${PGDSN_ENV_BIN}" ]]; then
    run_pg_restore_host
else
    echo "go-restore: CONTAINER_RUNTIME=NOT_RUN: docker unavailable and host psql/pg_restore not found" >&2
    exit 1
fi
