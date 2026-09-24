package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/store"
)

// generateWAVBytes generates a minimal valid PCM 16-bit 16kHz mono WAV for testing.
func generateWAVBytes(durationMs int) []byte {
	sampleRate := 16000
	numSamples := sampleRate * durationMs / 1000
	dataSize := numSamples * 2
	fileSize := 36 + dataSize

	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(fileSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // mono
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(2))  // block align
	_ = binary.Write(buf, binary.LittleEndian, uint16(16)) // 16-bit
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataSize))

	for i := 0; i < numSamples; i++ {
		_ = binary.Write(buf, binary.LittleEndian, int16(100))
	}
	return buf.Bytes()
}

func seedVoicePackageFixture(t *testing.T, st *store.Store, jurisdiction, pkgID, facID, placeID string) {
	t.Helper()
	now := time.Now().UTC()
	srcID := "SRC-VOICE-" + pkgID
	authID := "AUTH-VOICE-" + pkgID
	artID := "ART-VOICE-" + pkgID
	szID := "SZ-VOICE-" + facID
	rzID := "RZ-VOICE-" + pkgID
	rtID := "RT-VOICE-" + pkgID

	body := fmt.Sprintf(`{
		"red_zones":[{"id":"%s"}],
		"safe_zones":[{"id":"%s","status":"OPEN"}],
		"approved_routes":[{"id":"%s","from_zone_id":"%s","to_safe_zone_id":"%s","mode":"FOOT","approval":"AUTHORIZED_OPERATIONAL","verified_by":"officer.voice","valid_from":"2026-01-01T00:00:00Z","valid_until":"2030-01-01T00:00:00Z"}],
		"facilities":[{"id":"%s","safe_zone_id":"%s"}],
		"instruction_assets":[{"id":"INS-1","language":"en-IN"},{"id":"INS-2","language":"hi-IN"},{"id":"INS-3","language":"ml-IN"}],
		"allocation_policy":{"order":["%s"]}
	}`, rzID, szID, rtID, rzID, szID, facID, szID, facID)

	ctx := context.Background()
	if err := st.InTx(ctx, func(tx store.DBTX) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at)
			VALUES ($1, 'gov-kerala', 'kerala.sdma.gov.in', 'OPERATIONAL', 1, $2)`,
			srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_authorizations
				(authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			authID, srcID, "authority-voice", "doc-voice", jurisdiction, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'AUTHORIZED_OPERATIONAL',$5)`,
			artID, srcID, strings.Repeat("b", 64), now, "memory://voice"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, checksum_sha256, body, effective_at, expires_at)
			VALUES ($1,$2,$3,$4,1,$5,'AUTHORIZED_OPERATIONAL',$6,$7,$8,$9)`,
			pkgID, "ALERT-"+pkgID, srcID, artID, jurisdiction, strings.Repeat("c", 64), []byte(body),
			now.Add(-time.Hour), now.Add(24*time.Hour)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO place_aliases (alias_id, jurisdiction, lookup_key, place_id, place_kind)
			VALUES ($1, $2, 'meppadi', $3, 'ZONE')`,
			"ALIAS-"+placeID, jurisdiction, placeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, version, updated_at)
			VALUES ($1, $2, 50, 5, 1, $3)`,
			facID, now.Format("2006-01-02"), now); err != nil {
			return err
		}
		// Approved translation authority (isolated fixture): the model's
		// "destination_options" speech_key is only servable because a
		// translation authority recorded a jurisdiction/source-version
		// bound approval here. Nothing in production synthesises un-
		// approved speech. B01: language, source_id and template_sha256
		// are required for an active approval row.
		destText := "Destination choices are displayed on screen."
		sum := sha256.Sum256([]byte(destText))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO approved_translations
				(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
			VALUES ($1,$2,$3,$4,1,1,$5,$6,$7,$8,$9)`,
			"APPROVE-"+pkgID+"-destination_options", jurisdiction, "destination_options", "en-IN",
			srcID, hex.EncodeToString(sum[:]),
			"translator-voice", "doc-voice-translate", now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seedVoicePackageFixture: %v", err)
	}
}

func primeWorkers(t *testing.T, w *orchestration.Workers) {
	t.Helper()
	ctx := context.Background()
	if w.ASR != nil {
		if _, err := w.SnapshotHealth(ctx, orchestration.StageASR); err != nil {
			t.Fatalf("SnapshotHealth ASR: %v", err)
		}
	}
	if w.Middle != nil {
		if _, err := w.SnapshotHealth(ctx, orchestration.StageMiddle); err != nil {
			t.Fatalf("SnapshotHealth Middle: %v", err)
		}
	}
	if w.TTS != nil {
		if _, err := w.SnapshotHealth(ctx, orchestration.StageTTS); err != nil {
			t.Fatalf("SnapshotHealth TTS: %v", err)
		}
	}
}

// TestVoiceProcess_RealHTTP_PersistedScopedContext_Pipeline exercises the end-to-end
// voice pipeline over real Go HTTP with a real PostgreSQL database and loopback worker servers.
func TestVoiceProcess_RealHTTP_PersistedScopedContext_Pipeline(t *testing.T) {
	dsn, cleanupDB := disposableTestDB(t)
	defer cleanupDB()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	jurisdiction := "KL"
	pkgID := "PKG-KL-MEPPADI-01"
	facID := "FAC-KL-MEPPADI-01"
	placeID := "PLACE-KL-MEPPADI-01"
	seedVoicePackageFixture(t, st, jurisdiction, pkgID, facID, placeID)

	var asrCalls atomic.Int64
	var middleCalls atomic.Int64
	var ttsCalls atomic.Int64

	// 1. Loopback ASR Server
	asrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asrCalls.Add(1)
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{
				Ready:              true,
				Warm:               true,
				SupportedLanguages: []string{"en-IN", "hi-IN", "ml-IN"},
			})
			return
		}
		if r.URL.Path == "/transcribe" {
			var req contracts.ASRWorkerRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(contracts.ASRWorkerResponse{
				RequestID:      req.RequestID,
				Language:       req.Language,
				Text:           "Meppadi",
				State:          contracts.TranscriptionOK,
				ModelRevision:  "asr-conformer-v1",
				ArtifactDigest: "sha256-asr-mock",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer asrServer.Close()

	// 2. Loopback Middle (vLLM) Server
	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		middleCalls.Add(1)
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{
				Ready:              true,
				Warm:               true,
				SupportedLanguages: []string{"en-IN", "hi-IN", "ml-IN"},
			})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			var req contracts.MiddleWorkerRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			intent := contracts.IntentFocusPlace
			sk := "destination_options"
			_ = json.NewEncoder(w).Encode(contracts.MiddleWorkerResponse{
				RequestID:   req.RequestID,
				DataVersion: req.ScopedContext.DataVersion,
				Proposal: contracts.ModelOutput{
					SchemaVersion: contracts.ModelSchemaVersion,
					RequestID:     req.RequestID,
					DataVersion:   req.ScopedContext.DataVersion,
					Status:        contracts.StatusOK,
					Intent:        &intent,
					Language:      "en-IN",
					Actions: []contracts.Action{
						{
							Type:     contracts.ActionFocusFeature,
							TargetID: placeID,
						},
					},
					SpeechKey:        &sk,
					ClarificationIDs: []string{},
					EvidenceIDs:      []string{},
				},
				ModelRevision: "middle-qwen3-4b-v1",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer middleServer.Close()

	// 3. Loopback TTS Server
	ttsAudioBytes := generateWAVBytes(500)
	ttsAudioB64 := base64.StdEncoding.EncodeToString(ttsAudioBytes)
	ttsHash := sha256.Sum256(ttsAudioBytes)
	ttsChecksum := hex.EncodeToString(ttsHash[:])
	ttsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ttsCalls.Add(1)
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{
				Ready:              true,
				Warm:               true,
				SupportedLanguages: []string{"en-IN", "hi-IN", "ml-IN"},
			})
			return
		}
		if r.URL.Path == "/synthesize" {
			var req contracts.TTSWorkerRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(contracts.TTSWorkerResponse{
				RequestID:      req.RequestID,
				SpeechKey:      req.SpeechKey,
				Language:       req.Language,
				State:          contracts.TTSOK,
				AudioB64:       ttsAudioB64,
				ContentType:    "audio/wav",
				ChecksumSHA256: ttsChecksum,
				ModelRevision:  "tts-parler-v1",
				VoiceRevision:  "voice-ml-1",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ttsServer.Close()

	// 4. Construct Orchestrator
	asrClient := orchestration.NewHTTPWorkerClient(asrServer.URL, "tok-asr", asrServer.Client())
	midClient := orchestration.NewHTTPWorkerClient(middleServer.URL, "tok-mid", middleServer.Client())
	ttsClient := orchestration.NewHTTPWorkerClient(ttsServer.URL, "tok-tts", ttsServer.Client())
	workers := orchestration.NewWorkers(asrClient, midClient, ttsClient)
	primeWorkers(t, workers)

	resolver := store.NewScopedContextResolver(st)
	validator := orchestration.NewProductionValidator()
	// Isolated approval fixture: a NON-synthetic, jurisdiction/source/
	// version-bound template mirroring the approved_translations row
	// seeded above. DefaultTemplateRegistry is synthetic-only and the
	// operational path correctly refuses to synthesise it (fail closed),
	// so the citizen happy path must carry an actually-approved template.
	templateRegistry := orchestration.NewMapTemplateRegistry()
	templateRegistry.Add(contracts.ApprovedTemplate{
		SpeechKey:       "destination_options",
		Language:        "en-IN",
		TemplateVersion: 1,
		SourceVersion:   1,
		Text:            "Destination choices are displayed on screen.",
		SyntheticOnly:   false,
	})

	orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
		Limits:    orchestration.DefaultLimits(),
		Workers:   workers,
		Resolver:  resolver,
		Validator: validator,
		Templates: templateRegistry,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}

	voiceHandler := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	appServer := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithVoiceProcess(voiceHandler))
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	// Initial check on database row counts: zero consequential writes
	checkZeroConsequentialWrites(t, st)

	// A. Send Audio Request to /api/v3/voice/process
	wavData := generateWAVBytes(500)
	audioB64 := base64.StdEncoding.EncodeToString(wavData)
	reqBody := fmt.Sprintf(`{
		"request_id": "req-voice-full-01",
		"jurisdiction": "%s",
		"language": "en-IN",
		"input": {
			"kind": "audio",
			"body_b64": "%s",
			"content_type": "audio/wav"
		},
		"render": {
			"kind": "tts"
		}
	}`, jurisdiction, audioB64)

	resp, err := ts.Client().Post(ts.URL+"/api/v3/voice/process", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST /api/v3/voice/process: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/v3/voice/process returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Data   contracts.PipelineResponse `json:"data"`
		Errors []contracts.APIError       `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if env.Data.State != contracts.PipelineOK {
		t.Errorf("pipeline state = %v, want OK", env.Data.State)
	}
	if env.Data.ValidatedProposal.Intent == nil || *env.Data.ValidatedProposal.Intent != contracts.IntentFocusPlace {
		t.Errorf("proposal intent = %+v, want FOCUS_PLACE", env.Data.ValidatedProposal.Intent)
	}
	if len(env.Data.ValidatedProposal.Actions) != 1 || env.Data.ValidatedProposal.Actions[0].TargetID != placeID {
		t.Errorf("proposal action target_id = %+v, want %s", env.Data.ValidatedProposal.Actions, placeID)
	}
	if env.Data.Audio == nil || env.Data.Audio.ByteSize == 0 {
		t.Errorf("expected audio in response, got %+v", env.Data.Audio)
	}

	// Verify that workers were actually invoked
	if asrCalls.Load() == 0 || middleCalls.Load() == 0 || ttsCalls.Load() == 0 {
		t.Errorf("expected all workers to be invoked; asr=%d, mid=%d, tts=%d",
			asrCalls.Load(), middleCalls.Load(), ttsCalls.Load())
	}

	// Verify zero consequential writes in DB after pipeline completion
	checkZeroConsequentialWrites(t, st)
}

// TestVoiceProcess_RealHTTP_StaleSnapshotDuringInference verifies that changing
// or withdrawing the package while inference is running causes SnapshotRevalidate
// to fail closed with 409 STALE_SNAPSHOT, suppressing TTS and consequential writes.
func TestVoiceProcess_RealHTTP_StaleSnapshotDuringInference(t *testing.T) {
	dsn, cleanupDB := disposableTestDB(t)
	defer cleanupDB()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	jurisdiction := "KL"
	pkgID := "PKG-KL-STALE-01"
	facID := "FAC-KL-STALE-01"
	placeID := "PLACE-KL-STALE-01"
	seedVoicePackageFixture(t, st, jurisdiction, pkgID, facID, placeID)

	var ttsInvoked atomic.Bool

	// Middle worker that modifies the package in the database before returning its proposal
	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{Ready: true, Warm: true, SupportedLanguages: []string{"en-IN"}})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			var req contracts.MiddleWorkerRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			// Concurrently withdraw the package in the database to simulate mid-inference supersession
			ctx := context.Background()
			_, _ = st.DB().ExecContext(ctx, `UPDATE packages SET expires_at = NOW() - INTERVAL '1 hour' WHERE package_id = $1`, pkgID)

			intent := contracts.IntentFocusPlace
			sk := "destination_options"
			_ = json.NewEncoder(w).Encode(contracts.MiddleWorkerResponse{
				RequestID:   req.RequestID,
				DataVersion: req.ScopedContext.DataVersion,
				Proposal: contracts.ModelOutput{
					SchemaVersion: contracts.ModelSchemaVersion,
					RequestID:     req.RequestID,
					DataVersion:   req.ScopedContext.DataVersion,
					Status:        contracts.StatusOK,
					Intent:        &intent,
					Language:      "en-IN",
					Actions:       []contracts.Action{{Type: contracts.ActionFocusFeature, TargetID: placeID}},
					SpeechKey:     &sk,
				},
				ModelRevision: "middle-qwen3-4b-v1",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer middleServer.Close()

	ttsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{Ready: true, Warm: true, SupportedLanguages: []string{"en-IN"}})
			return
		}
		if r.URL.Path == "/synthesize" {
			ttsInvoked.Store(true)
			_ = json.NewEncoder(w).Encode(contracts.TTSWorkerResponse{State: contracts.TTSOK})
			return
		}
		http.NotFound(w, r)
	}))
	defer ttsServer.Close()

	midClient := orchestration.NewHTTPWorkerClient(middleServer.URL, "tok-mid", middleServer.Client())
	ttsClient := orchestration.NewHTTPWorkerClient(ttsServer.URL, "tok-tts", ttsServer.Client())
	workers := orchestration.NewWorkers(nil, midClient, ttsClient)
	primeWorkers(t, workers)

	resolver := store.NewScopedContextResolver(st)
	validator := orchestration.NewProductionValidator()
	templateRegistry := orchestration.DefaultTemplateRegistry()

	orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
		Limits:    orchestration.DefaultLimits(),
		Workers:   workers,
		Resolver:  resolver,
		Validator: validator,
		Templates: templateRegistry,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}

	voiceHandler := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	appServer := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithVoiceProcess(voiceHandler))
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	reqBody := fmt.Sprintf(`{
		"request_id": "req-stale-01",
		"jurisdiction": "%s",
		"language": "en-IN",
		"input": {
			"kind": "transcript",
			"text": "Meppadi"
		},
		"render": {
			"kind": "tts"
		}
	}`, jurisdiction)

	resp, err := ts.Client().Post(ts.URL+"/api/v3/voice/process", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected HTTP 409 Conflict on stale snapshot, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env contracts.Envelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrStaleSnapshot {
		t.Errorf("expected STALE_SNAPSHOT error code, got %+v", env.Errors)
	}

	if ttsInvoked.Load() {
		t.Errorf("TTS was invoked despite stale snapshot")
	}

	checkZeroConsequentialWrites(t, st)
}

// TestVoiceProcess_RealHTTP_CancellationDuringInference verifies that cancelling
// the client request aborts downstream inference without consequential writes.
func TestVoiceProcess_RealHTTP_CancellationDuringInference(t *testing.T) {
	dsn, cleanupDB := disposableTestDB(t)
	defer cleanupDB()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	jurisdiction := "KL"
	pkgID := "PKG-KL-CANCEL-01"
	facID := "FAC-KL-CANCEL-01"
	placeID := "PLACE-KL-CANCEL-01"
	seedVoicePackageFixture(t, st, jurisdiction, pkgID, facID, placeID)

	blockMiddle := make(chan struct{})
	middleStarted := make(chan struct{})

	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{Ready: true, Warm: true, SupportedLanguages: []string{"en-IN"}})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			select {
			case <-middleStarted:
			default:
				close(middleStarted)
			}
			select {
			case <-r.Context().Done():
				return
			case <-blockMiddle:
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer middleServer.Close()
	defer close(blockMiddle)

	midClient := orchestration.NewHTTPWorkerClient(middleServer.URL, "tok-mid", middleServer.Client())
	workers := orchestration.NewWorkers(nil, midClient, nil)
	primeWorkers(t, workers)

	orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
		Limits:    orchestration.DefaultLimits(),
		Workers:   workers,
		Resolver:  store.NewScopedContextResolver(st),
		Validator: orchestration.NewProductionValidator(),
		Templates: orchestration.DefaultTemplateRegistry(),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}

	voiceHandler := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	appServer := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithVoiceProcess(voiceHandler))
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	reqBody := fmt.Sprintf(`{
		"request_id": "req-cancel-01",
		"jurisdiction": "%s",
		"language": "en-IN",
		"input": {
			"kind": "transcript",
			"text": "Meppadi"
		}
	}`, jurisdiction)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/api/v3/voice/process", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	errCh := make(chan error, 1)
	go func() {
		resp, err := ts.Client().Do(httpReq)
		if err != nil {
			errCh <- err
			return
		}
		resp.Body.Close()
		errCh <- nil
	}()

	<-middleStarted
	cancel() // Cancel request while middle worker is parked

	<-errCh

	checkZeroConsequentialWrites(t, st)
}

// TestVoiceProcess_RealHTTP_QueueSaturation verifies that saturating the admission queue
// immediately rejects surplus requests with HTTP 503 QUEUE_SATURATED.
func TestVoiceProcess_RealHTTP_QueueSaturation(t *testing.T) {
	limits := orchestration.DefaultLimits()
	limits.MiddleMaxInflight = 1
	limits.MiddleQueueDepth = 1

	parkChan := make(chan struct{})

	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{Ready: true, Warm: true, SupportedLanguages: []string{"en-IN"}})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			select {
			case <-parkChan:
			case <-r.Context().Done():
			}
			intent := contracts.IntentFocusPlace
			_ = json.NewEncoder(w).Encode(contracts.MiddleWorkerResponse{
				RequestID:   "req-sat",
				DataVersion: "dv-test",
				Proposal: contracts.ModelOutput{
					SchemaVersion: contracts.ModelSchemaVersion,
					RequestID:     "req-sat",
					DataVersion:   "dv-test",
					Status:        contracts.StatusOK,
					Intent:        &intent,
					Language:      "en-IN",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer middleServer.Close()

	midClient := orchestration.NewHTTPWorkerClient(middleServer.URL, "tok-mid", middleServer.Client())
	workers := orchestration.NewWorkers(nil, midClient, nil)
	primeWorkers(t, workers)

	// Resolver using fake static context
	destText := "Destination choices are displayed on screen."
	dsum := sha256.Sum256([]byte(destText))
	sc := contracts.ScopedContext{
		DataVersion:      "dv-test",
		Jurisdiction:     "KL",
		AllowedLanguages: []string{"en-IN"},
		KnownPlaces: map[string]contracts.PlaceCandidate{
			"P1": {PlaceID: "P1", PlaceKind: "VILLAGE", Jurisdiction: "KL"},
		},
		TemplateKeys: []string{"destination_options"},
		ApprovedSpeechKeys: map[string][]string{
			"destination_options": {"en-IN"},
		},
		ApprovedTemplateSHA: map[string]string{
			"destination_options": hex.EncodeToString(dsum[:]),
		},
	}
	resolver := &testStaticResolver{sc: sc}

	orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
		Limits:    limits,
		Workers:   workers,
		Resolver:  resolver,
		Validator: orchestration.NewProductionValidator(),
		Templates: orchestration.DefaultTemplateRegistry(),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}

	voiceHandler := NewVoiceProcessHandler(orch, limits)
	appServer := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(voiceHandler))
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	// Launch requests concurrently
	var gotSaturated atomic.Bool
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"request_id":"req-sat-%d","jurisdiction":"KL","language":"en-IN","input":{"kind":"transcript","text":"P1"}}`, idx)
			resp, err := ts.Client().Post(ts.URL+"/api/v3/voice/process", "application/json", strings.NewReader(body))
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusServiceUnavailable {
					var env contracts.Envelope
					_ = json.NewDecoder(resp.Body).Decode(&env)
					if len(env.Errors) > 0 && env.Errors[0].Code == contracts.ErrQueueSaturated {
						gotSaturated.Store(true)
					}
				}
			}
		}(i)
	}
	time.Sleep(100 * time.Millisecond)
	close(parkChan)
	wg.Wait()

	if !gotSaturated.Load() {
		t.Errorf("expected at least one request to receive HTTP 503 QUEUE_SATURATED under load")
	}
}

// TestVoiceProcess_RealHTTP_DirectTextFallback_ModelDown proves that citizen text/touch
// guidance endpoints operate independently of model health, even when model workers are down.
func TestVoiceProcess_RealHTTP_DirectTextFallback_ModelDown(t *testing.T) {
	dsn, cleanupDB := disposableTestDB(t)
	defer cleanupDB()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	jurisdiction := "KL"
	pkgID := "PKG-KL-TEXTFALLBACK-01"
	facID := "FAC-KL-TEXTFALLBACK-01"
	placeID := "PLACE-KL-TEXTFALLBACK-01"
	seedVoicePackageFixture(t, st, jurisdiction, pkgID, facID, placeID)

	// No model workers configured (model is completely down/absent)
	appServer := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithSyntheticExercise(StaticSyntheticExercise(true)))
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	// 1. Voice pipeline returns 503 MODEL_UNAVAILABLE because workers are not configured
	voiceReq := `{"request_id":"req-model-down","jurisdiction":"KL","language":"en-IN","input":{"kind":"transcript","text":"Meppadi"}}`
	vResp, err := ts.Client().Post(ts.URL+"/api/v3/voice/process", "application/json", strings.NewReader(voiceReq))
	if err != nil {
		t.Fatalf("POST voice: %v", err)
	}
	defer vResp.Body.Close()
	if vResp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("voice with model down = %d, want 503", vResp.StatusCode)
	}

	// 2. Authoritative direct text/touch endpoint: /api/v3/places/resolve remains 100% OPERATIONAL
	placeReq := `{"query":"Meppadi","jurisdiction":"KL"}`
	pResp, err := ts.Client().Post(ts.URL+"/api/v3/places/resolve", "application/json", strings.NewReader(placeReq))
	if err != nil {
		t.Fatalf("POST places/resolve: %v", err)
	}
	defer pResp.Body.Close()
	if pResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pResp.Body)
		t.Errorf("places/resolve = %d, want 200; body=%s", pResp.StatusCode, string(body))
	}

	// 3. Authoritative direct guidance query: /api/v3/guidance/query remains 100% OPERATIONAL
	today := time.Now().UTC().Format("2006-01-02")
	tomorrow := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	guidanceReq := fmt.Sprintf(`{"jurisdiction":"%s","package_id":"%s","party_size":1,"start_date":"%s","end_date":"%s"}`,
		jurisdiction, pkgID, today, tomorrow)
	gResp, err := ts.Client().Post(ts.URL+"/api/v3/guidance/query", "application/json", strings.NewReader(guidanceReq))
	if err != nil {
		t.Fatalf("POST guidance/query: %v", err)
	}
	defer gResp.Body.Close()
	if gResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(gResp.Body)
		t.Errorf("guidance/query = %d, want 200; body=%s", gResp.StatusCode, string(body))
	}

	checkZeroConsequentialWrites(t, st)
}

func checkZeroConsequentialWrites(t *testing.T, st *store.Store) {
	t.Helper()
	var stayCount int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT count(*) FROM stays`).Scan(&stayCount); err != nil {
		t.Fatalf("check stays: %v", err)
	}
	if stayCount != 0 {
		t.Errorf("consequential write detected: stays count = %d, want 0", stayCount)
	}

	var resCount int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT count(*) FROM reservations`).Scan(&resCount); err != nil {
		t.Fatalf("check reservations: %v", err)
	}
	if resCount != 0 {
		t.Errorf("consequential write detected: reservations count = %d, want 0", resCount)
	}

	var auditCount int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT count(*) FROM audit_events`).Scan(&auditCount); err != nil {
		t.Fatalf("check audit_events: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("consequential write detected: audit_events count = %d, want 0", auditCount)
	}
}

type testStaticResolver struct {
	sc contracts.ScopedContext
}

func (r *testStaticResolver) Resolve(ctx context.Context, jurisdiction string) (contracts.ScopedContext, error) {
	return r.sc, nil
}

func (r *testStaticResolver) SnapshotRevalidate(ctx context.Context, sc contracts.ScopedContext) error {
	return nil
}
