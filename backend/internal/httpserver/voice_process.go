package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpjson"
	"sthira/backend/internal/orchestration"
)

// voice_process_handler.go — the typed /api/v3/voice/{transcriptions,
// process, speech} HTTP handlers. Owned by Worker 9 (p6-orchestration).
// The coordinator wires this handler into the server mux during the
// integration stage via the RegisterVoiceRoutes helper at the bottom
// of this file; server.go is not modified by this worker.
//
// Trust boundary: the handler is the public entry point. The typed
// request envelopes are decoded with strict JSON (no unknown fields,
// no duplicate keys, no trailing data). The orchestrator never
// accepts a client-supplied proposal as a substitute for running the
// production pipeline; the legacy /api/v3/voice/commands endpoint is
// unchanged and continues to validate client-supplied proposals only.
//
// Stage orders:
//   POST /api/v3/voice/transcriptions — bounded audio → typed transcript
//   POST /api/v3/voice/process         — full pipeline (audio or transcript → validated proposal → optional audio)
//   POST /api/v3/voice/speech          — approved template → audio
//
// All three handlers:
//   - enforce Content-Type (audio/* for transcriptions; application/json for the rest),
//   - enforce bounded bodies via MaxBytesReader,
//   - decode the typed request envelope strictly,
//   - assign a server-generated correlation ID if absent,
//   - translate orchestrator failures to stable contract error codes,
//   - emit low-cardinality metrics via the orchestrator's metrics sink.

const (
	voiceProcessRoute     = "/api/v3/voice/process"
	voiceTranscribeRoute  = "/api/v3/voice/transcriptions"
	voiceSpeechRoute      = "/api/v3/voice/speech"
	maxAudioRequestBytes  = 768 * 1024 // 512 KiB compressed audio + envelope overhead
	maxSpeechRequestBytes = 32 * 1024  // small JSON envelope
)

// VoiceProcessHandler is the orchestrator-side HTTP handler. It
// holds no per-request state and is safe to share across goroutines.
type VoiceProcessHandler struct {
	// orchestrator is the pipeline runner. The handler only routes
	// requests into it; the orchestrator owns the per-stage bounds.
	orchestrator *orchestration.Orchestrator
	// limits are the documented voice-handler ceiling for the
	// audio endpoint (the JSON endpoints use the orchestrator's
	// own limits).
	limits orchestration.Limits
}

// NewVoiceProcessHandler wires an orchestrator to the public HTTP
// boundary. The orchestrator may be nil when model configuration is incomplete;
// in that case, the routes stay unavailable (503 MODEL_UNAVAILABLE) while
// correctly enforcing HTTP methods (405 on non-POST).
func NewVoiceProcessHandler(orch *orchestration.Orchestrator, limits orchestration.Limits) *VoiceProcessHandler {
	return &VoiceProcessHandler{orchestrator: orch, limits: limits}
}

// RegisterVoiceRoutes mounts the three handlers onto a mux using the
// server's request-ID middleware. The coordinator calls this from
// server.go during the integration stage. Until then the routes
// return 404 — the existing TestServedRoutesMatchOpenAPI test
// documents this transitional state.
func (h *VoiceProcessHandler) RegisterVoiceRoutes(mux *http.ServeMux, withRequestID func(http.HandlerFunc) http.HandlerFunc) {
	if h == nil || mux == nil {
		return
	}
	mux.HandleFunc(voiceTranscribeRoute, withRequestID(h.handleTranscriptions))
	mux.HandleFunc(voiceProcessRoute, withRequestID(h.handleProcess))
	mux.HandleFunc(voiceSpeechRoute, withRequestID(h.handleSpeech))
}

// handleTranscriptions implements POST /api/v3/voice/transcriptions.
// The body is bounded compressed audio bytes (not JSON); the
// envelope is the X-Request-ID / X-Language headers. The response
// is the typed contracts.TranscriptionResponse embedded in the
// standard /api/v3 envelope.
func (h *VoiceProcessHandler) handleTranscriptions(w http.ResponseWriter, r *http.Request) {
	if !h.requireMethod(w, r, http.MethodPost) {
		return
	}
	if h.orchestrator == nil {
		h.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrModelUnavailable,
			"voice pipeline is unavailable: model worker is not configured", "", false)
		return
	}
	ct := r.Header.Get("Content-Type")
	if !contracts.IsSupportedTranscriptionContentType(ct) {
		h.writeError(w, r, http.StatusUnsupportedMediaType, contracts.ErrUnsupportedMedia,
			"Content-Type must be audio/wav, audio/webm, or audio/ogg", "", false)
		return
	}
	language := strings.TrimSpace(r.Header.Get("X-Language"))
	if language == "" {
		h.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue,
			"X-Language header is required", "language", false)
		return
	}
	limited := http.MaxBytesReader(w, r.Body, maxAudioRequestBytes)
	raw, err := io.ReadAll(limited)
	if err != nil {
		h.writeError(w, r, http.StatusRequestEntityTooLarge, contracts.ErrBodyTooLarge,
			"audio body exceeds size limit", "", false)
		return
	}
	if len(raw) == 0 {
		h.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue,
			"audio body is empty", "", false)
		return
	}
	if int64(len(raw)) > h.limits.MaxAudioCompressedBytes {
		h.writeError(w, r, http.StatusRequestEntityTooLarge, contracts.ErrBodyTooLarge,
			"audio body exceeds size limit", "", false)
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = "req-transcriptions"
	}
	// Build a transcript-only PipelineRequest and delegate to the
	// orchestrator's stageASR path. We do NOT call the full
	// pipeline because transcription is an ASR-only entry point.
	resp, err := h.transcribeOnly(r.Context(), contracts.TranscriptionRequest{
		RequestID:   requestID,
		Language:    language,
		ContentType: ct,
		ByteSize:    int64(len(raw)),
	}, raw)
	if err != nil {
		h.handlePipelineFailure(w, r, asPipelineError(err))
		return
	}
	h.writeData(w, r, http.StatusOK, resp.DataVersion, contracts.FreshnessUnknown, map[string]any{
		"request_id":   resp.RequestID,
		"data_version": resp.DataVersion,
		"language":     resp.Language,
		"text":         resp.Text,
		"confidence":   resp.Confidence,
		"alternatives": resp.Alternatives,
		"state":        string(resp.State),
	})
}

// transcribeOnly invokes the ASR stage only via the orchestrator.
// The full pipeline is not run; the transcription endpoint is the
// ASR-only entry point.
func (h *VoiceProcessHandler) transcribeOnly(ctx context.Context, req contracts.TranscriptionRequest, raw []byte) (contracts.TranscriptionResponse, error) {
	return h.orchestrator.Transcribe(ctx, req, raw)
}

// handleProcess implements POST /api/v3/voice/process. The body is
// the typed contracts.PipelineRequest envelope; the orchestrator
// runs the full pipeline.
func (h *VoiceProcessHandler) handleProcess(w http.ResponseWriter, r *http.Request) {
	if !h.requireMethod(w, r, http.MethodPost) {
		return
	}
	if h.orchestrator == nil {
		h.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrModelUnavailable,
			"voice pipeline is unavailable: model worker is not configured", "", false)
		return
	}
	if !h.requireJSONContentType(w, r) {
		return
	}
	limited := http.MaxBytesReader(w, r.Body, h.limits.MaxAudioCompressedBytes+int64(h.limits.MaxTranscriptUTF8Bytes)+1024)
	body, err := io.ReadAll(limited)
	if err != nil {
		h.writeError(w, r, http.StatusRequestEntityTooLarge, contracts.ErrBodyTooLarge,
			"request body exceeds size limit", "", false)
		return
	}
	var req contracts.PipelineRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{
		MaxBytes: int64(len(body)),
		MaxDepth: h.cfg().MaxJSONDepth,
	}); err != nil {
		h.jsonDecodeError(w, r, err)
		return
	}
	// Server-assigned correlation ID if absent.
	if req.RequestID == "" {
		req.RequestID = "req-process"
	}
	out, perr := h.orchestrator.Process(r.Context(), req, body)
	if perr != nil {
		h.handlePipelineFailure(w, r, asPipelineError(perr))
		return
	}
	resp := contracts.PipelineResponse{
		RequestID:         string(out.RequestID),
		DataVersion:       out.DataVersion,
		State:             out.State,
		ValidatedProposal: out.ValidatedProposal,
		Template:          out.Template,
		Audio:             out.Audio,
		StageFailures:     stageFailuresToStrings(out.Stages),
	}
	h.writeData(w, r, http.StatusOK, resp.DataVersion, contracts.FreshnessUnknown, resp)
}

// handleSpeech implements POST /api/v3/voice/speech. The body is the
// typed contracts.TTSRequest envelope; the handler validates the
// template key against the current scoped context and delegates
// synthesis to the TTS worker.
func (h *VoiceProcessHandler) handleSpeech(w http.ResponseWriter, r *http.Request) {
	if !h.requireMethod(w, r, http.MethodPost) {
		return
	}
	if h.orchestrator == nil {
		h.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrModelUnavailable,
			"voice pipeline is unavailable: model worker is not configured", "", false)
		return
	}
	if !h.requireJSONContentType(w, r) {
		return
	}
	limited := http.MaxBytesReader(w, r.Body, maxSpeechRequestBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		h.writeError(w, r, http.StatusRequestEntityTooLarge, contracts.ErrBodyTooLarge,
			"request body exceeds size limit", "", false)
		return
	}
	var req contracts.TTSRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{
		MaxBytes: int64(len(body)),
		MaxDepth: h.cfg().MaxJSONDepth,
	}); err != nil {
		h.jsonDecodeError(w, r, err)
		return
	}
	if req.RequestID == "" {
		req.RequestID = "req-speech"
	}
	out, perr := h.orchestrator.Synthesize(r.Context(), req, body)
	if perr != nil {
		h.handlePipelineFailure(w, r, asPipelineError(perr))
		return
	}
	h.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, out)
}

// handlePipelineFailure translates an orchestrator PipelineError
// into a stable contract error code + HTTP status. The error
// envelope is the standard /api/v3 envelope with one error.
func (h *VoiceProcessHandler) handlePipelineFailure(w http.ResponseWriter, r *http.Request, perr *orchestration.PipelineError) {
	if perr == nil {
		h.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal,
			"orchestrator returned a nil error", "", true)
		return
	}
	first := perr.Failures[0]
	msg := first.Reason
	if msg == "" {
		msg = string(first.Stage) + " failed"
	}
	h.writeError(w, r, perr.HTTPStatus, first.Code, msg, string(first.Stage), first.Retryable)
}

// stageFailuresToStrings flattens stage failures to the OpenAPI enum.
func stageFailuresToStrings(in []orchestration.StageFailure) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, f := range in {
		out = append(out, string(f.Stage))
	}
	return out
}

// requireMethod and requireJSONContentType duplicate the helpers on
// *Server so the handler can be tested in isolation. They mirror the
// server's helpers exactly so test assertions match production.
func (h *VoiceProcessHandler) requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		h.writeError(w, r, http.StatusMethodNotAllowed, contracts.ErrMethodNotAllowed,
			"method not allowed", "", false)
		return false
	}
	return true
}

func (h *VoiceProcessHandler) requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	mediaType := ct
	for i, c := range ct {
		if c == ';' {
			mediaType = ct[:i]
			break
		}
	}
	if mediaType != "application/json" {
		h.writeError(w, r, http.StatusUnsupportedMediaType, contracts.ErrUnsupportedMedia,
			"Content-Type must be application/json", "", false)
		return false
	}
	return true
}

// jsonDecodeError converts a strict-decode failure into a /api/v3 envelope.
func (h *VoiceProcessHandler) jsonDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var fe *httpjson.FieldError
	if errors.As(err, &fe) {
		status := http.StatusBadRequest
		if fe.Code == contracts.ErrBodyTooLarge {
			status = http.StatusRequestEntityTooLarge
		}
		h.writeError(w, r, status, fe.Code, fe.Message, fe.Field, false)
		return
	}
	h.writeError(w, r, http.StatusBadRequest, contracts.ErrMalformedJSON, "invalid request body", "", false)
}

// cfg returns the server config the handler uses for MaxJSONDepth.
// It is a function so the handler can be tested with a server
// (cfg via Server) or stand-alone (cfg via NewVoiceProcessHandler's
// limits parameter).
func (h *VoiceProcessHandler) cfg() Config {
	return Config{
		MaxJSONDepth: 32,
		MaxBodyBytes: int64(h.limits.MaxAudioCompressedBytes + int64(h.limits.MaxTranscriptUTF8Bytes) + 1024),
	}
}

// writeData writes a success envelope with exactly Data populated.
func (h *VoiceProcessHandler) writeData(w http.ResponseWriter, r *http.Request, status int, dataVersion string, source contracts.FreshnessState, data any) {
	env := contracts.Envelope{
		RequestID:     requestID(r),
		SchemaVersion: contracts.SchemaVersionV3,
		GeneratedAt:   nowUTC(),
		DataVersion:   dataVersion,
		SourceStatus:  source,
		Data:          data,
	}
	h.writeEnvelope(w, status, env)
}

// writeError writes an error envelope with exactly Errors populated.
func (h *VoiceProcessHandler) writeError(w http.ResponseWriter, r *http.Request, status int, code, message, field string, retryable bool) {
	env := contracts.Envelope{
		RequestID:     requestID(r),
		SchemaVersion: contracts.SchemaVersionV3,
		GeneratedAt:   nowUTC(),
		DataVersion:   "none",
		SourceStatus:  contracts.FreshnessUnknown,
		Errors: []contracts.APIError{{
			Code:          code,
			Message:       message,
			Field:         field,
			CorrelationID: requestID(r),
			Retryable:     retryable,
		}},
	}
	h.writeEnvelope(w, status, env)
}

func (h *VoiceProcessHandler) writeEnvelope(w http.ResponseWriter, status int, env contracts.Envelope) {
	body, err := httpjson.Marshal(env)
	if err != nil {
		// last-resort response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.Copy(w, bytes.NewReader([]byte(`{"errors":[{"code":"INTERNAL","message":"failed to encode response"}]}`)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// asPipelineError extracts a *orchestration.PipelineError from err.
// Returns a synthetic INTERNAL error when the err is not a
// PipelineError so the handler always has a typed failure to render.
func asPipelineError(err error) *orchestration.PipelineError {
	if err == nil {
		return nil
	}
	var pe *orchestration.PipelineError
	if errors.As(err, &pe) {
		return pe
	}
	return &orchestration.PipelineError{
		State:      orchestration.PipelineStateModelUnavailable(),
		HTTPStatus: http.StatusInternalServerError,
		Failures: []orchestration.StageFailure{{
			Stage:     orchestration.StageRender,
			Code:      contracts.ErrInternal,
			Reason:    err.Error(),
			Retryable: true,
		}},
	}
}

// Sentinel to silence unused-import warnings during incremental edits.
var _ = json.Marshal
