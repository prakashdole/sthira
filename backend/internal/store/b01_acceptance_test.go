package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// TestB01_NullLanguageRowCannotAuthorizeActive: an active approval with
// NULL language (legacy wildcard) must not authorize speech. Migration
// 0010's CHECK constraint rejects incomplete active rows; a historical
// revoked incomplete row is readable as revoked but never returned as an
// active approval.
func TestB01_NullLanguageRowCannotAuthorizeActive(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	now := nowUTC()
	srcID := findKLP6Source(t, fx.store.DB())
	dig := hex.EncodeToString(func() []byte { s := sha256.Sum256([]byte("Welcome, citizen.")); return s[:] }())

	// Active incomplete row (NULL language) must be rejected by CHECK.
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES ($1, $2, 'null_lang', NULL, 7, 7, $3, $4, 'reviewer-1', 'doc-1', $5)`,
		"APP-NULL-"+uid("X"), fx.jurisdictionID, srcID, dig, now); err == nil {
		t.Fatalf("active row with NULL language must be rejected by 0010 CHECK")
	}

	// Historical revoked incomplete row (NULL source_id) may exist for audit
	// but must not authorize.
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at, revoked_at)
		VALUES ($1, $2, 'legacy_null_src', 'en-IN', 7, 7, NULL, $3, 'reviewer-1', 'doc-1', $4, $4)`,
		"APP-LEGACY-"+uid("X"), fx.jurisdictionID, dig, now); err != nil {
		t.Fatalf("historical revoked incomplete row must be insertable: %v", err)
	}

	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if sc.IsSpeechKeyApprovedForLanguage("legacy_null_src", "en-IN") {
		t.Fatalf("revoked NULL-source row must not authorize")
	}
	if sc.IsSpeechKeyApprovedForLanguage("null_lang", "en-IN") {
		t.Fatalf("NULL language must not authorize")
	}
}

// TestB01_ExactApprovalAuthorizes: a complete exact binding (language +
// source_id + template_sha256) authorizes the matching speech key.
func TestB01_ExactApprovalAuthorizes(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	now := nowUTC()
	srcID := findKLP6Source(t, fx.store.DB())
	text := "Welcome, citizen."
	dsum := sha256.Sum256([]byte(text))
	dig := hex.EncodeToString(dsum[:])

	if _, err := fx.store.DB().ExecContext(context.Background(), `
		INSERT INTO approved_translations
			(translation_id, jurisdiction, speech_key, language, source_version, template_version, source_id, template_sha256, approved_by, evidence_ref, approved_at)
		VALUES ($1, $2, 'welcome', 'en-IN', 7, 7, $3, $4, 'reviewer-1', 'doc-1', $5)`,
		"APP-OK-"+uid("X"), fx.jurisdictionID, srcID, dig, now); err != nil {
		t.Fatalf("seed exact approval: %v", err)
	}

	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !sc.IsSpeechKeyApprovedForLanguage("welcome", "en-IN") {
		t.Fatalf("exact complete approval must authorize")
	}
	if sc.ApprovedTemplateSHA["welcome/en-IN"] != dig {
		t.Fatalf("ApprovedTemplateSHA welcome/en-IN = %q want %q", sc.ApprovedTemplateSHA["welcome/en-IN"], dig)
	}
	if gotDig, ok := sc.TemplateDigest("welcome", "en-IN"); !ok || gotDig != dig {
		t.Fatalf("TemplateDigest welcome/en-IN = %q want %q", gotDig, dig)
	}
	if !contains(sc.TemplateKeys, "welcome") {
		t.Fatalf("TemplateKeys must include welcome, got %v", sc.TemplateKeys)
	}

	// SnapshotRevalidate must pass for the stable snapshot.
	if err := resolver.SnapshotRevalidate(context.Background(), sc); err != nil {
		t.Fatalf("stable revalidate: %v", err)
	}

	// Digest change on disk must fail revalidate (stale cache / withdrawn text).
	other := hex.EncodeToString([]byte(strings.Repeat("b", 32)))
	if _, err := fx.store.DB().ExecContext(context.Background(), `
		UPDATE approved_translations SET template_sha256 = $1
		WHERE jurisdiction = $2 AND speech_key = 'welcome' AND revoked_at IS NULL`,
		other, fx.jurisdictionID); err != nil {
		t.Fatalf("update digest: %v", err)
	}
	if err := resolver.SnapshotRevalidate(context.Background(), sc); err == nil {
		t.Fatalf("digest change must fail SnapshotRevalidate")
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// unused time import guard
var _ = time.Second
