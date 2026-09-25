package offlinedelivery

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"sthira/backend/internal/offlinepkg"
)

var (
	// ErrRevisionRollback indicates an incoming manifest revision is lower than currently active.
	ErrRevisionRollback = errors.New("offlinedelivery: manifest revision regression detected")
	// ErrIdenticalRevisionModified indicates an identical revision number was submitted with altered content.
	ErrIdenticalRevisionModified = errors.New("offlinedelivery: identical manifest revision has differing checksum")
	// ErrManifestExpired indicates the manifest has passed its ValidUntil timestamp.
	ErrManifestExpired = errors.New("offlinedelivery: manifest valid_until has expired")
	// ErrTombstoneViolated indicates an active card attempts to use a tombstoned/revoked package.
	ErrTombstoneViolated = errors.New("offlinedelivery: active critical card references a tombstoned package")
	// ErrTombstoneDropped indicates a successor revision dropped an existing tombstone/revocation record.
	ErrTombstoneDropped = errors.New("offlinedelivery: successor manifest dropped previously recorded tombstone")
)

// JurisdictionManifestState holds the active manifest state for a jurisdiction.
type JurisdictionManifestState struct {
	Jurisdiction    string
	Revision        int
	ChecksumSHA256  string
	ValidUntil      time.Time
	RevokedPackages map[string]struct{}
	CancelledRoutes map[string]struct{}
	LastCommittedAt time.Time
}

// ManifestRevisionTracker enforces monotonic revision progression and tombstone retention
// across regional offline discovery manifests.
type ManifestRevisionTracker struct {
	mu     sync.RWMutex
	states map[string]*JurisdictionManifestState
}

// NewManifestRevisionTracker initializes an empty tracker.
func NewManifestRevisionTracker() *ManifestRevisionTracker {
	return &ManifestRevisionTracker{
		states: make(map[string]*JurisdictionManifestState),
	}
}

// ValidateNext validates whether next can be legally admitted as the successor manifest
// for its jurisdiction at time `now`.
func (t *ManifestRevisionTracker) ValidateNext(next *offlinepkg.Manifest, now time.Time) error {
	if next == nil {
		return errors.New("offlinedelivery: nil manifest")
	}

	validUntil, err := time.Parse(time.RFC3339, next.ValidUntil)
	if err != nil {
		return fmt.Errorf("offlinedelivery: invalid valid_until RFC3339: %w", err)
	}
	if !now.IsZero() && now.After(validUntil) {
		return ErrManifestExpired
	}

	t.mu.RLock()
	current, exists := t.states[next.Jurisdiction]
	t.mu.RUnlock()

	if !exists {
		// First manifest for this jurisdiction: verify internal self-consistency
		for _, revoked := range next.Revocations.RevokedPackages {
			if next.CriticalCard.PackageID == revoked {
				return ErrTombstoneViolated
			}
		}
		return nil
	}

	// 1. Monotonic revision progression check
	if next.Revision < current.Revision {
		return fmt.Errorf("%w: attempted %d < current %d", ErrRevisionRollback, next.Revision, current.Revision)
	}

	if next.Revision == current.Revision {
		if next.ChecksumSHA256 != current.ChecksumSHA256 {
			return fmt.Errorf("%w: revision %d checksum mismatch", ErrIdenticalRevisionModified, next.Revision)
		}
		// Exact identical revision resubmission: allowed
		return nil
	}

	// 2. next.Revision > current.Revision: verify tombstone preservation and non-violation
	if _, isRevoked := current.RevokedPackages[next.CriticalCard.PackageID]; isRevoked {
		return fmt.Errorf("%w: package %s was revoked in revision %d", ErrTombstoneViolated, next.CriticalCard.PackageID, current.Revision)
	}
	for _, revoked := range next.Revocations.RevokedPackages {
		if next.CriticalCard.PackageID == revoked {
			return fmt.Errorf("%w: package %s is listed in revocations", ErrTombstoneViolated, next.CriticalCard.PackageID)
		}
	}

	// 3. Ensure all previously revoked packages remain recorded in next.Revocations
	nextRevokedMap := make(map[string]struct{}, len(next.Revocations.RevokedPackages))
	for _, p := range next.Revocations.RevokedPackages {
		nextRevokedMap[p] = struct{}{}
	}
	for prevPkg := range current.RevokedPackages {
		if _, stillPresent := nextRevokedMap[prevPkg]; !stillPresent {
			return fmt.Errorf("%w: revoked package %s dropped in revision %d", ErrTombstoneDropped, prevPkg, next.Revision)
		}
	}

	// 4. Ensure all previously cancelled routes remain recorded in next.Revocations
	nextCancelledRoutes := make(map[string]struct{}, len(next.Revocations.CancelledRoutes))
	for _, r := range next.Revocations.CancelledRoutes {
		nextCancelledRoutes[r] = struct{}{}
	}
	for prevRoute := range current.CancelledRoutes {
		if _, stillPresent := nextCancelledRoutes[prevRoute]; !stillPresent {
			return fmt.Errorf("%w: cancelled route %s dropped in revision %d", ErrTombstoneDropped, prevRoute, next.Revision)
		}
	}

	return nil
}

// Commit validates and atomically commits the successor manifest state for the jurisdiction.
func (t *ManifestRevisionTracker) Commit(next *offlinepkg.Manifest, now time.Time) error {
	if err := t.ValidateNext(next, now); err != nil {
		return err
	}

	validUntil, _ := time.Parse(time.RFC3339, next.ValidUntil)

	revokedMap := make(map[string]struct{}, len(next.Revocations.RevokedPackages))
	for _, p := range next.Revocations.RevokedPackages {
		revokedMap[p] = struct{}{}
	}

	cancelledMap := make(map[string]struct{}, len(next.Revocations.CancelledRoutes))
	for _, r := range next.Revocations.CancelledRoutes {
		cancelledMap[r] = struct{}{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Double check under write lock
	if current, exists := t.states[next.Jurisdiction]; exists {
		if next.Revision < current.Revision {
			return ErrRevisionRollback
		}
		if next.Revision == current.Revision && next.ChecksumSHA256 != current.ChecksumSHA256 {
			return ErrIdenticalRevisionModified
		}
	}

	t.states[next.Jurisdiction] = &JurisdictionManifestState{
		Jurisdiction:    next.Jurisdiction,
		Revision:        next.Revision,
		ChecksumSHA256:  next.ChecksumSHA256,
		ValidUntil:      validUntil,
		RevokedPackages: revokedMap,
		CancelledRoutes: cancelledMap,
		LastCommittedAt: now,
	}

	return nil
}

// GetState returns a snapshot copy of the active state for the jurisdiction.
func (t *ManifestRevisionTracker) GetState(jurisdiction string) (JurisdictionManifestState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.states[jurisdiction]
	if !ok {
		return JurisdictionManifestState{}, false
	}

	revCopy := make(map[string]struct{}, len(s.RevokedPackages))
	for k := range s.RevokedPackages {
		revCopy[k] = struct{}{}
	}
	canCopy := make(map[string]struct{}, len(s.CancelledRoutes))
	for k := range s.CancelledRoutes {
		canCopy[k] = struct{}{}
	}

	return JurisdictionManifestState{
		Jurisdiction:    s.Jurisdiction,
		Revision:        s.Revision,
		ChecksumSHA256:  s.ChecksumSHA256,
		ValidUntil:      s.ValidUntil,
		RevokedPackages: revCopy,
		CancelledRoutes: canCopy,
		LastCommittedAt: s.LastCommittedAt,
	}, true
}
