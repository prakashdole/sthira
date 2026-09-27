//go:build integration

package httpserver

// Prototype scenario acceptance over real boundaries (PLUMBING_ONLY):
//
//   - the actual cmd/mock-workers executable, built from this tree and run
//     as a subprocess, serves the private ASR/middle/TTS protocol. It is a
//     labelled protocol fixture, NOT model inference;
//   - a task-owned disposable, migrated PostgreSQL/PostGIS database
//     (disposableTestDB) holds the synthetic package, dropped on cleanup;
//   - every request goes through the real /api/v3 handlers.
//
// Run explicitly: go test -tags integration -run 'TestProto' ./internal/httpserver/
// Skips (no psql / no admin DB) are skips, not passes.

import (
	"bufio"
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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/store"
)

// Fixed demo IDs the mock-workers executable proposes (it mirrors the
// cmd/sthira-exercise seed); the seeded package must contain them so the
// real validator accepts the proposals.
const (
	protoZoneID     = "SZDEMO-1"
	protoFacilityID = "FACDEMO-1"
)

func buildMockWorkers(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mock-workers")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/mock-workers")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/mock-workers: %v\n%s", err, out)
	}
	return bin
}

// startMockWorker runs the executable with -scenario and returns its URL
// (from the WORKER_URL= stdout line). The process is killed and reaped on
// test cleanup.
func startMockWorker(t *testing.T, bin, scenario string) string {
	t.Helper()
	cmd := exec.Command(bin, "-scenario", scenario, "-port", "0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mock worker: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	urlCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if u, ok := strings.CutPrefix(sc.Text(), "WORKER_URL="); ok {
				urlCh <- u
			}
		}
	}()
	select {
	case u := <-urlCh:
		return u
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for WORKER_URL from mock worker (%s)", scenario)
		return ""
	}
}

func seedProtoPackage(t *testing.T, st *store.Store, pkgID, jurisdiction string) {
	t.Helper()
	now := time.Now().UTC()
	ctx := context.Background()
	rzID, rtID := "RZDEMO-1", "RTDEMO-1"
	srcID, authID, artID := "SRC-"+pkgID, "AUTH-"+pkgID, "ART-"+pkgID

	body := fmt.Sprintf(`{
		"red_zones":[{"id":"%[1]s"}],
		"safe_zones":[{"id":"%[2]s","status":"OPEN"}],
		"approved_routes":[{"id":"%[3]s","from_zone_id":"%[1]s","to_safe_zone_id":"%[2]s","mode":"FOOT","approval":"SYNTHETIC_DEMO","verified_by":"proto.demo","valid_from":"2026-01-01T00:00:00Z","valid_until":"2030-01-01T00:00:00Z"}],
		"facilities":[{"id":"%[4]s","safe_zone_id":"%[2]s"}],
		"instruction_assets":[{"id":"INS-1","language":"en-IN"},{"id":"INS-2","language":"hi-IN"},{"id":"INS-3","language":"ml-IN"}],
		"allocation_policy":{"order":["%[2]s"],"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}
	}`, rzID, protoZoneID, rtID, protoFacilityID)

	err := st.InTx(ctx, func(tx store.DBTX) error {
		exec := func(q string, args ...any) error {
			_, err := tx.ExecContext(ctx, q, args...)
			return err
		}
		if err := exec(`INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			 VALUES ($1,'proto.demo','proto.example','OPERATIONAL',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		if err := exec(`INSERT INTO source_authorizations (authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
			 VALUES ($1,$2,'proto.demo','PROTO-NO-AUTH',$3,$4)`, authID, srcID, jurisdiction, now); err != nil {
			return err
		}
		if err := exec(`INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			 VALUES ($1,$2,1,$3,$4,'SYNTHETIC_DEMO','memory://proto')`, artID, srcID, strings.Repeat("a", 64), now); err != nil {
			return err
		}
		if err := exec(`INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			 VALUES ($1,'ALT-PROTO',$2,$3,1,$4,'SYNTHETIC_DEMO',$5,$6,$7,$8)`,
			pkgID, srcID, artID, jurisdiction, now.Add(-time.Hour), now.Add(24*time.Hour), strings.Repeat("b", 64), []byte(body)); err != nil {
			return err
		}
		if err := exec(`INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			 VALUES ($1,$2,'SAFE','SAFE','OPEN',100,1,$3)`, protoZoneID, pkgID, now); err != nil {
			return err
		}
		if err := exec(`INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			 VALUES ($1,$2,$3,'Asia/Kolkata',1,$4)`, protoFacilityID, pkgID, protoZoneID, now); err != nil {
			return err
		}
		if err := exec(`INSERT INTO route_versions (route_id, package_id, from_zone_id, to_safe_zone_id, approval, mode, verified_by, verified_at, valid_from, valid_until, geometry, version, updated_at)
			 VALUES ($1,$2,$3,$4,'SYNTHETIC_DEMO','FOOT','proto.demo',$5,$6,$7,ST_GeogFromText('SRID=4326;LINESTRING(76.10 11.55, 76.12 11.56)'),1,$5)`,
			rtID, pkgID, rzID, protoZoneID, now, "2026-01-01T00:00:00Z", "2030-01-01T00:00:00Z"); err != nil {
			return err
		}
		for d := 0; d < 3; d++ {
			if err := exec(`INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				 VALUES ($1, $2::date, 50, 0, 0, 0, 1, $3)`, protoFacilityID, now.AddDate(0, 0, d).Format("2006-01-02"), now); err != nil {
				return err
			}
		}
		// Clarification candidates are only known places via aliases.
		for i, a := range []struct{ id, kind string }{{protoZoneID, "ZONE"}, {protoFacilityID, "FACILITY"}} {
			if err := exec(`INSERT INTO place_aliases (alias_id, jurisdiction, lookup_key, place_id, place_kind)
				 VALUES ($1,$2,'meppadi',$3,$4)`, fmt.Sprintf("ALIAS-PROTO-%d", i), jurisdiction, a.id, a.kind); err != nil {
				return err
			}
		}
		tpls := orchestration.ExerciseTemplateRegistry(1, 1)
		for _, k := range tpls.Keys() {
			for _, lang := range []string{"en-IN", "hi-IN", "ml-IN"} {
				tpl, ok := tpls.Lookup(k, lang)
				if !ok {
					continue
				}
				h := sha256.Sum256([]byte(tpl.Text))
				if err := exec(`INSERT INTO approved_translations
						(translation_id, jurisdiction, speech_key, language, source_version, template_version,
						 approved_by, evidence_ref, approved_at, source_id, template_sha256, created_at)
					 VALUES ($1, $2, $3, $4, 1, 1, 'proto.demo', 'PROTO-NO-AUTH', $5, $6, $7, $5)`,
					fmt.Sprintf("TRANS-PROTO-%s-%s", k, lang), jurisdiction, k, lang, now, srcID, hex.EncodeToString(h[:])); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seedProtoPackage: %v", err)
	}
}

// wireProtoServer wires the real server + voice pipeline to one worker URL
// (the mock executable serves all three worker routes).
func wireProtoServer(t *testing.T, st *store.Store, workerURL string) *httptest.Server {
	t.Helper()
	voiceOpts, _, err := WireVoicePipeline(context.Background(), VoiceWiringConfig{
		Store:                   st,
		ASRURL:                  workerURL,
		MiddleURL:               workerURL,
		TTSURL:                  workerURL,
		HealthRefreshInterval:   time.Hour,
		AllowSyntheticTemplates: true,
		Templates:               orchestration.ExerciseTemplateRegistry(1, 1),
	})
	if err != nil {
		t.Fatalf("WireVoicePipeline: %v", err)
	}
	opts := append([]Option{
		WithStore(st),
		WithPersistedContextResolver(st),
		WithSyntheticExercise(StaticSyntheticExercise(true)),
	}, voiceOpts...)
	ts := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), opts...).Handler())
	t.Cleanup(ts.Close)
	return ts
}

type protoVoiceEnv struct {
	DataVersion string                     `json:"data_version"`
	Data        contracts.PipelineResponse `json:"data"`
	Errors      []contracts.APIError       `json:"errors"`
}

// protoWAVBytes is a well-formed 16 kHz / 16-bit / mono PCM WAV used as the
// citizen's (synthetic) microphone upload.
func protoWAVBytes(durationMs int) []byte {
	const rate = 16000
	dataSize := rate * durationMs / 1000 * 2
	buf := &bytes.Buffer{}
	w := func(v any) { _ = binary.Write(buf, binary.LittleEndian, v) }
	buf.WriteString("RIFF")
	w(uint32(36 + dataSize))
	buf.WriteString("WAVEfmt ")
	w(uint32(16)) // fmt chunk size is a 4-byte uint32
	w(uint16(1))  // PCM
	w(uint16(1))  // mono
	w(uint32(rate))
	w(uint32(rate * 2))
	w(uint16(2))  // block align
	w(uint16(16)) // bits per sample
	buf.WriteString("data")
	w(uint32(dataSize))
	for i := 0; i < dataSize/2; i++ {
		w(int16(100))
	}
	return buf.Bytes()
}

func postProtoVoice(t *testing.T, ts *httptest.Server, requestID, jurisdiction, lang string) (int, protoVoiceEnv) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"request_id":   requestID,
		"jurisdiction": jurisdiction,
		"language":     lang,
		"input": map[string]any{
			"kind":         "audio",
			"body_b64":     base64.StdEncoding.EncodeToString(protoWAVBytes(300)),
			"content_type": "audio/wav",
		},
		"render": map[string]any{"kind": "tts"},
	})
	resp, err := ts.Client().Post(ts.URL+"/api/v3/voice/process", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("voice/process: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env protoVoiceEnv
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode voice envelope: %v: %s", err, raw)
	}
	return resp.StatusCode, env
}

// assertPlayableAudio checks the bytes a browser would play: base64 decodes,
// byte count and SHA-256 match metadata, and the RIFF header matches the
// declared sample rate / channels / bit depth.
func assertPlayableAudio(t *testing.T, a *contracts.PipelineAudio) {
	t.Helper()
	if a == nil {
		t.Fatal("expected audio")
	}
	b, err := base64.StdEncoding.DecodeString(a.AudioB64)
	if err != nil {
		t.Fatalf("audio_b64 does not decode: %v", err)
	}
	sum := sha256.Sum256(b)
	if int64(len(b)) != a.ByteSize || hex.EncodeToString(sum[:]) != a.ChecksumSHA256 {
		t.Fatalf("audio bytes/checksum mismatch: len=%d byte_size=%d", len(b), a.ByteSize)
	}
	if len(b) < 44 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" || string(b[12:16]) != "fmt " {
		t.Fatalf("not a RIFF/WAVE fmt header")
	}
	le := binary.LittleEndian
	if le.Uint32(b[16:20]) < 16 || le.Uint16(b[20:22]) != 1 {
		t.Fatalf("fmt chunk not PCM")
	}
	ch, rate, bits := int(le.Uint16(b[22:24])), int(le.Uint32(b[24:28])), int(le.Uint16(b[34:36]))
	if ch != a.Settings.Channels || rate != a.Settings.SampleRate || bits != a.Settings.BitDepth {
		t.Fatalf("header ch=%d rate=%d bits=%d != declared %+v", ch, rate, bits, a.Settings)
	}
}

func TestProtoScenario_MockWorkersExecutable(t *testing.T) {
	dsn, cleanup := disposableTestDB(t)
	defer cleanup()
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	const pkgID, jur = "PKG-PROTO-1", "JUR-PROTO-1"
	seedProtoPackage(t, st, pkgID, jur)
	bin := buildMockWorkers(t)
	wantVersion := store.SnapshotDataVersion(pkgID, 1)
	hiText, _ := orchestration.ExerciseTemplateRegistry(1, 1).Lookup("destination_options", "hi-IN")

	cases := []struct {
		scenario, lang string
		check          func(t *testing.T, code int, env protoVoiceEnv)
	}{
		{"default", "en-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			p := env.Data.ValidatedProposal
			if code != 200 || len(p.Actions) != 1 || p.Actions[0].Type != contracts.ActionFocusFeature || p.Actions[0].TargetID != protoZoneID {
				t.Fatalf("code=%d actions=%+v", code, p.Actions)
			}
			assertPlayableAudio(t, env.Data.Audio)
		}},
		{"silent-zoom", "en-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			p := env.Data.ValidatedProposal
			if code != 200 || p.Intent == nil || *p.Intent != contracts.IntentZoom || len(p.Actions) != 1 || p.Actions[0].Type != contracts.ActionZoom {
				t.Fatalf("code=%d proposal=%+v", code, p)
			}
			if p.SpeechKey != nil || env.Data.Audio != nil || len(env.Data.StageFailures) != 0 {
				t.Fatalf("silent action must have no speech/audio/stage failures: %+v", env.Data)
			}
		}},
		{"destination-choice", "en-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			p := env.Data.ValidatedProposal
			if code != 200 || len(p.Actions) != 1 || p.Actions[0].Type != contracts.ActionShowChoices ||
				len(p.Actions[0].TargetIDs) != 1 || p.Actions[0].TargetIDs[0] != protoFacilityID {
				t.Fatalf("code=%d actions=%+v", code, p.Actions)
			}
			if len(env.Data.StageFailures) != 0 || env.Data.Template.Text == "" {
				t.Fatalf("stage_failures=%v template=%+v", env.Data.StageFailures, env.Data.Template)
			}
			assertPlayableAudio(t, env.Data.Audio)
			if env.Data.Audio.Language != "en-IN" {
				t.Fatalf("audio language=%s", env.Data.Audio.Language)
			}
		}},
		{"destination-choice", "hi-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			if code != 200 || len(env.Data.StageFailures) != 0 {
				t.Fatalf("code=%d stage_failures=%v errors=%v", code, env.Data.StageFailures, env.Errors)
			}
			if env.Data.ValidatedProposal.Language != "hi-IN" || env.Data.Template.Text != hiText.Text {
				t.Fatalf("template not the approved hi-IN text: %+v", env.Data.Template)
			}
			assertPlayableAudio(t, env.Data.Audio)
			if env.Data.Audio.Language != "hi-IN" {
				t.Fatalf("audio language=%s, want hi-IN", env.Data.Audio.Language)
			}
		}},
		{"arrival-confirm", "en-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			p := env.Data.ValidatedProposal
			if code != 200 || len(p.Actions) != 1 || p.Actions[0].Type != contracts.ActionOpenPanel || p.Actions[0].Panel != contracts.PanelArrivalConfirm {
				t.Fatalf("code=%d actions=%+v", code, p.Actions)
			}
			if p.SpeechKey != nil || env.Data.Audio != nil {
				t.Fatalf("arrival panel must be silent")
			}
		}},
		{"clarify", "en-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			p := env.Data.ValidatedProposal
			if code != 200 || env.Data.State != contracts.PipelineClarify || p.Status != contracts.StatusClarify {
				t.Fatalf("code=%d state=%s errors=%v", code, env.Data.State, env.Errors)
			}
			if p.Intent != nil || len(p.Actions) != 0 {
				t.Fatalf("CLARIFY must carry nil intent and no actions: %+v", p)
			}
			if strings.Join(p.ClarificationIDs, ",") != protoZoneID+","+protoFacilityID {
				t.Fatalf("clarification_ids=%v", p.ClarificationIDs)
			}
		}},
		{"data-unavailable", "en-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			p := env.Data.ValidatedProposal
			if code != 200 || p.Status != contracts.StatusDataUnavailable || p.Intent != nil || len(p.Actions) != 0 || env.Data.Audio != nil {
				t.Fatalf("code=%d proposal=%+v", code, p)
			}
		}},
		{"worker-failure", "en-IN", func(t *testing.T, code int, env protoVoiceEnv) {
			if code != http.StatusServiceUnavailable || len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrModelUnavailable {
				t.Fatalf("code=%d errors=%v", code, env.Errors)
			}
			if env.Data.Audio != nil || len(env.Data.ValidatedProposal.Actions) != 0 {
				t.Fatalf("worker failure must not yield actions/audio")
			}
		}},
	}
	for i, tc := range cases {
		t.Run(tc.scenario+"/"+tc.lang, func(t *testing.T) {
			ts := wireProtoServer(t, st, startMockWorker(t, bin, tc.scenario))
			code, env := postProtoVoice(t, ts, fmt.Sprintf("req-proto-%d", i), jur, tc.lang)
			if code == 200 && (env.DataVersion != wantVersion || env.Data.DataVersion != wantVersion) {
				t.Fatalf("data_version envelope=%q data=%q want %q", env.DataVersion, env.Data.DataVersion, wantVersion)
			}
			tc.check(t, code, env)
		})
	}
}

type protoGuidance struct {
	DataVersion  string `json:"data_version"`
	SourceStatus string `json:"source_status"`
	Data         struct {
		SnapshotVersion int `json:"snapshot_version"`
		Destinations    []struct {
			FacilityID string `json:"facility_id"`
		} `json:"destinations"`
	} `json:"data"`
}

func queryProtoGuidance(t *testing.T, ts *httptest.Server, pkgID, jur string) protoGuidance {
	t.Helper()
	now := time.Now().UTC()
	rec := doUnauthed(t, ts, http.MethodPost, "/api/v3/guidance/query", fmt.Sprintf(
		`{"jurisdiction":%q,"package_id":%q,"party_size":1,"start_date":%q,"end_date":%q}`,
		jur, pkgID, now.Format("2006-01-02"), now.AddDate(0, 0, 1).Format("2006-01-02")))
	if rec.code != http.StatusOK {
		t.Fatalf("guidance/query: %d %s", rec.code, rec.body)
	}
	var g protoGuidance
	if err := json.Unmarshal([]byte(rec.body), &g); err != nil {
		t.Fatalf("decode guidance: %v", err)
	}
	return g
}

// TestProtoSnapshotIdentity proves guidance/query and voice/process emit the
// same data_version for the same persisted package snapshot, that a version
// change moves both (invalidating older guidance/audio), and that stale
// snapshot bindings stay rejected.
func TestProtoSnapshotIdentity(t *testing.T) {
	dsn, cleanup := disposableTestDB(t)
	defer cleanup()
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	const pkgID, jur = "PKG-SNAP-1", "JUR-SNAP-1"
	seedProtoPackage(t, st, pkgID, jur)
	ts := wireProtoServer(t, st, startMockWorker(t, buildMockWorkers(t), "destination-choice"))

	g1 := queryProtoGuidance(t, ts, pkgID, jur)
	code, v1 := postProtoVoice(t, ts, "req-snap-1", jur, "en-IN")
	if code != 200 {
		t.Fatalf("voice/process: %d %v", code, v1.Errors)
	}
	want1 := store.SnapshotDataVersion(pkgID, 1)
	if g1.SourceStatus != "CURRENT" || g1.DataVersion != want1 || g1.Data.SnapshotVersion != 1 {
		t.Fatalf("guidance identity: %+v, want %s", g1, want1)
	}
	if v1.DataVersion != g1.DataVersion || v1.Data.DataVersion != g1.DataVersion {
		t.Fatalf("guidance %q vs voice envelope %q / data %q", g1.DataVersion, v1.DataVersion, v1.Data.DataVersion)
	}
	assertPlayableAudio(t, v1.Data.Audio)

	// Reservation bound to the guidance snapshot succeeds.
	_, token := createSession(t, ts)
	now := time.Now().UTC()
	d0, d1 := now.Format("2006-01-02"), now.AddDate(0, 0, 1).Format("2006-01-02")
	if rec := doAuthed(t, ts, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(protoFacilityID, pkgID, 1, d0, d1, "snap-key-1", g1.Data.SnapshotVersion)); rec.code != http.StatusCreated {
		t.Fatalf("reservation at guidance snapshot: %d %s", rec.code, rec.body)
	}

	// Package snapshot changes.
	if _, err := st.DB().ExecContext(context.Background(), `UPDATE packages SET version = 2 WHERE package_id = $1`, pkgID); err != nil {
		t.Fatalf("bump version: %v", err)
	}
	g2 := queryProtoGuidance(t, ts, pkgID, jur)
	want2 := store.SnapshotDataVersion(pkgID, 2)
	// Translation approvals are bound to source_version 1, so spoken
	// guidance for the new snapshot fails closed (no audio) until re-approved.
	if code, v := postProtoVoice(t, ts, "req-snap-2", jur, "en-IN"); code == 200 || v.Data.Audio != nil {
		t.Fatalf("speech after snapshot change must fail closed: code=%d", code)
	}
	// A silent action on the new snapshot carries the new identity.
	silent := wireProtoServer(t, st, startMockWorker(t, buildMockWorkers(t), "silent-zoom"))
	code, v2 := postProtoVoice(t, silent, "req-snap-3", jur, "en-IN")
	if code != 200 || g2.DataVersion != want2 || g2.Data.SnapshotVersion != 2 || v2.DataVersion != want2 {
		t.Fatalf("after bump: guidance=%q(%d) voice=%d %q want %q", g2.DataVersion, g2.Data.SnapshotVersion, code, v2.DataVersion, want2)
	}
	// Old guidance/audio identity no longer equals the current snapshot, so
	// the browser's equality gate refuses the old audio.
	if g2.DataVersion == g1.DataVersion || v1.DataVersion == g2.DataVersion {
		t.Fatal("version change did not change snapshot identity")
	}
	// A reservation still bound to the old snapshot is rejected.
	rec := doAuthed(t, ts, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(protoFacilityID, pkgID, 1, d0, d1, "snap-key-2", g1.Data.SnapshotVersion))
	if rec.code != http.StatusConflict || !strings.Contains(rec.body, contracts.ErrStaleVersion) {
		t.Fatalf("stale snapshot reservation: %d %s", rec.code, rec.body)
	}

	// An expired package has no operational snapshot identity.
	if _, err := st.DB().ExecContext(context.Background(), `UPDATE packages SET expires_at = now() - interval '1 minute' WHERE package_id = $1`, pkgID); err != nil {
		t.Fatalf("expire: %v", err)
	}
	g3 := queryProtoGuidance(t, ts, pkgID, jur)
	if g3.SourceStatus == "CURRENT" || g3.DataVersion != "none" || g3.Data.SnapshotVersion != 0 {
		t.Fatalf("expired guidance must not carry a snapshot identity: %+v", g3)
	}
}
