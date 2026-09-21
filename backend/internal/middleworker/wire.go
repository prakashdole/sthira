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
// are mirrored; everything else (audit fields, sensitive tokens)
// is orchestrator-owned and never reaches the worker.
type ScopedContext struct {
	RequestID       string `json:"request_id,omitempty"`
	DataVersion     string `json:"data_version"`
	Jurisdiction    string `json:"jurisdiction,omitempty"`
	SchemaVersion   string `json:"schema_version"`
	SourceStatus    string `json:"source_status,omitempty"`
	SourceVersion   int    `json:"source_version,omitempty"`
	TemplateVersion int    `json:"template_version,omitempty"`

	AllowedLanguages []string `json:"allowed_languages,omitempty"`
	TemplateKeys     []string `json:"template_keys,omitempty"`

	KnownPlaces     map[string]PlaceCandidate `json:"known_places,omitempty"`
	KnownRedZones   map[string]ZoneRef        `json:"known_red_zones,omitempty"`
	KnownSafeZones  map[string]ZoneRef        `json:"known_safe_zones,omitempty"`
	KnownRoutes     map[string]RouteRef       `json:"known_routes,omitempty"`
	KnownFacilities map[string]FacilityRef    `json:"known_facilities,omitempty"`

	VerifiedRoutes       map[string][]RouteRef `json:"verified_routes,omitempty"`
	EligibleDestinations []EligibleChoice      `json:"eligible_destinations,omitempty"`
	IssuedAt             string                `json:"issued_at,omitempty"`
}

// PlaceCandidate mirrors contracts.PlaceCandidate.
type PlaceCandidate struct {
	PlaceID      string `json:"place_id"`
	PlaceKind    string `json:"place_kind"`
	Name         string `json:"name,omitempty"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
}

// ZoneRef mirrors contracts.ZoneRef.
type ZoneRef struct {
	ZoneID        string `json:"zone_id"`
	ZoneRole      string `json:"zone_role"`
	Status        string `json:"status"`
	SourceVersion int    `json:"source_version"`
}

// FacilityRef mirrors contracts.FacilityRef.
type FacilityRef struct {
	FacilityID    string `json:"facility_id"`
	SafeZoneID    string `json:"safe_zone_id"`
	Name          string `json:"name,omitempty"`
	CapacityKnown bool   `json:"capacity_known"`
	Free          int    `json:"free,omitempty"`
	SourceVersion int    `json:"source_version"`
}

// RouteRef mirrors contracts.RouteRef.
type RouteRef struct {
	RouteID       string `json:"route_id"`
	FromZoneID    string `json:"from_zone_id,omitempty"`
	ToSafeZoneID  string `json:"to_safe_zone_id"`
	Status        string `json:"status"`
	Verified      bool   `json:"verified"`
	SourceVersion int    `json:"source_version"`
}

// EligibleChoice mirrors contracts.EligibleChoice.
type EligibleChoice struct {
	Facility      FacilityRef `json:"facility"`
	PermittedRank int         `json:"permitted_rank"`
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
