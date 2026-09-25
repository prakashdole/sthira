#!/usr/bin/env bash
# scripts/go-backup.sh — explicit database backup using `pg_dump` from
# either the package's PostGIS image (when Docker is available) or the
# host-installed pg_dump (fallback). The output is a custom-format dump
# suitable for `pg_restore` later.
#
# Refusal rules:
#   - Backups are written OUTSIDE the repository by default; the parent
#     directory is ${STHIRA_BACKUP_DIR:-/var/backups/<project>} which the
#     operator must create with mode 700. The package `chmod 700`s the
#     directory on creation. Backup files are written with mode 600.
#   - Refuses to overwrite an existing file unless --force is passed.
#   - The DSN is delivered via the PGURI environment variable so the
#     password never appears in argv (visible via `ps`/process listings).

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

"${PACKAGE_DIR}/bin/env-preflight.sh"

PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
BACKUP_DIR="${STHIRA_BACKUP_DIR:-/var/backups/${PROJECT_NAME}}"
mkdir -p "${BACKUP_DIR}"
chmod 700 "${BACKUP_DIR}"

FORCE=0
for arg in "$@"; do
    case "${arg}" in
        --force) FORCE=1 ;;
        -h|--help)
            cat <<USAGE
Usage: $0 [--force]

Writes \${STHIRA_BACKUP_DIR:-/var/backups/${PROJECT_NAME}}/<timestamp>.dump
from the database described by STHIRA_DATABASE_DSN. Refuses to overwrite
an existing file unless --force is passed.
USAGE
            exit 0
            ;;
        *) echo "go-backup: unknown flag ${arg}" >&2; exit 2 ;;
    esac
done

ts="$(date -u +%Y%m%dT%H%M%SZ)"
target="${BACKUP_DIR}/${PROJECT_NAME}-${ts}.dump"

if [[ -e "${target}" ]] && (( ! FORCE )); then
    echo "go-backup: refusing to overwrite ${target} (use --force)" >&2
    exit 1
fi

run_pg_dump_via_docker() {
    local out="$1"
    local env_file
    env_file="$(mktemp -t sthira-go-pgenv.XXXXXX)"
    chmod 600 "${env_file}"
    "${PGDSN_ENV_BIN}" "${STHIRA_DATABASE_DSN}" > "${env_file}"
    # Use the same image the running stack uses, in --rm mode. Network
    # is bridged to the compose stack so the dump can reach postgres by
    # its service hostname. pg_dump inside the container reads PG* from
    # the env-file we just generated.
    docker run --rm --network "${PROJECT_NAME}_sthira-go-backend" -i \
        --env-file "${env_file}" \
        "${STHIRA_PG_IMAGE:-postgis/postgis:18-3.6}" \
        pg_dump --format=custom --no-owner --no-privileges --quote-all-identifiers \
        > "${out}.tmp"
    rm -f "${env_file}"
    mv "${out}.tmp" "${out}"
    chmod 600 "${out}"
}

run_pg_dump_host() {
    local out="$1"
    local pg_dump_bin="${2:-$(command -v pg_dump)}"
    local env_file
    env_file="$(mktemp -t sthira-go-pgenv.XXXXXX)"
    chmod 600 "${env_file}"
    "${PGDSN_ENV_BIN}" "${STHIRA_DATABASE_DSN}" > "${env_file}"
    # Inline source the env file so the current shell has PG* exported
    # before we exec pg_dump. pg_dump itself accepts no --env-file; the
    # only way to feed PG* without putting them in argv is to export them
    # in the parent.
    set -a
    # shellcheck disable=SC1090
    . "${env_file}"
    set +a
    rm -f "${env_file}"
    "${pg_dump_bin}" \
        --format=custom --no-owner --no-privileges --quote-all-identifiers \
        > "${out}.tmp"
    mv "${out}.tmp" "${out}"
    chmod 600 "${out}"
}

# Locate or build pgdsn-env, the URI->libpq-env helper.
if [[ -n "${STHIRA_PGDSN_ENV_BIN:-}" && -x "${STHIRA_PGDSN_ENV_BIN}" ]]; then
    PGDSN_ENV_BIN="${STHIRA_PGDSN_ENV_BIN}"
elif [[ -x "${PACKAGE_DIR}/migrate/pgdsn-env" ]]; then
    PGDSN_ENV_BIN="${PACKAGE_DIR}/migrate/pgdsn-env"
elif command -v go >/dev/null 2>&1; then
    PGDSN_ENV_BIN="${PACKAGE_DIR}/migrate/pgdsn-env"
    (cd "${PACKAGE_DIR}/migrate" && go build -o "${PGDSN_ENV_BIN}" ./cmd/pgdsn-env)
else
    echo "go-backup: pgdsn-env missing and Go not on PATH; cannot dump without it" >&2
    exit 1
fi

if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    run_pg_dump_via_docker "${target}"
else
    # Allow the operator to point at a specific pg_dump binary (useful
    # when the system pg_dump version does not match the server version
    # — pg_dump refuses on a major version mismatch).
    if [[ -n "${STHIRA_PG_DUMP:-}" && -x "${STHIRA_PG_DUMP}" ]]; then
        PG_DUMP_BIN="${STHIRA_PG_DUMP}"
    elif command -v pg_dump >/dev/null 2>&1; then
        PG_DUMP_BIN="$(command -v pg_dump)"
    else
        echo "go-backup: docker unavailable AND no pg_dump on host" >&2
        exit 1
    fi
    run_pg_dump_host "${target}" "${PG_DUMP_BIN}"
fi

echo "go-backup: wrote ${target} ($(wc -c < "${target}" | tr -d ' ') bytes)"
