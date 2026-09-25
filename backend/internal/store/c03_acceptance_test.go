package store

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/orchestration/orchestrationtest"
)

var _ = time.Second

// makeMockWAV returns minimal valid 16kHz mono 16-bit PCM WAV bytes.
func makeMockWAV() []byte {
	dataSize := 320
	fileSize := 36 + dataSize
	b := make([]byte, 44+dataSize)
	copy(b[0:4], "RIFF")
	b[4] = byte(fileSize)
	b[5] = byte(fileSize >> 8)
	b[6] = byte(fileSize >> 16)
	b[7] = byte(fileSize >> 24)
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	b[16] = 16 // PCM header size
	b[20] = 1  // PCM format
	b[22] = 1  // channels = 1 (mono)
	// Sample rate: 16000
	b[24] = 0x80
	b[25] = 0x3E
	b[26] = 0
	b[27] = 0
	// Byte rate: 16000 * 2 = 32000
	b[28] = 0x00
	b[29] = 0x7D
	b[30] = 0
	b[31] = 0
	b[32] = 2  // block align
	b[34] = 16 // bits per sample
	copy(b[36:40], "data")
	b[40] = byte(dataSize)
	b[41] = byte(dataSize >> 8)
	b[42] = byte(dataSize >> 16)
	b[43] = byte(dataSize >> 24)
	return b
}

// TestC03_MultiLanguage_SameKey_Synthesize tests:
// 1. Real DB with same key ("welcome") and different en-IN / ml-IN approved texts.
// 2. Exact digest retrieval by speech key AND language.
// 3. Both correct languages synthesize through actual orchestrator with protocol-test workers.
func TestC03_MultiLanguage_SameKey_Synthesize(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	now := nowUTC()
	srcID := findKLP6Source(t, fx.store.DB())

	textEN := "Welcome, citizen."
	textML := "സ്വാഗതം, പൗരനേ."
	sumEN := sha256.Sum256([]byte(textEN))
	sumML := sha256.Sum256([]byte(textML))
	digEN := hex.EncodeToString(sumEN[:])
	digML := hex.EncodeToString(sumML[:])

	// Seed real DB with both languages for same speech key "welcome" (version 7 matches fixture package)
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES
			($1, $2, 'welcome', 'en-IN', 7, 7, $3, $4, 'reviewer-en', 'doc-en', $5),
			($6, $2, 'welcome', 'ml-IN', 7, 7, $3, $7, 'reviewer-ml', 'doc-ml', $5)`,
		"APP-C03-EN-"+uid("X"), fx.jurisdictionID, srcID, digEN, now,
		"APP-C03-ML-"+uid("X"), digML); err != nil {
		t.Fatalf("seed approvals: %v", err)
	}

	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// 1. Check ScopedContext resolution
	if !contains(sc.TemplateKeys, "welcome") {
		t.Fatalf("TemplateKeys must contain 'welcome', got %v", sc.TemplateKeys)
	}
	langs := sc.ApprovedSpeechKeys["welcome"]
	if !contains(langs, "en-IN") || !contains(langs, "ml-IN") {
		t.Fatalf("ApprovedSpeechKeys[welcome] must contain en-IN and ml-IN, got %v", langs)
	}

	// 2. Exact digest verification
	if sc.ApprovedTemplateSHA["welcome/en-IN"] != digEN {
		t.Fatalf("ApprovedTemplateSHA welcome/en-IN = %q, want %q", sc.ApprovedTemplateSHA["welcome/en-IN"], digEN)
	}
	if sc.ApprovedTemplateSHA["welcome/ml-IN"] != digML {
		t.Fatalf("ApprovedTemplateSHA welcome/ml-IN = %q, want %q", sc.ApprovedTemplateSHA["welcome/ml-IN"], digML)
	}

	gotEN, okEN := sc.TemplateDigest("welcome", "en-IN")
	if !okEN || gotEN != digEN {
		t.Fatalf("TemplateDigest welcome en-IN = (%q, %v), want (%q, true)", gotEN, okEN, digEN)
	}
	gotML, okML := sc.TemplateDigest("welcome", "ml-IN")
	if !okML || gotML != digML {
		t.Fatalf("TemplateDigest welcome ml-IN = (%q, %v), want (%q, true)", gotML, okML, digML)
	}
	if gotUnapproved, ok := sc.TemplateDigest("welcome", "hi-IN"); ok || gotUnapproved != "" {
		t.Fatalf("unapproved language hi-IN must fail closed, got (%q, %v)", gotUnapproved, ok)
	}

	// 3. Synthesize both languages through the actual orchestrator
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	asr.MarkReady()
	mid.MarkReady()
	tts.MarkReady()

	mockWAV := makeMockWAV()
	ttsSum := sha256.Sum256(mockWAV)
	ttsSHA := hex.EncodeToString(ttsSum[:])
	ttsB64 := base64.StdEncoding.EncodeToString(mockWAV)

	ttsCallCount := 0
	tts.SetSynthesizeHook(func(_ context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		ttsCallCount++
		return contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      req.SpeechKey,
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       ttsB64,
			ContentType:    "audio/wav",
			ChecksumSHA256: ttsSHA,
			ModelRevision:  "r0",
			VoiceRevision:  "v0",
		}, nil
	})

	tpls := orchestration.NewMapTemplateRegistry()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: sc.TemplateVersion, SourceVersion: sc.SourceVersion,
		Text: textEN, TemplateSHA256: digEN, SyntheticOnly: false,
	})
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "ml-IN", TemplateVersion: sc.TemplateVersion, SourceVersion: sc.SourceVersion,
		Text: textML, TemplateSHA256: digML, SyntheticOnly: false,
	})

	workers := orchestration.NewWorkers(asr, mid, tts)
	_, _ = workers.SnapshotHealth(context.Background(), orchestration.StageTTS)
	orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
		Limits:      orchestration.DefaultLimits(),
		Workers:     workers,
		Resolver:    resolver,
		Validator:   orchestration.NewProductionValidator(),
		Templates:   tpls,
		Correlation: orchestration.NewCorrelationMap(),
		Metrics:     orchestration.NewMetrics(),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}

	// Synthesize EN
	resEN, err := orch.Synthesize(context.Background(), contracts.TTSRequest{
		RequestID:     "req-en",
		Jurisdiction:  fx.jurisdictionID,
		SpeechKey:     "welcome",
		Language:      "en-IN",
		SourceVersion: sc.SourceVersion,
		Settings:      contracts.TTSSynthesisSettings{SampleRate: 16000, BitDepth: 16, Channels: 1},
	}, nil)
	if err != nil {
		t.Fatalf("Synthesize EN failed: %v", err)
	}
	if resEN.State != contracts.TTSOK || resEN.ChecksumSHA256 != ttsSHA {
		t.Fatalf("unexpected resEN: %+v", resEN)
	}

	// Synthesize ML
	resML, err := orch.Synthesize(context.Background(), contracts.TTSRequest{
		RequestID:     "req-ml",
		Jurisdiction:  fx.jurisdictionID,
		SpeechKey:     "welcome",
		Language:      "ml-IN",
		SourceVersion: sc.SourceVersion,
		Settings:      contracts.TTSSynthesisSettings{SampleRate: 16000, BitDepth: 16, Channels: 1},
	}, nil)
	if err != nil {
		t.Fatalf("Synthesize ML failed: %v", err)
	}
	if resML.State != contracts.TTSOK || resML.ChecksumSHA256 != ttsSHA {
		t.Fatalf("unexpected resML: %+v", resML)
	}

	if ttsCallCount != 2 {
		t.Fatalf("expected 2 TTS worker calls, got %d", ttsCallCount)
	}
}

// TestC03_WrongLanguageOrCrossSourceDigestRejected tests:
// 1. Wrong-language digest rejects before calling TTS worker.
// 2. Cross-source digest rejects before calling TTS worker.
func TestC03_WrongLanguageOrCrossSourceDigestRejected(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	now := nowUTC()
	srcID := findKLP6Source(t, fx.store.DB())

	textEN := "Welcome, citizen."
	textML := "സ്വാഗതം, പൗരനേ."
	sumEN := sha256.Sum256([]byte(textEN))
	sumML := sha256.Sum256([]byte(textML))
	digEN := hex.EncodeToString(sumEN[:])
	digML := hex.EncodeToString(sumML[:])

	// Seed DB with valid approvals for srcID (version 7)
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES
			($1, $2, 'welcome', 'en-IN', 7, 7, $3, $4, 'reviewer-en', 'doc-en', $5),
			($6, $2, 'welcome', 'ml-IN', 7, 7, $3, $7, 'reviewer-ml', 'doc-ml', $5)`,
		"APP-C03-W-EN-"+uid("X"), fx.jurisdictionID, srcID, digEN, now,
		"APP-C03-W-ML-"+uid("X"), digML); err != nil {
		t.Fatalf("seed approvals: %v", err)
	}

	resolver := NewScopedContextResolver(fx.store)

	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	asr.MarkReady()
	mid.MarkReady()
	tts.MarkReady()

	ttsInvoked := false
	tts.SetSynthesizeHook(func(_ context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		ttsInvoked = true
		return contracts.TTSWorkerResponse{State: contracts.TTSOK}, nil
	})

	// Template registry has mismatched digest: EN speech_key holds Malayalam text
	tpls := orchestration.NewMapTemplateRegistry()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 7, SourceVersion: 7,
		Text: textML, TemplateSHA256: digML, // Mismatched text/digest for en-IN!
	})

	workers := orchestration.NewWorkers(asr, mid, tts)
	_, _ = workers.SnapshotHealth(context.Background(), orchestration.StageTTS)
	orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
		Limits:      orchestration.DefaultLimits(),
		Workers:     workers,
		Resolver:    resolver,
		Validator:   orchestration.NewProductionValidator(),
		Templates:   tpls,
		Correlation: orchestration.NewCorrelationMap(),
		Metrics:     orchestration.NewMetrics(),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}

	// Case 1: Wrong-language digest must fail closed before TTS
	_, err = orch.Synthesize(context.Background(), contracts.TTSRequest{
		RequestID:     "req-mismatch",
		Jurisdiction:  fx.jurisdictionID,
		SpeechKey:     "welcome",
		Language:      "en-IN",
		SourceVersion: 7,
		Settings:      contracts.TTSSynthesisSettings{SampleRate: 16000, BitDepth: 16, Channels: 1},
	}, nil)
	if err == nil {
		t.Fatalf("expected digest mismatch to fail Synthesize")
	}
	if ttsInvoked {
		t.Fatalf("TTS worker must NOT be called when digest mismatch is detected")
	}
	var pe *orchestration.PipelineError
	if !errors.As(err, &pe) || pe.Failures[0].Code != contracts.ErrTemplateUnknown {
		t.Fatalf("expected ErrTemplateUnknown failure, got %v", err)
	}

	// Case 2: Cross-source approval cannot authorize speech for another source
	// Insert an unlinked source and give it an approval for "welcome" in "hi-IN"
	otherSrc := "SRC-OTHER-" + uid("X")
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at)
		VALUES ($1, 'other-gov', 'other.gov.example', 'OPERATIONAL', 1, $2)`,
		otherSrc, now); err != nil {
		t.Fatalf("create other source: %v", err)
	}
	digHI := hex.EncodeToString(sumEN[:])
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES ($1, $2, 'welcome', 'hi-IN', 7, 7, $3, $4, 'reviewer-hi', 'doc-hi', $5)`,
		"APP-C03-CROSS-"+uid("X"), fx.jurisdictionID, otherSrc, digHI, now); err != nil {
		t.Fatalf("seed cross-source approval: %v", err)
	}

	// Re-resolve active context for the package (bound to srcID)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// "hi-IN" must NOT be authorized because it belongs to otherSrc
	if sc.IsSpeechKeyApprovedForLanguage("welcome", "hi-IN") {
		t.Fatalf("cross-source approval must not authorize language")
	}
	if dig, ok := sc.TemplateDigest("welcome", "hi-IN"); ok || dig != "" {
		t.Fatalf("cross-source approval must not provide template digest")
	}
}

// TestC03_SnapshotRevalidate_LanguageRevocationAndDrift tests that changing or
// revoking either language during slow work fails SnapshotRevalidate.
func TestC03_SnapshotRevalidate_LanguageRevocationAndDrift(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	now := nowUTC()
	srcID := findKLP6Source(t, fx.store.DB())

	textEN := "Welcome, citizen."
	textML := "സ്വാഗതം, പൗരനേ."
	sumEN := sha256.Sum256([]byte(textEN))
	sumML := sha256.Sum256([]byte(textML))
	digEN := hex.EncodeToString(sumEN[:])
	digML := hex.EncodeToString(sumML[:])

	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES
			($1, $2, 'welcome', 'en-IN', 7, 7, $3, $4, 'reviewer-en', 'doc-en', $5),
			($6, $2, 'welcome', 'ml-IN', 7, 7, $3, $7, 'reviewer-ml', 'doc-ml', $5)`,
		"APP-C03-REV-EN-"+uid("X"), fx.jurisdictionID, srcID, digEN, now,
		"APP-C03-REV-ML-"+uid("X"), digML); err != nil {
		t.Fatalf("seed approvals: %v", err)
	}

	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// 1. Initial snapshot passes revalidate
	if err := resolver.SnapshotRevalidate(context.Background(), sc); err != nil {
		t.Fatalf("initial SnapshotRevalidate must pass: %v", err)
	}

	// 2. Revoking ml-IN during work causes SnapshotRevalidate to fail
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		UPDATE approved_translations SET revoked_at = $1
		WHERE jurisdiction = $2 AND speech_key = 'welcome' AND language = 'ml-IN'`,
		now, fx.jurisdictionID); err != nil {
		t.Fatalf("revoke ml-IN: %v", err)
	}
	if err := resolver.SnapshotRevalidate(context.Background(), sc); err == nil {
		t.Fatalf("revoking ml-IN must fail SnapshotRevalidate")
	}

	// 3. Un-revoke ml-IN and alter en-IN digest (text drift on disk)
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		UPDATE approved_translations SET revoked_at = NULL
		WHERE jurisdiction = $1 AND speech_key = 'welcome' AND language = 'ml-IN'`,
		fx.jurisdictionID); err != nil {
		t.Fatalf("un-revoke ml-IN: %v", err)
	}
	otherSHA := hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")[:32])
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		UPDATE approved_translations SET template_sha256 = $1
		WHERE jurisdiction = $2 AND speech_key = 'welcome' AND language = 'en-IN'`,
		otherSHA, fx.jurisdictionID); err != nil {
		t.Fatalf("drift en-IN: %v", err)
	}
	if err := resolver.SnapshotRevalidate(context.Background(), sc); err == nil {
		t.Fatalf("digest drift for en-IN must fail SnapshotRevalidate")
	}
}

// TestC03_RowOrderIndependence tests that resolving templates is 100% independent
// of DB row return order.
func TestC03_RowOrderIndependence(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	now := nowUTC()
	srcID := findKLP6Source(t, fx.store.DB())

	digEN := orchestrationtest.DigestString("Welcome, citizen.")
	digML := orchestrationtest.DigestString("സ്വാഗതം, പൗരനേ.")

	// Insert ml-IN first, en-IN second (version 7)
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES
			($1, $2, 'welcome', 'ml-IN', 7, 7, $3, $4, 'reviewer-ml', 'doc-ml', $5),
			($6, $2, 'welcome', 'en-IN', 7, 7, $3, $7, 'reviewer-en', 'doc-en', $5)`,
		"APP-C03-ORD-ML-"+uid("X"), fx.jurisdictionID, srcID, digML, now,
		"APP-C03-ORD-EN-"+uid("X"), digEN); err != nil {
		t.Fatalf("seed approvals: %v", err)
	}

	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Neither row overwrote the other
	if sc.ApprovedTemplateSHA["welcome/en-IN"] != digEN {
		t.Errorf("welcome/en-IN = %q, want %q", sc.ApprovedTemplateSHA["welcome/en-IN"], digEN)
	}
	if sc.ApprovedTemplateSHA["welcome/ml-IN"] != digML {
		t.Errorf("welcome/ml-IN = %q, want %q", sc.ApprovedTemplateSHA["welcome/ml-IN"], digML)
	}
	dEN, okEN := sc.TemplateDigest("welcome", "en-IN")
	if !okEN || dEN != digEN {
		t.Errorf("TemplateDigest welcome/en-IN = (%q, %v), want (%q, true)", dEN, okEN, digEN)
	}
	dML, okML := sc.TemplateDigest("welcome", "ml-IN")
	if !okML || dML != digML {
		t.Errorf("TemplateDigest welcome/ml-IN = (%q, %v), want (%q, true)", dML, okML, digML)
	}
}

// TestC03_ConflictingActiveApprovals_FailsClosed tests that duplicate active approvals
// for the same (jurisdiction, source_version, template_version, source_id, speech_key, language)
// with conflicting digests fail closed instead of arbitrarily picking one.
func TestC03_ConflictingActiveApprovals_FailsClosed(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	now := nowUTC()
	srcID := findKLP6Source(t, fx.store.DB())

	dig1 := orchestrationtest.DigestString("Welcome version 1.")
	dig2 := orchestrationtest.DigestString("Welcome version 2.")

	// Find the unique constraint on approved_translations in the real DB and drop it
	// to test defense-in-depth against corrupt/duplicated rows.
	var conName string
	_ = fx.store.DB().QueryRowContext(context.Background(), `
		SELECT constraint_name FROM information_schema.table_constraints
		WHERE table_name = 'approved_translations' AND constraint_type = 'UNIQUE'
		LIMIT 1`).Scan(&conName)
	if conName != "" {
		if _, err := fx.store.DB().ExecContext(context.Background(), fmt.Sprintf("ALTER TABLE approved_translations DROP CONSTRAINT %q", conName)); err != nil {
			t.Fatalf("drop constraint: %v", err)
		}
	}

	// Insert conflicting active rows with differing template_sha256 (version 7)
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES
			($1, $2, 'welcome', 'en-IN', 7, 7, $3, $4, 'reviewer-1', 'doc-1', $5),
			($6, $2, 'welcome', 'en-IN', 7, 7, $3, $7, 'reviewer-2', 'doc-2', $5)`,
		"APP-C03-CONF-1-"+uid("X"), fx.jurisdictionID, srcID, dig1, now,
		"APP-C03-CONF-2-"+uid("X"), dig2); err != nil {
		t.Fatalf("seed conflicting rows: %v", err)
	}

	resolver := NewScopedContextResolver(fx.store)
	_, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err == nil {
		t.Fatalf("expected Resolve to fail closed on conflicting active approvals")
	}
	if !strings.Contains(err.Error(), "conflicting active approvals") {
		t.Fatalf("expected error message to mention conflicting active approvals, got: %v", err)
	}
}

// TestC03_ExactLanguageBoundary_RejectsMissingTupleAndFlatKey proves that:
// 1. Correct (speech_key, language) tuple resolves successfully.
// 2. Missing tuple key rejects even if language is in ApprovedSpeechKeys and a flat key exists.
// 3. Unapproved language rejects.
// 4. Legacy flat key alone fails closed without falling back.
func TestC03_ExactLanguageBoundary_RejectsMissingTupleAndFlatKey(t *testing.T) {
	sc := contracts.ScopedContext{
		AllowedLanguages: []string{"en-IN", "ml-IN"},
		TemplateKeys:     []string{"welcome"},
		ApprovedSpeechKeys: map[string][]string{
			"welcome": {"en-IN", "ml-IN"},
		},
		ApprovedTemplateSHA: map[string]string{
			"welcome":       "flat-digest",
			"welcome/en-IN": "en-digest",
		},
	}

	// 1. Correct EN tuple passes
	digEN, okEN := sc.TemplateDigest("welcome", "en-IN")
	if !okEN || digEN != "en-digest" {
		t.Fatalf("expected en-digest, got (%q, %v)", digEN, okEN)
	}

	// 2. Missing ML tuple rejects EVEN THOUGH "welcome" flat key exists and ml-IN is in ApprovedSpeechKeys
	digML, okML := sc.TemplateDigest("welcome", "ml-IN")
	if okML || digML != "" {
		t.Fatalf("expected missing tuple (welcome, ml-IN) to reject, but got (%q, %v)", digML, okML)
	}

	// 3. Wrong-language (not in ApprovedSpeechKeys) rejects
	digHI, okHI := sc.TemplateDigest("welcome", "hi-IN")
	if okHI || digHI != "" {
		t.Fatalf("expected unapproved language to reject, but got (%q, %v)", digHI, okHI)
	}

	// 4. Legacy flat key alone without tuple must fail closed
	scFlatOnly := contracts.ScopedContext{
		AllowedLanguages: []string{"en-IN"},
		TemplateKeys:     []string{"welcome"},
		ApprovedSpeechKeys: map[string][]string{
			"welcome": {"en-IN"},
		},
		ApprovedTemplateSHA: map[string]string{
			"welcome": "flat-digest",
		},
	}
	digFlat, okFlat := scFlatOnly.TemplateDigest("welcome", "en-IN")
	if okFlat || digFlat != "" {
		t.Fatalf("expected flat key alone to reject, but got (%q, %v)", digFlat, okFlat)
	}
}

