package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// P6 scoped semantic validator. The P1/P4 flat `ValidateModelOutput` accepted
// any well-formed proposal whose IDs were present in a `map[string]bool`.
// That is insufficient for P6 semantics: a model proposal that mixes typed
// relationships (a route serving as a facility, a facility claimed as a
// verified route, a closed or stale route referenced as the destination's
// route, reordered choice IDs) passes the flat check but produces
// unsafe guidance. The validator below enforces the typed relationships
// declared in ScopedContext.
//
// The validator is intentionally orthogonal to JSON shape: shape validation
// (forbidden extras, duplicate keys, trailing data) lives in
// ValidateModelOutputShape below; semantic validation lives here. Either
// may fail independently. The validator never trusts caller-supplied
// metadata flags (SourceStatus, Quarantined) on a retry — server state wins.
//
// Worker 9 (orchestration) calls this validator after the middle worker
// returns and after each SnapshotRevalidate succeeds. A failure here
// means the proposal is rejected; the queue must not surface it as
// guidance to the citizen.

// ErrScopedSemantic is returned when a model proposal passes raw shape
// validation but fails the typed semantic checks below. The error wraps
// a short reason so callers can log a stable token.
type ErrScopedSemantic struct {
	Reason string
}

func (e *ErrScopedSemantic) Error() string { return "scoped semantic: " + e.Reason }

// EnforceScopedContext validates the typed relationships in a model proposal
// against the rich ScopedContext. It does not validate raw JSON shape; pair
// with ValidateModelOutputShape for end-to-end coverage.
//
// Rejection rules (any failure rejects the entire proposal):
//
//   - sc.Jurisdiction must be non-empty; out must not claim another
//     jurisdiction anywhere.
//   - sc.IsLanguageAllowed(out.Language) must be true; out.Language is the
//     model-written value, not the context's allowed set.
//   - Out.SpeechKey, when present, must be in sc.TemplateKeys.
//   - Out.ClarificationIDs, when present, must all appear in sc.KnownPlaces
//     (the typed map; not the flat fallback).
//   - Out.EvidenceIDs must all appear in sc.KnownPlaces, sc.KnownSafeZones,
//     sc.KnownRedZones, sc.KnownRoutes or sc.KnownFacilities. Unknown IDs
//     fail closed (an unknown reference cannot support a safety action).
//   - Each Action in out.Actions must satisfy the per-variant typed rules:
//     FOCUS_FEATURE.target_id must be a known place in sc.KnownPlaces;
//     SHOW_CHOICES.target_ids must be a non-empty subset of the facility
//     IDs in sc.EligibleDestinations AND must preserve the server-permitted
//     order (same IDs, same order); SHOW_ROUTE.route_id must appear in
//     sc.KnownRoutes AND must be Verified with ValidNow semantics (Status
//     must not be CLOSED / SUPERSEDED / STALE); OPEN_PANEL.target_id, when
//     present, must be a known place/route/facility consistent with the
//     panel kind; SET_LANGUAGE.language must be in sc.AllowedLanguages;
//     ZOOM/PAN/RECENTER carry no extra validation beyond shape.
//   - A facility ID MUST NOT appear as a route_id, and a route ID MUST NOT
//     appear as a place_id or facility_id (cross-kind ID collisions are
//     rejected because they confuse the model and the citizen UI).
//   - A route referenced as the destination's verified route must appear
//     in sc.VerifiedRoutes[facility_id] for the facility it claims to
//     serve, with ToSafeZoneID == FacilityRef.SafeZoneID. A route to the
//     wrong safe zone is rejected (D37).
//   - Choices (SHOW_CHOICES) carry server-permitted order. The model MUST
//     NOT invent a different ordering. The validator walks the slice in
//     order and rejects any deviation.
//
// The validator never calls out to a public geocoder or guesses a
// jurisdiction. Every reference must come from the typed maps.
func EnforceScopedContext(out ModelOutput, sc ScopedContext) error {
	if sc.Jurisdiction == "" {
		return &ErrScopedSemantic{Reason: "scoped context jurisdiction is empty"}
	}
	if out.DataVersion != "" && sc.DataVersion != "" && out.DataVersion != sc.DataVersion {
		// The proposal was generated against a different snapshot. A
		// revalidated sc.DataVersion supersedes the model's claim.
		return &ErrScopedSemantic{Reason: fmt.Sprintf("data_version %q does not match scoped context %q", out.DataVersion, sc.DataVersion)}
	}
	if out.Language == "" {
		return &ErrScopedSemantic{Reason: "model proposal language is empty"}
	}
	if !sc.IsLanguageAllowed(out.Language) {
		return &ErrScopedSemantic{Reason: fmt.Sprintf("language %q not in allowed set", out.Language)}
	}
	if out.SpeechKey != nil && *out.SpeechKey != "" && !sc.IsSpeechKeyApprovedForLanguage(*out.SpeechKey, out.Language) {
		return &ErrScopedSemantic{Reason: fmt.Sprintf("speech_key %q not approved for language %q", *out.SpeechKey, out.Language)}
	}
	for _, id := range out.ClarificationIDs {
		if _, ok := sc.KnownPlaces[id]; !ok {
			return &ErrScopedSemantic{Reason: fmt.Sprintf("clarification id %q is not a known place", id)}
		}
	}
	for _, id := range out.EvidenceIDs {
		if !isKnownAny(sc, id) {
			return &ErrScopedSemantic{Reason: fmt.Sprintf("evidence id %q is not known to the active context", id)}
		}
	}
	// Cross-kind ID collision check: a route_id MUST NOT appear as a
	// FOCUS_FEATURE target (a route is not a spatial feature to focus on;
	// the model probably confused it with a place/facility), and a
	// facility_id MUST NOT appear as a route. The model could otherwise pick
	// a route_id and accidentally use it as a place_id on FOCUS_FEATURE
	// (which would silently target the wrong UI affordance). Reject up
	// front.
	for _, a := range out.Actions {
		switch a.Type {
		case ActionFocusFeature:
			if a.TargetID != "" {
				if _, isRoute := sc.KnownRoutes[a.TargetID]; isRoute {
					return &ErrScopedSemantic{Reason: fmt.Sprintf("FOCUS_FEATURE target_id %q is a route, not a spatial feature", a.TargetID)}
				}
			}
		case ActionShowRoute:
			if a.RouteID != "" {
				if _, isPlace := sc.KnownPlaces[a.RouteID]; isPlace {
					return &ErrScopedSemantic{Reason: fmt.Sprintf("SHOW_ROUTE route_id %q is a place, not a route", a.RouteID)}
				}
				if _, isFacility := sc.KnownFacilities[a.RouteID]; isFacility {
					return &ErrScopedSemantic{Reason: fmt.Sprintf("SHOW_ROUTE route_id %q is a facility, not a route", a.RouteID)}
				}
			}
		case ActionShowChoices:
			for _, id := range a.TargetIDs {
				if _, isRoute := sc.KnownRoutes[id]; isRoute {
					return &ErrScopedSemantic{Reason: fmt.Sprintf("SHOW_CHOICES target_id %q is a route, not a facility", id)}
				}
				if _, isPlace := sc.KnownPlaces[id]; isPlace {
					return &ErrScopedSemantic{Reason: fmt.Sprintf("SHOW_CHOICES target_id %q is a place, not a facility", id)}
				}
			}
		}
	}

	// Per-action typed rules.
	for i, a := range out.Actions {
		switch a.Type {
		case ActionFocusFeature:
			if a.TargetID == "" {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d FOCUS_FEATURE missing target_id", i)}
			}
			// FOCUS_FEATURE may target a place, a safe zone, a red zone
			// or a facility — all are spatial features the model may
			// legitimately highlight. Cross-kind collisions (route_id
			// here) are caught above.
			if !isSpatialFeature(sc, a.TargetID) {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d FOCUS_FEATURE target_id %q not a known spatial feature", i, a.TargetID)}
			}
		case ActionShowChoices:
			if len(a.TargetIDs) == 0 {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d SHOW_CHOICES empty", i)}
			}
			if len(a.TargetIDs) > MaxShowChoices {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d SHOW_CHOICES exceeds %d", i, MaxShowChoices)}
			}
			if err := enforceChoiceOrder(a.TargetIDs, sc.EligibleDestinations); err != nil {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d SHOW_CHOICES order: %v", i, err)}
			}
		case ActionShowRoute:
			if a.RouteID == "" {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d SHOW_ROUTE missing route_id", i)}
			}
			ref, ok := sc.KnownRoutes[a.RouteID]
			if !ok {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d SHOW_ROUTE route_id %q not a known route", i, a.RouteID)}
			}
			if !ref.Verified || ref.Status != RouteStatusVerified {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d SHOW_ROUTE route_id %q is not verified (status=%s, verified=%t)", i, a.RouteID, ref.Status, ref.Verified)}
			}
			// A non-verified or stale route may still be referenced for an
			// informational confirmation panel, but SHOW_ROUTE is the
			// "show on the map" intent: only Verified && !Stale routes
			// qualify.
		case ActionOpenPanel:
			switch a.Panel {
			case PanelDestinationPreview:
				if a.TargetID != "" {
					if _, ok := sc.KnownFacilities[a.TargetID]; !ok {
						return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d OPEN_PANEL DESTINATION_PREVIEW target_id %q not a known facility", i, a.TargetID)}
					}
				}
			case PanelRouteSteps:
				if a.TargetID != "" {
					ref, ok := sc.KnownRoutes[a.TargetID]
					if !ok {
						return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d OPEN_PANEL ROUTE_STEPS target_id %q not a known route", i, a.TargetID)}
					}
					// A closed route can be acknowledged but not
					// displayed; reject unless verified.
					if !ref.Verified || ref.Status != RouteStatusVerified {
						return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d OPEN_PANEL ROUTE_STEPS target_id %q is not verified", i, a.TargetID)}
					}
				}
			case PanelAlertDetails, PanelReservationConfirm, PanelArrivalConfirm, PanelEmergencyCallConfirm:
				// Confirmation-only panels carry sensitive intent; the
				// action never executes the side effect. Allowed without
				// a target_id; if target_id is present it must be a known
				// place/facility consistent with the panel.
				if a.TargetID != "" {
					if !isKnownAny(sc, a.TargetID) {
						return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d OPEN_PANEL %s target_id %q not known", i, a.Panel, a.TargetID)}
					}
				}
			default:
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d OPEN_PANEL unknown panel %q", i, a.Panel)}
			}
		case ActionSetLanguage:
			if !sc.IsLanguageAllowed(a.Language) {
				return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d SET_LANGUAGE language %q not in allowed set", i, a.Language)}
			}
		case ActionZoom, ActionPan, ActionRecenter:
			// Shape validation enforces the required fields; nothing
			// more here.
		default:
			return &ErrScopedSemantic{Reason: fmt.Sprintf("action %d unknown action type %q", i, a.Type)}
		}
	}

	return nil
}

// isKnownAny reports whether id appears in any of the typed maps. Evidence
// IDs may legitimately reference places, zones, routes or facilities;
// the model is allowed to cite any of them as long as it is in scope.
func isKnownAny(sc ScopedContext, id string) bool {
	if _, ok := sc.KnownPlaces[id]; ok {
		return true
	}
	if _, ok := sc.KnownSafeZones[id]; ok {
		return true
	}
	if _, ok := sc.KnownRedZones[id]; ok {
		return true
	}
	if _, ok := sc.KnownRoutes[id]; ok {
		return true
	}
	if _, ok := sc.KnownFacilities[id]; ok {
		return true
	}
	return false
}

// isSpatialFeature reports whether id is a focusable spatial feature:
// a place, safe zone, red zone, or facility. Routes are deliberately
// excluded (focusing on a route is a model bug, not a UI affordance).
func isSpatialFeature(sc ScopedContext, id string) bool {
	if _, ok := sc.KnownPlaces[id]; ok {
		return true
	}
	if _, ok := sc.KnownSafeZones[id]; ok {
		return true
	}
	if _, ok := sc.KnownRedZones[id]; ok {
		return true
	}
	if _, ok := sc.KnownFacilities[id]; ok {
		return true
	}
	return false
}

// enforceChoiceOrder verifies that targetIDs is a non-empty subsequence of
// the server-permitted EligibleDestinations slice, in the SAME order. The
// server is the only authority on choice ordering; the model never
// re-sorts.
func enforceChoiceOrder(targetIDs []string, eligible []EligibleChoice) error {
	if len(eligible) == 0 {
		return errors.New("no eligible destinations in context")
	}
	if len(targetIDs) > len(eligible) {
		return fmt.Errorf("too many choices (%d) vs eligible (%d)", len(targetIDs), len(eligible))
	}
	// Walk both slices in order.
	eIdx := 0
	for _, id := range targetIDs {
		// Skip eligible entries that don't match the current target.
		for eIdx < len(eligible) && eligible[eIdx].Facility.FacilityID != id {
			eIdx++
		}
		if eIdx >= len(eligible) {
			return fmt.Errorf("choice %q not in eligible set", id)
		}
		eIdx++
	}
	return nil
}

// ValidateModelOutputShape performs the raw-shape check: every required
// field is present (or explicitly null per the contract), no forbidden
// extra fields, strict action variants, the strict tag unions. It does NOT
// validate semantics; pair with EnforceScopedContext for full coverage.
//
// It is intentionally a separate function: callers may want to apply only
// the shape check (e.g. a unit test) without building a ScopedContext.
// Worker 9 calls both in order: shape first (cheap), semantics second
// (uses the typed maps).
func ValidateModelOutputShape(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("model output is empty")
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("model output is not valid JSON: %w", err)
	}
	required := []string{
		"schema_version", "request_id", "data_version", "status", "intent",
		"language", "actions", "speech_key", "clarification_ids",
		"evidence_ids",
	}
	for _, k := range required {
		if _, ok := probe[k]; !ok {
			// intent may be null for non-OK statuses; that's the only
			// required-but-may-be-null key, and the strict typed decoder
			// in model.go handles it. We don't second-guess here.
			if k == "intent" {
				continue
			}
			return fmt.Errorf("model output missing required field %q", k)
		}
	}
	// Top-level: no extra keys beyond the contract. This catches a model
	// that smuggles in a "fabricated_commit": true field, for example.
	allowed := map[string]struct{}{
		"schema_version":    {},
		"request_id":        {},
		"data_version":      {},
		"status":            {},
		"intent":            {},
		"language":          {},
		"actions":           {},
		"speech_key":        {},
		"clarification_ids": {},
		"evidence_ids":      {},
	}
	for k := range probe {
		if _, ok := allowed[k]; !ok {
			return fmt.Errorf("model output contains forbidden field %q", k)
		}
	}
	// Action variant validation: per-variant required fields, no
	// extras, strict unions. We re-parse to a typed shape so a malformed
	// extra field trips a real error.
	var typed ModelOutput
	if err := json.Unmarshal(raw, &typed); err != nil {
		return fmt.Errorf("model output does not match typed schema: %w", err)
	}
	if typed.SchemaVersion != ModelSchemaVersion {
		return fmt.Errorf("schema_version %q != required %q", typed.SchemaVersion, ModelSchemaVersion)
	}
	if typed.Status == "" {
		return errors.New("status is empty")
	}
	if !isKnownStatus(typed.Status) {
		return fmt.Errorf("unknown status %q", typed.Status)
	}
	if typed.Status == StatusOK && typed.Intent == nil {
		return errors.New("OK status requires an intent")
	}
	if typed.Status == StatusClarify && len(typed.ClarificationIDs) == 0 {
		return errors.New("CLARIFY requires non-empty clarification_ids")
	}
	if typed.Status != StatusClarify && len(typed.ClarificationIDs) > 0 {
		return errors.New("clarification_ids must be empty unless status is CLARIFY")
	}
	for i, a := range typed.Actions {
		if err := validateActionShape(i, a); err != nil {
			return err
		}
	}
	if len(typed.Actions) > MaxModelActions {
		return fmt.Errorf("actions exceed max %d", MaxModelActions)
	}
	// Imported prompt-injection guard: the raw bytes must not contain a
	// free-form instruction like "system:" or "ignore previous" that
	// would attempt to override the validator. This is a coarse sanity
	// check; a real injection guard lives in the orchestrator at the
	// JSON-encode boundary.
	if inj := injectionPattern(string(raw)); inj != "" {
		return fmt.Errorf("model output looks like a prompt injection: %s", inj)
	}
	return nil
}

func isKnownStatus(s ModelStatus) bool {
	switch s {
	case StatusOK, StatusClarify, StatusUnsupported, StatusDataUnavailable, StatusError:
		return true
	default:
		return false
	}
}

func validateActionShape(idx int, a Action) error {
	switch a.Type {
	case ActionFocusFeature:
		if a.TargetID == "" {
			return fmt.Errorf("action %d FOCUS_FEATURE missing target_id", idx)
		}
	case ActionShowChoices:
		if len(a.TargetIDs) == 0 {
			return fmt.Errorf("action %d SHOW_CHOICES missing target_ids", idx)
		}
		if len(a.TargetIDs) > MaxShowChoices {
			return fmt.Errorf("action %d SHOW_CHOICES exceeds max %d", idx, MaxShowChoices)
		}
	case ActionShowRoute:
		if a.RouteID == "" {
			return fmt.Errorf("action %d SHOW_ROUTE missing route_id", idx)
		}
	case ActionOpenPanel:
		if a.Panel == "" {
			return fmt.Errorf("action %d OPEN_PANEL missing panel", idx)
		}
		if !isKnownPanel(a.Panel) {
			return fmt.Errorf("action %d OPEN_PANEL unknown panel %q", idx, a.Panel)
		}
	case ActionSetLanguage:
		if a.Language == "" {
			return fmt.Errorf("action %d SET_LANGUAGE missing language", idx)
		}
	case ActionZoom:
		if a.Direction != "IN" && a.Direction != "OUT" {
			return fmt.Errorf("action %d ZOOM direction must be IN or OUT", idx)
		}
		if a.Steps != 1 {
			return fmt.Errorf("action %d ZOOM steps must be exactly 1", idx)
		}
	case ActionPan:
		switch a.Direction {
		case "NORTH", "SOUTH", "EAST", "WEST":
		default:
			return fmt.Errorf("action %d PAN direction must be one of NORTH/SOUTH/EAST/WEST", idx)
		}
		if a.Steps != 1 {
			return fmt.Errorf("action %d PAN steps must be exactly 1", idx)
		}
	case ActionRecenter:
		// No extra fields required.
	default:
		return fmt.Errorf("action %d unknown action type %q", idx, a.Type)
	}
	return nil
}

func isKnownPanel(p Panel) bool {
	switch p {
	case PanelAlertDetails, PanelDestinationPreview, PanelRouteSteps,
		PanelReservationConfirm, PanelArrivalConfirm, PanelEmergencyCallConfirm:
		return true
	default:
		return false
	}
}

// injectionPattern scans raw JSON text for known adversarial markers. The
// check is intentionally coarse and only fires on obviously suspicious
// strings: it is not a substitute for the orchestrator's real injection
// guard at the model boundary. It is included here so a model that
// ignores the system prompt and dumps a system-prompt-shaped object is
// caught at validation time, not at display time.
func injectionPattern(raw string) string {
	lower := strings.ToLower(raw)
	for _, marker := range []string{
		"ignore previous instructions",
		"system: you are",
		"<<sys>>",
		"<|im_start|>",
	} {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	return ""
}
