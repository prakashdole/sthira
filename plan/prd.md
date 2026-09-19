# Product requirements — immediate and temporary relocation

Version 3 planning baseline · 2026-09-19 · Implementation pending.

## Purpose

Help a person understand an official disaster situation, choose among eligible relocation destinations, and follow verified guidance to immediate shelter or a temporary facility for one week to one month. Sthira is an interface and coordination service over authorized incident information. It does not forecast flooding/landslides, draw risk zones, certify land or plan permanent resettlement.

## People and scope

- Residents, visitors, carers, older adults, people with disabilities and people with limited literacy or digital experience.
- Design for a child to understand basic controls and for adults aged 70–80 to complete the flow. Do not claim that a young child can evacuate independently; test child interactions only with appropriate guardian involvement.
- Android devices with 3 GB RAM, plus a supported lower-end iPhone cohort. Minimum OS/device list is an explicit launch decision.
- One million total users; concentrated incident peaks, poor connectivity and power interruptions are expected.
- 10–15 states with 2–3 historical flood/landslide scenarios **per state in total**, chosen for relevant hazards. Not both hazards in every state; not all Indian states. Primary regional languages are mandatory for selected states, subject to verified model and human-language gates.

## Immediate journey

1. Launch a locally installed, quiet interface. Show a large speak control, a readable chat button, emergency help and last data status. No registration wall for public guidance.
2. Ask for foreground location only when needed; alternatively understand a spoken/typed village, landmark, district or saved area. Do not require a state/village form as the normal path. Disambiguate repeated village names with a short question and large choices. Location is not obtained by the LLM.
3. Retrieve current versioned incident data. Show official red-zone geometry, time, source and instructions. Explain missing/stale/no-coverage states without drawing replacement zones.
4. Present a small set of eligible destinations with known route length, facility type, accessibility, transport requirements and capacity status. Allow the person to choose based on local knowledge. Unknown facts remain unknown. “Nearest” means shortest length among eligible, verified routes when a comparison policy permits it; never nearest straight-line land or a model safety score.
5. Preview the selected destination and its route. Authority-only allocation constraints may restrict choice and must be explained. A blocked route report cannot silently create an alternative route.
6. Obtain explicit confirmation for any reservation/arrival/call handoff. The middle model can open a confirmation screen but cannot authorize capacity changes or place calls.
7. Show the next instruction and landmark, with a map and complete equivalent text/audio. Read useful answers, changed instructions and clarification questions; omit “Yes, I am finding it” or narration of camera movements.
8. Let the user report arrival explicitly. Convert any existing reservation into occupancy once; never subtract capacity twice. A self-report is not an independent rescue or welfare confirmation.

## Temporary relocation journey

A facility distinguishes emergency assembly/refuge from overnight shelter and 7–30 day temporary accommodation. Publish only supplied opening/closing dates, permitted stay, space, water/sanitation, accessibility, family needs, contact and transport information. Do not infer suitability from building type.

Support a confirmed temporary stay, authorized extension within the programme's 7–30 day scope, departure, cancellation and transfer. For a transfer, avoid abandoning the existing stay before the new acceptance is confirmed; reconcile both facilities atomically or through explicit durable pending states. Capacity may be date-specific for temporary stays. Do not expose a future bed as available today. Over-30-day cases escalate to the responsible operator; do not automatically evict or invent permanent-relocation workflows.

## Voice and non-voice interaction

Recorded speech → server ASR → constrained language interpretation → deterministic domain validation → client map/cards → optional approved speech. The middle model resolves intent and requests display of known data; it cannot manufacture operational facts. Typed chat uses the same contract. All essential actions work with touch and screen readers even during model failure.

The chat icon may sit in a corner, but must have a visible label and at least a 48 dp/44 pt platform-appropriate touch target. Essential controls cannot be tiny or hidden. Provide strong contrast, large adjustable text, captions, no color-only hazard meaning, no mandatory animation and accessible local error messages. ISL content needs a qualified reviewer and is not replaced by English text.

## Offline contract

Install the UI, icons and essential strings on device. Download optional regional maps, gazetteer aliases, approved audio and signed incident packages before travel or during connectivity. Freshness rules apply equally offline. Preserve a valid cache after failed refresh; retain supersession/cancellation tombstones. Expired data may be shown as historical context but cannot remain active navigation. No network means no new verified assignment, live capacity, ASR/LLM/TTS generation or guaranteed alert delivery. Provide locally stored official contact information and approved generic emergency instructions.

## Explicit exclusions

Permanent relocation, property/land acquisition, beneficiary/scheme management, hazard prediction, AI-authored routes, automated rescue dispatch, background surveillance/geofence arrival, automatic calls, crowdsourced safety certification, advertising and behavioral tracking.

## Acceptance

Every requirement maps to a phase and evidence in [trd.md](trd.md) and [assurance.md](assurance.md). Software completion requires both mobile platforms, selected state languages, real 3 GB-device checks, meaningful degraded-mode behavior, capacity integrity and reproducible load/security evidence. Demo success alone is not operational readiness. Route authority, production data and final jurisdiction approvals remain separately gated in [open-decisions.md](open-decisions.md).
