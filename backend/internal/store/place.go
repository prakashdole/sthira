package store

import (
	"context"
	"strings"
)

// Place resolution maps a jurisdiction-scoped name/alias/admin-ID to candidate
// places. It returns explicit ambiguity (multiple candidates) or not-found,
// never a guessed geocode. Lookup keys are normalized (trimmed, lowercased);
// matching is exact on the normalized key, so a partial name is not silently
// resolved to a place.
//
// This is a public, account-free lookup (R22): it exposes only place IDs and
// kinds already present in the active package, no private assignment data.

// PlaceCandidate is one resolved place.
type PlaceCandidate struct {
	PlaceID      string `json:"place_id"`
	PlaceKind    string `json:"place_kind"` // ZONE | FACILITY | ADMIN
	Jurisdiction string `json:"jurisdiction,omitempty"`
}

// ErrPlaceNotFound is returned when no alias matches the query.
var ErrPlaceNotFound = errPlace("store: place not found")

// ErrPlaceAmbiguous is returned when a query matches multiple distinct places.
type AmbiguousPlaceError struct {
	Candidates []PlaceCandidate `json:"candidates"`
}

func (e *AmbiguousPlaceError) Error() string { return "store: ambiguous place" }

type errPlace string

func (e errPlace) Error() string { return string(e) }

// NormalizeLookupKey normalizes an alias/admin-ID for matching.
func NormalizeLookupKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ResolvePlace resolves lookupKey within jurisdiction. It returns exactly one
// candidate, ErrPlaceNotFound, or *AmbiguousPlaceError with the distinct
// candidate places. Distinct aliases mapping to the same place collapse to one
// candidate (not ambiguous). Matching considers both the normalized lookup_key
// and the lowercased place_id so a citizen selecting a specific candidate ID
// resolves unambiguously.
func ResolvePlace(ctx context.Context, db DBTX, jurisdiction, lookupKey string) (PlaceCandidate, error) {
	key := NormalizeLookupKey(lookupKey)
	if key == "" {
		return PlaceCandidate{}, ErrPlaceNotFound
	}
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT place_id, place_kind FROM place_aliases
		WHERE jurisdiction = $1 AND (lookup_key = $2 OR LOWER(place_id) = $2)
		ORDER BY place_id`, jurisdiction, key)
	if err != nil {
		return PlaceCandidate{}, err
	}
	defer rows.Close()
	var cands []PlaceCandidate
	for rows.Next() {
		var c PlaceCandidate
		if err := rows.Scan(&c.PlaceID, &c.PlaceKind); err != nil {
			return PlaceCandidate{}, err
		}
		c.Jurisdiction = jurisdiction
		cands = append(cands, c)
	}
	if err := rows.Err(); err != nil {
		return PlaceCandidate{}, err
	}
	switch len(cands) {
	case 0:
		return PlaceCandidate{}, ErrPlaceNotFound
	case 1:
		return cands[0], nil
	default:
		// Bound candidates per contract
		if len(cands) > 10 {
			cands = cands[:10]
		}
		return PlaceCandidate{}, &AmbiguousPlaceError{Candidates: cands}
	}
}

// InsertPlaceAlias registers an alias. Used by package import / test fixtures.
func InsertPlaceAlias(ctx context.Context, db DBTX, aliasID, jurisdiction, lookupKey, placeID, placeKind string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO place_aliases (alias_id, jurisdiction, lookup_key, place_id, place_kind)
		VALUES ($1, $2, $3, $4, $5)`,
		aliasID, jurisdiction, NormalizeLookupKey(lookupKey), placeID, placeKind)
	return err
}
