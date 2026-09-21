package contracts

// P6 voice pipeline (orchestration entry) contract. Worker 9 owns the
// handler that drives audio → ASR → scoped context → middle → independent
// validator → approved template → optional TTS. The typed envelopes below
// freeze the HTTP request/response shapes; Worker 9 owns the orchestration
// internals.

// PipelineState is the top-level outcome of a voice-pipeline call. It is
// distinct from ModelStatus because the pipeline can fail at stages the
// middleware never sees (audio decode, TTS unavailability, queue
// saturation).
type PipelineState string

const (
	PipelineOK               PipelineState = "OK"
	PipelineClarify          PipelineState = "CLARIFY"
	PipelineUnsupported      PipelineState = "UNSUPPORTED"
	PipelineDataUnavailable  PipelineState = "DATA_UNAVAILABLE"
	PipelineModelUnavailable PipelineState = "MODEL_UNAVAILABLE"
	PipelineCanceled         PipelineState = "CANCELED"
)

// PipelineInputKind discriminates the Input union. Exactly one of Audio or
// Transcript is populated.
type PipelineInputKind string

const (
	PipelineInputAudio      PipelineInputKind = "audio"
	PipelineInputTranscript PipelineInputKind = "transcript"
)

// PipelineInput carries the user's request, either as bounded audio or as a
// typed transcript. The orchestrator accepts exactly one.
type PipelineInput struct {
	Kind PipelineInputKind `json:"kind"`
	// Audio (set when Kind == "audio"):
	ContentType string `json:"content_type,omitempty"`
	BodyB64     string `json:"body_b64,omitempty"` // base64 of bounded compressed bytes
	// Transcript (set when Kind == "transcript"):
	Text       string   `json:"text,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// PipelineRenderKind discriminates the Render union.
type PipelineRenderKind string

const (
	PipelineRenderNone PipelineRenderKind = "none"
	PipelineRenderTTS  PipelineRenderKind = "tts"
)

// PipelineRender controls whether the orchestrator returns only the
// validated proposal or also synthesizes the approved template.
type PipelineRender struct {
	Kind PipelineRenderKind `json:"kind"`
}

// PipelineRequest is the typed shape the public handler decodes from the
// request body. The handler enforces the same strict JSON decode used
// elsewhere on the /api/v3 boundary.
type PipelineRequest struct {
	RequestID      string         `json:"request_id"`
	Jurisdiction   string         `json:"jurisdiction"`
	Language       string         `json:"language"`
	Input          PipelineInput  `json:"input"`
	Render         PipelineRender `json:"render"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}

// PipelineTemplateArg is one resolved argument used to render the approved
// template. Args are validated IDs/strings, never arbitrary prose.
type PipelineTemplateArg struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PipelineTemplate carries the rendered template reference (NOT the rendered
// text — TTS only sees what it needs).
type PipelineTemplate struct {
	SpeechKey       string                `json:"speech_key"`
	TemplateVersion int                   `json:"template_version"`
	Args            []PipelineTemplateArg `json:"args,omitempty"`
}

// PipelineAudio describes the synthesized audio (when Render.TTS). It is
// embedded in the standard /api/v3 envelope by the caller; the audio bytes
// are returned alongside the JSON envelope with the same checksum_sha256 so
// the client can verify before display.
type PipelineAudio struct {
	AudioID         string               `json:"audio_id"`
	ContentType     string               `json:"content_type"`
	ByteSize        int64                `json:"byte_size"`
	ChecksumSHA256  string               `json:"checksum_sha256"`
	CacheHit        bool                 `json:"cache_hit"`
	Language        string               `json:"language"`
	ModelRevision   string               `json:"model_revision"`
	VoiceRevision   string               `json:"voice_revision"`
	TemplateVersion int                  `json:"template_version"`
	SourceVersion   int                  `json:"source_version"`
	Settings        TTSSynthesisSettings `json:"settings"`
}

// PipelineResponse is the typed envelope returned by the orchestrator.
type PipelineResponse struct {
	RequestID         string           `json:"request_id"`
	DataVersion       string           `json:"data_version"`
	State             PipelineState    `json:"state"`
	ValidatedProposal ModelOutput      `json:"validated_proposal"`
	Template          PipelineTemplate `json:"template"`
	Audio             *PipelineAudio   `json:"audio,omitempty"`
	// StageFailures records which stage first failed (when State is not OK).
	// Values: "audio", "asr", "context", "middle", "validator", "template",
	// "tts". Empty on success.
	StageFailures []string `json:"stage_failures,omitempty"`
}

// PipelineLimits are the documented ceilings for the public voice-process
// entry point. Larger inputs are rejected at the handler (400 INVALID_VALUE).
// Per-stage deadlines are enforced by the orchestrator; see
// plan/parameters.md (8s total cap) and Worker 9's accepted defaults.
const (
	PipelineMaxAudioCompressedBytes int64   = 512 * 1024 // 512 KiB
	PipelineMaxAudioDecodedSeconds  float64 = 20.0
	PipelineMaxTranscriptUTF8Bytes  int     = 4096
)
