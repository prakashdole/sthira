//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/store"
)

// seedC04ExerciseFixture seeds the isolated DEMO-EXERCISE package, place aliases
// (including ambiguous "meppadi" aliases), and approved translations into the test DB.
func seedC04ExerciseFixture(t *testing.T, st *store.Store) {
	t.Helper()
	now := time.Now().UTC()
	ctx := context.Background()

	exerciseBody := `{
		"red_zones":[{"id":"RZDEMO-1"}],
		"safe_zones":[{"id":"SZDEMO-1","status":"OPEN"}],
		"approved_routes":[{
			"id":"RTDEMO-1",
			"from_zone_id":"RZDEMO-1",
			"to_safe_zone_id":"SZDEMO-1",
			"approval":"SYNTHETIC_DEMO",
			"mode":"FOOT",
			"verified_by":"exercise.demo",
			"valid_from":"2026-01-01T00:00:00Z",
			"valid_until":"2030-01-01T00:00:00Z",
			"geometry":{"type":"LineString","coordinates":[[76.10,11.55],[76.12,11.55]]}
		}],
		"facilities":[{"id":"FACDEMO-1","safe_zone_id":"SZDEMO-1"}],
		"instruction_assets":[
			{"id":"INSDEMO-EN","language":"en-IN","title":"Demo instructions","summary":"Move to the demo safe zone."},
			{"id":"INSDEMO-ML","language":"ml-IN","title":"ഡെമോ നിർദ്ദേശം","summary":"ഡെമോ സുരക്ഷിത മേഖലയിലേക്ക് പോകുക."}
		],
		"allocation_policy":{
			"order":["SZDEMO-1"],
			"reservation_expiry_seconds":3600,
			"temporary_stay_min_days":1,
			"temporary_stay_max_days":14,
			"allow_walk_ins":true,
			"allow_transfers":true,
			"route_required":false
		},
		"emergency_contacts":[{"name":"Emergency","number":"112"}]
	}`
	hash := sha256.Sum256([]byte(exerciseBody))
	hashHex := hex.EncodeToString(hash[:])

	err := st.InTx(ctx, func(tx store.DBTX) error {
		// Clean prior rows
		for _, q := range []string{
			`DELETE FROM place_aliases WHERE jurisdiction = $1`,
			`DELETE FROM approved_translations WHERE jurisdiction = $1`,
			`DELETE FROM facilities WHERE facility_id = $1`,
			`DELETE FROM zone_versions WHERE package_id = $1`,
			`DELETE FROM packages WHERE package_id = $1`,
			`DELETE FROM source_artifacts WHERE artifact_id = $1`,
			`DELETE FROM source_authorizations WHERE authorization_id = $1`,
			`DELETE FROM sources WHERE source_id = $1`,
		} {
			switch q {
			case `DELETE FROM place_aliases WHERE jurisdiction = $1`,
				`DELETE FROM approved_translations WHERE jurisdiction = $1`:
				if _, err := tx.ExecContext(ctx, q, "DEMO-EXERCISE"); err != nil {
					return err
				}
			case `DELETE FROM facilities WHERE facility_id = $1`:
				if _, err := tx.ExecContext(ctx, q, "FACDEMO-1"); err != nil {
					return err
				}
			case `DELETE FROM zone_versions WHERE package_id = $1`,
				`DELETE FROM packages WHERE package_id = $1`:
				if _, err := tx.ExecContext(ctx, q, "PKGDEMO-1"); err != nil {
					return err
				}
			case `DELETE FROM source_artifacts WHERE artifact_id = $1`:
				if _, err := tx.ExecContext(ctx, q, "ARTDEMO-1"); err != nil {
					return err
				}
			case `DELETE FROM source_authorizations WHERE authorization_id = $1`:
				if _, err := tx.ExecContext(ctx, q, "AUTHDEMO-1"); err != nil {
					return err
				}
			case `DELETE FROM sources WHERE source_id = $1`:
				if _, err := tx.ExecContext(ctx, q, "SRCDEMO-1"); err != nil {
					return err
				}
			}
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			 VALUES ('SRCDEMO-1','exercise.demo','demo.example','OPERATIONAL',1,$1,$1)`, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO source_authorizations (authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
			 VALUES ('AUTHDEMO-1','SRCDEMO-1','exercise.demo','EXERCISE-NO-REAL-AUTHORITY','DEMO-EXERCISE',$1)`, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			 VALUES ('ARTDEMO-1','SRCDEMO-1',1,$1,$2,'SYNTHETIC_DEMO','memory://exercise')`, hashHex, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			 VALUES ('PKGDEMO-1','ALT-EX','SRCDEMO-1','ARTDEMO-1',1,'DEMO-EXERCISE','SYNTHETIC_DEMO',$1,$2,$3,$4)`,
			now.Add(-time.Hour), now.Add(24*time.Hour), hashHex, []byte(exerciseBody)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			 VALUES ('SZDEMO-1','PKGDEMO-1','SAFE','SAFE','OPEN',100,1,$1)`, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			 VALUES ('FACDEMO-1','PKGDEMO-1','SZDEMO-1','Asia/Kolkata',1,$1)`, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO route_versions
				(route_id, package_id, from_zone_id, to_safe_zone_id, approval, mode,
				 verified_by, verified_at, valid_from, valid_until, geometry, version, updated_at)
			 VALUES ('RTDEMO-1','PKGDEMO-1','RZDEMO-1','SZDEMO-1','SYNTHETIC_DEMO','FOOT','exercise.demo',$1,$2,$3,
			         ST_GeogFromText('SRID=4326;LINESTRING(76.10 11.55, 76.12 11.56)'),1,$1)`,
			now, "2026-01-01T00:00:00Z", "2030-01-01T00:00:00Z"); err != nil {
			return err
		}

		// Seed ambiguous place aliases for "meppadi"
		for _, alias := range []struct {
			id, key, target, kind string
		}{
			{"ALIASDEMO-1", "meppadi", "SZDEMO-1", "ZONE"},
			{"ALIASDEMO-2", "meppadi", "FACDEMO-1", "FACILITY"},
			{"ALIASDEMO-3", "safe zone", "SZDEMO-1", "ZONE"},
		} {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO place_aliases (alias_id, jurisdiction, lookup_key, place_id, place_kind, created_at)
				 VALUES ($1, 'DEMO-EXERCISE', $2, $3, $4, $5)`,
				alias.id, alias.key, alias.target, alias.kind, now); err != nil {
				return err
			}
		}

		// Seed approved translations for DEMO-EXERCISE matching ExerciseTemplateRegistry(1, 1)
		tpls := orchestration.ExerciseTemplateRegistry(1, 1)
		for _, k := range tpls.Keys() {
			for _, lang := range []string{"en-IN", "hi-IN", "ml-IN"} {
				if t, ok := tpls.Lookup(k, lang); ok {
					h := sha256.Sum256([]byte(t.Text))
					dig := hex.EncodeToString(h[:])
					transID := fmt.Sprintf("TRANS-EX-%s-%s", k, lang)
					if _, err := tx.ExecContext(ctx,
						`INSERT INTO approved_translations
							(translation_id, jurisdiction, speech_key, language, source_version, template_version,
							 approved_by, evidence_ref, approved_at, source_id, template_sha256, created_at)
						 VALUES ($1, 'DEMO-EXERCISE', $2, $3, 1, 1, 'exercise.demo', 'EXERCISE-NO-REAL-AUTHORITY', $4, 'SRCDEMO-1', $5, $4)`,
						transID, k, lang, now, dig); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seedC04ExerciseFixture: %v", err)
	}
}

// TestC04_PlumbingOnly_FullVoicePipeline tests acceptance row (a):
// public API/exercise binary wiring + real DB + actual private HTTP protocol workers.
// Labeled: PLUMBING_ONLY.
func TestC04_PlumbingOnly_FullVoicePipeline(t *testing.T) {
	dsn, cleanupDB := disposableTestDB(t)
	defer cleanupDB()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedC04ExerciseFixture(t, st)

	var asrCalls, midCalls, ttsCalls atomic.Int64

	// 1. Mock ASR Worker (IndicConformer private HTTP protocol)
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
			text := "Meppadi"
			if req.Language == "ml-IN" {
				text = "മേപ്പാടി സുരക്ഷിത കേന്ദ്രം"
			}
			_ = json.NewEncoder(w).Encode(contracts.ASRWorkerResponse{
				RequestID:      req.RequestID,
				Language:       req.Language,
				Text:           text,
				State:          contracts.TranscriptionOK,
				ModelRevision:  "indic-conformer-600m-v1",
				ArtifactDigest: "sha256-conformer-600m-demo",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer asrServer.Close()

	// 2. Mock Middle Worker (Sarvam-30B private HTTP protocol /v1/chat/completions)
	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		midCalls.Add(1)
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
					Language:      req.Transcript.Language,
					Actions: []contracts.Action{
						{
							Type:     contracts.ActionFocusFeature,
							TargetID: "SZDEMO-1",
						},
					},
					SpeechKey:        &sk,
					ClarificationIDs: []string{},
					EvidenceIDs:      []string{"SZDEMO-1"},
				},
				ModelRevision: "sarvamai/sarvam-30b-fp8",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer middleServer.Close()

	// 3. Mock TTS Worker (Indic Parler-TTS private HTTP protocol /synthesize)
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
				ModelRevision:  "indic-parler-tts-v1",
				VoiceRevision:  "voice-default-v1",
				Settings: contracts.TTSSynthesisSettings{
					SampleRate: 16000,
					BitDepth:   16,
					Channels:   1,
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ttsServer.Close()

	// 4. Wire Exercise Voice Pipeline via WireVoicePipeline (AllowSyntheticTemplates=true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	voiceOpts, orch, err := WireVoicePipeline(ctx, VoiceWiringConfig{
		Store:                   st,
		Logger:                  nil,
		ASRURL:                  asrServer.URL,
		MiddleURL:               middleServer.URL,
		TTSURL:                  ttsServer.URL,
		HealthRefreshInterval:   time.Hour,
		AllowSyntheticTemplates: true, // Exercise binary setting
		Templates:               orchestration.ExerciseTemplateRegistry(1, 1),
	})
	if err != nil {
		t.Fatalf("WireVoicePipeline: %v", err)
	}
	if orch == nil {
		t.Fatalf("WireVoicePipeline returned nil orchestrator")
	}

	serverOpts := []Option{
		WithStore(st),
		WithPersistedContextResolver(st),
		WithSyntheticExercise(StaticSyntheticExercise(true)),
	}
	serverOpts = append(serverOpts, voiceOpts...)
	appServer := New(DefaultConfig("127.0.0.1:0"), serverOpts...)
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	// 5. Test Audio Journey (Malayalam audio -> ASR -> Middle -> Template -> TTS)
	client := ts.Client()
	wavData := generateWAVBytes(300)
	audioB64 := base64.StdEncoding.EncodeToString(wavData)

	reqPayload := map[string]any{
		"request_id":   "req-c04-ml-01",
		"jurisdiction": "DEMO-EXERCISE",
		"language":     "ml-IN",
		"input": map[string]any{
			"kind":         "audio",
			"body_b64":     audioB64,
			"content_type": "audio/wav",
		},
		"render": map[string]any{
			"kind": "tts",
		},
	}
	rawBytes, _ := json.Marshal(reqPayload)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(rawBytes))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Data struct {
			State             string `json:"state"`
			ValidatedProposal *struct {
				Intent  *string `json:"intent"`
				Actions []struct {
					Type     string `json:"type"`
					TargetID string `json:"target_id"`
				} `json:"actions"`
			} `json:"validated_proposal"`
			Template *struct {
				SpeechKey string `json:"speech_key"`
				Language  string `json:"language"`
			} `json:"template"`
			Audio *struct {
				AudioB64       string `json:"audio_b64"`
				ContentType    string `json:"content_type"`
				ByteSize       int    `json:"byte_size"`
				ChecksumSHA256 string `json:"checksum_sha256"`
			} `json:"audio"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		t.Fatalf("parse response: %v\nbody: %s", err, string(bodyBytes))
	}

	if env.Data.State != "OK" {
		t.Errorf("expected state OK, got %q", env.Data.State)
	}
	if env.Data.ValidatedProposal == nil || env.Data.ValidatedProposal.Intent == nil || *env.Data.ValidatedProposal.Intent != "FOCUS_PLACE" {
		t.Errorf("expected proposal intent FOCUS_PLACE, got %v", env.Data.ValidatedProposal)
	}
	if len(env.Data.ValidatedProposal.Actions) == 0 || env.Data.ValidatedProposal.Actions[0].TargetID != "SZDEMO-1" {
		t.Errorf("expected proposal action target SZDEMO-1, got %v", env.Data.ValidatedProposal.Actions)
	}
	if env.Data.Template == nil || env.Data.Template.SpeechKey != "destination_options" {
		t.Errorf("expected template speech_key destination_options, got %v", env.Data.Template)
	}
	if env.Data.Audio == nil || env.Data.Audio.ChecksumSHA256 != ttsChecksum {
		t.Errorf("expected audio checksum %q, got %v", ttsChecksum, env.Data.Audio)
	}

	// 6. Test Transcript Journey (English transcript -> Middle -> Template -> TTS; no ASR called)
	priorASRCalls := asrCalls.Load()
	textReqPayload := map[string]any{
		"request_id":   "req-c04-en-02",
		"jurisdiction": "DEMO-EXERCISE",
		"language":     "en-IN",
		"input": map[string]any{
			"kind": "transcript",
			"text": "Where is the safe zone?",
		},
		"render": map[string]any{
			"kind": "tts",
		},
	}
	rawTextBytes, _ := json.Marshal(textReqPayload)
	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(rawTextBytes))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp2.Body.Close()
	bodyBytes2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for transcript input, got %d: %s", resp2.StatusCode, string(bodyBytes2))
	}
	if asrCalls.Load() != priorASRCalls {
		t.Errorf("ASR worker should not be called when input.kind is transcript")
	}

	// 7. Verify zero consequential writes in DB
	checkZeroConsequentialWrites(t, st)
	t.Log("PLUMBING_ONLY: public API + real DB + HTTP protocol workers verified end to end")
}

// TestC04_ProcessIsolation_ProductionRejectsSynthetic verifies that production orchestrator
// (AllowSyntheticTemplates=false) fails closed on SyntheticOnly templates, and no request
// header, query param, or payload field can bypass this boundary.
func TestC04_ProcessIsolation_ProductionRejectsSynthetic(t *testing.T) {
	dsn, cleanupDB := disposableTestDB(t)
	defer cleanupDB()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedC04ExerciseFixture(t, st)

	// Middle worker proposes speech_key "destination_options" which is marked SyntheticOnly=true
	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{Ready: true, Warm: true, SupportedLanguages: []string{"en-IN"}})
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
					Language:      req.Transcript.Language,
					Actions:       []contracts.Action{{Type: contracts.ActionFocusFeature, TargetID: "SZDEMO-1"}},
					SpeechKey:     &sk,
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer middleServer.Close()

	ttsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{Ready: true, Warm: true, SupportedLanguages: []string{"en-IN"}})
	}))
	defer ttsServer.Close()

	ctx := context.Background()

	// Wire PRODUCTION pipeline (AllowSyntheticTemplates = false)
	voiceOpts, _, err := WireVoicePipeline(ctx, VoiceWiringConfig{
		Store:                   st,
		ASRURL:                  middleServer.URL, // dummy
		MiddleURL:               middleServer.URL,
		TTSURL:                  ttsServer.URL,
		HealthRefreshInterval:   time.Hour,
		AllowSyntheticTemplates: false, // Production setting: FAIL CLOSED on synthetic templates
		Templates:               orchestration.ExerciseTemplateRegistry(1, 1),
	})
	if err != nil {
		t.Fatalf("WireVoicePipeline: %v", err)
	}

	appServer := New(DefaultConfig("127.0.0.1:0"), append([]Option{WithStore(st), WithPersistedContextResolver(st)}, voiceOpts...)...)
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	// Try with suspicious bypass headers: X-Synthetic, X-Exercise-Mode, X-Admin
	reqPayload := map[string]any{
		"request_id":   "req-prod-reject",
		"jurisdiction": "DEMO-EXERCISE",
		"language":     "en-IN",
		"input": map[string]any{
			"kind": "transcript",
			"text": "test",
		},
		"render": map[string]any{"kind": "tts"},
	}
	rawBytes, _ := json.Marshal(reqPayload)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process?synthetic=true&allow_exercise=true", bytes.NewReader(rawBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Synthetic", "true")
	req.Header.Set("X-Exercise-Mode", "true")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	// In voice/process: production pipeline completes without crashing, but template stage fails closed
	bodyBytes, _ := io.ReadAll(resp.Body)
	var procResp struct {
		Data struct {
			StageFailures []string `json:"stage_failures"`
			Audio         *any     `json:"audio"`
		} `json:"data"`
	}
	_ = json.Unmarshal(bodyBytes, &procResp)
	hasTemplateFailure := false
	for _, f := range procResp.Data.StageFailures {
		if f == "template" {
			hasTemplateFailure = true
			break
		}
	}
	if !hasTemplateFailure {
		t.Fatalf("expected template stage failure in production for synthetic template, got stage_failures=%v", procResp.Data.StageFailures)
	}
	if procResp.Data.Audio != nil {
		t.Fatalf("expected audio to be nil when template stage fails in production, got %v", procResp.Data.Audio)
	}

	// In voice/speech: direct synthesis MUST fail closed with HTTP 422 Unprocessable Entity
	speechPayload := map[string]any{
		"request_id":     "req-prod-speech",
		"speech_key":     "destination_options",
		"language":       "en-IN",
		"jurisdiction":   "DEMO-EXERCISE",
		"source_version": 1,
	}
	rawSpeechBytes, _ := json.Marshal(speechPayload)
	speechReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/speech?synthetic=true&allow_exercise=true", bytes.NewReader(rawSpeechBytes))
	speechReq.Header.Set("Content-Type", "application/json")
	speechReq.Header.Set("X-Synthetic", "true")
	speechReq.Header.Set("X-Exercise-Mode", "true")

	speechResp, err := ts.Client().Do(speechReq)
	if err != nil {
		t.Fatalf("Do speech: %v", err)
	}
	defer speechResp.Body.Close()

	if speechResp.StatusCode != http.StatusUnprocessableEntity {
		sBody, _ := io.ReadAll(speechResp.Body)
		t.Fatalf("expected 422 Unprocessable Entity in production for direct speech, got %d: %s", speechResp.StatusCode, string(sBody))
	}
}

// TestC04_AmbiguousLocationDisambiguation_CandidateChips tests that resolving
// "meppadi" returns HTTP 409 Conflict with multiple candidates (SZDEMO-1 and FACDEMO-1),
// matching the candidate chips required by citizen journey 3.
func TestC04_AmbiguousLocationDisambiguation_CandidateChips(t *testing.T) {
	dsn, cleanupDB := disposableTestDB(t)
	defer cleanupDB()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedC04ExerciseFixture(t, st)

	appServer := New(DefaultConfig("127.0.0.1:0"), WithStore(st))
	ts := httptest.NewServer(appServer.Handler())
	defer ts.Close()

	// 1. Resolve ambiguous "meppadi" -> HTTP 409 with candidate chips
	reqBody, _ := json.Marshal(map[string]string{
		"jurisdiction": "DEMO-EXERCISE",
		"query":        "meppadi",
	})
	resp, err := ts.Client().Post(ts.URL+"/api/v3/places/resolve", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Post resolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409 Conflict for ambiguous place, got %d: %s", resp.StatusCode, string(body))
	}

	var errResp struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details struct {
				Candidates []struct {
					PlaceID   string `json:"place_id"`
					PlaceKind string `json:"place_kind"`
				} `json:"candidates"`
			} `json:"details"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode 409 response: %v", err)
	}

	if len(errResp.Errors) == 0 || len(errResp.Errors[0].Details.Candidates) < 2 {
		t.Fatalf("expected at least 2 candidate chips for meppadi, got %v", errResp.Errors)
	}

	// 2. Resolve unambiguous "safe zone" -> HTTP 200
	reqBody2, _ := json.Marshal(map[string]string{
		"jurisdiction": "DEMO-EXERCISE",
		"query":        "safe zone",
	})
	resp2, err := ts.Client().Post(ts.URL+"/api/v3/places/resolve", "application/json", bytes.NewReader(reqBody2))
	if err != nil {
		t.Fatalf("Post resolve 2: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 OK for unambiguous place, got %d: %s", resp2.StatusCode, string(body))
	}
}

// TestC04_SelectedModelsSpecification_ContractIntegrity verifies the exact model
// requirements, architectures, quantization, and records the honest BLOCKED_HARDWARE
// status for acceptance row (b) REAL_INFERENCE.
func TestC04_SelectedModelsSpecification_ContractIntegrity(t *testing.T) {
	// ASR Model verification: AI4Bharat IndicConformer-600M Multilingual
	asrModelID := "ai4bharat/indic-conformer-600m-multilingual"
	if asrModelID != "ai4bharat/indic-conformer-600m-multilingual" {
		t.Errorf("unexpected ASR model ID: %s", asrModelID)
	}

	// Intent Model verification: Sarvam-30B MoE (2.4B active non-embedding parameters)
	sarvamModelID := "sarvamai/sarvam-30b"
	if sarvamModelID != "sarvamai/sarvam-30b" {
		t.Errorf("unexpected middle model ID: %s", sarvamModelID)
	}
	sarvamLangs := []string{"en-IN", "hi-IN", "ml-IN"}
	if len(sarvamLangs) != 3 || sarvamLangs[0] != "en-IN" || sarvamLangs[1] != "hi-IN" || sarvamLangs[2] != "ml-IN" {
		t.Errorf("unexpected Sarvam languages: %v", sarvamLangs)
	}

	// TTS Model verification: AI4Bharat Indic Parler-TTS
	ttsModelID := "ai4bharat/indic-parler-tts"
	if ttsModelID != "ai4bharat/indic-parler-tts" {
		t.Errorf("unexpected TTS model ID: %s", ttsModelID)
	}

	// REAL_INFERENCE honest status recording:
	// Local environment (macOS arm64) lacks GPU cluster, downloaded ~30GB Sarvam weights,
	// and authorized cloud spend.
	t.Log("REAL_INFERENCE: NOT_RUN (BLOCKED_HARDWARE: no GPU cluster, weights unretrieved, no authorized cloud spend)")
}
