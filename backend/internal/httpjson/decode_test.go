package httpjson

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sthira/backend/internal/contracts"
)

type simpleBody struct {
	RequestID   string `json:"request_id"`
	DataVersion string `json:"data_version"`
}

func limits() Limits { return Limits{MaxBytes: 1 << 20, MaxDepth: 8} }

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *FieldError, got %T (%v)", err, err)
	}
	return fe.Code
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../testdata/json", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestDecodeStrictValid(t *testing.T) {
	var v simpleBody
	if err := DecodeStrict([]byte(`{"request_id":"REQ-1","data_version":"EXERCISE-7"}`), &v, limits()); err != nil {
		t.Fatalf("valid body rejected: %v", err)
	}
	if v.RequestID != "REQ-1" || v.DataVersion != "EXERCISE-7" {
		t.Fatalf("unexpected decode: %+v", v)
	}
}

func TestDecodeStrictRejectsDuplicateKey(t *testing.T) {
	var v simpleBody
	err := DecodeStrict(loadFixture(t, "duplicate_key.json"), &v, limits())
	if got := codeOf(t, err); got != contracts.ErrDuplicateKey {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrDuplicateKey, got, err)
	}
}

func TestDecodeStrictRejectsTrailingData(t *testing.T) {
	var v simpleBody
	err := DecodeStrict(loadFixture(t, "trailing_data.json"), &v, limits())
	if got := codeOf(t, err); got != contracts.ErrTrailingData {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrTrailingData, got, err)
	}
}

func TestDecodeStrictRejectsUnknownField(t *testing.T) {
	var v simpleBody
	err := DecodeStrict(loadFixture(t, "unknown_field.json"), &v, limits())
	if got := codeOf(t, err); got != contracts.ErrUnknownField {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrUnknownField, got, err)
	}
}

func TestDecodeStrictRejectsMalformed(t *testing.T) {
	var v simpleBody
	err := DecodeStrict(loadFixture(t, "malformed.json"), &v, limits())
	if got := codeOf(t, err); got != contracts.ErrMalformedJSON {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrMalformedJSON, got, err)
	}
}

func TestDecodeStrictRejectsOversized(t *testing.T) {
	var v simpleBody
	big := []byte(`{"request_id":"` + strings.Repeat("a", 2<<20) + `"}`)
	err := DecodeStrict(big, &v, limits())
	if got := codeOf(t, err); got != contracts.ErrBodyTooLarge {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrBodyTooLarge, got, err)
	}
}

func TestDecodeStrictRejectsDeepNesting(t *testing.T) {
	var v any
	deep := strings.Repeat(`{"a":`, 20) + `1` + strings.Repeat(`}`, 20)
	err := DecodeStrict([]byte(deep), &v, limits())
	if got := codeOf(t, err); got != contracts.ErrDepthExceeded {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrDepthExceeded, got, err)
	}
}

func TestDecodeStrictRejectsWrongType(t *testing.T) {
	var v simpleBody
	err := DecodeStrict([]byte(`{"request_id":123,"data_version":"X"}`), &v, limits())
	if got := codeOf(t, err); got != contracts.ErrInvalidValue {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrInvalidValue, got, err)
	}
}

func TestDecodeStrictRejectsEmptyBody(t *testing.T) {
	var v simpleBody
	err := DecodeStrict([]byte(""), &v, limits())
	if got := codeOf(t, err); got != contracts.ErrMalformedJSON {
		t.Fatalf("want %s, got %s (%v)", contracts.ErrMalformedJSON, got, err)
	}
}
