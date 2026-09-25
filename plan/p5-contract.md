# P5 Implementation Contract: Offline Package & Map-Delivery Protocol

**Phase:** P5 — Offline package and map-delivery protocol  
**Status:** IN_PROGRESS — engineering acceptance reopened for corrections A–G
**Date:** 2026-09-20  
**Governing Baseline:** Sthira v2 (`plan/architecture.md`, `plan/trd.md`, `plan/parameters.md`, `plan/decisions.md`)  
**Active Blockers Retained:** O01, O05, O06, O07, O14 (no invented government facts, licenses, or operational route approvals)  

---

## 1. Scope, Boundaries & Invariants

P5 implements the offline package, public incident card, signed manifest delivery, map resource validation, disk-backed protocol test client, and offline pending operation queue. It provides the protocol and client-side offline storage foundations for the future P8 mobile clients.

### 1.1 Non-Goals for P5
- **No native mobile UI:** Mobile frontend (Android/iOS) is deferred to P8. P5 builds a disk-backed Go protocol test client/fixture harness.
- **No speech/model serving:** Regional ASR, middle model, and TTS belong to P6.
- **No operational route endorsement:** O05 remains open. Routes remain bound by evidence class (`SYNTHETIC_DEMO` vs `AUTHORIZED_OPERATIONAL`).
- **No production map licensing claims:** O06 remains open. P5 specifies the redistributable resource descriptor and completeness validation without claiming operational third-party map rights.

### 1.2 Core Invariants
1. **Public vs. Private Separation:** Public incident cards and manifests MUST NEVER contain private session tokens, citizen identifiers, reservations, stay records, or capacity holds. Public resources are cacheable; private resources carry `Cache-Control: no-store`.
2. **Standard HTTP Only:** No custom delta or chunk streaming protocols. All incremental downloads and resumption rely strictly on standard HTTP headers: `ETag`, `If-None-Match`, `Range`, `If-Range`, and `206 Partial Content`.
3. **Pending Does Not Mean Reserved:** Offline operations (holds, reservations, check-ins) queued while disconnected do NOT hold capacity. Capacity is decremented only upon atomic server-side commit when reconnected. If the hold window or snapshot expires before reconnection, the pending operation fails closed (`STALE_VERSION` / `EXPIRED`).
4. **Untrusted Client Clock:** Device clocks can drift, be tampered with, or roll back. Client validity windows MUST fail closed against monotonic elapsed time and server-asserted timestamps.
5. **Fail-Closed on Tampering or Revocation:** Corrupted signatures, unknown signer keys, mismatched digests, or tombstoned revisions immediately invalidate local active guidance. Old manifests cannot resurrect revoked routes.

---

## 2. Ownership & Dependency Boundaries

To allow five implementation agents to execute in parallel without merge conflicts, overlapping files, or circular imports, work is strictly partitioned into five isolated packages plus one integration role.

```
                    ┌────────────────────────┐
                    │      Agent 1:          │
                    │ backend/internal/      │
                    │ offlinepkg             │
                    │ (Types, Canonicalizer, │
                    │  Signatures, Trust)    │
                    └───────────┬────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│    Agent 2:      │  │    Agent 4:      │  │    Agent 3:      │
│ backend/internal/│  │ backend/internal/│  │ backend/internal/│
│ offlinedelivery  │  │ offlineresources │  │ offlineclient    │
│ (HTTP Handlers,  │  │ (Map/Style/Audio │  │ (Disk Storage,   │
│  Range, ETag)    │  │  Completeness &  │  │  Resumable Fetch,│
└────────┬─────────┘  │  Budget Checks)  │  │  Activation Swap)│
         │            └─────────┬────────┘  └─────────┬────────┘
         │                      │                     │
         │                      ▼                     │
         │            ┌──────────────────┐            │
         │            │    Agent 5:      │            │
         │            │ backend/internal/│            │
         │            │ offlinequeue     │            │
         │            │ (Pending Offline │            │
         │            │  Writes & Replay)│            │
         │            └─────────┬────────┘            │
         │                      │                     │
         └──────────────────────┼─────────────────────┘
                                │
                                ▼
                    ┌────────────────────────┐
                    │   Integration Agent:   │
                    │ backend/internal/      │
                    │ httpserver & tests     │
                    │ (Server Wiring,        │
                    │  OpenAPI, E2E Tests)   │
                    └────────────────────────┘
```

### 2.1 Ownership Matrix

| Agent | Target Package Directory | Primary Responsibilities | Permitted Imports |
| :--- | :--- | :--- | :--- |
| **Agent 1** | `backend/internal/offlinepkg/` | Public manifest, card, and resource data models; JSON canonicalization; SHA-256 digest computation; Ed25519 signature creation/verification; Pinned trust store; Structural validators. | Go standard library (`crypto/*`, `encoding/*`, `time`, etc.), `backend/internal/contracts`. |
| **Agent 2** | `backend/internal/offlinedelivery/` | HTTP handlers for manifest, card, and resource streaming; ETag generation; `If-None-Match` conditional handling; Standard RFC 9110 `Range` and `If-Range` slicing; Byte-budget enforcement. | Go standard library (`net/http`, `io`, etc.), `backend/internal/contracts`, `backend/internal/httpjson`, `backend/internal/offlinepkg`. |
| **Agent 3** | `backend/internal/offlineclient/` | Disk-backed protocol test client; Two-phase atomic downloads (`.part` staging -> atomic rename); Resumable download loop; Active version activation; Tombstone management; Clock-drift evaluator. | Go standard library (`os`, `path/filepath`, `net/http`, etc.), `backend/internal/offlinepkg`. |
| **Agent 4** | `backend/internal/offlineresources/` | Offline resource packaging checks; MapLibre style JSON dependency resolution (sprite, glyphs, tile endpoints); Regional budget audits (target ≤50 MiB); License/attribution presence check. | Go standard library (`encoding/json`, `archive/*`, etc.), `backend/internal/offlinepkg`. |
| **Agent 5** | `backend/internal/offlinequeue/` | Client-side durable pending operation queue; JSON file storage for offline reservation and event attempts; Monotonic sequencing; Server replay dispatcher; Hold expiration / stale version failure handler. | Go standard library (`os`, `sync`, `net/http`, etc.), `backend/internal/contracts`. |
| **Integration** | `backend/internal/httpserver/`, `backend/contracts/` | Mounts delivery routes in `Server`; DB projection adapter (`store` -> `PublicationSource`); Updates `openapi.yaml`; End-to-end integration tests under network throttling. | All internal packages. |

### 2.2 Invariant: No Circular Dependencies
- `offlinepkg` has ZERO internal dependencies other than `contracts`.
- `offlinedelivery`, `offlineclient`, and `offlineresources` import `offlinepkg`, but NEVER import each other.
- `offlinequeue` depends only on `contracts` and standard library.
- Integration Agent wires components together at `httpserver` and integration test layers.

### 2.3 Stdlib-Only and No Shared Mutable Globals
- **Stdlib only.** All five agents implement against the Go standard library only (`crypto/ed25519`, `crypto/sha256`, `encoding/json`, `net/http`, `os`, `path/filepath`, `archive/*`, `compress/gzip`, `sync`, `time`, `log/slog`, etc.). No new third-party modules are added by P5; the integration agent adds **nothing** to `backend/go.mod` or `backend/go.sum` for P5 work. If an agent finds a real need for an external module it returns a blocker rather than silently adding a dependency.
- **No shared mutable globals.** Each package's state lives behind explicit values passed at construction (e.g., `NewClient(cfg)`, `NewHandler(src, logger)`, `NewReplayWorker(store, dispatcher, now)`) or behind per-request scopes. Nothing in `offlinepkg`/`offlinedelivery`/`offlineclient`/`offlineresources`/`offlinequeue` declares a package-level mutable variable (no `var foo = ...` initialization). Tests construct fresh instances per case; clock is injected (`now func() time.Time`) rather than read directly from `time.Now`.

---

## 3. Wire Formats & Schemas

All P5 public HTTP payloads are UTF-8 encoded JSON objects, wrapped in `backend/internal/contracts/envelope.go` envelopes where dynamic API envelopes apply, or served as raw canonical JSON artifacts when fetched as immutable/versioned files.

### 3.1 Public Region Manifest (`GET /api/v3/regions/{id}/manifest`)

The manifest is the root discovery object for a region/jurisdiction. It is small, frequently revalidated, and cryptographically signed.

```json
{
  "schema_version": "3.0",
  "manifest_id": "MAN-KL-2026-09-20-01",
  "jurisdiction": "KL",
  "revision": 4,
  "generated_at": "2026-09-20T07:00:00Z",
  "valid_until": "2026-09-20T09:00:00Z",
  "source_status": "CURRENT",
  "critical_card": {
    "package_id": "PKG-WAYANAD-2026-V4",
    "version": 4,
    "uri": "/api/v3/packages/PKG-WAYANAD-2026-V4/versions/4",
    "checksum_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "uncompressed_bytes": 48120,
    "compressed_bytes": 12450,
    "content_type": "application/json"
  },
  "resources": [
    {
      "resource_id": "res-map-wayanad-vector-v1",
      "type": "VECTOR_TILES",
      "uri": "/api/v3/resources/res-map-wayanad-vector-v1",
      "checksum_sha256": "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
      "byte_size": 38450112,
      "content_type": "application/vnd.mapbox-vector-tile",
      "required": false,
      "attribution": "Synthetic fixture test pack; O06 pending official redistribution license."
    },
    {
      "resource_id": "res-style-wayanad-v1",
      "type": "MAP_STYLE",
      "uri": "/api/v3/resources/res-style-wayanad-v1",
      "checksum_sha256": "9b1e...c421",
      "byte_size": 65536,
      "content_type": "application/json",
      "required": false,
      "attribution": "OpenMapTiles / MapLibre style JSON specification."
    }
  ],
  "revocations": {
    "revoked_packages": ["PKG-WAYANAD-2026-V2"],
    "cancelled_routes": ["RT-WAYANAD-003"],
    "superseded_versions": [
      { "package_id": "PKG-WAYANAD-2026-V3", "version": 3 }
    ]
  },
  "provenance": {
    "authority": "kerala.sdma.gov.in",
    "dataset_id": "DS-KL-2026-09",
    "evidence_class": "SYNTHETIC_DEMO"
  },
  "checksum_sha256": "a82f...912a",
  "signature": {
    "algorithm": "Ed25519",
    "key_id": "kpub:KL:sdma:2026:01",
    "value": "base64EncodedSignatureString=="
  }
}
```

### 3.2 Public Incident Card (`GET /api/v3/packages/{id}/versions/{version}`)

The Incident Card is the minimal, self-contained public projection of an active operational package. It contains ALL information required for citizen evacuation, safe destination lookup, and instruction display while completely disconnected.

```json
{
  "schema_version": "3.0",
  "package_id": "PKG-WAYANAD-2026-V4",
  "version": 4,
  "jurisdiction": "KL",
  "evidence_class": "SYNTHETIC_DEMO",
  "effective_at": "2026-09-20T06:00:00Z",
  "expires_at": "2026-09-21T06:00:00Z",
  "alert": {
    "identifier": "ALERT-KL-2026-09-20-FLOOD-01",
    "sender": "kerala.sdma.gov.in",
    "headline": "Flash Flood & Landslide Warning — Meppadi / Chooralmala",
    "severity": "Severe",
    "urgency": "Immediate",
    "certainty": "Observed",
    "area_description": "Meppadi Panchayath, Vythiri Taluk, Wayanad"
  },
  "red_zones": [
    {
      "id": "RZ-MEPPADI-01",
      "name": "Chooralmala Riverine Hazard Zone",
      "centroid": [76.1234, 11.5432],
      "geometry": {
        "type": "Polygon",
        "coordinates": [[[76.12, 11.54], [76.13, 11.54], [76.13, 11.55], [76.12, 11.55], [76.12, 11.54]]]
      }
    }
  ],
  "safe_zones": [
    {
      "id": "SZ-MEPPADI-SCHOOL",
      "name": "St. Joseph GHSS Relief Centre",
      "role": "EMERGENCY_SHELTER",
      "status": "OPEN",
      "capacity_mode": "DEFINED",
      "total_capacity": 350,
      "location": [76.1450, 11.5600],
      "services": ["DRINKING_WATER", "MEDICAL_FIRST_AID", "SANITATION"]
    }
  ],
  "approved_routes": [
    {
      "id": "RT-WAYANAD-001",
      "from_zone_id": "RZ-MEPPADI-01",
      "to_safe_zone_id": "SZ-MEPPADI-SCHOOL",
      "mode": "FOOT",
      "approval": "SYNTHETIC_DEMO",
      "verified_by": "officer.sdma.kl",
      "verified_at": "2026-09-20T06:30:00Z",
      "valid_from": "2026-09-20T06:30:00Z",
      "valid_until": "2026-09-21T06:00:00Z",
      "geometry": {
        "type": "LineString",
        "coordinates": [[76.125, 11.545], [76.135, 11.552], [76.145, 11.560]]
      }
    }
  ],
  "facilities": [
    {
      "id": "FAC-SZ-01",
      "safe_zone_id": "SZ-MEPPADI-SCHOOL",
      "name": "Main Auditorium Shelter",
      "address": "Meppadi P.O., Wayanad",
      "contact_phone": "04936202201"
    }
  ],
  "instructions": [
    {
      "id": "INS-KL-001",
      "language": "ml-IN",
      "title": "ഉടൻ സുരക്ഷിത സ്ഥാനത്തേക്ക് മാറുക",
      "summary": "ചൂരൽമല പുഴയോരത്ത് താമസിക്കുന്നവർ ഉടൻ സെന്റ് ജോസഫ് സ്കൂളിലെ ദുരിതാശ്വാസ ക്യാമ്പിലേക്ക് മാറുക."
    },
    {
      "id": "INS-EN-001",
      "language": "en-IN",
      "title": "Immediate Evacuation Advisory",
      "summary": "Residents near Chooralmala river basin must relocate immediately to St. Joseph School Shelter."
    }
  ],
  "emergency_contacts": [
    { "name": "District Disaster Control Room Wayanad", "number": "1077" },
    { "name": "Emergency Police / Fire / Ambulance", "number": "112" }
  ],
  "allocation_policy": {
    "order": ["SZ-MEPPADI-SCHOOL"],
    "reservation_expiry_seconds": 3600,
    "temporary_stay_min_days": 7,
    "temporary_stay_max_days": 30,
    "allow_walk_ins": true,
    "allow_transfers": false,
    "route_required": true
  },
  "checksum_sha256": "d4e2...81a0",
  "signature": {
    "algorithm": "Ed25519",
    "key_id": "kpub:KL:sdma:2026:01",
    "value": "base64EncodedSignatureString=="
  }
}
```

### 3.3 Resource Descriptor Specification

Resources are opaque binary or static structured assets referenced by the manifest.
- **Resource Types:** `VECTOR_TILES`, `MAP_STYLE`, `MAP_SPRITE`, `MAP_GLYPHS`, `GAZETTEER`, `EMERGENCY_AUDIO`.
- **Target Size Limits:**
  - Critical Card: **≤ 64 KiB** compressed.
  - Optional Regional Map Pack: **≤ 50 MiB** cumulative budget per district/incident region.
  - Single Audio Asset: **≤ 512 KiB** compressed.

---

## 4. Cryptographic Specification & Key Management

### 4.1 Signature Algorithm: Ed25519
All manifest and card signatures MUST use **Ed25519** (RFC 8032 / FIPS 186-5) over the SHA-256 hash or directly over canonical bytes.
- Signature field format:
  - `algorithm`: Exactly `"Ed25519"`.
  - `key_id`: String identifier (e.g., `"kpub:KL:sdma:2026:01"`).
  - `value`: Standard Base64 encoding (RFC 4648 §4) of the 64-byte Ed25519 raw signature.

### 4.2 Deterministic Canonical Bytes & Checksum
The exact bytes signed and checksummed are produced by deterministic JSON canonicalization:
1. Strip mutable integrity metadata:
   - For `Manifest`: set `checksum_sha256 = ""` and `signature = nil`.
   - For `PublicIncidentCard`: set `checksum_sha256 = ""` and `signature = nil`.
2. Format canonical JSON matching `backend/internal/opkg/package.go:canonicalJSON`:
   - Keys recursively sorted in lexicographical ASCII order.
   - Compact delimiters (`:` and `,` with no trailing/leading whitespace).
   - Standard UTF-8 character encoding with standard JSON string escaping.
   - No floating-point normalization differences: coordinate arrays format consistently.
3. Integrity checksum:
   $$\text{checksum\_sha256} = \text{hex}(\text{SHA-256}(\text{canonicalBytes}))$$
4. Signature computation:
   $$\text{sig} = \text{ed25519.Sign}(\text{privateKey}, \text{canonicalBytes})$$

### 4.3 Pinned Trust Store & Authorization Scoping
Integrity $\neq$ Authority. A cryptographically valid signature is rejected if the signing key is unauthorized.
- **Trust Store Configuration:**
  ```go
  type TrustedKey struct {
      KeyID                 string
      PublicKey             ed25519.PublicKey
      PermittedJurisdiction string    // e.g. "KL"; "*" permitted ONLY in synthetic test harnesses
      ValidFrom             time.Time
      ValidUntil            time.Time
      Revoked               bool
  }
  ```
- **Jurisdiction Boundary:** A key authorized for `"KL"` CANNOT sign manifests or packages for `"TN"`. Any jurisdiction mismatch returns `ErrSignerUnauthorized`.

---

## 5. Content Identifiers, Expiry, Revision & Revocation

### 5.1 Content Identifiers & Digest Encoding
- **Uniform Digest Encoding:** `sha256:<64-char lowercase hex>` across manifests, cards, resources, and ETags.
- **Package Snapshot URI:** `/api/v3/packages/{package_id}/versions/{version}`. Immutable once published.

### 5.2 Expiry Semantics & Untrusted Client Clocks
1. **Timestamp Representation:** All temporal bounds (`generated_at`, `valid_until`, `effective_at`, `expires_at`) use RFC 3339 UTC strings (`YYYY-MM-DDTHH:MM:SSZ`).
2. **Untrusted Clock Defense:**
   - When a client receives a manifest, it records the manifest's `generated_at` time against the local client clock and tracks local monotonic elapsed time ($\Delta t = \text{time.Since}(\text{fetchTime})$).
   - If the client system clock jumps backwards ($\text{now} < \text{fetchTime}$), clock tampering is flagged and expiry is computed using monotonic uptime.
   - If the maximum offline duration exceeds `expires_at`, guidance transitions to `EXPIRED`.

### 5.3 Monotonic Revision & Rollback Prevention
- Each region manifest contains a strictly increasing integer `revision`.
- Clients store `last_activated_revision`. If a downloaded manifest has $\text{revision} < \text{last\_activated\_revision}$, it MUST be rejected immediately (`ErrVersionRollback`). Replay of older manifests is prohibited.

### 5.4 Revocation & Durable Tombstones
1. The manifest's `revocations` block lists `revoked_packages` and `cancelled_routes`.
2. When a signed manifest is verified, the client immediately records durable tombstones in disk storage:
   - Any active package in `revoked_packages` is marked `REVOKED` and removed from active guidance.
   - Any route in `cancelled_routes` is removed from active route options.
3. Tombstones persist across app restarts and network loss; an older cached package cannot be re-activated once tombstoned.

---

## 6. HTTP Delivery Protocol & OpenAPI Reconciliation

All delivery paths operate under the `/api/v3` server prefix defined in `backend/contracts/openapi.yaml`.

### 6.1 Proposed Public Delivery Endpoints

| HTTP Path | Method | Auth | Cache Header | ETag | Range / Partial |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `/api/v3/regions/{id}/manifest` | GET, HEAD | Public | `public, max-age=60, must-revalidate` | Strong (`"sha256-..."`) | No |
| `/api/v3/packages/{id}/versions/{version}` | GET, HEAD | Public | `public, max-age=31536000, immutable` | Strong (`"sha256-..."`) | Yes (`206`, `If-Range`) |
| `/api/v3/resources/{id}` | GET, HEAD | Public | `public, max-age=31536000, immutable` | Strong (`"sha256-..."`) | Yes (`206`, `If-Range`) |

### 6.2 Caching & Conditional Requests
- **ETag Construction:** `ETag: "sha256-<checksum_sha256>"`.
- **Conditional GET:** If client sends `If-None-Match: "sha256-<checksum>"`, server returns `304 Not Modified` with an empty body and preserved caching headers.

### 6.3 Resumable Downloads (Range & If-Range)
- Endpoints serving packages and resources support RFC 9110 byte ranges:
  - Header `Accept-Ranges: bytes` emitted on all `200 OK` responses.
  - Header `Range: bytes=start-end`, `Range: bytes=start-`, or `Range: bytes=-suffix`.
  - On valid range: returns `206 Partial Content` with `Content-Range: bytes start-end/total`.
  - Header `If-Range: "sha256-<checksum>"`: If ETag matches, return `206 Partial Content`; if ETag drifted, return full `200 OK` with complete new entity.
  - Unsatisfiable range returns `416 Range Not Satisfiable` with `Content-Range: bytes */total`.

### 6.4 Error Representation
Errors adhere to `backend/internal/contracts/envelope.go`:
```json
{
  "request_id": "req-019283aa",
  "schema_version": "3.0",
  "generated_at": "2026-09-20T07:15:00Z",
  "data_version": "none",
  "source_status": "UNKNOWN",
  "errors": [
    {
      "code": "NOT_FOUND",
      "message": "resource res-map-wayanad-vector-v1 not found",
      "field": "id",
      "correlation_id": "req-019283aa",
      "retryable": false
    }
  ]
}
```

### 6.5 OpenAPI Reconciliation
The existing `backend/contracts/openapi.yaml` documents the P1/P4 `/api/v3` slice — `health/{live,ready}`, `voice/commands`, `sessions`, `places/resolve`, `guidance/query`, `reservations[/...]`, `operations/...` — plus the `Envelope`, `APIError`, `VoiceCommandRequest`, `ModelOutput` and `Action` schemas (lines 22–549 at CLEAN `4b2b08f`). P5 APPENDS three new path groups under the same `/api/v3` prefix:

- `/regions/{id}/manifest`
- `/packages/{id}/versions/{version}`
- `/resources/{id}`

The integration agent writes a strict **additive** diff: no existing path, schema, error code, parameter or response is changed. New paths and their request/response schemas appear in the `paths:` and `components/schemas:` blocks alongside the existing entries; the document's top-level `openapi`, `info.version`, `info.description` and the `servers` entry are untouched. The same `Envelope`/`APIError` shapes defined by `backend/internal/contracts/envelope.go` are reused, not redefined. Manifest and card responses are documented as **raw JSON artifacts** (not wrapped in `Envelope`) because they are immutable versioned files; manifest HTTP errors and resource 4xx/5xx paths use the standard `Envelope`/`APIError` shapes.

---

## 7. Minimal Go Types & Exported Function Signatures

These exported contracts define the seams between parallel implementation agents. Agents must implement these exact types without modifications.

### 7.1 Agent 1: `backend/internal/offlinepkg`

```go
package offlinepkg

import (
	"crypto/ed25519"
	"errors"
	"time"
)

var (
	ErrInvalidSignature     = errors.New("offlinepkg: invalid digital signature")
	ErrSignerUnauthorized   = errors.New("offlinepkg: signing key not authorized for jurisdiction")
	ErrKeyExpired            = errors.New("offlinepkg: signing key is expired or not yet valid")
	ErrChecksumMismatch     = errors.New("offlinepkg: checksum verification failed")
	ErrVersionRollback      = errors.New("offlinepkg: manifest revision is older than active revision")
	ErrMalformedData        = errors.New("offlinepkg: data failed structural validation")
)

type ResourceType string

const (
	TypeVectorTiles   ResourceType = "VECTOR_TILES"
	TypeMapStyle      ResourceType = "MAP_STYLE"
	TypeMapSprite     ResourceType = "MAP_SPRITE"
	TypeMapGlyphs     ResourceType = "MAP_GLYPHS"
	TypeGazetteer     ResourceType = "GAZETTEER"
	TypeEmergencyAudio ResourceType = "EMERGENCY_AUDIO"
)

type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Value     string `json:"value"` // Base64 RFC 4648
}

type CriticalCardDescriptor struct {
	PackageID         string `json:"package_id"`
	Version           int    `json:"version"`
	URI               string `json:"uri"`
	ChecksumSHA256    string `json:"checksum_sha256"`
	UncompressedBytes int64  `json:"uncompressed_bytes"`
	CompressedBytes   int64  `json:"compressed_bytes"`
	ContentType       string `json:"content_type"`
}

type ResourceDescriptor struct {
	ResourceID     string       `json:"resource_id"`
	Type           ResourceType `json:"type"`
	URI            string       `json:"uri"`
	ChecksumSHA256 string       `json:"checksum_sha256"`
	ByteSize       int64        `json:"byte_size"`
	ContentType    string       `json:"content_type"`
	Required       bool         `json:"required"`
	Attribution    string       `json:"attribution"`
}

type RevocationBlock struct {
	RevokedPackages    []string             `json:"revoked_packages,omitempty"`
	CancelledRoutes    []string             `json:"cancelled_routes,omitempty"`
	SupersededVersions []SupersededVersion  `json:"superseded_versions,omitempty"`
}

type SupersededVersion struct {
	PackageID string `json:"package_id"`
	Version   int    `json:"version"`
}

type ManifestProvenance struct {
	Authority     string `json:"authority"`
	DatasetID     string `json:"dataset_id"`
	EvidenceClass string `json:"evidence_class"`
}

type Manifest struct {
	SchemaVersion  string                 `json:"schema_version"`
	ManifestID     string                 `json:"manifest_id"`
	Jurisdiction   string                 `json:"jurisdiction"`
	Revision       int                    `json:"revision"`
	GeneratedAt    string                 `json:"generated_at"`
	ValidUntil     string                 `json:"valid_until"`
	SourceStatus   string                 `json:"source_status"`
	CriticalCard   CriticalCardDescriptor `json:"critical_card"`
	Resources      []ResourceDescriptor   `json:"resources,omitempty"`
	Revocations    RevocationBlock        `json:"revocations"`
	Provenance     ManifestProvenance     `json:"provenance"`
	ChecksumSHA256 string                 `json:"checksum_sha256"`
	Signature      *Signature             `json:"signature,omitempty"`
}

type PublicIncidentCard struct {
	SchemaVersion     string             `json:"schema_version"`
	PackageID         string             `json:"package_id"`
	Version           int                `json:"version"`
	Jurisdiction      string             `json:"jurisdiction"`
	EvidenceClass     string             `json:"evidence_class"`
	EffectiveAt       string             `json:"effective_at"`
	ExpiresAt         string             `json:"expires_at"`
	Alert             AlertCard          `json:"alert"`
	RedZones          []RedZoneCard      `json:"red_zones"`
	SafeZones         []SafeZoneCard     `json:"safe_zones"`
	ApprovedRoutes    []RouteCard        `json:"approved_routes"`
	Facilities        []FacilityCard     `json:"facilities"`
	Instructions      []InstructionCard  `json:"instructions"`
	EmergencyContacts []EmergencyContact `json:"emergency_contacts"`
	AllocationPolicy  PolicyCard         `json:"allocation_policy"`
	ChecksumSHA256    string             `json:"checksum_sha256"`
	Signature         *Signature         `json:"signature,omitempty"`
}

type AlertCard struct {
	Identifier      string `json:"identifier"`
	Sender          string `json:"sender"`
	Headline        string `json:"headline"`
	Severity        string `json:"severity"`
	Urgency         string `json:"urgency"`
	Certainty       string `json:"certainty"`
	AreaDescription string `json:"area_description"`
}

type RedZoneCard struct {
	ID       string    `json:"id"`
	Name     string    `json:"name,omitempty"`
	Centroid []float64 `json:"centroid,omitempty"`
	Geometry any       `json:"geometry"`
}

type SafeZoneCard struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	Status        string    `json:"status"`
	CapacityMode  string    `json:"capacity_mode"`
	TotalCapacity *int      `json:"total_capacity,omitempty"`
	Location      []float64 `json:"location,omitempty"`
	Services      []string  `json:"services,omitempty"`
}

type RouteCard struct {
	ID           string    `json:"id"`
	FromZoneID   string    `json:"from_zone_id"`
	ToSafeZoneID string    `json:"to_safe_zone_id"`
	Mode         string    `json:"mode"`
	Approval     string    `json:"approval"`
	VerifiedBy   string    `json:"verified_by,omitempty"`
	VerifiedAt   string    `json:"verified_at,omitempty"`
	ValidFrom    string    `json:"valid_from,omitempty"`
	ValidUntil   string    `json:"valid_until,omitempty"`
	Geometry     any       `json:"geometry"`
}

type FacilityCard struct {
	ID           string `json:"id"`
	SafeZoneID   string `json:"safe_zone_id"`
	Name         string `json:"name"`
	Address      string `json:"address,omitempty"`
	ContactPhone string `json:"contact_phone,omitempty"`
}

type InstructionCard struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
}

type EmergencyContact struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

type PolicyCard struct {
	Order                    []string `json:"order"`
	ReservationExpirySeconds *int     `json:"reservation_expiry_seconds,omitempty"`
	TemporaryStayMinDays     *int     `json:"temporary_stay_min_days,omitempty"`
	TemporaryStayMaxDays     *int     `json:"temporary_stay_max_days,omitempty"`
	AllowWalkIns             *bool    `json:"allow_walk_ins,omitempty"`
	AllowTransfers           *bool    `json:"allow_transfers,omitempty"`
	RouteRequired            *bool    `json:"route_required,omitempty"`
}

type TrustedKey struct {
	KeyID                 string
	PublicKey             ed25519.PublicKey
	PermittedJurisdiction string
	ValidFrom             time.Time
	ValidUntil            time.Time
	Revoked               bool
}

type TrustStore interface {
	VerifySignature(keyID, jurisdiction string, canonicalBytes []byte, sigBase64 string) error
	LookupKey(keyID string) (TrustedKey, error)
}

func CanonicalBytes(v any) ([]byte, error)
func ChecksumSHA256(canonical []byte) string
func SignCanonical(privateKey ed25519.PrivateKey, keyID string, v any) (*Signature, error)
func ValidateManifestStructure(m *Manifest) error
func ValidateCardStructure(c *PublicIncidentCard) error
```

### 7.2 Agent 2: `backend/internal/offlinedelivery`

```go
package offlinedelivery

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"sthira/backend/internal/offlinepkg"
)

type ResourceContent struct {
	Reader        io.ReadSeekCloser
	ContentType   string
	ContentLength int64
	ChecksumSHA256 string
	ETag          string
}

type PublicationSource interface {
	GetManifest(ctx context.Context, jurisdiction string) (*offlinepkg.Manifest, []byte, error)
	GetCard(ctx context.Context, packageID string, version int) (*offlinepkg.PublicIncidentCard, []byte, error)
	GetResource(ctx context.Context, resourceID string) (*ResourceContent, error)
}

type Handler struct {
	src    PublicationSource
	logger *slog.Logger
}

func NewHandler(src PublicationSource, logger *slog.Logger) *Handler

// HTTP handler functions
func (h *Handler) HandleGetManifest(w http.ResponseWriter, r *http.Request)
func (h *Handler) HandleGetCard(w http.ResponseWriter, r *http.Request)
func (h *Handler) HandleGetResource(w http.ResponseWriter, r *http.Request)
```

### 7.3 Agent 3: `backend/internal/offlineclient`

```go
package offlineclient

import (
	"context"
	"net/http"
	"time"

	"sthira/backend/internal/offlinepkg"
)

type FreshnessState string

const (
	FreshnessCurrent FreshnessState = "CURRENT"
	FreshnessStale   FreshnessState = "STALE"
	FreshnessExpired FreshnessState = "EXPIRED"
	FreshnessRevoked FreshnessState = "REVOKED"
)

type ClientConfig struct {
	BaseURL    string
	StorageDir string
	HTTPClient *http.Client
	TrustStore offlinepkg.TrustStore
	Now        func() time.Time
}

type SyncReport struct {
	ManifestUpdated bool
	ActiveRevision  int
	CardUpdated     bool
	BytesDownloaded int64
	RevokedPackages []string
	CancelledRoutes []string
}

type ProtocolClient struct {
	// unexported fields
}

func NewClient(cfg ClientConfig) (*ProtocolClient, error)
func (c *ProtocolClient) Sync(ctx context.Context, jurisdiction string) (*SyncReport, error)
func (c *ProtocolClient) GetActiveCard() (*offlinepkg.PublicIncidentCard, FreshnessState, error)
func (c *ProtocolClient) IsRouteCancelled(routeID string) bool
func (c *ProtocolClient) DownloadResource(ctx context.Context, desc offlinepkg.ResourceDescriptor, resume bool) error
func (c *ProtocolClient) HasResource(resourceID string) (bool, error)
```

### 7.4 Agent 4: `backend/internal/offlineresources`

```go
package offlineresources

import (
	"context"

	"sthira/backend/internal/offlinepkg"
)

type StyleValidationResult struct {
	Valid          bool
	MissingSprites []string
	MissingGlyphs  []string
	MissingSources []string
}

type RegionalPackAudit struct {
	TotalBytes          int64
	BudgetLimitBytes    int64 // 50 MiB = 52,428,800 bytes
	ExceedsBudget       bool
	ResourceCount       int
	AttributionMissing  []string
}

type ResourceValidator interface {
	ValidateMapStyle(styleJSON []byte, availableResources []offlinepkg.ResourceDescriptor) (*StyleValidationResult, error)
	AuditRegionalPack(resources []offlinepkg.ResourceDescriptor) (*RegionalPackAudit, error)
}

func NewValidator() ResourceValidator
```

### 7.5 Agent 5: `backend/internal/offlinequeue`

```go
package offlinequeue

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrQueueEmpty        = errors.New("offlinequeue: queue is empty")
	ErrOperationConflict = errors.New("offlinequeue: duplicate idempotency key")
)

type OpState string

const (
	StatePending    OpState = "PENDING"
	StateInFlight   OpState = "IN_FLIGHT"
	StateCommitted  OpState = "COMMITTED"
	StateFailedStale OpState = "FAILED_STALE" // Snapshot drift or hold expired
	StateFailedPerm OpState = "FAILED_PERM"
)

type PendingOperation struct {
	ID              string          `json:"id"`
	CreatedAt       time.Time       `json:"created_at"`
	IdempotencyKey  string          `json:"idempotency_key"`
	Endpoint        string          `json:"endpoint"`
	BearerToken     string          `json:"bearer_token"` // Citizen session token
	Payload         json.RawMessage `json:"payload"`
	SnapshotVersion int             `json:"snapshot_version"`
	State           OpState         `json:"state"`
	RetryCount      int             `json:"retry_count"`
	LastError       string          `json:"last_error,omitempty"`
}

type QueueStore interface {
	Enqueue(ctx context.Context, op PendingOperation) error
	PeekPending(ctx context.Context) ([]PendingOperation, error)
	UpdateState(ctx context.Context, id string, state OpState, lastErr string) error
	PurgeCommitted(ctx context.Context, olderThan time.Duration) (int, error)
}

type HTTPDispatcher interface {
	PostJSON(ctx context.Context, endpoint string, token string, body []byte) (statusCode int, respBody []byte, err error)
}

type DrainReport struct {
	Processed int
	Committed int
	Stale     int
	Failed    int
}

type ReplayWorker struct {
	store      QueueStore
	dispatcher HTTPDispatcher
	now        func() time.Time
}

func NewReplayWorker(store QueueStore, dispatcher HTTPDispatcher, now func() time.Time) *ReplayWorker
func (w *ReplayWorker) DrainQueue(ctx context.Context) (*DrainReport, error)
```

---

## 8. Resource Constraints, Budgets & Acceptance Verification

### 8.1 Measured Resource Budgets
All implementations must measure and assert strict resource budgets:
1. **Critical Incident Card:** Compressed size (gzip, default compression) MUST be **$\le$ 64 KiB** ($65{,}536$ bytes).
2. **Optional Regional Map Pack:** Cumulative uncompressed size for a declared district/hazard pack (vector tiles + style + sprites + glyphs) MUST be **$\le$ 50 MiB** ($52{,}428{,}800$ bytes). This is an illustrative protocol budget, not a measured Wayanad pack.
3. **Emergency Audio Pack:** Compressed audio assets MUST be **$\le$ 512 KiB** per asset.

### 8.2 Network Simulation Profile
All offline download and resumption tests must be executed and verified under the standard constrained network profile:
- **Downlink bandwidth:** 400 kbit/s
- **Uplink bandwidth:** 128 kbit/s
- **Round-Trip Time (RTT):** 400 ms
- **Request-drop simulation:** 2% deterministic RoundTripper error rate; this is not kernel-level packet loss. A zero-drop run does not prove interruption recovery.
- **Complete Disconnect:** Simulating sudden network loss during active transfer.

### 8.3 Required Acceptance Test Matrix

| Test ID | Name | Tested Behavior & Failure Condition | Passing Evidence |
| :--- | :--- | :--- | :--- |
| **TC-P5-01** | `TestCardCompressedBudget` | Pack a realistic multi-zone Wayanad scenario card. Measure compressed byte size. | Assert size $\le 65{,}536$ bytes. |
| **TC-P5-02** | `TestResumableTransfer` | Download resource via Range requests. Intentionally terminate connection at 40%. Send `Range: bytes=N-` with `If-Range`. | Download completes, reassembled file matches expected SHA-256. |
| **TC-P5-03** | `TestCorruptedSignature` | Alter 1 byte in canonical card. Attempt client activation. | Client returns `ErrInvalidSignature`. Active store remains unchanged. |
| **TC-P5-04** | `TestUnauthorizedKey` | Sign valid card using an Ed25519 key authorized for `"TN"` while manifest jurisdiction is `"KL"`. | Client returns `ErrSignerUnauthorized`. Activation rejected. |
| **TC-P5-05** | `TestRevisionRollback` | Client has active revision 4. Server serves valid signed manifest with revision 3. | Client rejects with `ErrVersionRollback`. Active card remains at rev 4. |
| **TC-P5-06** | `TestRevocationTombstone` | Server advertises manifest with `cancelled_routes: ["RT-01"]`. | Client removes `RT-01`. Subsequent route query marks route cancelled. |
| **TC-P5-07** | `TestAtomicSwapCrash` | Client downloads new version into `.part` file. Interrupt before `os.Rename`. | Active directory retains pristine previous version with zero corruption. |
| **TC-P5-08** | `TestUntrustedClockRollback`| Client clock artificially set back 2 hours. Monotonic uptime checked. | Client does not extend card expiry past server-asserted `expires_at`. |
| **TC-P5-09** | `TestOfflineQueueStaleReplay`| Offline reservation held beyond policy TTL (1 hr). Network restores. | Server returns `STALE_VERSION` / `EXPIRED`. Queue marks `FAILED_STALE`. Zero capacity leaked. |
| **TC-P5-10** | `TestPublicCachePrivacyLeak`| Request all public delivery endpoints. Inspect headers and bodies. | No `Authorization` echo, no citizen session tokens, no reservations. `Cache-Control: public...`. |

---

## 9. Implementation Roadmap & Agent Instructions

### Execution Order for Parallel Agents
1. **Agent 1 (`offlinepkg`)** commits first. This produces the wire types, canonicalizer, Ed25519 verification, and trust store.
2. **Agents 2, 3, 4, 5** can then execute concurrently, using `offlinepkg` as their only shared internal dependency.
3. **Integration Agent** takes the committed packages from Agents 1–5, mounts the handlers in `backend/internal/httpserver/server.go`, binds the OpenAPI contract in `backend/contracts/openapi.yaml`, wires storage adapters, and runs the end-to-end acceptance suite.

---

## 10. Document Revision History

- **2026-09-20:** Initial P5 shared implementation contract created. Fixed interfaces, wire types, cryptography, and test matrix for Agents 1–5.
- **2026-09-20 (freeze):** Status set to REVIEWED CONTRACT (common baseline for Agents 1–5) at CLEAN `91a743c`. No technical changes.
- **2026-09-21:** Reopened as IN_PROGRESS after engineering review found reference-binding, freshness, activation, resume, publication, queue, and resource-integration gaps. The compact correction matrix and current evidence live in `plan/prompt.md`; P6 remains NOT_STARTED.
- **2026-09-20 (amend):** Added §2.3 (stdlib-only + no shared mutable globals) and §6.5 (additive OpenAPI reconciliation) to make requirements 4, 7 and 8 explicit. No other edits.
