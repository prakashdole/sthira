// Package contracts defines the executable /api/v3 boundary types.
//
// All text carried by these types is inert data. Consumers must escape it for
// their output context and must never evaluate imported source text. The
// boundary preserves source provenance, evidence class and version on every
// operational fact; unknown is always distinct from empty, zero or current.
package contracts

// FreshnessState describes how current a fact or its source is. UNKNOWN is a
// first-class value: it never collapses into CURRENT or EMPTY.
type FreshnessState string

const (
	FreshnessCurrent     FreshnessState = "CURRENT"
	FreshnessStale       FreshnessState = "STALE"
	FreshnessExpired     FreshnessState = "EXPIRED"
	FreshnessUnavailable FreshnessState = "UNAVAILABLE"
	FreshnessUnknown     FreshnessState = "UNKNOWN"
)

// EvidenceClass marks whether a fact is authorized operational data or visible
// non-operational material. Demo geometry is never presented as operational.
type EvidenceClass string

const (
	EvidenceSyntheticDemo EvidenceClass = "SYNTHETIC_DEMO"
	EvidenceOfficial      EvidenceClass = "OFFICIAL"
)

// ValidationState is the validation outcome attached to an operational fact.
type ValidationState string

const (
	ValidationPending     ValidationState = "PENDING"
	ValidationValid       ValidationState = "VALID"
	ValidationInvalid     ValidationState = "INVALID"
	ValidationQuarantined ValidationState = "QUARANTINED"
	ValidationConflicting ValidationState = "CONFLICTING"
)

// SourceProvenance accompanies every operational fact. Missing or unapproved
// provenance cannot authorize guidance.
type SourceProvenance struct {
	SourceID       string        `json:"source_id"`
	AuthorityName  string        `json:"authority_name"`
	SourceURI      string        `json:"source_uri"`
	ArtifactID     string        `json:"artifact_id"`
	ArtifactSHA256 string        `json:"artifact_sha256"`
	RetrievedAt    string        `json:"retrieved_at"` // UTC RFC 3339
	IssuedAt       string        `json:"issued_at"`    // UTC RFC 3339
	Version        int           `json:"version"`
	EvidenceClass  EvidenceClass `json:"evidence_class"`
}

// FactState pairs the validation outcome with freshness for a fact.
type FactState struct {
	Validation      ValidationState `json:"validation"`
	Freshness       FreshnessState  `json:"freshness"`
	LastRefreshedAt string          `json:"last_refreshed_at"` // UTC RFC 3339
}

// APIError is the stable error shape inside the envelope. Code is a stable
// machine-readable token; Message is human-readable and already escaped.
type APIError struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	Field         string `json:"field,omitempty"`
	CorrelationID string `json:"correlation_id"`
	Retryable     bool   `json:"retryable"`
	Details       any    `json:"details,omitempty"`
}

// Envelope is the single response wrapper for the /api/v3 boundary. Exactly
// one of Data or Errors is populated; the boundary never returns both or
// neither. Data is typed per endpoint by the handler.
type Envelope struct {
	RequestID     string         `json:"request_id"`
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"` // UTC RFC 3339
	DataVersion   string         `json:"data_version"`
	SourceStatus  FreshnessState `json:"source_status"`
	Data          any            `json:"data,omitempty"`
	Errors        []APIError     `json:"errors,omitempty"`
}

// SchemaVersionV3 is the frozen /api/v3 schema version emitted in envelopes.
const SchemaVersionV3 = "3.0"
