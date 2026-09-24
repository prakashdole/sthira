package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpserver"
)

type corsAPIErrorResponse struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func TestCORS_DefaultDisabledIsolation(t *testing.T) {
	srv := httpserver.New(httpserver.DefaultConfig("127.0.0.1:0"))
	handler := srv.Handler()

	// Preflight from any origin should be 403 Forbidden when CORS is not configured.
	req := httptest.NewRequest(http.MethodOptions, "/health/live", nil)
	req.Header.Set("Origin", "https://unauthorized.evil.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unconfigured origin preflight, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unexpected Access-Control-Allow-Origin on forbidden origin")
	}

	// Security headers must still be present
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected X-Frame-Options: DENY, got %q", rec.Header().Get("X-Frame-Options"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options: nosniff, got %q", rec.Header().Get("X-Content-Type-Options"))
	}

	var errResp corsAPIErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if len(errResp.Errors) == 0 || errResp.Errors[0].Code != contracts.ErrForbidden {
		t.Fatalf("expected error code %s, got %+v", contracts.ErrForbidden, errResp)
	}
}

func TestCORS_AllowedOriginPreflightAndActual(t *testing.T) {
	allowedOrigin := "https://citizen.sthira.gov.in"
	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithAllowedOrigins(allowedOrigin),
	)
	handler := srv.Handler()

	// 1. Preflight OPTIONS
	preflight := httptest.NewRequest(http.MethodOptions, "/health/live", nil)
	preflight.Header.Set("Origin", allowedOrigin)
	preflight.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, preflight)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for allowed preflight, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != allowedOrigin {
		t.Fatalf("expected Access-Control-Allow-Origin: %q, got %q", allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", rec.Header().Get("Vary"))
	}
	if rec.Header().Get("Access-Control-Max-Age") != "86400" {
		t.Fatalf("expected Access-Control-Max-Age: 86400, got %q", rec.Header().Get("Access-Control-Max-Age"))
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected X-Frame-Options: DENY on preflight response")
	}

	// 2. Actual GET Request
	actual := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	actual.Header.Set("Origin", allowedOrigin)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, actual)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for allowed origin GET, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != allowedOrigin {
		t.Fatalf("expected Access-Control-Allow-Origin: %q, got %q", allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", rec.Header().Get("Vary"))
	}
	if rec.Header().Get("Access-Control-Expose-Headers") == "" {
		t.Fatalf("expected Access-Control-Expose-Headers to be set")
	}

	// 3. Disallowed origin GET Request (should not receive Access-Control-Allow-Origin)
	untrusted := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	untrusted.Header.Set("Origin", "https://malicious.example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, untrusted)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for untrusted origin, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected NO Access-Control-Allow-Origin for disallowed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_WildcardRejectionAndNormalization(t *testing.T) {
	// Attempt to configure "*" wildcard along with valid origin
	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithAllowedOrigins("*", "https://sthira.kerala.gov.in/"),
	)
	handler := srv.Handler()

	// Wildcard preflight must be forbidden
	preflightWildcard := httptest.NewRequest(http.MethodOptions, "/health/live", nil)
	preflightWildcard.Header.Set("Origin", "*")
	preflightWildcard.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, preflightWildcard)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for wildcard origin preflight, got %d", rec.Code)
	}

	// Normalized origin without trailing slash should match origin configured with trailing slash
	preflightNorm := httptest.NewRequest(http.MethodOptions, "/health/live", nil)
	preflightNorm.Header.Set("Origin", "https://sthira.kerala.gov.in")
	preflightNorm.Header.Set("Access-Control-Request-Method", "GET")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, preflightNorm)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for normalized origin, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://sthira.kerala.gov.in" {
		t.Fatalf("expected Access-Control-Allow-Origin: https://sthira.kerala.gov.in, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_NoOriginHeaderPassThrough(t *testing.T) {
	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithAllowedOrigins("https://sthira.gov.in"),
	)
	handler := srv.Handler()

	// Request with no Origin header (standard client, curl, mobile app)
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for request without Origin header, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin for request without Origin header")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected security headers to remain intact")
	}
}
