package provider

import (
	"encoding/base64"
)

// stdBase64Decode wraps encoding/base64 with strict semantics so
// downstream code doesn't accidentally accept URL-safe variants.
func stdBase64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
