package offlinepkg

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseManifestAndCardSuccess(t *testing.T) {
	priv, _ := generateTestKey(t, "key-kl-01", "KL")
	m := validManifestFixture(t, "key-kl-01", priv)
	mData, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal manifest: %v", err)
	}

	parsedM, err := ParseManifest(mData, DefaultLimits)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}
	if parsedM.ManifestID != m.ManifestID {
		t.Errorf("manifest_id = %q, want %q", parsedM.ManifestID, m.ManifestID)
	}

	c := validCardFixture(t, "key-kl-01", priv)
	cData, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal card: %v", err)
	}

	parsedC, err := ParseCard(cData, DefaultLimits)
	if err != nil {
		t.Fatalf("ParseCard failed: %v", err)
	}
	if parsedC.PackageID != c.PackageID {
		t.Errorf("package_id = %q, want %q", parsedC.PackageID, c.PackageID)
	}
}

func TestParseOversizedInput(t *testing.T) {
	tinyLimits := Limits{MaxBytes: 64, MaxDepth: 10}
	hugeData := []byte(`{"schema_version": "3.0", "extra_padding": "` + strings.Repeat("A", 100) + `"}`)

	_, err := ParseManifest(hugeData, tinyLimits)
	if err == nil {
		t.Fatal("ParseManifest succeeded on oversized input; want error")
	}
	if !errors.Is(err, ErrMalformedData) {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestParseDeeplyNestedJSON(t *testing.T) {
	// Construct JSON exceeding max depth
	shallowLimits := Limits{MaxBytes: 10000, MaxDepth: 4}
	deep := `{"a":{"b":{"c":{"d":{"e":{"f":1}}}}}}`

	_, err := ParseCard([]byte(deep), shallowLimits)
	if err == nil {
		t.Fatal("ParseCard succeeded on deeply nested input; want error")
	}
	if !errors.Is(err, ErrMalformedData) {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestParseDuplicateKeys(t *testing.T) {
	dupJSON := []byte(`{
		"schema_version": "3.0",
		"schema_version": "3.0",
		"manifest_id": "MAN-01"
	}`)

	_, err := ParseManifest(dupJSON, DefaultLimits)
	if err == nil {
		t.Fatal("ParseManifest succeeded on duplicate keys; want error")
	}
	if !errors.Is(err, ErrMalformedData) {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestParseTrailingData(t *testing.T) {
	priv, _ := generateTestKey(t, "key-kl-01", "KL")
	m := validManifestFixture(t, "key-kl-01", priv)
	mData, _ := json.Marshal(m)

	trailing := append(mData, []byte(` {"trailing":"data"}`)...)
	_, err := ParseManifest(trailing, DefaultLimits)
	if err == nil {
		t.Fatal("ParseManifest succeeded on trailing data; want error")
	}
	if !errors.Is(err, ErrMalformedData) {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestParseRejectsPrivateFieldsSeparation(t *testing.T) {
	priv, _ := generateTestKey(t, "key-kl-01", "KL")
	c := validCardFixture(t, "key-kl-01", priv)

	// Inject prohibited private citizen/session/token fields into the card JSON
	rawMap := make(map[string]any)
	rawBytes, _ := json.Marshal(c)
	_ = json.Unmarshal(rawBytes, &rawMap)

	prohibitedFields := []string{
		"bearer_token",
		"citizen_session_id",
		"user_private_key",
		"reservation_token",
		"citizen_phone_number",
		"session_credentials",
	}

	for _, field := range prohibitedFields {
		t.Run("reject_"+field, func(t *testing.T) {
			testMap := make(map[string]any)
			for k, v := range rawMap {
				testMap[k] = v
			}
			testMap[field] = "sensitive_private_value_123"

			injectedData, err := json.Marshal(testMap)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			_, err = ParseCard(injectedData, DefaultLimits)
			if err == nil {
				t.Fatalf("ParseCard succeeded with prohibited field %q; want ErrMalformedData", field)
			}
			if !errors.Is(err, ErrMalformedData) {
				t.Fatalf("err = %v, want ErrMalformedData", err)
			}
		})
	}
}
