package store

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSchemaRevisionMatchesMigrations locks the invariant
// `SchemaRevision == len(backend/migrations/[0-9]*.sql)` so the readiness
// prober cannot drift out of sync with the actual migration set. Bumping
// the constant without adding a migration (or vice versa) fails the build.
//
// Uses the repository root inferred from the test's working directory; the
// `go test` runner always sets the working directory to the package being
// tested, so we walk up until we find `go.mod`.
func TestSchemaRevisionMatchesMigrations(t *testing.T) {
	root := repoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "migrations", "[0-9]*.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no migrations found under %s/migrations", root)
	}
	highest := 0
	for _, m := range matches {
		base := filepath.Base(m)
		revStr := strings.SplitN(strings.TrimSuffix(base, filepath.Ext(base)), "_", 2)[0]
		rev, err := strconv.Atoi(revStr)
		if err != nil {
			t.Fatalf("parse migration %q: %v", base, err)
		}
		if rev > highest {
			highest = rev
		}
	}
	if SchemaRevision != highest {
		t.Fatalf("SchemaRevision = %d, want %d (highest migration revision under backend/migrations)", SchemaRevision, highest)
	}
}

// repoRoot walks up from the test's cwd to find the backend module root
// (the directory that contains both `migrations/` and `go.mod`). Used to
// locate migrations when the test binary runs from a package subdirectory.
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
	t.Fatalf("could not find backend module root (with migrations/) from %s", dir)
	return ""
}
