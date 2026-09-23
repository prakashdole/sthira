// Package orchestrationtest provides test doubles for the
// orchestration package: fake workers (ASR/Middle/TTS), a fake
// resolver, a fake validator, and a fake template registry. It
// exists so the orchestrator's package-internal fakes (in
// fakes_test.go) stay test-only, while external packages (such as
// httpserver tests) can wire a real orchestrator through public
// constructors.
package orchestrationtest

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
)

// Worker is the orchestrationtest fake worker. It implements
// orchestration.WorkerClient by composing three per-method hooks
// and the Health signature.
type Worker struct {
	mu sync.Mutex

	ready atomic.Bool

	transcribeCalls atomic.Int64
	proposeCalls    atomic.Int64
	synthesizeCalls atomic.Int64

	transcribeHook func(ctx context.Context, req contracts.ASRWorkerRequest) (contracts.ASRWorkerResponse, error)
	proposeHook    func(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error)
	synthesizeHook func(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error)

	audioBytes    []byte
	languages     []string
	audioSettings contracts.TTSSynthesisSettings
}

// NewWorker returns a fake worker with sensible defaults. The audio
// default is a canonical 1-second silence WAV at the request's
// declared rate (16000 Hz mono PCM16), so the orchestrator's
// declared-settings vs RIFF-header check passes for the default
// happy path. Tests that need a mismatch install a custom
// synthesizeHook or override the audio bytes.
func NewWorker() *Worker {
	return &Worker{
		languages:  []string{"en-IN", "hi-IN"},
		audioBytes: silenceWAV(16000, 1),
		audioSettings: contracts.TTSSynthesisSettings{
			SampleRate: 16000, BitDepth: 16, Channels: 1,
		},
	}
}

// silenceWAV produces a canonical PCM 16-bit LE mono WAV holding
// `seconds` of zeros at `rate`. The orchestrator's RIFF-header
// validator requires PCM format=1, 16-bit, mono; this helper emits
// exactly that.
func silenceWAV(rate int, seconds float64) []byte {
	samples := int(float64(rate) * seconds)
	dataLen := samples * 2
	total := 36 + dataLen
	b := make([]byte, 44+dataLen)
	copy(b[0:4], "RIFF")
	binary.LittleEndian.PutUint32(b[4:8], uint32(total))
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	binary.LittleEndian.PutUint32(b[16:20], 16)
	binary.LittleEndian.PutUint16(b[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(b[22:24], 1) // mono
	binary.LittleEndian.PutUint32(b[24:28], uint32(rate))
	binary.LittleEndian.PutUint32(b[28:32], uint32(rate*2))
	binary.LittleEndian.PutUint16(b[32:34], 2)
	binary.LittleEndian.PutUint16(b[34:36], 16)
	copy(b[36:40], "data")
	binary.LittleEndian.PutUint32(b[40:44], uint32(dataLen))
	// Samples default to zero.
	return b
}

// MarkReady sets the worker's reported /health state to ready+warm.
func (w *Worker) MarkReady()    { w.ready.Store(true) }
func (w *Worker) MarkNotReady() { w.ready.Store(false) }

// Health implements orchestration.WorkerClient.
func (w *Worker) Health(_ context.Context) (contracts.WorkerHealth, bool) {
	return contracts.WorkerHealth{
		Ready:              w.ready.Load(),
		Warm:               w.ready.Load(),
		SupportedLanguages: w.languages,
		Queue:              contracts.QueueStats{Depth: 0, MaxDepth: 4, MaxConcurrency: 2},
		Models: []contracts.ModelInfo{{
			ModelID:    "fake-model",
			Revision:   "r0",
			License:    "Apache-2.0",
			Runtime:    "fake",
			Hardware:   "cpu",
			RemoteCode: false,
		}},
	}, true
}

// Transcribe implements orchestration.WorkerClient.
func (w *Worker) Transcribe(ctx context.Context, req contracts.ASRWorkerRequest) (contracts.ASRWorkerResponse, error) {
	w.transcribeCalls.Add(1)
	if err := ctx.Err(); err != nil {
		return contracts.ASRWorkerResponse{}, err
	}
	if w.transcribeHook != nil {
		return w.transcribeHook(ctx, req)
	}
	return contracts.ASRWorkerResponse{
		RequestID:     req.RequestID,
		Language:      req.Language,
		Text:          "hello",
		State:         contracts.TranscriptionOK,
		ModelRevision: "r0",
	}, nil
}

// Propose implements orchestration.WorkerClient.
func (w *Worker) Propose(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
	w.proposeCalls.Add(1)
	if err := ctx.Err(); err != nil {
		return contracts.MiddleWorkerResponse{}, err
	}
	if w.proposeHook != nil {
		return w.proposeHook(ctx, req)
	}
	return contracts.MiddleWorkerResponse{
		RequestID:     req.RequestID,
		DataVersion:   req.ScopedContext.DataVersion,
		ModelRevision: "r0",
		Proposal: contracts.ModelOutput{
			SchemaVersion:    contracts.ModelSchemaVersion,
			RequestID:        req.RequestID,
			DataVersion:      req.ScopedContext.DataVersion,
			Status:           contracts.StatusOK,
			Intent:           IntentPtr(contracts.IntentRecenter),
			Language:         req.Transcript.Language,
			Actions:          []contracts.Action{{Type: contracts.ActionRecenter}},
			ClarificationIDs: nil,
			EvidenceIDs:      []string{req.ScopedContext.RequestID},
		},
	}, nil
}

// Synthesize implements orchestration.WorkerClient.
func (w *Worker) Synthesize(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
	w.synthesizeCalls.Add(1)
	if err := ctx.Err(); err != nil {
		return contracts.TTSWorkerResponse{}, err
	}
	if w.synthesizeHook != nil {
		return w.synthesizeHook(ctx, req)
	}
	sum := sha256.Sum256(w.audioBytes)
	return contracts.TTSWorkerResponse{
		RequestID:      req.RequestID,
		SpeechKey:      req.SpeechKey,
		Language:       req.Language,
		State:          contracts.TTSOK,
		AudioB64:       base64.StdEncoding.EncodeToString(w.audioBytes),
		ContentType:    "audio/wav",
		ChecksumSHA256: hex.EncodeToString(sum[:]),
		ModelRevision:  "r0",
		VoiceRevision:  "v0",
		Settings:       w.audioSettings,
	}, nil
}

// Calls returns the per-method call counts.
func (w *Worker) Calls() (asr, mid, tts int64) {
	return w.transcribeCalls.Load(), w.proposeCalls.Load(), w.synthesizeCalls.Load()
}

// TranscribeCalls returns the cumulative Transcribe call count.
func (w *Worker) TranscribeCalls() int64 { return w.transcribeCalls.Load() }

// ProposeCalls returns the cumulative Propose call count.
func (w *Worker) ProposeCalls() int64 { return w.proposeCalls.Load() }

// SynthesizeCalls returns the cumulative Synthesize call count.
func (w *Worker) SynthesizeCalls() int64 { return w.synthesizeCalls.Load() }

// SetProposeHook installs a custom Propose hook.
func (w *Worker) SetProposeHook(hook func(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error)) {
	w.proposeHook = hook
}

// SetTranscribeHook installs a custom Transcribe hook.
func (w *Worker) SetTranscribeHook(hook func(ctx context.Context, req contracts.ASRWorkerRequest) (contracts.ASRWorkerResponse, error)) {
	w.transcribeHook = hook
}

// SetSynthesizeHook installs a custom Synthesize hook.
func (w *Worker) SetSynthesizeHook(hook func(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error)) {
	w.synthesizeHook = hook
}

// IntentPtr returns a pointer to the given Intent constant. The
// orchestrator package also defines this privately; we expose a
// thin alias for the test package's convenience.
func IntentPtr(i contracts.Intent) *contracts.Intent { return &i }

// Resolver is the orchestrationtest fake ScopedResolver.
type Resolver struct {
	mu              sync.Mutex
	scoped          contracts.ScopedContext
	contexts        map[string]contracts.ScopedContext
	revalidateError error
	revalidateCalls atomic.Int64
	revalidateHook  func(ctx context.Context, sc contracts.ScopedContext) error
}

// NewResolver returns a resolver seeded with the given scoped context.
func NewResolver(sc contracts.ScopedContext) *Resolver {
	r := &Resolver{scoped: sc, contexts: make(map[string]contracts.ScopedContext)}
	if sc.Jurisdiction != "" {
		r.contexts[sc.Jurisdiction] = sc
	}
	return r
}

// AddJurisdiction registers an additional operational context for a jurisdiction.
func (r *Resolver) AddJurisdiction(sc contracts.ScopedContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.contexts == nil {
		r.contexts = make(map[string]contracts.ScopedContext)
	}
	r.contexts[sc.Jurisdiction] = sc
}

// Resolve implements orchestration.ScopedContextResolver.
func (r *Resolver) Resolve(_ context.Context, jurisdiction string) (contracts.ScopedContext, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sc, ok := r.contexts[jurisdiction]; ok {
		return sc, nil
	}
	if len(r.contexts) > 0 && r.scoped.Jurisdiction != "" && jurisdiction != r.scoped.Jurisdiction {
		return contracts.ScopedContext{}, errors.New("resolver: no operational context for jurisdiction")
	}
	if r.scoped.Jurisdiction == "" {
		r.scoped.Jurisdiction = jurisdiction
	}
	return r.scoped, nil
}

// SnapshotRevalidate implements orchestration.ScopedRevalidator.
func (r *Resolver) SnapshotRevalidate(ctx context.Context, sc contracts.ScopedContext) error {
	r.revalidateCalls.Add(1)
	if r.revalidateHook != nil {
		return r.revalidateHook(ctx, sc)
	}
	return r.revalidateError
}

// SetRevalidateHook installs a custom SnapshotRevalidate hook.
func (r *Resolver) SetRevalidateHook(hook func(ctx context.Context, sc contracts.ScopedContext) error) {
	r.revalidateHook = hook
}

// SetRevalidateError sets the error returned by SnapshotRevalidate.
func (r *Resolver) SetRevalidateError(err error) {
	r.revalidateError = err
}

// Validator is the orchestrationtest fake VoiceValidator.
type Validator struct {
	mu         sync.Mutex
	enforced   atomic.Int64
	enforceErr error
	shapeErr   error
}

// NewValidator returns a validator that always passes.
func NewValidator() *Validator { return &Validator{} }

// ValidateShape implements orchestration.VoiceValidator.
func (v *Validator) ValidateShape(_ []byte) error { return v.shapeErr }

// Enforce implements orchestration.VoiceValidator.
func (v *Validator) Enforce(_ contracts.ModelOutput, _ contracts.ScopedContext) error {
	v.enforced.Add(1)
	return v.enforceErr
}

// FlatValidate implements orchestration.VoiceValidator.
func (v *Validator) FlatValidate(contracts.ModelOutput, string, string, map[string]bool, map[string]bool) error {
	return nil
}

// SetEnforceError sets the error returned by Enforce.
func (v *Validator) SetEnforceError(err error) { v.enforceErr = err }

// Templates is the orchestrationtest fake TemplateRegistry.
type Templates struct {
	mu   sync.Mutex
	tpls map[string]contracts.ApprovedTemplate
}

// NewTemplates returns an empty registry.
func NewTemplates() *Templates {
	return &Templates{tpls: map[string]contracts.ApprovedTemplate{}}
}

// Add registers an approved template by (key, language).
func (r *Templates) Add(t contracts.ApprovedTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tpls[t.SpeechKey+"/"+t.Language] = t
}

// Lookup implements orchestration.TemplateRegistry.
func (r *Templates) Lookup(speechKey, language string) (contracts.ApprovedTemplate, bool) {
	t, ok := r.tpls[speechKey+"/"+language]
	return t, ok
}

// Keys implements orchestration.TemplateRegistry.
func (r *Templates) Keys() []string {
	out := make([]string, 0, len(r.tpls))
	for k := range r.tpls {
		for i, c := range k {
			if c == '/' {
				out = append(out, k[:i])
				break
			}
		}
	}
	return out
}

// Build constructs a typed ScopedContext for tests.
func BuildScopedContext(jurisdiction, language string) contracts.ScopedContext {
	// Digests for the two default keys' canonical test texts. Tests that
	// register a different Text must update ApprovedTemplateSHA accordingly.
	welcomeSHA := DigestString("Welcome, citizen.")
	destSHA := DigestString("Destination choices are displayed on screen.")
	return contracts.ScopedContext{
		RequestID:        "sc-" + jurisdiction,
		DataVersion:      "PKG-1:1",
		Jurisdiction:     jurisdiction,
		SchemaVersion:    contracts.SchemaVersionV3,
		SourceID:         "SRC-1",
		SourceStatus:     contracts.FreshnessCurrent,
		SourceVersion:    1,
		TemplateVersion:  1,
		AllowedLanguages: []string{language},
		TemplateKeys:     []string{"welcome", "destination_options"},
		ApprovedSpeechKeys: map[string][]string{
			"welcome":             {language},
			"destination_options": {language},
		},
		ApprovedTemplateSHA: map[string]string{
			"welcome":             welcomeSHA,
			"destination_options": destSHA,
		},
		IssuedAt: "2026-09-21T00:00:00Z",
	}
}

// DigestString is the SHA-256 hex of a canonical template string.
func DigestString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// NewOrchestrator wires a fresh orchestrator with the supplied
// fakes. Defaults match DefaultLimits().
func NewOrchestrator(asr, mid, tts *Worker, resolver *Resolver, validator *Validator, tpls *Templates) (*orchestration.Orchestrator, error) {
	asr.MarkReady()
	mid.MarkReady()
	tts.MarkReady()
	workers := orchestration.NewWorkers(asr, mid, tts)
	// SnapshotHealth transitions the workers' internal ready state
	// so the orchestrator's IsReady short-circuit returns true.
	workers.SnapshotHealth(context.Background(), orchestration.StageASR)
	workers.SnapshotHealth(context.Background(), orchestration.StageMiddle)
	workers.SnapshotHealth(context.Background(), orchestration.StageTTS)
	cfg := orchestration.PipelineConfig{
		Limits:      orchestration.DefaultLimits(),
		Workers:     workers,
		Resolver:    resolver,
		Validator:   validator,
		Templates:   tpls,
		Correlation: orchestration.NewCorrelationMap(),
		Metrics:     orchestration.NewMetrics(),
	}
	return orchestration.NewOrchestrator(cfg)
}
