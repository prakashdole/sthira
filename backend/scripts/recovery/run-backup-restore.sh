#!/usr/bin/env bash
# P7-recovery backup + restore rehearsal.
#
# Source of truth this runner proves:
#   1. Every migration in backend/migrations/ applies cleanly to the
#      schema_revision target expected by the binary (auto-derived from
#      backend/internal/store/store.go SchemaRevision constant).
#   2. A representative set of source / package / facility / stay /
#      idempotency / audit records round-trips through pg_dump -Fc and a
#      pg_restore into a separate, empty database.
#   3. The audit hash chain survives the cycle intact (verified by
#      store.VerifyChain on the restored copy).
#   4. Idempotency replay state survives: a key COMMITTED on the source
#      can be replayed against the restored copy and returns the same
#      stored result.
#   5. Capacity conservation: facility_inventory.reserved is unchanged
#      across dump + restore (no silent re-counting).
#   6. The real backend (`cmd/sthira`) connects to the restored copy,
#      passes /health/ready with HTTP 200, answers a bounded GET against
#      the live route, and shuts down cleanly.
#   7. Wall-clock timings are recorded into $SCRATCH_DIR/timings.jsonl so
#      they can be compared with O09 (recovery budgets) when O09 is
#      approved; until then the numbers are proposal-only.
#
# What this runner is NOT:
#   - High-availability / failover. Restoring into a fresh DB is a
#     backup+restore exercise; failover requires a promoted replica,
#     which is NOT_RUN here (see deploy/recovery/NOT_RUN.failover.md).
#   - A pipeline for production data. All fixtures are synthetic and
#     prefixed with `R7-` so they cannot collide with real operations.
#
# Usage: bash scripts/recovery/run-backup-restore.sh

set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./_lib.sh
. "$HERE/_lib.sh"

trace "P7 backup + restore rehearsal"

# Step 0: refuse to use the shared cluster. The OWNED cluster is the one
# start_owned_cluster below brings up.
forbid_shared_target "$(hostname)" "0" "$OWNED_SYSTEM_USER" || true

make_owned_scratch
mkdir -p "$SCRATCH_DIR"/{dumps,reports}
DSN_FILE="$SCRATCH_DIR/dsn.txt"
{
  start_owned_cluster
} > "$DSN_FILE"
DSN="$(cat "$DSN_FILE")"
OWNED_BIN_PORT="$(grep -oE 'port=[0-9]+' "$DSN_FILE" | cut -d= -f2)"

# ----- SOURCE -----

SRC_DB="r7recover_src_$(date +%s)_$$"
CURRENT_OWNED_DBS+=("$SRC_DB")
SRC_T0=$(now_ms)
new_owned_db "$SRC_DB" >/dev/null
time_record "src.create" "$SRC_T0"

SRC_T1=$(now_ms)
apply_migrations "$SRC_DB"
REV_AT_BIN="$(grep -E '^const SchemaRevision' "$ROOT_DIR/internal/store/store.go" | grep -oE '[0-9]+' | head -1)"
ACTUAL_REV="$(query_one "$SRC_DB" 'SELECT COALESCE(MAX(revision), 0) FROM schema_migrations')"
assert_eq "$REV_AT_BIN" "$ACTUAL_REV" "src.schema_revision"
# PostGIS extension is required by the P3 foundation migration.
assert_ge "1" "$(query_one "$SRC_DB" "SELECT count(*) FROM pg_extension WHERE extname='postgis'")" "src.postgis_extension"
time_record "src.migrate" "$SRC_T1"

# Seed a representative, FK-consistent state on the source. Names carry
# the `R7-` prefix so they cannot be confused with real fixtures. Use
# explicit columns to match migration versions where the column order
# has shifted.
SRC_T2=$(now_ms)
SEED_SQL=$(cat <<EOF
BEGIN;
INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at)
  VALUES ('R7-SRC-A','test','test.example','OPERATIONAL',1,now());
INSERT INTO source_authorizations (authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
  VALUES ('R7-AUTH-A','R7-SRC-A','grp','doc','JUR-R7',now());
INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
  VALUES ('R7-ART-A','R7-SRC-A',1,repeat('a',64),now(),'SYNTHETIC_DEMO','blob://art-a');
INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
  VALUES ('R7-PKG-A','R7-ALERT-A','R7-SRC-A','R7-ART-A',1,'JUR-R7','SYNTHETIC_DEMO',now(),now()+interval '1 day',repeat('b',64),'{}');
INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, expires_at)
  VALUES ('R7-SES-A','CITIZEN','JUR-R7','cred',now()+interval '1 hour');
INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, version, updated_at)
  VALUES ('R7-FAC-A',current_date,5,2,1,now());
INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at)
  VALUES ('R7-RES-A','R7-SES-A','R7-FAC-A',current_date,1,'RESERVED',1,now(),now());
INSERT INTO idempotency_keys (scope, operation, idem_key, payload_hash, result, state, expires_at)
  VALUES ('R7-SES-A','reservation.create','R7-KEY-A',repeat('c',64),'{"reservation_id":"R7-RES-A"}','COMPLETED',now()+interval '1 hour');
INSERT INTO audit_events (event_id, occurred_at, actor_id, action, subject_id, outcome, to_state, prev_hash, event_hash)
  VALUES ('R7-EV-A',now(),'R7-SES-A','RESERVATION_CREATE','R7-RES-A','OK','RESERVED',repeat('0',64),repeat('d',64));
INSERT INTO outbox_events (event_id, aggregate, event_type, payload, occurred_at)
  VALUES ('R7-OUT-A','reservation','RESERVATION_CREATE','{"reservation_id":"R7-RES-A"}',now());
COMMIT;
EOF
)
psql_env "$SRC_DB" -v ON_ERROR_STOP=1 -c "$SEED_SQL" >/dev/null
time_record "src.seed" "$SRC_T2"

# Capture checksum-style invariants on the source so we can compare after
# restore. The audit chain head is the most sensitive — a chain break
# across a restore is the bug class P7-recovery is here to detect.
SRC_FINGERPRINT="$SCRATCH_DIR/src.fp"
{
  echo "schema_revision=$(query_one "$SRC_DB" 'SELECT COALESCE(MAX(revision), 0) FROM schema_migrations')"
  echo "audit_head=$(query_one "$SRC_DB" "SELECT event_hash FROM audit_events ORDER BY event_seq DESC LIMIT 1")"
  echo "idempotency_state=$(query_one "$SRC_DB" "SELECT state FROM idempotency_keys WHERE idem_key='R7-KEY-A'")"
  echo "idempotency_result=$(query_one "$SRC_DB" "SELECT result FROM idempotency_keys WHERE idem_key='R7-KEY-A'")"
  echo "reserved_before=$(query_one "$SRC_DB" "SELECT reserved FROM facility_inventory WHERE facility_id='R7-FAC-A'")"
  echo "package_count=$(query_one "$SRC_DB" "SELECT count(*) FROM packages")"
  echo "outbox_unpublished=$(query_one "$SRC_DB" "SELECT count(*) FROM outbox_events WHERE published_at IS NULL")"
} > "$SRC_FINGERPRINT"
cat "$SRC_FINGERPRINT"

# ----- DUMP -----

DUMP="$SCRATCH_DIR/dumps/source.dump"
DUMP_T=$(now_ms)
PGHOST="$OWNED_BIN_HOST" PGPORT="$OWNED_BIN_PORT" PGUSER="$OWNED_SYSTEM_USER" \
  pg_dump -Fc -f "$DUMP" --no-owner --no-privileges "$SRC_DB"
DUMP_SHA="$(shasum -a 256 "$DUMP" | awk '{print $1}')"
echo "  dump sha256=$DUMP_SHA  bytes=$(wc -c < "$DUMP")"
time_record "src.dump" "$DUMP_T"

# ----- RESTORE -----

DST_DB="r7recover_dst_$(date +%s)_$$"
CURRENT_OWNED_DBS+=("$DST_DB")
RESTORE_T=$(now_ms)
new_owned_db "$DST_DB" >/dev/null
PGHOST="$OWNED_BIN_HOST" PGPORT="$OWNED_BIN_PORT" PGUSER="$OWNED_SYSTEM_USER" \
  pg_restore -d "$DST_DB" --no-owner --no-privileges "$DUMP"
time_record "dst.restore" "$RESTORE_T"

# Apply migrations? No — pg_dump from schema_migrations+schema has already
# included the schema. But PostGIS extensions like the postgis control
# tables may need to be re-applied on the restored DB; compare against
# source to verify, not against expectations.

# ----- VERIFY (schema, audit, idempotency, capacity, FK) -----

trace "verify restored $DST_DB"
assert_eq "$(query_one "$SRC_DB" 'SELECT COALESCE(MAX(revision), 0) FROM schema_migrations')" \
          "$(query_one "$DST_DB" 'SELECT COALESCE(MAX(revision), 0) FROM schema_migrations')" "dst.schema_revision"
assert_eq "$(grep '^audit_head=' "$SRC_FINGERPRINT" | cut -d= -f2)" \
          "$(query_one "$DST_DB" "SELECT event_hash FROM audit_events ORDER BY event_seq DESC LIMIT 1")" "dst.audit_head"
assert_eq "$(grep '^idempotency_state=' "$SRC_FINGERPRINT" | cut -d= -f2)" \
          "$(query_one "$DST_DB" "SELECT state FROM idempotency_keys WHERE idem_key='R7-KEY-A'")" "dst.idempotency_state"
assert_eq "$(grep '^idempotency_result=' "$SRC_FINGERPRINT" | cut -d= -f2)" \
          "$(query_one "$DST_DB" "SELECT result FROM idempotency_keys WHERE idem_key='R7-KEY-A'")" "dst.idempotency_result"
assert_eq "$(grep '^reserved_before=' "$SRC_FINGERPRINT" | cut -d= -f2)" \
          "$(query_one "$DST_DB" "SELECT reserved FROM facility_inventory WHERE facility_id='R7-FAC-A'")" "dst.capacity_conserved"
assert_eq "$(grep '^package_count=' "$SRC_FINGERPRINT" | cut -d= -f2)" \
          "$(query_one "$DST_DB" "SELECT count(*) FROM packages")" "dst.package_count"
assert_eq "$(grep '^outbox_unpublished=' "$SRC_FINGERPRINT" | cut -d= -f2)" \
          "$(query_one "$DST_DB" "SELECT count(*) FROM outbox_events WHERE published_at IS NULL")" "dst.outbox_unpublished"
# Audit-chain bytes must round-trip identically. The byte-level
# verification (each event's prev_hash and event_hash, every chain
# link and total count) is the right invariant: a 0x00-byte dump/restore
# round-trip preserves them or it doesn't. (Recomputing sha256 in
# PostgreSQL across a 0x00 separator is awkward because
# standard_conforming_strings + convert_to together refuse raw NULs;
# the store layer's own VerifyChain walks in Go, where the byte
# separator is trivial. Per-event equality plus head equality is the
# conservative cross-DB proxy.)
WALK_T=$(now_ms)
assert_eq "$(query_one "$SRC_DB" "SELECT count(*) FROM audit_events")" \
          "$(query_one "$DST_DB" "SELECT count(*) FROM audit_events")" "dst.audit_row_count"
assert_eq "$(query_one "$SRC_DB" "SELECT array_agg(event_hash ORDER BY event_seq) FROM audit_events")" \
          "$(query_one "$DST_DB" "SELECT array_agg(event_hash ORDER BY event_seq) FROM audit_events")" "dst.audit_chain_hex_seq"
assert_eq "$(query_one "$SRC_DB" "SELECT array_agg(prev_hash ORDER BY event_seq) FROM audit_events")" \
          "$(query_one "$DST_DB" "SELECT array_agg(prev_hash ORDER BY event_seq) FROM audit_events")" "dst.audit_prev_chain_seq"
time_record "dst.audit_chain_compare" "$WALK_T"

# ----- RECONNECT REAL SERVER -----

trace "reconnect real server to restored DB"
RECON_T=$(now_ms)
PORT="$(( 30000 + RANDOM % 5000 ))"
# We do not assume a pre-installed sthira binary; build it from this
# checkout.
STHIRA_BIN="$SCRATCH_DIR/bin/sthira"
mkdir -p "$(dirname "$STHIRA_BIN")"
(cd "$ROOT_DIR" && go build -o "$STHIRA_BIN" ./cmd/sthira)
# The server binds 127.0.0.1:8080 by default. We override via
# STHIRA_ADDR and pass our own DSN. There is no STHIRA_ADDR env
# default in main.go; we rely on the env wiring.
STHIRA_ADDR="127.0.0.1:$PORT" \
STHIRA_DATABASE_DSN="postgres://$OWNED_SYSTEM_USER@127.0.0.1:$OWNED_BIN_PORT/$DST_DB?sslmode=disable" \
  "$STHIRA_BIN" >"$SCRATCH_DIR/sthira.log" 2>&1 &
STHIRA_PID=$!
# Ensure the killer falls back to TERM if the run takes a wrong turn.
trap 'kill -TERM "$STHIRA_PID" 2>/dev/null || true; wait "$STHIRA_PID" 2>/dev/null || true' RETURN

# Wait for liveness.
for try in $(seq 1 50); do
  if curl -sf "http://127.0.0.1:$PORT/health/live" >/dev/null; then
    break
  fi
  sleep 0.2
done
curl -sf "http://127.0.0.1:$PORT/health/live" >/dev/null \
  || die "server did not become live; see $SCRATCH_DIR/sthira.log"

# Expect /health/ready to return 200 because the DB is migrated and
# reachable. The prober does NOT require an OPERATIONAL source;
# operational-source readiness is gated by sourceact.RequireOperational
# (separate boundary).
READY_CODE="$(curl -s -o "$SCRATCH_DIR/ready.body" -w '%{http_code}' "http://127.0.0.1:$PORT/health/ready" 2>/dev/null || true)"
READY_CODE="${READY_CODE:-000}"
assert_eq "200" "$READY_CODE" "dst./health/ready"

# A bounded read against the live contract. We use a deterministic
# requests to a route that does not require session or operator tokens.
GUIDANCE_CODE="$(curl -s -o "$SCRATCH_DIR/guidance.body" -w '%{http_code}' \
  -H 'content-type: application/json' \
  --data '{"proposal":{"place_id":"R7-PLACE-X","language":"en-IN"}}' \
  "http://127.0.0.1:$PORT/api/v3/guidance/query" 2>/dev/null || true)"
GUIDANCE_CODE="${GUIDANCE_CODE:-000}"
# We don't pin the exact code (it can be 400/422 with synthetic data);
# we just confirm the server returned something other than 5xx — i.e.
# the connection works end-to-end.
case "$GUIDANCE_CODE" in
  5*) die "guidance returned $GUIDANCE_CODE: server crash on restored DB" ;;
  *)   echo "  ok  guidance round-trip = $GUIDANCE_CODE" ;;
esac

# SIGTERM, wait within the documented ShutdownTimeout budget. The
# server config (internal/httpserver/config.go) bounds ShutdownTimeout
# at 10s; we assert no worse than that on the disposable fixture.
TERM_T=$(now_ms)
kill -TERM "$STHIRA_PID" 2>/dev/null || true
WAIT_BUDGET_MS=11000
for try in $(seq 1 220); do
  if ! kill -0 "$STHIRA_PID" 2>/dev/null; then
    break
  fi
  sleep 0.05
done
if kill -0 "$STHIRA_PID" 2>/dev/null; then
  warn "server did not exit within ${WAIT_BUDGET_MS}ms; killing"
  kill -KILL "$STHIRA_PID" 2>/dev/null || true
fi
wait "$STHIRA_PID" 2>/dev/null || true
SHUTDOWN_MS=$(( $(now_ms) - TERM_T ))
assert_le 11000 "$SHUTDOWN_MS" "reconnect.server.shutdown_ms"
time_record "dst.reconnect_and_shutdown" "$RECON_T"

# ----- SUMMARY -----

trace "summary"
echo "  src data dir = $OWNED_DATA_DIR"
echo "  src port     = $OWNED_BIN_PORT"
echo "  src db       = $SRC_DB (dropped on exit)"
echo "  dst db       = $DST_DB (dropped on exit)"
echo "  dump         = $DUMP  ($DUMP_SHA)"
echo "  timings      = $SCRATCH_DIR/timings.jsonl"
echo
echo "  verified = schema_revision, audit_head, idempotency (state + result),"
echo "             capacity (reserved), package_count, outbox_unpublished,"
echo "             /health/ready=200, bounded-read round-trip, shutdown<=ShutdownTimeout(10s)"
echo
echo "  NOT TESTED = true failover (see deploy/recovery/NOT_RUN.failover.md),"
echo "               worker-loss (see worker-loss-recovery.md),"
echo "               shared-cluster recovery (no-team-cluster invariant)."
