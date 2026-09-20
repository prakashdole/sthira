#!/bin/bash
# Verify migration 0005 on a populated disposable DB: legacy unbound OPERATOR
# sessions are revoked, citizen sessions preserved, revision reaches 5.
# Never touches sthira_test. Creates and drops its own uniquely-named DB.
DB="sthira_migtest_$(date +%s)"
MIG="/Users/apple/Documents/Projects/MonitoringZ/backend/migrations"

echo "== create $DB =="
psql 'postgres://apple@localhost:5432/postgres' -c "CREATE DATABASE $DB;" || exit 1

for f in 0001_p3_foundation 0002_p4_stays 0003_p4_operator 0004_p4_operator_grants; do
  echo "== apply $f =="
  psql "postgres://apple@localhost:5432/$DB" -q -f "$MIG/$f.sql" || { echo "FAILED $f"; psql 'postgres://apple@localhost:5432/postgres' -c "DROP DATABASE $DB;"; exit 1; }
done

echo "== seed legacy sessions =="
psql "postgres://apple@localhost:5432/$DB" -c "INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, created_at, expires_at) VALUES ('SESS-LEGACY-OP','OPERATOR','JTEST','hash-x',now(),now()+interval '1 hour'), ('SESS-CIT','CITIZEN',NULL,'hash-y',now(),now()+interval '1 hour');"

echo "== BEFORE 0005 =="
psql "postgres://apple@localhost:5432/$DB" -c "SELECT session_id, principal_kind, revoked_at IS NOT NULL AS revoked FROM sessions;"

echo "== apply 0005 =="
psql "postgres://apple@localhost:5432/$DB" -q -f "$MIG/0005_p4_operator_identity.sql" || { echo "FAILED 0005"; psql 'postgres://apple@localhost:5432/postgres' -c "DROP DATABASE $DB;"; exit 1; }

echo "== AFTER 0005 =="
psql "postgres://apple@localhost:5432/$DB" -c "SELECT session_id, principal_kind, operator_subject, revoked_at IS NOT NULL AS revoked FROM sessions;"
psql "postgres://apple@localhost:5432/$DB" -c "SELECT MAX(revision) AS rev FROM schema_migrations;"

echo "== drop $DB =="
psql 'postgres://apple@localhost:5432/postgres' -c "DROP DATABASE $DB;"
echo "DONE"
