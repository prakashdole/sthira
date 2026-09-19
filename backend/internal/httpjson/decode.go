// Package httpjson implements the strict JSON boundary for /api/v3.
//
// encoding/json alone does not reject duplicate object keys, does not bound
// nesting depth, and silently ignores unknown fields unless asked. This
// package enforces, in order: size limit, single value with no trailing data,
// depth limit, no duplicate keys, and (optionally) no unknown fields against a
// target struct. All failures map to stable contract error codes.
package httpjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"sthira/backend/internal/contracts"
)

// Limits bound an inbound JSON document.
type Limits struct {
	// MaxBytes is the maximum raw body size in bytes.
	MaxBytes int64
	// MaxDepth is the maximum nesting depth of objects/arrays.
	MaxDepth int
}

// DefaultLimits are the conservative boundary defaults.
var DefaultLimits = Limits{MaxBytes: 1 << 20, MaxDepth: 32}

// FieldError describes a single validation failure with a stable code.
type FieldError struct {
	Code    string
	Message string
	Field   string
}

func (e *FieldError) Error() string { return e.Message }

// DecodeStrict reads body (already size-bounded by the caller via the server)
// and strictly decodes it into v. It enforces single-value JSON, no trailing
// data, depth and duplicate-key limits, and rejects unknown fields when v is a
// struct. It returns a *FieldError with a stable contract code on failure.
func DecodeStrict(body []byte, v any, limits Limits) error {
	if limits.MaxBytes > 0 && int64(len(body)) > limits.MaxBytes {
		return &FieldError{Code: contracts.ErrBodyTooLarge, Message: "request body exceeds size limit"}
	}
	if err := checkWellFormed(body, limits); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return decodeError(err)
	}
	// A second token means trailing data after the single JSON value.
	if err := ensureEOF(dec); err != nil {
		return err
	}
	return nil
}

// checkWellFormed validates depth and duplicate keys without decoding into a
// target, so the checks apply uniformly before any typed decoding.
func checkWellFormed(body []byte, limits Limits) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	return walkValue(dec, 0, limits)
}

// walkValue consumes one JSON value stream, tracking depth and duplicate keys.
func walkValue(dec *json.Decoder, depth int, limits Limits) error {
	if limits.MaxDepth > 0 && depth > limits.MaxDepth {
		return &FieldError{Code: contracts.ErrDepthExceeded, Message: "JSON nesting exceeds depth limit"}
	}
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return &FieldError{Code: contracts.ErrMalformedJSON, Message: "empty or truncated JSON"}
		}
		return &FieldError{Code: contracts.ErrMalformedJSON, Message: "malformed JSON: " + err.Error()}
	}
	switch delim := tok.(type) {
	case json.Delim:
		switch delim {
		case '{':
			return walkObject(dec, depth, limits)
		case '[':
			return walkArray(dec, depth, limits)
		}
	}
	return nil
}

func walkObject(dec *json.Decoder, depth int, limits Limits) error {
	seen := map[string]struct{}{}
	for dec.More() {
		tok, err := dec.Token() // key
		if err != nil {
			return &FieldError{Code: contracts.ErrMalformedJSON, Message: "malformed JSON object key"}
		}
		key, ok := tok.(string)
		if !ok {
			return &FieldError{Code: contracts.ErrMalformedJSON, Message: "object key is not a string"}
		}
		if _, dup := seen[key]; dup {
			return &FieldError{Code: contracts.ErrDuplicateKey, Message: "duplicate JSON object key", Field: key}
		}
		seen[key] = struct{}{}
		if err := walkValue(dec, depth+1, limits); err != nil {
			return err
		}
	}
	_, err := dec.Token() // closing '}'
	if err != nil {
		return &FieldError{Code: contracts.ErrMalformedJSON, Message: "malformed JSON object"}
	}
	return nil
}

func walkArray(dec *json.Decoder, depth int, limits Limits) error {
	for dec.More() {
		if err := walkValue(dec, depth+1, limits); err != nil {
			return err
		}
	}
	_, err := dec.Token() // closing ']'
	if err != nil {
		return &FieldError{Code: contracts.ErrMalformedJSON, Message: "malformed JSON array"}
	}
	return nil
}

func ensureEOF(dec *json.Decoder) error {
	if _, err := dec.Token(); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return &FieldError{Code: contracts.ErrMalformedJSON, Message: "malformed JSON: " + err.Error()}
	}
	return &FieldError{Code: contracts.ErrTrailingData, Message: "unexpected data after JSON value"}
}

// decodeError maps a typed-decode failure to a stable contract code.
func decodeError(err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		return &FieldError{Code: contracts.ErrMalformedJSON, Message: "malformed JSON: " + err.Error()}
	case errors.As(err, &typeErr):
		return &FieldError{Code: contracts.ErrInvalidValue, Message: "invalid value: " + err.Error(), Field: typeErr.Field}
	}
	// DisallowUnknownFields surfaces as "unknown field ..." errors.
	var unknownField = "json: unknown field "
	if msg := err.Error(); len(msg) >= len(unknownField) && msg[:len(unknownField)] == unknownField {
		return &FieldError{Code: contracts.ErrUnknownField, Message: msg, Field: msg[len(unknownField):]}
	}
	if errors.Is(err, io.EOF) {
		return &FieldError{Code: contracts.ErrMalformedJSON, Message: "empty request body"}
	}
	return &FieldError{Code: contracts.ErrMalformedJSON, Message: err.Error()}
}

// Marshal encodes v for a response. Responses are server-built and trusted, so
// standard encoding is sufficient.
func Marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}
	return b, nil
}
