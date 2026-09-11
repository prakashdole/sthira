---
title: Sthira Citizen Emergency Guidance — Immediate Build Plan
document_id: STHIRA-PLAN
version: 2.0
status: Ready for implementation after prerequisite confirmations
as_of: 2026-09-11
---

# Immediate Build Plan

## Goal

Deliver a production-shaped vertical slice using synthetic government-format data: receive an alert, show official context, assign a pre-approved safe zone, guide by map/text/voice, open emergency calling, confirm arrival, and update capacity safely.

## First implementation slice

1. Create `sthira_v2` domain contracts for CAP alert, operational zone, facility, approved route, assignment, arrival, and capacity event.
2. Add synthetic Wayanad fixture package with unmistakable demo labeling.
3. Build CAP parser/validator and active-alert endpoint.
4. Build map/non-map emergency screen with source, times, severity, precipitation context, zone, destination, route, and 112 action.
5. Build assignment/capacity transaction and arrival confirmation with idempotency/concurrency tests.
6. Add deterministic text command parser and map-camera controller.
7. Integrate self-hosted IndicConformer after local hardware/model spike; keep touch controls complete.
8. Spike and validate exact Indic Parler-TTS artifact before integrating it.
9. Add offline cache and stale/conflict/error states.
10. Run accessibility, security, API, concurrency, and live-browser verification.

## Do not begin with

- Geofencing or background location.
- Custom hazard models or satellite analysis.
- A general chatbot/LLM.
- Direct ERSS dispatch integration.
- Multi-state tenancy, permanent-relocation workflows, or microservices.
- Private maps, weather, routing, speech, or telemetry.

## Definition of done for the slice

- Synthetic alert-to-arrival flow works on mobile and desktop.
- Every official-looking datum has a demo/source/version label.
- Route remains usable without the map; core flow remains usable without voice/audio.
- Repeated arrival confirmation cannot double-decrement.
- Concurrent assignments cannot overbook.
- Expired/cancelled/conflicting alerts and closed/full shelters fail safely.
- Voice commands move/focus the map and never execute prohibited side effects.
- `.txt` files remain unchanged.

## Before a live pilot

Close ODN-001 through ODN-019 as applicable, receive source samples/permissions, replace synthetic operational packages, conduct human language/ISL review, and pass Phase 7.
