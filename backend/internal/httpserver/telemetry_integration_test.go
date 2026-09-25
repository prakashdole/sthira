package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPprofEndpoints_DisabledByDefault(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	cfg.EnablePprof = false

	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/ failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when pprof is disabled, got %d", resp.StatusCode)
	}
}

func TestPprofEndpoints_TokenGuarded(t *testing.T) {
	const secretToken = "super-secret-pprof-token-xyz"
	cfg := DefaultConfig("127.0.0.1:0")

	srv := New(cfg, WithPprof(secretToken))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Missing token -> 401 Unauthorized
	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/ failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on missing token, got %d", resp.StatusCode)
	}

	// 2. Wrong token -> 401 Unauthorized
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/debug/pprof/", nil)
	req.Header.Set("X-Pprof-Token", "wrong-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /debug/pprof/ with wrong token failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong token, got %d", resp.StatusCode)
	}

	// 3. Valid X-Pprof-Token -> 200 OK
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/debug/pprof/", nil)
	req.Header.Set("X-Pprof-Token", secretToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /debug/pprof/ with valid X-Pprof-Token failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on valid X-Pprof-Token, got %d", resp.StatusCode)
	}

	// 4. Valid Authorization Bearer -> 200 OK for /debug/pprof/heap
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/debug/pprof/heap", nil)
	req.Header.Set("Authorization", "Bearer "+secretToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /debug/pprof/heap with valid Bearer token failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on valid Bearer token, got %d", resp.StatusCode)
	}
}

func TestAccessLog_PrivacyInvariants(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg, WithAccessLog(true))
	srv.logger = logger

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Make request with sensitive bearer token and headers
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health/live", nil)
	req.Header.Set("Authorization", "Bearer citizen-secret-token-12345")
	req.Header.Set("X-Citizen-GPS", "76.105,11.570")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	logOutput := logBuf.String()

	// Must contain operational metadata
	if !strings.Contains(logOutput, "http request completed") {
		t.Fatalf("expected access log to contain 'http request completed', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "path=/health/live") {
		t.Fatalf("expected access log to contain path, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "status=200") {
		t.Fatalf("expected access log to contain status=200, got: %s", logOutput)
	}

	// Strictly NEVER leak sensitive data
	if strings.Contains(logOutput, "citizen-secret-token-12345") {
		t.Fatalf("PRIVACY VIOLATION: Access log leaked bearer token: %s", logOutput)
	}
	if strings.Contains(logOutput, "76.105") || strings.Contains(logOutput, "11.570") {
		t.Fatalf("PRIVACY VIOLATION: Access log leaked GPS coordinates: %s", logOutput)
	}
}
