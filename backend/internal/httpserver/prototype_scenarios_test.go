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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/store"
)

func protoDB(t *testing.T) (string, func()) {
	t.Helper()
	admin := os.Getenv("STHIRA_TEST_ADMIN_DSN")
	if admin == "" {
		admin = "postgres://apple@localhost:5432/postgres"
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Skip("psql not on PATH; skipping DB test")
	}

	dbName := fmt.Sprintf("sthira_proto_%d_%d", time.Now().UnixNano(), os.Getpid())
	cmd := exec.Command("psql", admin, "-c", fmt.Sprintf(`CREATE DATABASE %s`, dbName))
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot create DB %s: %v", dbName, err)
	}

	cleanup := func() {
		dropCmd := exec.Command("psql", admin, "-c", fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName))
		_ = dropCmd.Run()
	}

	dsn := strings.Replace(admin, "/postgres", "/"+dbName, 1)
	migDir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		cleanup()
		t.Fatalf("ReadDir(%s): %v", migDir, err)
	}
	var migFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migFiles = append(migFiles, entry.Name())
		}
	}
	sort.Strings(migFiles)
	for _, f := range migFiles {
		migPath := filepath.Join(migDir, f)
		pCmd := exec.Command("psql", dsn, "-v", "ON_ERROR_STOP=1", "-q", "-f", migPath)
		out, err := pCmd.CombinedOutput()
		if err != nil {
			cleanup()
			t.Fatalf("migration %s failed: %v: %s", f, err, string(out))
		}
	}
	return dsn, cleanup
}

func seedProtoPackage(t *testing.T, st *store.Store, pkgID, jurisdiction string) {
	t.Helper()
	now := time.Now().UTC()
	ctx := context.Background()
	szID := "SZ-" + pkgID
	facID := "FAC-" + pkgID
	rzID := "RZ-" + pkgID
	rtID := "RT-" + pkgID
	srcID := "SRC-" + pkgID
	authID := "AUTH-" + pkgID
	artID := "ART-" + pkgID

	body := fmt.Sprintf(`{
		"red_zones":[{"id":"%s"}],
		"safe_zones":[{"id":"%s","status":"OPEN"}],
		"approved_routes":[{"id":"%s","from_zone_id":"%s","to_safe_zone_id":"%s","mode":"FOOT","approval":"SYNTHETIC_DEMO","verified_by":"proto.demo","valid_from":"2026-01-01T00:00:00Z","valid_until":"2030-01-01T00:00:00Z"}],
		"facilities":[{"id":"%s","safe_zone_id":"%s"}],
		"instruction_assets":[{"id":"INS-1","language":"en-IN"},{"id":"INS-2","language":"hi-IN"},{"id":"INS-3","language":"ml-IN"}],
		"allocation_policy":{"order":["%s"]}
	}`, rzID, szID, rtID, rzID, szID, facID, szID, szID)

	err := st.InTx(ctx, func(tx store.DBTX) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			 VALUES ($1,'proto.demo','proto.example','OPERATIONAL',1,$2,$2)`,
			srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO source_authorizations (authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
			 VALUES ($1,$2,'proto.demo','PROTO-NO-AUTH',$3,$4)`,
			authID, srcID, jurisdiction, now); err != nil {
			return err
		}
		hashHex := hex.EncodeToString([]byte(strings.Repeat("a", 32)))
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			 VALUES ($1,$2,1,$3,$4,'SYNTHETIC_DEMO','memory://proto')`,
			artID, srcID, hashHex, now); err != nil {
			return err
		}
		chkHex := hex.EncodeToString([]byte(strings.Repeat("b", 32)))
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			 VALUES ($1,'ALT-%s',$2,$3,1,$4,'SYNTHETIC_DEMO',$5,$6,$7,$8)`,
			pkgID, srcID, artID, jurisdiction,
			now.Add(-time.Hour), now.Add(24*time.Hour), chkHex, []byte(body)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			 VALUES ($1,$2,'SAFE','SAFE','OPEN',100,1,$3)`,
			szID, pkgID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			 VALUES ($1,$2,$3,'Asia/Kolkata',1,$4)`,
			facID, pkgID, szID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO route_versions (route_id, package_id, from_zone_id, to_safe_zone_id, approval, mode, verified_by, verified_at, valid_from, valid_until, geometry, version, updated_at)
			 VALUES ($1,$2,$3,$4,'SYNTHETIC_DEMO','FOOT','proto.demo',$5,$6,$7,ST_GeogFromText('SRID=4326;LINESTRING(76.10 11.55, 76.12 11.56)'),1,$5)`,
			rtID, pkgID, rzID, szID, now, "2026-01-01T00:00:00Z", "2030-01-01T00:00:00Z"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, version, updated_at)
			 VALUES ($1, $2, 50, 5, 1, $3)`,
			facID, now.Format("2006-01-02"), now); err != nil {
			return err
		}
		tpls := orchestration.ExerciseTemplateRegistry(1, 1)
		for _, k := range tpls.Keys() {
			for _, lang := range []string{"en-IN", "hi-IN", "ml-IN"} {
				if tpl, ok := tpls.Lookup(k, lang); ok {
					h := sha256.Sum256([]byte(tpl.Text))
					dig := hex.EncodeToString(h[:])
					transID := fmt.Sprintf("TRANS-PROTO-%s-%s-%s", k, lang, pkgID)
					if _, err := tx.ExecContext(ctx,
						`INSERT INTO approved_translations
							(translation_id, jurisdiction, speech_key, language, source_version, template_version,
							 approved_by, evidence_ref, approved_at, source_id, template_sha256, created_at)
						 VALUES ($1, $2, $3, $4, 1, 1, 'proto.demo', 'PROTO-NO-AUTH', $5, $6, $7, $5)`,
						transID, jurisdiction, k, lang, now, srcID, dig); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seedProtoPackage: %v", err)
	}
}

type mockScenarioWorker struct {
	scenario    string
	asrCalls    atomic.Int64
	middleCalls atomic.Int64
	ttsCalls    atomic.Int64
}

func (m *mockScenarioWorker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.WorkerHealth{
			Ready:              true,
			Warm:               true,
			SupportedLanguages: []string{"en-IN", "hi-IN", "ml-IN"},
		})
	case "/transcribe":
		m.asrCalls.Add(1)
		var req contracts.ASRWorkerRequest
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.ASRWorkerResponse{
			RequestID:      req.RequestID,
			Language:       "en-IN",
			Text:           "Meppadi",
			State:          contracts.TranscriptionOK,
			ModelRevision:  "indic-conformer-600m-v1",
			ArtifactDigest: "sha256-conformer-600m-demo",
		})
	case "/v1/chat/completions":
		m.middleCalls.Add(1)
		if m.scenario == "worker-failure" {
			http.Error(w, `{"error":"model unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		var req contracts.MiddleWorkerRequest
		json.NewDecoder(r.Body).Decode(&req)
		dataVer := req.ScopedContext.DataVersion
		if dataVer == "" {
			dataVer = "PKGDEMO-1:1"
		}
		lang := "en-IN"
		if req.Transcript.Language != "" {
			lang = req.Transcript.Language
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(buildScenarioResponse(m.scenario, req.RequestID, dataVer, lang))
	case "/synthesize":
		m.ttsCalls.Add(1)
		var req contracts.TTSWorkerRequest
		json.NewDecoder(r.Body).Decode(&req)
		wav := protoWAVBytes(400)
		wavB64 := base64.StdEncoding.EncodeToString(wav)
		wavHash := sha256.Sum256(wav)
		wavChecksum := hex.EncodeToString(wavHash[:])
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      req.SpeechKey,
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       wavB64,
			ContentType:    "audio/wav",
			ChecksumSHA256: wavChecksum,
			ModelRevision:  "indic-parler-tts-v1",
			VoiceRevision:  "voice-default-v1",
			Settings: contracts.TTSSynthesisSettings{
				SampleRate: 16000,
				BitDepth:   16,
				Channels:   1,
			},
		})
	default:
		http.NotFound(w, r)
	}
}

func buildScenarioResponse(scenario, requestID, dataVer, lang string) contracts.MiddleWorkerResponse {
	switch scenario {
	case "silent-zoom":
		intent := contracts.IntentZoom
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:       requestID,
				DataVersion:     dataVer,
				Status:          contracts.StatusOK,
				Intent:          &intent,
				Language:        lang,
				Actions:         []contracts.Action{{Type: contracts.ActionZoom, Direction: "IN", Steps: 1}},
				SpeechKey:       nil,
				ClarificationIDs: []string{},
				EvidenceIDs:     []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}
	case "destination-choice":
		intent := contracts.IntentListDestinations
		sk := "destination_options"
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:       requestID,
				DataVersion:     dataVer,
				Status:          contracts.StatusOK,
				Intent:          &intent,
				Language:        lang,
				Actions:         []contracts.Action{{Type: contracts.ActionShowChoices, TargetIDs: []string{"FAC-PKG-DEST-1"}}},
				SpeechKey:       &sk,
				ClarificationIDs: []string{},
				EvidenceIDs:     []string{"FAC-PKG-DEST-1"},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}
	case "arrival-confirm":
		intent := contracts.IntentOpenConfirmation
		panel := contracts.PanelArrivalConfirm
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:       requestID,
				DataVersion:     dataVer,
				Status:          contracts.StatusOK,
				Intent:          &intent,
				Language:        lang,
				Actions:         []contracts.Action{{Type: contracts.ActionOpenPanel, Panel: panel}},
				SpeechKey:       nil,
				ClarificationIDs: []string{},
				EvidenceIDs:     []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}
	case "clarify":
		intent := contracts.IntentFocusPlace
		sk := "clarify_place"
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:       requestID,
				DataVersion:     dataVer,
				Status:          contracts.StatusClarify,
				Intent:          &intent,
				Language:        lang,
				Actions:         []contracts.Action{},
				SpeechKey:       &sk,
				ClarificationIDs: []string{"SZ-PKG-CLARIFY-1", "FAC-PKG-CLARIFY-1"},
				EvidenceIDs:     []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}
	case "data-unavailable":
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:       requestID,
				DataVersion:     dataVer,
				Status:          contracts.StatusDataUnavailable,
				Intent:          nil,
				Language:        lang,
				Actions:         []contracts.Action{},
				SpeechKey:       nil,
				ClarificationIDs: []string{},
				EvidenceIDs:     []string{},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}
	default:
		intent := contracts.IntentFocusPlace
		sk := "destination_options"
		return contracts.MiddleWorkerResponse{
			RequestID:   requestID,
			DataVersion: dataVer,
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:       requestID,
				DataVersion:     dataVer,
				Status:          contracts.StatusOK,
				Intent:          &intent,
				Language:        lang,
				Actions:         []contracts.Action{{Type: contracts.ActionFocusFeature, TargetID: "SZ-PKG-DEFAULT-1"}},
				SpeechKey:       &sk,
				ClarificationIDs: []string{},
				EvidenceIDs:     []string{"SZ-PKG-DEFAULT-1"},
			},
			ModelRevision: "sarvamai/sarvam-30b-fp8",
		}
	}
}

func protoWAVBytes(durationMs int) []byte {
	sampleRate := 16000
	numSamples := sampleRate * durationMs / 1000
	dataSize := numSamples * 2
	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	var le uint32 = uint32(36 + dataSize)
	buf.Write([]byte{
		byte(le), byte(le >> 8), byte(le >> 16), byte(le >> 24),
	})
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	var le16 uint16 = 16
	buf.Write([]byte{
		byte(le16), byte(le16 >> 8),
	})
	var PCM uint16 = 1
	buf.Write([]byte{
		byte(PCM), byte(PCM >> 8),
	})
	var mono uint16 = 1
	buf.Write([]byte{
		byte(mono), byte(mono >> 8),
	})
	var sr uint32 = uint32(sampleRate)
	buf.Write([]byte{
		byte(sr), byte(sr >> 8), byte(sr >> 16), byte(sr >> 24),
	})
	var br uint32 = uint32(sampleRate * 2)
	buf.Write([]byte{
		byte(br), byte(br >> 8), byte(br >> 16), byte(br >> 24),
	})
	var ba uint16 = 2
	buf.Write([]byte{
		byte(ba), byte(ba >> 8),
	})
	var bd uint16 = 16
	buf.Write([]byte{
		byte(bd), byte(bd >> 8),
	})
	buf.WriteString("data")
	var ds uint32 = uint32(dataSize)
	buf.Write([]byte{
		byte(ds), byte(ds >> 8), byte(ds >> 16), byte(ds >> 24),
	})
	for i := 0; i < numSamples; i++ {
		var s int16 = 100
		buf.Write([]byte{
			byte(s), byte(s >> 8),
		})
	}
	return buf.Bytes()
}

func wireProtoTestServer(t *testing.T, dsn string, scenario string, templates *orchestration.MapTemplateRegistry) (*httptest.Server, *store.Store, *mockScenarioWorker) {
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	worker := &mockScenarioWorker{scenario: scenario}
	asrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		worker.ServeHTTP(w, r)
	}))
	middleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		worker.ServeHTTP(w, r)
	}))
	ttsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		worker.ServeHTTP(w, r)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	voiceOpts, _, err := WireVoicePipeline(ctx, VoiceWiringConfig{
		Store:                   st,
		Logger:                  nil,
		ASRURL:                  asrServer.URL,
		MiddleURL:               middleServer.URL,
		TTSURL:                  ttsServer.URL,
		HealthRefreshInterval:   time.Hour,
		AllowSyntheticTemplates: true,
		Templates:               templates,
	})
	if err != nil {
		t.Fatalf("WireVoicePipeline: %v", err)
	}

	serverOpts := []Option{
		WithStore(st),
		WithPersistedContextResolver(st),
		WithSyntheticExercise(StaticSyntheticExercise(true)),
	}
	serverOpts = append(serverOpts, voiceOpts...)
	appServer := New(DefaultConfig("127.0.0.1:0"), serverOpts...)
	ts := httptest.NewServer(appServer.Handler())

	return ts, st, worker
}

func protoVoicePayload(requestID, jurisdiction, lang string) []byte {
	wavData := protoWAVBytes(300)
	audioB64 := base64.StdEncoding.EncodeToString(wavData)
	payload := map[string]any{
		"request_id":   requestID,
		"jurisdiction": jurisdiction,
		"language":     lang,
		"input": map[string]any{
			"kind":         "audio",
			"body_b64":     audioB64,
			"content_type": "audio/wav",
		},
		"render": map[string]any{
			"kind": "tts",
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func TestProtoScenario_SilentZoom(t *testing.T) {
	dsn, cleanup := protoDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedProtoPackage(t, st, "PKG-ZOOM-1", "JUR-ZOOM-1")

	ts, _, worker := wireProtoTestServer(t, dsn, "silent-zoom", orchestration.ExerciseTemplateRegistry(1, 1))
	defer ts.Close()

	body := protoVoicePayload("req-zoom-1", "JUR-ZOOM-1", "en-IN")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Data struct {
			State             string `json:"state"`
			ValidatedProposal *struct {
				Intent  *string `json:"intent"`
				Actions []struct {
					Type     string `json:"type"`
					Direction string `json:"direction"`
					Steps    int    `json:"steps"`
				} `json:"actions"`
				SpeechKey any `json:"speech_key"`
			} `json:"validated_proposal"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if env.Data.ValidatedProposal == nil || env.Data.ValidatedProposal.Intent == nil {
		t.Fatalf("expected proposal with intent")
	}
	if *env.Data.ValidatedProposal.Intent != "ZOOM" {
		t.Errorf("expected ZOOM intent, got %v", *env.Data.ValidatedProposal.Intent)
	}
	if len(env.Data.ValidatedProposal.Actions) == 0 {
		t.Fatalf("expected actions")
	}
	if env.Data.ValidatedProposal.Actions[0].Type != "ZOOM" {
		t.Errorf("expected ZOOM action, got %v", env.Data.ValidatedProposal.Actions[0].Type)
	}
	if env.Data.ValidatedProposal.Actions[0].Direction != "IN" {
		t.Errorf("expected IN direction, got %v", env.Data.ValidatedProposal.Actions[0].Direction)
	}
	if env.Data.ValidatedProposal.SpeechKey != nil {
		t.Errorf("expected nil speech_key for silent zoom, got %v", env.Data.ValidatedProposal.SpeechKey)
	}
	if worker.asrCalls.Load() == 0 {
		t.Errorf("ASR should have been called")
	}
	if worker.middleCalls.Load() == 0 {
		t.Errorf("Middle worker should have been called")
	}
}

func TestProtoScenario_DestinationChoice(t *testing.T) {
	dsn, cleanup := protoDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedProtoPackage(t, st, "PKG-DEST-1", "JUR-DEST-1")

	ts, _, worker := wireProtoTestServer(t, dsn, "destination-choice", orchestration.ExerciseTemplateRegistry(1, 1))
	defer ts.Close()

	body := protoVoicePayload("req-dest-1", "JUR-DEST-1", "en-IN")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Data struct {
			State             string `json:"state"`
			ValidatedProposal *struct {
				Intent  *string `json:"intent"`
				Actions []struct {
					Type      string   `json:"type"`
					TargetIDs []string `json:"target_ids"`
				} `json:"actions"`
				SpeechKey *string `json:"speech_key"`
			} `json:"validated_proposal"`
			Template *struct {
				SpeechKey string `json:"speech_key"`
			} `json:"template"`
			Audio *struct {
				AudioB64 string `json:"audio_b64"`
			} `json:"audio"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if env.Data.ValidatedProposal == nil || env.Data.ValidatedProposal.Intent == nil {
		t.Fatalf("expected proposal with intent")
	}
	if *env.Data.ValidatedProposal.Intent != "LIST_DESTINATIONS" {
		t.Errorf("expected LIST_DESTINATIONS intent, got %v", *env.Data.ValidatedProposal.Intent)
	}
	if len(env.Data.ValidatedProposal.Actions) == 0 {
		t.Fatalf("expected actions")
	}
	if env.Data.ValidatedProposal.Actions[0].Type != "SHOW_CHOICES" {
		t.Errorf("expected SHOW_CHOICES action, got %v", env.Data.ValidatedProposal.Actions[0].Type)
	}
	if len(env.Data.ValidatedProposal.Actions[0].TargetIDs) != 1 || env.Data.ValidatedProposal.Actions[0].TargetIDs[0] != "FAC-PKG-DEST-1" {
		t.Errorf("expected FAC-PKG-DEST-1, got %v", env.Data.ValidatedProposal.Actions[0].TargetIDs)
	}
	if env.Data.ValidatedProposal.SpeechKey == nil || *env.Data.ValidatedProposal.SpeechKey != "destination_options" {
		t.Errorf("expected destination_options speech_key, got %v", env.Data.ValidatedProposal.SpeechKey)
	}
	if env.Data.Template == nil || env.Data.Template.SpeechKey != "destination_options" {
		t.Errorf("expected template with destination_options")
	}
	if env.Data.Audio == nil || env.Data.Audio.AudioB64 == "" {
		t.Errorf("expected audio in response")
	}
	if worker.ttsCalls.Load() == 0 {
		t.Errorf("TTS should have been called")
	}
}

func TestProtoScenario_ArrivalConfirm(t *testing.T) {
	dsn, cleanup := protoDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedProtoPackage(t, st, "PKG-ARRIVE-1", "JUR-ARRIVE-1")

	ts, _, _ := wireProtoTestServer(t, dsn, "arrival-confirm", orchestration.ExerciseTemplateRegistry(1, 1))
	defer ts.Close()

	body := protoVoicePayload("req-arrive-1", "JUR-ARRIVE-1", "en-IN")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Data struct {
			State             string `json:"state"`
			ValidatedProposal *struct {
				Intent  *string `json:"intent"`
				Actions []struct {
					Type  string `json:"type"`
					Panel string `json:"panel"`
				} `json:"actions"`
				SpeechKey any `json:"speech_key"`
			} `json:"validated_proposal"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if env.Data.ValidatedProposal == nil || env.Data.ValidatedProposal.Intent == nil {
		t.Fatalf("expected proposal with intent")
	}
	if *env.Data.ValidatedProposal.Intent != "OPEN_CONFIRMATION" {
		t.Errorf("expected OPEN_CONFIRMATION intent, got %v", *env.Data.ValidatedProposal.Intent)
	}
	if len(env.Data.ValidatedProposal.Actions) == 0 {
		t.Fatalf("expected actions")
	}
	if env.Data.ValidatedProposal.Actions[0].Type != "OPEN_PANEL" {
		t.Errorf("expected OPEN_PANEL action, got %v", env.Data.ValidatedProposal.Actions[0].Type)
	}
	if env.Data.ValidatedProposal.Actions[0].Panel != "ARRIVAL_CONFIRMATION" {
		t.Errorf("expected ARRIVAL_CONFIRMATION panel, got %v", env.Data.ValidatedProposal.Actions[0].Panel)
	}
	if env.Data.ValidatedProposal.SpeechKey != nil {
		t.Errorf("expected nil speech_key for arrival confirm, got %v", env.Data.ValidatedProposal.SpeechKey)
	}
}

func TestProtoScenario_Clarify(t *testing.T) {
	dsn, cleanup := protoDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedProtoPackage(t, st, "PKG-CLARIFY-1", "JUR-CLARIFY-1")

	ts, _, _ := wireProtoTestServer(t, dsn, "clarify", orchestration.ExerciseTemplateRegistry(1, 1))
	defer ts.Close()

	body := protoVoicePayload("req-clarify-1", "JUR-CLARIFY-1", "en-IN")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Data struct {
			State             string `json:"state"`
			ValidatedProposal *struct {
				Status          string   `json:"status"`
				Intent          *string `json:"intent"`
				ClarificationIDs []string `json:"clarification_ids"`
				SpeechKey       *string `json:"speech_key"`
			} `json:"validated_proposal"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if env.Data.ValidatedProposal == nil {
		t.Fatalf("expected proposal")
	}
	if env.Data.ValidatedProposal.Status != "CLARIFY" {
		t.Errorf("expected CLARIFY status, got %v", env.Data.ValidatedProposal.Status)
	}
	if len(env.Data.ValidatedProposal.ClarificationIDs) != 2 {
		t.Errorf("expected 2 clarification_ids, got %v", env.Data.ValidatedProposal.ClarificationIDs)
	}
}

func TestProtoScenario_DataUnavailable(t *testing.T) {
	dsn, cleanup := protoDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedProtoPackage(t, st, "PKG-UNAVAIL-1", "JUR-UNAVAIL-1")

	ts, _, _ := wireProtoTestServer(t, dsn, "data-unavailable", orchestration.ExerciseTemplateRegistry(1, 1))
	defer ts.Close()

	body := protoVoicePayload("req-unavail-1", "JUR-UNAVAIL-1", "en-IN")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var env struct {
		Data struct {
			State             string `json:"state"`
			ValidatedProposal *struct {
				Status  string   `json:"status"`
				Intent  any     `json:"intent"`
				Actions []any    `json:"actions"`
			} `json:"validated_proposal"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if env.Data.ValidatedProposal == nil {
		t.Fatalf("expected proposal")
	}
	if env.Data.ValidatedProposal.Status != "DATA_UNAVAILABLE" {
		t.Errorf("expected DATA_UNAVAILABLE status, got %v", env.Data.ValidatedProposal.Status)
	}
	if env.Data.ValidatedProposal.Intent != nil {
		t.Errorf("expected nil intent for DATA_UNAVAILABLE, got %v", env.Data.ValidatedProposal.Intent)
	}
	if len(env.Data.ValidatedProposal.Actions) != 0 {
		t.Errorf("expected no actions for DATA_UNAVAILABLE, got %v", env.Data.ValidatedProposal.Actions)
	}
}

func TestProtoScenario_WorkerFailure(t *testing.T) {
	dsn, cleanup := protoDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	seedProtoPackage(t, st, "PKG-FAIL-1", "JUR-FAIL-1")

	ts, _, _ := wireProtoTestServer(t, dsn, "worker-failure", orchestration.ExerciseTemplateRegistry(1, 1))
	defer ts.Close()

	body := protoVoicePayload("req-fail-1", "JUR-FAIL-1", "en-IN")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v3/voice/process", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 for worker failure, got 200")
	}
}
