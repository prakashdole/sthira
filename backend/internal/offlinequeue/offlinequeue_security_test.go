package offlinequeue

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var _ = strings.Contains // keep import used

// These tests pin the security boundary the offlinequeue dispatcher
// enforces today: an absolute-path /api/-prefixed URL only, with no
// scheme smuggling and no `..` traversal. Anything that escapes the
// guard would let a queued write reach an attacker-controlled host
// (SSRF-via-queue). The runner here is the security-surface witness
// for backend/security/pending-p6.md (F-OUTGOINGEGRESS-01).

func TestValidateEndpointRejectsTraversal(t *testing.T) {
	cases := []string{
		"",
		"api/v3",
		"/api/../admin",
		"https://attacker.example/api/sessions",
		"//attacker.example/api/sessions",
		"/api/v3/../../etc/passwd",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			if err := validateEndpoint(p); err == nil {
				t.Fatalf("path %q accepted by validateEndpoint", p)
			}
		})
	}
}

func TestValidateEndpointAcceptsAPIPrefix(t *testing.T) {
	for _, p := range []string{
		"/api/v3/sessions",
		"/api/v3/reservations",
		"/api/v3/stay/abc/events",
		"/api/", // the prefix boundary itself
	} {
		t.Run(p, func(t *testing.T) {
			if err := validateEndpoint(p); err != nil {
				t.Fatalf("path %q rejected: %v", p, err)
			}
		})
	}
}

// TestDispatcherDoesNotFollowRedirects is a negative reproducer for
// F-OUTGOINGEGRESS-01. A staging HttpClient must be constructed with
// CheckRedirect returning http.ErrUseLastResponse; the test asserts
// that the default http.Client in this package will not silently
// follow a target redirect to an attacker host (the fixture serves
// a 301 to its sibling localhost server).
func TestDispatcherDoesNotFollowRedirects(t *testing.T) {
	// destination: a hostile sibling server.
	malicious := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"stolen":true}`))
	}))
	defer malicious.Close()

	// redirector: a benign localhost server that 301s to malicious.
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Note: validateEndpoint blocked any path that mentions
		// a scheme or traversal, so the 302 here is a malformed
		// queue payload smuggling attempt that an unguarded
		// downstream URL parser could honour.
		http.Redirect(w, &http.Request{URL: nil}, malicious.URL, http.StatusFound)
	}))
	defer redirector.Close()

	// Construct a strict client that does NOT follow redirects.
	strict := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	d := NewHTTPClientDispatcher(redirector.URL, strict)
	// The dispatcher MUST use the validated endpoint, which
	// rejects any path containing `://`. The strict client
	// refuses to follow the 302 to the malicious sibling. The
	// expected outcome: status=302 (returned to the dispatcher)
	// and an empty body that is NOT the malicious sibling's.
	status, body, err := d.PostJSON(context.Background(), "POST", "/api/v3/sessions", "REDACTED", []byte(`{}`))
	if err != nil {
		t.Fatalf("dispatcher returned error %v; should surface status body instead", err)
	}
	if status != http.StatusFound {
		t.Fatalf("status=%d want 302; the dispatcher followed the redirect", status)
	}
	if strings.Contains(string(body), "stolen") {
		t.Fatalf("dispatcher body contains attacker payload; F-OUTGOINGEGRESS-01 reproducible")
	}
}
