---
title: Sthira Citizen Emergency Guidance — Delivery Phases
document_id: STHIRA-PHASES
version: 2.0
status: Execution plan
as_of: 2026-09-11
---

# Delivery Phases

Each phase ends with working evidence. Live integrations stay behind feature flags until their source activation gate passes.

## Phase 0 — Pivot and repository safety

**Outcome:** v2 can be built without accidentally extending the permanent-relocation system.

- Freeze and inventory reusable code.
- Create a v2 namespace/API boundary; do not destructively rewrite legacy modules at once.
- Add synthetic CAP, red-zone, safe-zone, route, facility and capacity fixtures.
- Establish CI, threat model, accessibility test baseline, and ADR process.

**Exit:** v2 skeleton runs independently; tests prove demo/production separation.

## Phase 1 — Official alert backbone

- Implement hardened CAP XML parser, raw artifact store, ETag polling, deduplication, update/cancel handling, and freshness.
- Add allow-listed source registry, quarantine, health status, and synthetic SACHET-compatible fixtures.
- Build alert API and mobile text-first active/no-alert/stale states.

**Exit:** replayable 200/304/update/cancel/outage tests and source-to-screen trace.

## Phase 2 — Government operational package

- Define signed/versioned DDMA package for red zones, safe zones, routes, instructions, facilities, capacities, and allocation policy.
- Add PostGIS validation, geography bounds, lifecycle, correction, and operator import/publish UI.
- Integrate government basemap only after terms/load decision.

**Exit:** invalid or expired data cannot reach citizen views; a valid synthetic package renders map and non-map guidance.

## Phase 3 — Assignment and capacity

- Implement official-policy eligibility and assignment.
- Implement atomic reservation/arrival/release/correction ledger.
- Add party-size and Yes/No arrival experience.
- Stress concurrent assignments, retries, stale clients, closure, and reassignment.

**Exit:** no oversubscription or duplicate decrement under fault/concurrency suite.

## Phase 4 — Voice Map Control

- Deploy IndicConformer inference behind an internal API.
- Build language-aware allow-listed grammar and official gazetteer resolver.
- Implement camera commands, visible transcript, confidence/clarification, and touch parity.
- Benchmark noisy emergency speech per pilot language and target hardware.

**Exit:** approved task accuracy/latency thresholds pass; prohibited commands cause no side effect.

## Phase 5 — Speech, ISL, and accessibility

- Verify exact Indic Parler-TTS artifact/license/languages; integrate only approved languages.
- Create/cache human-approved spoken instructions.
- Produce approved ISL instruction assets with Deaf/ISL review.
- Complete WCAG 2.2 AA/GIGW testing, reduced motion, captions, screen-reader and non-map flows.

**Exit:** accessibility cohorts complete critical tasks; unsupported modes are labeled honestly.

## Phase 6 — Emergency and offline operation

- Add citizen-confirmed 112/local-number dialler handoff.
- Cache latest valid emergency card and route instructions.
- Exercise feed loss, map loss, voice loss, database failover, stale route, full shelter, and operator correction.

**Exit:** manual fallbacks and runbooks pass game-day rehearsal.

## Phase 7 — Shadow pilot

- Activate authorized government connectors in a non-public environment.
- Compare source-to-display latency, conflicts, capacity reconciliation, ASR performance, and operator workload.
- Conduct privacy/security assessment and incident-response rehearsal.

**Exit:** DDMA/KSDMA, accessibility, security, privacy, and operations owners sign the pilot gate.

## Phase 8 — Controlled citizen pilot

- Limited geography/users/time window, 24×7 government ownership, visible pilot notice, rollback switch, and independent monitoring.
- Measure comprehension, successful route retrieval, arrival integrity, calls opened, availability, latency, and harms/near misses.

**Exit:** evidence-based go/iterate/stop decision. No automatic scale-up.

## Future research only

Geofence arrival requires a separate ADR, DPIA, explicit permission design, field accuracy trials, battery testing, fraud/false-positive analysis, and authority approval. It is not a hidden Phase 8 deliverable.
