package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverMigrationsOrdering(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0001_p3.sql":       "BEGIN;\nCOMMIT;\n",
		"0003_skip_gap.sql": "BEGIN;\nCOMMIT;\n",
		"0002_p4.sql":       "BEGIN;\nCOMMIT;\n",
		"0004_p5.sql":       "BEGIN;\nCOMMIT;\n",
		"0005_p6.sql":       "BEGIN;\nCOMMIT;\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Gap should be rejected (0002 -> 0003 is fine, but we put 0001,0003,0002,0004,0005 → once sorted it's 1,2,3,4,5 contiguous)
	// To test the gap rule, leave 0002 out.
	if err := os.Remove(filepath.Join(dir, "0002_p4.sql")); err != nil {
		t.Fatalf("remove 0002: %v", err)
	}

	_, err := discoverMigrations(dir)
	if err == nil {
		t.Fatalf("expected gap error, got nil")
	}

	// With the gap fixed it should accept
	if err := os.WriteFile(filepath.Join(dir, "0002_p4.sql"), []byte("BEGIN;\nCOMMIT;\n"), 0o600); err != nil {
		t.Fatalf("rewrite 0002: %v", err)
	}
	got, err := discoverMigrations(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("want 5 migrations, got %d", len(got))
	}
	for i := range got {
		if got[i].revision != i+1 {
			t.Fatalf("revision[%d]=%d, want %d", i, got[i].revision, i+1)
		}
	}
}

func TestDiscoverMigrationsDuplicates(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"0001_p3.sql":      "BEGIN;\nCOMMIT;\n",
		"0002_p4_a.sql":    "BEGIN;\nCOMMIT;\n",
		"0002_p4_b.sql":    "BEGIN;\nCOMMIT;\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	_, err := discoverMigrations(dir)
	if err == nil {
		t.Fatalf("expected duplicate revision error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate migration revision 2") {
		t.Fatalf("expected duplicate error message, got: %v", err)
	}
}

func TestSplitRevision(t *testing.T) {
	cases := map[string]struct {
		rev  int
		rest string
		ok   bool
	}{
		"0007_p5_publication_lifecycle.sql": {7, "p5_publication_lifecycle.sql", true},
		"0001_p3_foundation.sql":            {1, "p3_foundation.sql", true},
		"0000_invalid_really.sql":           {0, "invalid_really.sql", true},
		"README.md":                         {0, "", false},
		"no_prefix.sql":                     {0, "", false},
		"abcd_no_underscore.sql":            {0, "", false},
	}
	for name, want := range cases {
		rev, rest, ok := splitRevision(name)
		if rev != want.rev || rest != want.rest || ok != want.ok {
			t.Errorf("splitRevision(%q) = (%d,%q,%v); want (%d,%q,%v)",
				name, rev, rest, ok, want.rev, want.rest, want.ok)
		}
	}
}

func TestRedactedDSN(t *testing.T) {
	cases := []struct {
		in   string
		out  string
		hasP bool
	}{
		{"postgres://user:secret@host:5432/db", "postgres://user:REDACTED@host:5432/db", true},
		// No password component: the user block is present but the password
		// is absent, so redactedDSN must report hasP=false (cannot infer a
		// secret was ever supplied).
		{"postgres://user@host:5432/db", "postgres://user@host:5432/db", false},
		{"not-a-url", "not-a-url", false},
	}
	for _, c := range cases {
		got, hp := redactedDSN(c.in)
		if got != c.out || hp != c.hasP {
			t.Errorf("redactedDSN(%q) = (%q,%v); want (%q,%v)", c.in, got, hp, c.out, c.hasP)
		}
	}
}

func TestMergePGEnv(t *testing.T) {
	baseEnv := []string{
		"PATH=/usr/bin",
		"HOME=/root",
		"PGHOST=oldhost",
		"PGUSER=olduser",
	}
	overrides := []string{
		"PGHOST=newhost",
		"PGPORT=5432",
		"PGUSER=newuser",
		"PGPASSWORD=secret",
		"IGNORED_KEY=value",
	}

	merged := mergePGEnv(baseEnv, overrides)
	envMap := make(map[string]string)
	for _, entry := range merged {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["PGHOST"] != "newhost" {
		t.Errorf("PGHOST = %q, want newhost", envMap["PGHOST"])
	}
	if envMap["PGPORT"] != "5432" {
		t.Errorf("PGPORT = %q, want 5432", envMap["PGPORT"])
	}
	if envMap["PGUSER"] != "newuser" {
		t.Errorf("PGUSER = %q, want newuser", envMap["PGUSER"])
	}
	if envMap["PGPASSWORD"] != "secret" {
		t.Errorf("PGPASSWORD = %q, want secret", envMap["PGPASSWORD"])
	}
	if envMap["PATH"] != "/usr/bin" {
		t.Errorf("PATH = %q, want /usr/bin", envMap["PATH"])
	}
	if _, ok := envMap["IGNORED_KEY"]; ok {
		t.Errorf("IGNORED_KEY should have been excluded")
	}
}

func TestNewPsqlRunner(t *testing.T) {
	dsn := "postgres://myuser:mypass@db.internal:5433/mydb?sslmode=disable&application_name=migrator"
	runner, err := newPsqlRunner("psql", dsn)
	if err != nil {
		t.Fatalf("newPsqlRunner error: %v", err)
	}

	envMap := make(map[string]string)
	for _, entry := range runner.pgEnv {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["PGHOST"] != "db.internal" {
		t.Errorf("PGHOST = %q, want db.internal", envMap["PGHOST"])
	}
	if envMap["PGPORT"] != "5433" {
		t.Errorf("PGPORT = %q, want 5433", envMap["PGPORT"])
	}
	if envMap["PGUSER"] != "myuser" {
		t.Errorf("PGUSER = %q, want myuser", envMap["PGUSER"])
	}
	if envMap["PGPASSWORD"] != "mypass" {
		t.Errorf("PGPASSWORD = %q, want mypass", envMap["PGPASSWORD"])
	}
	if envMap["PGDATABASE"] != "mydb" {
		t.Errorf("PGDATABASE = %q, want mydb", envMap["PGDATABASE"])
	}
	if envMap["PGSSLMODE"] != "disable" {
		t.Errorf("PGSSLMODE = %q, want disable", envMap["PGSSLMODE"])
	}
	if envMap["PGAPPNAME"] != "migrator" {
		t.Errorf("PGAPPNAME = %q, want migrator", envMap["PGAPPNAME"])
	}
}
