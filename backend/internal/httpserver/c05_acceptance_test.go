package httpserver_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"sthira/backend/internal/httpserver"
	"sthira/backend/internal/store"
)

// openAdminDB opens a connection to the PostgreSQL test cluster using STHIRA_TEST_ADMIN_DSN.
func openAdminDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("STHIRA_TEST_ADMIN_DSN")
	if dsn == "" {
		dsn = "postgres://localhost:5432/postgres?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open admin db: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping: PostgreSQL admin db unreachable at %s: %v", dsn, err)
	}
	return db, dsn
}

// createOwnedDB creates a temporary disposable database and returns its DSN and cleanup function.
func createOwnedDB(t *testing.T, prefix string) (string, *store.Store, func()) {
	t.Helper()
	adminDB, adminDSN := openAdminDB(t)
	defer adminDB.Close()

	dbName := fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano()%1000000, os.Getpid())
	if _, err := adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)); err != nil {
		t.Fatalf("failed to create database %s: %v", dbName, err)
	}

	childDSN := strings.Replace(adminDSN, "/postgres?", "/"+dbName+"?", 1)
	if !strings.Contains(adminDSN, "/postgres?") {
		// DSN might end with /postgres
		if strings.HasSuffix(adminDSN, "/postgres") {
			childDSN = strings.TrimSuffix(adminDSN, "/postgres") + "/" + dbName
		}
	}

	st, err := store.Open(childDSN)
	if err != nil {
		_, _ = adminDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))
		t.Fatalf("failed to open store for %s: %v", dbName, err)
	}

	cleanup := func() {
		_ = st.Close()
		adm, _ := sql.Open("pgx", adminDSN)
		if adm != nil {
			_, _ = adm.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))
			_ = adm.Close()
		}
	}

	return childDSN, st, cleanup
}

// applyMigrations applies all backend SQL migrations to the store.
func applyMigrations(t *testing.T, st *store.Store) {
	t.Helper()
	root := repoRoot(t)
	migFiles, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil || len(migFiles) == 0 {
		t.Fatalf("failed to find migrations under %s/migrations: %v", root, err)
	}

	for _, f := range migFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", f, err)
		}
		if _, err := st.DB().Exec(string(content)); err != nil {
			t.Fatalf("failed to execute migration %s: %v", filepath.Base(f), err)
		}
	}
}

// TestC05_SentinelDatabasePreserved ensures that a pre-existing database is never
// overwritten or dropped by the rehearsal runner unless STHIRA_DEMO_REUSE=1 is set.
func TestC05_SentinelDatabasePreserved(t *testing.T) {
	adminDB, adminDSN := openAdminDB(t)
	defer adminDB.Close()

	sentinelDB := fmt.Sprintf("sthira_sentinel_%d", time.Now().UnixNano()%1000000)
	if _, err := adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, sentinelDB)); err != nil {
		t.Fatalf("failed to create sentinel db: %v", err)
	}
	defer func() {
		_, _ = adminDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, sentinelDB))
	}()

	root := repoRoot(t)
	scriptPath := filepath.Join(root, "..", "scripts", "run_demo_rehearsal.sh")

	// Run without STHIRA_DEMO_REUSE=1: must exit with code 4 and refuse to drop
	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("STHIRA_TEST_ADMIN_DSN=%s", adminDSN),
		fmt.Sprintf("STHIRA_DEMO_DB=%s", sentinelDB),
		"STHIRA_DEMO_REUSE=0",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected script to fail with exit code 4 on existing DB, but it succeeded:\n%s", string(out))
	}
	if !strings.Contains(string(out), "Refusing to DROP or overwrite") {
		t.Fatalf("expected output to mention refusal to drop existing DB, got:\n%s", string(out))
	}

	// Verify the sentinel DB STILL exists
	var exists int
	err = adminDB.QueryRow(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname='%s'", sentinelDB)).Scan(&exists)
	if err != nil || exists != 1 {
		t.Fatalf("sentinel database was unexpectedly dropped or corrupted: %v", err)
	}
}

// TestC05_SchemaDiagnostics_LowCurrentMissing proves that readiness diagnostics accurately
// reflect schema status:
//   - Current schema (rev 10): HTTP 200 with migrations READY
//   - Low schema (rev < 10): HTTP 503 with migrations SCHEMA_MISMATCH
//   - Missing schema_migrations table: HTTP 503 with migrations UNAVAILABLE
func TestC05_SchemaDiagnostics_LowCurrentMissing(t *testing.T) {
	_, st, cleanup := createOwnedDB(t, "sthira_c05_diag")
	defer cleanup()

	applyMigrations(t, st)

	prober := store.NewReadinessProber(st.DB(), store.SchemaRevision)
	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithStore(st),
		httpserver.WithProber(prober),
	)

	// 1. Current schema (Revision 10) -> READY
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on current schema revision 10, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	report := srv.CheckReadiness(context.Background())
	if report.Subsystems["migrations"].Status != httpserver.StatusReady {
		t.Fatalf("expected migrations READY, got %+v", report.Subsystems["migrations"])
	}
	if report.Subsystems["database"].Status != httpserver.StatusReady {
		t.Fatalf("expected database READY, got %+v", report.Subsystems["database"])
	}

	// 2. Low schema (Simulate schema revision at 8) -> SCHEMA_MISMATCH (HTTP 503)
	if _, err := st.DB().Exec("DELETE FROM schema_migrations WHERE revision > 8"); err != nil {
		t.Fatalf("failed to delete revisions above 8: %v", err)
	}
	recLow := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recLow, req)
	if recLow.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable on low schema revision, got %d", recLow.Code)
	}
	reportLow := srv.CheckReadiness(context.Background())
	if reportLow.Status != "NOT_READY" {
		t.Fatalf("expected global NOT_READY on low revision, got %q", reportLow.Status)
	}
	if reportLow.Subsystems["migrations"].Status != httpserver.StatusMismatch {
		t.Fatalf("expected migrations SCHEMA_MISMATCH on rev 8, got %+v", reportLow.Subsystems["migrations"])
	}

	// 3. Missing schema_migrations table -> UNAVAILABLE (HTTP 503)
	if _, err := st.DB().Exec("DROP TABLE schema_migrations CASCADE"); err != nil {
		t.Fatalf("failed to drop schema_migrations table: %v", err)
	}
	recMissing := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recMissing, req)
	if recMissing.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable on missing migrations table, got %d", recMissing.Code)
	}
	reportMissing := srv.CheckReadiness(context.Background())
	if reportMissing.Status != "NOT_READY" {
		t.Fatalf("expected global NOT_READY on missing table, got %q", reportMissing.Status)
	}
	if reportMissing.Subsystems["migrations"].Status != httpserver.StatusUnavailable {
		t.Fatalf("expected migrations UNAVAILABLE on missing table, got %+v", reportMissing.Subsystems["migrations"])
	}
}

// TestC05_SubsystemsDistinct verifies that API DB readiness is distinguished from
// model readiness, source activation, and unavailable operator IdP.
func TestC05_SubsystemsDistinct(t *testing.T) {
	_, st, cleanup := createOwnedDB(t, "sthira_c05_dist")
	defer cleanup()

	applyMigrations(t, st)

	prober := store.NewReadinessProber(st.DB(), store.SchemaRevision)
	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithStore(st),
		httpserver.WithProber(prober),
		// IdP and voiceProcess unconfigured
	)

	report := srv.CheckReadiness(context.Background())

	// Database is connected
	if report.Subsystems["database"].Status != httpserver.StatusReady {
		t.Errorf("expected database READY, got %+v", report.Subsystems["database"])
	}
	// Migrations are current
	if report.Subsystems["migrations"].Status != httpserver.StatusReady {
		t.Errorf("expected migrations READY, got %+v", report.Subsystems["migrations"])
	}
	// Source activation reports no operational sources activated (not confusing DB connection with approved data)
	srcSub := report.Subsystems["source_activation"]
	if srcSub.Status != httpserver.StatusUnavailable {
		t.Errorf("expected source_activation UNAVAILABLE before activation, got %+v", srcSub)
	}
	// IdP is disabled (fails closed without IdP)
	idpSub := report.Subsystems["idp"]
	if idpSub.Status != httpserver.StatusDisabled {
		t.Errorf("expected idp DISABLED, got %+v", idpSub)
	}
	// Voice models are disabled
	modelsSub := report.Subsystems["models"]
	if modelsSub.Status != httpserver.StatusDisabled {
		t.Errorf("expected models DISABLED, got %+v", modelsSub)
	}
}

// TestC05_FullIntegratedRehearsalScript runs scripts/run_demo_rehearsal.sh and verifies
// that all 7 journeys execute, PLUMBING_ONLY passes, REAL_INFERENCE is truthfully disclosed,
// and the temporary database is cleaned up.
func TestC05_FullIntegratedRehearsalScript(t *testing.T) {
	adminDB, adminDSN := openAdminDB(t)
	defer adminDB.Close()

	root := repoRoot(t)
	scriptPath := filepath.Join(root, "..", "scripts", "run_demo_rehearsal.sh")

	testDBName := fmt.Sprintf("sthira_c05_test_%d", time.Now().UnixNano()%1000000)

	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("STHIRA_TEST_ADMIN_DSN=%s", adminDSN),
		fmt.Sprintf("STHIRA_DEMO_DB=%s", testDBName),
	)

	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	if err != nil {
		t.Fatalf("run_demo_rehearsal.sh failed with %v:\n%s", err, outputStr)
	}

	// Verify all 7 journeys passed
	for i := 1; i <= 7; i++ {
		journeyMarker := fmt.Sprintf("Journey %d", i)
		if !strings.Contains(outputStr, journeyMarker) {
			t.Errorf("expected output to contain %s", journeyMarker)
		}
	}

	// Verify honest status disclosure
	if !strings.Contains(outputStr, "PLUMBING_ONLY:           PASS") {
		t.Errorf("expected output to contain PLUMBING_ONLY: PASS")
	}
	if !strings.Contains(outputStr, "REAL_INFERENCE:          NOT_RUN (BLOCKED_HARDWARE") {
		t.Errorf("expected output to truthfully contain REAL_INFERENCE: NOT_RUN (BLOCKED_HARDWARE)")
	}

	// Verify temporary database was cleanly removed
	var exists int
	_ = adminDB.QueryRow(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname='%s'", testDBName)).Scan(&exists)
	if exists == 1 {
		t.Errorf("temporary database %s was not dropped during cleanup", testDBName)
		_, _ = adminDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, testDBName))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "migrations")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find backend module root from %s", dir)
	return ""
}
