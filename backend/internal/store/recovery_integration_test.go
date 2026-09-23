//go:build recoverytest

// Recovery integration test: exercises a real PostgreSQL source -> pg_dump
// -> fresh target -> schema/state/audit invariants, gated on
// STHIRA_TEST_DSN + STHIRA_RUN_RECOVERY=1. Lives in this package so it
// can read migrations/ + the same fixtures the rest of P7-recovery
// exercises, without spinning up its own binary.
//
// Distinct from crash_process_test.go (which exercises crash-after-commit
// against a running server). This file proves: a fresh DB restored from
// pg_dump of the source carries the same schema_revision, the same
// idempotency replay state, the same audit-chain head, and the same
// reserved-capacity count. Process-level reconnect lives in
// scripts/recovery/run-backup-restore.sh.

package store_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// libpqEnvFromDSN exposes a libpq-formatted DSN as env vars so every
// `psql` / `pg_dump` / `pg_restore` invocation picks them up via the
// usual connection-string-keyword path. Returns the maintenance
// database name (always "postgres" for tests).
func libpqEnvFromDSN(dsn string) ([]string, string, error) {
	parts := map[string]string{}
	for _, kv := range strings.Fields(dsn) {
		if k, v, ok := strings.Cut(kv, "="); ok {
			parts[k] = v
		}
	}
	for _, k := range []string{"host", "port", "user"} {
		if parts[k] == "" {
			return nil, "", fmt.Errorf("STHIRA_TEST_DSN missing required %q= keyword", k)
		}
	}
	db := parts["dbname"]
	if db == "" {
		db = "postgres"
	}
	env := []string{}
	for k, v := range parts {
		if k == "password" || k == "dbname" {
			continue
		}
		env = append(env, "PG"+strings.ToUpper(k)+"="+v)
	}
	return env, db, nil
}

// runPsql runs a psql with the libpq env from STHIRA_TEST_DSN, naming
// the target database explicitly via -d.
func runPsql(t *testing.T, libpqEnv []string, db string, args ...string) ([]byte, error) {
	t.Helper()
	full := append([]string{"-d", db, "-v", "ON_ERROR_STOP=1"}, args...)
	cmd := exec.Command("psql", full...)
	cmd.Env = append(os.Environ(), libpqEnv...)
	return cmd.CombinedOutput()
}

func TestBackupRestoreAndReconnect(t *testing.T) {
	// Prefer the homebrew postgresql@18 bin directory when available
	// so pg_dump / pg_restore match the server version. Without this
	// PATH override the test fails on hosts where the default pg_dump
	// is older than the test database. We do not error out if @18 is
	// absent — the team's server may be running a different major.
	for _, p := range []string{
		"/opt/homebrew/opt/postgresql@18/bin",
		"/opt/homebrew/opt/postgresql@17/bin",
		"/opt/homebrew/opt/postgresql@16/bin",
	} {
		if _, err := os.Stat(p + "/pg_dump"); err == nil {
			os.Setenv("PATH", p+":"+os.Getenv("PATH"))
			t.Logf("using postgres bin dir: %s", p)
			break
		}
	}
	raw := os.Getenv("STHIRA_TEST_DSN")
	if raw == "" || os.Getenv("STHIRA_RUN_RECOVERY") != "1" {
		t.Skip("requires STHIRA_TEST_DSN (libpq form) and STHIRA_RUN_RECOVERY=1")
	}
	env, maint, err := libpqEnvFromDSN(raw)
	if err != nil {
		t.Fatal(err)
	}

	runID := fmt.Sprintf("r7store%d", time.Now().UnixNano())
	srcDB := "r7recover_" + runID + "_src"
	dstDB := "r7recover_" + runID + "_dst"
	for _, db := range []string{srcDB, dstDB} {
		if !strings.HasPrefix(db, "r7recover_") {
			t.Fatalf("refuse non-prefixed DB name %q", db)
		}
	}
	dumpPath := filepath.Join(t.TempDir(), "dump.sql")

	t.Cleanup(func() {
		for _, db := range []string{srcDB, dstDB} {
			// Even on t.Cleanup we refuse to drop anything non-prefixed.
			if !strings.HasPrefix(db, "r7recover_") {
				continue
			}
			_, _ = runPsql(t, env, maint, "-c", "DROP DATABASE IF EXISTS "+db)
		}
	})

	for _, cmd := range [][]string{
		{"CREATE DATABASE " + srcDB},
	} {
		if out, err := runPsql(t, env, maint, "-c", cmd[0]); err != nil {
			t.Fatalf("setup %v: %v\n%s", cmd, err, out)
		}
	}

	// Apply every migration file in order.
	migDir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", migDir, err)
	}
	var migFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migFiles = append(migFiles, entry.Name())
		}
	}
	sort.Strings(migFiles)
	for _, f := range migFiles {
		if out, err := runPsql(t, env, srcDB, "-q", "-f", filepath.Join(migDir, f)); err != nil {
			t.Fatalf("apply %s: %v\n%s", f, err, out)
		}
	}

	// Seed representative fixtures with R7- prefix. Use explicit
	// column lists so future migration edits that reorder columns
	// do not silently miscompile. Each %s substitution maps to one
	// arg in the call below in the same order it appears left-to-right.
	seed := fmt.Sprintf(`
BEGIN;
INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, expires_at)
  VALUES ('%[1]s-S','CITIZEN','JUR-R7','cred',now()+interval '1 hour');
INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, version, updated_at)
  VALUES ('%[1]s-F',current_date,5,2,1,now());
INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at)
  VALUES ('%[1]s-R','%[1]s-S','%[1]s-F',current_date,1,'RESERVED',1,now(),now());
INSERT INTO idempotency_keys (scope, operation, idem_key, payload_hash, result, state, expires_at)
  VALUES ('%[1]s-S','reservation.create','%[1]s-K',repeat('c',64),'{"reservation_id":"%[1]s-R"}','COMPLETED',now()+interval '1 hour');
INSERT INTO audit_events (event_id, occurred_at, actor_id, action, subject_id, outcome, to_state, prev_hash, event_hash)
  VALUES ('%[1]s-E', now(), '%[1]s-S','RESERVATION_CREATE','%[1]s-R','OK','RESERVED',repeat('0',64), repeat('1',64));
COMMIT;
`, runID)
	if out, err := runPsql(t, env, srcDB, "-c", seed); err != nil {
		t.Fatalf("seed: %v\n%s", err, out)
	}

	// Capture source invariants.
	srcRev := strings.TrimSpace(mustQuery(t, env, srcDB, "SELECT COALESCE(MAX(revision),0) FROM schema_migrations"))
	srcIDEM := strings.TrimSpace(mustQuery(t, env, srcDB, "SELECT state FROM idempotency_keys WHERE idem_key='"+runID+"-K'"))
	srcReserved := strings.TrimSpace(mustQuery(t, env, srcDB, "SELECT reserved FROM facility_inventory WHERE facility_id='"+runID+"-F'"))
	srcHead := strings.TrimSpace(mustQuery(t, env, srcDB, "SELECT event_hash FROM audit_events ORDER BY event_seq DESC LIMIT 1"))
	if srcRev != "9" {
		t.Fatalf("schema_revision = %q, want 9 (binary SchemaRevision constant)", srcRev)
	}

	// pg_dump -Fc -> SHA-256 -> restore into fresh DB.
	dumpOut, err := runPg(env, "pg_dump", "-Fc", "-f", dumpPath, srcDB)
	if err != nil {
		t.Fatalf("pg_dump: %v\n%s", err, dumpOut)
	}
	dumpBytes, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	sum := sha256.Sum256(dumpBytes)
	if out, err := runPsql(t, env, maint, "-c", "CREATE DATABASE "+dstDB); err != nil {
		t.Fatalf("create dst: %v\n%s", err, out)
	}
	if out, err := runPg(env, "pg_restore", "-d", dstDB, dumpPath); err != nil {
		t.Fatalf("pg_restore: %v\n%s", err, out)
	}

	dstRev := strings.TrimSpace(mustQuery(t, env, dstDB, "SELECT COALESCE(MAX(revision),0) FROM schema_migrations"))
	dstIDEM := strings.TrimSpace(mustQuery(t, env, dstDB, "SELECT state FROM idempotency_keys WHERE idem_key='"+runID+"-K'"))
	dstReserved := strings.TrimSpace(mustQuery(t, env, dstDB, "SELECT reserved FROM facility_inventory WHERE facility_id='"+runID+"-F'"))
	dstHead := strings.TrimSpace(mustQuery(t, env, dstDB, "SELECT event_hash FROM audit_events ORDER BY event_seq DESC LIMIT 1"))

	if dstRev != srcRev {
		t.Errorf("schema_revision: src=%s dst=%s", srcRev, dstRev)
	}
	if dstIDEM != srcIDEM {
		t.Errorf("idempotency_state: src=%s dst=%s", srcIDEM, dstIDEM)
	}
	if dstReserved != srcReserved {
		t.Errorf("reserved: src=%s dst=%s", srcReserved, dstReserved)
	}
	if dstHead != srcHead {
		t.Errorf("audit chain head: src=%s dst=%s", srcHead, dstHead)
	}

	// Minimal evidence log (no secrets, no PII).
	rep := map[string]any{
		"run_id":         runID,
		"schema_rev":     srcRev,
		"idem_state":     srcIDEM,
		"reserved":       srcReserved,
		"audit_head":     srcHead,
		"dump_sha256":    hex.EncodeToString(sum[:]),
		"dump_bytes":     len(dumpBytes),
		"wallclock_done": time.Now().UTC().Format(time.RFC3339Nano),
	}
	b, _ := json.MarshalIndent(rep, "", "  ")
	t.Logf("recovery evidence:\n%s", string(b))
}

func mustQuery(t *testing.T, env []string, db, sql string) string {
	t.Helper()
	out, err := runPsql(t, env, db, "-tAc", sql)
	if err != nil {
		t.Fatalf("query %q on %s: %v\n%s", sql, db, err, out)
	}
	return string(out)
}

// runPg invokes a CLI tool (pg_dump / pg_restore) with the libpq env
// from STHIRA_TEST_DSN.
func runPg(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}
