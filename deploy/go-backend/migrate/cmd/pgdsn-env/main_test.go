package main

import (
	"net/url"
	"strings"
	"testing"
)

func parseDSNToEnv(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if host := u.Hostname(); host != "" {
		b.WriteString("PGHOST=" + host + "\n")
	}
	if port := u.Port(); port != "" {
		b.WriteString("PGPORT=" + port + "\n")
	}
	if u.User != nil {
		b.WriteString("PGUSER=" + u.User.Username() + "\n")
		if pw, hasPw := u.User.Password(); hasPw {
			b.WriteString("PGPASSWORD=" + pw + "\n")
		}
	}
	if u.Path != "" && u.Path != "/" {
		b.WriteString("PGDATABASE=" + strings.TrimPrefix(u.Path, "/") + "\n")
	}
	if q := u.Query(); len(q) > 0 {
		if v := q.Get("sslmode"); v != "" {
			b.WriteString("PGSSLMODE=" + v + "\n")
		}
		if v := q.Get("application_name"); v != "" {
			b.WriteString("PGAPPNAME=" + v + "\n")
		}
	}
	b.WriteString("PGCONNECT_TIMEOUT=15\n")
	return b.String(), nil
}

func TestParseDSNToEnv(t *testing.T) {
	dsn := "postgres://admin:secret123@db.example.com:5433/production_db?sslmode=require&application_name=sthira"
	envStr, err := parseDSNToEnv(dsn)
	if err != nil {
		t.Fatalf("parseDSNToEnv error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(envStr), "\n")
	envMap := make(map[string]string)
	for _, l := range lines {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	expected := map[string]string{
		"PGHOST":            "db.example.com",
		"PGPORT":            "5433",
		"PGUSER":            "admin",
		"PGPASSWORD":        "secret123",
		"PGDATABASE":        "production_db",
		"PGSSLMODE":         "require",
		"PGAPPNAME":         "sthira",
		"PGCONNECT_TIMEOUT": "15",
	}

	for k, v := range expected {
		if envMap[k] != v {
			t.Errorf("key %s = %q, want %q", k, envMap[k], v)
		}
	}
}

func TestParseDSNMinimal(t *testing.T) {
	dsn := "postgres://localhost/testdb"
	envStr, err := parseDSNToEnv(dsn)
	if err != nil {
		t.Fatalf("parseDSNToEnv error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(envStr), "\n")
	envMap := make(map[string]string)
	for _, l := range lines {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["PGHOST"] != "localhost" {
		t.Errorf("PGHOST = %q, want localhost", envMap["PGHOST"])
	}
	if envMap["PGDATABASE"] != "testdb" {
		t.Errorf("PGDATABASE = %q, want testdb", envMap["PGDATABASE"])
	}
	if _, ok := envMap["PGPASSWORD"]; ok {
		t.Errorf("unexpected PGPASSWORD when none provided")
	}
	if envMap["PGCONNECT_TIMEOUT"] != "15" {
		t.Errorf("PGCONNECT_TIMEOUT = %q, want 15", envMap["PGCONNECT_TIMEOUT"])
	}
}
