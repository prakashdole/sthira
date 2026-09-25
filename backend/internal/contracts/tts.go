package contracts

// P6 TTS entry-point contract. The handler receives a request naming an
// approved template, language, validated args and a source_version, and
// returns a content-addressed audio envelope. TTS never receives arbitrary
// public text; only the rendered template text, the model/voice revision,
// and the source_version flow to the worker.

// TTSState is the top-level outcome of a TTS call.
type TTSState string

const (
	TTSOK                  TTSState = "OK"
	TTSUnsupportedLanguage TTSState = "UNSUPPORTED_LANGUAGE"
	TTSAudioUnavailable    TTSState = "AUDIO_UNAVAILABLE"
	TTSTimeout             TTSState = "TIMEOUT"
	TTSUnavailable         TTSState = "UNAVAILABLE"
	TTSCanceled            TTSState = "CANCELED"
)

// SpeechArgs is the validated arguments object for an approved template.
// Worker 4 owns the typed arg schema per template; Worker 7 only receives a
// structurally-valid object. The orchestrator validates args against the
// template before calling the worker.
type SpeechArgs struct {
	// Args is a JSON object: keys are template-arg names, values are
	// validated strings/numbers/IDs. Nested objects are permitted only when
	// the template declares them. Arbitrary text fields are rejected at the
	// orchestrator, never reaching the worker.
	Args map[string]any `json:"args"`
}

// TTSSynthesisSettings is a deterministic snapshot of the synthesis
// parameters used to produce the audio. The TTS cache key includes this
// snapshot so a parameter change invalidates the cache.
type TTSSynthesisSettings struct {
	// SampleRate is the output PCM sample rate in Hz.
	SampleRate int `json:"sample_rate"`
	// BitDepth is the output PCM bit depth (16 today).
	BitDepth int `json:"bit_depth"`
	// Channels is 1 (mono).
	Channels int `json:"channels"`
	// SpeakingRate is the model's speaking-rate parameter, or 0 for default.
	SpeakingRate float64 `json:"speaking_rate,omitempty"`
}

// TTSRequest is the typed shape the public handler decodes from the request
// body.
type TTSRequest struct {
	RequestID     string               `json:"request_id"`
	Jurisdiction  string               `json:"jurisdiction"`
	SpeechKey     string               `json:"speech_key"` // must be in ScopedContext.TemplateKeys
	Language      string               `json:"language"`   // must be in ScopedContext.AllowedLanguages
	Args          SpeechArgs           `json:"args"`
	SourceVersion int                  `json:"source_version"` // must equal current source version
	Settings      TTSSynthesisSettings `json:"settings"`
}

// TTSResponse is the typed envelope returned by the handler. It carries the
// content-addressed audio descriptor; the audio bytes themselves are
// returned in the HTTP body (audio/wav) with the same checksum_sha256 in the
// envelope so the orchestrator can verify before forwarding.
type TTSResponse struct {
	RequestID       string               `json:"request_id"`
	SpeechKey       string               `json:"speech_key"`
	Language        string               `json:"language"`
	SourceVersion   int                  `json:"source_version"`
	TemplateVersion int                  `json:"template_version"`
	ModelRevision   string               `json:"model_revision"`
	VoiceRevision   string               `json:"voice_revision"`
	AudioID         string               `json:"audio_id"`     // content-addressed; SHA-256 of audio bytes
	ContentType     string               `json:"content_type"` // audio/wav
	ByteSize        int64                `json:"byte_size"`
	ChecksumSHA256  string               `json:"checksum_sha256"`
	CacheHit        bool                 `json:"cache_hit"`
	State           TTSState             `json:"state"`
	Settings        TTSSynthesisSettings `json:"settings"`
	AudioB64        string               `json:"audio_b64,omitempty"`
}

// TTSCacheKey is the canonical cache key used by the TTS worker and the
// orchestrator. It is built from strictly-typed components so a change in
// any one of them (e.g., a model/voice revision bump, a synthesis-setting
// change) invalidates the cache deterministically.
//
// The cache key is sha256 hex of the JSON encoding of this struct.
type TTSCacheKey struct {
	SpeechKey       string     `json:"speech_key"`
	Language        string     `json:"language"`
	Args            SpeechArgs `json:"args"`
	SourceVersion   int        `json:"source_version"`
	TemplateVersion int        `json:"template_version"`
	// TemplateSHA256 binds the cache entry to the approved template
	// digest so a text change invalidates prior audio.
	TemplateSHA256 string               `json:"template_sha256"`
	ModelRevision  string               `json:"model_revision"`
	VoiceRevision  string               `json:"voice_revision"`
	Settings       TTSSynthesisSettings `json:"settings"`
}

// ApprovedTemplate is the typed description of an approved speech template.
// Worker 4 owns the registry exposed through ScopedContext.TemplateKeys;
// Worker 7 reads the registry but never edits it. The rendered text for a
// (key, language) pair is supplied by the coordinator through the
// TemplateRegistry seam below.
type ApprovedTemplate struct {
	SpeechKey       string `json:"speech_key"`
	Language        string `json:"language"`
	TemplateVersion int    `json:"template_version"`
	SourceVersion   int    `json:"source_version"`
	// Text is the rendered, approved text. Worker 7 receives this text and
	// the model/voice revision; arbitrary user text never reaches the
	// worker.
	Text string `json:"text"`
	// ArgSchema is the JSON Schema for the args object. Worker 4 enforces it
	// at the orchestrator; Worker 7 trusts the orchestrator's validation.
	// Nil when the template has no args.
	ArgSchema any `json:"arg_schema,omitempty"`
	// SyntheticOnly is true when the template is a TEST/DEMO template and
	// must be rejected outside isolated test configuration.
	SyntheticOnly bool `json:"synthetic_only,omitempty"`
	// TemplateSHA256 is the SHA-256 hex of Text (canonical template bytes).
	// Computed on registry Add when empty; compared to the DB-approved digest.
	TemplateSHA256 string `json:"template_sha256,omitempty"`
}

// TemplateRegistry is the read-only seam Worker 7 reads from and Worker 4
// produces. The orchestrator calls Lookup(speech_key, language) before
// passing the rendered text to TTS.
type TemplateRegistry interface {
	Lookup(speechKey, language string) (ApprovedTemplate, bool)
	// Keys returns the approved speech_key set for the current snapshot.
	// Order is not semantic.
	Keys() []string
}
