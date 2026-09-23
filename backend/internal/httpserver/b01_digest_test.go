package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/orchestration/orchestrationtest"
)

// TestB01_VoiceSpeech_DigestMismatchRejected: public /voice/speech must
// refuse to synthesize when the registry template digest does not match
// the approved ScopedContext digest (B01 content authorization). Outbound
// TTS must not run for unauthorized text.
func TestB01_VoiceSpeech_DigestMismatchRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	// Approved digest is for the fixture text; registry holds different text.
	sc.ApprovedTemplateSHA["welcome"] = orchestrationtest.DigestString("Welcome, citizen.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	wrong := "Not the approved text."
	sum := sha256.Sum256([]byte(wrong))
	_ = hex.EncodeToString(sum[:]) // documented: wrong text would digest differently
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: wrong, SyntheticOnly: false,
	})
	orch, err := orchestrationtest.NewOrchestrator(asr, mid, tts, resolver, validator, tpls)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	h := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	srv := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(h))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	body := `{"request_id":"req-b01-dig","jurisdiction":"JTEST","speech_key":"welcome","language":"en-IN","args":{"args":{}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v3/voice/speech", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	var payload map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("digest mismatch must not yield 200 audio; got %d body=%v", resp.StatusCode, payload)
	}
	if tts.SynthesizeCalls() != 0 {
		t.Fatalf("outbound TTS must not run for unauthorized text; calls=%d", tts.SynthesizeCalls())
	}
}
