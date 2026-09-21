package main

import (
	"os"
	"path/filepath"
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
