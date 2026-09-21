package contracts

// Private worker protocol (Go orchestrator ↔ Python/native inference worker).
// The shapes below are the typed envelopes each private worker must speak.
// The worker code lives in isolated go.mod files (per plan/p6-contract.md
// ownership); the orchestrator and the worker consume these types via
// JSON-over-HTTP and agree on the field names here.

// ModelInfo is one descriptor of a model the worker currently holds in
// memory. Workers report the loaded models in their /health response so the
// orchestrator can fail fast when the artifact the snapshot expects is not
// the one the worker actually loaded.
type ModelInfo struct {
	ModelID        string `json:"model_id"` // e.g. "indic-conformer-600m-multilingual"
	Revision       string `json:"revision"`
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
	License        string `json:"license,omitempty"`  // SPDX identifier or "LicensePending"
	Runtime        string `json:"runtime,omitempty"`  // e.g. "onnxruntime-1.18"
	Hardware       string `json:"hardware,omitempty"` // e.g. "cpu", "cuda:a100"
	RemoteCode     bool   `json:"remote_code"`        // trust-this must be false in production
}

// ArtifactDigest is the SHA-256 of an auxiliary artifact (tokenizer, voice
// embedding, prompt template, etc.). The orchestrator may pin specific
// digests and refuse mismatches.
type ArtifactDigest struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	License        string `json:"license,omitempty"`
}

// QueueStats reports the worker's current admission state. The orchestrator
// reads this from /health and refuses to dispatch new work when the queue is
// saturated or the worker reports Warm=false.
type QueueStats struct {
	Depth          int `json:"depth"`           // pending requests
	MaxDepth       int `json:"max_depth"`       // bound; saturate response when depth >= max
	MaxConcurrency int `json:"max_concurrency"` // bound
}

// WorkerHealth is the typed envelope returned by every worker's GET /health.
// Worker 5/6/7 implement the endpoint; Worker 9 reads it on startup and
// before each dispatch.
type WorkerHealth struct {
	Ready              bool             `json:"ready"`
	Warm               bool             `json:"warm"` // true once the model has actually loaded
	Models             []ModelInfo      `json:"models"`
	Artifacts          []ArtifactDigest `json:"artifacts"`
	SupportedLanguages []string         `json:"supported_languages"`
	Queue              QueueStats       `json:"queue"`
	// StartedAt is the worker's process start time (UTC RFC 3339).
	StartedAt string `json:"started_at,omitempty"`
	// BuildRevision is the worker's own build/revision identifier, recorded
	// for incident triage only.
	BuildRevision string `json:"build_revision,omitempty"`
}

// ASRWorkerRequest is the typed shape the orchestrator sends to the ASR
// worker's POST /transcribe. The worker must reject languages not in
// SupportedLanguages and codecs not in the worker's allow list.
type ASRWorkerRequest struct {
	RequestID      string  `json:"request_id"`
	Language       string  `json:"language"`
	ContentType    string  `json:"content_type"`
	AudioB64       string  `json:"audio_b64"` // base64 of bounded compressed bytes
	ByteSize       int64   `json:"byte_size"`
	DecodedSeconds float64 `json:"decoded_seconds"`
	// DeadlineMillis is the per-request deadline the orchestrator sets; the
	// worker must honor it.
	DeadlineMillis int64 `json:"deadline_ms"`
}

// ASRWorkerResponse is the typed envelope the ASR worker returns.
type ASRWorkerResponse struct {
	RequestID      string                     `json:"request_id"`
	Language       string                     `json:"language"`
	Text           string                     `json:"text"`
	Confidence     *float64                   `json:"confidence"`
	Alternatives   []TranscriptionAlternative `json:"alternatives,omitempty"`
	State          TranscriptionState         `json:"state"`
	ModelRevision  string                     `json:"model_revision"`
	ArtifactDigest string                     `json:"artifact_digest"`
}

// MiddleWorkerRequest is the typed shape the orchestrator sends to the
// middle-model worker's POST /v1/chat/completions. The worker enforces its
// own structured-output schema and never receives arbitrary tool/credential
// access.
type MiddleWorkerRequest struct {
	RequestID string `json:"request_id"`
	// ScopedContext is the JSON-encoded contracts.ScopedContext. The worker
	// treats every field as data, never as instructions.
	ScopedContext ScopedContext `json:"scoped_context"`
	// Transcript is the ASR output (or direct transcript) the model interprets.
	Transcript ASRWorkerResponse `json:"transcript"`
	// Stop tokens are worker-owned; the orchestrator forwards only the
	// transcript and the context.
	MaxOutputTokens int   `json:"max_output_tokens"`
	DeadlineMillis  int64 `json:"deadline_ms"`
}

// MiddleWorkerResponse is the typed envelope the middle worker returns. The
// orchestrator independently validates the embedded ModelOutput before any
// downstream stage consumes it; valid JSON does not bypass validation.
type MiddleWorkerResponse struct {
	RequestID     string      `json:"request_id"`
	DataVersion   string      `json:"data_version"`
	Proposal      ModelOutput `json:"proposal"`
	FinishReason  string      `json:"finish_reason,omitempty"`
	ModelRevision string      `json:"model_revision"`
}

// TTSWorkerRequest is the typed shape the orchestrator sends to the TTS
// worker's POST /synthesize. Only Go-approved text and versions flow.
type TTSWorkerRequest struct {
	RequestID       string               `json:"request_id"`
	SpeechKey       string               `json:"speech_key"`
	Language        string               `json:"language"`
	Text            string               `json:"text"` // rendered template text only
	SourceVersion   int                  `json:"source_version"`
	TemplateVersion int                  `json:"template_version"`
	Settings        TTSSynthesisSettings `json:"settings"`
	DeadlineMillis  int64                `json:"deadline_ms"`
}

// TTSWorkerResponse is the typed envelope the TTS worker returns.
type TTSWorkerResponse struct {
	RequestID      string   `json:"request_id"`
	SpeechKey      string   `json:"speech_key"`
	Language       string   `json:"language"`
	State          TTSState `json:"state"`
	AudioB64       string   `json:"audio_b64"`    // base64 of PCM/WAV bytes
	ContentType    string   `json:"content_type"` // audio/wav
	ChecksumSHA256 string   `json:"checksum_sha256"`
	ModelRevision  string   `json:"model_revision"`
	VoiceRevision  string   `json:"voice_revision"`
}

// WorkerPrivateHeaders are the HTTP header names the orchestrator uses for
// correlation and per-request deadlines. Both sides agree on these names.
const (
	WorkerHeaderRequestID = "X-Sthira-Request-ID"
	WorkerHeaderDeadline  = "X-Sthira-Deadline-Ms"
	WorkerHeaderStage     = "X-Sthira-Stage" // "asr" | "middle" | "tts"
)
