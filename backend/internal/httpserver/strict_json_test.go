package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sthira/backend/internal/contracts"
)

type testErrorEnvelope struct {
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field"`
	} `json:"errors"`
}

func TestStrictJSON_UnknownFieldRejected(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := `{"jurisdiction":"IN-KL","query":"Meppadi","malicious_injection":12345}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v3/places/resolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for unknown field, got %d", resp.StatusCode)
	}

	var env testErrorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrUnknownField {
		t.Fatalf("expected error code %s, got %+v", contracts.ErrUnknownField, env.Errors)
	}
}

func TestStrictJSON_DuplicateKeyRejected(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := `{"jurisdiction":"IN-KL","jurisdiction":"IN-KL","query":"Meppadi"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for duplicate key, got %d", resp.StatusCode)
	}

	var env testErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrDuplicateKey {
		t.Fatalf("expected error code %s, got %+v", contracts.ErrDuplicateKey, env.Errors)
	}
}

func TestStrictJSON_TrailingDataRejected(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := `{"jurisdiction":"IN-KL","query":"Meppadi"} 12345`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for trailing data, got %d", resp.StatusCode)
	}

	var env testErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrTrailingData {
		t.Fatalf("expected error code %s, got %+v", contracts.ErrTrailingData, env.Errors)
	}
}

func TestStrictJSON_DepthExceededRejected(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	cfg.MaxJSONDepth = 4
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := `{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/places/resolve", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for depth exceeded, got %d", resp.StatusCode)
	}

	var env testErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrDepthExceeded {
		t.Fatalf("expected error code %s, got %+v", contracts.ErrDepthExceeded, env.Errors)
	}
}
