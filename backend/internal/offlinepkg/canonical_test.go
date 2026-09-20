package offlinepkg

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func TestCanonicalBytesIndependentKnownVector(t *testing.T) {
	// Object with intentionally unsorted keys, numbers, booleans, and arrays.
	input := map[string]any{
		"zebra":  "stripes",
		"apple":  "fruit",
		"count":  42,
		"active": true,
		"items": []any{
			map[string]any{"b": 2, "a": 1},
			"raw",
		},
	}

	got, err := CanonicalBytes(input)
	if err != nil {
		t.Fatalf("CanonicalBytes error: %v", err)
	}

	// Exact expected canonical representation:
	// - Keys sorted: "active", "apple", "count", "items", "zebra"
	// - Inner object sorted: "a", "b"
	// - No whitespace around ':' or ','
	expected := `{"active":true,"apple":"fruit","count":42,"items":[{"a":1,"b":2},"raw"],"zebra":"stripes"}`

	if string(got) != expected {
		t.Fatalf("CanonicalBytes mismatch:\n got: %s\nwant: %s", string(got), expected)
	}
}

func TestCanonicalBytesStripsIntegrityMetadata(t *testing.T) {
	priv, _ := generateTestKey(t, "key-1", "KL")
	m := validManifestFixture(t, "key-1", priv)

	// Ensure fixture has non-empty checksum and signature
	if m.ChecksumSHA256 == "" || m.Signature == nil {
		t.Fatal("manifest fixture must have checksum and signature initially")
	}

	can, err := CanonicalBytes(m)
	if err != nil {
		t.Fatalf("CanonicalBytes(manifest): %v", err)
	}

	canStr := string(can)
	// Canonical representation MUST have checksum_sha256 as "" and signature as omitted / null.
	if strings.Contains(canStr, `"checksum_sha256":""`) == false {
		t.Errorf("canonical manifest should contain empty checksum_sha256, got: %s", canStr)
	}
	if strings.Contains(canStr, `"signature"`) {
		t.Errorf("canonical manifest should omit nil signature, got: %s", canStr)
	}

	// Also verify PublicIncidentCard
	c := validCardFixture(t, "key-1", priv)
	cardCan, err := CanonicalBytes(c)
	if err != nil {
		t.Fatalf("CanonicalBytes(card): %v", err)
	}
	cardStr := string(cardCan)
	if strings.Contains(cardStr, `"checksum_sha256":""`) == false {
		t.Errorf("canonical card should contain empty checksum_sha256, got: %s", cardStr)
	}
	if strings.Contains(cardStr, `"signature"`) {
		t.Errorf("canonical card should omit nil signature, got: %s", cardStr)
	}
}

func TestChecksumSHA256KnownVector(t *testing.T) {
	// Known standard test vector:
	// SHA-256("hello world") = b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
	data := []byte("hello world")
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	got := ChecksumSHA256(data)
	if got != expected {
		t.Fatalf("ChecksumSHA256 = %q, want %q", got, expected)
	}
}

func TestSignCanonicalDirectVerification(t *testing.T) {
	priv, tk := generateTestKey(t, "test-key-01", "KL")
	m := map[string]any{
		"event": "flood",
		"level": 3,
	}

	sig, err := SignCanonical(priv, "test-key-01", m)
	if err != nil {
		t.Fatalf("SignCanonical error: %v", err)
	}

	if sig.Algorithm != "Ed25519" {
		t.Errorf("algorithm = %q, want Ed25519", sig.Algorithm)
	}
	if sig.KeyID != "test-key-01" {
		t.Errorf("key_id = %q, want test-key-01", sig.KeyID)
	}

	// Verify directly using crypto/ed25519 stdlib without any custom helper
	sigRaw, err := base64.StdEncoding.DecodeString(sig.Value)
	if err != nil {
		t.Fatalf("base64.DecodeString: %v", err)
	}

	canBytes, err := CanonicalBytes(m)
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}

	if !ed25519.Verify(tk.PublicKey, canBytes, sigRaw) {
		t.Fatal("direct ed25519.Verify failed on SignCanonical output")
	}
}
