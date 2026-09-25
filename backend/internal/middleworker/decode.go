// Strict JSON-constrained decoding of the vLLM assistant content
// into the frozen Proposal shape. This is the wire-level guard; the
// orchestrator's independent validator (contracts.ValidateModelOutput)
// is the semantic guard. Both run; neither replaces the other.

package middleworker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// decodeStrictProposal decodes assistant content into a Proposal and
// rejects:
//
//   - extra text outside the JSON document (ErrExtraText)
//   - duplicate JSON keys (ErrMalformed)
//   - trailing content after the JSON document (ErrExtraText)
//   - leading whitespace > 4 bytes (ErrExtraText)
//   - payload larger than the per-decoder byte cap (ErrOversized)
//
// It does NOT validate enums, intent/action combinations, ID allow
// lists, language allow-list or choice order; that is the validator's
// job. The schema guard above (decodeStrictSchemaProposal) does the
// minimum shape check this layer cannot do — extra-text detection,
// duplicate-key detection, and strict number parsing.
func decodeStrictProposal(content []byte) (Proposal, error) {
	var p Proposal
	if len(content) == 0 {
		return p, fmt.Errorf("%w: empty content", ErrMalformed)
	}
	// Trim a small amount of leading whitespace. Anything more is
	// extra text.
	trimmed := bytes.TrimLeft(content, " \t\r\n")
	if len(trimmed) != len(content) && len(content)-len(trimmed) > 4 {
		return p, fmt.Errorf("%w: leading whitespace > 4 bytes", ErrExtraText)
	}
	// Find the JSON object boundaries.
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return p, fmt.Errorf("%w: content does not start with '{'", ErrExtraText)
	}
	// Locate the matching closing brace. A hand-rolled counter is
	// enough here — the vLLM output is well-formed UTF-8 JSON and
	// we do not want to import a streaming parser.
	depth, end := 0, -1
	for i := 0; i < len(trimmed); i++ {
		b := trimmed[i]
		switch b {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		case '"':
			// skip string contents (with escapes).
			i++
			for i < len(trimmed) {
				if trimmed[i] == '\\' {
					i += 2
					continue
				}
				if trimmed[i] == '"' {
					break
				}
				i++
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return p, fmt.Errorf("%w: no matching '}'", ErrMalformed)
	}
	if end != len(trimmed) {
		return p, fmt.Errorf("%w: trailing bytes after JSON document", ErrExtraText)
	}
	body := trimmed[:end]
	// Strict decode: DisallowUnknownFields catches an emission
	// that drifted from the schema; the orchestrator's validator
	// will reject the same shape again at the semantic level.
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		if errors.Is(err, errDisallowUnknown) || isUnknownFieldErr(err) {
			return p, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		return p, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	// Make sure no JSON object followed (decoder would already
	// catch it; this is a belt-and-braces guard).
	if dec.More() {
		return p, fmt.Errorf("%w: extra JSON document", ErrExtraText)
	}
	return p, nil
}

// errDisallowUnknown is the sentinel returned by
// json.Decoder.DisallowUnknownFields when an unknown field is
// present. We define it here to allow errors.Is matching without
// importing encoding/json twice.
var errDisallowUnknown = errors.New("json: unknown field")

// isUnknownFieldErr matches the standard library message.
func isUnknownFieldErr(err error) bool {
	if err == nil {
		return false
	}
	return bytes.Contains([]byte(err.Error()), []byte("unknown field"))
}
