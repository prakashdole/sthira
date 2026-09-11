---
title: Sthira Citizen Emergency Guidance — Feature Catalogue
document_id: STHIRA-FEATURES
version: 2.0
status: Implementation baseline
as_of: 2026-09-11
---

# Feature Catalogue

| ID | Feature | Baseline outcome | Depends on |
| --- | --- | --- | --- |
| FEAT-001 | Official alert inbox | Current area-specific CAP alert with source, severity, time, expiry | GOV-001 |
| FEAT-002 | Emergency detail card | Hazard, precipitation/context, official instructions and uncertainty | FEAT-001, official enrichments |
| FEAT-003 | Official zone map | Citizen location, red zone, safe zones, source/version legend | DDMA/SDMA package |
| FEAT-004 | Safe-zone assignment | Apply official allocation order to open capacity | Facility, route, policy data |
| FEAT-005 | Route guidance | Approved route geometry plus step/landmark list | Active route version |
| FEAT-006 | Voice Map Control | Voice focus/pan/zoom/show/repeat/language commands | ASR + deterministic parser |
| FEAT-007 | Multilingual speech | Self-hosted transcription and spoken guidance | IndicConformer, Indic Parler-TTS |
| FEAT-008 | ISL guidance | Approved sign video/animation with captions/transcript | Government/ISL review |
| FEAT-009 | Emergency call | Confirmed handoff to 112/local official number | Device capability |
| FEAT-010 | Arrival confirmation | Yes/no party arrival interaction | Assignment |
| FEAT-011 | Capacity ledger | Atomic reserve/arrive/release/correct operations | Official capacity/policy |
| FEAT-012 | Offline emergency card | Latest valid alert, destination, route steps, numbers | Service worker/cache |
| FEAT-013 | Operator source health | Freshness, validation, quarantine and conflict view | Source adapters |
| FEAT-014 | Operational package publisher | Authorized import/version/publish/cancel | Operator identity |
| FEAT-015 | Audit reconstruction | What government version each citizen view/action used | Append-only audit |
| FEAT-016 | Accessible non-map mode | Complete keyboard/screen-reader/text flow | All citizen features |

## Core experience states

- **No active alert:** calm state, official-source status, preparedness access.
- **Active alert:** high-salience severity, official time/source, one primary instruction.
- **Assignment pending:** capacity/policy lookup; no destination promise.
- **Route active:** safe-zone identity, next step, route overview, call button.
- **Data stale/conflicting:** visible degraded state and official fallback contacts.
- **Arrival question:** party-aware Yes/No; no automatic confirmation.
- **Arrived:** acknowledgement and government-approved next instructions.

## Voice command examples

| Citizen says | Intent | Effect |
| --- | --- | --- |
| “Show my safe zone” | `SHOW_SAFE_ZONE` | Focus assigned facility |
| “Take me to the red area” | `SHOW_ALERT_AREA` | Show official alert area; do not route into it |
| “Focus on Meppadi” | `FOCUS_PLACE` | Resolve official gazetteer name and animate camera |
| “Show the full route” | `SHOW_ROUTE` | Fit approved route bounds |
| “Zoom in / move left” | `ZOOM_IN / PAN` | Camera-only action |
| “Call emergency” | `OPEN_EMERGENCY_CALL` | Open confirmation sheet; do not dial |
| “Yes, I reached” | Prohibited voice side effect | Ask user to press the visible confirmation button |

## Future register

Geofence arrival, native apps, direct ERSS dispatch, responder chat, and personalized accessibility profiles are candidates only. They are not implied by baseline APIs or acceptance.
