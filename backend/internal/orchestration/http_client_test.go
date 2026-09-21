package orchestration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
)

func TestHTTPWorkerClient_Health(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{
			Ready:              true,
			Warm:               true,
			SupportedLanguages: []string{"en-IN", "hi-IN"},
		})
	}))
	defer srv.Close()

	client := NewHTTPWorkerClient(srv.URL, "test-tok", srv.Client())
	h, ok := client.Health(context.Background())
	if !ok {
		t.Fatalf("expected health OK")
	}
	if !h.Ready || !h.Warm || len(h.SupportedLanguages) != 2 {
		t.Fatalf("unexpected health payload: %+v", h)
	}

	badClient := NewHTTPWorkerClient(srv.URL, "wrong-tok", srv.Client())
	_, ok = badClient.Health(context.Background())
	if ok {
		t.Fatalf("expected health false with bad token")
	}
}

func TestHTTPWorkerClient_Endpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/transcribe":
			_ = json.NewEncoder(w).Encode(contracts.ASRWorkerResponse{
				RequestID: "req-1",
				Language:  "en-IN",
				Text:      "hello",
				State:     contracts.TranscriptionOK,
			})
		case "/v1/chat/completions":
			_ = json.NewEncoder(w).Encode(contracts.MiddleWorkerResponse{
				RequestID:   "req-1",
				DataVersion: "dv-1",
				Proposal: contracts.ModelOutput{
					Status: "OK",
				},
			})
		case "/synthesize":
			_ = json.NewEncoder(w).Encode(contracts.TTSWorkerResponse{
				RequestID: "req-1",
				SpeechKey: "key-1",
				State:     contracts.TTSOK,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewHTTPWorkerClient(srv.URL, "", srv.Client())

	asrResp, err := client.Transcribe(context.Background(), contracts.ASRWorkerRequest{RequestID: "req-1"})
	if err != nil || asrResp.Text != "hello" {
		t.Fatalf("Transcribe failed: %v, %+v", err, asrResp)
	}

	midResp, err := client.Propose(context.Background(), contracts.MiddleWorkerRequest{RequestID: "req-1"})
	if err != nil || midResp.DataVersion != "dv-1" {
		t.Fatalf("Propose failed: %v, %+v", err, midResp)
	}

	ttsResp, err := client.Synthesize(context.Background(), contracts.TTSWorkerRequest{RequestID: "req-1"})
	if err != nil || ttsResp.SpeechKey != "key-1" {
		t.Fatalf("Synthesize failed: %v, %+v", err, ttsResp)
	}
}

func TestHTTPWorkerClient_ErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/transcribe":
			http.Error(w, "busy", http.StatusTooManyRequests)
		case "/v1/chat/completions":
			http.Error(w, "down", http.StatusServiceUnavailable)
		case "/synthesize":
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer srv.Close()

	client := NewHTTPWorkerClient(srv.URL, "", srv.Client())

	_, err := client.Transcribe(context.Background(), contracts.ASRWorkerRequest{RequestID: "req-1"})
	if err != ErrQueueSaturated {
		t.Fatalf("expected ErrQueueSaturated, got %v", err)
	}

	_, err = client.Propose(context.Background(), contracts.MiddleWorkerRequest{RequestID: "req-1"})
	if err != ErrModelUnavailable {
		t.Fatalf("expected ErrModelUnavailable, got %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = client.Synthesize(ctx, contracts.TTSWorkerRequest{RequestID: "req-1"})
	if err != ErrModelTimeout {
		t.Fatalf("expected ErrModelTimeout, got %v", err)
	}
}
