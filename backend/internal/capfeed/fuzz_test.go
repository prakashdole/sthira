package capfeed

import (
	"testing"
	"time"
)

// FuzzParse ensures the bounded parser never panics and always returns either a
// valid Parsed or a *CAPError (never a bare panic / hang) on arbitrary bytes.
// It is bounded by MaxXMLBytes and the injected clock.
func FuzzParse(f *testing.F) {
	seeds := []string{
		baseFields().xml(),
		"",
		"<alert>",
		"<?xml version=\"1.0\"?><!DOCTYPE alert><alert/>",
		"<alert><info><polygon>1,2</polygon></info></alert>",
		"\x00\x01\x02 not xml",
		"<alert xmlns=\"urn:oasis:names:tc:emergency:cap:1.2\"><identifier>A</identifier></alert>",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		opts := ParseOptions{SourceURI: "fixture:fuzz", RetrievedAt: time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)}
		p, err := Parse([]byte(in), opts)
		if err != nil {
			// Must be a safe CAPError, never an unexpected panic path.
			if _, ok := err.(*CAPError); !ok {
				t.Fatalf("non-CAPError returned: %T %v", err, err)
			}
			return
		}
		// A successful parse must satisfy its own invariants.
		if p.Alert.Identifier == "" {
			t.Fatal("parsed alert with empty identifier")
		}
		if !p.Alert.Expires.After(p.Alert.Effective) {
			t.Fatal("parsed alert with expires <= effective")
		}
		if len(p.SHA256) != 64 {
			t.Fatal("parsed alert missing digest")
		}
	})
}
