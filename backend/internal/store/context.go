package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Persisted context resolution. The voice-commands boundary validates a model
// proposal against an authoritative server-side snapshot. This resolver derives
// that snapshot from the CURRENT persisted package whose source is OPERATIONAL,
// holds a valid authorization, and is not quarantined — never from client-echoed
// request/data-version values.
//
// Consistency: every field (data version, jurisdiction, known IDs, languages)
// comes from ONE package row, so a snapshot never mixes fields across revisions.
// Fail closed: absent, expired, unauthorized, suspended, revoked or quarantined
// evidence yields no snapshot.

// ErrNoOperationalContext is returned when no current authorized operational
// package exists to resolve a context from.
var ErrNoOperationalContext = errors.New("store: no current authorized operational package")

// ContextSnapshot is the resolved authoritative context (mirror of the
// httpserver snapshot, kept here to avoid an import cycle).
type ContextSnapshot struct {
	DataVersion      string
	Jurisdiction     string
	KnownIDs         map[string]bool
	EnabledLanguages map[string]bool
}

// packageBody is the minimal shape of the persisted packages.body JSON needed to
// derive known IDs and languages. The body is the canonical opkg.Package; we
// read only the ID/language fields and tolerate absent optional sections.
type packageBody struct {
	RedZones []struct {
		ID string `json:"id"`
	} `json:"red_zones"`
	SafeZones []struct {
		ID string `json:"id"`
	} `json:"safe_zones"`
	Routes []struct {
		ID string `json:"id"`
	} `json:"approved_routes"`
	Facilities []struct {
		ID string `json:"id"`
	} `json:"facilities"`
	Instructions []struct {
		ID       string `json:"id"`
		Language string `json:"language"`
	} `json:"instruction_assets"`
}

// ResolveContext resolves the authoritative snapshot for a jurisdiction from the
// persisted store. It selects the single current package (effective now, not
// expired, not superseded) in that jurisdiction whose source is OPERATIONAL,
// holds a currently-valid authorization in the same jurisdiction, and is not
// QUARANTINED. Scoping by jurisdiction keeps one jurisdiction's context from
// being resolved from another's package (cross-jurisdiction contexts are never
// accepted). The data version is the package's snapshot version
// (package_id:version) so a source/package update or a revocation/quarantine
// changes or removes the resolved snapshot.
func ResolveContext(ctx context.Context, db DBTX, jurisdiction string, now time.Time) (ContextSnapshot, error) {
	if jurisdiction == "" {
		return ContextSnapshot{}, ErrNoOperationalContext
	}
	var (
		pkgID   string
		version int
		body    []byte
	)
	// One consistent row: the package and its source state are read together, so
	// version/body always come from the same revision. The source must be
	// OPERATIONAL (not suspended/retired/quarantined) and hold a valid
	// authorization in the package's own jurisdiction.
	err := db.QueryRowContext(ctx, `
		SELECT p.package_id, p.version, p.body
		FROM packages p
		JOIN sources s ON s.source_id = p.source_id
		WHERE p.jurisdiction = $2
		  AND s.state = 'OPERATIONAL'
		  AND p.effective_at <= $1 AND p.expires_at > $1
		  AND p.superseded_by IS NULL
		  AND EXISTS (
			SELECT 1 FROM source_authorizations sa
			WHERE sa.source_id = p.source_id AND sa.jurisdiction = p.jurisdiction
			  AND (sa.expires_at IS NULL OR sa.expires_at > $1)
		  )
		ORDER BY p.version DESC, p.package_id
		LIMIT 1`, now, jurisdiction).
		Scan(&pkgID, &version, &body)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return ContextSnapshot{}, ErrNoOperationalContext
		}
		return ContextSnapshot{}, err
	}

	var pb packageBody
	if err := json.Unmarshal(body, &pb); err != nil {
		return ContextSnapshot{}, fmt.Errorf("store: decode package body: %w", err)
	}

	known := map[string]bool{}
	for _, z := range pb.RedZones {
		known[z.ID] = true
	}
	for _, z := range pb.SafeZones {
		known[z.ID] = true
	}
	for _, rt := range pb.Routes {
		known[rt.ID] = true
	}
	for _, f := range pb.Facilities {
		known[f.ID] = true
	}
	langs := map[string]bool{}
	for _, a := range pb.Instructions {
		if a.Language != "" {
			langs[a.Language] = true
		}
	}

	return ContextSnapshot{
		DataVersion:      fmt.Sprintf("%s:%d", pkgID, version),
		Jurisdiction:     jurisdiction,
		KnownIDs:         known,
		EnabledLanguages: langs,
	}, nil
}

// ResolveAnyOperationalContext resolves the snapshot for whichever jurisdiction
// has a current authorized OPERATIONAL package, choosing the highest package
// version across jurisdictions. Used where no specific jurisdiction is implied
// by the request. Prefer ResolveContext (jurisdiction-scoped) when one is known.
func ResolveAnyOperationalContext(ctx context.Context, db DBTX, now time.Time) (ContextSnapshot, error) {
	var jurisdiction string
	err := db.QueryRowContext(ctx, `
		SELECT p.jurisdiction
		FROM packages p
		JOIN sources s ON s.source_id = p.source_id
		WHERE s.state = 'OPERATIONAL'
		  AND p.effective_at <= $1 AND p.expires_at > $1
		  AND p.superseded_by IS NULL
		  AND EXISTS (
			SELECT 1 FROM source_authorizations sa
			WHERE sa.source_id = p.source_id AND sa.jurisdiction = p.jurisdiction
			  AND (sa.expires_at IS NULL OR sa.expires_at > $1)
		  )
		ORDER BY p.version DESC, p.package_id
		LIMIT 1`, now).Scan(&jurisdiction)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return ContextSnapshot{}, ErrNoOperationalContext
		}
		return ContextSnapshot{}, err
	}
	return ResolveContext(ctx, db, jurisdiction, now)
}
