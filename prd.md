---
title: Sthira Citizen Emergency Guidance — Product Requirements
document_id: STHIRA-PRD
version: 2.0
status: Implementation baseline
as_of: 2026-09-11
---

# Sthira Citizen Emergency Guidance

## 1. Product statement

Sthira is a public, mobile-first interface between existing government disaster systems and citizens during an emergency. It does not predict hazards, discover red zones, select safe zones, allocate facilities, dispatch responders, or replace government authority. It makes already-authorized information easier to understand and act on.

The product consumes government data only. It does not buy, collect, or enrich operational data from private companies. Open-source software and locally hosted language models may process the government content and a citizen's commands, but they are not sources of disaster truth.

## 2. Problem

Government agencies already publish alerts, warnings, hazard zones, instructions, weather observations, and emergency contacts. District authorities may also prepare shelters, capacities, and evacuation routes. During a crisis, this information is fragmented, technical, difficult to navigate, and not always available in the citizen's language or preferred modality.

Sthira reduces that last-mile usability gap.

## 3. Users

- Citizen or visitor in or near an officially affected area.
- Citizen assisting a child, older adult, or person with a disability.
- DDMA/SDMA operator publishing approved safe zones, routes, and capacity updates.
- Emergency operator receiving a citizen-initiated call through the device dialler.
- Auditor verifying that displayed guidance matched the official source at that time.

## 4. Baseline journey

1. Sthira receives an authoritative government alert.
2. The citizen sees hazard type, official severity, affected area, issue/expiry time, source, and relevant observations such as precipitation when officially supplied.
3. The interface shows the citizen's position only after permission, the official red zone, and government-pre-allocated safe zones.
4. Sthira selects only among eligible government allocations using the government's current capacity and routing policy. It never invents a shelter or declares land safe.
5. The citizen receives simple step-by-step instructions in English or an available Indian language, with text, speech, and approved Indian Sign Language media where available.
6. The citizen may control the map by voice: focus on a named place, show the alert area, show the assigned safe zone, zoom, pan, recenter, or repeat the route. This is the “God's view” concept, formally named **Voice Map Control**.
7. The citizen can open the device dialler with 112 or an official local number. The call requires an explicit tap/confirmation; the application does not place silent calls.
8. At the destination, Sthira asks whether the citizen and stated party have arrived. A confirmed `YES` creates an idempotent arrival event and decreases **remaining** capacity by the confirmed party size. `NO` offers route help or emergency calling.

## 5. Scope

### Must ship

- Government CAP alert ingestion and source attribution.
- Official red-zone, safe-zone, route, instruction, and capacity imports.
- Severity, hazard detail, precipitation/context, timestamps, and expiry.
- Mobile map plus equivalent non-map instructions.
- English and pilot regional language; architecture for all supported Indic languages.
- AI4Bharat IndicConformer-600M-multilingual for speech-to-text, self-hosted.
- AI4Bharat Indic Parler-TTS for supported text-to-speech languages, self-hosted.
- Deterministic command grammar between transcription and map actions.
- Explicit arrival prompt, party-size confirmation, and concurrency-safe capacity decrement.
- 112/local government emergency-number handoff.
- Offline cache of the latest valid alert, assignment, route, and instructions.
- Accessibility aligned with GIGW and WCAG 2.2 AA.

### Future, not baseline

- Automatic geofence-based arrival detection.
- Background location tracking.
- Automatic reassignment based on passive movement.
- Direct dispatch integration, two-way responder chat, or automatic emergency calls.

### Explicitly out of scope

- Hazard, red-zone, safe-zone, route, or facility-capacity prediction.
- Satellite-based independent decision making, including Sentinel-derived safety claims.
- Crowdsourced or private-company disaster feeds.
- Permanent-relocation planning, land acquisition, beneficiary selection, or scheme administration.
- An LLM deciding what an alert means or where a citizen should go.

## 6. Product principles

1. **Authority before convenience:** stale or unsigned data is never presented as current official guidance.
2. **One urgent action at a time:** the emergency screen emphasizes current danger, destination, next instruction, and call-for-help.
3. **Map and non-map parity:** every route has textual landmarks/instructions; every map action has buttons and screen-reader equivalents.
4. **Voice is a control surface:** transcription may be probabilistic; execution is deterministic and confirmed when consequential.
5. **Privacy by minimization:** location can remain on-device except when required for an authorized assignment/arrival workflow and disclosed to the citizen.
6. **No false certainty:** official source, observation time, expiry, and unavailable data are visible.

## 7. Success measures

- 95% of valid alerts visible within the agreed government-feed latency.
- 95th-percentile first useful screen under 3 seconds on the pilot network profile when cached data exists.
- 100% of displayed red/safe zones and routes traceable to an active government version.
- Zero capacity decrements without an explicit citizen confirmation in the baseline.
- Zero double decrements on retries or repeated taps.
- 100% of emergency calls require a citizen action.
- Task success for alert comprehension, route retrieval, voice map control, and arrival confirmation across the pilot languages and accessibility cohorts.

## 8. Pilot boundary

The pilot jurisdiction is Wayanad, Kerala unless the product owner changes it through an accepted decision. Pilot operations require written DDMA/KSDMA ownership for safe-zone inventory, route approval, capacity definition, update cadence, and incident support. Synthetic fixtures are used until that agreement and the necessary feed access are in place.
