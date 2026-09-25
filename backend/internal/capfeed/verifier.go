package capfeed

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrDigestMismatch indicates the alert content does not match the canonical digest.
	ErrDigestMismatch = errors.New("alert canonical digest mismatch: potential content tampering detected")
	// ErrDuplicateAlert indicates an alert with identical content digest was already ingested.
	ErrDuplicateAlert = errors.New("duplicate alert digest detected")
)

// CanonicalAlertArea represents normalized polygon boundaries with deterministic coordinates.
type CanonicalAlertArea struct {
	Index    int      `json:"index"`
	Polygons []string `json:"polygons"`
}

// CanonicalAlertPayload is the deterministic, sorted structure used for SHA-256 digest computation.
type CanonicalAlertPayload struct {
	Identifier  string               `json:"identifier"`
	Sender      string               `json:"sender"`
	Sent        string               `json:"sent"` // RFC3339 UTC
	Status      string               `json:"status"`
	MsgType     string               `json:"msg_type"`
	Scope       string               `json:"scope"`
	References  []string             `json:"references"`
	Language    string               `json:"language"`
	Event       string               `json:"event"`
	Effective   string               `json:"effective"` // RFC3339 UTC
	Expires     string               `json:"expires"`   // RFC3339 UTC
	Headline    string               `json:"headline"`
	Areas       []CanonicalAlertArea `json:"areas"`
	Operational bool                 `json:"operational"`
}

// NormalizeAlert transforms an Alert into a deterministic canonical representation.
// Whitespace in text fields is collapsed and trimmed; polygon coordinates are formatted
// to 6 decimal places precision (EPSG:4326 longitude,latitude); references are sorted.
func NormalizeAlert(alert Alert) CanonicalAlertPayload {
	refs := make([]string, len(alert.References))
	copy(refs, alert.References)
	for i := range refs {
		refs[i] = strings.TrimSpace(refs[i])
	}
	sort.Strings(refs)

	areas := make([]CanonicalAlertArea, 0, len(alert.Areas))
	for idx, ring := range alert.Areas {
		pointStrs := make([]string, 0, len(ring))
		for _, pt := range ring {
			// Canonical coordinate formatting: longitude,latitude with 6 decimal places
			pointStrs = append(pointStrs, fmt.Sprintf("%.6f,%.6f", pt.Coordinates[0], pt.Coordinates[1]))
		}
		ringStr := strings.Join(pointStrs, " ")
		areas = append(areas, CanonicalAlertArea{
			Index:    idx,
			Polygons: []string{ringStr},
		})
	}

	return CanonicalAlertPayload{
		Identifier:  strings.TrimSpace(alert.Identifier),
		Sender:      strings.TrimSpace(alert.Sender),
		Sent:        alert.Sent.UTC().Format(time.RFC3339),
		Status:      string(alert.Status),
		MsgType:     string(alert.MsgType),
		Scope:       string(alert.Scope),
		References:  refs,
		Language:    strings.TrimSpace(alert.Language),
		Event:       strings.Join(strings.Fields(alert.Event), " "),
		Effective:   alert.Effective.UTC().Format(time.RFC3339),
		Expires:     alert.Expires.UTC().Format(time.RFC3339),
		Headline:    strings.Join(strings.Fields(alert.Headline), " "),
		Areas:       areas,
		Operational: alert.Operational,
	}
}

// ComputeCanonicalDigest computes the SHA-256 digest over the canonical JSON payload of an alert.
func ComputeCanonicalDigest(alert Alert) (string, error) {
	payload := NormalizeAlert(alert)
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("canonical alert marshal failed: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// VerifyAlertIntegrity verifies that the alert's canonical digest matches the expected digest.
// Uses constant-time comparison to prevent side-channel timing attacks.
func VerifyAlertIntegrity(alert Alert, expectedDigest string) error {
	actualDigest, err := ComputeCanonicalDigest(alert)
	if err != nil {
		return err
	}
	cleanExpected := strings.ToLower(strings.TrimSpace(expectedDigest))
	if subtle.ConstantTimeCompare([]byte(actualDigest), []byte(cleanExpected)) != 1 {
		return ErrDigestMismatch
	}
	return nil
}

// AlertDeduplicator tracks verified alert digests within a sliding TTL window to prevent replayed alerts.
type AlertDeduplicator struct {
	mu       sync.Mutex
	seen     map[string]time.Time
	ttl      time.Duration
	capacity int
}

// NewAlertDeduplicator creates a thread-safe deduplicator with the given TTL and capacity bound.
func NewAlertDeduplicator(ttl time.Duration, capacity int) *AlertDeduplicator {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if capacity <= 0 {
		capacity = 10000
	}
	return &AlertDeduplicator{
		seen:     make(map[string]time.Time),
		ttl:      ttl,
		capacity: capacity,
	}
}

// CheckAndRecord checks if the digest was already seen within the TTL window.
// Returns ErrDuplicateAlert if already seen, or nil and records it if new.
func (d *AlertDeduplicator) CheckAndRecord(digest string, now time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Periodic purge if capacity is exceeded
	if len(d.seen) >= d.capacity {
		for k, t := range d.seen {
			if now.Sub(t) > d.ttl {
				delete(d.seen, k)
			}
		}
	}

	if lastSeen, ok := d.seen[digest]; ok {
		if now.Sub(lastSeen) <= d.ttl {
			return ErrDuplicateAlert
		}
	}

	d.seen[digest] = now
	return nil
}
