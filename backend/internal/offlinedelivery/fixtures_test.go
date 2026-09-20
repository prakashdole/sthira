package offlinedelivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
	"sync/atomic"
)

// bytesReadSeekCloser wraps a bytes.Reader with a no-op Close.
type bytesReadSeekCloser struct {
	*bytes.Reader
}

func (b *bytesReadSeekCloser) Close() error {
	return nil
}

func newBytesStream(data []byte, contentType string) *ResourceContent {
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	return &ResourceContent{
		Reader:         &bytesReadSeekCloser{bytes.NewReader(data)},
		ContentType:    contentType,
		ContentLength:  int64(len(data)),
		ChecksumSHA256: hexSum,
		ETag:           FormatETag(hexSum),
	}
}

// mockPublicationSource provides thread-safe mock fixtures and call counting.
type mockPublicationSource struct {
	mu            sync.Mutex
	manifests     map[string]*ManifestRecord
	cards         map[string]*CardRecord
	resources     map[string][]byte
	resourceMimes map[string]string

	manifestCalls atomic.Int64
	cardCalls     atomic.Int64
	resourceCalls atomic.Int64
}

func newMockPublicationSource() *mockPublicationSource {
	m := &mockPublicationSource{
		manifests:     make(map[string]*ManifestRecord),
		cards:         make(map[string]*CardRecord),
		resources:     make(map[string][]byte),
		resourceMimes: make(map[string]string),
	}
	m.seedDefaultFixtures()
	return m
}

func (m *mockPublicationSource) seedDefaultFixtures() {
	// 1. Regional Manifest for KL
	manifestJSON := []byte(`{
  "schema_version": "3.0",
  "manifest_id": "MNF-KL-seq102",
  "jurisdiction": "KL",
  "sequence": 102,
  "published_at": "2026-09-20T04:45:00Z",
  "expires_at": "2026-09-20T06:00:00Z",
  "active_cards": [
    {
      "card_id": "CARD-KL-WAYANAD-2026-09-v1",
      "package_id": "PKG-WAYANAD-01",
      "version": 1,
      "alert_id": "ALERT-2026-KL-001",
      "checksum_sha256": "3fa98e4f1a23847291a27e9b04f81c9a7d6e5c4b3a210f9e8d7c6b5a4f3e2d1c",
      "size_bytes": 18450,
      "compressed_size_bytes": 4820,
      "download_path": "/api/v3/packages/PKG-WAYANAD-01/versions/1",
      "expires_at": "2026-09-21T04:00:00Z"
    }
  ],
  "revoked_cards": [
    {
      "card_id": "CARD-KL-OLD-v2",
      "package_id": "PKG-OLD-8",
      "revoked_at": "2026-09-20T04:00:00Z",
      "reason": "SUPERSEDED"
    }
  ],
  "resources": [
    {
      "resource_id": "RES-MAP-KL-WAYANAD-v1",
      "resource_type": "MAP_VECTOR_BUNDLE",
      "version": 1,
      "checksum_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "size_bytes": 1048576,
      "download_path": "/api/v3/resources/RES-MAP-KL-WAYANAD-v1",
      "license_id": "SYNTHETIC_FIXTURE",
      "license_status": "SYNTHETIC_FIXTURE"
    }
  ],
  "checksum_sha256": "c8a4d70b55f1a56111a8b98e7e163584852924fa68b75f8f8f2b7f0f622998a1",
  "signature": {
    "algorithm": "Ed25519",
    "key_id": "ed25519:kl-ddma-2026-01",
    "value": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
  }
}`)
	m.setManifest("KL", manifestJSON, "c8a4d70b55f1a56111a8b98e7e163584852924fa68b75f8f8f2b7f0f622998a1")

	// 2. Incident Card for Wayanad PKG-WAYANAD-01 v1
	cardJSON := []byte(`{
  "schema_version": "3.0",
  "card_id": "CARD-KL-WAYANAD-2026-09-v1",
  "package_id": "PKG-WAYANAD-01",
  "version": 1,
  "jurisdiction": "KL",
  "alert_id": "ALERT-2026-KL-001",
  "evidence_class": "AUTHORIZED_OPERATIONAL",
  "effective_at": "2026-09-20T04:00:00Z",
  "expires_at": "2026-09-21T04:00:00Z",
  "hazard": {
    "event_type": "LANDSLIDE",
    "severity": "Extreme",
    "urgency": "Immediate",
    "certainty": "Observed",
    "headline": "Massive debris flow in Meppadi sector",
    "instruction": "Evacuate immediately via approved foot/vehicle paths to designated relief shelters."
  },
  "red_zones": [
    {
      "id": "RZ-MEPPADI-01",
      "name": "Chooralmala Debris Zone",
      "status": "ACTIVE"
    }
  ],
  "safe_zones": [
    {
      "id": "SZ-MEPPADI-HS",
      "name": "Higher Secondary School Relief Centre",
      "location": [76.128, 11.554],
      "status": "OPEN",
      "role": "EMERGENCY_SHELTER"
    }
  ],
  "routes": [
    {
      "id": "RT-MEPPADI-01",
      "from_zone_id": "RZ-MEPPADI-01",
      "to_safe_zone_id": "SZ-MEPPADI-HS",
      "mode": "FOOT",
      "approval": "AUTHORIZED_OPERATIONAL",
      "valid_from": "2026-09-20T04:00:00Z",
      "valid_until": "2026-09-21T04:00:00Z",
      "geometry": {
        "type": "LineString",
        "coordinates": [[76.122, 11.505], [76.125, 11.530], [76.128, 11.554]]
      }
    }
  ],
  "instructions": [
    {
      "id": "INS-01-ML",
      "language": "ml-IN",
      "title": "ഉടൻ പുറപ്പെടുക",
      "text": "നിശ്ചിത പാതയിലൂടെ മാത്രം സഞ്ചരിക്കുക."
    }
  ],
  "emergency_contacts": [
    { "name": "Emergency Operations", "number": "112" }
  ],
  "checksum_sha256": "3fa98e4f1a23847291a27e9b04f81c9a7d6e5c4b3a210f9e8d7c6b5a4f3e2d1c",
  "signature": {
    "algorithm": "Ed25519",
    "key_id": "ed25519:kl-ddma-2026-01",
    "value": "sig-card-bytes-hex"
  }
}`)
	m.setCard("PKG-WAYANAD-01", 1, cardJSON, "3fa98e4f1a23847291a27e9b04f81c9a7d6e5c4b3a210f9e8d7c6b5a4f3e2d1c")

	// 3. Binary Resource: 64 KiB pseudo-vector map payload
	resourceBytes := make([]byte, 65536)
	for i := range resourceBytes {
		resourceBytes[i] = byte(i % 256)
	}
	m.setResource("RES-MAP-KL-WAYANAD-v1", resourceBytes, "application/vnd.mapbox-vector-tile")
}

func (m *mockPublicationSource) setManifest(jurisdiction string, rawJSON []byte, checksum string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manifests[jurisdiction] = &ManifestRecord{
		Jurisdiction:   jurisdiction,
		Revision:       102,
		RawJSON:        rawJSON,
		ChecksumSHA256: checksum,
		SourceStatus:   "CURRENT",
	}
}

func (m *mockPublicationSource) setCard(pkgID string, version int, rawJSON []byte, checksum string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := cardKey(pkgID, version)
	m.cards[key] = &CardRecord{
		PackageID:      pkgID,
		Version:        version,
		RawJSON:        rawJSON,
		ChecksumSHA256: checksum,
		SourceStatus:   "CURRENT",
	}
}

func (m *mockPublicationSource) setResource(resourceID string, data []byte, mime string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[resourceID] = data
	m.resourceMimes[resourceID] = mime
}

func (m *mockPublicationSource) GetManifest(ctx context.Context, jurisdiction string) (*ManifestRecord, error) {
	m.manifestCalls.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.manifests[jurisdiction]
	if !ok {
		return nil, ErrNotFound
	}
	return rec, nil
}

func (m *mockPublicationSource) GetCard(ctx context.Context, packageID string, version int) (*CardRecord, error) {
	m.cardCalls.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.cards[cardKey(packageID, version)]
	if !ok {
		return nil, ErrNotFound
	}
	return rec, nil
}

func (m *mockPublicationSource) GetResource(ctx context.Context, resourceID string) (*ResourceContent, error) {
	m.resourceCalls.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.resources[resourceID]
	if !ok {
		return nil, ErrNotFound
	}
	mime := m.resourceMimes[resourceID]
	return newBytesStream(data, mime), nil
}

func cardKey(pkgID string, ver int) string {
	return pkgID + ":" + string(rune('0'+ver))
}

// slowReader implements io.ReadSeekCloser with artificial latency to test cancellation.
type slowReader struct {
	data   []byte
	offset int64
	block  chan struct{}
}

func newSlowReader(data []byte) *slowReader {
	return &slowReader{
		data:  data,
		block: make(chan struct{}),
	}
}

func (s *slowReader) Read(p []byte) (n int, err error) {
	<-s.block // Blocks until unblocked or test completes
	return 0, io.EOF
}

func (s *slowReader) Seek(offset int64, whence int) (int64, error) {
	s.offset = offset
	return s.offset, nil
}

func (s *slowReader) Close() error {
	select {
	case <-s.block:
	default:
		close(s.block)
	}
	return nil
}
