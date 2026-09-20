package offlinepkg

import (
	"errors"
	"testing"
	"time"
)

func TestVerifySignatureValid(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-01", "KL")
	store := NewTrustStore(tk)

	payload := map[string]any{"jurisdiction": "KL", "status": "active"}
	sig, err := SignCanonical(priv, "key-kl-01", payload)
	if err != nil {
		t.Fatalf("SignCanonical: %v", err)
	}

	can, err := CanonicalBytes(payload)
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}

	if err := store.VerifySignature("key-kl-01", "KL", can, sig.Value); err != nil {
		t.Fatalf("VerifySignature failed: %v", err)
	}
}

func TestVerifySignatureAlteredContent(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-01", "KL")
	store := NewTrustStore(tk)

	payload := map[string]any{"jurisdiction": "KL", "status": "active"}
	sig, err := SignCanonical(priv, "key-kl-01", payload)
	if err != nil {
		t.Fatalf("SignCanonical: %v", err)
	}

	alteredPayload := map[string]any{"jurisdiction": "KL", "status": "tampered"}
	alteredCan, err := CanonicalBytes(alteredPayload)
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}

	err = store.VerifySignature("key-kl-01", "KL", alteredCan, sig.Value)
	if err == nil {
		t.Fatal("VerifySignature succeeded on altered content; want error")
	}
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want ErrInvalidSignature", err)
	}
}

func TestVerifySignatureUnknownKey(t *testing.T) {
	_, tk := generateTestKey(t, "key-kl-01", "KL")
	store := NewTrustStore(tk)

	err := store.VerifySignature("key-unknown-99", "KL", []byte("{}"), "c2lnbmF0dXJl")
	if err == nil {
		t.Fatal("VerifySignature succeeded on unknown key; want error")
	}
	if !errors.Is(err, ErrSignerUnauthorized) && !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("err = %v, want ErrSignerUnauthorized or ErrUnknownKey", err)
	}
}

func TestVerifySignatureRevokedKey(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-01", "KL")
	tk.Revoked = true
	store := NewTrustStore(tk)

	payload := map[string]any{"jurisdiction": "KL"}
	sig, _ := SignCanonical(priv, "key-kl-01", payload)
	can, _ := CanonicalBytes(payload)

	err := store.VerifySignature("key-kl-01", "KL", can, sig.Value)
	if err == nil {
		t.Fatal("VerifySignature succeeded on revoked key; want error")
	}
	if !errors.Is(err, ErrSignerUnauthorized) && !errors.Is(err, ErrKeyRevoked) {
		t.Fatalf("err = %v, want ErrSignerUnauthorized or ErrKeyRevoked", err)
	}
}

func TestVerifySignatureWrongJurisdictionScope(t *testing.T) {
	priv, tk := generateTestKey(t, "key-tn-01", "TN") // Key authorized ONLY for TN
	store := NewTrustStore(tk)

	payload := map[string]any{"jurisdiction": "KL"}
	sig, _ := SignCanonical(priv, "key-tn-01", payload)
	can, _ := CanonicalBytes(payload)

	// Attempt to verify key against document claiming jurisdiction "KL"
	err := store.VerifySignature("key-tn-01", "KL", can, sig.Value)
	if err == nil {
		t.Fatal("VerifySignature succeeded for wrong jurisdiction; want error")
	}
	if !errors.Is(err, ErrSignerUnauthorized) {
		t.Fatalf("err = %v, want ErrSignerUnauthorized", err)
	}
}

func TestVerifySignatureExpiredKey(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-expired", "KL")
	// Key expired yesterday
	tk.ValidFrom = time.Now().Add(-48 * time.Hour).UTC()
	tk.ValidUntil = time.Now().Add(-24 * time.Hour).UTC()
	store := NewTrustStore(tk)

	payload := map[string]any{"jurisdiction": "KL"}
	sig, _ := SignCanonical(priv, "key-kl-expired", payload)
	can, _ := CanonicalBytes(payload)

	err := store.VerifySignature("key-kl-expired", "KL", can, sig.Value)
	if err == nil {
		t.Fatal("VerifySignature succeeded on expired key; want error")
	}
	if !errors.Is(err, ErrKeyExpired) {
		t.Fatalf("err = %v, want ErrKeyExpired", err)
	}
}

func TestVerifySignatureNotYetValidKey(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-future", "KL")
	// Key valid starting tomorrow
	tk.ValidFrom = time.Now().Add(24 * time.Hour).UTC()
	tk.ValidUntil = time.Now().Add(48 * time.Hour).UTC()
	store := NewTrustStore(tk)

	payload := map[string]any{"jurisdiction": "KL"}
	sig, _ := SignCanonical(priv, "key-kl-future", payload)
	can, _ := CanonicalBytes(payload)

	err := store.VerifySignature("key-kl-future", "KL", can, sig.Value)
	if err == nil {
		t.Fatal("VerifySignature succeeded on not-yet-valid key; want error")
	}
	if !errors.Is(err, ErrKeyExpired) {
		t.Fatalf("err = %v, want ErrKeyExpired", err)
	}
}

func TestVerifySignatureCorruptedBase64(t *testing.T) {
	_, tk := generateTestKey(t, "key-kl-01", "KL")
	store := NewTrustStore(tk)

	err := store.VerifySignature("key-kl-01", "KL", []byte("{}"), "!!!invalid-base64!!!")
	if err == nil {
		t.Fatal("VerifySignature succeeded on corrupted base64; want error")
	}
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want ErrInvalidSignature", err)
	}
}
