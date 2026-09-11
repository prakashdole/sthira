---
title: Sthira Citizen Emergency Guidance — Technical Requirements
document_id: STHIRA-TRD
version: 2.0
status: Implementation baseline
as_of: 2026-09-11
---

# Technical Requirements

## 1. Requirement language

`SHALL` is mandatory for the baseline. Every official fact SHALL carry source, source identifier, effective/expiry time where supplied, ingestion time, version, and validation state.

## 2. Functional requirements

### Government data and alerts

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| FR-001 | Ingest SACHET CAP alerts using the published caching contract. | 200/304/update/cancel fixtures |
| FR-002 | Preserve raw CAP and normalized fields without changing official severity, certainty, urgency, area, or instruction meaning. | Round-trip fixture test |
| FR-003 | Accept only allow-listed government domains, credentials, files, and operator publishers. | Negative source tests |
| FR-004 | Import versioned government red zones, safe zones, routes, facility capacities, accessibility attributes, and instruction sets. | Signed fixture/import report |
| FR-005 | Quarantine invalid, out-of-area, expired, duplicate-conflicting, or untraceable inputs. | Validation suite |
| FR-006 | Show the active source, issue time, expiry, last refresh, and stale/unavailable state. | UI/API assertions |
| FR-007 | Enrich alerts only with compatible official observations such as IMD precipitation; label observation versus forecast and its time/unit. | Data-contract tests |

### Citizen emergency experience

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| FR-008 | Display hazard, official severity, affected area, concise approved instructions, and a clear next action. | Usability test |
| FR-009 | Request foreground location only after explanation and permission; provide manual place selection. | Permission-denied test |
| FR-010 | Display official red zones, assigned safe zone, and approved route without claiming Sthira predicted them. | Copy/provenance test |
| FR-011 | Provide non-map route steps and landmarks equivalent to the map. | Screen-reader/map-failure test |
| FR-012 | Provide English and configured regional-language content; never machine-translate authoritative instructions unless the authority approves that workflow. | Localization review |
| FR-013 | Provide approved ISL video/animation when available and label absence honestly. | Deaf-user review |
| FR-014 | Open the device dialler for 112 or configured official contact after explicit citizen confirmation. | Device integration test |
| FR-015 | Work in low-bandwidth mode with the latest valid cached alert, route card, and instructions. | Network-loss test |

### Voice and map

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| FR-016 | Transcribe supported Indic speech with self-hosted IndicConformer-600M-multilingual. | Per-language benchmark |
| FR-017 | Parse transcripts through an allow-listed deterministic command grammar. | Command corpus test |
| FR-018 | Support focus, pan, zoom, recenter, alert area, safe zone, route, repeat, language, and emergency-call-open intents. | End-to-end voice tests |
| FR-019 | Ask for clarification below the approved confidence threshold and offer visible controls at all times. | Noise/ambiguity tests |
| FR-020 | Use self-hosted Indic Parler-TTS only for verified supported languages; cache approved speech and provide fallback. | Model/license/runtime matrix |
| FR-021 | Voice SHALL NOT confirm arrival, change capacity, silently call, or override an official assignment. | Adversarial tests |

### Assignment, arrival, capacity

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| FR-022 | Assign only an open, officially published safe zone with an active approved route and adequate reservable capacity. | Eligibility tests |
| FR-023 | Use the government-supplied allocation priority/version; if absent, return `ASSIGNMENT_UNAVAILABLE`. | Missing-policy test |
| FR-024 | Capture party size explicitly and expose whether capacity is reserved or only informational. | UI/API test |
| FR-025 | Ask `Have you and your party reached <safe zone>?` in the chosen language. | Localization test |
| FR-026 | On explicit `YES`, create one idempotent arrival event and reduce remaining capacity exactly once by confirmed party size. | Retry/concurrency test |
| FR-027 | On `NO`, retain guidance and offer rerouting help or emergency call without decrementing capacity. | State-machine test |
| FR-028 | Never let remaining capacity fall below zero; return a conflict and request reassignment. | Parallel-write test |
| FR-029 | Permit only authorized government corrections to total capacity, closure state, routes, and arrival records. | Authorization/audit test |
| FR-030 | Do not implement automatic/geofenced arrival in baseline. | Code/config inspection |

### Operations and audit

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| FR-031 | Provide an operator view for feed health, stale data, quarantines, active alerts, facility state, and capacity conflicts. | Operator scenario |
| FR-032 | Record source-to-screen render version and every assignment/capacity transition in tamper-evident audit. | Audit reconstruction |
| FR-033 | Allow an authority to publish a correction/cancellation that reaches clients and invalidates superseded routes. | Correction drill |
| FR-034 | Separate demo, shadow, pilot, and production data and presentation. | Isolation tests |

## 3. API contracts

Minimal endpoints:

```text
GET  /v2/alerts/active?lat=&lon=&lang=
GET  /v2/alerts/{id}
GET  /v2/guidance/{alert_id}?session_id=
POST /v2/assignments
GET  /v2/assignments/{id}
POST /v2/assignments/{id}/arrival-confirmations
POST /v2/voice/transcriptions
POST /v2/voice/commands/parse
POST /v2/speech
GET  /v2/operations/feed-health
POST /v2/operations/packages
```

Every response envelope includes `request_id`, `generated_at`, `data_version`, and `source_status`. Errors use stable codes and safe localized copy.

## 4. State machines

```text
Alert: RECEIVED -> VALIDATED -> ACTIVE -> EXPIRED | CANCELLED | SUPERSEDED
Safe zone: DRAFT -> PUBLISHED -> OPEN -> FULL | CLOSED -> REOPENED
Assignment: CREATED -> RESERVED -> ARRIVED | DECLINED | EXPIRED | CANCELLED
Voice command: CAPTURED -> TRANSCRIBED -> PARSED -> CONFIRMED? -> EXECUTED | REJECTED
```

Illegal transitions return a conflict and create no side effect.

## 5. Non-functional requirements

- NFR-001: WCAG 2.2 AA and applicable GIGW 3.0 checks; keyboard, screen reader, contrast, captions, reduced motion, and 200% zoom.
- NFR-002: Alert/guidance API 99.9% monthly target during pilot activation, subject to approved operations agreement.
- NFR-003: Cached emergency screen usable within 3 seconds at p95 on the defined low-end device/network profile.
- NFR-004: Capacity writes are ACID, idempotent, and recoverable; RPO 0 for committed ledger events.
- NFR-005: Government-source polling applies backoff, jitter, ETag/Last-Modified where supported, and provider rate limits.
- NFR-006: No private telemetry, advertising, private map tiles, or external cloud speech API in the production emergency path.
- NFR-007: Raw voice is deleted immediately after successful/failed processing unless explicit, separately approved diagnostic consent exists.
- NFR-008: Location precision and retention are minimized and visible to the citizen.
- NFR-009: All externally displayed times include timezone and freshness.
- NFR-010: Security testing covers OWASP ASVS/API risks, imported-content injection, SSRF, XML parser hardening, replay, and authorization.
- NFR-011: English/regional-language/ISL content receives human emergency-domain review before publication.
- NFR-012: Every critical dependency has a manual fallback and named operational owner.

## 6. Release gates

No public pilot until: official source permission is documented; feed samples pass; DDMA publishes safe zones/routes/capacity and allocation policy; 112 behavior is device-tested; language and ISL content is approved; ASR/TTS benchmarks meet thresholds; offline/stale/conflict drills pass; privacy/security reviews close; and a 24×7 government escalation path exists.
