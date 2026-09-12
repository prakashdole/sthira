---
title: Sthira Citizen Emergency Guidance — Decision Record
document_id: STHIRA-DECISIONS
version: 2.0
status: Controlled
as_of: 2026-09-11
---

# Decision Record

Prior permanent-relocation decisions are superseded by DEC-001. Git history preserves their rationale; they are removed from the active documentation so implementation agents do not build the former product.

## Index

| ID | Decision | Status |
| --- | --- | --- |
| DEC-001 | Pivot to a citizen emergency-guidance bridge | ACCEPTED |
| DEC-002 | Government operational data only | ACCEPTED |
| DEC-003 | Authorities predefine red zones, safe zones, routes, capacity, and allocation policy | ACCEPTED |
| DEC-004 | SACHET CAP is the primary alert backbone | ACCEPTED |
| DEC-005 | Voice Map Control implements the “God's view” interaction | ACCEPTED |
| DEC-006 | Self-host AI4Bharat ASR/TTS with deterministic commands | ACCEPTED |
| DEC-007 | Explicit arrival confirmation only in baseline | ACCEPTED |
| DEC-008 | Capacity is an atomic event ledger | ACCEPTED |
| DEC-009 | 112 uses citizen-confirmed device dialler handoff | ACCEPTED |
| DEC-010 | Mobile PWA, modular monolith, PostGIS | ACCEPTED |
| DEC-011 | ISL is the signed-language baseline | ACCEPTED |
| DEC-012 | Map and voice have accessible fallbacks | ACCEPTED |
| DEC-013 | Wayanad remains the proposed pilot | ACCEPTED |
| DEC-014 | Government-approved route only | ACCEPTED |
| DEC-015 | Geofenced arrival is future-only | ACCEPTED |
| DEC-016 | Local CAP ingestion is fixture-backed until source activation | ACCEPTED |

## DEC-001 — Product pivot

**Context:** The former system supported long-term relocation planning and generated analytical candidate-site decisions. The new goal is immediate citizen guidance using pre-existing government decisions.

**Decision:** Replace the active product baseline with a citizen-facing alert, guidance, voice-map, emergency-call, arrival, and capacity interface.

**Rejected:** Incrementally attaching the new interface to the former relocation product; keeping both as equal scopes.

**Consequences:** Existing code is legacy/prototype material and requires a new implementation plan. Permanent relocation, land, scheme, and predictive site-screening modules are not part of v2.

## DEC-002 — Government data only

**Decision:** Operational disaster facts must come from authorized Indian government systems. Private and crowdsourced feeds are excluded. Locally operated open-source libraries/models are allowed as processing dependencies.

**Rationale:** Trust, accountability, and the stated product boundary.

**Consequence:** Basemap/routing/telemetry choices must not leak data to private providers. Missing government data yields a degraded state, not private fallback.

## DEC-003 — Preserve government authority

**Decision:** Government authorities pre-identify red zones, safe zones, routes, capacities, facility state, and allocation priority. Sthira validates and presents these facts and applies the supplied allocation policy.

**Rejected:** Predicting safe zones; generating “best” routes from a commercial router; inferring capacity.

## DEC-004 — CAP backbone

**Decision:** Use NDMA SACHET CAP as the primary pan-India alert interface, preserving raw CAP and its update/cancel semantics. Specialized sources provide compatible context.

**Consequence:** ETag-aware polling, CAP parsing, replay protection, and source conflict handling are first-class.

## DEC-005 — Voice Map Control

**Decision:** Interpret “God's view” as an accessible, animated map camera controlled by allow-listed voice commands.

**Rationale:** It provides the intended feeling of direct spatial control without implying omniscience or predictive authority.

**Consequence:** Use the product term “Voice Map Control” in UI and engineering; “God's view” may remain as an internal concept only.

## DEC-006 — Voice models

**Decision:** Adopt IndicConformer-600M-multilingual for ASR and propose Indic Parler-TTS for TTS, self-hosted. A deterministic parser executes commands. Model activation is per language after licensing, accuracy, latency, and hardware validation.

**Rejected:** LLM-controlled maps; private cloud speech by default; accepting benchmark claims without reproducible evaluation.

## DEC-007 — Arrival

**Decision:** Ask the citizen to confirm arrival. Automatic geofence arrival is excluded from baseline.

**Rationale:** Consent, battery, permission, false-positive, and capacity-integrity risk.

## DEC-008 — Capacity ledger

**Decision:** Model reservations, arrivals, releases, and corrections as immutable idempotent events inside ACID transactions.

**Consequence:** Repeated taps/network retries cannot double-decrement; officials correct with compensating events.

## DEC-009 — Emergency call

**Decision:** Open the device dialler to 112 or an approved local number and require explicit user action.

**Consequence:** The product never claims dispatch, connection, or location sharing without a future authorized ERSS integration.

## DEC-010 — Technical shape

**Decision:** Build a mobile-first PWA, FastAPI modular monolith, PostgreSQL/PostGIS, workers, and internal self-hosted speech services.

**Rejected:** Premature microservices and native apps before PWA capability is measured.

## DEC-011 — Signed language

**Decision:** Use Indian Sign Language assets reviewed with competent Deaf/ISL users and ISLRTC-aligned terminology. Do not label English text as ASL or ISL.

## DEC-012 — Accessible equivalence

**Decision:** Maps, animation, voice, and audio are enhancements. Text, landmarks, touch, keyboard, screen reader, captions, and reduced-motion modes remain complete.

## DEC-013 — Pilot

**Decision:** Retain Wayanad, Kerala as the proposed pilot solely to focus integration and evaluation. This does not imply DDMA/KSDMA approval.

## DEC-014 — Routing

**Decision:** Baseline Sthira displays authority-approved evacuation routes. It does not calculate a route from ordinary road data during an incident.

**Rationale:** A shortest route may cross a hazard, closure, bridge failure, or responder corridor.

## DEC-015 — Future geofencing

**Decision:** Keep geofence arrival in the future register only. No background tracking or automatic occupancy transition is built now.

**Review trigger:** A separate privacy, safety, battery, platform, legal, and field trial with explicit government approval.

## DEC-016 — Fixture-backed CAP until activation

**Decision:** Keep CAP ingestion local and fixture-backed, with an explicit synthetic evidence class and degraded API metadata, until an authorized SACHET endpoint, credentials, retention terms, and operational sample are provided.

**Consequence:** The local parser and lifecycle are testable without implying a live government integration. External HTTP activation remains blocked.

## DEC-017 — Synthetic package integrity and local API

**Decision:** The demo operational package must carry a canonical checksum and
explicit manifest facts; the local service may preview, publish, supersede,
cancel, and rollback only within the synthetic jurisdiction seam. The active
package API is always labelled `SYNTHETIC_DEMO`.

**Consequence:** A missing or altered manifest cannot reach the demo guidance
surface, while real signatures, operator identity, and authority packages remain
external activation gates.

## DEC-018 — Offline cache expiry

**Decision:** Cache only the last verified synthetic scenario and use it only
while its explicit expiry has not passed and the browser is offline. An online
API failure remains fail-closed.

**Consequence:** Network loss can preserve honest source-stamped demo guidance
without turning an unavailable or expired response into current guidance.
