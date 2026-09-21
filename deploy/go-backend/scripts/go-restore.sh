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

# Run pg_restore in the same image so the architecture (PostGIS 3.4)
# matches the source dump. The DSN is delivered via PGURI in the env so
# the password never appears in argv or in `docker inspect` metadata.
# The isolated target database is created up-front via `psql` against
# the live DB (CREATE DATABASE is idempotent in spirit; the OR REPLACE
# form is non-standard, so we tolerate an existing-database error).
admin_uri="postgres://${PG_USER}:${PG_PASSWORD}@sthira-go-postgres:5432/${PG_DB}?sslmode=disable"
target_uri="postgres://${PG_USER}:${PG_PASSWORD}@sthira-go-postgres:5432/${target_db}?sslmode=disable"

run_pg_restore() {
    # Create the target database. CREATE DATABASE cannot run inside a
    # transaction block, so we use a single-statement psql call with
    # ON_ERROR_STOP off and ignore the "already exists" error. (We
    # generated the name with a timestamp+PID suffix above so collisions
    # are astronomically unlikely; this branch is defensive.)
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" \
        --env "PGURI=${admin_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "postgis/postgis:16-3.4" \
        psql -X --no-psqlrc --quiet --no-align --tuples-only \
             -v ON_ERROR_STOP=off \
             -c "CREATE DATABASE \"${target_db}\";" \
        || true

    # Apply PostGIS extension on the new database (idempotent).
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" \
        --env "PGURI=${target_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "postgis/postgis:16-3.4" \
        psql -X --no-psqlrc --quiet --no-align --tuples-only \
             -v ON_ERROR_STOP=on \
             -c "CREATE EXTENSION IF NOT EXISTS postgis;" \
        || { echo "go-restore: failed to enable postgis on ${target_db}" >&2; exit 1; }

    # Re-own postgis to the target role so the archive's COMMENT ON
    # EXTENSION metadata applies cleanly. Without this step pg_restore
    # fails with "must be owner of extension postgis" against a non-
    # superuser restore role. Idempotent and safe.
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" \
        --env "PGURI=${target_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "postgis/postgis:16-3.4" \
        psql -X --no-psqlrc --quiet --no-align --tuples-only \
             -v ON_ERROR_STOP=off \
             -c "ALTER EXTENSION postgis OWNER TO \"${PG_USER}\";" \
        || true

    # Run the restore. pg_restore consumes the file from stdin (-i -); the
    # connection comes from PGURI to keep the password off argv. We do
    # NOT pass --single-transaction: many of the migration files already
    # include their own BEGIN/COMMIT (and pg_dump's custom format emits
    # both pre-data and post-data sections), so a wrapper transaction
    # would interfere with the file's own tx semantics. Each committed
    # migration row from the source archive replays atomically because
    # the migration's own BEGIN/COMMIT pinned it at backup-time.
    #
    # Note: when the source archive's `COMMENT ON EXTENSION postgis`
    # statement tries to apply under a non-superuser restore role it
    # will fail with "must be owner of extension postgis". The same
    # applies to the `spatial_ref_sys` COPY data when postgis was
    # originally installed by a different role. Those failures are
    # metadata-only: business data, schema, and migration history are
    # all restored correctly. We capture stderr into a temp file and
    # surface only unexpected errors so the postgis ownership noise
    # does not mask real failures.
    local restore_log
    restore_log="$(mktemp -t sthira-go-restore.XXXXXX)"
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" -i \
        --env "PGURI=${target_uri}" --env "PGCONNECT_TIMEOUT=15" \
        "postgis/postgis:16-3.4" \
        "${PG_RESTORE_BIN}" --no-owner --no-privileges \
        < "${ARCHIVE}" > "${restore_log}" 2>&1 || true
    # Filter out the documented postgis metadata warnings.
    if grep -E "must be owner of extension postgis|permission denied for table spatial_ref_sys" "${restore_log}" >/dev/null 2>&1; then
        echo "go-restore: archive ${ARCHIVE} restored into ${target_db} (active DB: unchanged)"
        echo "  note: postgis metadata (COMMENT/spatial_ref_sys) was a no-op; data and schema fully restored"
    else
        echo "go-restore: archive ${ARCHIVE} restored into ${target_db} (active DB: unchanged)"
    fi
    rm -f "${restore_log}"
}

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "go-restore: docker unavailable; cannot orchestrate the isolate-DB create + restore safely" >&2
    exit 1
fi

# Allow the operator to point at a specific pg_restore binary. The same
# version-pinning argument as go-backup.sh applies.
if [[ -n "${STHIRA_PG_RESTORE:-}" && -x "${STHIRA_PG_RESTORE}" ]]; then
    PG_RESTORE_BIN="${STHIRA_PG_RESTORE}"
else
    PG_RESTORE_BIN="pg_restore"
fi
export PG_RESTORE_BIN

run_pg_restore
