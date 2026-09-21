// pgdsn-env prints libpq-compatible PG* environment variables on stdout,
// one KEY=val per line, parsed from a libpq URI given as the first CLI
// argument. Useful as input to `pg_dump --env-file` or `docker run
// --env-file`.
//
// This helper exists because libpq does not honour a single PGURI
// variable; it expects each connection parameter as a separate env var.
// Combining the deployment scripts' secret discipline (DSN only on the
// environment, not argv) with pg_dump's strict libpq expectations
// motivates this tiny, stdlib-only parser.
package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: pgdsn-env <postgres-uri>")
		os.Exit(2)
	}
	u, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgdsn-env: parse:", err)
		os.Exit(2)
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
	os.Stdout.WriteString(b.String())
}
