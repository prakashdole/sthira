// Package corpus defines the P6 evaluation case schema and JSON loaders.
//
// A Case is the smallest unit of evaluation. Every case carries:
//
//   - Provenance: source/license/consent. Only SYNTHETIC Provenance is
//     permitted in Git-tracked fixtures; CONSENTED cases are loaded via
//     a side-manifest with actual bytes paths outside the repo.
//   - LangCohort: language/dialect/cohort. Cohort in {ADULT_SYNTHETIC,
//     PII_FORBIDDEN}: child/elder speech in Git is forbidden by
//     Provenance.Kind, so the schema expresses that as a runtime
//     invariant rather than a separate Cohort value.
//   - Input: either a synthetic transcript or a byte-stream ref. Audio
//     bytes never live in fixtures; they sit behind a sha256 ref.
//   - Expected: the outcome locked at design time. The runner reconciles
//     the observed pipeline outcome against Expected and never mutates
//     Expected.
//
// Categories cover every dimension in the brief: ambiguous locality,
// code-switching, unknown language/locality, noisy/clipped audio,
// nearest-route, other-route, claimed shortcut, arrival, reservation,
// emergency call, prompt injection, invented IDs, expired snapshot,
// cancellation, model outage, simple camera movement, repeat guidance,
// language change.
package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// SchemaVersion is the corpus format the runner understands. Bumping it
// is a tracked change to how cases are loaded — keep asserts explicit.
const SchemaVersion = "1.0.0"

// Split separates train / dev / eval. Only EVAL is part of measurement;
// TRAIN and DEV are rubric surfaces that the runner can verify against
// but never aggregates into measured rates.
type Split string

const (
	SplitTrain Split = "TRAIN"
	SplitDev   Split = "DEV"
	SplitEval  Split = "EVAL"
)

func (s Split) Valid() bool {
	switch s {
	case SplitTrain, SplitDev, SplitEval:
		return true
	}
	return false
}

// ProvenanceKind tags how a case was produced. SYNTHETIC = fabricated
// from named synthetic sources; CONSENTED = recorded speech with the
// documented consent trail (always loaded via manifest, never
// committed to source).
type ProvenanceKind string

const (
	ProvenanceSynthetic ProvenanceKind = "SYNTHETIC"
	ProvenanceConsented ProvenanceKind = "CONSENTED"
)

// Provenance carries the source/license/consent trail. License is the
// SPDX-ish identifier of the source material (LICENSED, SYNTHETIC, ...).
// ConsentID is required when Kind == CONSENTED; absolute outside-repo
// paths only.
type Provenance struct {
	Kind       ProvenanceKind `json:"kind"`
	Source     string         `json:"source"`
	License    string         `json:"license"`
	ConsentID  string         `json:"consent_id,omitempty"`
	RecordedAt string         `json:"recorded_at,omitempty"`
	Notes      string         `json:"notes,omitempty"`
}

// Cohort gates WHO spoke the audio. ADULT_SYNTHETIC covers the
// legitimate "synthetic adult utterance" — child/elder personal
// recordings are forbidden in Git at this layer regardless of consent,
// so the schema expresses the prohibition structurally.
type Cohort string

const (
	CohortAdultSynthetic Cohort = "ADULT_SYNTHETIC"
	// PIIForbid is a sentinel — it cannot appear in any committed case.
	// Real child/elder speech must be loaded as a CONSENTED manifest
	// entry whose SyntheticTranscript + SyntheticAudioDigest is set by
	// the offline-prep tool, never authored alongside the case.
	PIIForbid Cohort = "PII_FORBIDDEN"
)

// LangCohort groups the language dimension so a single case can carry
// one dialect and one cohort without exploding the schema.
type LangCohort struct {
	Language string `json:"language"`
	Dialect  string `json:"dialect,omitempty"`
	Region   string `json:"region,omitempty"`
	Cohort   Cohort `json:"cohort"`
}

// InputKind discriminates the input union.
type InputKind string

const (
	InputTranscript InputKind = "transcript"
	InputAudio      InputKind = "audio"
)

// Input carries the user's request shape — synthetic transcript or a
// content-addressed audio reference. Audio bytes never live in the
// fixture file: only the sha256 and an outside-repo path consumed by
// the consent-aware loader. SyntheticAudio tags a SYNTHETIC-provenance
// audio_ref whose profile (noise/clip) was generated from a synthetic
// source — it may commit the reference only because the underlying
// bytes live outside the repo under a synthetic generator.
type Input struct {
	Kind           InputKind `json:"kind"`
	Text           string    `json:"text,omitempty"`
	ContentType    string    `json:"content_type,omitempty"`
	AudioPath      string    `json:"audio_path,omitempty"`
	AudioByteSize  int64     `json:"audio_byte_size,omitempty"`
	AudioDigest    string    `json:"audio_digest,omitempty"`
	NoiseProfile   string    `json:"noise_profile,omitempty"`
	ClipAtSeconds  float64   `json:"clip_at_seconds,omitempty"`
	SyntheticAudio bool      `json:"synthetic_audio,omitempty"`
}

// Context mirrors the ScopedContext fields a case needs. Every field is
// optional; missing fields inherit from the harness baseline context.
// Resource refs are keyed by their typed ID, exactly as the worker
// receives them.
type Context struct {
	Language      string   `json:"language,omitempty"`
	TemplateKeys  []string `json:"template_keys,omitempty"`
	Places        []string `json:"places,omitempty"`
	RedZones      []string `json:"red_zones,omitempty"`
	SafeZones     []string `json:"safe_zones,omitempty"`
	Routes        []string `json:"routes,omitempty"`
	Facilities    []string `json:"facilities,omitempty"`
	DataVersion   string   `json:"data_version,omitempty"`
	SourceVersion int      `json:"source_version,omitempty"`
}

// ExpectedOutcome is the locked decision the runner must reconcile
// against. Non-OK outcomes carry no actions, no evidence_ids, no
// intents; OK carries typed action expectations.
type ExpectedOutcome string

const (
	ExpectOK              ExpectedOutcome = "OK"
	ExpectCLARIFY         ExpectedOutcome = "CLARIFY"
	ExpectUNSUPPORTED     ExpectedOutcome = "UNSUPPORTED"
	ExpectDATAUNAVAILABLE ExpectedOutcome = "DATA_UNAVAILABLE"
	ExpectERROR           ExpectedOutcome = "ERROR"
	ExpectREFUSE          ExpectedOutcome = "REFUSE"
	ExpectCancel          ExpectedOutcome = "CANCELED"
	ExpectDegraded        ExpectedOutcome = "DEGRADED"
)

// ActionSpec is one expected action of a successfully-evaluated case.
// Shape is loose because the contract here is "this kind of typed
// action must appear" — exact schema fields are validated by the
// orchestrator's JSON decoder, not the harness.
type ActionSpec struct {
	Type     string   `json:"type"`
	Contains []string `json:"contains,omitempty"` // IDs that must appear in the action's id slice
}

// SpeechSpec is one approved template expected to render.
type SpeechSpec struct {
	Key  string   `json:"key"`
	Args []string `json:"args,omitempty"` // arg names expected
}

// Expected is the case's locked ground truth. LatencyBudgetMS is a
// signal to the runner, not a guarantee — slower cases still count
// toward latency reporting.
type Expected struct {
	Outcome         ExpectedOutcome `json:"outcome"`
	Language        string          `json:"language,omitempty"`
	Intents         []string        `json:"intents,omitempty"`
	SpeechKeys      []SpeechSpec    `json:"speech_keys,omitempty"`
	Actions         []ActionSpec    `json:"actions,omitempty"`
	ClarifyIDs      []string        `json:"clarify_ids,omitempty"`
	EvidenceIDs     []string        `json:"evidence_ids,omitempty"`
	RejectIDs       []string        `json:"reject_ids,omitempty"` // IDs that must NOT appear in any action
	LatencyBudgetMS int             `json:"latency_budget_ms,omitempty"`
}

// Category is the dimension this case is exercising. Mirrors the brief.
type Category string

const (
	CategoryAmbiguousLocality Category = "AMBIGUOUS_LOCALITY"
	CategoryCodeSwitch        Category = "CODE_SWITCH"
	CategoryUnknownLocality   Category = "UNKNOWN_LOCALITY"
	CategoryUnknownLanguage   Category = "UNKNOWN_LANGUAGE"
	CategoryNoisyAudio        Category = "NOISY_AUDIO"
	CategoryClippedAudio      Category = "CLIPPED_AUDIO"
	CategoryNearestRoute      Category = "NEAREST_ROUTE"
	CategoryOtherRoute        Category = "OTHER_ROUTE"
	CategoryClaimedShortcut   Category = "CLAIMED_SHORTCUT"
	CategoryArrivalSelf       Category = "ARRIVAL_SELF_REPORT"
	CategoryReservation       Category = "RESERVATION_REQUEST"
	CategoryEmergencyCall     Category = "EMERGENCY_CALL"
	CategoryPromptInjection   Category = "PROMPT_INJECTION"
	CategoryInventedID        Category = "INVENTED_ID"
	CategoryExpiredSnapshot   Category = "EXPIRED_SNAPSHOT"
	CategoryCancellation      Category = "CANCELLATION"
	CategoryModelOutage       Category = "MODEL_OUTAGE"
	CategoryCameraMove        Category = "CAMERA_MOVE"
	CategoryRepeatGuidance    Category = "REPEAT_GUIDANCE"
	CategoryChangeLanguage    Category = "CHANGE_LANGUAGE"
)

// Case is one evaluation unit.
type Case struct {
	ID         string     `json:"id"`
	Split      Split      `json:"split"`
	Provenance Provenance `json:"provenance"`
	LangCohort LangCohort `json:"lang_cohort"`
	Category   Category   `json:"category"`
	Input      Input      `json:"input"`
	Context    Context    `json:"context,omitempty"`
	Expected   Expected   `json:"expected"`
	Notes      string     `json:"notes,omitempty"`
	SafetyTag  string     `json:"safety_tag,omitempty"`
}

// Suite is the on-disk file shape.
type Suite struct {
	SchemaVersion string `json:"schema_version"`
	Name          string `json:"name"`
	Notes         string `json:"notes,omitempty"`
	Cases         []Case `json:"cases"`
}

// categoryList is the master catalog. A case with a Category outside
// this list is rejected at load.
var categoryList = map[Category]struct{}{
	CategoryAmbiguousLocality: {},
	CategoryCodeSwitch:        {},
	CategoryUnknownLocality:   {},
	CategoryUnknownLanguage:   {},
	CategoryNoisyAudio:        {},
	CategoryClippedAudio:      {},
	CategoryNearestRoute:      {},
	CategoryOtherRoute:        {},
	CategoryClaimedShortcut:   {},
	CategoryArrivalSelf:       {},
	CategoryReservation:       {},
	CategoryEmergencyCall:     {},
	CategoryPromptInjection:   {},
	CategoryInventedID:        {},
	CategoryExpiredSnapshot:   {},
	CategoryCancellation:      {},
	CategoryModelOutage:       {},
	CategoryCameraMove:        {},
	CategoryRepeatGuidance:    {},
	CategoryChangeLanguage:    {},
}

var validID = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,80}$`)

// Validate runs schema-level invariants. Field-level validators (e.g.
// category must exist) are here; semantic checks (e.g. cohort gate) are
// in Probe.
func (c Case) Validate() error {
	if c.ID == "" {
		return errors.New("id required")
	}
	if !validID.MatchString(c.ID) {
		return fmt.Errorf("id %q: must match %s", c.ID, validID)
	}
	if !c.Split.Valid() {
		return fmt.Errorf("id %q: split %q invalid", c.ID, c.Split)
	}
	if c.LangCohort.Language == "" {
		return fmt.Errorf("id %q: lang_cohort.language required", c.ID)
	}
	if _, ok := categoryList[c.Category]; !ok {
		return fmt.Errorf("id %q: category %q not in catalog", c.ID, c.Category)
	}
	if c.Input.Kind == "" {
		return fmt.Errorf("id %q: input.kind required", c.ID)
	}
	switch c.Input.Kind {
	case InputTranscript:
		if strings.TrimSpace(c.Input.Text) == "" {
			return fmt.Errorf("id %q: synthetic transcript required for transcript input", c.ID)
		}
	case InputAudio:
		if c.Input.AudioDigest == "" {
			return fmt.Errorf("id %q: audio input requires audio_digest (sha256:hex)", c.ID)
		}
		if !validSha256Hex.MatchString(c.Input.AudioDigest) {
			return fmt.Errorf("id %q: audio_digest must be sha256 + 64 hex chars", c.ID)
		}
	default:
		return fmt.Errorf("id %q: input.kind %q unknown", c.ID, c.Input.Kind)
	}
	if !validExpectedOutcome(c.Expected.Outcome) {
		return fmt.Errorf("id %q: expected.outcome %q invalid", c.ID, c.Expected.Outcome)
	}
	if c.Provenance.Kind == "" {
		return fmt.Errorf("id %q: provenance.kind required", c.ID)
	}
	if c.LangCohort.Cohort == "" {
		return fmt.Errorf("id %q: lang_cohort.cohort required", c.ID)
	}
	return nil
}

// Probe runs additional safety invariants. SYNTHETIC provenance must
// never pair with cohort markings that suggest a real speaker was
// recorded; CONSENTED provenance must always carry a ConsentID.
//
// Returning an error here means the corpus is unsafe to ship. The
// runner refuses to load a suite that fails Probe.
func (c Case) Probe() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Provenance.Kind == ProvenanceConsented && c.Provenance.ConsentID == "" {
		return fmt.Errorf("id %q: consented case missing consent_id", c.ID)
	}
	if c.LangCohort.Cohort == PIIForbid {
		return fmt.Errorf("id %q: cohort PII_FORBIDDEN must not appear in cases", c.ID)
	}
	if c.Input.Kind == InputAudio {
		// Real recorded audio must be CONSENTED. Synthetic-audio
		// references (engineered noise / clipped synthetic speech)
		// are also allowed under SYNTHETIC provenance when
		// SyntheticAudio is set and the notes declare the source.
		switch c.Provenance.Kind {
		case ProvenanceConsented:
			if c.Input.AudioPath == "" {
				return fmt.Errorf("id %q: audio input requires audio_path (outside-repo)", c.ID)
			}
		case ProvenanceSynthetic:
			if !c.Input.SyntheticAudio {
				return fmt.Errorf("id %q: SYNTHETIC audio must be tagged synthetic_audio=true", c.ID)
			}
			if c.Input.AudioPath == "" {
				return fmt.Errorf("id %q: synthetic audio reference requires audio_path (outside-repo)", c.ID)
			}
		}
	}
	if c.Input.NoiseProfile != "" && c.Input.Kind != InputAudio {
		return fmt.Errorf("id %q: noise_profile only valid for audio input", c.ID)
	}
	if c.Input.ClipAtSeconds != 0 && c.Input.Kind != InputAudio {
		return fmt.Errorf("id %q: clip_at_seconds only valid for audio input", c.ID)
	}
	// OK and CLARIFY are both "outcome present" shapes — they can carry
	// actions / speech_keys / clarify_ids / intents. The remaining
	// outcomes (UNSUPPORTED, DATA_UNAVAILABLE, ERROR, REFUSE,
	// CANCELED, DEGRADED) carry a typed SpeechKey (the failure
	// announcement) but no observed-actions or clarify_ids.
	if c.Expected.Outcome != ExpectOK && c.Expected.Outcome != ExpectCLARIFY {
		if len(c.Expected.Intents) > 0 ||
			len(c.Expected.Actions) > 0 ||
			len(c.Expected.ClarifyIDs) > 0 {
			return fmt.Errorf("id %q: outcome %q must not carry intents/actions/clarify", c.ID, c.Expected.Outcome)
		}
	}
	return nil
}

// IsRelevant reports whether this case is in scope for a given run
// mode. RunMode is the runner's filter string; filters split by
// language or cohort or category.
func (c Case) IsRelevant(filter map[string]string) bool {
	for k, v := range filter {
		switch k {
		case "language":
			if c.LangCohort.Language != v {
				return false
			}
		case "dialect":
			if c.LangCohort.Dialect != v {
				return false
			}
		case "region":
			if c.LangCohort.Region != v {
				return false
			}
		case "split":
			if string(c.Split) != v {
				return false
			}
		case "category":
			if string(c.Category) != v {
				return false
			}
		case "cohort":
			if string(c.LangCohort.Cohort) != v {
				return false
			}
		}
	}
	return true
}

// TextDigest computes the case's stable identity hash so two cases with
// the same textual content can be detected. The hash covers all
// decision fields the runner compares against.
func (c Case) TextDigest() string {
	h := sha256.New()
	h.Write([]byte(c.ID))
	h.Write([]byte{0})
	h.Write([]byte(c.Split))
	h.Write([]byte{0})
	h.Write([]byte(string(c.Category)))
	h.Write([]byte{0})
	h.Write([]byte(c.LangCohort.Language))
	h.Write([]byte{0})
	h.Write([]byte(c.Input.Text))
	h.Write([]byte{0})
	h.Write([]byte(c.Input.AudioDigest))
	h.Write([]byte{0})
	h.Write([]byte(string(c.Expected.Outcome)))
	for _, s := range c.Expected.SpeechKeys {
		h.Write([]byte{0})
		h.Write([]byte(s.Key))
		sort.Strings(s.Args)
		for _, a := range s.Args {
			h.Write([]byte{1})
			h.Write([]byte(a))
		}
	}
	for _, id := range c.Expected.ClarifyIDs {
		h.Write([]byte{0})
		h.Write([]byte(id))
	}
	return hex.EncodeToString(h.Sum(nil))
}

var validSha256Hex = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validExpectedOutcome(o ExpectedOutcome) bool {
	switch o {
	case ExpectOK, ExpectCLARIFY, ExpectUNSUPPORTED, ExpectDATAUNAVAILABLE,
		ExpectERROR, ExpectREFUSE, ExpectCancel, ExpectDegraded:
		return true
	}
	return false
}

// ValidateSuite verifies every case, then suite-level invariants
// (unique IDs, schema version).
func ValidateSuite(s Suite) error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q: expected %q", s.SchemaVersion, SchemaVersion)
	}
	seen := map[string]string{}
	for _, c := range s.Cases {
		if err := c.Probe(); err != nil {
			return err
		}
		if prev, ok := seen[c.ID]; ok {
			return fmt.Errorf("duplicate id %q (first seen at index %s)", c.ID, prev)
		}
		seen[c.ID] = c.ID
	}
	return nil
}

// Parse loads JSON bytes into a Suite and validates. The caller owns
// the byte source — file path or feature flag plumbed in by main.
func Parse(data []byte) (Suite, error) {
	var s Suite
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Suite{}, fmt.Errorf("corpus json: %w", err)
	}
	if err := ValidateSuite(s); err != nil {
		return Suite{}, err
	}
	return s, nil
}
