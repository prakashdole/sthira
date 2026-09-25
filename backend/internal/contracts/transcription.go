package contracts

import (
	"mime"
	"strings"
)

// P6 transcription entry-point contract (ASR boundary). The HTTP handler
// receives a bounded compressed audio upload and returns a typed transcript.
// The handler does NOT store the transcript server-side; persistence is the
// caller's responsibility (orchestration). It does NOT log audio bytes.
//
// Confidence is nullable: nil/unknown means "not calibrated". A non-nil value
// is a calibrated probability in [0,1]; an explicit zero is a real signal of
// zero, not "unknown".

// TranscriptionState is the top-level outcome of an ASR call. Distinct from
// the middleware ModelStatus: ASR cannot return CLARIFY (clarification is a
// downstream concern).
type TranscriptionState string

const (
	TranscriptionOK                  TranscriptionState = "OK"
	TranscriptionUnsupportedLanguage TranscriptionState = "UNSUPPORTED_LANGUAGE"
	TranscriptionAudioUnavailable    TranscriptionState = "AUDIO_UNAVAILABLE"
	TranscriptionTimeout             TranscriptionState = "TIMEOUT"
	TranscriptionUnavailable         TranscriptionState = "UNAVAILABLE"
	TranscriptionCancelled           TranscriptionState = "CANCELED"
)

// TranscriptionRequest is the typed shape the public handler decodes the
// request headers into. The audio body itself is decoded separately against
// the document codec limits.
type TranscriptionRequest struct {
	RequestID   string `json:"request_id"`
	Language    string `json:"language"` // must be in ScopedContext.AllowedLanguages
	ContentType string `json:"content_type"`
	// ByteSize is the bounded body size in bytes (compressed). The handler
	// sets this after the bounded reader accepts the body.
	ByteSize int64 `json:"byte_size"`
	// DecodedDurationSeconds is set by the handler after codec decode
	// (success or failure). 0 means unknown/decode failed.
	DecodedDurationSeconds float64 `json:"decoded_duration_seconds,omitempty"`
}

// TranscriptionAlternative is one n-best alternative. ASR artifacts that do
// not support n-best emit an empty slice; downstream code must not assume
// alternatives is populated.
type TranscriptionAlternative struct {
	Text       string   `json:"text"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// TranscriptionResponse is the typed envelope returned by the handler. It is
// embedded in the standard /api/v3 envelope by the caller.
type TranscriptionResponse struct {
	RequestID    string                     `json:"request_id"`
	DataVersion  string                     `json:"data_version"` // current source snapshot
	Language     string                     `json:"language"`
	Text         string                     `json:"text"`
	Confidence   *float64                   `json:"confidence"` // nil = unknown / not calibrated
	Alternatives []TranscriptionAlternative `json:"alternatives,omitempty"`
	State        TranscriptionState         `json:"state"`
	// ModelRevision is the artifact revision that produced the transcript, so
	// orchestrators can detect uncalibrated revisions against the pinned list.
	ModelRevision string `json:"model_revision,omitempty"`
	// ArtifactDigest is the SHA-256 of the artifact bytes (when the worker
	// exposes it). Empty when the worker only reports a revision string.
	ArtifactDigest string `json:"artifact_digest,omitempty"`
}

// Bounded compressed audio limits for the public transcription entry point.
// Larger inputs are rejected at the handler (400 INVALID_VALUE) before any
// worker call. These are starting budgets per plan/parameters.md; they are
// NOT measurements.
const (
	MaxTranscriptionCompressedBytes int64   = 512 * 1024 // 512 KiB
	MaxTranscriptionDecodedSeconds  float64 = 20.0
	MinTranscriptionSampleRate      int     = 8000
	MaxTranscriptionSampleRate      int     = 48000
)

// SupportedTranscriptionContentTypes is the allow-list for the public entry
// point. Other types are rejected at the handler (415).
var SupportedTranscriptionContentTypes = []string{
	"audio/wav",
	"audio/webm",
	"audio/ogg",
	"audio/opus",
}

// IsSupportedTranscriptionContentType reports whether the MIME type is on
// the allow list. Uses stdlib mime.ParseMediaType to canonicalize parameters
// (such as codecs=opus) consistently.
func IsSupportedTranscriptionContentType(rawCT string) bool {
	if strings.TrimSpace(rawCT) == "" {
		return false
	}
	mediaType, params, err := mime.ParseMediaType(rawCT)
	if err != nil {
		return false
	}
	switch mediaType {
	case "audio/wav":
		if codec, ok := params["codecs"]; ok && codec != "" && codec != "1" && codec != "pcm" {
			return false
		}
		return true
	case "audio/webm", "audio/ogg":
		if codec, ok := params["codecs"]; ok && codec != "" && codec != "opus" {
			return false
		}
		return true
	case "audio/opus":
		return true
	default:
		return false
	}
}
