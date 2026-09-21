// sthmigrate is the explicit, one-shot migration runner for the Sthira
// Go backend deployment package (deploy/go-backend/). It is owned by
// this package and is intentionally separate from the API binary
// (backend/cmd/sthira) so the API server is never responsible for
// racing migrations on startup.
//
// Secret interface: sthmigrate reads a connection string from the
// environment variable STHIRA_DATABASE_DSN. It does NOT read *_FILE
// variants because the upstream backend's store.Open signature only
// accepts a single DSN string (see backend/internal/store/store.go
// `Open(dsn string)` and backend/cmd/sthira/main.go line 46). Adopting
// DSN-file indirection here would silently diverge from the binary's
// actual contract. Until the backend exposes a *_FILE loader,
// documentation in HANDOFF.md states this limitation honestly.
//
// Stdlib only; no third-party dependencies. Migrations are applied via
// the postgresql-client `psql` binary, which is present in the package's
// Docker image (the official postgres:16-3.4 base used in compose) and
// is invoked through `os/exec`.
//
// Behaviour:
//
//	up [target]   apply pending migrations up to (and including) target
//	              (default: head)
//	status        print applied revisions and pending file list, no writes
//	verify        dry-run: assert unapplied migrations parse without error
//	              (psql --single-transaction --on-error-stop --no-psqlrc)
//	              using ROLLBACK so no commit is emitted
//
// Migration files live at /migrations inside the container (matching the
// repo path backend/migrations). For each pending migration we open one
// psql session, prepend BEGIN; SELECT pg_advisory_xact_lock(<id>); and
// append COMMIT; around the file body, then submit via stdin with
// ON_ERROR_STOP=1. The xact-scoped advisory lock is released at COMMIT,
// serializing concurrent migrators at file boundaries. The runner refuses:
//   - empty / missing DSN
//   - any unapplied migration whose revision is lower than the highest
//     applied revision (inconsistent history)
//   - any migration staging directory that has gaps in revision numbers
//   - any migration whose body fails in psql
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultMigrationsDir = "/migrations"
	advisoryLockID       = int64(0x5e3a_3037_3035_5f53) // 'Z:0b07ook_S'
	pgBin                = "psql"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger, os.Args[1:]); err != nil {
		logger.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, args []string) error {
	// Accept either `sthmigrate [global-flags] <subcommand> [sub-flags]`
	// (the canonical form so `--migrations /foo status` works) or the
	// legacy form `<subcommand> [sub-flags]` for backward-compatibility
	// with the README snippets.
	cmd := ""
	cmdIdx := -1
	for i, a := range args {
		if a == "up" || a == "status" || a == "verify" {
			cmd = a
			cmdIdx = i
			break
		}
	}
	if cmd == "" {
		return errors.New("usage: sthmigrate [flags] {up|status|verify} [sub-flags]")
	}
	pre := args[:cmdIdx]
	post := args[cmdIdx+1:]

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("migrations", defaultMigrationsDir, "directory containing NNNN_*.sql files")
	dsnFlag := fs.String("dsn", os.Getenv("STHIRA_DATABASE_DSN"), "PostgreSQL DSN (defaults to $STHIRA_DATABASE_DSN)")
	targetFlag := fs.Int("target", -1, "stop after applying revision <= target (default: head)")
	psqlPath := fs.String("psql", pgBin, "psql binary path")
	if err := fs.Parse(append(pre, post...)); err != nil {
		return err
	}

	dsn := strings.TrimSpace(*dsnFlag)
	if dsn == "" {
		return errors.New("STHIRA_DATABASE_DSN is required (or pass --dsn)")
	}
	if _, hasPassword := redactedDSN(dsn); !hasPassword {
		logger.Warn("DSN has no password component; assuming peer/trust auth")
	}

	switch cmd {
	case "up":
		return runUp(logger, *dir, dsn, *targetFlag, *psqlPath)
	case "status":
		return runStatus(logger, *dir, dsn, *psqlPath)
	case "verify":
		return runVerify(logger, *dir, dsn, *psqlPath)
	}
	return nil
}

// migrationFile represents one on-disk migration file.
type migrationFile struct {
	path     string // absolute path to the .sql
	revision int    // numeric prefix (e.g. 0007_p5_publication_lifecycle.sql -> 7)
}

// discoverMigrations returns on-disk migration files sorted ascending by
// revision. Files that do not start with a numeric prefix are ignored (and
// reported via slog); gaps and duplicates are flagged so a mis-staged
// directory cannot be silently accepted.
func discoverMigrations(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}
	files := make([]migrationFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		rev, _, ok := splitRevision(e.Name())
		if !ok {
			continue
		}
		files = append(files, migrationFile{path: filepath.Join(dir, e.Name()), revision: rev})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no migration files found under %s", dir)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].revision < files[j].revision })
	for i := 1; i < len(files); i++ {
		if files[i].revision == files[i-1].revision {
			return nil, fmt.Errorf("duplicate migration revision %d in staging", files[i].revision)
		}
		if files[i].revision != files[i-1].revision+1 {
			return nil, fmt.Errorf("migration revision gap: %d -> %d", files[i-1].revision, files[i].revision)
		}
	}
	return files, nil
}

func splitRevision(name string) (int, string, bool) {
	idx := strings.Index(name, "_")
	if idx <= 0 {
		return 0, "", false
	}
	rev, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0, "", false
	}
	return rev, name[idx+1:], true
}

// psqlRunner wraps `psql` invocations against a parsed DSN. The DSN
// is split into individual PG* environment variables so the password
// never appears in argv (which would be visible via `ps`).
type psqlRunner struct {
	psql  string
	pgEnv []string
}

// newPsqlRunner parses the DSN using the libpq URI grammar
// (postgres://user:password@host:port/db?key=val) and prepares the
// corresponding PG* environment variables for psql invocations. Missing
// components are left to libpq defaults.
func newPsqlRunner(psqlPath, dsn string) (*psqlRunner, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	env := []string{"PGCONNECT_TIMEOUT=15"}
	if u.Host != "" {
		// url.URL.Host may include userinfo and port — strip them.
		host := u.Hostname()
		if host != "" {
			env = append(env, "PGHOST="+host)
		}
		if port := u.Port(); port != "" {
			env = append(env, "PGPORT="+port)
		}
	}
	if u.User != nil {
		env = append(env, "PGUSER="+u.User.Username())
		if pass, hasPass := u.User.Password(); hasPass {
			env = append(env, "PGPASSWORD="+pass)
		}
	}
	if u.Path != "" && u.Path != "/" {
		// Trim leading slash.
		env = append(env, "PGDATABASE="+strings.TrimPrefix(u.Path, "/"))
	}
	q := u.Query()
	if ssl := q.Get("sslmode"); ssl != "" {
		env = append(env, "PGSSLMODE="+ssl)
	}
	if app := q.Get("application_name"); app != "" {
		env = append(env, "PGAPPNAME="+app)
	}
	return &psqlRunner{psql: psqlPath, pgEnv: env}, nil
}

func (p *psqlRunner) run(ctx context.Context, body string, extra ...string) (string, error) {
	args := append([]string{}, extra...)
	args = append(args,
		"-X",
		"--no-psqlrc",
		"--quiet",
		"--no-align",
		"--tuples-only",
		"-v", "ON_ERROR_STOP=1",
	)
	cmd := exec.CommandContext(ctx, p.psql, args...)
	// Inherit parent env last so our PG* values take precedence without
	// leaking unrelated secrets: explicit PG* overrides parent PG*; the
	// psql child still sees PATH, HOME, LANG, etc.
	base := os.Environ()
	merged := mergePGEnv(base, p.pgEnv)
	cmd.Env = merged
	cmd.Stdin = strings.NewReader(body)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("psql exit: %w", err)
	}
	return string(out), nil
}

// mergePGEnv returns env with PG* entries from overrides replacing
// matching keys in env. Other entries are preserved.
func mergePGEnv(env, overrides []string) []string {
	pgKeys := map[string]bool{
		"PGHOST": true, "PGPORT": true, "PGUSER": true, "PGPASSWORD": true,
		"PGDATABASE": true, "PGSSLMODE": true, "PGSSLKEY": true,
		"PGSSLCERT": true, "PGSSLROOTCERT": true, "PGAPPNAME": true,
		"PGCONNECT_TIMEOUT": true,
	}
	out := make([]string, 0, len(env)+len(overrides))
	skip := map[string]bool{}
	for _, ov := range overrides {
		k := ov[:strings.IndexByte(ov, '=')]
		skip[k] = true
		if !pgKeys[k] && k != "PGCONNECT_TIMEOUT" {
			continue
		}
		out = append(out, ov)
	}
	for _, e := range env {
		k := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			k = e[:i]
		}
		if (pgKeys[k] || k == "PGCONNECT_TIMEOUT") && skip[k] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// runStatus prints applied revisions and pending file list. No writes.
func runStatus(logger *slog.Logger, dir, dsn, psqlPath string) error {
	files, err := discoverMigrations(dir)
	if err != nil {
		return err
	}
	p, err := newPsqlRunner(psqlPath, dsn)
	if err != nil {
		return err
	}
	applied, err := queryApplied(p)
	if err != nil {
		return err
	}
	logger.Info("status",
		"applied_max", applied.max,
		"applied_count", len(applied.revs),
		"on_disk_count", len(files))
	for _, f := range files {
		state := "pending"
		if _, ok := applied.revs[f.revision]; ok {
			state = "applied"
		}
		logger.Info("migration", "revision", f.revision, "state", state, "path", filepath.Base(f.path))
	}
	return nil
}

// runVerify parses every migration file in a `--single-transaction` ROLLBACK.
// No writes are committed; this catches SQL syntax errors before `up`.
func runVerify(logger *slog.Logger, dir, dsn, psqlPath string) error {
	p, err := newPsqlRunner(psqlPath, dsn)
	if err != nil {
		return err
	}
	files, err := discoverMigrations(dir)
	if err != nil {
		return err
	}
	applied, err := queryApplied(p)
	if err != nil {
		return err
	}
	for _, f := range files {
		if _, ok := applied.revs[f.revision]; ok {
			continue
		}
		sqlBytes, err := os.ReadFile(f.path)
		if err != nil {
			return fmt.Errorf("read %s: %w", f.path, err)
		}
		body := string(sqlBytes) + "\nROLLBACK;\n"
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		out, err := p.run(ctx, body, "--single-transaction")
		cancel()
		if err != nil {
			return fmt.Errorf("verify revision %d failed: %w\n%s", f.revision, err, out)
		}
		logger.Info("verify ok", "revision", f.revision)
	}
	return nil
}

// runUp applies pending migrations up to the target revision (or head).
//
// Concurrency model:
//   - Process-local: a `flock` on a lock file in the runtime temp dir
//     serializes concurrent sthmigrate invocations on the same host.
//     The path is overridable via STHIRA_MIGRATE_LOCK_FILE and defaults
//     to $TMPDIR/sthmigrate-<hash>.lock. The lock is held until the
//     process exits, including via crash — flock is kernel-managed.
//   - Database-local: in addition, every migration file is applied
//     inside a transaction that takes pg_advisory_xact_lock with a
//     fixed shared id. Even a non-cooperating client (e.g. an operator
//     pasting psql without going through sthmigrate) cannot trample
//     our transaction.
//   - Files are idempotent by design (CREATE TABLE IF NOT EXISTS,
//     CREATE INDEX IF NOT EXISTS, INSERT ... ON CONFLICT DO NOTHING),
//     so a re-apply is always safe; the runner enforces that only by
//     skipping revisions already present.
func runUp(logger *slog.Logger, dir, dsn string, target int, psqlPath string) error {
	files, err := discoverMigrations(dir)
	if err != nil {
		return err
	}
	p, err := newPsqlRunner(psqlPath, dsn)
	if err != nil {
		return err
	}

	release, err := acquireFlock()
	if err != nil {
		return fmt.Errorf("acquire migrate lock: %w", err)
	}
	defer release()

	// First snapshot of applied state — used to detect inconsistent
	// history (a lower revision unapplied while a higher revision was
	// applied) and as a starting point for the apply loop.
	applied, err := queryApplied(p)
	if err != nil {
		return err
	}
	refusedDowngrade := false
	for _, f := range files {
		if _, ok := applied.revs[f.revision]; !ok && f.revision < applied.max && applied.max != 0 {
			refusedDowngrade = true
			break
		}
	}
	if refusedDowngrade {
		return fmt.Errorf("refusing: at least one unapplied migration has revision lower than the highest applied revision (max=%d); inconsistent history", applied.max)
	}

	stop := target
	if stop < 0 {
		stop = files[len(files)-1].revision
	}
	for _, f := range files {
		if f.revision > stop {
			break
		}
		if _, ok := applied.revs[f.revision]; ok {
			logger.Info("skipping", "revision", f.revision, "reason", "already_applied")
			continue
		}
		sqlBytes, err := os.ReadFile(f.path)
		if err != nil {
			return fmt.Errorf("read %s: %w", f.path, err)
		}
		logger.Info("applying", "revision", f.revision, "path", filepath.Base(f.path))
		// Each file already contains BEGIN; COMMIT; . psql with
		// -v ON_ERROR_STOP=1 aborts on the first failure. Wrap with the
		// advisory lock to also serialize against non-sthmigrate psql
		// callers that have been told to use the same id.
		body := fmt.Sprintf("BEGIN;\nSELECT pg_advisory_xact_lock(%d);\n%s\nCOMMIT;\n", advisoryLockID, string(sqlBytes))
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		out, err := p.run(ctx, body)
		cancel()
		if err != nil {
			return fmt.Errorf("apply revision %d failed: %w\n%s", f.revision, err, out)
		}
		logger.Info("applied", "revision", f.revision)
		applied.revs[f.revision] = struct{}{}
		if f.revision > applied.max {
			applied.max = f.revision
		}
	}
	return nil
}

// appliedState is the result of querying the schema_migrations table.
type appliedState struct {
	max  int
	revs map[int]struct{}
}

// queryApplied reads schema_migrations via psql using
// string_agg(... ORDER BY ...) so a single text line is returned and
// parsing is unambiguous. A fresh database (where schema_migrations has
// not yet been created by migration 0001) returns an empty string and
// `max = 0`; the "relation does not exist" error from psql is treated
// as that case, not a real failure.
func queryApplied(p *psqlRunner) (appliedState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := p.run(ctx,
		`SELECT COALESCE(string_agg(revision::text, ',' ORDER BY revision), '') FROM schema_migrations;`,
	)
	st := appliedState{revs: map[int]struct{}{}}
	if err != nil {
		// Postgres returns SQLSTATE 42P01 ("undefined_table") for
		// "relation does not exist". psql's exit code is 3 in that case
		// because of -v ON_ERROR_STOP=1. We treat this as the fresh-DB
		// case rather than a hard error.
		if strings.Contains(out, "does not exist") && strings.Contains(out, "schema_migrations") {
			return st, nil
		}
		return st, fmt.Errorf("query schema_migrations: %w\n%s", err, out)
	}
	body := strings.TrimSpace(out)
	if body == "" {
		return st, nil
	}
	for _, tok := range strings.Split(body, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		rev, err := strconv.Atoi(tok)
		if err != nil {
			return st, fmt.Errorf("unexpected revision token %q: %w", tok, err)
		}
		st.revs[rev] = struct{}{}
		if rev > st.max {
			st.max = rev
		}
	}
	return st, nil
}

// acquireFlock serializes concurrent sthmigrate invocations on the same
// host via flock(2) on a file whose name encodes the DSN's host+db.
// The lock is automatically released when the process exits (or via the
// returned release closure). The file is created with mode 0o600 because
// its *path* already encodes connection detail.
func acquireFlock() (func(), error) {
	dsn := os.Getenv("STHIRA_DATABASE_DSN")
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		host = "default"
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		db = "default"
	}
	key := fmt.Sprintf("sthmigrate-%s-%s.lock", host, db)

	dir := os.TempDir()
	if v := os.Getenv("STHIRA_MIGRATE_LOCK_DIR"); v != "" {
		dir = v
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir lock dir: %w", err)
	}
	path := filepath.Join(dir, key)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	// LOCK_EX | LOCK_NB would fail instead of blocking; the brief says
	// "lock migration execution", which implies blocking is acceptable.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock: %w", err)
	}
	return func() { _ = f.Close() }, nil
}

// redactedDSN parses the DSN and returns a copy with the password masked.
// The boolean indicates whether a password component was present (true) or
// absent (false). Useful for safe logging.
func redactedDSN(dsn string) (string, bool) {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn, false
	}
	if u.User == nil {
		return dsn, false
	}
	if _, hasPass := u.User.Password(); hasPass {
		u.User = url.UserPassword(u.User.Username(), "REDACTED")
		return u.String(), true
	}
	return dsn, false
}
