package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
)

// newTestServer returns a server with no prober (readiness must be BLOCKED).
func newTestServer() *Server {
	return New(DefaultConfig("127.0.0.1:0"),
		WithKnownIDs("PLACE-DEMO-1", "FACILITY-DEMO-1", "FACILITY-DEMO-2"),
	)
}

// response captures a real HTTP response for assertions.
type response struct {
	code   int
	header http.Header
	body   string
}

func do(t *testing.T, srv *httptest.Server, method, path, ct, body string) response {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return response{code: resp.StatusCode, header: resp.Header, body: string(b)}
}

func decodeEnvelope(t *testing.T, rec response) contracts.Envelope {
	t.Helper()
	var env contracts.Envelope
	if err := json.Unmarshal([]byte(rec.body), &env); err != nil {
		t.Fatalf("response is not a valid envelope: %v\nbody: %s", err, rec.body)
	}
	return env
}

func TestLivenessAlwaysLive(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodGet, "/health/live", "", "")
	if rec.code != http.StatusOK {
		t.Fatalf("liveness = %d, want 200", rec.code)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) != 0 || env.Data == nil {
		t.Fatalf("liveness envelope malformed: %+v", env)
	}
}

func TestReadinessBlockedWithoutProber(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodGet, "/health/ready", "", "")
	if rec.code != http.StatusServiceUnavailable {
		t.Fatalf("readiness without prober = %d, want 503 (never false READY)", rec.code)
	}
	env := decodeEnvelope(t, rec)
	if env.Data != nil || len(env.Errors) == 0 {
		t.Fatalf("readiness must report an error, not data: %+v", env)
	}
}

type failingProber struct{}

func (failingProber) Probe(ctx context.Context) error { return errors.New("db dial refused") }

func TestReadinessBlockedWhenDependencyFails(t *testing.T) {
	s := New(DefaultConfig("127.0.0.1:0"), WithProber(failingProber{}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodGet, "/health/ready", "", "")
	if rec.code != http.StatusServiceUnavailable {
		t.Fatalf("readiness with failing dependency = %d, want 503", rec.code)
	}
	// The internal reason must be redacted from the response.
	if strings.Contains(rec.body, "db dial refused") {
		t.Fatalf("readiness leaked internal reason: %s", rec.body)
	}
}

type okProber struct{}

func (okProber) Probe(ctx context.Context) error { return nil }

func TestReadinessReadyWhenProberPasses(t *testing.T) {
	s := New(DefaultConfig("127.0.0.1:0"), WithProber(okProber{}))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodGet, "/health/ready", "", "")
	if rec.code != http.StatusOK {
		t.Fatalf("readiness with passing prober = %d, want 200", rec.code)
	}
}

func TestLivenessRejectsWrongMethod(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodPost, "/health/live", "application/json", "{}")
	if rec.code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health/live = %d, want 405", rec.code)
	}
	if rec.header.Get("Allow") != http.MethodGet {
		t.Fatalf("missing/incorrect Allow header: %q", rec.header.Get("Allow"))
	}
}

func validCommandBody() string {
	return `{"request_id":"REQ-DEMO-1","data_version":"EXERCISE-7","proposal":` +
		`{"schema_version":"3.0","request_id":"REQ-DEMO-1","data_version":"EXERCISE-7","status":"OK","intent":"FOCUS_PLACE","language":"ml-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"PLACE-DEMO-1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":["PLACE-DEMO-1"]}}`
}

func TestVoiceCommandsAcceptsValidProposal(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json", validCommandBody())
	if rec.code != http.StatusOK {
		t.Fatalf("valid proposal = %d, want 200; body: %s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if env.Data == nil || len(env.Errors) != 0 {
		t.Fatalf("valid proposal must return data: %+v", env)
	}
}

func TestVoiceCommandsRejectsWrongContentType(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "text/plain", validCommandBody())
	if rec.code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain = %d, want 415", rec.code)
	}
}

func TestVoiceCommandsRejectsWrongMethod(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	rec := do(t, srv, http.MethodGet, "/api/v3/voice/commands", "", "")
	if rec.code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/v3/voice/commands = %d, want 405", rec.code)
	}
}

func TestVoiceCommandsRejectsDuplicateKey(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	body := `{"request_id":"REQ-DEMO-1","request_id":"REQ-DEMO-2","data_version":"EXERCISE-7","proposal":{}}`
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json", body)
	if rec.code != http.StatusBadRequest {
		t.Fatalf("duplicate key = %d, want 400", rec.code)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrDuplicateKey {
		t.Fatalf("want %s, got %+v", contracts.ErrDuplicateKey, env.Errors)
	}
}

func TestVoiceCommandsRejectsUnknownField(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	body := `{"request_id":"REQ-DEMO-1","data_version":"EXERCISE-7","proposal":{},"injected":true}`
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json", body)
	if rec.code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, want 400", rec.code)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrUnknownField {
		t.Fatalf("want %s, got %+v", contracts.ErrUnknownField, env.Errors)
	}
}

func TestVoiceCommandsRejectsProhibitedAction(t *testing.T) {
	srv := httptest.NewServer(newTestServer().Handler())
	defer srv.Close()
	body := `{"request_id":"REQ-DEMO-1","data_version":"EXERCISE-7","proposal":` +
		`{"schema_version":"3.0","request_id":"REQ-DEMO-1","data_version":"EXERCISE-7","status":"OK","intent":"OPEN_CONFIRMATION","language":"en-IN","actions":[{"type":"DIAL_112","target_id":"112"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}}`
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json", body)
	if rec.code != http.StatusUnprocessableEntity {
		t.Fatalf("prohibited action = %d, want 422", rec.code)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrValidation {
		t.Fatalf("want %s, got %+v", contracts.ErrValidation, env.Errors)
	}
}

func TestVoiceCommandsRejectsOversizedBody(t *testing.T) {
	s := New(Config{
		Addr: "127.0.0.1:0", MaxBodyBytes: 256, MaxJSONDepth: 8,
		ReadHeaderTimeout: time.Second, ReadTimeout: time.Second,
		WriteTimeout: time.Second, IdleTimeout: time.Second, ShutdownTimeout: time.Second,
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	body := `{"request_id":"` + strings.Repeat("a", 1000) + `"}`
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json", body)
	if rec.code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body = %d, want 413", rec.code)
	}
}
