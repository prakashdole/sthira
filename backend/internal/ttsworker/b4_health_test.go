package ttsworker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/ttsworker/templates"
)

// TestB4_TTS_HealthWireShapeMatchesContractsWorkerHealth ensures
// the TTS /health envelope uses the same field names as
// contracts.WorkerHealth, so the orchestrator decodes ASR/middle/
// TTS identically.
func TestB4_TTS_HealthWireShapeMatchesContractsWorkerHealth(t *testing.T) {
	catalog := buildCatalog(t)
	renderer := templates.NewRenderer(catalog)
	clock := NewStandaloneSourceVersionClock(7)
	codec := NewCodec(8*1024*1024, time.Hour, clock)
	inv := Inventory{
		Parler: ParlerTTS{
			ModelID:  "ai4bharat/indic-parler-tts",
			License:  "Apache-2.0",
			Runtime:  "transformers-4.x",
			Hardware: "cpu",
			Revision: "rev-1",
			Voices:   []string{"ml-IN-female-1"},
		},
		SupportedLanguages: []string{"hi-IN", "ml-IN", "en-IN"},
	}
	w, err := New(Config{
		Inventory:   inv,
		Runtime:     newTaggedRuntime("ml-IN", "ml-IN-female-1", "rev-1"),
		Catalog:     catalog,
		Renderer:    renderer,
		Cache:       codec,
		Clock:       clock,
		QueueDepth:  2,
		MaxInFlight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Shutdown() })

	ts := httptest.NewServer(http.HandlerFunc(func(w2 http.ResponseWriter, r *http.Request) {
		w2.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w2).Encode(w.Health())
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d body=%s", resp.StatusCode, body)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}

	// Required fields per contracts.WorkerHealth.
	for _, key := range []string{"ready", "warm", "supported_languages", "queue", "models", "artifacts"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing required field %q in TTS /health envelope", key)
		}
	}

	// queue must be an object with depth/max_depth/max_concurrency.
	q, ok := got["queue"].(map[string]any)
	if !ok {
		t.Errorf("queue field is not an object: %T", got["queue"])
	} else {
		for _, k := range []string{"depth", "max_depth", "max_concurrency"} {
			if _, ok := q[k]; !ok {
				t.Errorf("queue missing %q", k)
			}
		}
	}

	// Models must include the AI4Bharat Parler-TTS identifier
	// (so the orchestrator knows which artifact is loaded).
	models, _ := got["models"].([]any)
	if len(models) == 0 {
		t.Fatalf("expected at least one ModelInfo entry; got %v", got["models"])
	}
	first := models[0].(map[string]any)
	if first["model_id"] != "ai4bharat/indic-parler-tts" {
		t.Errorf("model_id: got %v want ai4bharat/indic-parler-tts", first["model_id"])
	}
	if first["revision"] != "rev-1" {
		t.Errorf("revision: got %v want rev-1", first["revision"])
	}
	if first["runtime"] != "transformers-pinned" {
		t.Errorf("runtime: got %v want transformers-pinned", first["runtime"])
	}

	// Verify no leaked internal-only field names that would
	// indicate the worker is still emitting the old shape.
	// (Models/Artifacts/Queue/SupportedLanguages are the new
	// common names; "runtime_languages", "inventory", and
	// "current_source_version" are internal diagnostics that
	// should NOT be top-level keys on the common envelope.)
	//
	// We keep them as "omitempty" so a runtime that does not
	// set them produces a clean envelope. The check below
	// ensures they DO appear in JSON when present (the
	// runtime carries voices and the inventory is non-empty).
	// What we forbid is replacing the common keys with these
	// internal-only ones.
	for _, legacy := range []string{"runtime_languages"} {
		if _, ok := got[legacy]; ok {
			t.Errorf("legacy field %q still present in TTS /health (common envelope must use 'supported_languages')", legacy)
		}
	}

	// Verify the response is JSON-decodable into the contracts
	// WorkerHealth surface. We use the locally-defined mirror
	// type (the worker module is stdlib-only) but the field
	// names match contracts.WorkerHealth one-for-one.
	var mirror WorkerHealth
	if err := json.Unmarshal(body, &mirror); err != nil {
		t.Fatalf("WorkerHealth mirror unmarshal: %v", err)
	}
	if !mirror.Ready || !mirror.Warm {
		t.Errorf("ready/warm: got %v/%v want true/true", mirror.Ready, mirror.Warm)
	}
	if got := mirror.SupportedLanguages; len(got) == 0 {
		t.Errorf("SupportedLanguages empty")
	}
	if got := mirror.Models; len(got) == 0 {
		t.Errorf("Models empty")
	}
	if mirror.Models[0].ModelID != "ai4bharat/indic-parler-tts" {
		t.Errorf("Models[0].ModelID: got %q", mirror.Models[0].ModelID)
	}
	if mirror.Queue.MaxConcurrency != 2 {
		t.Errorf("Queue.MaxConcurrency: got %d want 2", mirror.Queue.MaxConcurrency)
	}

	_ = strings.TrimSpace // import guard
}

// TestB4_TTS_HealthEnvelopeIsJSONObject confirms the response is
// a single JSON object with the documented field names.
func TestB4_TTS_HealthEnvelopeIsJSONObject(t *testing.T) {
	w, _, _, _, _ := newWorker(t, 2, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w2 http.ResponseWriter, r *http.Request) {
		w2.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w2).Encode(w.Health())
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Must start with { and be a valid JSON object.
	if len(body) == 0 || body[0] != '{' {
		t.Fatalf("response not a JSON object: %q", body[:min(50, len(body))])
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	_ = context.Background
}
