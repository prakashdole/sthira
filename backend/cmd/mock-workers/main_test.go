package main

// Contract tests for the mock private-worker protocol server. They drive the
// same mux main() serves (newMux) over a real loopback listener, so every
// assertion is about bytes an orchestrator would actually receive.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"sthira/backend/internal/contracts"
)

const (
	mockZoneID     = "SZDEMO-1"
	mockFacilityID = "FACDEMO-1"
)

// mockWAVSampleCount is the sample count main() synthesizes for the mock TTS
// audio.
const mockWAVSampleCount = 400

// serve starts the worker routes for one scenario with exactly the audio
// main() hands to newMux, so the served responses are the process responses.
// The executable's own wiring of that audio is covered end to end by
// internal/httpserver's integration-tagged prototype scenario test.
func serve(t *testing.T, sc scenario) *httptest.Server {
	t.Helper()
	wavBytes := generateWAVBytes(mockWAVSampleCount)
	wavSum := sha256.Sum256(wavBytes)
	ts := httptest.NewServer(newMux(sc, base64.StdEncoding.EncodeToString(wavBytes), hex.EncodeToString(wavSum[:])))
	t.Cleanup(ts.Close)
	return ts
}

func postJSON(t *testing.T, ts *httptest.Server, path string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal %s request: %v", path, err)
	}
	resp, err := ts.Client().Post(ts.URL+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s response: %v", path, err)
	}
	return resp.StatusCode, out
}

func middleRequest(requestID, lang string) contracts.MiddleWorkerRequest {
	return contracts.MiddleWorkerRequest{
		RequestID:     requestID,
		ScopedContext: contracts.ScopedContext{DataVersion: "PKGDEMO-1:1"},
		Transcript:    contracts.ASRWorkerResponse{Language: lang},
	}
}

// TestHealthReadyForEveryScenario: the orchestrator refuses to dispatch when
// /health does not report Ready, and demo.sh probes it before every scenario.
// The handler is scenario-independent, so all scenarios must report a warm,
// ready worker speaking the three enabled languages.
func TestHealthReadyForEveryScenario(t *testing.T) {
	if len(validScenarios) == 0 {
		t.Fatal("validScenarios is empty")
	}
	for sc := range validScenarios {
		t.Run(string(sc), func(t *testing.T) {
			ts := serve(t, sc)
			resp, err := ts.Client().Get(ts.URL + "/health")
			if err != nil {
				t.Fatalf("GET /health: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /health status = %d, want 200", resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type = %q, want application/json", ct)
			}
			var health contracts.WorkerHealth
			if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
				t.Fatalf("decode WorkerHealth: %v", err)
			}
			if !health.Ready || !health.Warm {
				t.Fatalf("health = %+v, want ready and warm", health)
			}
			if want := []string{"en-IN", "hi-IN", "ml-IN"}; !reflect.DeepEqual(health.SupportedLanguages, want) {
				t.Fatalf("supported_languages = %v, want %v", health.SupportedLanguages, want)
			}
		})
	}
}

// TestTTSAudioMatchesChecksumForEveryScenario: the orchestrator verifies the
// audio before it forwards audio_b64 to a browser, so the served bytes must
// decode from base64, hash to the checksum_sha256 in the same envelope, and
// carry a RIFF/WAVE header. The checksum is also pinned to the exact mock WAV
// main() generates, so serving different audio would fail here.
func TestTTSAudioMatchesChecksumForEveryScenario(t *testing.T) {
	wantSum := sha256.Sum256(generateWAVBytes(mockWAVSampleCount))
	wantChecksum := hex.EncodeToString(wantSum[:])

	for sc := range validScenarios {
		t.Run(string(sc), func(t *testing.T) {
			ts := serve(t, sc)
			code, raw := postJSON(t, ts, "/synthesize", contracts.TTSWorkerRequest{
				RequestID: "req-tts-1",
				SpeechKey: "destination_options",
				Language:  "en-IN",
			})
			if code != http.StatusOK {
				t.Fatalf("POST /synthesize status = %d, want 200: %s", code, raw)
			}
			var resp contracts.TTSWorkerResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatalf("decode TTSWorkerResponse: %v: %s", err, raw)
			}
			if resp.State != contracts.TTSOK || resp.ContentType != "audio/wav" {
				t.Fatalf("state = %q content_type = %q, want OK / audio/wav", resp.State, resp.ContentType)
			}
			if resp.ChecksumSHA256 != wantChecksum {
				t.Fatalf("checksum_sha256 = %q, want sha256 of the mock WAV %q", resp.ChecksumSHA256, wantChecksum)
			}
			audio, err := base64.StdEncoding.DecodeString(resp.AudioB64)
			if err != nil {
				t.Fatalf("audio_b64 does not decode: %v", err)
			}
			sum := sha256.Sum256(audio)
			if got := hex.EncodeToString(sum[:]); got != resp.ChecksumSHA256 {
				t.Fatalf("sha256(audio) = %q, want checksum_sha256 %q", got, resp.ChecksumSHA256)
			}
			if len(audio) < 12 || string(audio[0:4]) != "RIFF" || string(audio[8:12]) != "WAVE" {
				t.Fatalf("audio is not a RIFF/WAVE buffer: %d bytes, header %q %q", len(audio), audio[0:4], audio[8:12])
			}
		})
	}
}

// TestMiddleProposalPerScenario: the middle worker is a protocol fixture, so
// every scenario's proposal must be a valid, self-consistent model output
// (contracts.ValidateModelOutput is the same check the orchestrator applies).
// worker-failure is the one scenario that must NOT answer with a proposal.
func TestMiddleProposalPerScenario(t *testing.T) {
	const (
		requestID = "req-mw-1"
		dataVer   = "PKGDEMO-1:1"
		lang      = "en-IN"
	)
	known := map[string]bool{mockZoneID: true, mockFacilityID: true}
	enabled := map[string]bool{"en-IN": true, "hi-IN": true, "ml-IN": true}

	cases := []struct {
		sc          scenario
		wantCode    int
		wantStatus  contracts.ModelStatus
		wantIntent  contracts.Intent // "" means a null intent
		wantSpeech  string           // "" means a null speech_key
		wantActions []contracts.Action
	}{
		{
			sc: scenarioDefault, wantCode: http.StatusOK, wantStatus: contracts.StatusOK,
			wantIntent: contracts.IntentFocusPlace, wantSpeech: "destination_options",
			wantActions: []contracts.Action{{Type: contracts.ActionFocusFeature, TargetID: mockZoneID}},
		},
		{
			sc: scenarioSilentZoom, wantCode: http.StatusOK, wantStatus: contracts.StatusOK,
			wantIntent: contracts.IntentZoom, wantSpeech: "",
			wantActions: []contracts.Action{{Type: contracts.ActionZoom, Direction: "IN", Steps: 1}},
		},
		{
			sc: scenarioDestinationChoice, wantCode: http.StatusOK, wantStatus: contracts.StatusOK,
			wantIntent: contracts.IntentListDestinations, wantSpeech: "destination_options",
			wantActions: []contracts.Action{{Type: contracts.ActionShowChoices, TargetIDs: []string{mockFacilityID}}},
		},
		{
			sc: scenarioArrivalConfirm, wantCode: http.StatusOK, wantStatus: contracts.StatusOK,
			wantIntent: contracts.IntentOpenConfirmation, wantSpeech: "",
			wantActions: []contracts.Action{{Type: contracts.ActionOpenPanel, Panel: contracts.PanelArrivalConfirm}},
		},
		{
			sc: scenarioClarify, wantCode: http.StatusOK, wantStatus: contracts.StatusClarify,
			wantIntent: "", wantSpeech: "clarify_place", wantActions: []contracts.Action{},
		},
		{
			sc: scenarioDataUnavailable, wantCode: http.StatusOK, wantStatus: contracts.StatusDataUnavailable,
			wantIntent: "", wantSpeech: "", wantActions: []contracts.Action{},
		},
		{
			sc: scenarioWorkerFailure, wantCode: http.StatusServiceUnavailable, wantStatus: "",
			wantIntent: "", wantSpeech: "", wantActions: nil,
		},
	}

	if len(cases) != len(validScenarios) {
		t.Fatalf("table covers %d scenarios, validScenarios has %d", len(cases), len(validScenarios))
	}
	for _, tc := range cases {
		if !validScenarios[tc.sc] {
			t.Fatalf("table case %q is not a valid scenario", tc.sc)
		}
		t.Run(string(tc.sc), func(t *testing.T) {
			ts := serve(t, tc.sc)
			code, raw := postJSON(t, ts, "/v1/chat/completions", middleRequest(requestID, lang))
			if code != tc.wantCode {
				t.Fatalf("POST /v1/chat/completions status = %d, want %d: %s", code, tc.wantCode, raw)
			}
			if tc.wantCode != http.StatusOK {
				// A failing middle worker must not leak a usable proposal.
				var envelope contracts.MiddleWorkerResponse
				if err := json.Unmarshal(raw, &envelope); err != nil {
					t.Fatalf("decode MiddleWorkerResponse: %v: %s", err, raw)
				}
				if envelope.Proposal.Status != "" || len(envelope.Proposal.Actions) != 0 {
					t.Fatalf("failed scenario returned a proposal: %+v", envelope.Proposal)
				}
				return
			}
			var resp contracts.MiddleWorkerResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatalf("decode MiddleWorkerResponse: %v: %s", err, raw)
			}
			p := resp.Proposal
			if p.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", p.Status, tc.wantStatus)
			}
			if tc.wantIntent == "" {
				if p.Intent != nil {
					t.Fatalf("intent = %q, want null for status %q", *p.Intent, p.Status)
				}
			} else if p.Intent == nil || *p.Intent != tc.wantIntent {
				t.Fatalf("intent = %v, want %q", p.Intent, tc.wantIntent)
			}
			if tc.wantSpeech == "" {
				if p.SpeechKey != nil {
					t.Fatalf("speech_key = %q, want null for scenario %q", *p.SpeechKey, tc.sc)
				}
			} else if p.SpeechKey == nil || *p.SpeechKey != tc.wantSpeech {
				t.Fatalf("speech_key = %v, want %q", p.SpeechKey, tc.wantSpeech)
			}
			if !reflect.DeepEqual(p.Actions, tc.wantActions) {
				t.Fatalf("actions = %+v, want %+v", p.Actions, tc.wantActions)
			}
			if err := contracts.ValidateModelOutput(p, requestID, dataVer, known, enabled); err != nil {
				t.Fatalf("proposal fails contract validation: %v", err)
			}
		})
	}
}

// TestDestinationChoiceOffersFacilityChoices: the destination-choice scenario
// is the one that must present a facility on the map, so its single action
// must be SHOW_CHOICES naming the demo facility.
func TestDestinationChoiceOffersFacilityChoices(t *testing.T) {
	ts := serve(t, scenarioDestinationChoice)
	code, raw := postJSON(t, ts, "/v1/chat/completions", middleRequest("req-dest-1", "en-IN"))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", code, raw)
	}
	var resp contracts.MiddleWorkerResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode MiddleWorkerResponse: %v: %s", err, raw)
	}
	if len(resp.Proposal.Actions) != 1 {
		t.Fatalf("actions = %+v, want exactly one", resp.Proposal.Actions)
	}
	action := resp.Proposal.Actions[0]
	if action.Type != contracts.ActionShowChoices {
		t.Fatalf("action type = %q, want %q", action.Type, contracts.ActionShowChoices)
	}
	if !reflect.DeepEqual(action.TargetIDs, []string{mockFacilityID}) {
		t.Fatalf("target_ids = %v, want [%s]", action.TargetIDs, mockFacilityID)
	}
	if !slices.Contains(resp.Proposal.EvidenceIDs, mockFacilityID) {
		t.Fatalf("evidence_ids = %v, want the offered facility %s", resp.Proposal.EvidenceIDs, mockFacilityID)
	}
}

// TestWorkerFailureOnlyMiddleEndpointFails: the scenario fails the middle
// worker, and only the middle worker. ASR and TTS must keep answering, and the
// default scenario must still answer the middle route, so a demo that switches
// scenarios is not silently running a broken ASR or TTS.
func TestWorkerFailureOnlyMiddleEndpointFails(t *testing.T) {
	ts := serve(t, scenarioWorkerFailure)

	if code, raw := postJSON(t, ts, "/v1/chat/completions", middleRequest("req-fail-1", "en-IN")); code != http.StatusServiceUnavailable {
		t.Fatalf("middle status = %d, want 503: %s", code, raw)
	}
	if code, raw := postJSON(t, ts, "/transcribe", contracts.ASRWorkerRequest{
		RequestID:   "req-asr-1",
		Language:    "en-IN",
		ContentType: "audio/wav",
	}); code != http.StatusOK {
		t.Fatalf("ASR status = %d, want 200 under worker-failure: %s", code, raw)
	}
	if code, raw := postJSON(t, ts, "/synthesize", contracts.TTSWorkerRequest{
		RequestID: "req-tts-1",
		SpeechKey: "destination_options",
		Language:  "en-IN",
	}); code != http.StatusOK {
		t.Fatalf("TTS status = %d, want 200 under worker-failure: %s", code, raw)
	}

	ok := serve(t, scenarioDefault)
	if code, raw := postJSON(t, ok, "/v1/chat/completions", middleRequest("req-ok-1", "en-IN")); code != http.StatusOK {
		t.Fatalf("default middle status = %d, want 200: %s", code, raw)
	}
}
