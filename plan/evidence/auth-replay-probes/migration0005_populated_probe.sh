#!/bin/sh
# Worker-D probe D (baseline 4095400): populated legacy-session migration test.
# Applies the ACTUAL migration sequence: 0001..0004 -> populate -> 0005 -> 0006..0007
# on a uniquely owned disposable DB. Never edits migration history.
# Verified 2026-09-21 on PostgreSQL 18 + PostGIS; expected outcome in results.txt.
set -eu
MIG=$(dirname "$0")/../../../backend/migrations   # adjust to repo layout
DB=${1:-authreplay_wd_mig}
createdb "$DB"
for f in 0001 0002 0003 0004; do
  psql -q -v ON_ERROR_STOP=1 -d "$DB" -f "$(ls $MIG/${f}_*.sql)" >/dev/null
done
TOK=deadbeef0123456789
CRED=$(printf '%s' "$TOK" | shasum -a 256 | cut -d' ' -f1)
psql -q -v ON_ERROR_STOP=1 -d "$DB" -v cred="$CRED" <<'SQL'
-- legacy self-attested OPERATOR session (pre-0005 row shape): live, MFA set
INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, created_at, expires_at, mfa_verified_at)
VALUES ('SESS-legacy-selfattested','OPERATOR','MH', :'cred', now(), now() + interval '8 hours', now());
INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, created_at, expires_at)
VALUES ('SESS-citizen-keep','CITIZEN',NULL,'citizen-cred-hash', now(), now() + interval '24 hours');
SQL
echo "--- before 0005:"
psql -d "$DB" -tAc "SELECT session_id, coalesce(revoked_at::text,'LIVE') FROM sessions ORDER BY session_id"
psql -q -v ON_ERROR_STOP=1 -d "$DB" -f "$(ls $MIG/0005_*.sql)"
echo "--- after 0005:"
psql -d "$DB" -tAc "SELECT session_id, coalesce(revoked_at::text,'LIVE') FROM sessions ORDER BY session_id"
for f in 0006 0007; do
  psql -q -v ON_ERROR_STOP=1 -d "$DB" -f "$(ls $MIG/${f}_*.sql)" >/dev/null
done
echo "--- after 0007:"
psql -d "$DB" -tAc "SELECT session_id, coalesce(revoked_at::text,'LIVE') FROM sessions ORDER BY session_id; SELECT MAX(revision) FROM schema_migrations"
# Keep DB for probe E1 (real HTTP 401 with token $TOK), then: dropdb "$DB"
