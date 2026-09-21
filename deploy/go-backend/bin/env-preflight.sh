#!/usr/bin/env bash
# bin/env-preflight.sh — required-configuration guard for the lifecycle
# scripts. Refuses to start a stack with a default or empty DSN / password /
# API secret. Idempotent: exits 0 if config looks usable, 1 otherwise.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

err() { echo "preflight: $*" >&2; }

# Project name defaults are fixed but overridable via STHIRA_DEPLOY_PROJECT.
PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
export STHIRA_DEPLOY_PROJECT="${PROJECT_NAME}"

# Load .env if present (compose also reads it, but lifecycle scripts run
# outside compose so they need it locally too). File permissions are
# required to be 0o600 because it carries the DB password; the preflight
# refuses to load a world-readable .env so a stray `chmod 644 .env` cannot
# silently move the secret into broader reach.
ENV_FILE="${STHIRA_DEPLOY_ENV_FILE:-${PACKAGE_DIR}/.env}"
if [[ -f "${ENV_FILE}" ]]; then
    perms="$(stat -f '%Lp' "${ENV_FILE}" 2>/dev/null || stat -c '%a' "${ENV_FILE}" 2>/dev/null || echo unknown)"
    case "${perms}" in
        600|400|"" ) ;;
        *) err ".env at ${ENV_FILE} has permissions 0o${perms}; refusing to load. Run: chmod 600 ${ENV_FILE}"; exit 1 ;;
    esac
    set -a
    # shellcheck disable=SC1090
    . "${ENV_FILE}"
    set +a
fi

required=(
    "STHIRA_PG_USER:Postgres role name (used by the postgres container; user is also embedded into the DSN)"
    "STHIRA_PG_DATABASE:Postgres database name"
    "STHIRA_PG_PASSWORD:Postgres role password (never committed; read from ignored local .env)"
    "STHIRA_DATABASE_DSN:PostgreSQL DSN consumed by both the API (STHIRA_DATABASE_DSN env) and sthmigrate"
)
missing=0
for kv in "${required[@]}"; do
    name="${kv%%:*}"
    desc="${kv#*:}"
    value="${!name:-}"
    if [[ -z "${value}" ]]; then
        err "missing ${name} — ${desc}"
        missing=1
    fi
done

# DSN sanity: must look like a URL the backend's pgx driver can parse.
# The simplest check is that it starts with `postgres://` or `postgresql://`.
dsn="${STHIRA_DATABASE_DSN:-}"
case "${dsn}" in
    postgres://*|postgresql://*) ;;
    *) err "STHIRA_DATABASE_DSN must start with postgres:// or postgresql:// (got: ${dsn%%@*}<redacted>)"; missing=1 ;;
esac

# Password must not be the obvious default and must not appear on the CLI
# in the script output. Compose env expansion is fine; here we only refuse
# the literal value "postgres" which is the DevContainer default and the
# most common mistake.
if [[ "${STHIRA_PG_PASSWORD:-}" == "postgres" || "${STHIRA_PG_PASSWORD:-}" == "password" || "${STHIRA_PG_PASSWORD:-}" == "changeme" ]]; then
    err "STHIRA_PG_PASSWORD must be changed from the bundled default"
    missing=1
fi
if (( missing )); then
    err "see ${PACKAGE_DIR}/.env.example for the schema; the env values themselves stay outside the repo"
    exit 1
fi

# Worker URLs are optional extension points (see HANDOFF.md § Voice pipeline).
# When set they must be present in matched triples (URL + TOKEN, paired for
# ASR/MIDDLE/TTS); partial configuration is rejected so the API does not
# start in a half-wired state.
for stage in ASR MIDDLE TTS; do
    url_var="STHIRA_${stage}_URL"
    tok_var="STHIRA_${stage}_TOKEN"
    url_val="${!url_var:-}"
    tok_val="${!tok_var:-}"
    if [[ -n "${url_val}" && -z "${tok_val}" ]]; then
        err "${tok_var} must be set if ${url_var} is set"
        missing=1
    fi
    if [[ -z "${url_val}" && -n "${tok_val}" ]]; then
        err "${url_var} must be set if ${tok_var} is set"
        missing=1
    fi
done

if (( missing )); then
    exit 1
fi

# Compose project naming must collide with neither the demo nor recovery
# stacks. compose.recovery.yml uses prefix r7recover_ against its DB; demo
# uses port 8000 with no prefix. We additionally refuse an empty or pathy
# name so a stray `STHIRA_DEPLOY_PROJECT=../foo` cannot escalate to wiping
# an adjacent volume.
if [[ ! "${PROJECT_NAME}" =~ ^[a-z0-9_-]+$ ]]; then
    err "STHIRA_DEPLOY_PROJECT=${PROJECT_NAME} must match ^[a-z0-9_-]+\$"
    exit 1
fi

echo "preflight ok: project=${PROJECT_NAME}, pg_db=${STHIRA_PG_DATABASE}"
