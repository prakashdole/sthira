package middleworker

import _ "embed"

//go:embed schema/model_output.schema.json
var defaultModelOutputSchema []byte

// DefaultModelOutputSchema returns the pinned JSON Schema for guided generation.
func DefaultModelOutputSchema() []byte {
	return append([]byte(nil), defaultModelOutputSchema...)
}
