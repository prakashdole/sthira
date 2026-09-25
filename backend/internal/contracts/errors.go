package contracts

// Stable machine-readable error codes for the /api/v3 boundary. These are the
// contract; handlers and validators must use them so clients can rely on the
// token rather than the message text.
const (
	ErrMalformedJSON         = "MALFORMED_JSON"
	ErrDuplicateKey          = "DUPLICATE_KEY"
	ErrTrailingData          = "TRAILING_DATA"
	ErrBodyTooLarge          = "BODY_TOO_LARGE"
	ErrDepthExceeded         = "DEPTH_EXCEEDED"
	ErrUnknownField          = "UNKNOWN_FIELD"
	ErrInvalidValue          = "INVALID_VALUE"
	ErrValidation            = "VALIDATION_FAILED"
	ErrMethodNotAllowed      = "METHOD_NOT_ALLOWED"
	ErrUnsupportedMedia      = "UNSUPPORTED_MEDIA_TYPE"
	ErrAmbiguousPlace        = "AMBIGUOUS_PLACE"
	ErrDataUnavailable       = "DATA_UNAVAILABLE"
	ErrStaleVersion          = "STALE_VERSION"
	ErrRouteUnverified       = "ROUTE_UNVERIFIED"
	ErrCapacityUnknown       = "CAPACITY_UNKNOWN"
	ErrCapacityConflict      = "CAPACITY_CONFLICT"
	ErrIdempotencyConflict   = "IDEMPOTENCY_CONFLICT"
	ErrLanguageUnsupported   = "LANGUAGE_UNSUPPORTED"
	ErrModelUnavailable      = "MODEL_UNAVAILABLE"
	ErrUnauthorized          = "UNAUTHORIZED"
	ErrForbidden             = "FORBIDDEN"
	ErrDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
	ErrRateLimited           = "RATE_LIMITED"
	ErrNotFound              = "NOT_FOUND"
	ErrInternal              = "INTERNAL"
	ErrClockDrift            = "CLOCK_DRIFT"

	// P6 — voice pipeline and inference. Additive only; existing codes are
	// unchanged. See plan/p6-contract.md.
	ErrTranscriptUnavailable = "TRANSCRIPT_UNAVAILABLE"
	ErrAudioUnavailable      = "AUDIO_UNAVAILABLE"
	ErrModelTimeout          = "MODEL_TIMEOUT"
	ErrQueueSaturated        = "QUEUE_SATURATED"
	ErrInferenceCancelled    = "INFERENCE_CANCELLED"
	ErrTemplateUnknown       = "TEMPLATE_UNKNOWN"
	ErrStaleSnapshot         = "STALE_SNAPSHOT"
)
