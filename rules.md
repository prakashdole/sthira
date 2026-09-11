---
title: Sthira Citizen Emergency Guidance — Normative Rules
document_id: STHIRA-RULES
version: 2.0
status: Normative
as_of: 2026-09-11
---

# Normative Rules

## Authority and truth

- **RUL-001:** Sthira is a citizen interface, not a disaster-prediction or statutory decision system.
- **RUL-002:** Only an authorized government source may originate operational alerts, red zones, safe zones, routes, capacities, closures, or emergency instructions.
- **RUL-003:** Software-derived transformations never acquire higher authority than their source.
- **RUL-004:** Every displayed operational fact identifies source, version, issue/effective time, expiry where supplied, and last refresh.
- **RUL-005:** Missing, invalid, expired, conflicting, or out-of-coverage data is shown as unavailable/uncertain, never silently filled.
- **RUL-006:** CAP updates and cancellations supersede prior displays according to CAP references and authority policy.
- **RUL-007:** Supporting observations may add context but cannot overwrite an incident instruction.
- **RUL-008:** Sentinel or other non-government/private data is excluded from the production decision path.

## Zones, routes, and assignment

- **RUL-009:** Sthira never predicts a red zone or declares a location safe.
- **RUL-010:** A safe zone must be pre-identified and published by the competent authority.
- **RUL-011:** An evacuation route must be authority-approved and active for the incident/version.
- **RUL-012:** Assignment uses only the authority-supplied allocation policy and eligible facilities.
- **RUL-013:** No facility is promised when capacity state is missing, stale, closed, or inadequate.
- **RUL-014:** A map visualization is advisory presentation of official data, not an independent safety assessment.
- **RUL-015:** Every map route has equivalent textual/landmark instructions.

## Capacity and arrival

- **RUL-016:** Party size is explicit; the interface never assumes one person.
- **RUL-017:** Remaining capacity is derived from official total, authorized adjustments, live reservations, and confirmed occupancy.
- **RUL-018:** Capacity writes are atomic, idempotent, auditable, and cannot produce a negative balance.
- **RUL-019:** Baseline arrival requires a visible/touch confirmation by the citizen; voice alone cannot confirm it.
- **RUL-020:** `YES` decrements availability exactly once by confirmed party size.
- **RUL-021:** `NO`, dismiss, timeout, or network failure never decrements confirmed occupancy.
- **RUL-022:** A government correction is an additive compensating event, not destructive ledger editing.
- **RUL-023:** Automatic geofence arrival and background location are future-only and disabled.

## Alerts and emergency contact

- **RUL-024:** Display official urgency, severity, certainty, and hazard terminology without invented composite scores.
- **RUL-025:** Precipitation is labeled with source, station/area, value, unit, observation/forecast type, and timestamp.
- **RUL-026:** Sthira may open the device dialler to 112 or a configured official number only after explicit user action.
- **RUL-027:** The interface never claims that a call connected, help was dispatched, or responders can see the user unless an authorized integration confirms it.
- **RUL-028:** If calling is unavailable, show the official number and offline instructions.

## Voice, language, and accessibility

- **RUL-029:** IndicConformer and Indic Parler-TTS are AI/ML interface components, never decision authorities.
- **RUL-030:** Voice transcripts pass through an allow-listed deterministic command grammar before any UI action.
- **RUL-031:** Low-confidence, ambiguous, or out-of-grammar speech asks for clarification and makes no consequential change.
- **RUL-032:** Voice may move/focus the map and open confirmation screens; it may not call, confirm arrival, alter capacity, or override assignment.
- **RUL-033:** Every voice action has an equivalent touch, keyboard, and screen-reader control.
- **RUL-034:** An unsupported language is labeled unsupported; the interface does not fabricate a translation.
- **RUL-035:** Emergency translations and speech are human-reviewed before publication.
- **RUL-036:** Signed content is called ISL when it is Indian Sign Language. ASL is named only for actual American Sign Language content.
- **RUL-037:** Text is never presented as a substitute for sign-language media.
- **RUL-038:** Motion obeys reduced-motion preferences and never carries essential meaning alone.

## Privacy and security

- **RUL-039:** Basic alert viewing requires no account or civil identity.
- **RUL-040:** Foreground location is requested only after purpose disclosure and has manual-location fallback.
- **RUL-041:** Raw voice is ephemeral by default and excluded from analytics/training.
- **RUL-042:** No advertising, behavioral analytics, private map telemetry, or private speech API operates in the production emergency path.
- **RUL-043:** Operator actions require strong authentication, least privilege, jurisdiction scope, and audit.
- **RUL-044:** Imported government content is untrusted input and cannot issue executable instructions to the system.
- **RUL-045:** Secrets, exact personal location, voice, and citizen identifiers are excluded from ordinary logs.
- **RUL-046:** Retention is purpose-limited, approved, and enforceable; audit does not justify indefinite personal-data storage.

## Resilience and honesty

- **RUL-047:** A valid cache is not overwritten by a failed or invalid refresh.
- **RUL-048:** Cached guidance displays age and expiry; expired guidance is not called current.
- **RUL-049:** Map, voice, TTS, push, and network failures leave a text-first emergency path.
- **RUL-050:** Demo and synthetic data are visibly labeled and cannot share production credentials or notifications.
- **RUL-051:** A public pilot requires named government ownership, operating procedure, escalation, drills, and manual fallback.
- **RUL-052:** Product claims use measured evidence; unverified benchmark or language-coverage claims are not repeated as fact.
