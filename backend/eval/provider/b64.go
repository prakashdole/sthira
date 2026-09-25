package provider

import (
	"encoding/base64"
	"errors"
	"io"
)

// stdBase64Decode decodes a base64 string using only stdlib.
// Wraps encoding/base64 with strict semantics so downstream code
// doesn't accidentally accept URL-safe variants.
func stdBase64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// errOversized is returned when the bounded reader overflows the
// configured cap.
var errOversized = errors.New("provider: response body exceeded configured byte cap")

// ioLimitReaderRead returns up to max bytes from r. Returns
// errOversized if the body is larger than max. Accepts any
// io.Reader; the http.Response.Body case is the canonical caller.
func ioLimitReaderRead(r interface{ Read(p []byte) (int, error) }, max int64) ([]byte, error) {
	if rr, ok := r.(io.Reader); ok {
		buf, err := io.ReadAll(io.LimitReader(rr, max+1))
		if err != nil {
			return nil, err
		}
		if int64(len(buf)) > max {
			return nil, errOversized
		}
		return buf, nil
	}
	return nil, errOversized
}
