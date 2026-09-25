package corpus

import (
	"strings"
	"testing"
)

const goodCase = `{
  "id": "test.synthetic.transcript.001",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "synthetic-fixture", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "CAMERA_MOVE",
  "input": {"kind": "transcript", "text": "hello"},
  "context": {"language": "ml-IN"},
  "expected": {"outcome": "OK", "intents": ["FOCUS_PLACE"], "latency_budget_ms": 1000}
}`

// Probe accepts the good fixture.
func TestCaseAcceptsGood(t *testing.T) {
	c, err := parseCase(goodCase)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := c.Probe(); err != nil {
		t.Fatalf("probe: %v", err)
	}
}

// Each probe/validate rule has at least one negative case below.
func TestCaseRejectsEmptyID(t *testing.T) {
	if err := parseHasError(`{
  "id": "",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "AMBIGUOUS_LOCALITY",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "CLARIFY"}
}`); err == nil {
		t.Fatalf("expected error: empty id")
	}
}

func TestCaseRejectsBadSplit(t *testing.T) {
	if err := parseHasError(`{
  "id": "test.bad.split",
  "split": "PRODUCTION",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "AMBIGUOUS_LOCALITY",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "CLARIFY"}
}`); err == nil {
		t.Fatalf("expected error: invalid split")
	}
}

func TestCaseRejectsUnknownCategory(t *testing.T) {
	if err := parseHasError(`{
  "id": "test.bad.cat",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "MAKE_COFFEE",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "OK", "intents": ["FOCUS_PLACE"]}
}`); err == nil {
		t.Fatalf("expected error: unknown category")
	}
}

func TestCaseRejectsAudioWithoutSyntheticFlag(t *testing.T) {
	if err := parseHasError(`{
  "id": "test.audio.notagsynthetic",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "NOISY_AUDIO",
  "input": {"kind": "audio", "content_type": "audio/wav",
    "audio_digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
    "audio_path": "/outside-repo/synthetic/x.wav",
    "audio_byte_size": 100, "noise_profile": "pink-30db"},
  "expected": {"outcome": "CLARIFY"}
}`); err == nil {
		t.Fatalf("expected error: SYNTHETIC audio must set synthetic_audio=true")
	}
}

func TestCaseAcceptsSyntheticAudio(t *testing.T) {
	c, err := parseCase(`{
  "id": "test.audio.tagsynthetic.ok",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "NOISY_AUDIO",
  "input": {"kind": "audio", "content_type": "audio/wav",
    "audio_digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
    "audio_path": "/outside-repo/synthetic/x.wav",
    "audio_byte_size": 100, "noise_profile": "pink-30db",
    "synthetic_audio": true},
  "expected": {"outcome": "CLARIFY"}
}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := c.Probe(); err != nil {
		t.Fatalf("probe: %v", err)
	}
}

func TestCaseRejectsPIIForbid(t *testing.T) {
	if err := parseHasError(`{
  "id": "test.child.bad",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "PII_FORBIDDEN"},
  "category": "AMBIGUOUS_LOCALITY",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "CLARIFY"}
}`); err == nil {
		t.Fatalf("expected error: PII_FORBIDDEN refused")
	}
}

func TestConsentedMustHaveConsentID(t *testing.T) {
	if err := parseHasError(`{
  "id": "test.consent.no.id",
  "split": "EVAL",
  "provenance": {"kind": "CONSENTED", "source": "x", "license": "OWN-RECORDED"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "AMBIGUOUS_LOCALITY",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "CLARIFY"}
}`); err == nil {
		t.Fatalf("expected error: CONSENTED missing consent_id")
	}
}

func TestNonOKOutcomeCannotCarryIntents(t *testing.T) {
	if err := parseHasError(`{
  "id": "test.nonok.no.intent",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "EXPIRED_SNAPSHOT",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "DATA_UNAVAILABLE", "intents": ["FOCUS_PLACE"]}
}`); err == nil {
		t.Fatalf("expected error: non-OK outcome with intents")
	}
}

func TestOKOutcomeCanCarryIntents(t *testing.T) {
	c, err := parseCase(`{
  "id": "test.ok.intent",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "CAMERA_MOVE",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "OK", "intents": ["FOCUS_PLACE"]}
}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := c.Probe(); err != nil {
		t.Fatalf("ok case rejected: %v", err)
	}
}

func TestIsRelevantFilter(t *testing.T) {
	c, err := parseCase(`{
  "id": "test.filter",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "AMBIGUOUS_LOCALITY",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "CLARIFY"}
}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.IsRelevant(map[string]string{"language": "ml-IN"}) {
		t.Fatalf("expected relevant for ml-IN")
	}
	if c.IsRelevant(map[string]string{"language": "hi-IN"}) {
		t.Fatalf("expected not relevant for hi-IN")
	}
}

func TestTextDigestStable(t *testing.T) {
	c1, err := parseCase(`{
  "id": "t.digest",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "CAMERA_MOVE",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "OK", "intents": ["FOCUS_PLACE"]}
}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c2, err := parseCase(`{
  "id": "t.digest",
  "split": "EVAL",
  "provenance": {"kind": "SYNTHETIC", "source": "x", "license": "SYNTHETIC"},
  "lang_cohort": {"language": "ml-IN", "cohort": "ADULT_SYNTHETIC"},
  "category": "CAMERA_MOVE",
  "input": {"kind": "transcript", "text": "x"},
  "expected": {"outcome": "OK", "intents": ["FOCUS_PLACE"]}
}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c1.TextDigest() != c2.TextDigest() {
		t.Fatalf("digest drift: %s vs %s", c1.TextDigest(), c2.TextDigest())
	}
}

func parseCase(s string) (Case, error) {
	wrapped := `{"schema_version":"` + SchemaVersion + `","cases":[` + s + `]}`
	suite, err := Parse([]byte(wrapped))
	if err != nil {
		return Case{}, err
	}
	return suite.Cases[0], nil
}

func parseHasError(s string) error {
	_, err := parseCase(s)
	return err
}

// Test that DisallowUnknownFields keeps the schema strict — adding a
// typo'd key should be rejected.
func TestStrictFields(t *testing.T) {
	wrapped := `{"schema_version":"` + SchemaVersion + `","cases":[], "wrongField": 1}`
	if _, err := Parse([]byte(wrapped)); err == nil {
		t.Fatalf("expected error: unknown field")
	} else if !strings.Contains(err.Error(), "wrongField") {
		t.Fatalf("expected wrongField in error: %v", err)
	}
}
