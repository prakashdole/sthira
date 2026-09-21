package contracts

import "fmt"

// Middle-model output contract (STHIRA_INTERFACE_CONTROLLER_V3). The model
// proposes display actions only; it can never decide safety, originate routes,
// reserve places, confirm arrival, dispatch help or obtain credentials. Go
// independently validates every field against the current request, source
// snapshot, jurisdiction and known IDs. A non-OK status carries no actions.

// ModelSchemaVersion is the literal the middle model must echo.
const ModelSchemaVersion = "3.0"

// ModelStatus is the top-level outcome of a middle-model proposal.
type ModelStatus string

const (
	StatusOK              ModelStatus = "OK"
	StatusClarify         ModelStatus = "CLARIFY"
	StatusUnsupported     ModelStatus = "UNSUPPORTED"
	StatusDataUnavailable ModelStatus = "DATA_UNAVAILABLE"
	StatusError           ModelStatus = "ERROR"
)

// Intent is the allowed middle-model intent enum.
type Intent string

const (
	IntentFocusPlace         Intent = "FOCUS_PLACE"
	IntentShowAlertArea      Intent = "SHOW_ALERT_AREA"
	IntentListDestinations   Intent = "LIST_DESTINATIONS"
	IntentPreviewDestination Intent = "PREVIEW_DESTINATION"
	IntentShowRoute          Intent = "SHOW_ROUTE"
	IntentShowMyLocation     Intent = "SHOW_MY_LOCATION"
	IntentZoom               Intent = "ZOOM"
	IntentPan                Intent = "PAN"
	IntentRecenter           Intent = "RECENTER"
	IntentRepeatGuidance     Intent = "REPEAT_GUIDANCE"
	IntentChangeLanguage     Intent = "CHANGE_LANGUAGE"
	IntentOpenConfirmation   Intent = "OPEN_CONFIRMATION"
)

var validIntents = map[Intent]bool{
	IntentFocusPlace: true, IntentShowAlertArea: true, IntentListDestinations: true,
	IntentPreviewDestination: true, IntentShowRoute: true, IntentShowMyLocation: true,
	IntentZoom: true, IntentPan: true, IntentRecenter: true, IntentRepeatGuidance: true,
	IntentChangeLanguage: true, IntentOpenConfirmation: true,
}

// ActionType is the strict tagged action variant discriminator.
type ActionType string

const (
	ActionFocusFeature ActionType = "FOCUS_FEATURE"
	ActionShowChoices  ActionType = "SHOW_CHOICES"
	ActionShowRoute    ActionType = "SHOW_ROUTE"
	ActionOpenPanel    ActionType = "OPEN_PANEL"
	ActionZoom         ActionType = "ZOOM"
	ActionPan          ActionType = "PAN"
	ActionRecenter     ActionType = "RECENTER"
	ActionSetLanguage  ActionType = "SET_LANGUAGE"
)

// Panel is the allowed confirmation/detail panel enum.
type Panel string

const (
	PanelAlertDetails         Panel = "ALERT_DETAILS"
	PanelDestinationPreview   Panel = "DESTINATION_PREVIEW"
	PanelRouteSteps           Panel = "ROUTE_STEPS"
	PanelReservationConfirm   Panel = "RESERVATION_CONFIRMATION"
	PanelArrivalConfirm       Panel = "ARRIVAL_CONFIRMATION"
	PanelEmergencyCallConfirm Panel = "EMERGENCY_CALL_CONFIRMATION"
)

var validPanels = map[Panel]bool{
	PanelAlertDetails: true, PanelDestinationPreview: true, PanelRouteSteps: true,
	PanelReservationConfirm: true, PanelArrivalConfirm: true, PanelEmergencyCallConfirm: true,
}

var validActionTypes = map[ActionType]bool{
	ActionFocusFeature: true, ActionShowChoices: true, ActionShowRoute: true,
	ActionOpenPanel: true, ActionZoom: true, ActionPan: true,
	ActionRecenter: true, ActionSetLanguage: true,
}

// IsValidIntent reports whether i is an allowed middle-model intent.
func IsValidIntent(i Intent) bool {
	return validIntents[i]
}

// IsValidPanel reports whether p is an allowed confirmation/detail panel.
func IsValidPanel(p Panel) bool {
	return validPanels[p]
}

// IsValidActionType reports whether a is an allowed action variant.
func IsValidActionType(a ActionType) bool {
	return validActionTypes[a]
}

const (
	// MaxModelActions bounds the action array.
	MaxModelActions = 5
	// MaxShowChoices bounds SHOW_CHOICES target IDs (server-permitted order).
	MaxShowChoices = 3
	// MaxClarificationIDs bounds clarification candidates per page.
	MaxClarificationIDs = 3
)

// Action is one strict tagged action. Only the fields legal for its Type may
// be set; the validator enforces the per-variant shape and forbids extras.
type Action struct {
	Type      ActionType `json:"type"`
	TargetID  string     `json:"target_id,omitempty"`
	TargetIDs []string   `json:"target_ids,omitempty"`
	RouteID   string     `json:"route_id,omitempty"`
	Panel     Panel      `json:"panel,omitempty"`
	Direction string     `json:"direction,omitempty"`
	Steps     int        `json:"steps,omitempty"`
	Language  string     `json:"language,omitempty"`
}

// ModelOutput is the exact top-level middle-model output object.
type ModelOutput struct {
	SchemaVersion    string      `json:"schema_version"`
	RequestID        string      `json:"request_id"`
	DataVersion      string      `json:"data_version"`
	Status           ModelStatus `json:"status"`
	Intent           *Intent     `json:"intent"` // null for non-OK status
	Language         string      `json:"language"`
	Actions          []Action    `json:"actions"`
	SpeechKey        *string     `json:"speech_key"` // approved template key or null
	ClarificationIDs []string    `json:"clarification_ids"`
	EvidenceIDs      []string    `json:"evidence_ids"`
}

// knownIDs is the set of IDs the validator may reference, supplied by the
// trusted context (never by the model).
type knownIDs map[string]bool

// ValidateModelOutput independently validates a middle-model proposal against
// the current request and the trusted context. It returns the first violation.
// requestID and dataVersion are the server's current values; known is the set
// of IDs present in the active context; enabledLanguages is the enabled set.
func ValidateModelOutput(out ModelOutput, requestID, dataVersion string, known map[string]bool, enabledLanguages map[string]bool) error {
	if out.SchemaVersion != ModelSchemaVersion {
		return fmt.Errorf("schema_version must be %q", ModelSchemaVersion)
	}
	if out.RequestID != requestID {
		return fmt.Errorf("request_id must echo the current server request")
	}
	if out.DataVersion != dataVersion {
		return fmt.Errorf("data_version must echo the current context snapshot")
	}
	if !enabledLanguages[out.Language] {
		return fmt.Errorf("language %q is not an enabled context language", out.Language)
	}

	ok := out.Status == StatusOK
	if !ok {
		// Non-OK status: no intent, no actions.
		if out.Intent != nil {
			return fmt.Errorf("non-OK status must have null intent")
		}
		if len(out.Actions) != 0 {
			return fmt.Errorf("non-OK status must have no actions")
		}
	} else {
		if out.Intent == nil {
			return fmt.Errorf("OK status requires an intent")
		}
		if !validIntents[*out.Intent] {
			return fmt.Errorf("unknown intent %q", *out.Intent)
		}
	}

	if out.Status == StatusClarify {
		if len(out.ClarificationIDs) == 0 {
			return fmt.Errorf("CLARIFY requires clarification_ids")
		}
	} else if len(out.ClarificationIDs) != 0 {
		return fmt.Errorf("clarification_ids must be empty unless status is CLARIFY")
	}
	if len(out.ClarificationIDs) > MaxClarificationIDs {
		return fmt.Errorf("clarification_ids exceeds max %d", MaxClarificationIDs)
	}

	if len(out.Actions) > MaxModelActions {
		return fmt.Errorf("actions exceed max %d", MaxModelActions)
	}
	for i, a := range out.Actions {
		if err := validateAction(a, known); err != nil {
			return fmt.Errorf("action %d: %w", i, err)
		}
	}

	for _, id := range out.ClarificationIDs {
		if !known[id] {
			return fmt.Errorf("clarification id %q not in active context", id)
		}
	}
	for _, id := range out.EvidenceIDs {
		if !known[id] {
			return fmt.Errorf("evidence id %q not in active context", id)
		}
	}
	return nil
}

func validateAction(a Action, known knownIDs) error {
	switch a.Type {
	case ActionFocusFeature:
		return requireKnownID("target_id", a.TargetID, known)
	case ActionShowChoices:
		if len(a.TargetIDs) == 0 || len(a.TargetIDs) > MaxShowChoices {
			return fmt.Errorf("SHOW_CHOICES requires 1..%d target_ids", MaxShowChoices)
		}
		for _, id := range a.TargetIDs {
			if err := requireKnownID("target_ids", id, known); err != nil {
				return err
			}
		}
		return nil
	case ActionShowRoute:
		return requireKnownID("route_id", a.RouteID, known)
	case ActionOpenPanel:
		if !validPanels[a.Panel] {
			return fmt.Errorf("unknown panel %q", a.Panel)
		}
		if a.TargetID != "" {
			return requireKnownID("target_id", a.TargetID, known)
		}
		return nil
	case ActionZoom:
		if a.Direction != "IN" && a.Direction != "OUT" {
			return fmt.Errorf("ZOOM direction must be IN or OUT")
		}
		return requireSteps(a.Steps)
	case ActionPan:
		switch a.Direction {
		case "NORTH", "SOUTH", "EAST", "WEST":
		default:
			return fmt.Errorf("PAN direction must be NORTH/SOUTH/EAST/WEST")
		}
		return requireSteps(a.Steps)
	case ActionRecenter:
		return nil
	case ActionSetLanguage:
		if a.Language == "" {
			return fmt.Errorf("SET_LANGUAGE requires language")
		}
		return nil
	default:
		return fmt.Errorf("unknown action type %q", a.Type)
	}
}

func requireKnownID(field, id string, known knownIDs) error {
	if id == "" {
		return fmt.Errorf("%s is required", field)
	}
	if !known[id] {
		return fmt.Errorf("%s %q not in active context", field, id)
	}
	return nil
}

func requireSteps(steps int) error {
	if steps != 1 {
		return fmt.Errorf("steps must be exactly 1")
	}
	return nil
}
