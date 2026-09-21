// Wire types for the middle worker protocol. These mirror the field
// names and shapes of contracts.MiddleWorkerRequest /
// contracts.MiddleWorkerResponse / contracts.WorkerHealth exactly. The
// worker module is isolated under its own go.mod and does not import
// the orchestrator's contracts package — the JSON wire format IS the
// contract.
//
// This file is the only place where field names are declared. The
// pinned schema fixture in schema/model_output.schema.json must match
// the embedded Proposal shape. A drift between this struct and the
// schema is a contract bug; tests assert it.

package middleworker

// ScopedContext is the JSON-decoded shape of contracts.ScopedContext
// the worker sees. Only the fields the model is allowed to consume
// are mirrored; everything else (request_id, source registry, audit
// fields) is orchestrator-owned and never reaches the worker.
type ScopedContext struct {
	// SchemaVersion is the literal the worker echoes back via the
	// proposal schema_version. The orchestrator validates it.
	SchemaVersion string `json:"schema_version"`
	// DataVersion is the source-snapshot identifier. The proposal
	// must echo it verbatim.
	DataVersion string `json:"data_version"`
	// AllowedLanguages is the explicit allow-list (per
	// plan/p6-contract.md). The worker rejects any language outside
	// this set.
	AllowedLanguages []string `json:"allowed_languages"`
	// TemplateKeys is the explicit allow-list for speech_key.
	TemplateKeys []string `json:"template_keys"`
	// PlaceCandidates are the typed place references available to
	// the model. The model can only reference IDs from this list.
	PlaceCandidates []PlaceCandidate `json:"place_candidates"`
	// FacilityCandidates are typed facility references. Facilities
	// cannot appear in SHOW_CHOICES; they may appear only in
	// OPEN_PANEL DESTINATION_PREVIEW.
	FacilityCandidates []FacilityCandidate `json:"facility_candidates"`
	// VerifiedRoutes are typed route references. A route must be
	// verified, not closed, and bound to a safe zone to be usable.
	VerifiedRoutes []RouteCandidate `json:"verified_routes"`
	// EligibleDestinations is the single source of permitted order
	// for SHOW_CHOICES.
	EligibleDestinations []EligibleDestination `json:"eligible_destinations"`
	// SourceVersion and TemplateVersion are explicit so the worker
	// never has to re-parse the package body.
	SourceVersion   int `json:"source_version"`
	TemplateVersion int `json:"template_version"`
}

// PlaceCandidate mirrors contracts.PlaceCandidate (subset the worker
// is allowed to see). Coordinates are bounded and never reach the
// assistant output.
type PlaceCandidate struct {
	ID           string `json:"id"`
	Jurisdiction string `json:"jurisdiction"`
	Version      int    `json:"version"`
	DisplayName  string `json:"display_name,omitempty"`
}

// FacilityCandidate mirrors contracts.FacilityCandidate.
type FacilityCandidate struct {
	ID              string `json:"id"`
	Jurisdiction    string `json:"jurisdiction"`
	Version         int    `json:"version"`
	PermittedOrder  int    `json:"permitted_order"`
	SafeZoneID      string `json:"safe_zone_id"`
	VerifiedRouteID string `json:"verified_route_id"`
	OrderCapacity   int    `json:"order_capacity"`
}

// RouteCandidate mirrors contracts.RouteRef.
type RouteCandidate struct {
	ID           string `json:"id"`
	Jurisdiction string `json:"jurisdiction"`
	Version      int    `json:"version"`
	ToSafeZoneID string `json:"to_safe_zone_id"`
	Verified     bool   `json:"verified"`
	Closed       bool   `json:"closed"`
	ValidNow     bool   `json:"valid_now"`
}

// EligibleDestination is a single ordered facility the model may
// include in SHOW_CHOICES.
type EligibleDestination struct {
	FacilityID string `json:"facility_id"`
	SafeZoneID string `json:"safe_zone_id"`
	RouteID    string `json:"route_id"`
	Order      int    `json:"order"`
}

// TranscriptInput is the ASR output (or direct transcript) the worker
// interprets. Mirrors contracts.ASRWorkerResponse.
type TranscriptInput struct {
	RequestID      string   `json:"request_id"`
	Language       string   `json:"language"`
	Text           string   `json:"text"`
	Confidence     *float64 `json:"confidence,omitempty"`
	State          string   `json:"state"`
	ModelRevision  string   `json:"model_revision,omitempty"`
	ArtifactDigest string   `json:"artifact_digest,omitempty"`
}

// RequestEnvelope is the body of POST /v1/chat/completions on the
// worker side. Mirrors contracts.MiddleWorkerRequest.
type RequestEnvelope struct {
	RequestID       string          `json:"request_id"`
	ScopedContext   ScopedContext   `json:"scoped_context"`
	Transcript      TranscriptInput `json:"transcript"`
	MaxOutputTokens int             `json:"max_output_tokens"`
	DeadlineMillis  int64           `json:"deadline_ms"`
}

// Proposal is the structured output the worker emits. Mirrors
// contracts.ModelOutput exactly so the orchestrator can decode and
// pass it to contracts.ValidateModelOutput without further
// translation. The JSON-constrained decode enforces the bounds; the
// orchestrator's independent validator enforces the semantics.
type Proposal struct {
	SchemaVersion    string   `json:"schema_version"`
	RequestID        string   `json:"request_id"`
	DataVersion      string   `json:"data_version"`
	Status           string   `json:"status"`
	Intent           *string  `json:"intent"`
	Language         string   `json:"language"`
	Actions          []Action `json:"actions"`
	SpeechKey        *string  `json:"speech_key"`
	ClarificationIDs []string `json:"clarification_ids"`
	EvidenceIDs      []string `json:"evidence_ids"`
}

// Action is one strict tagged action variant. Mirrors
// contracts.Action. Unknown variants fail the decode.
type Action struct {
	Type      string   `json:"type"`
	TargetID  string   `json:"target_id,omitempty"`
	TargetIDs []string `json:"target_ids,omitempty"`
	RouteID   string   `json:"route_id,omitempty"`
	Panel     string   `json:"panel,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Steps     int      `json:"steps,omitempty"`
	Language  string   `json:"language,omitempty"`
}

// ResponseEnvelope is the body the worker returns. Mirrors
// contracts.MiddleWorkerResponse.
type ResponseEnvelope struct {
	RequestID     string   `json:"request_id"`
	DataVersion   string   `json:"data_version"`
	Proposal      Proposal `json:"proposal"`
	FinishReason  string   `json:"finish_reason,omitempty"`
	ModelRevision string   `json:"model_revision"`
}

// QueueStats mirrors contracts.QueueStats.
type QueueStats struct {
	Depth          int `json:"depth"`
	MaxDepth       int `json:"max_depth"`
	MaxConcurrency int `json:"max_concurrency"`
}

// ModelInfo mirrors contracts.ModelInfo. The worker only ever holds
// ONE model at a time so this slice has length 1 in practice.
type ModelInfo struct {
	ModelID        string `json:"model_id"`
	Revision       string `json:"revision"`
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
	License        string `json:"license,omitempty"`
	Runtime        string `json:"runtime,omitempty"`
	Hardware       string `json:"hardware,omitempty"`
	RemoteCode     bool   `json:"remote_code"`
}

// ArtifactDigest mirrors contracts.ArtifactDigest.
type ArtifactDigest struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	License        string `json:"license,omitempty"`
}

// HealthEnvelope mirrors contracts.WorkerHealth.
type HealthEnvelope struct {
	Ready              bool             `json:"ready"`
	Warm               bool             `json:"warm"`
	Models             []ModelInfo      `json:"models"`
	Artifacts          []ArtifactDigest `json:"artifacts"`
	SupportedLanguages []string         `json:"supported_languages"`
	Queue              QueueStats       `json:"queue"`
	StartedAt          string           `json:"started_at,omitempty"`
	BuildRevision      string           `json:"build_revision,omitempty"`
}

// Middle-status values the worker reports. Distinct from the
// pipeline-level state in contracts.PipelineState; the orchestrator
// maps them.
const (
	MiddleStateOK                = "OK"
	MiddleStateTimeout           = "TIMEOUT"
	MiddleStateUnavailable       = "UNAVAILABLE"
	MiddleStateCanceled          = "CANCELED"
	MiddleStateMalformed         = "MALFORMED"
	MiddleStateSchemaUnsupported = "SCHEMA_UNSUPPORTED"
	MiddleStateContextExceeded   = "CONTEXT_EXCEEDED"
	MiddleStateOutputExceeded    = "OUTPUT_EXCEEDED"
)
