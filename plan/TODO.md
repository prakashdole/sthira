# Sthira — where we are and what is left

Updated: 2026-09-27 18:00. Snapshot: `CLEAN` at the mac-final integration (commits `11ddcfc`…`e7e2171`, see `plan/prompt.md` top and `plan/worker-reports/opus.md`). No pending code edits. This is the owner's overview, not a replacement for the detailed execution plan.

## Where we are now

**We are finishing the browser prototype and P6–P7 integration/verification. We have not accepted P8 mobile delivery or P9 whole-system readiness.** Some later-phase code and documents exist, but that does not mean those phases are complete.

Our immediate goal is a convincing, honest demonstration: browser UI → Go backend → speech recognition → Sarvam → validated map actions and speech output. We are preserving the production architecture while prioritizing the demo.

There are **12 delivery phases, P1–P12, plus preparation phase P0**. They overlap and are not equal-sized, so counting completed phases would give a misleading percentage.

Legend: `[x]` = completed within the stated scope; `[ ]` = remaining, in progress, or awaiting verification. Historical verification is not a fresh test of every current change.

## 1. What we already have

- [x] Go backend foundation and executable API contracts.
- [x] PostgreSQL/PostGIS storage foundation, transactions, audit and idempotency mechanisms; recorded database/recovery verification.
- [x] Government-data ingestion and package contracts; operational authority/data acceptance remains separate.
- [x] Reservation/stay lifecycle implementation with snapshot, capacity and replay protections; recorded integration regressions.
- [x] Offline package protocol implementation and recorded engineering checks; real regional packs and native-device acceptance remain separate.
- [x] ASR, middle-model and TTS worker interfaces, executable services and local adapter tests.
- [x] Browser prototype wired to backend voice, guidance and stay endpoints.
- [x] Synthetic/mock-worker end-to-end plumbing and database journeys demonstrated in recorded checks.
- [x] Backend security, recovery and performance preparation exists; final integrated acceptance is still required.

**Not yet proved by the above:** a complete current real-model voice demo, reliable browser interaction on target devices, reviewed speech quality, or production readiness.

## 2. Immediate checklist — finish the prototype

### A. Integrate the current workers

- [x] Opus finishes stale-response/language-switch handling and reservation/selection/arrival consistency (`aa97e69`, `41a0278`, `e066056`).
- [x] Integrate and review Gemini's map, panel and destination-proximity fixes (`fc0128b`; safe-zone ambiguity corrected in `41a0278`).
- [x] Remove or explicitly isolate mutable test-only browser hooks (dev server + `?sthira-test-hooks=1` only; absent from `dist`).
- [x] Responsive CSS and dialog keyboard/focus checks for 375/1024/1440 px in EN/HI/ML (`1fa3fed`, `74dd911`; headless Chromium hit-tests). True 200 % zoom not checked.
- [x] Verify the final integrated revision and record coherent local commits on `CLEAN` (`0d290da`; 73/73 browser checks).

### B. Prove the actual browser journey

- [x] Real browser (headless Chromium): onboarding, text command, repeated 3D/recenter from UI and voice, layers, panels (`map`, `dialog` sections).
- [ ] Correct destination identity; missing coordinates never imply proximity to another shelter. (unit-tested; not yet observed in a browser)
- [x] Reservation → arrival works through visible controls (headless Chromium, `e066056`); failures never show fabricated success.
- [ ] Language changes, cancellation, backgrounding and stale responses behave correctly. (late old-language response, recording cancel/restart and language switch while recording observed in Chromium; real tab backgrounding not exercised)
- [x] Audio playback/replay, worker failure and recovery in Chromium (`audio`, `audio-denied`, `outage`, real mock-worker stop/restart). Audible quality not reviewed.
- [ ] Check laptop/phone-sized layouts, keyboard use and long regional-language text. (Chromium viewport checks pass; a real phone and Safari remain)
- [x] Record actual browser results separately from helper tests and curl/Node checks (`plan/evidence/prototype-browser-verification.md`).

### C. Prepare and run real models

- [ ] Finish offline audio-format/adapter compatibility checks. (first batch integrated `b35a875`; a later revision is uncommitted)
- [ ] Finish the reusable model evaluation corpus and its offline validation. (`corpus-check` integrated `edb8aad`; a later revision is uncommitted)
- [x] Confirm locally that launch instructions match the actual modules, configuration and host/container paths (`6861d42`, `real-inference-launch-check.md` §7).
- [ ] Obtain an explicit bounded paid verification window before starting cloud work.
- [ ] During that window, verify the actual ASR, Sarvam and TTS services individually.
- [ ] Verify microphone → transcript → valid action → visible UI response → intelligible speech end to end.
- [ ] Test English and Hindi; obtain appropriate human review for Malayalam and other claimed language support.
- [ ] Measure observed latency and record failures honestly; preserve concise evidence and stop owned paid compute as authorized.

Selected models: **IndicConformer-600M-Multi → Sarvam-30B FP8 → Indic Parler-TTS**. Current ASR language is supplied by the UI; automatic language detection is not an accepted capability. Current full real-model acceptance remains pending; this checklist does not assert the current AWS instance state.

### D. Make the demo understandable

- [ ] Correct and verify the presenter guide against actual frontend/backend responsibilities.
- [ ] Rehearse the five-minute demo with explicit synthetic versus real-model labels.
- [ ] Prepare an honest fallback for network/model failure.
- [ ] Finish the presentation using verified capabilities, sources, authority dependencies and limitations.

**Demo checkpoint:** A–D have concrete evidence or clearly disclosed limitations. Do not describe mock audio as real inference or a phone-sized browser as a native mobile app.

## 3. Full roadmap — beyond this demo

| Phase | Purpose | Current owner-level status | What remains |
|---|---|---|---|
| P0 | Scope and migration baseline | Recorded complete | Preserve agreed scope and history. |
| P1 | Go foundation and API contracts | Recorded complete | Maintain compatibility as fixes integrate. |
| P2 | Government-data contracts and scenarios | Infrastructure implemented; acceptance partial | Authorized source/scenario data and provenance acceptance. |
| P3 | Durable storage and ledger | Foundation recorded complete | Verify affected behavior after later changes. |
| P4 | Destination choice and stays | Engineering implemented; full phase open | Current UI integration, operational route/stay authority and live operator identity. |
| P5 | Offline package/map delivery | Engineering implemented; full phase open | Licensed real map packs, regional measurements and actual client/device acceptance. |
| P6 | ASR, constrained model and TTS | Active verification | Current real-weight inference, full voice journey and human language/voice review. |
| P7 | Backend security/performance handoff | Substantial preparation; final gate not accepted | Close current correctness findings and verify the coherent integrated build. |
| P8 | Android and iPhone clients | Code exists; acceptance reopened | Actual native builds, real transport/storage/voice journeys and physical-device evidence. |
| P9 | Whole-system/regional readiness | Materials exist; acceptance reopened | P8 prerequisites, authorized regional scenarios, qualified reviews and realistic drills. |
| P10 | Cleanup and final artifact | Cleanup performed historically; final gate provisional | Recheck the eventual release artifact after its prerequisites pass. |
| P11 | Authorized integrations/shadow exercises | Blocked on external authorization and prerequisites | Government/source approvals, operational integration and supervised exercises. |
| P12 | Controlled public launch | Not authorized/accepted | P11 acceptance, explicit release approval and operational rollout readiness. |

### After the prototype — next substantial engineering work

- [ ] Reconcile buildable Android/iOS projects and run actual builds, not syntax-only checks.
- [ ] Verify native API transport, private token storage, durable offline queue and package validation.
- [ ] Connect native citizen journeys to real backend responses; no local fabricated booking/arrival success.
- [ ] Test microphone, playback, location permissions and offline/restart behavior on physical devices.
- [ ] Complete operator workflows against the selected identity provider and scoped authorization.
- [ ] Validate regional data, map rights, language quality and whole-system drills.
- [ ] Recheck security, performance, recovery, packaging and release operations for the actual release candidate.
- [ ] Obtain authorization for shadow exercises and, later, controlled launch.

## 4. External decisions code alone cannot finish

- [ ] Authorized scenario/government source data (O01).
- [ ] Operational route authority (O05).
- [ ] Official map redistribution/license evidence (O06).
- [ ] Authority-owned stay/facility policy (O07).
- [ ] Live operator identity provider/MFA (O14).
- [ ] Required human regional-language, voice and field reviews.

These gates do not prevent an honestly labelled synthetic prototype. They do prevent claiming operational certification or production readiness. See [open-decisions.md](open-decisions.md) for the full decision register.

## 5. Current worker ownership

| Worker | Current lane | Completion rule |
|---|---|---|
| Opus | Correctness, integration and commits | Reviews deliverables and records final integrated evidence. |
| Gemini | Responsive/accessibility CSS | Earlier logic fixes still require integration; new CSS needs browser checks. |
| MiniMax A | Presenter-guide factual corrections | Source-backed guide, no unsupported capability claims. |
| MiniMax B | Browser verification | Actual browser observations or explicit NOT_RUN/blocker. |
| MiniMax C | Model evaluation corpus | Compatible corpus/oracle verified offline; real inference remains separate. |
| MiniMax D | Audio format/adapter boundaries | Meaningful local codec checks; no claim of recognition or voice quality. |

Worker-reported completion is not integrated acceptance. Keep finished-but-unreviewed work unchecked above. Avoid assigning more overlapping implementation until Opus integrates the current batch.

## How to keep this useful

Update this overview at each integrated checkpoint, not after every command. Tick a box only for its stated outcome, and link the concise evidence if needed. Keep detailed instructions in [prompt.md](prompt.md), the roadmap in [phases.md](phases.md), and results in `evidence/`.

This overview uses current/reopened acceptance notices rather than older conflicting DONE rows. It is a planning summary, not a new full-codebase audit or permission to start paid infrastructure, publish, or launch.
