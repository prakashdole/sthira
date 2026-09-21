package provider

import (
	"context"
	"errors"
	"sync"

	"sthira/backend/eval/corpus"
)

// Deterministic is a rule-based stand-in implementing Provider. It is
// NOT real ASR / middle / TTS. Its only job is to let the harness
// exercise its reconciliation, its report writers and its filters
// without standing up workers.
//
// Determinism: the response is fully derived from the Case ID, the
// transcript and the configured outcome. Equal inputs always produce
// equal outputs.
type Deterministic struct {
	mu sync.Mutex

	// Probe rule table. The case's Category chooses which rule fires.
	// A rule that needs a Category not in the table returns
	// ErrUnsupported, which the runner records as NOT_EVALUATED.
	probe map[corpus.Category]deterministicRule
}

type deterministicRule struct {
	ASR       func(req ASRRequest) (ASROutcome, error)
	Middle    func(req MiddleRequest) (MiddleOutcome, error)
	TTS       func(req TTSRequest) (TTSOutcome, error)
	Languages map[string]struct{}
}

// NewDeterministic returns a Deterministic provider initialized with
// the rule table that mirrors the synthetic suite.
func NewDeterministic() *Deterministic {
	d := &Deterministic{probe: map[corpus.Category]deterministicRule{}}
	d.installDefaultRules()
	return d
}

func (d *Deterministic) Mode() Mode { return ModeDeterministic }

// installDefaultRules wires a rule for every Category that ships in the
// synthetic suite. Rules return ErrorOutcome or DataUnavailable for
// failure-model categories; CLARIFY for ambiguous/name conflict
// categories; OK for direct intent categories.
func (d *Deterministic) installDefaultRules() {
	d.probe[corpus.CategoryAmbiguousLocality] = deterministicRule{
		ASR:    echoTranscript,
		Middle: clarifyWithKnownIDs,
		TTS:    ttsClarify,
	}
	d.probe[corpus.CategoryCodeSwitch] = deterministicRule{
		ASR:    echoTranscript,
		Middle: focusPlace,
		TTS:    ttsAcknowledgment,
	}
	d.probe[corpus.CategoryUnknownLocality] = deterministicRule{
		ASR:    echoTranscript,
		Middle: dataUnavailable,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryUnknownLanguage] = deterministicRule{
		ASR:    unsupportedLanguageASR,
		Middle: unsupportedLanguageMiddle,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryNoisyAudio] = deterministicRule{
		ASR:    unknownAudioASR,
		Middle: clarifyUnknownAudio,
		TTS:    ttsClarify,
	}
	d.probe[corpus.CategoryClippedAudio] = deterministicRule{
		ASR:    unknownAudioASR,
		Middle: clarifyUnknownAudio,
		TTS:    ttsClarify,
	}
	d.probe[corpus.CategoryNearestRoute] = deterministicRule{
		ASR:    echoTranscript,
		Middle: nearestRoute,
		TTS:    ttsAcknowledgment,
	}
	d.probe[corpus.CategoryOtherRoute] = deterministicRule{
		ASR:    echoTranscript,
		Middle: otherRoute,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryClaimedShortcut] = deterministicRule{
		ASR:    echoTranscript,
		Middle: shortcutRefused,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryArrivalSelf] = deterministicRule{
		ASR:    echoTranscript,
		Middle: arrivalPanel,
		TTS:    ttsAcknowledgment,
	}
	d.probe[corpus.CategoryReservation] = deterministicRule{
		ASR:    echoTranscript,
		Middle: reservationPanel,
		TTS:    ttsAcknowledgment,
	}
	d.probe[corpus.CategoryEmergencyCall] = deterministicRule{
		ASR:    echoTranscript,
		Middle: emergencyPanel,
		TTS:    ttsAcknowledgment,
	}
	d.probe[corpus.CategoryPromptInjection] = deterministicRule{
		ASR:    echoTranscript,
		Middle: injectionSurvives,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryInventedID] = deterministicRule{
		ASR:    echoTranscript,
		Middle: unknownRoute,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryExpiredSnapshot] = deterministicRule{
		ASR:    echoTranscript,
		Middle: dataUnavailable,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryCancellation] = deterministicRule{
		ASR:    echoTranscript,
		Middle: cancelled,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryModelOutage] = deterministicRule{
		ASR:    echoTranscript,
		Middle: degraded,
		TTS:    ttsRefuse,
	}
	d.probe[corpus.CategoryCameraMove] = deterministicRule{
		ASR:    echoTranscript,
		Middle: cameraMove,
		TTS:    ttsSilent,
	}
	d.probe[corpus.CategoryRepeatGuidance] = deterministicRule{
		ASR:    echoTranscript,
		Middle: repeatGuidance,
		TTS:    ttsAcknowledgment,
	}
	d.probe[corpus.CategoryChangeLanguage] = deterministicRule{
		ASR:    echoTranscript,
		Middle: changeLanguage,
		TTS:    ttsAcknowledgment,
	}
}

// ASR returns the deterministic ASR answer. Echoes the
// transcript when the input is synthetic; returns an unsupported state
// for unknown-language inputs.
func (d *Deterministic) ASR(ctx context.Context, req ASRRequest) (ASROutcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	rule, ok := d.probe[req.Case.Category]
	if !ok {
		return ASROutcome{}, errUnsupported(req.Case, "no deterministic ASR rule")
	}
	if rule.ASR == nil {
		return ASROutcome{}, errUnsupported(req.Case, "no ASR rule")
	}
	if !d.ruleApplies(rule, req.Case.LangCohort.Language) {
		return ASROutcome{}, errUnsupported(req.Case, "language not in rule table")
	}
	if err := ctx.Err(); err != nil {
		return ASROutcome{}, err
	}
	return rule.ASR(req)
}

// Middle returns the deterministic Middle answer.
func (d *Deterministic) Middle(ctx context.Context, req MiddleRequest) (MiddleOutcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	rule, ok := d.probe[req.Case.Category]
	if !ok {
		return MiddleOutcome{}, errUnsupported(req.Case, "no deterministic Middle rule")
	}
	if rule.Middle == nil {
		return MiddleOutcome{}, errUnsupported(req.Case, "no Middle rule")
	}
	if !d.ruleApplies(rule, req.Case.LangCohort.Language) {
		return MiddleOutcome{}, errUnsupported(req.Case, "language not in rule table")
	}
	if err := ctx.Err(); err != nil {
		return MiddleOutcome{}, err
	}
	return rule.Middle(req)
}

// TTS returns the deterministic TTS answer.
func (d *Deterministic) TTS(ctx context.Context, req TTSRequest) (TTSOutcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	rule, ok := d.probe[req.Case.Category]
	if !ok {
		return TTSOutcome{}, errUnsupported(req.Case, "no deterministic TTS rule")
	}
	if rule.TTS == nil {
		return TTSOutcome{}, errUnsupported(req.Case, "no TTS rule")
	}
	if !d.ruleApplies(rule, req.Case.LangCohort.Language) {
		return TTSOutcome{}, errUnsupported(req.Case, "language not in rule table")
	}
	if err := ctx.Err(); err != nil {
		return TTSOutcome{}, err
	}
	return rule.TTS(req)
}

// ruleApplies tests whether the rule has a binding for the case's
// language. A missing binding means the language is NOT in
// scope for the rule and the case is NOT_EVALUATED at every stage.
func (d *Deterministic) ruleApplies(r deterministicRule, lang string) bool {
	if len(r.Languages) == 0 {
		return true
	}
	_, ok := r.Languages[lang]
	return ok
}

func errUnsupported(c corpus.Case, why string) error {
	return errors.Join(ErrUnsupported, errors.New(c.ID+": "+why))
}

// --- default rules -------------------------------------------------

func echoTranscript(req ASRRequest) (ASROutcome, error) {
	if req.Case.Input.Kind != corpus.InputTranscript {
		return ASROutcome{State: "AUDIO_UNAVAILABLE", Language: req.Case.LangCohort.Language}, nil
	}
	return ASROutcome{
		State:      "OK",
		Text:       req.Case.Input.Text,
		Language:   req.Case.LangCohort.Language,
		RevisionID: "deterministic-v1",
	}, nil
}

func unsupportedLanguageASR(req ASRRequest) (ASROutcome, error) {
	return ASROutcome{State: "UNSUPPORTED_LANGUAGE", Language: req.Case.LangCohort.Language}, nil
}

func unknownAudioASR(req ASRRequest) (ASROutcome, error) {
	return ASROutcome{State: "AUDIO_UNAVAILABLE", Language: req.Case.LangCohort.Language}, nil
}

func clarifyWithKnownIDs(req MiddleRequest) (MiddleOutcome, error) {
	ids := req.Case.Expected.ClarifyIDs
	if len(ids) == 0 {
		return MiddleOutcome{}, errors.New("ambiguous case missing clarify_ids")
	}
	actions := []string{"FOCUS_FEATURE:" + ids[0]}
	return MiddleOutcome{
		Status:      "CLARIFY",
		Intent:      "FOCUS_PLACE",
		Language:    req.Case.LangCohort.Language,
		Actions:     actions,
		SpeechKey:   "clarify_place",
		ClarifyIDs:  ids,
		EvidenceIDs: ids,
	}, nil
}

func focusPlace(req MiddleRequest) (MiddleOutcome, error) {
	place := firstID(req.Case.Context.Places)
	if place == "" {
		return MiddleOutcome{}, errors.New("code-switch case missing place id")
	}
	return MiddleOutcome{
		Status:      "OK",
		Intent:      "FOCUS_PLACE",
		Language:    req.Case.LangCohort.Language,
		Actions:     []string{"FOCUS_FEATURE:" + place},
		EvidenceIDs: []string{place},
	}, nil
}

func dataUnavailable(_ MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:    "DATA_UNAVAILABLE",
		Language:  "ml-IN",
		SpeechKey: "verified_route_unavailable",
	}, nil
}

func unsupportedLanguageMiddle(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{Status: "UNSUPPORTED", Language: req.Case.LangCohort.Language}, nil
}

func clarifyUnknownAudio(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:    "CLARIFY",
		Language:  req.Case.LangCohort.Language,
		SpeechKey: "clarify_audio_unclear",
	}, nil
}

func nearestRoute(req MiddleRequest) (MiddleOutcome, error) {
	choice := firstChoiceWithFacility(req.Case)
	if choice == "" {
		return MiddleOutcome{}, errors.New("nearest case missing facility")
	}
	return MiddleOutcome{
		Status:      "OK",
		Intent:      "LIST_DESTINATIONS",
		Language:    req.Case.LangCohort.Language,
		Actions:     []string{"SHOW_CHOICES:" + choice},
		SpeechKey:   "destination_options",
		EvidenceIDs: []string{choice},
	}, nil
}

func otherRoute(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:      "DATA_UNAVAILABLE",
		Language:    req.Case.LangCohort.Language,
		SpeechKey:   "verified_route_unavailable",
		EvidenceIDs: req.Case.Context.Routes,
	}, nil
}

func shortcutRefused(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:    "DATA_UNAVAILABLE",
		Language:  req.Case.LangCohort.Language,
		SpeechKey: "verified_route_unavailable",
	}, nil
}

func arrivalPanel(req MiddleRequest) (MiddleOutcome, error) {
	facility := firstID(req.Case.Context.Facilities)
	if facility == "" {
		return MiddleOutcome{}, errors.New("arrival case missing facility")
	}
	return MiddleOutcome{
		Status:      "OK",
		Intent:      "OPEN_CONFIRMATION",
		Language:    req.Case.LangCohort.Language,
		Actions:     []string{"OPEN_PANEL:ARRIVAL_CONFIRMATION:" + facility},
		EvidenceIDs: []string{facility},
	}, nil
}

func reservationPanel(req MiddleRequest) (MiddleOutcome, error) {
	facility := firstID(req.Case.Context.Facilities)
	if facility == "" {
		return MiddleOutcome{}, errors.New("reservation case missing facility")
	}
	return MiddleOutcome{
		Status:      "OK",
		Intent:      "OPEN_CONFIRMATION",
		Language:    req.Case.LangCohort.Language,
		Actions:     []string{"OPEN_PANEL:RESERVATION_CONFIRMATION:" + facility},
		EvidenceIDs: []string{facility},
	}, nil
}

func emergencyPanel(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:      "OK",
		Intent:      "OPEN_CONFIRMATION",
		Language:    req.Case.LangCohort.Language,
		Actions:     []string{"OPEN_PANEL:EMERGENCY_CALL_CONFIRMATION"},
		EvidenceIDs: []string{"PLACE-DEMO-1"},
	}, nil
}

func injectionSurvives(req MiddleRequest) (MiddleOutcome, error) {
	// The harness treats the transcript as data. The deterministic
	// rule answers with the ordinary outcome — a poison instruction
	// cannot mutate the rule. If the corpus is doing its job, the
	// expected outcome is the same as for ordinary focus-place.
	if firstID(req.Case.Context.Places) == "" {
		return MiddleOutcome{}, errors.New("injection case missing place")
	}
	return focusPlace(req)
}

func unknownRoute(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:    "DATA_UNAVAILABLE",
		Language:  req.Case.LangCohort.Language,
		SpeechKey: "verified_route_unavailable",
	}, nil
}

func cancelled(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{Status: "CANCELED", Language: req.Case.LangCohort.Language}, nil
}

func degraded(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:      "DEGRADED",
		Language:    req.Case.LangCohort.Language,
		SpeechKey:   "partial_response",
		EvidenceIDs: []string{firstID(req.Case.Context.SafeZones)},
	}, nil
}

func cameraMove(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:   "OK",
		Intent:   "ZOOM",
		Language: req.Case.LangCohort.Language,
		Actions:  []string{"ZOOM:IN:1"},
	}, nil
}

func repeatGuidance(req MiddleRequest) (MiddleOutcome, error) {
	return MiddleOutcome{
		Status:    "OK",
		Intent:    "REPEAT_GUIDANCE",
		Language:  req.Case.LangCohort.Language,
		Actions:   []string{"REPEAT_GUIDANCE"},
		SpeechKey: "repeat_template",
	}, nil
}

func changeLanguage(req MiddleRequest) (MiddleOutcome, error) {
	lang := req.Case.LangCohort.Language
	return MiddleOutcome{
		Status:   "OK",
		Intent:   "CHANGE_LANGUAGE",
		Language: lang,
		Actions:  []string{"SET_LANGUAGE:" + lang},
	}, nil
}

func ttsAcknowledgment(_ TTSRequest) (TTSOutcome, error) {
	return TTSOutcome{State: "OK", SpeechKey: "approved_guidance", ByteSize: 1024, DurationMS: 240}, nil
}

func ttsClarify(_ TTSRequest) (TTSOutcome, error) {
	return TTSOutcome{State: "OK", SpeechKey: "clarify_place", ByteSize: 1024, DurationMS: 320}, nil
}

func ttsRefuse(_ TTSRequest) (TTSOutcome, error) {
	return TTSOutcome{State: "OK", SpeechKey: "verified_route_unavailable", ByteSize: 1024, DurationMS: 280}, nil
}

func ttsSilent(_ TTSRequest) (TTSOutcome, error) {
	return TTSOutcome{State: "OK", SpeechKey: "camera_move_silent", ByteSize: 0, DurationMS: 0}, nil
}

// --- helpers -------------------------------------------------------

func firstID(xs []string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

// routesForFacility reads the VerifiedRoutes field via the opaque map
// (the case only carries IDs, not bundles). We just return the route
// list verbatim; the runner does the resolution against the harness
// baseline context.
func routesForFacility(c corpus.Case, _ string) []string {
	out := make([]string, 0, len(c.Context.Routes))
	for _, r := range c.Context.Routes {
		if r == "" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func firstChoiceWithFacility(c corpus.Case) string {
	for _, f := range c.Context.Facilities {
		if f == "" {
			continue
		}
		return f
	}
	return ""
}
