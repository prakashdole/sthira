package httpjson

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"sthira/backend/internal/contracts"
)

// These tests pin the security boundary the orchestrator relies on:
// oversize bodies, depth overflows, duplicate keys, hostile encoding
// (BOM / null / unknown / double-UTF), trailing data, and a single
// leading-value probe. Each test asserts a stable error code at the
// /api/v3 boundary; the runner here is the security-surface witness
// for backend/security/threat-model.md.

type probe struct {
	RequestID string `json:"request_id"`
}

func TestOversizeBodyRejectedAtBoundary(t *testing.T) {
	body := bytes.Repeat([]byte("a"), int(DefaultLimits.MaxBytes)+1)
	var v probe
	err := DecodeStrict(body, &v, DefaultLimits)
	if err == nil {
		t.Fatalf("oversize body accepted; F-OUTGOINGEGRESS-class vulnerability")
	}
	if fe, ok := err.(*FieldError); !ok || fe.Code != contracts.ErrBodyTooLarge {
		t.Fatalf("expected ErrBodyTooLarge, got %T %v", err, err)
	}
}

func TestDepthOverflowRejected(t *testing.T) {
	// Build nested JSON to depth = MaxDepth + 1.
	var buf strings.Builder
	buf.WriteString(`{"k":`)
	for i := 0; i < int(DefaultLimits.MaxDepth)+1; i++ {
		buf.WriteString(`{"k":`)
	}
	buf.WriteString(`"v"`)
	for i := 0; i < int(DefaultLimits.MaxDepth)+2; i++ {
		buf.WriteString(`}`)
	}
	var v probe
	err := DecodeStrict([]byte(buf.String()), &v, DefaultLimits)
	if err == nil {
		t.Fatalf("depth overflow accepted")
	}
	if fe, ok := err.(*FieldError); !ok || fe.Code != contracts.ErrDepthExceeded {
		t.Fatalf("expected ErrDepthExceeded, got %T %v", err, err)
	}
}

func TestDuplicateKeyRejected(t *testing.T) {
	body := []byte(`{"request_id":"R1","request_id":"R2"}`)
	var v probe
	err := DecodeStrict(body, &v, DefaultLimits)
	if err == nil {
		t.Fatalf("duplicate key accepted; canonical-id collision possible")
	}
	if fe, ok := err.(*FieldError); !ok || fe.Code != contracts.ErrDuplicateKey {
		t.Fatalf("expected ErrDuplicateKey, got %T %v", err, err)
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	body := []byte(`{"request_id":"R1","invented_field":"x"}`)
	var v probe
	err := DecodeStrict(body, &v, DefaultLimits)
	if err == nil {
		t.Fatalf("unknown field accepted; contract drift hidden")
	}
	if fe, ok := err.(*FieldError); !ok || fe.Code != contracts.ErrUnknownField {
		t.Fatalf("expected ErrUnknownField, got %T %v", err, err)
	}
}

func TestTrailingDataRejected(t *testing.T) {
	body := []byte(`{"request_id":"R1"} {"x":1}`)
	var v probe
	err := DecodeStrict(body, &v, DefaultLimits)
	if err == nil {
		t.Fatalf("trailing data accepted; cross-doc smuggling possible")
	}
	if fe, ok := err.(*FieldError); !ok || fe.Code != contracts.ErrTrailingData {
		t.Fatalf("expected ErrTrailingData, got %T %v", err, err)
	}
}

func TestEmptyBodyRejected(t *testing.T) {
	var v probe
	err := DecodeStrict([]byte(""), &v, DefaultLimits)
	if err == nil {
		t.Fatalf("empty body accepted")
	}
	if fe, ok := err.(*FieldError); !ok || fe.Code != contracts.ErrMalformedJSON {
		t.Fatalf("expected ErrMalformedJSON, got %T %v", err, err)
	}
}

// TestHostileEncodingsRejected confirms the decoder does not
// short-circuit on inputs that JSON libraries historically
// mis-handle: BOM, NUL, lone UTF-16 LE/BE.
func TestHostileEncodingsRejected(t *testing.T) {
	cases := map[string][]byte{
		"BOM UTF-8":        {0xEF, 0xBB, 0xBF, '{', '}', 0},
		"lone NUL":         {'{', 0, '}'},
		"non-JSON garbage": {0xDE, 0xAD, 0xBE, 0xEF},
		"leading garbage":  []byte("xx{}"),
		"UTF-16 LE BOM":    {0xFF, 0xFE, '{', '}', 0},
		"UTF-16 BE BOM":    {0xFE, 0xFF, 0, '{', 0, '}', 0, 0},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			var v probe
			err := DecodeStrict(body, &v, DefaultLimits)
			if err == nil {
				t.Fatalf("hostile encoding accepted: %s", name)
			}
			if fe, ok := err.(*FieldError); !ok ||
				(fe.Code != contracts.ErrMalformedJSON && fe.Code != contracts.ErrTrailingData) {
				t.Fatalf("expected ErrMalformedJSON or ErrTrailingData, got %v", err)
			}
		})
	}
}

// TestShapeProbeIsValid (positive control): a well-formed, in-shape,
// single-key JSON body decodes cleanly.
func TestShapeProbeIsValid(t *testing.T) {
	var v probe
	if err := DecodeStrict([]byte(`{"request_id":"OK"}`), &v, DefaultLimits); err != nil {
		t.Fatalf("clean body rejected: %v", err)
	}
	if v.RequestID != "OK" {
		t.Fatalf("clean body decoded wrong: %+v", v)
	}
	// Sanity: encoding/json's standard library still parses the
	// same body in non-strict mode; the strict decoder is layered
	// on top, not a replacement.
	var w probe
	if err := json.Unmarshal([]byte(`{"request_id":"OK"}`), &w); err != nil {
		t.Fatalf("stdlib json failed: %v", err)
	}
}
