#!/usr/bin/env bash
# P7 recovery — shared shell library for backup, restore, dependency-outage,
# graceful-drain and lifecycle-rehearsal runners.
#
# SAFETY INVARIANTS (enforced here, never overridden by a runner):
#
#   1. Every database this file touches MUST be in an explicit OWNED_DB_NAME
#      list with a `r7recover_` prefix. Helpers reject any other name — the
#      script fails before it does anything to a database it didn't create.
#
#   2. No default DSN: every runner MUST source this file and pass an
#      explicit host/port/user when it talks to a live cluster. The script
#      never connects to the team's PostgreSQL instance without an explicit
#      RECOVERY_ALLOW_SHARED=1 ack (used only by experiments on shared
#      infra; the default refuses).
#
#   3. The owned cluster is run via the system's PG bin dir but bound to a
#      randomized port in the high 20000s and a unique data directory under
#      ${RECOVERY_TMP:-/tmp}/p7-recovery. It MUST NEVER bind to a port or
#      directory that exists prior to the run.
#
#   4. Drop helpers refuse any database whose name does not match an
#      allowlist or that is not in the current OWNED_DB_NAME set.
#
#   5. `trap` cleanup drops ONLY the prefixed DBs from CURRENT_OWNED_DBS and
#      stops ONLY the cluster that this run started. Cleanup runs on EXIT,
#      including error / interrupt paths.

set -euo pipefail

# --- Paths ---

RECOVERY_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$RECOVERY_LIB_DIR/.." && pwd)"     # backend/scripts
ROOT_DIR="$(cd "$SCRIPTS_DIR/.." && pwd)"             # backend
MIGRATION_DIR="$ROOT_DIR/migrations"
RECOVERY_TMP="${RECOVERY_TMP:-/tmp/p7-recovery}"

# Where artifacts go. Cleared by clean_scratch.
SCRATCH_DIR=""

# Where the owned cluster writes its data + socket. set up by start_owned_cluster.
OWNED_DATA_DIR=""
OWNED_PORT=""
OWNED_CLUSTER_PID=""
OWNED_SYSTEM_USER="${USER:-$(id -un)}"

# Per-run database name allowlist. The runner MUST populate this before it
# calls any helper that takes a DB name. Names must begin with r7recover_.
CURRENT_OWNED_DBS=()

# Tracing — every helper echoes the stage and the inputs that influence it.
trace() { printf '== %s ==\n' "$*" >&2; }
warn()  { printf '!! %s\n' "$*" >&2; }
die()   { printf 'FATAL: %s\n' "$*" >&2; exit 1; }

# --- safety checks ---

# require_owned_db_name NAME
# Refuses anything that is not r7recover_ prefix or already in
# CURRENT_OWNED_DBS. Used by every helper that mutates a database.
require_owned_db_name() {
  local name="$1"
  if [[ "$name" == r7recover_* ]]; then
    return 0
  fi
  # A name not in the allowlist is rejected, even if the caller swears it's safe.
  local allowed
  for allowed in "${CURRENT_OWNED_DBS[@]:-}"; do
    if [[ "$allowed" == "$name" ]]; then
      return 0
    fi
  done
  die "refuse to operate on database '$name': must start with 'r7recover_' or be in CURRENT_OWNED_DBS"
}

# forbid_shared_target HOST PORT USER
# Refuses to point a runner at the team's PostgreSQL cluster. Detection rules:
#   - port 5432 AND user 'postgres' or missing explicit port override
#   - default Unix socket /tmp/.s.PGSQL.5432 AND user 'postgres' or root
# Operators must set RECOVERY_ALLOW_SHARED=1 once to acknowledge the risk
# before running any shared-cluster experiment.
forbid_shared_target() {
  local host="$1"
  local port="${2:-}"
  local user="${3:-}"
  if [[ "${RECOVERY_ALLOW_SHARED:-}" == "1" ]]; then
    return 0
  fi
  if [[ "$port" == "5432" && ( -z "$user" || "$user" == "postgres" || "$user" == "$OWNED_SYSTEM_USER" ) ]]; then
    die "refuse to use shared PostgreSQL host $host:$port (set RECOVERY_ALLOW_SHARED=1 to override; never set it on CI)"
  fi
  if [[ -z "$port" && ( -z "$user" || "$user" == "postgres" ) ]]; then
    die "refuse to use the default PostgreSQL socket (set RECOVERY_ALLOW_SHARED=1 to override)"
  fi
}

# --- ephemeral scratch + cleanup ---

# make_owned_scratch
# Allocates a unique scratch directory and an owned DB prefix. Registers a
# trap that drops ONLY the owned DBs and ONLY the cluster started by this
# run (other processes' ports and dirs are untouched).
make_owned_scratch() {
  mkdir -p "$RECOVERY_TMP"
  SCRATCH_DIR="$(mktemp -d "$RECOVERY_TMP/r7recover-XXXXXX")"
  trap on_exit_cleanup EXIT
  trap 'on_exit_cleanup; trap - INT; kill -INT $$' INT
  trap 'on_exit_cleanup; trap - HUP; kill -HUP $$' HUP
  trap 'on_exit_cleanup; trap - TERM; kill -TERM $$' TERM
}

# on_exit_cleanup
# On EXIT (including error paths) drop only owned DBs and stop only the
# cluster THIS run started. Never iterate the global pg_database list.
# Never call pg_dropdb on a non-prefixed name.
#
# Order matters: drop the DBs while the cluster is still up; then stop
# the cluster; only then remove the scratch dir. If we stopped the
# cluster first the drops would have to skip their connection attempt.
on_exit_cleanup() {
  local rc=$?
  set +e
  if [[ ${#CURRENT_OWNED_DBS[@]} -gt 0 && -n "$OWNED_CLUSTER_PID" ]]; then
    local db
    # Drop in reverse order of creation (newest first) so a partially-
    # created target DB doesn't shadow the source.
    for (( idx=${#CURRENT_OWNED_DBS[@]}-1; idx >= 0; idx-- )); do
      local db="${CURRENT_OWNED_DBS[$idx]}"
      require_owned_db_name "$db" 2>/dev/null || { warn "skipping non-owned $db"; continue; }
      trace "drop owned db $db"
      drop_owned_db "$db" 2>"$SCRATCH_DIR/cleanup.log" || warn "drop $db failed; see $SCRATCH_DIR/cleanup.log"
    done
  fi
  if [[ -n "$OWNED_CLUSTER_PID" ]]; then
    trace "stop_owned_cluster pid=$OWNED_CLUSTER_PID data=$OWNED_DATA_DIR port=$OWNED_BIN_PORT"
    pg_ctl -D "$OWNED_DATA_DIR" -m fast stop >/dev/null 2>&1 || true
    OWNED_CLUSTER_PID=""
  fi
  if [[ -n "$SCRATCH_DIR" && -d "$SCRATCH_DIR" ]]; then
    rm -rf "$SCRATCH_DIR"
  fi
  return $rc
}

# --- DB operations on the owned cluster ---

# new_owned_db [DESTINATION_NAME]
# Creates a uniquely-named DB. The default name is r7recover_<pid-ts>, which
# is already in the allowlist. To use a different name the caller must also
# list it in CURRENT_OWNED_DBS before the call.
new_owned_db() {
  local db="${1:-r7recover_$(date +%s)-$RANDOM}"
  require_owned_db_name "$db"
  psql_env postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$db\""
  CURRENT_OWNED_DBS+=("$db")
  echo "$db"
}

# drop_owned_db NAME
# Drops ONLY an owned DB. Refuses to touch anything else, including via
# --if-exists against an unknown name (the script must know the name).
drop_owned_db() {
  local db="$1"
  require_owned_db_name "$db"
  psql_env postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS \"$db\""
}

# apply_migrations DB
# Applies every migration file in order.
apply_migrations() {
  local db="$1"
  require_owned_db_name "$db"
  local m
  for m in "$MIGRATION_DIR"/0*.sql; do
    [[ -f "$m" ]] || continue
    trace "apply $m to $db"
    psql_env "$db" -v ON_ERROR_STOP=1 -f "$m" >/dev/null
  done
}

# query_one DB SQL -> stdout
# Single-select helper for the assertion scripts.
query_one() {
  local db="$1"
  local sql="$2"
  require_owned_db_name "$db"
  psql_env "$db" -tAc "$sql"
}

# psql_env DB ARGS...
# Wraps `psql` with the libpq env vars pointing at the owned cluster.
# Never carries a default port or user; if `OWNED_BIN_PORT` is unset we
# error out explicitly rather than silently fall back to /tmp socket.
psql_env() {
  if [[ -z "$OWNED_BIN_PORT" || -z "$OWNED_BIN_HOST" ]]; then
    die "psql_env called before start_owned_cluster"
  fi
  PGHOST="$OWNED_BIN_HOST" PGPORT="$OWNED_BIN_PORT" PGUSER="$OWNED_SYSTEM_USER" \
    psql "$@"
}

# --- timing ---

# now_ms: portable millisecond-resolution epoch.
#
# macOS / BSD `date` returns %N as 9-digit nanoseconds-within-second and
# supports `%s%N` (10 second digits + 9 ns digits). We strip the last 6
# digits of the nanoseconds portion to get ms within the second, then
# compose `s * 1000 + ms_within_second`. GNU date produces the same
# shape so the same parsing works on Linux.
now_ms() {
  local ts
  ts="$(date +%s%N 2>/dev/null || true)"
  if [[ -n "$ts" && ${#ts} -ge 13 && "$ts" != *N* ]]; then
    # Split into seconds (all but last 9 chars) and nanoseconds (last 9).
    local len=${#ts}
    local ns_off=$(( len - 9 ))
    local s_part="${ts:0:$ns_off}"
    local ns_part="${ts:$ns_off}"
    # Truncate to ms (drop last 6 digits of the ns part). bash arithmetic
    # needs decimal-safe inputs; leading zeros on ns_part would break $((10#..)),
    # so strip them.
    local ns_ms="${ns_part%??????}"
    if [[ -z "$ns_ms" ]]; then ns_ms="0"; fi
    # Convert leading zeros if any.
    echo $(( 10#$s_part * 1000 + 10#$ns_ms ))
    return 0
  fi
  # Fallback to python3 (always present on modern macOS).
  if command -v python3 >/dev/null 2>&1; then
    python3 -c 'import time; print(int(time.time()*1000))'
    return 0
  fi
  # Last resort: seconds only. Adequate for drain window tests.
  echo "$(($(date +%s) * 1000))"
}

# time_record STAGE START_MS
# Records wall_ms from START_MS to now. Logs to $SCRATCH_DIR/timings.jsonl.
time_record() {
  local stage="$1"
  local started_ms="$2"
  local elapsed_ms=$(( $(now_ms) - started_ms ))
  printf '{"stage":"%s","wall_ms":%d,"host":"%s","started_at":"%s"}\n' \
    "$stage" "$elapsed_ms" "$(hostname)" "$(date -u +%FT%TZ)" \
    >> "$SCRATCH_DIR/timings.jsonl"
}

# --- cluster lifecycle ---

# start_owned_cluster
# Allocates a port + data dir, runs initdb, starts the cluster, waits for
# pg_isready, returns the DSN on stdout.
start_owned_cluster() {
  if [[ -z "$SCRATCH_DIR" ]]; then
    die "start_owned_cluster called before make_owned_scratch"
  fi
  OWNED_DATA_DIR="$SCRATCH_DIR/pgdata"
  # Pick a port in the high 20000s that no other runner is using. We avoid
  # 5432 explicitly. The chosen port may collide with an unrelated
  # service; if so, retry a few times before giving up.
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    OWNED_PORT="$(( 24000 + RANDOM % 4000 ))"
    if ! (echo > /dev/tcp/127.0.0.1/$OWNED_PORT) 2>/dev/null; then
      break
    fi
  done
  if [[ -z "$OWNED_PORT" ]]; then
    die "could not allocate a free port in 24000-27999"
  fi
  OWNED_BIN_HOST="127.0.0.1"

  # Use the homebrew postgresql@18 bin dir when available (it ships with
  # postgis). Without postgis, the P3 foundation migration fails on
  # CREATE EXTENSION postgis. If neither path is available, fall back
  # to whatever `command -v initdb` returns and let the migration fail
  # loudly.
  PGBIN_DIR=""
  for cand in /opt/homebrew/opt/postgresql@18/bin /opt/homebrew/opt/postgresql@17/bin /opt/homebrew/opt/postgresql@16/bin; do
    if [[ -x "$cand/initdb" ]]; then
      PGBIN_DIR="$cand"
      break
    fi
  done
  if [[ -z "$PGBIN_DIR" ]]; then
    PGBIN_DIR="$(dirname "$(command -v initdb)")"
  fi
  export PATH="$PGBIN_DIR:$PATH"
  OWNED_PGBIN="$PGBIN_DIR"
  trace "initdb data=$OWNED_DATA_DIR bin=$OWNED_PGBIN"
  initdb -D "$OWNED_DATA_DIR" -E UTF8 --locale=C -U "$OWNED_SYSTEM_USER" -A trust >"$SCRATCH_DIR/initdb.log" 2>&1
  # Listen on the random port only; no Unix socket in /tmp so it cannot
  # collide with another cluster sharing a host.
  cat >> "$OWNED_DATA_DIR/postgresql.conf" <<EOF
port = ${OWNED_PORT}
listen_addresses = '127.0.0.1'
unix_socket_directories = '${OWNED_DATA_DIR}'
shared_buffers = 64MB
fsync = on
log_min_messages = warning
EOF
  trace "pg_ctl start port=$OWNED_PORT"
  pg_ctl -D "$OWNED_DATA_DIR" -l "$SCRATCH_DIR/pg.log" -o "-p $OWNED_PORT -h 127.0.0.1 -k ${OWNED_DATA_DIR}" start >"$SCRATCH_DIR/start.log" 2>&1
  OWNED_CLUSTER_PID="$(head -1 "$OWNED_DATA_DIR/postmaster.pid" 2>/dev/null || echo '')"
  # Wait for readiness.
  for _ in $(seq 1 30); do
    if pg_isready -h 127.0.0.1 -p "$OWNED_PORT" -q; then
      OWNED_BIN_HOST="127.0.0.1"
      OWNED_BIN_PORT="$OWNED_PORT"
      trace "cluster ready at 127.0.0.1:$OWNED_PORT pid=$OWNED_CLUSTER_PID"
      echo "host=$OWNED_BIN_HOST port=$OWNED_PORT user=$OWNED_SYSTEM_USER"
      return 0
    fi
    sleep 0.2
  done
  die "owned cluster did not become ready; see $SCRATCH_DIR/pg.log and start.log"
}

# Note: OWNED_BIN_PORT is set by start_owned_cluster (above). Runners that
# want to use psql_env must call start_owned_cluster first.

# stop_owned_cluster [MODE]
# Mode is `fast` (default) or `immediate`. We intentionally never use
# `pg_ctl -m immediate` for exercises (it can skip shared-buffer flush; it
# does not represent a graceful failover). Failover simulation uses
# `fast`; restart simulation is `fast` too. Immediate is reserved for
# scripts that explicitly opt in via RECOVERY_ALLOW_IMMEDIATE=1.
stop_owned_cluster() {
  local mode="${1:-fast}"
  if [[ "$mode" == "immediate" && "${RECOVERY_ALLOW_IMMEDIATE:-}" != "1" ]]; then
    die "stop_owned_cluster immediate requires RECOVERY_ALLOW_IMMEDIATE=1"
  fi
  if [[ -z "$OWNED_DATA_DIR" ]]; then
    die "stop_owned_cluster: no owned cluster"
  fi
  trace "pg_ctl stop data=$OWNED_DATA_DIR mode=$mode"
  pg_ctl -D "$OWNED_DATA_DIR" -m "$mode" stop >/dev/null 2>&1 || true
  OWNED_CLUSTER_PID=""
}

# restart_owned_cluster
# The simplest lifecycle operation: a graceful shutdown + restart on the
# same data dir. Distinct from restore (same schema, same data) and from
# HA / failover (would require a promoted replica).
restart_owned_cluster() {
  stop_owned_cluster fast
  for _ in $(seq 1 30); do
    if pg_isready -h 127.0.0.1 -p "${OWNED_BIN_PORT:-$OWNED_PORT}" -q; then
      return 0
    fi
    sleep 0.2
  done
  trace "pg_ctl start post-restart"
  pg_ctl -D "$OWNED_DATA_DIR" -l "$SCRATCH_DIR/pg-restarred.log" \
    -o "-p $OWNED_BIN_PORT -h 127.0.0.1 -k ${OWNED_DATA_DIR}" start \
    >"$SCRATCH_DIR/restart.log" 2>&1
  for _ in $(seq 1 30); do
    if pg_isready -h 127.0.0.1 -p "$OWNED_BIN_PORT" -q; then
      return 0
    fi
    sleep 0.2
  done
  die "restart_owned_cluster: cluster did not become ready after restart"
}

# --- assertion helpers ---

# assert_eq EXPECTED ACTUAL LABEL
assert_eq() {
  local expected="$1"
  local actual="$2"
  local label="$3"
  if [[ "$expected" != "$actual" ]]; then
    die "ASSERT $label: expected [$expected], got [$actual]"
  fi
  echo "  ok  $label = $actual"
}

# assert_ge EXPECTED ACTUAL LABEL
assert_ge() {
  local min="$1"
  local actual="$2"
  local label="$3"
  if (( actual < min )); then
    die "ASSERT $label: expected >= $min, got $actual"
  fi
  echo "  ok  $label = $actual (>= $min)"
}

# assert_le MAX ACTUAL LABEL
assert_le() {
  local max="$1"
  local actual="$2"
  local label="$3"
  if (( actual > max )); then
    die "ASSERT $label: expected <= $max, got $actual"
  fi
  echo "  ok  $label = $actual (<= $max)"
}
