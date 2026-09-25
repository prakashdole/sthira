package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
)

func TestClockDrift_ValidAndAbsentTimestamp(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Absent header passes through to handler
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(`{"jurisdiction":"IN-KL","query":"Meppadi"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST without timestamp: %v", err)
	}
	resp.Body.Close()
	// Passes clock drift check (store unconfigured -> 503 is expected, not 400 CLOCK_DRIFT)
	if resp.StatusCode == http.StatusBadRequest {
		t.Fatalf("unexpected 400 for absent timestamp")
	}

	// 2. Valid current timestamp passes
	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(`{"jurisdiction":"IN-KL","query":"Meppadi"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(HeaderClientTimestamp, time.Now().UTC().Format(time.RFC3339))
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("POST with current timestamp: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode == http.StatusBadRequest {
		t.Fatalf("unexpected 400 for valid current timestamp")
	}
}

func TestClockDrift_InvalidFormatRejected(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(`{"jurisdiction":"IN-KL","query":"Meppadi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderClientTimestamp, "invalid-unix-1234567")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unparseable timestamp, got %d", resp.StatusCode)
	}

	var env testErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrInvalidValue {
		t.Fatalf("expected INVALID_VALUE, got %+v", env.Errors)
	}
}

func TestClockDrift_DistantFutureRejected(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	future := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(`{"jurisdiction":"IN-KL","query":"Meppadi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderClientTimestamp, future)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for future timestamp, got %d", resp.StatusCode)
	}

	var env testErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrClockDrift {
		t.Fatalf("expected CLOCK_DRIFT, got %+v", env.Errors)
	}
}

func TestClockDrift_StaleStatefulRequestRejected(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	stale := time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(`{"jurisdiction":"IN-KL","query":"Meppadi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderClientTimestamp, stale)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for stale stateful request, got %d", resp.StatusCode)
	}

	var env testErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrClockDrift {
		t.Fatalf("expected CLOCK_DRIFT, got %+v", env.Errors)
	}
}
