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

// HTTPConfig configures the HTTP provider. Workers expose a typed
// envelope end-point; URLs are passed in by main, never hard-coded.
type HTTPConfig struct {
	ASRURL     string // POST TranscriptionRequest + audio body
	MiddleURL  string // POST PipelineRequest JSON
	TTSURL     string // POST TTSRequest JSON
	AuthBearer string // optional
	Timeout    time.Duration
}

// HTTPProvider speaks the frozen wire shapes over HTTP. It is the only
// real-run Provider. It does not bundle adapter logic; the worker
// processes own that.
type HTTPProvider struct {
	cfg    HTTPConfig
	client *http.Client
}

// NewHTTP returns a Provider whose Mode is ModeHTTP. The caller is
// responsible for keeping the URL strings in lockstep with the
// orchestrator's published protocol — changing this is a contract
// change, not a runner change.
func NewHTTP(cfg HTTPConfig) *HTTPProvider {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &HTTPProvider{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (p *HTTPProvider) Mode() Mode { return ModeHTTP }

// ASR POSTs the typed audio envelope and returns the typed
// transcription outcome. Audio bytes are sent as a raw body inside a
// multipart-free convention: the request line carries the
// request_id, and the JSON envelope fits in headers? — too clever.
// Use a small typed wire:
//
//	POST {base}/transcribe?request_id=...
//	Content-Type: <content_type>
//	Body: <bounded audio bytes>
//
// and rely on the response JSON. This is intentionally NOT
// re-implementing the orchestrator's full handler; it is a thin
// enforcement seam. The URL is supplied by main (the orchestrator's
// pipeline endpoint).
func (p *HTTPProvider) ASR(ctx context.Context, req ASRRequest) (ASROutcome, error) {
	if p.cfg.ASRURL == "" {
		return ASROutcome{}, errors.New("HTTP ASR URL not configured")
	}
	body, err := decodeAudioB64(req.AudioB64)
	if err != nil {
		return ASROutcome{}, fmt.Errorf("decode audio: %w", err)
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.ASRURL, bytes.NewReader(body))
	if err != nil {
		return ASROutcome{}, err
	}
	if req.ContentType != "" {
		hreq.Header.Set("Content-Type", req.ContentType)
	} else {
		hreq.Header.Set("Content-Type", "application/octet-stream")
	}
	if req.RequestID != "" {
		hreq.Header.Set("X-Request-ID", req.RequestID)
	}
	if req.Language != "" {
		hreq.Header.Set("X-Language", req.Language)
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
	var out ASROutcomeEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ASROutcome{}, fmt.Errorf("decode ASR json: %w", err)
	}
	return ASROutcome{
		State:      out.State,
		Text:       out.Text,
		Language:   out.Language,
		Confidence: out.Confidence,
		RevisionID: out.ModelRevision,
	}, nil
}

// Middle POSTs the typed pipeline envelope.
func (p *HTTPProvider) Middle(ctx context.Context, req MiddleRequest) (MiddleOutcome, error) {
	if p.cfg.MiddleURL == "" {
		return MiddleOutcome{}, errors.New("HTTP Middle URL not configured")
	}
	body, err := json.Marshal(PipelineEnvelopeBridge(req))
	if err != nil {
		return MiddleOutcome{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.MiddleURL, bytes.NewReader(body))
	if err != nil {
		return MiddleOutcome{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	if req.RequestID != "" {
		hreq.Header.Set("X-Request-ID", req.RequestID)
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
	var out PipelineResponseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return MiddleOutcome{}, fmt.Errorf("decode Middle json: %w", err)
	}
	return MiddleOutcome{
		Status:      out.Status,
		Intent:      out.Intent,
		Language:    out.Language,
		Actions:     out.Actions,
		SpeechKey:   out.SpeechKey,
		ClarifyIDs:  out.ClarifyIDs,
		EvidenceIDs: out.EvidenceIDs,
	}, nil
}

// TTS POSTs the typed TTS envelope.
func (p *HTTPProvider) TTS(ctx context.Context, req TTSRequest) (TTSOutcome, error) {
	if p.cfg.TTSURL == "" {
		return TTSOutcome{}, errors.New("HTTP TTS URL not configured")
	}
	body, err := json.Marshal(TTSEnvelopeBridge(req))
	if err != nil {
		return TTSOutcome{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TTSURL, bytes.NewReader(body))
	if err != nil {
		return TTSOutcome{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	if req.RequestID != "" {
		hreq.Header.Set("X-Request-ID", req.RequestID)
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
	var out TTSResponseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return TTSOutcome{}, fmt.Errorf("decode TTS json: %w", err)
	}
	return TTSOutcome{
		State:      out.State,
		SpeechKey:  out.SpeechKey,
		ByteSize:   out.ByteSize,
		DurationMS: out.DurationMS,
	}, nil
}

// --- wire shapes ---------------------------------------------------

// ASROutcomeEnvelope mirrors the orchestrator's downstream
// TranscriptionResponse. Field names reuse the contract vocabulary
// without redeclaring the type so eval remains independent of the
// orchestrator's package.
type ASROutcomeEnvelope struct {
	State         string   `json:"state"`
	Text          string   `json:"text"`
	Language      string   `json:"language"`
	Confidence    *float64 `json:"confidence,omitempty"`
	ModelRevision string   `json:"model_revision,omitempty"`
}

// PipelineEnvelopeBridge converts a MiddleRequest into a JSON value
// shaped like the orchestrator's typed pipeline envelope — input is a
// typed transcript, output is non-TTS render. The worker reads the
// same wire shape; we never diverge.
func PipelineEnvelopeBridge(req MiddleRequest) map[string]any {
	return map[string]any{
		"request_id": req.RequestID,
		"language":   req.Language,
		"input":      map[string]any{"kind": "transcript", "text": req.Transcript},
		"render":     map[string]any{"kind": "none"},
		"context":    req.Context,
		"case_id":    req.Case.ID,
	}
}

// PipelineResponseEnvelope mirrors the orchestrator's typed
// PipelineResponse. We never parse fields the runner doesn't need.
type PipelineResponseEnvelope struct {
	Status      string   `json:"status"`
	Intent      string   `json:"intent,omitempty"`
	Language    string   `json:"language"`
	Actions     []string `json:"actions,omitempty"`
	SpeechKey   string   `json:"speech_key,omitempty"`
	ClarifyIDs  []string `json:"clarification_ids,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

// TTSEnvelopeBridge converts a TTSRequest into the orchestrator's
// typed envelope.
func TTSEnvelopeBridge(req TTSRequest) map[string]any {
	return map[string]any{
		"request_id":     req.RequestID,
		"speech_key":     req.SpeechKey,
		"language":       req.Language,
		"args":           req.Args,
		"source_version": req.SourceVersion,
	}
}

// TTSResponseEnvelope mirrors the orchestrator's TTSResponse. We
// carry byte size + duration to keep the runner's stage accounting
// honest.
type TTSResponseEnvelope struct {
	State      string `json:"state"`
	SpeechKey  string `json:"speech_key,omitempty"`
	ByteSize   int    `json:"byte_size,omitempty"`
	DurationMS int    `json:"duration_ms,omitempty"`
}

// --- helpers -------------------------------------------------------

// decodeAudioB64 decodes base64 audio into raw bytes. The harness
// stores audio outside the repo, so we always go through this.
func decodeAudioB64(b64 string) ([]byte, error) {
	if b64 == "" {
		return nil, errors.New("empty audio payload")
	}
	// Use stdlib base64 via std (see encodeBase64 companion) — keep
	// imports lean.
	return stdBase64Decode(b64)
}

// httpStatusError converts a status code into a typed error class
// the runner can match against.
func httpStatusError(code int) error {
	if code == http.StatusRequestTimeout {
		return errors.New("http: timeout")
	}
	if code == http.StatusServiceUnavailable {
		return errors.New("http: 503 — model unavailable")
	}
	if code/100 == 5 {
		return fmt.Errorf("http: %d — server", code)
	}
	return fmt.Errorf("http: %d — client", code)
}
