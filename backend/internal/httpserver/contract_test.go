package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServedRoutesMatchOpenAPI asserts every path the OpenAPI contract
// documents is actually routed by the server (a wrong-method request returns
// 405, not 404). This catches a contract/implementation drift where the spec
// names an endpoint the server does not serve. It does not exercise behavior;
// the real-DB suite covers that.
func TestServedRoutesMatchOpenAPI(t *testing.T) {
	// Paths from contracts/openapi.yaml, with the wrong method to trigger 405.
	cases := []struct {
		path       string
		wrongVerb  string
		wantStatus int
	}{
		// Health endpoints are served unprefixed (existing convention); the
		// OpenAPI `servers: /api/v3` prefix does not apply to them.
		{"/health/live", http.MethodPost, http.StatusMethodNotAllowed},
		{"/health/ready", http.MethodPost, http.StatusMethodNotAllowed},
		{"/api/v3/voice/commands", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/sessions", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/places/resolve", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/guidance/query", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/reservations", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/reservations/STAY-x", http.MethodDelete, http.StatusMethodNotAllowed},
		{"/api/v3/reservations/STAY-x/events", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/operations/sessions", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/operations/sources/SRC-x/transitions", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/operations/sources/SRC-x/quarantine", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/operations/stays/STAY-x/corrections", http.MethodGet, http.StatusMethodNotAllowed},
		{"/api/v3/regions/KL/manifest", http.MethodPost, http.StatusMethodNotAllowed},
		{"/api/v3/packages/PKG-1/versions/1", http.MethodPost, http.StatusMethodNotAllowed},
		{"/api/v3/resources/RES-1", http.MethodPost, http.StatusMethodNotAllowed},
		// P6 — the new /voice/transcriptions, /voice/process and /voice/speech
		// handlers are owned by Workers 5/7/9 and are wired into the router
		// during the integration stage. Until then they intentionally return
		// 404 and are not asserted by this test.
	}
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()

	for _, tc := range cases {
		req, err := http.NewRequest(tc.wrongVerb, srv.URL+tc.path, strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.wrongVerb, tc.path, err)
		}
		resp.Body.Close()
		// 405 means the path is routed but the method is wrong (expected). 404
		// means the path is not routed at all — contract drift.
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("%s: documented path not routed (404)", tc.path)
		}
	}
}
