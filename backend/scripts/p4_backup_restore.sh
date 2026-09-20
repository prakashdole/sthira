#!/usr/bin/env bash
# P3-closure backup/restore exercise (A3). Uses uniquely named disposable
# synthetic databases, never overwrites or drops a pre-existing database, and
# tracks which databases this run created. No credentials are embedded: it uses
# the local trust socket (PGUSER defaults to the current user).
#
# Usage: bash scripts/p4_backup_restore.sh
# Requires: pg_dump, pg_restore, createdb, dropdb, psql on PATH (PostgreSQL 18).
set -euo pipefail

# Unique disposable names for this run; never a fixed existing database.
RUN_ID="$$-$(date +%s)"
SRC_DB="sthira_p4_src_${RUN_ID}"
DST_DB="sthira_p4_restore_${RUN_ID}"
DUMP_FILE="$(mktemp -t sthira_p4_dump).dump"
MIGRATION="$(cd "$(dirname "$0")/.." && pwd)/migrations/0001_p3_foundation.sql"

PGUSER="${PGUSER:-$USER}"
export PGUSER

# Use client tools matching the server major version. Homebrew's pg18 is
# keg-only, so prefer its bin dir; override with PGBIN if installed elsewhere.
PGBIN="${PGBIN:-/opt/homebrew/opt/postgresql@18/bin}"
if [ -x "$PGBIN/pg_dump" ]; then
  PATH="$PGBIN:$PATH"
fi

cleanup() {
  # Drop ONLY the databases this run created.
  dropdb --if-exists "$SRC_DB" 2>/dev/null || true
  dropdb --if-exists "$DST_DB" 2>/dev/null || true
  rm -f "$DUMP_FILE"
}
trap cleanup EXIT

echo "== versions =="
pg_dump --version
psql -d postgres -tAc "SELECT version();" | head -1

# Safety: refuse to touch a database that already exists.
for db in "$SRC_DB" "$DST_DB"; do
  if psql -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$db'" | grep -q 1; then
    echo "REFUSE: database $db already exists; would not overwrite" >&2
    exit 1
  fi
done

echo "== create disposable source $SRC_DB =="
createdb "$SRC_DB"
psql -d "$SRC_DB" -v ON_ERROR_STOP=1 -f "$MIGRATION" >/dev/null
echo "PostGIS: $(psql -d "$SRC_DB" -tAc "SELECT PostGIS_Version();")"

echo "== seed synthetic records with known relationships =="
psql -d "$SRC_DB" -v ON_ERROR_STOP=1 <<'SQL' >/dev/null
BEGIN;
INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at)
  VALUES ('SRC-BR', 'gov-test', 'gov.example', 'OPERATIONAL', 1, now());
INSERT INTO source_authorizations (authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
  VALUES ('AUTH-BR', 'SRC-BR', 'authority-1', 'doc-1', 'JUR-BR', now());
INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
  VALUES ('ART-BR', 'SRC-BR', 1, repeat('a',64), now(), 'SYNTHETIC_DEMO', 'blob://art-br');
INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
  VALUES ('PKG-BR', 'ALERT-BR', 'SRC-BR', 'ART-BR', 1, 'JUR-BR', 'SYNTHETIC_DEMO', now(), now()+interval '1 day', repeat('b',64), '{}');
INSERT INTO sessions (session_id, principal_kind, credential_ref, expires_at)
  VALUES ('SES-BR', 'CITIZEN', 'cred', now()+interval '1 hour');
INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, version, updated_at)
  VALUES ('FAC-BR', current_date, 5, 2, 3, now());
INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at)
  VALUES ('RES-BR', 'SES-BR', 'FAC-BR', current_date, 2, 'RESERVED', 1, now(), now());
INSERT INTO idempotency_keys (scope, operation, idem_key, payload_hash, result, state, expires_at)
  VALUES ('SES-BR', 'reservation.create', 'KEY-BR', repeat('c',64), '{"reservation_id":"RES-BR"}', 'COMPLETED', now()+interval '1 hour');
INSERT INTO audit_events (event_id, occurred_at, actor_id, action, subject_id, outcome, to_state, prev_hash, event_hash)
  VALUES ('EV-BR', now(), 'SES-BR', 'RESERVATION_CREATE', 'RES-BR', 'OK', 'RESERVED', repeat('0',64), repeat('d',64));
INSERT INTO outbox_events (event_id, aggregate, event_type, payload, occurred_at)
  VALUES ('OUT-BR', 'reservation', 'RESERVATION_CREATE', '{"reservation_id":"RES-BR"}', now());
COMMIT;
SQL

echo "== dump $SRC_DB =="
pg_dump -Fc -f "$DUMP_FILE" "$SRC_DB"

echo "== restore into separate $DST_DB =="
createdb "$DST_DB"
pg_restore -d "$DST_DB" "$DUMP_FILE"

echo "== verify restored database =="
psql -d "$DST_DB" -v ON_ERROR_STOP=1 <<'SQL'
-- Migration revision and PostGIS.
SELECT 'revision', COALESCE(MAX(revision),0) FROM schema_migrations;
SELECT 'postgis', PostGIS_Version();
-- SRID 4326 geography columns present.
SELECT 'geography_columns', count(*) FROM geography_columns;
-- Record relationships (FK integrity), not just row counts.
SELECT 'reservation_joins', count(*)
  FROM reservations r
  JOIN sessions s ON s.session_id = r.session_id
  JOIN facility_inventory fi ON fi.facility_id = r.facility_id AND fi.service_date = r.service_date
  WHERE r.reservation_id = 'RES-BR';
SELECT 'package_joins', count(*)
  FROM packages p
  JOIN sources src ON src.source_id = p.source_id
  JOIN source_artifacts a ON a.artifact_id = p.artifact_id
  WHERE p.package_id = 'PKG-BR';
-- Capacity conservation.
SELECT 'conservation_ok', (reserved <= capacity) FROM facility_inventory WHERE facility_id='FAC-BR';
-- Idempotency replay state preserved.
SELECT 'idem_completed', count(*) FROM idempotency_keys
  WHERE scope='SES-BR' AND operation='reservation.create' AND idem_key='KEY-BR' AND state='COMPLETED'
    AND result->>'reservation_id' = 'RES-BR';
-- Audit + outbox state preserved.
SELECT 'audit_rows', count(*) FROM audit_events WHERE subject_id='RES-BR';
SELECT 'outbox_unpublished', count(*) FROM outbox_events WHERE event_id='OUT-BR' AND published_at IS NULL;
SQL

echo "== OK: backup/restore verified on $DST_DB (disposed on exit) =="
