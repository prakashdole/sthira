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

// Middle POSTs the typed PipelineRequest envelope (mirrors
// contracts.PipelineRequest) and decodes the typed
// PipelineResponse. Jurisdiction is sourced from the case's
// Context, NOT invented from data_version parsing.
func (p *HTTPProvider) Middle(ctx context.Context, req MiddleRequest) (MiddleOutcome, error) {
	if p.cfg.MiddleURL == "" {
		return MiddleOutcome{}, errors.New("HTTP Middle URL not configured")
	}
	jurisdiction := req.Case.Context.Jurisdiction
	if jurisdiction == "" {
		return MiddleOutcome{}, errors.New("middle: jurisdiction required (D36)")
	}
	srcVer := req.Case.Context.SourceVersion
	if srcVer <= 0 {
		return MiddleOutcome{}, errors.New("middle: source_version required (D43)")
	}
	input := map[string]any{
		"kind": "transcript",
		"text": req.Transcript,
	}
	render := map[string]any{"kind": "none"}
	wire := map[string]any{
		"request_id":      req.RequestID,
		"jurisdiction":    jurisdiction,
		"language":        req.Language,
		"input":           input,
		"render":          render,
		"idempotency_key": "eval-" + req.RequestID,
		// data_version is NOT in PipelineRequest; the orchestrator
		// resolves it from the scoped context. The harness
		// surfaces it as a request-side hint so the server can
		// fail-fast on stale context.
		"data_version_hint": req.Case.Context.DataVersion,
		"source_version":    srcVer,
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
	var out PipelineResponseWire
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return MiddleOutcome{}, fmt.Errorf("decode middle json: %w", err)
	}
	if out.RequestID != "" && out.RequestID != req.RequestID {
		return MiddleOutcome{}, fmt.Errorf("middle: request_id mismatch got=%q want=%q",
			out.RequestID, req.RequestID)
	}
	// The runner consumes status + intent + actions + speech_key.
	// The contract returns the validated_proposal; we extract
	// the typed fields from it.
	status := string(out.State)
	intent := ""
	var actions []string
	speechKey := ""
	var clarifyIDs, evidenceIDs []string
	if out.ValidatedProposal != nil {
		intent = stringPtr(out.ValidatedProposal.Intent)
		for _, a := range out.ValidatedProposal.Actions {
			actions = append(actions, actionToString(a))
		}
		speechKey = stringPtr(out.ValidatedProposal.SpeechKey)
		clarifyIDs = append(clarifyIDs, out.ValidatedProposal.ClarificationIDs...)
		evidenceIDs = append(evidenceIDs, out.ValidatedProposal.EvidenceIDs...)
	}
	// Template speech_key overrides proposal speech_key (matches
	// orchestrator logic).
	if out.Template.SpeechKey != "" {
		speechKey = out.Template.SpeechKey
	}
	return MiddleOutcome{
		Status:      status,
		Intent:      intent,
		Language:    req.Language,
		Actions:     actions,
		SpeechKey:   speechKey,
		ClarifyIDs:  clarifyIDs,
		EvidenceIDs: evidenceIDs,
	}, nil
}

// TTS POSTs the typed TTSWorkerRequest envelope (mirrors
// contracts.TTSWorkerRequest) and decodes the typed
// TTSWorkerResponse. The runner is forbidden from supplying free
// text; the speech text is sourced from a validated template via
// the server-side /api/v3/voice/speech endpoint. The harness
// supplies only the speech_key, jurisdiction and source_version
// so the worker renders the approved template text itself.
func (p *HTTPProvider) TTS(ctx context.Context, req TTSRequest) (TTSOutcome, error) {
	if p.cfg.TTSURL == "" {
		return TTSOutcome{}, errors.New("HTTP TTS URL not configured")
	}
	jurisdiction := req.Case.Context.Jurisdiction
	if jurisdiction == "" {
		return TTSOutcome{}, errors.New("tts: jurisdiction required")
	}
	srcVer := req.SourceVersion
	if srcVer <= 0 {
		srcVer = req.Case.Context.SourceVersion
	}
	if srcVer <= 0 {
		srcVer = 1
	}
	templateVersion := 1
	if req.Case.Context.TemplateVersion > 0 {
		templateVersion = req.Case.Context.TemplateVersion
	}
	// The harness does NOT send free text — the speech text is
	// server-rendered from the approved template. The wire
	// request sends the speech_key, language, jurisdiction,
	// source_version, and template_version only.
	wire := map[string]any{
		"request_id":       req.RequestID,
		"speech_key":       req.SpeechKey,
		"language":         req.Language,
		"text":             "", // server renders; client does not supply free text
		"source_version":   srcVer,
		"template_version": templateVersion,
		"settings": map[string]any{
			"sample_rate": 22050,
			"bit_depth":   16,
			"channels":    1,
		},
		"deadline_ms":     5000,
		"jurisdiction":    jurisdiction,
		"idempotency_key": "tts-eval-" + req.RequestID,
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
	return TTSOutcome{
		State:      string(out.State),
		SpeechKey:  out.SpeechKey,
		ByteSize:   int(out.ByteSize),
		DurationMS: 0, // PipelineAudio carries duration; TTSWorkerResponse doesn't
	}, nil
}

// --- wire shapes (mirror of contracts/*; eval module is stdlib-only) ---

// ASRWorkerResponseWire mirrors contracts.ASRWorkerResponse.
type ASRWorkerResponseWire struct {
	RequestID      string   `json:"request_id"`
	Language       string   `json:"language"`
	Text           string   `json:"text"`
	Confidence     *float64 `json:"confidence,omitempty"`
	State          string   `json:"state"`
	ModelRevision  string   `json:"model_revision,omitempty"`
	ArtifactDigest string   `json:"artifact_digest,omitempty"`
}

// PipelineResponseWire mirrors contracts.PipelineResponse.
type PipelineResponseWire struct {
	RequestID         string               `json:"request_id"`
	DataVersion       string               `json:"data_version"`
	State             string               `json:"state"`
	ValidatedProposal *ModelOutputWire     `json:"validated_proposal,omitempty"`
	Template          PipelineTemplateWire `json:"template"`
	Audio             *PipelineAudioWire   `json:"audio,omitempty"`
	StageFailures     []string             `json:"stage_failures,omitempty"`
}

// ModelOutputWire mirrors contracts.ModelOutput.
type ModelOutputWire struct {
	SchemaVersion    string       `json:"schema_version"`
	RequestID        string       `json:"request_id"`
	DataVersion      string       `json:"data_version"`
	Status           string       `json:"status"`
	Intent           *string      `json:"intent,omitempty"`
	Language         string       `json:"language"`
	Actions          []ActionWire `json:"actions"`
	SpeechKey        *string      `json:"speech_key,omitempty"`
	ClarificationIDs []string     `json:"clarification_ids"`
	EvidenceIDs      []string     `json:"evidence_ids"`
}

// ActionWire mirrors contracts.Action (loose; the runner only
// needs the type and target identifiers for accounting).
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

// PipelineTemplateWire mirrors contracts.PipelineTemplate.
type PipelineTemplateWire struct {
	SpeechKey       string        `json:"speech_key"`
	TemplateVersion int           `json:"template_version"`
	Args            []TemplateArg `json:"args,omitempty"`
}

// TemplateArg mirrors contracts.PipelineTemplateArg.
type TemplateArg struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PipelineAudioWire mirrors contracts.PipelineAudio.
type PipelineAudioWire struct {
	AudioID         string `json:"audio_id"`
	ContentType     string `json:"content_type"`
	ByteSize        int64  `json:"byte_size"`
	ChecksumSHA256  string `json:"checksum_sha256"`
	CacheHit        bool   `json:"cache_hit"`
	Language        string `json:"language"`
	ModelRevision   string `json:"model_revision"`
	VoiceRevision   string `json:"voice_revision"`
	TemplateVersion int    `json:"template_version"`
	SourceVersion   int    `json:"source_version"`
}

// TTSWorkerResponseWire mirrors contracts.TTSWorkerResponse.
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

func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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
