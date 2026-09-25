package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_AppliedAcrossEndpoints(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	testCases := []struct {
		name        string
		path        string
		method      string
		wantStatus  int
		wantNoStore bool
	}{
		{
			name:        "Liveness",
			path:        "/health/live",
			method:      http.MethodGet,
			wantStatus:  http.StatusOK,
			wantNoStore: false,
		},
		{
			name:        "NotFound_404",
			path:        "/non-existent-route-xyz",
			method:      http.MethodGet,
			wantStatus:  http.StatusNotFound,
			wantNoStore: false,
		},
		{
			name:        "MethodNotAllowed_405",
			path:        "/health/live",
			method:      http.MethodPost,
			wantStatus:  http.StatusMethodNotAllowed,
			wantNoStore: false,
		},
		{
			name:        "SensitiveSessionsRoute",
			path:        "/api/v3/sessions",
			method:      http.MethodGet,
			wantStatus:  http.StatusMethodNotAllowed, // POST only, but headers still apply
			wantNoStore: true,
		},
		{
			name:        "SensitiveReservationsRoute",
			path:        "/api/v3/reservations",
			method:      http.MethodGet,
			wantStatus:  http.StatusServiceUnavailable, // unwired store returns 503; headers still apply
			wantNoStore: true,
		},
		{
			name:        "SensitiveOperationsRoute",
			path:        "/api/v3/operations/sessions",
			method:      http.MethodGet,
			wantStatus:  http.StatusMethodNotAllowed,
			wantNoStore: true,
		},
		{
			name:        "SensitiveObservabilityRoute",
			path:        "/api/v3/observability/metrics",
			method:      http.MethodGet,
			wantStatus:  http.StatusServiceUnavailable, // unconfigured token
			wantNoStore: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Do request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}

			// Mandatory defense-in-depth headers on every single response
			assertHeader(t, resp, "X-Frame-Options", "DENY")
			assertHeader(t, resp, "X-Content-Type-Options", "nosniff")
			assertHeader(t, resp, "Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			assertHeader(t, resp, "Referrer-Policy", "strict-origin-when-cross-origin")
			assertHeader(t, resp, "Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			assertHeader(t, resp, "X-XSS-Protection", "0")

			if tc.wantNoStore {
				assertHeader(t, resp, "Cache-Control", "no-store, no-cache, must-revalidate, private")
				assertHeader(t, resp, "Pragma", "no-cache")
			}
		})
	}
}

func TestSecurityHeaders_RateLimited429CarriesHeaders(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg, WithRateLimit(1, 1))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Consume token
	resp1, _ := http.Get(ts.URL + "/health/live")
	resp1.Body.Close()

	// 2nd request triggers 429
	resp2, err := http.Get(ts.URL + "/health/live")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp2.StatusCode)
	}

	assertHeader(t, resp2, "X-Frame-Options", "DENY")
	assertHeader(t, resp2, "X-Content-Type-Options", "nosniff")
	assertHeader(t, resp2, "Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	assertHeader(t, resp2, "Referrer-Policy", "strict-origin-when-cross-origin")
	assertHeader(t, resp2, "Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	assertHeader(t, resp2, "X-XSS-Protection", "0")
}

func assertHeader(t *testing.T, resp *http.Response, name, want string) {
	t.Helper()
	got := resp.Header.Get(name)
	if got != want {
		t.Errorf("header %q = %q, want %q", name, got, want)
	}
}
