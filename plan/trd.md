# Technical requirements and migration contracts

2026-09-19 · Proposed Go target. `SHALL` describes acceptance requirements, not existing APIs. Existing Python routes are `/api/v2/...`; inspect implementation/OpenAPI instead of copying old documentation paths.

## Requirements and evidence

| ID | SHALL behavior | Evidence / phase |
| --- | --- | --- |
| T01 | One Go application boundary with bounded HTTP requests/timeouts, safe errors, shutdown and separate liveness/readiness | P1 HTTP/contract tests |
| T02 | Strict typed contracts preserve source, jurisdiction, version, times, evidence class and unknown values | P1/P2 malformed/round-trip tests |
| T03 | CAP raw retention, 200/304 cache rules, sender/identifier/reference lifecycle, update/cancel and quarantine | P2 recorded transport fixtures |
| T04 | Import/publish/revoke signed operational packages with cross-reference, CRS, geometry, temporal and scope validation | P2/P3 fixture and real DB tests |
| T05 | Durable source facts, identity scope, audit/outbox and atomic capacity transactions | P3 multi-process DB/concurrency tests |
| T06 | Eligible destination choices and route preview separate from explicit reservation; never model authority | P4 domain/API tests |
| T07 | Immediate and temporary stays with correct capacity across dates, arrival, expiry, departure, transfer and extension | P4 conservation/restart tests |
| T08 | Small versioned regional packages, signed manifest, revocation and interrupted-download recovery | P5 offline protocol tests |
| T09 | Server speech pipeline with per-language benchmarks and independently validated middle-model output | P6 real inference + adversarial corpus |
| T10 | Mobile-native/local assets, both-platform offline maps, voice/chat/touch/screen-reader parity | P8/P9 real-device flow |
| T11 | Threat model, jurisdiction/session access control, secret isolation, supply-chain and active testing | P7/P9 assurance report |
| T12 | Measured normal/surge/cold-cache/hotspot performance and recovery, no government call amplification | P7/P9 load/restore evidence |
| T13 | 10–15 state catalogue with 2–3 sourced scenarios each and correct synthetic geometry labels | P2/P9 manifest review |
| T14 | No legacy/prototype runtime in final artifacts; preserved contracts/evidence and clean install | P10 package/route/dependency check |
| T15 | Authorized real-source integration, shadow replay and local operational sign-off | P11/P12 external gate |

## Domain record boundaries

Use stable opaque IDs scoped by jurisdiction and source. Store UTC instants with explicit timezone; temporary stay dates use facility local-date semantics plus the facility timezone. Geometry declares EPSG:4326 GeoJSON longitude/latitude order at the API boundary; perform validated CRS conversion at ingestion if needed. Do not confuse CAP polygon latitude/longitude ordering with GeoJSON.

- **Source artifact/state:** official owner, raw artifact digest/location, media type, received time, permitted use, schema and activation evidence.
- **Alert version:** source identifier/sender, CAP status/type/scope/references, issued/effective/expiry, severity/urgency/certainty, area and approved instructions. Non-Public/Exercise/Test records cannot become live Public guidance.
- **Incident package:** package ID/version, jurisdiction, evidence class, source references, effective/expiry window, signing key ID/signature and payload digest. A verified digest alone does not establish authority.
- **Zone/facility:** source version, geometry, role (assembly, emergency shelter, temporary accommodation), open/closed state, verified suitability/services/accessibility, contacts and capacity mode. Nullable fields stay unknown.
- **Route:** incident, source/version, origin/destination, mode, geometry, ordered landmarks, verified length when supplied, restrictions, verification actor/time, validity, closures and authority evidence. Candidate status cannot pass operational guidance gates.
- **Policy:** government-owned eligibility/choice restrictions, reservation expiry, temporary dates/extensions/transfers/walk-ins, capacity rules and conflict precedence. No inferred default.
- **Reservation/stay:** private session ownership, party size, selected facility/route/source snapshot, mode, dates, status and policy version.
- **Capacity event:** scoped idempotency key, payload hash, request/result, affected facility/date rows, before/after conservation values, actor, time and source/policy version.
- **Audit/outbox:** transactionally persisted change, minimal actor reference, object/version and outcome. Protect append-only permissions and external checkpoints; a hash chain alone is not proof against a privileged rewrite.
- **Language/voice:** approved language IDs, model/artifact revision, bounded transient transcript, intent/actions/template IDs, validation outcome and timings. No raw audio in persistent application state.

## API migration strategy

Freeze the actual `/api/v2` contract as a reference. Introduce a **proposed `/api/v3` contract** for the Go destination-choice/stay model; finalize its schemas in P1. Keep v2 only for the old demo client during migration. Do not silently replace v2 response shapes or duplicate every legacy endpoint. P10 retires compatibility after both new clients pass.

| Proposed path | Purpose / authorization |
| --- | --- |
| `GET /health/live`, `GET /health/ready` | Process health versus dependency/operational state; redact internals |
| `GET /api/v3/regions/{id}/manifest` | Public signed active-version manifest with cache policy |
| `GET /api/v3/packages/{id}/versions/{version}` | Immutable public incident projection; no personal assignment data |
| `POST /api/v3/places/resolve` | Bounded transient location/name query; return scoped candidates or ambiguity |
| `POST /api/v3/guidance/query` | Read eligible known destinations/routes for an incident and stated needs |
| `POST /api/v3/sessions` | Minimal private session/capability creation; no mandatory civil identity |
| `POST /api/v3/reservations` | Explicit confirmed choice; session auth, current snapshot, idempotency key |
| `GET /api/v3/reservations/{id}` | Owner/operator-authorized state only; private no-store |
| `POST /api/v3/reservations/{id}/events` | Typed explicit arrival/cancel/depart/extend/transfer request; same authorization and idempotency rules |
| `POST /api/v3/voice/transcriptions` | Bounded binary audio + declared language; content/duration validation |
| `POST /api/v3/voice/commands` | Typed chat/transcript + snapshot context; read/display proposals only |
| `POST /api/v3/voice/speech` | Validated response/template reference, not arbitrary user TTS text |
| `/api/v3/operations/...` | Scoped import, review, publish, revoke, correction, quarantine and feed health; MFA/operator authorization |

P1 specifies exact schemas/statuses; these are not existing endpoints. JSON payloads are sufficient; do not add GraphQL/gRPC solely for scale. Public resources support conditional requests. Sensitive content never enters public caches or query-string tokens. Sessions/authorization travel via a reviewed platform-safe mechanism, not caller-selected IDs alone.

Response envelope: `request_id`, `schema_version`, `generated_at`, `data_version`, `source_status`, `data` or stable `error.code`, and retry guidance when safe. Unknown/failure is distinct from empty/current/zero. Errors cover `AMBIGUOUS_PLACE`, `DATA_UNAVAILABLE`, `STALE_VERSION`, `ROUTE_UNVERIFIED`, `CAPACITY_UNKNOWN`, `CAPACITY_CONFLICT`, `IDEMPOTENCY_CONFLICT`, `LANGUAGE_UNSUPPORTED`, `MODEL_UNAVAILABLE`, `FORBIDDEN` and `RATE_LIMITED`. Bind server validation to the current snapshot, not just the model's echoed version.

## State and concurrency

```text
Source: DISCOVERED → ACCESS_REQUESTED → SAMPLE_ACQUIRED → VALIDATED → AUTHORIZED → OPERATIONAL
        OPERATIONAL → SUSPENDED or RETIRED
Package: RECEIVED → VALIDATED → PUBLISHED → SUPERSEDED / REVOKED / EXPIRED
Choice: PREVIEW → EXPLICIT_CONFIRMATION → RESERVED or INFORMATIONAL / CONFLICT
Stay: RESERVED → ARRIVED → DEPARTED; expiry/cancel before arrival; transfer/extension are audited transitions
Voice: CAPTURED → TRANSCRIBED → PROPOSED → VALIDATED → DISPLAYED; CLARIFY/REJECT/CANCEL have no unsafe side effects
```

Use SQL constraints, uniqueness, transactions and row locks/atomic conditional updates. Scope idempotency to actor/session + operation + key and bind payload/version. A lost response after commit must be retrievable with the same key. Define retention long enough for the supported retry window. Transaction expiry workers race safely with arrival/transfer; process-local locks alone are insufficient.

## Import and deployment safety

Bound JSON/XML nesting, duplicate keys, body/decompressed sizes, timeouts and geometry complexity. Reject DTD/entities and unsupported CRS/invalid coordinates; retain rejected artifacts in restricted quarantine with reasons. Do not allow arbitrary remote resource URLs from CAP/package fields to trigger fetches. Model weights requiring remote Python code need pinned review and sandboxed runtime.

P3 verifies actual migrations/PostGIS on the intended database, not SQLite substitutes. P7 verifies network segmentation, least privilege, restore and correct readiness probes. P10 runs a clean install/build without local caches/secrets/model downloads during boot. Production is a release/activation gate, not an environment string that bypasses missing source evidence.
