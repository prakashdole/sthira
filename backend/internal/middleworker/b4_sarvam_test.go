package middleworker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestB4_SarvamChatTemplateWiredIntoRequestPath verifies the
// Sarvam chat template reaches the vLLM endpoint when the
// runtime is configured. Without this, the template would be a
// dangling constant — wired into SarvamConfig but never sent
// over the wire.
func TestB4_SarvamChatTemplateWiredIntoRequestPath(t *testing.T) {
	// Stand up a fake vLLM that records the request body and
	// returns a minimal valid chat completion.
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- body
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "test",
			"object":  "chat.completion",
			"created": 0,
			"model":   SarvamModelID,
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"schema_version":"3.0","request_id":"R-b4","data_version":"v1","status":"OK","intent":null,"language":"hi-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`,
					},
				},
			},
		})
	}))
	defer srv.Close()

	cli, err := NewClient(ClientConfig{
		BaseURL: srv.URL,
		Limits:  DefaultLimits,
	})
	if err != nil {
		t.Fatal(err)
	}
	rt, err := NewHTTPClientRuntime(SarvamConfig(cli, "you are a constrained voice map"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Propose(testCtx(t), RequestEnvelope{
		RequestID: "R-b4",
		ScopedContext: ScopedContext{
			RequestID:        "R-b4",
			DataVersion:      "v1",
			SchemaVersion:    "3.0",
			Jurisdiction:     "KL-WYD",
			AllowedLanguages: []string{"hi-IN"},
		},
		Transcript:      TranscriptInput{RequestID: "R-b4", Language: "hi-IN", Text: "hello", State: "OK"},
		MaxOutputTokens: 256,
		DeadlineMillis:  5000,
	}); err != nil {
		t.Fatal(err)
	}

	body := <-got
	var req chatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.ChatTemplate == "" {
		t.Fatal("chat_template not sent; Sarvam reasoning controls are not wired")
	}
	if !strings.Contains(req.ChatTemplate, "enable_thinking = false") {
		t.Errorf("SarvamChatTemplate must disable thinking; got %q", req.ChatTemplate)
	}
	// Verify the template contains the documented Gemma-style turn
	// tokens. (If the model card specifies different tokens,
	// deployment-time verification is required; the runtime will
	// surface 400 MALFORMED if the live server rejects them.)
	if !strings.Contains(req.ChatTemplate, "<|start_of_turn|>") {
		t.Errorf("SarvamChatTemplate must use Gemma-style turn tokens; got %q", req.ChatTemplate)
	}
	if req.Model != SarvamModelID {
		t.Errorf("model: got %q want %q", req.Model, SarvamModelID)
	}
	// Verify the chat_template field is sent only when non-empty
	// (the omitempty rule): the wire JSON omits it for runtimes
	// that don't override.
	cli2, err := NewClient(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	rt2, err := NewHTTPClientRuntime(HTTPClientRuntimeConfig{
		Client: cli2, ModelID: "stub", System: "stub",
	})
	if err != nil {
		t.Fatal(err)
	}
	got2 := make(chan []byte, 1)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got2 <- body
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": `{"schema_version":"3.0","request_id":"R-b4-2","data_version":"v1","status":"OK","intent":null,"language":"hi-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`},
			}},
		})
	}))
	defer srv2.Close()
	cli2b, _ := NewClient(ClientConfig{BaseURL: srv2.URL})
	rt2.client = cli2b
	if _, err := rt2.Propose(testCtx(t), RequestEnvelope{
		RequestID: "R-b4-2",
		ScopedContext: ScopedContext{
			RequestID: "R-b4-2", DataVersion: "v1", SchemaVersion: "3.0",
			Jurisdiction: "KL-WYD", AllowedLanguages: []string{"hi-IN"},
		},
		Transcript:      TranscriptInput{RequestID: "R-b4-2", Language: "hi-IN", State: "OK"},
		MaxOutputTokens: 256,
		DeadlineMillis:  5000,
	}); err != nil {
		t.Fatal(err)
	}
	body2 := <-got2
	// The "chat_template" key must be absent (omitempty + empty value).
	if bytes.Contains(body2, []byte("chat_template")) {
		t.Errorf("chat_template should be omitted when empty; got body %s", body2)
	}
}
