// Package provider's HTTP client speaks the frozen P6 contract
// shapes from backend/internal/contracts. The eval module is
// stdlib-only and cannot import the contracts package directly,
// so the wire types below mirror the contract fields one-for-one.
//
// Wire contracts (this file is the source of truth for what the
// harness sends; backend/internal/contracts/*.go is the source
// of truth for what the orchestrator/worker accepts):
//
//	POST {ASRURL}                  ASRWorkerRequest JSON
//	                               -> ASRWorkerResponse JSON
//
//	POST {MiddleURL}               PipelineRequest JSON
//	                               -> PipelineResponse JSON
//	                               (the typed envelope, NOT a
//	                               status/intent/actions flat
//	                               object; the runner extracts
//	                               from validated_proposal)
//
//	POST {TTSURL}                  TTSWorkerRequest JSON
//	                               -> TTSWorkerResponse JSON
//
// All three MUST carry: request_id (round-trip), language
// (where supported), and the actual jurisdiction + source_version
// fields where the contract requires them.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// HTTPConfig configures the HTTP provider. URLs are passed in by
// main, never hard-coded.
type HTTPConfig struct {
	ASRURL     string // POST ASRWorkerRequest JSON
	MiddleURL  string // POST PipelineRequest JSON
	TTSURL     string // POST TTSWorkerRequest JSON
	AuthBearer string // optional
	Timeout    time.Duration
}

// HTTPProvider speaks the frozen wire shapes over HTTP. It is the
// only real-run Provider.
type HTTPProvider struct {
	cfg    HTTPConfig
	client *http.Client
}

// NewHTTP returns a Provider whose Mode is ModeHTTP.
func NewHTTP(cfg HTTPConfig) *HTTPProvider {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &HTTPProvider{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (p *HTTPProvider) Mode() Mode { return ModeHTTP }

// ASR POSTs the typed ASRWorkerRequest envelope (mirrors
// contracts.ASRWorkerRequest) and returns the typed
// ASRWorkerResponse. Audio bytes are sent as base64 in the
// "audio_b64" field of the JSON body — same wire shape the
// orchestrator uses when calling private workers.
func (p *HTTPProvider) ASR(ctx context.Context, req ASRRequest) (ASROutcome, error) {
	if p.cfg.ASRURL == "" {
		return ASROutcome{}, errors.New("HTTP ASR URL not configured")
	}
	audioBytes, err := decodeAudioB64(req.AudioB64)
	if err != nil {
		return ASROutcome{}, fmt.Errorf("decode audio: %w", err)
	}
	wire := map[string]any{
		"request_id":      req.RequestID,
		"language":        req.Language,
		"content_type":    req.ContentType,
		"audio_b64":       req.AudioB64, // server can decode without us round-tripping
		"byte_size":       int64(len(audioBytes)),
		"decoded_seconds": 0.0, // server recomputes
		"deadline_ms":     5000,
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return ASROutcome{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.ASRURL, bytes.NewReader(body))
	if err != nil {
		return ASROutcome{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	if req.RequestID != "" {
		hreq.Header.Set("X-Sthira-Request-ID", req.RequestID)
	}
	if req.Language != "" {
		hreq.Header.Set("X-Sthira-Language", req.Language)
	}
	if p.cfg.AuthBearer != "" {
		hreq.Header.Set("Authorization", "Bearer "+p.cfg.AuthBearer)
	}
	resp, err := p.client.Do(hreq)
	if err != nil {
		return ASROutcome{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return ASROutcome{}, httpStatusError(resp.StatusCode)
	}
	rawBody, err := boundedRead(resp.Body, 64*1024)
	if err != nil {
		return ASROutcome{}, fmt.Errorf("decode ASR json: %w", err)
	}
	var out ASRWorkerResponseWire
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return ASROutcome{}, fmt.Errorf("decode ASR json: %w", err)
	}
	if out.RequestID != "" && out.RequestID != req.RequestID {
		return ASROutcome{}, fmt.Errorf("asr: request_id mismatch got=%q want=%q",
			out.RequestID, req.RequestID)
	}
	return ASROutcome{
		State:      string(out.State),
		Text:       out.Text,
		Language:   out.Language,
		Confidence: out.Confidence,
		RevisionID: out.ModelRevision,
	}, nil
}

// Middle POSTs the real private middle-worker envelope
// (middleworker.RequestEnvelope, mirrored here) to the worker's
// /v1/chat/completions endpoint and decodes the real
// ResponseEnvelope. It does NOT send a public PipelineRequest to a
// private worker, and it does NOT invent fields the envelope does
// not have (no top-level source_version, no data_version_hint):
// versioning travels inside scoped_context where the worker reads
// it. The proposal is decoded NESTED (out.Proposal), never flat.
func (p *HTTPProvider) Middle(ctx context.Context, req MiddleRequest) (MiddleOutcome, error) {
	if p.cfg.MiddleURL == "" {
		return MiddleOutcome{}, errors.New("HTTP Middle URL not configured")
	}
	// The transcript round-trips correlation: same request_id the
	// ASR stage used. The middle wire schema_version is the frozen
	// 3.0 model-output contract (NOT the corpus 1.0.0 format).
	dataVersion := req.Case.Context.DataVersion
	if dataVersion == "" {
		return MiddleOutcome{}, errors.New("middle: context.data_version required")
	}
	if req.Case.Context.Jurisdiction == "" {
		return MiddleOutcome{}, errors.New("middle: context.jurisdiction required (D36)")
	}
	scoped := map[string]any{
		"schema_version": "3.0",
		"data_version":   dataVersion,
		"jurisdiction":   req.Case.Context.Jurisdiction,
	}
	if req.Case.Context.SourceVersion > 0 {
		scoped["source_version"] = req.Case.Context.SourceVersion
	}
	if req.Case.Context.TemplateVersion > 0 {
		scoped["template_version"] = req.Case.Context.TemplateVersion
	}
	if len(req.Case.Context.TemplateKeys) > 0 {
		scoped["template_keys"] = req.Case.Context.TemplateKeys
	}
	if req.Case.Context.Language != "" {
		scoped["allowed_languages"] = []string{req.Case.Context.Language}
	}
	wire := map[string]any{
		"request_id":     req.RequestID,
		"scoped_context": scoped,
		"transcript": map[string]any{
			"request_id": req.RequestID,
			"language":   req.Language,
			"text":       req.Transcript,
			"state":      "OK",
		},
		"max_output_tokens": 256,
		"deadline_ms":       5000,
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return MiddleOutcome{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.MiddleURL, bytes.NewReader(body))
	if err != nil {
		return MiddleOutcome{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	if req.RequestID != "" {
		hreq.Header.Set("X-Sthira-Request-ID", req.RequestID)
	}
	if p.cfg.AuthBearer != "" {
		hreq.Header.Set("Authorization", "Bearer "+p.cfg.AuthBearer)
	}
	resp, err := p.client.Do(hreq)
	if err != nil {
		return MiddleOutcome{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return MiddleOutcome{}, httpStatusError(resp.StatusCode)
	}
	rawBody, err := boundedRead(resp.Body, 256*1024)
	if err != nil {
		return MiddleOutcome{}, fmt.Errorf("decode middle json: %w", err)
	}
	var out MiddleWorkerResponseWire
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return MiddleOutcome{}, fmt.Errorf("decode middle json: %w", err)
	}
	if out.RequestID != "" && out.RequestID != req.RequestID {
		return MiddleOutcome{}, fmt.Errorf("middle: request_id mismatch got=%q want=%q",
			out.RequestID, req.RequestID)
	}
	if out.Proposal.SchemaVersion == "" || out.Proposal.Status == "" {
		// A 200 whose nested proposal is empty is NOT a success
		// with nil error (prompt item 5).
		return MiddleOutcome{}, fmt.Errorf("%w: middle worker returned 200 with empty proposal", ErrMalformedResponse)
	}
	if out.Proposal.RequestID != req.RequestID {
		return MiddleOutcome{}, fmt.Errorf("middle: proposal request_id mismatch got=%q want=%q",
			out.Proposal.RequestID, req.RequestID)
	}
	status := out.Proposal.Status
	intent := ""
	if out.Proposal.Intent != nil {
		intent = *out.Proposal.Intent
	}
	var actions []string
	for _, a := range out.Proposal.Actions {
		actions = append(actions, actionToString(a))
	}
	speechKey := ""
	if out.Proposal.SpeechKey != nil {
		speechKey = *out.Proposal.SpeechKey
	}
	return MiddleOutcome{
		Status:      status,
		Intent:      intent,
		Language:    out.Proposal.Language,
		Actions:     actions,
		SpeechKey:   speechKey,
		ClarifyIDs:  out.Proposal.ClarificationIDs,
		EvidenceIDs: out.Proposal.EvidenceIDs,
	}, nil
}

// TTS POSTs the typed TTSWorkerRequest envelope (mirrors
// middleworker/ttsworker SynthesizeRequest) to the TTS WORKER's
// private /synthesize endpoint. This is the worker-conformance
// boundary: the harness must supply the ACTUAL rendered template
// text (the worker only synthesizes text it was given) and the
// real source_version/template_version, and must never invent
// version 1. The public /api/v3/voice/speech orchestration host is
// exercised by Worker 1's integrated conformance, not here.
func (p *HTTPProvider) TTS(ctx context.Context, req TTSRequest) (TTSOutcome, error) {
	if p.cfg.TTSURL == "" {
		return TTSOutcome{}, errors.New("HTTP TTS URL not configured")
	}
	if req.SpeechKey == "" {
		return TTSOutcome{}, errors.New("tts: speech_key required")
	}
	if req.Text == "" {
		return TTSOutcome{}, errors.New("tts: rendered text required (worker synthesizes given text only; never fabricated)")
	}
	if req.SourceVersion <= 0 {
		return TTSOutcome{}, errors.New("tts: source_version required (never invented)")
	}
	if req.TemplateVersion <= 0 {
		return TTSOutcome{}, errors.New("tts: template_version required (never invented)")
	}
	settings := map[string]any{}
	if req.SampleRate > 0 {
		settings["sample_rate"] = req.SampleRate
		settings["bit_depth"] = 16
		settings["channels"] = 1
	}
	wire := map[string]any{
		"request_id":       req.RequestID,
		"speech_key":       req.SpeechKey,
		"language":         req.Language,
		"text":             req.Text,
		"source_version":   req.SourceVersion,
		"template_version": req.TemplateVersion,
		"deadline_ms":      5000,
	}
	if req.Voice != "" {
		wire["voice"] = req.Voice
	}
	if len(settings) > 0 {
		wire["settings"] = settings
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return TTSOutcome{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TTSURL, bytes.NewReader(body))
	if err != nil {
		return TTSOutcome{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	if req.RequestID != "" {
		hreq.Header.Set("X-Sthira-Request-ID", req.RequestID)
	}
	if p.cfg.AuthBearer != "" {
		hreq.Header.Set("Authorization", "Bearer "+p.cfg.AuthBearer)
	}
	resp, err := p.client.Do(hreq)
	if err != nil {
		return TTSOutcome{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return TTSOutcome{}, httpStatusError(resp.StatusCode)
	}
	rawBody, err := boundedRead(resp.Body, 1024*1024)
	if err != nil {
		return TTSOutcome{}, fmt.Errorf("decode tts json: %w", err)
	}
	var out TTSWorkerResponseWire
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return TTSOutcome{}, fmt.Errorf("decode tts json: %w", err)
	}
	if out.RequestID != "" && out.RequestID != req.RequestID {
		return TTSOutcome{}, fmt.Errorf("tts: request_id mismatch got=%q want=%q",
			out.RequestID, req.RequestID)
	}
	if out.State == "" {
		return TTSOutcome{}, fmt.Errorf("%w: tts worker returned 200 with empty state", ErrMalformedResponse)
	}
	if out.State == "OK" && out.ByteSize <= 0 {
		return TTSOutcome{}, fmt.Errorf("%w: tts OK without bytes", ErrMalformedResponse)
	}
	return TTSOutcome{
		State:      out.State,
		SpeechKey:  out.SpeechKey,
		ByteSize:   int(out.ByteSize),
		DurationMS: 0, // TTSWorkerResponse carries no duration; PipelineAudio (public host) does
	}, nil
}

// --- wire shapes (mirror of the actual worker servers; eval module
// is stdlib-only and cannot import the worker packages) ---

// ASRWorkerResponseWire mirrors the asrworker /transcribe
// responseJSON (contracts.ASRWorkerResponse).
type ASRWorkerResponseWire struct {
	RequestID      string   `json:"request_id"`
	Language       string   `json:"language"`
	Text           string   `json:"text"`
	Confidence     *float64 `json:"confidence,omitempty"`
	State          string   `json:"state"`
	ModelRevision  string   `json:"model_revision,omitempty"`
	ArtifactDigest string   `json:"artifact_digest,omitempty"`
}

// MiddleWorkerResponseWire mirrors middleworker.ResponseEnvelope.
// The proposal is a NESTED object, not flat top-level fields; the
// provider decodes out.Proposal.* exactly where the real server
// puts them.
type MiddleWorkerResponseWire struct {
	RequestID     string       `json:"request_id"`
	DataVersion   string       `json:"data_version"`
	Proposal      ProposalWire `json:"proposal"`
	FinishReason  string       `json:"finish_reason,omitempty"`
	ModelRevision string       `json:"model_revision"`
}

// ProposalWire mirrors middleworker.Proposal.
type ProposalWire struct {
	SchemaVersion    string       `json:"schema_version"`
	RequestID        string       `json:"request_id"`
	DataVersion      string       `json:"data_version"`
	Status           string       `json:"status"`
	Intent           *string      `json:"intent"`
	Language         string       `json:"language"`
	Actions          []ActionWire `json:"actions"`
	SpeechKey        *string      `json:"speech_key"`
	ClarificationIDs []string     `json:"clarification_ids"`
	EvidenceIDs      []string     `json:"evidence_ids"`
}

// ActionWire mirrors the action variants the worker emits.
type ActionWire struct {
	Type      string   `json:"type"`
	TargetID  string   `json:"target_id,omitempty"`
	TargetIDs []string `json:"target_ids,omitempty"`
	RouteID   string   `json:"route_id,omitempty"`
	Panel     string   `json:"panel,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Steps     int      `json:"steps,omitempty"`
	Language  string   `json:"language,omitempty"`
}

// TTSWorkerResponseWire mirrors the ttsworker /synthesize response
// (SynthesizeResponse / contracts.TTSWorkerResponse).
type TTSWorkerResponseWire struct {
	RequestID      string `json:"request_id"`
	SpeechKey      string `json:"speech_key"`
	Language       string `json:"language"`
	State          string `json:"state"`
	AudioB64       string `json:"audio_b64,omitempty"`
	ContentType    string `json:"content_type,omitempty"`
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
	ModelRevision  string `json:"model_revision,omitempty"`
	VoiceRevision  string `json:"voice_revision,omitempty"`
	ByteSize       int64  `json:"byte_size,omitempty"`
	CacheHit       bool   `json:"cache_hit"`
}

// --- helpers ---------------------------------------------------------

// decodeAudioB64 decodes base64 audio into raw bytes. The harness
// stores audio outside the repo, so we always go through this.
func decodeAudioB64(b64 string) ([]byte, error) {
	if b64 == "" {
		return nil, errors.New("empty audio payload")
	}
	return stdBase64Decode(b64)
}

// httpStatusError converts a status code into a typed error class
// the runner can match against.
func httpStatusError(code int) error {
	switch {
	case code == http.StatusRequestTimeout:
		return errors.New("http: timeout")
	case code == http.StatusServiceUnavailable:
		return errors.New("http: 503 — model unavailable")
	case code == http.StatusTooManyRequests:
		return errors.New("http: 429 — queue saturated")
	case code/100 == 5:
		return fmt.Errorf("http: %d — server", code)
	default:
		return fmt.Errorf("http: %d — client", code)
	}
}

// boundedRead reads up to max bytes; truncates on overflow.
func boundedRead(r interface{ Read(p []byte) (int, error) }, max int64) ([]byte, error) {
	// Use stdlib via io.LimitReader to avoid pulling in extra deps.
	return ioLimitReaderRead(r, max)
}

// actionToString serializes an ActionWire to a stable identifier
// the runner's reconcile logic can match against.
func actionToString(a ActionWire) string {
	if a.TargetID != "" {
		return a.Type + ":" + a.TargetID
	}
	if len(a.TargetIDs) > 0 {
		return a.Type + ":[" + a.TargetIDs[0] + "]"
	}
	return a.Type
}
