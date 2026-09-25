package offlinepkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Limits bounds input data sizes and nesting depths to prevent resource exhaustion.
type Limits struct {
	MaxBytes int64
	MaxDepth int
}

// DefaultLimits provides conservative defaults (1 MiB max size, 32 nesting levels).
var DefaultLimits = Limits{
	MaxBytes: 1 << 20,
	MaxDepth: 32,
}

// ParseManifest strictly decodes and validates a JSON Manifest against limits and schema.
func ParseManifest(data []byte, limits Limits) (*Manifest, error) {
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = DefaultLimits.MaxBytes
	}
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = DefaultLimits.MaxDepth
	}

	if int64(len(data)) > limits.MaxBytes {
		return nil, fmt.Errorf("%w: input size %d exceeds limit %d", ErrMalformedData, len(data), limits.MaxBytes)
	}

	if err := checkWellFormed(data, limits.MaxDepth); err != nil {
		return nil, err
	}

	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%w: json decode: %v", ErrMalformedData, err)
	}
	if err := ensureEOF(dec); err != nil {
		return nil, err
	}

	if err := ValidateManifestStructure(&m); err != nil {
		return nil, err
	}

	return &m, nil
}

// ParseCard strictly decodes and validates a JSON PublicIncidentCard against limits and schema.
func ParseCard(data []byte, limits Limits) (*PublicIncidentCard, error) {
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = DefaultLimits.MaxBytes
	}
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = DefaultLimits.MaxDepth
	}

	if int64(len(data)) > limits.MaxBytes {
		return nil, fmt.Errorf("%w: input size %d exceeds limit %d", ErrMalformedData, len(data), limits.MaxBytes)
	}

	if err := checkWellFormed(data, limits.MaxDepth); err != nil {
		return nil, err
	}

	var c PublicIncidentCard
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%w: json decode: %v", ErrMalformedData, err)
	}
	if err := ensureEOF(dec); err != nil {
		return nil, err
	}

	if err := ValidateCardStructure(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

func checkWellFormed(data []byte, maxDepth int) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("%w: empty input", ErrMalformedData)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	return walkValue(dec, 0, maxDepth)
}

func walkValue(dec *json.Decoder, depth, maxDepth int) error {
	if maxDepth > 0 && depth > maxDepth {
		return fmt.Errorf("%w: depth %d exceeds maximum depth %d", ErrMalformedData, depth, maxDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: truncated json", ErrMalformedData)
		}
		return fmt.Errorf("%w: token error: %v", ErrMalformedData, err)
	}

	switch delim := tok.(type) {
	case json.Delim:
		switch delim {
		case '{':
			return walkObject(dec, depth+1, maxDepth)
		case '[':
			return walkArray(dec, depth+1, maxDepth)
		}
	}
	return nil
}

func walkObject(dec *json.Decoder, depth, maxDepth int) error {
	seen := make(map[string]struct{})
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("%w: object key read: %v", ErrMalformedData, err)
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("%w: non-string object key", ErrMalformedData)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: duplicate key %q", ErrMalformedData, key)
		}
		seen[key] = struct{}{}

		if err := walkValue(dec, depth, maxDepth); err != nil {
			return err
		}
	}
	// Consume closing '}'
	_, err := dec.Token()
	return err
}

func walkArray(dec *json.Decoder, depth, maxDepth int) error {
	for dec.More() {
		if err := walkValue(dec, depth, maxDepth); err != nil {
			return err
		}
	}
	// Consume closing ']'
	_, err := dec.Token()
	return err
}

func ensureEOF(dec *json.Decoder) error {
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%w: trailing data after top-level value", ErrMalformedData)
		}
		return fmt.Errorf("%w: check EOF: %v", ErrMalformedData, err)
	}
	return nil
}
