// Tests for the loopback HTTP server. End-to-end over real HTTP
// with a stub runtime and a fresh listener.

package middleworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeServerFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "proposal.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func startTestServer(t *testing.T, langs []string) (*Server, string) {
	t.Helper()
	fixture := writeServerFixture(t, `{"schema_version":"3.0","request_id":"REQ-1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`)
	rt, err := NewStubRuntime(fixture)
	if err != nil {
		t.Fatalf("NewStubRuntime: %v", err)
	}
	rt.SetLanguages(langs)
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1, BuildRevision: "test-build"})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	srv, err := NewServer(ServerConfig{
		Address:      "127.0.0.1:0",
		Token:        "test-token",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ShutdownWait: 2 * time.Second,
	}, w)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	go func() { _ = srv.Start() }()
	// Wait for the server to be ready by hitting /health.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + srv.Addr() + "/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Cleanup(func() {
		_ = srv.Shutdown()
	})
	return srv, srv.Addr()
}

func TestServer_HealthRequiresAuth(t *testing.T) {
	_, addr := startTestServer(t, []string{"en-IN"})
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status=%d", resp.StatusCode)
	}
}

func TestServer_ProposeRequiresAuth(t *testing.T) {
	_, addr := startTestServer(t, []string{"en-IN"})
	body := mustEncode(t, RequestEnvelope{
		RequestID: "REQ-1",
		ScopedContext: ScopedContext{
			SchemaVersion: "3.0", DataVersion: "v1",
		},
		Transcript: TranscriptInput{
			RequestID: "REQ-1", Language: "en-IN", Text: "show shelter", State: "OK",
		},
		MaxOutputTokens: 64,
		DeadlineMillis:  5000,
	})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", resp.StatusCode)
	}
}

func TestServer_ProposeHappyPath(t *testing.T) {
	_, addr := startTestServer(t, []string{"en-IN"})
	body := mustEncode(t, RequestEnvelope{
		RequestID: "REQ-1",
		ScopedContext: ScopedContext{
			SchemaVersion: "3.0", DataVersion: "v1",
		},
		Transcript: TranscriptInput{
			RequestID: "REQ-1", Language: "en-IN", Text: "show shelter", State: "OK",
		},
		MaxOutputTokens: 64,
		DeadlineMillis:  5000,
	})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bs, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d, body=%s", resp.StatusCode, bs)
	}
	var env ResponseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Proposal.RequestID != "REQ-1" {
		t.Errorf("proposal.request_id=%q", env.Proposal.RequestID)
	}
	if env.Proposal.Status != "OK" {
		t.Errorf("status=%q", env.Proposal.Status)
	}
}

func TestServer_ProposeMalformedBodyFailsClosed(t *testing.T) {
	_, addr := startTestServer(t, []string{"en-IN"})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/chat/completions", bytes.NewReader([]byte(`{not json`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d", resp.StatusCode)
	}
	state := resp.Header.Get("X-Sthira-State")
	if state != MiddleStateMalformed {
		t.Errorf("X-Sthira-State=%q, want %q", state, MiddleStateMalformed)
	}
}

func TestServer_ProposeUnknownFieldFailsClosed(t *testing.T) {
	_, addr := startTestServer(t, []string{"en-IN"})
	body := []byte(`{"request_id":"REQ-1","scoped_context":{"schema_version":"3.0","data_version":"v1"},"transcript":{"request_id":"REQ-1","language":"en-IN","text":"x","state":"OK"},"unknown_field":"surprise"}`)
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d", resp.StatusCode)
	}
}

func TestServer_HealthEnvelopeShape(t *testing.T) {
	_, addr := startTestServer(t, []string{"en-IN", "hi-IN"})
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	var h HealthEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !h.Ready || !h.Warm {
		t.Errorf("health: ready=%v warm=%v", h.Ready, h.Warm)
	}
	if h.Queue.MaxDepth != 2 || h.Queue.MaxConcurrency != 1 {
		t.Errorf("queue: %+v", h.Queue)
	}
	if len(h.Models) != 1 || h.Models[0].RemoteCode {
		t.Errorf("models=%+v", h.Models)
	}
}

func TestServer_ShutdownReturns(t *testing.T) {
	_, addr := startTestServer(t, []string{"en-IN"})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/shutdown", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /shutdown: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status=%d", resp.StatusCode)
	}
}

func TestServer_NewServerRejectsEmptyAddress(t *testing.T) {
	w, _ := NewWorker(Config{Runtime: &stubWithLanguages{}, QueueDepth: 1, MaxInFlight: 1})
	_, err := NewServer(ServerConfig{Address: ""}, w)
	if err == nil {
		t.Fatalf("expected error on empty address")
	}
}

func TestServer_NewServerRejectsNilWorker(t *testing.T) {
	_, err := NewServer(ServerConfig{Address: "127.0.0.1:0"}, nil)
	if err == nil {
		t.Fatalf("expected error on nil worker")
	}
}

func mustEncode(t *testing.T, v any) []byte {
	t.Helper()
	bs, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bs
}

var _ = errors.Is
var _ = strings.Contains
