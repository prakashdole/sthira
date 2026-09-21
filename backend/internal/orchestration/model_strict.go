package orchestration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"sthira/backend/internal/contracts"
)

// Strict middle-model response validation on the RAW bytes.
//
// Typed decoding into contracts.ModelOutput cannot distinguish an
// absent optional field from an explicit zero/empty one (`""`, `0`,
// `[]`, `null`). The contract forbids that class of ambiguity for
// model proposals: a RECENTER action that explicitly carries
// `"target_id": ""` is the model claiming intent it cannot express,
// and must be rejected before the value is lost through the typed
// decode. This file implements the presence-level half of the
// contract; validateProposal/EnforceScopedContext keep the typed
// value-level half (enums, correlation, known IDs).

// middleResponseTopKeys are the exact keys of
// contracts.MiddleWorkerResponse (finish_reason optional).
var middleResponseTopKeys = map[string]bool{
	"request_id": true, "data_version": true, "proposal": true,
	"finish_reason": true, "model_revision": true,
}

// proposalTopKeys are the exact keys of contracts.ModelOutput.
// Required (must be present, value may be null where the type
// allows it): schema_version, request_id, data_version, status,
// language, actions.
var proposalRequiredKeys = map[string]bool{
	"schema_version": true, "request_id": true, "data_version": true,
	"status": true, "language": true, "actions": true,
}

var proposalTopKeys = map[string]bool{
	"schema_version": true, "request_id": true, "data_version": true,
	"status": true, "intent": true, "language": true, "actions": true,
	"speech_key": true, "clarification_ids": true, "evidence_ids": true,
}

// actionAllowedKeys maps the action discriminator to the exact set
// of JSON keys legal for that variant. `type` is always required.
var actionAllowedKeys = map[contracts.ActionType]map[string]bool{
	contracts.ActionFocusFeature: {"type": true, "target_id": true},
	contracts.ActionShowChoices:  {"type": true, "target_ids": true},
	contracts.ActionShowRoute:    {"type": true, "route_id": true},
	contracts.ActionOpenPanel:    {"type": true, "panel": true, "target_id": true},
	contracts.ActionZoom:         {"type": true, "direction": true, "steps": true},
	contracts.ActionPan:          {"type": true, "direction": true, "steps": true},
	contracts.ActionRecenter:     {"type": true},
	contracts.ActionSetLanguage:  {"type": true, "language": true},
}

// validateModelResponseRaw checks the raw middle-worker response
// bytes against the strict contract shape before typed decoding is
// relied upon. maxProposalBytes is the model-response ceiling
// (distinct from the public request budget and the worker envelope
// size limit).
func validateModelResponseRaw(raw []byte, maxProposalBytes int) error {
	if maxProposalBytes <= 0 {
		maxProposalBytes = DefaultLimits().MaxRawModelBytes
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("model response is not a JSON object: %w", err)
	}
	for k := range envelope {
		if !middleResponseTopKeys[k] {
			return fmt.Errorf("model response: unknown field %q", k)
		}
	}
	for _, k := range []string{"request_id", "data_version", "proposal", "model_revision"} {
		if _, ok := envelope[k]; !ok {
			return fmt.Errorf("model response: missing required field %q", k)
		}
	}
	proposalBS := envelope["proposal"]
	if len(proposalBS) > maxProposalBytes {
		return fmt.Errorf("model proposal exceeds %d bytes (got %d)", maxProposalBytes, len(proposalBS))
	}
	return validateProposalRaw(proposalBS)
}

func validateProposalRaw(raw []byte) error {
	var proposal map[string]json.RawMessage
	if err := json.Unmarshal(raw, &proposal); err != nil {
		return fmt.Errorf("model proposal is not a JSON object: %w", err)
	}
	for k := range proposal {
		if !proposalTopKeys[k] {
			return fmt.Errorf("model proposal: unknown field %q", k)
		}
	}
	for k := range proposalRequiredKeys {
		if _, ok := proposal[k]; !ok {
			return fmt.Errorf("model proposal: missing required field %q", k)
		}
	}
	var status contracts.ModelStatus
	if err := json.Unmarshal(proposal["status"], &status); err != nil {
		return fmt.Errorf("model proposal: status is not a string: %w", err)
	}
	switch status {
	case contracts.StatusOK, contracts.StatusClarify, contracts.StatusUnsupported,
		contracts.StatusDataUnavailable, contracts.StatusError:
	default:
		return fmt.Errorf("model proposal: unknown status %q", string(status))
	}
	intent, hasIntent := proposal["intent"]
	if hasIntent && !bytes.Equal(bytes.TrimSpace(intent), []byte("null")) && status != contracts.StatusOK {
		return errors.New("model proposal: non-OK status carries a non-null intent")
	}
	if !hasIntent && status == contracts.StatusOK {
		return errors.New("model proposal: OK status requires an intent")
	}
	var actions []map[string]json.RawMessage
	if err := json.Unmarshal(proposal["actions"], &actions); err != nil {
		return fmt.Errorf("model proposal: actions is not an array: %w", err)
	}
	if len(actions) > contracts.MaxModelActions {
		return fmt.Errorf("model proposal: actions exceed max %d", len(actions))
	}
	for i, a := range actions {
		if err := validateActionRaw(i, a); err != nil {
			return err
		}
	}
	return nil
}

func validateActionRaw(i int, a map[string]json.RawMessage) error {
	typeBS, ok := a["type"]
	if !ok {
		return fmt.Errorf("model proposal: action %d missing type", i)
	}
	var t contracts.ActionType
	if err := json.Unmarshal(typeBS, &t); err != nil {
		return fmt.Errorf("model proposal: action %d type is not a string: %w", i, err)
	}
	allowed, known := actionAllowedKeys[t]
	if !known {
		return fmt.Errorf("model proposal: action %d unknown type %q", i, string(t))
	}
	for k := range a {
		if !allowed[k] {
			return fmt.Errorf("model proposal: action %d (%s) carries forbidden field %q", i, string(t), k)
		}
	}
	return nil
}
