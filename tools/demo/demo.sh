#!/usr/bin/env bash
# ==============================================================================
# Sthira v2 - one-command demo bring-up.
#
#   ./tools/demo/demo.sh start     build THIS tree, fresh disposable database,
#                                  mock workers + sthira-exercise + Vite, all on
#                                  127.0.0.1, and wait for READY
#   ./tools/demo/demo.sh status    per-component process/port/URL + /health/ready
#   ./tools/demo/demo.sh stop      kill ONLY this run's PIDs, drop ONLY its DB
#
# This replaces the manual sequence in scripts/run_demo_rehearsal.sh for day-to-
# day demo use. The rehearsal script stays the acceptance/journey harness.
# ==============================================================================

set -euo pipefail

# Resolve the repo root from this script's own location so any cwd works.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend/v2"
FRONTEND_DIR_REAL="$(cd "$FRONTEND_DIR" && pwd -P)"

# ---- env-var scheme (every one of these is optional) -------------------------
DEMO_DIR="${DEMO_DIR:-/tmp/sthira-demo}"   # bin/ and run/ live here
BIN_DIR="$DEMO_DIR/bin"
RUN_DIR="$DEMO_DIR/run"
STATE_FILE="$RUN_DIR/state.env"           # PIDs, db name, started_at (no secrets)
WORKER_PORT="${DEMO_WORKER_PORT:-18880}"  # mock workers (or 0 when --real-workers)
BACKEND_PORT="${DEMO_BACKEND_PORT:-18881}"  # sthira-exercise
UI_PORT="${DEMO_UI_PORT:-18882}"          # Vite dev server
SCENARIO="${DEMO_SCENARIO:-destination-choice}"
READY_TIMEOUT_SECS="${DEMO_READY_TIMEOUT_SECS:-60}"
ADMIN_DSN="${DEMO_ADMIN_DSN:-${STHIRA_TEST_ADMIN_DSN:-postgres://localhost:5432/postgres?sslmode=disable}}"
EXERCISE_BIN="$BIN_DIR/sthira-exercise"
WORKERS_BIN="$BIN_DIR/mock-workers"
EXERCISE_LOG="$RUN_DIR/sthira-exercise.log"
WORKERS_LOG="$RUN_DIR/mock-workers.log"
UI_LOG="$RUN_DIR/vite.log"
LSOFT_BIN="$(command -v lsof || echo /usr/sbin/lsof)"

# ---- run state (overwritten from the state file by stop/status) --------------
STATE_ACTIVE=0
STARTED_AT=""
STOPPED_AT=""
DB_NAME=""
DB_CREATED=0
WORKER_MODE="mock"
WORKER_PID=""
EXERCISE_PID=""
UI_PID=""

REAL_WORKERS=0

die() {
    printf 'ERROR: %s\n' "$*" >&2
    exit 1
}

usage() {
    cat <<'EOF'
usage: demo.sh <start|stop|status> [--real-workers]

  start   build this tree, create a disposable database, start mock workers +
          sthira-exercise + Vite on 127.0.0.1, wait for /health/ready READY
  status  report each component (pid/port/URL), the database and readiness
  stop    kill only the PIDs recorded in the state file, drop only its database

env (all optional):
  DEMO_WORKER_PORT=18880  DEMO_BACKEND_PORT=18881  DEMO_UI_PORT=18882
  DEMO_SCENARIO=destination-choice
  DEMO_DIR=/tmp/sthira-demo            DEMO_READY_TIMEOUT_SECS=60
  DEMO_ADMIN_DSN (else STHIRA_TEST_ADMIN_DSN, else
                 postgres://localhost:5432/postgres?sslmode=disable)

  --real-workers  use STHIRA_{ASR,MIDDLE,TTS}_URL and the optional
                  STHIRA_{ASR,MIDDLE,TTS}_TOKEN already in your shell instead
                  of starting the mocks. Tokens are never printed or stored.
EOF
}

# ---------------------------------------------------------------- state file
# key=value lines, whitelist-only. Never eval, never persist a token or a DSN.
read_state() {
    local line k v
    STATE_ACTIVE=0; STARTED_AT=""; STOPPED_AT=""; DB_NAME=""; DB_CREATED=0
    WORKER_MODE="mock"; WORKER_PID=""; EXERCISE_PID=""; UI_PID=""
    [ -f "$STATE_FILE" ] || return 1
    while IFS= read -r line; do
        case "$line" in *=*) ;; *) continue ;; esac
        k="${line%%=*}"
        v="${line#*=}"
        case "$k" in
            STATE_ACTIVE|STARTED_AT|STOPPED_AT|DB_NAME|DB_CREATED|WORKER_MODE|\
            WORKER_PID|EXERCISE_PID|UI_PID|WORKER_PORT|BACKEND_PORT|UI_PORT|SCENARIO)
                printf -v "$k" '%s' "$v"
                ;;
        esac
    done < "$STATE_FILE"
    return 0
}

write_state() {
    mkdir -p "$RUN_DIR"
    {
        printf 'STATE_ACTIVE=%s\n' "$STATE_ACTIVE"
        printf 'STARTED_AT=%s\n' "$STARTED_AT"
        printf 'STOPPED_AT=%s\n' "$STOPPED_AT"
        printf 'DB_NAME=%s\n' "$DB_NAME"
        printf 'DB_CREATED=%s\n' "$DB_CREATED"
        printf 'WORKER_MODE=%s\n' "$WORKER_MODE"
        printf 'WORKER_PORT=%s\n' "$WORKER_PORT"
        printf 'BACKEND_PORT=%s\n' "$BACKEND_PORT"
        printf 'UI_PORT=%s\n' "$UI_PORT"
        printf 'SCENARIO=%s\n' "$SCENARIO"
        printf 'WORKER_PID=%s\n' "$WORKER_PID"
        printf 'EXERCISE_PID=%s\n' "$EXERCISE_PID"
        printf 'UI_PID=%s\n' "$UI_PID"
    } > "$STATE_FILE"
}

# ------------------------------------------------------------------- helpers
tcp_open() { (exec 3<>"/dev/tcp/127.0.0.1/$1") >/dev/null 2>&1; }

require_free_port() {
    if tcp_open "$2"; then
        die "port $2 ($1) is already in use. Stop that process or set a different port env var."
    fi
}

db_dsn() { # <dbname> -> DSN with the database segment replaced
    local base="${ADMIN_DSN%/*}"
    [ "$base" != "$ADMIN_DSN" ] || die "DEMO_ADMIN_DSN must name a database path, e.g. postgres://localhost:5432/postgres?sslmode=disable"
    printf '%s/%s' "$base" "$1"
}

# $1 = kind (bin|cwd), $2 = pid, $3 = marker to prove it is the process we started,
# $4 = optional extra argv substring required for the `cwd` kind (a bare cwd match
#      is not proof of ownership: any other vite/npm run in frontend/v2 would match).
pid_is_ours() {
    local kind="$1" pid="$2" marker="$3" argv_marker="${4:-}" args
    [ -n "$pid" ] || return 1
    kill -0 "$pid" 2>/dev/null || return 1
    case "$kind" in
        bin)
            args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
            case "$args" in *"$marker"*) return 0 ;; esac
            ;;
        cwd)
            args="$("$LSOFT_BIN" -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' || true)"
            [ "$args" = "$marker" ] || return 1
            if [ -n "$argv_marker" ]; then
                args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
                case "$args" in *"$argv_marker"*) return 0 ;; *) return 1 ;; esac
            fi
            return 0
            ;;
    esac
    return 1
}

# `npm run dev` spawns sh -> node, so a verified npm PID owns a whole subtree.
kill_tree() {
    local pid="$1" child
    local children=()
    while IFS= read -r child; do
        if [ -n "$child" ]; then children+=("$child"); fi
    done < <(pgrep -P "$pid" 2>/dev/null || true)
    if [ ${#children[@]} -gt 0 ]; then
        for child in "${children[@]}"; do kill_tree "$child"; done
    fi
    kill "$pid" 2>/dev/null || true
}

# Compact readiness read: HTTP status + data.status / data.subsystems.models.status,
# or the API's own error message when it fails closed.
ready_of() {
    local out body code
    out="$(curl -s -w '\n%{http_code}' --max-time 2 "$1/health/ready" 2>/dev/null || true)"
    if [ -z "$out" ]; then printf 'UNREACHABLE'; return 0; fi
    code="${out##*$'\n'}"
    body="${out%$'\n'*}"
    if [ -z "$body" ]; then printf 'HTTP %s (empty body)' "$code"; return 0; fi
    printf 'HTTP %s %s' "$code" "$(node -e '
        const j = JSON.parse(process.argv[1]);
        const d = j.data;
        if (!d) {
            const e = (j.errors || [])[0] || {};
            process.stdout.write("NO_DATA" + (e.message || e.code ? " (" + (e.message || e.code) + ")" : ""));
            process.exit(0);
        }
        const m = (d.subsystems || {}).models || {};
        process.stdout.write(d.status + "/models=" + (m.status || "absent"));
    ' "$body" 2>/dev/null || printf 'UNPARSEABLE')"
}

db_state() { # -> exists | absent | unknown (psql failed)
    local n
    if [ -z "$DB_NAME" ]; then printf 'n/a'; return 0; fi
    if n="$(psql "$ADMIN_DSN" -tAc "SELECT 1 FROM pg_database WHERE datname='$DB_NAME';" 2>/dev/null)"; then
        if [ "$n" = "1" ]; then printf 'exists'; else printf 'absent'; fi
    else
        printf 'unknown (psql failed)'
    fi
}

# ------------------------------------------------------------------ cmd: start
require_tools() {
    local t
    for t in "$@"; do
        command -v "$t" >/dev/null 2>&1 || die "required command not found: $t"
    done
}

cleanup_on_failure() {
    local rc=$?
    [ "$STATE_ACTIVE" -eq 1 ] || return 0
    set +e
    printf '\n!! start failed (exit %s) - cleaning up the partial demo run\n' "$rc" >&2
    stop_run
    return "$rc"
}

cmd_start() {
    require_tools go psql curl node npm sed grep tail

    # Validate --real-workers input before anything is built or created, so a
    # config error costs a second and leaves no database behind.
    local asr="" mid="" tts=""
    if [ "$REAL_WORKERS" -eq 1 ]; then
        asr="${STHIRA_ASR_URL:-}"
        mid="${STHIRA_MIDDLE_URL:-}"
        tts="${STHIRA_TTS_URL:-}"
        [ -n "$asr" ] || die "--real-workers requires STHIRA_ASR_URL in your environment"
        [ -n "$mid" ] || die "--real-workers requires STHIRA_MIDDLE_URL in your environment"
        [ -n "$tts" ] || die "--real-workers requires STHIRA_TTS_URL in your environment"
    fi

    # Only the active-run guard needs the old state; env must win for ports and
    # scenario, so never let a stale state file overwrite this run's config.
    if [ -f "$STATE_FILE" ] && grep -q '^STATE_ACTIVE=1$' "$STATE_FILE"; then
        die "a demo run is already recorded as active (state $STATE_FILE). Run '$0 stop' first."
    fi

    mkdir -p "$BIN_DIR" "$RUN_DIR"
    : > "$EXERCISE_LOG"
    : > "$WORKERS_LOG"
    : > "$UI_LOG"

    # 1. Build from this tree.
    printf '>> building cmd/sthira-exercise and cmd/mock-workers from %s\n' "$BACKEND_DIR"
    (cd "$BACKEND_DIR" && go build -o "$EXERCISE_BIN" ./cmd/sthira-exercise) \
        || die "go build ./cmd/sthira-exercise failed"
    (cd "$BACKEND_DIR" && go build -o "$WORKERS_BIN" ./cmd/mock-workers) \
        || die "go build ./cmd/mock-workers failed"

    # 2. Fresh disposable database.
    DB_NAME="sthira_demo_$(date -u +%Y%m%dT%H%M%SZ)_$$"
    [ "${#DB_NAME}" -le 63 ] || die "generated database name is too long: $DB_NAME"
    case "$DB_NAME" in
        [a-zA-Z_]* ) ;;
        * ) die "generated database name is not a valid identifier: $DB_NAME" ;;
    esac
    case "$DB_NAME" in
        *[!a-zA-Z0-9_]* ) die "generated database name is not a valid identifier: $DB_NAME" ;;
    esac
    local demods
    demods="$(db_dsn "$DB_NAME")"

    printf '>> creating disposable database %s\n' "$DB_NAME"
    psql "$ADMIN_DSN" -v ON_ERROR_STOP=1 -q -c "CREATE DATABASE \"$DB_NAME\";" \
        || die "could not create database $DB_NAME"
    DB_CREATED=1
    STATE_ACTIVE=1
    STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    trap cleanup_on_failure EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    write_state

    # 3. Migrations, sorted, stop on the first failure and name the file.
    printf '>> applying migrations from %s/migrations/00*.sql\n' "$BACKEND_DIR"
    shopt -s nullglob
    local migs=( "$BACKEND_DIR"/migrations/00*.sql )
    shopt -u nullglob
    [ ${#migs[@]} -gt 0 ] || die "no 00*.sql migrations found in $BACKEND_DIR/migrations"
    local f
    for f in "${migs[@]}"; do
        printf '   %s\n' "$(basename "$f")"
        psql "$demods" -v ON_ERROR_STOP=1 -q -f "$f" >/dev/null \
            || die "migration failed: $f (psql output above is the reason)"
    done

    # 4a. Workers (mock unless --real-workers).
    require_free_port "mock workers" "$WORKER_PORT"
    require_free_port "sthira-exercise" "$BACKEND_PORT"
    require_free_port "vite UI" "$UI_PORT"

    if [ "$REAL_WORKERS" -eq 0 ]; then
        printf '>> starting mock workers on 127.0.0.1:%s (scenario %s)\n' "$WORKER_PORT" "$SCENARIO"
        "$WORKERS_BIN" -port "$WORKER_PORT" -scenario "$SCENARIO" >"$WORKERS_LOG" 2>&1 &
        WORKER_PID=$!
        write_state
        local worker_url="" w=0
        while [ "$w" -lt 100 ]; do
            if grep -q '^WORKER_URL=' "$WORKERS_LOG" 2>/dev/null; then
                worker_url="$(sed -n 's/^WORKER_URL=//p' "$WORKERS_LOG" | tail -n 1)"
                break
            fi
            if ! kill -0 "$WORKER_PID" 2>/dev/null; then
                die "mock workers exited during startup:$(printf '\n')$(cat "$WORKERS_LOG")"
            fi
            w=$((w + 1))
            sleep 0.1
        done
        [ -n "$worker_url" ] || die "mock workers did not print WORKER_URL= within 10s:$(printf '\n')$(cat "$WORKERS_LOG")"
        [ "$worker_url" = "http://127.0.0.1:$WORKER_PORT" ] \
            || die "mock workers reported $worker_url but port $WORKER_PORT was expected"
        WORKER_MODE="mock"
        write_state

        # One mock process serves /health, /transcribe, /v1/chat/completions and
        # /synthesize, so all three STHIRA_*_URL values are the same origin.
        STHIRA_ADDR="127.0.0.1:$BACKEND_PORT" \
        STHIRA_DATABASE_DSN="$demods" \
        STHIRA_EXERCISE_SEED=1 \
        STHIRA_INSTANCE_ID="sthira-demo-${DB_NAME}" \
        STHIRA_ASR_URL="$worker_url" \
        STHIRA_MIDDLE_URL="$worker_url" \
        STHIRA_TTS_URL="$worker_url" \
        STHIRA_ASR_TOKEN="" \
        STHIRA_MIDDLE_TOKEN="" \
        STHIRA_TTS_TOKEN="" \
            "$EXERCISE_BIN" >"$EXERCISE_LOG" 2>&1 &
    else
        printf '>> using real workers from the environment (no worker process, no token is read or stored by this script)\n'
        WORKER_MODE="real"
        # Values are assigned into the child environment; the token is never a
        # command-line argument, so it cannot appear in `ps` or in this log.
        STHIRA_ADDR="127.0.0.1:$BACKEND_PORT" \
        STHIRA_DATABASE_DSN="$demods" \
        STHIRA_EXERCISE_SEED=1 \
        STHIRA_INSTANCE_ID="sthira-demo-${DB_NAME}" \
        STHIRA_ASR_URL="$asr" \
        STHIRA_MIDDLE_URL="$mid" \
        STHIRA_TTS_URL="$tts" \
        STHIRA_ASR_TOKEN="${STHIRA_ASR_TOKEN:-}" \
        STHIRA_MIDDLE_TOKEN="${STHIRA_MIDDLE_TOKEN:-}" \
        STHIRA_TTS_TOKEN="${STHIRA_TTS_TOKEN:-}" \
            "$EXERCISE_BIN" >"$EXERCISE_LOG" 2>&1 &
    fi
    EXERCISE_PID=$!
    write_state
    printf '>> exercise pid %s, waiting for /health/ready\n' "$EXERCISE_PID"

    # 4b. Vite (cwd frontend/v2; package.json already pins --host 127.0.0.1).
    (cd "$FRONTEND_DIR" \
        && export VITE_BACKEND_URL="http://127.0.0.1:$BACKEND_PORT" VITE_PORT="$UI_PORT" \
        && exec npm run dev) >"$UI_LOG" 2>&1 &
    UI_PID=$!
    write_state
    printf '>> vite pid %s (npm run dev, cwd %s)\n' "$UI_PID" "$FRONTEND_DIR"

    # 5. Bounded readiness wait with the reason on failure.
    local base="http://127.0.0.1:$BACKEND_PORT" ready="" n=0
    while [ "$n" -lt "$READY_TIMEOUT_SECS" ]; do
        if ! kill -0 "$EXERCISE_PID" 2>/dev/null; then
            die "the exercise process (pid $EXERCISE_PID) exited during startup. Tail of $EXERCISE_LOG:$(printf '\n')$(tail -n 20 "$EXERCISE_LOG")"
        fi
        ready="$(ready_of "$base")"
        [ "$ready" = "HTTP 200 READY/models=READY" ] && break
        n=$((n + 1))
        sleep 1
    done
    if [ "$ready" != "HTTP 200 READY/models=READY" ]; then
        case "$ready" in
            UNREACHABLE) why="no HTTP answer from $base/health/ready" ;;
            UNPARSEABLE) why="$base/health/ready did not return a readable report" ;;
            "")          why="readiness never answered" ;;
            *)           why="readiness reported '$ready' (need HTTP 200 with data.status=READY and subsystems.models.status=READY)" ;;
        esac
        die "not ready after ${READY_TIMEOUT_SECS}s: $why. Tail of $EXERCISE_LOG:$(printf '\n')$(tail -n 20 "$EXERCISE_LOG")"
    fi

    n=0
    while [ "$n" -lt "$READY_TIMEOUT_SECS" ]; do
        tcp_open "$UI_PORT" && break
        if ! kill -0 "$UI_PID" 2>/dev/null; then
            die "the Vite process (pid $UI_PID) exited during startup. Tail of $UI_LOG:$(printf '\n')$(tail -n 20 "$UI_LOG")"
        fi
        n=$((n + 1))
        sleep 1
    done
    tcp_open "$UI_PORT" || die "Vite is not accepting connections on 127.0.0.1:$UI_PORT after ${READY_TIMEOUT_SECS}s. Tail of $UI_LOG:$(printf '\n')$(tail -n 20 "$UI_LOG")"

    trap - EXIT INT TERM
    STATE_ACTIVE=1
    write_state

    # 6. Where to look.
    printf '\nDemo is up.\n'
    printf '  UI (Vite)        http://127.0.0.1:%s\n' "$UI_PORT"
    printf '  Exercise API     http://127.0.0.1:%s   (readiness %s)\n' "$BACKEND_PORT" "$ready"
    if [ "$REAL_WORKERS" -eq 0 ]; then
        printf '  Mock workers     http://127.0.0.1:%s   (scenario %s)\n' "$WORKER_PORT" "$SCENARIO"
    else
        printf '  Workers          external (--real-workers, no pid)\n'
    fi
    printf '  Database         %s (%s)\n' "$DB_NAME" "$(db_state)"
    printf '  State file       %s\n' "$STATE_FILE"
    printf '  Logs             %s | %s | %s\n' "$EXERCISE_LOG" "$WORKERS_LOG" "$UI_LOG"
    printf '  Stop with        %s stop\n' "$0"
}

# ------------------------------------------------------------------- cmd: stop
stop_one() { # <label> <pid> <kind> <marker> [argv-marker]
    local label="$1" pid="$2" kind="$3" marker="$4" argv_marker="${5:-}"
    if [ -z "$pid" ]; then
        printf '  %-13s no pid recorded\n' "$label"
        return 0
    fi
    if pid_is_ours "$kind" "$pid" "$marker" "$argv_marker"; then
        kill_tree "$pid"
        printf '  %-13s stopped (pid %s)\n' "$label" "$pid"
    elif kill -0 "$pid" 2>/dev/null; then
        printf '  %-13s pid %s is running but is NOT this run (command line/cwd mismatch) - not killed\n' "$label" "$pid"
    else
        printf '  %-13s already stopped (pid %s)\n' "$label" "$pid"
    fi
}

stop_run() {
    require_tools psql
    stop_one "exercise" "$EXERCISE_PID" bin "$EXERCISE_BIN"
    stop_one "vite" "$UI_PID" cwd "$FRONTEND_DIR_REAL" "run dev"
    if [ "$WORKER_MODE" = "real" ]; then
        printf '  %-13s external (--real-workers), nothing to kill\n' "workers"
    else
        stop_one "mock-workers" "$WORKER_PID" bin "$WORKERS_BIN"
    fi
    # Give the exercise a moment to release its connections before DROP.
    if [ "$DB_CREATED" = "1" ] && [ -n "$DB_NAME" ]; then
        local n=0
        while [ "$n" -lt 3 ]; do
            if psql "$ADMIN_DSN" -v ON_ERROR_STOP=1 -q -c "DROP DATABASE IF EXISTS \"$DB_NAME\";" >/dev/null 2>&1; then
                printf '  %-13s dropped %s\n' "database" "$DB_NAME"
                DB_CREATED=0
                break
            fi
            n=$((n + 1))
            sleep 1
        done
        if [ "$DB_CREATED" = "1" ]; then
            printf '  %-13s FAILED to drop %s\n' "database" "$DB_NAME" >&2
            psql "$ADMIN_DSN" -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS \"$DB_NAME\";" >&2 || true
            return 1
        fi
    else
        printf '  %-13s no database created by this run\n' "database"
    fi
    STATE_ACTIVE=0
    STOPPED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    write_state
    return 0
}

cmd_stop() {
    require_tools psql
    if ! read_state; then
        printf 'Nothing to stop: no state file at %s\n' "$STATE_FILE"
        return 0
    fi
    if [ "$STATE_ACTIVE" -ne 1 ]; then
        printf 'Nothing to stop: the run recorded in %s is already stopped%s\n' \
            "$STATE_FILE" "${STOPPED_AT:+ (stopped $STOPPED_AT)}"
        return 0
    fi
    printf 'Stopping the demo run started %s (database %s).\n' "$STARTED_AT" "$DB_NAME"
    stop_run || return 1
    printf 'Stopped. State file kept at %s (STATE_ACTIVE=0).\n' "$STATE_FILE"
    return 0
}

# ----------------------------------------------------------------- cmd: status
component_line() { # <label> <state> <port> <url> <note>
    printf '  %-13s %-10s port %-6s %-27s %s\n' "$1" "$2" "${3:--}" "${4:--}" "${5:--}"
}

cmd_status() {
    if read_state; then
        if [ "$STATE_ACTIVE" -eq 1 ]; then
            printf 'run: ACTIVE, started %s\n' "$STARTED_AT"
        else
            printf 'run: stopped%s (state %s)\n' "${STOPPED_AT:+ $STOPPED_AT}" "$STATE_FILE"
        fi
    else
        printf 'run: none (no state file at %s)\n' "$STATE_FILE"
        DB_NAME=""; DB_CREATED=0
    fi

    local base="http://127.0.0.1:$BACKEND_PORT" ui="http://127.0.0.1:$UI_PORT" wk="http://127.0.0.1:$WORKER_PORT"

    if [ -n "$WORKER_PID" ] && pid_is_ours bin "$WORKER_PID" "$WORKERS_BIN"; then
        component_line "mock-workers" "running" "$WORKER_PORT" "$wk" "pid $WORKER_PID, scenario $SCENARIO"
    elif [ "$WORKER_MODE" = "real" ]; then
        component_line "workers" "external" "-" "-" "--real-workers, no local process"
    elif [ -n "$WORKER_PID" ]; then
        component_line "mock-workers" "not-running" "$WORKER_PORT" "$wk" "pid $WORKER_PID not alive/not ours"
    else
        component_line "mock-workers" "not-running" "$WORKER_PORT" "$wk" "never started"
    fi

    if [ -n "$EXERCISE_PID" ] && pid_is_ours bin "$EXERCISE_PID" "$EXERCISE_BIN"; then
        component_line "exercise" "running" "$BACKEND_PORT" "$base" "pid $EXERCISE_PID"
    else
        component_line "exercise" "not-running" "$BACKEND_PORT" "$base" "pid ${EXERCISE_PID:-none} not alive/not ours"
    fi

    if [ -n "$UI_PID" ] && pid_is_ours cwd "$UI_PID" "$FRONTEND_DIR_REAL"; then
        component_line "vite UI" "running" "$UI_PORT" "$ui" "pid $UI_PID"
    else
        component_line "vite UI" "not-running" "$UI_PORT" "$ui" "pid ${UI_PID:-none} not alive/not ours"
    fi

    printf '  %-13s %-10s %s\n' "database" "$(db_state)" "${DB_NAME:-none recorded}"

    if [ -n "$EXERCISE_PID" ] && pid_is_ours bin "$EXERCISE_PID" "$EXERCISE_BIN"; then
        printf '  %-13s %s\n' "ready" "$(ready_of "$base")"
    else
        printf '  %-13s n/a (no live exercise process)\n' "ready"
    fi
}

# ------------------------------------------------------------------------ main
main() {
    local cmd="${1:-}"
    case "$cmd" in
        start|stop|status) shift ;;
        ""|-h|--help) usage; [ -n "$cmd" ] || exit 2; return 0 ;;
        *) usage; die "unknown subcommand: $cmd" ;;
    esac
    while [ $# -gt 0 ]; do
        case "$1" in
            --real-workers) REAL_WORKERS=1 ;;
            *) usage; die "unknown argument: $1" ;;
        esac
        shift
    done
    case "$cmd" in
        start) cmd_start ;;
        stop) cmd_stop ;;
        status) cmd_status ;;
    esac
}

main "$@"
