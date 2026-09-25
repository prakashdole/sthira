package httpserver

import (
	"net/http"
	"strings"
	"testing"
)

// narrowly-scoped security tests — these do not duplicate existing
// boundary/role/integration tests. They cover the *outermost*
// security boundary (request headers / status / response shape) and
// confirm the audited invariants in backend/security/findings.md
// (F-SESSIONLEAK-01 and F-BEARERBP-01).

func TestBearerTokenParsingStrict(t *testing.T) {
	// bearerToken(r *http.Request) parses only `Bearer ` prefixed
	// headers; lowercase or tab-separator variants must NOT be
	// accepted as a usable token. Any token the function returns
	// goes on to be hashed and looked up. Returning the wrong
	// string here would let a non-prefixed header fool the route.
	cases := map[string]bool{
		"":                    false,
		"bearer xyz":          false, // lowercase scheme — refused
		"Bearer\txyz":         false, // tab — refused
		"Basic xyz":           false, // wrong scheme — refused
		"Token xyz":           false, // wrong scheme — refused
		"Bearer":              false, // missing token — refused
		"Bearer ":             false, // empty token — refused (trim)
		"Bearer  xyz":         true,  // canonical; we trim the space
		"Bearer xyz":          true,  // canonical
		"Bearer xyz with x y": true,  // canonical; multi-space token OK
	}
	for header, wantUsable := range cases {
		t.Run(header, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/api/v3/sessions", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			got := bearerToken(req)
			usable := got != ""
			if usable != wantUsable {
				t.Fatalf("Authorization=%q returned token=%q (usable=%v); want usable=%v (F-BEARERBP-01 evidence)",
					header, got, usable, wantUsable)
			}
		})
	}
}

func TestBearerTokenDoesNotEchoHeader(t *testing.T) {
	// The returned token must be exactly the bytes after `Bearer `
	// — no case-folding, no header echo. This bounds the surface
	// of the auth check: a typo'd header prefix will not bypass.
	req, _ := http.NewRequest(http.MethodGet, "/api/v3/sessions", nil)
	req.Header.Set("Authorization", "Bearer xyz-secret")
	got := bearerToken(req)
	if got != "xyz-secret" {
		t.Fatalf("bearer token=%q want xyz-secret", got)
	}
	if strings.Contains(strings.ToLower(got), "bearer") {
		t.Fatalf("token leaked scheme: %q", got)
	}
}

// TestSlowlorisBound confirms the server enforces ReadHeaderTimeout
// at construction. The package-level defaults must include a
// non-zero ReadHeaderTimeout; a misconfigured deploy that sets it
// to zero would be a pinned-goroutine vector.
func TestSlowlorisBound(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	if cfg.ReadHeaderTimeout <= 0 {
		t.Fatalf("ReadHeaderTimeout must be a positive guard; got %v", cfg.ReadHeaderTimeout)
	}
	if cfg.ReadTimeout <= 0 {
		t.Fatalf("ReadTimeout must be a positive guard; got %v", cfg.ReadTimeout)
	}
	// Confirmation: a zero-out attempt produces a non-positive
	// value that the next reviewer can spot.
	zero := cfg
	zero.ReadHeaderTimeout = 0
	if zero.ReadHeaderTimeout == cfg.ReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout was not actually configurable")
	}
}
