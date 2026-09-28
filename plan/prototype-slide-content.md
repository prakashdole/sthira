# Sthira — Prototype Slide Content Brief

**File:** `plan/prototype-slide-content.md`
**Prepared from:** `plan/evidence/prototype-presenter-guide.md` (source-inspected at `6861d42`, brought forward here); `plan/TODO.md`; `plan/prompt.md` handoff 2026-09-27 23:35; `tools/demo/README.md`; `plan/evidence/real-inference-launch-check.md`
**Bring-up baseline:** `2d48319` on branch `CLEAN` (`c6d96c3..HEAD` = `ee0b83a`, `debc39e`, `1271257`, `3c28504`, `b15cc4a`, `2d48319`). Anything below written against the earlier `6861d42` baseline has been updated.
**Status:** Unstaged. For Opus review. No runtime verification performed in this bring-up; every result is attributed to the commit that recorded it.

---

## Slide 1 — Title

**Sthira: Voice-Guided Citizen Evacuation Guidance**

[Project name / prototype]

- **Purpose:** A voice-driven web interface that presents authority-supplied evacuation guidance (synthetic exercise data in this prototype), accepts citizen destination requests, and records arrival — bridging the gap between official instructions and the citizen's phone.
- [Team name]
- [Date]
- [Placeholder: exercise or demo identifier — do not invent a live government partnership]

*On-slide label (our own slide label, not a screen string):* "Synthetic exercise prototype — not an operational system"

*Speaker note — the app states this itself, verbatim:* SOURCE — the source-details screen carries `demoNotice` = "This screen uses a synthetic demo package. It is not an active government alert." (`frontend/v2/src/i18n.ts:27`, `rg -F` match)

---

## Slide 2 — Problem and Citizen Journey

**Headline:** From official guidance to confirmed arrival — in the citizen's language

**Four-step journey (plain language, no function names):**

1. **Understand the guidance** — citizen opens the web app, selects their language, sees the map with evacuation zones and shelters overlaid
2. **View a destination** — they speak or type a request; the system shows eligible shelters with travel routes
3. **Request a reservation** — they confirm a shelter; the system records the request and shows a pending state until the server acknowledges it
4. **Report arrival** — they tap "Confirm arrival now" once physically at the shelter; the system records arrival only after explicit server confirmation

**One important distinction (speaker notes, not on-slide):**
*GPS shows where the phone is, not whether the person is safe. The system uses proximity as a trigger, not proof. Real safety confirmation requires the citizen's explicit report and a server acknowledgement.*

**A destination with no coordinates (speaker notes — safety property, not a prediction):**
*A shelter that has no coordinates is still reservable and the stay is still recorded. Because it has no location, the app must not conclude the citizen is near some other shelter. OBSERVED(`1271257`): the browser harness reserves coordinate-less `FACDEMO-2` while the device GPS sits on `FACDEMO-1`'s coordinates, requires no near-destination and no arrival prompt, and uses `FACDEMO-1` as a positive control; a copy of the frontend with a shelter-coordinate fallback fails 3 checks. This is a proven identity rule, not a claim that anyone is safe or unsafe.*

**Diagram suggestion:** A simple flow: Citizen phone → Map + voice → Server → Reservation record → Arrival confirmed

**Speaker notes:**
- The citizen never touches a database directly
- All persistent writes route through the Go backend via authenticated API calls
- The browser is a display and input surface; it applies local validation before sending but never writes reservation state
- Proximity detection (150 m threshold, 100 m accuracy cap, 30 s staleness) triggers arrival eligibility checks — it does not grant arrival
- **"In the citizen's language" is a DRAFT, say it as one.** SOURCE (`3c28504`): the Hindi and Malayalam wording in the reservation-confirm, arrival, destination and alert-detail dialogs, and in the reservation/arrival/guidance error messages, is marked `// HI/ML: DRAFT_REQUIRES_NATIVE_REVIEW` at `i18n.ts:40,80,120` and has not been reviewed by a native speaker — PENDING. The English output is byte-identical after the move (per the commit message). This is a different claim from speech quality: translation review is DRAFT, while ASR/TTS speech-quality evaluation is NOT_RUN. Do not merge them.

**Source:** `plan/evidence/prototype-presenter-guide.md` §1 (client/server division); `journey.ts` (`evaluateProximity`); `1271257` (coordinate-less destination identity, browser-proved); `3c28504` (HI/ML dialog drafts)

---

## Slide 3 — Architecture and Three Models

**Headline:** Browser microphone → Go validation → three AI models → audible response

**Pipeline (four stages, non-technical language):**

1. **Browser** — citizen taps mic or types; the browser captures audio or text, sends it to the backend, and renders the map response
2. **Speech recognition (IndicConformer ONNX)** — converts spoken audio to text; language tag is set by the UI, not inferred
3. **Language model (Sarvam-30B FP8 via vLLM)** — takes the transcript and approved context, produces a structured action (e.g., "show shelter FAC-1 on map"); the server independently validates this action before it is accepted
4. **Speech synthesis (Indic Parler-TTS)** — converts the approved text response to audio in the citizen's language

**Backend guarantees (speaker notes, not on-slide bullets):**
- PostgreSQL/PostGIS stores reservations, stays, and event history
- The Go backend enforces all authorization: the model cannot bypass the reservation or arrival API
- All proposal validation is server-side; frontend validation is a secondary UI safeguard only
- The readiness probe now reports a dead model worker instead of a latched "healthy" — SOURCE (`ee0b83a`): each stage's readiness comes from the live per-stage ready flag the orchestrator gates dispatch on, not from the last health snapshot a dead worker keeps forever, and an unready models subsystem now actually clears the ready state. All three stages are always listed so a dead one cannot be dropped from the count. Keep this modest: a dead worker is detected at the readiness probe and surfaces as a 503 on the next voice command. It is not a prediction of anything.

**On-slide diagram suggestion:** A horizontal pipeline diagram:
`[Phone] → [IndicConformer ASR] → [Sarvam-30B] → [Go validation] → [Parler-TTS] → [Phone]`
with a secondary arrow showing `[Map / Reservation DB]` below the Go validation step

**Separation to show:** The map and database sit below the pipeline, connected to the Go backend — they are not part of the voice pipeline itself.

**Speaker notes:**
- MapLibre GL renders satellite imagery and evacuation overlays; it is a display client, not an inference engine
- PostgreSQL with PostGIS handles spatial queries, capacity, and idempotent reservation writes (using idempotency keys to prevent double-booking)
- The model does not write to the database directly; it proposes actions that the Go orchestrator validates and the API endpoints execute
- The frontend runs validation checks (`validateGuidanceSnapshot`, `buildReservationPayload`) as convenience safeguards — the real enforcement is server-side

**Sources:** `plan/evidence/prototype-presenter-guide.md` §1 (pipeline, endpoints); `orchestrator.go:1043` (validateProposal); `orchestrator.go:594` (stageTemplate); `SarvamModelID = sarvamai/sarvam-30b` (`sarvam_config.go`); IndicConformer model ID `ai4bharat/indic-conformer-600m-multilingual` (`manifest.go`); `ee0b83a` (`httpserver/readiness.go` live per-stage ready flag)

---

## Slide 4 — Data Sources, Authorities and Trust

**Headline:** What is live, what is synthetic, what is proposed

**Three-category table (small, on-slide):**

| Category | Status | What it means |
|---|---|---|
| Synthetic exercise data | Running now | Demo zones, routes, and facilities; not authority-approved |
| ASR / model / TTS workers | Source inspected only | Code exists; real-model inference NOT_RUN |
| Government source integration | Proposed, not connected | Operational approval workflows not yet integrated |

**Five pending approvals (speaker notes, not on-slide):**
- Authorized scenario and source data (government authority)
- Operational route and stay approval authority
- Licensed map redistribution evidence
- Authority-owned facility and shelter policy
- Live operator identity provider and MFA

**What to say (and what not to claim on-slide):**
- Do say: "This prototype uses synthetic data to demonstrate the interface and data flows."
- Do not say: anything implying a live government partnership, certified data source, or operational deployment
- Do not label hazard overlays as predictions — they are synthetic exercise overlays

**Speaker notes:**
- The architecture is designed to connect to authority-supplied guidance; the prototype demonstrates the interface using synthetic exercise data
- Real hazard data, approved safe zones, and authorized routes would come from government systems in an operational deployment
- Map tiles require internet connectivity (Esri ArcGIS); the prototype does not work fully offline
- Emergency call `tel:112` is a browser telephony link, not a system integration

**Sources:** `plan/evidence/prototype-presenter-guide.md` §4 (Judge Q&A); `plan/TODO.md` §4 (external decisions); `real-inference-launch-check.md` §5 (non-claims, AWS instance stopped)

---

## Slide 5 — Prototype Demonstration and Evidence

**Headline:** What the prototype can do today, honestly

**Small capability / evidence table (on-slide):**

| Capability | Evidence class | What it means |
|---|---|---|
| Map, voice console, language switch, emergency dial | Source inspected | Code exists; not executed in browser |
| Reservation → arrival with server acknowledgement | OBSERVED(`1271257`) | Visible-control journey exercised by the headless browser harness; integrated run 77 PASS, 0 FAIL (that commit message). Not a human walkthrough |
| Coordinate-less destination never implies proximity to another shelter | OBSERVED(`1271257`) | Coordinate-less `FACDEMO-2` reserved with GPS on `FACDEMO-1`; no near/arrival prompt; 3 checks fail on a coordinate-fallback copy |
| Readiness probe detects a dead model worker | SOURCE + OBSERVED(`ee0b83a`) | Live per-stage ready flag replaces the latched snapshot; the new `readiness_models_test.go` fails when the fix is reverted |
| Idempotent writes, stale-version rejection | Test verified | Integration tests pass; carried forward from the `6861d42` run |
| Reproducible synthetic stack (one command, scenario selector) | SOURCE(`b15cc4a`) | `./tools/demo/demo.sh start \| status \| stop`; a bring-up tool, not a product feature |
| Real ASR / Sarvam / TTS pipeline | NOT_RUN | AWS instance stopped; no current real inference |
| HI/ML dialog wording | PENDING | Draft pending native-speaker review (`3c28504`); English byte-identical |
| Human speech quality review | NOT_RUN | No Hindi or Malayalam evaluation recorded |

**Status line — do not describe it as continuous monitoring (speaker notes):**
*SOURCE: the status line re-reads `/health/ready` only at app startup (`main.ts:2250`) and on the browser `online` event (`main.ts:2275`) — those are its only two call sites. A refresh after a command is PENDING (planned with Worker 2) and NOT_RUN. So the readiness fix above is detected on the next probe, not instantly on screen.*

**Known middle-worker defects are still OPEN — do not imply the readiness fix closes them (speaker notes):**
*SOURCE(`debc39e`): nine env-gated `TestDefect_*` tests (fake adapters only, skipped unless `LC_DEFECT_REPRO=1`, all nine red when enabled) reproduce production defects that are not yet fixed. Two matter for any real-worker demo: the middle stage can report READY while the vLLM endpoint is dead, and an empty transcript can produce an OK proposal. The synthetic demo default is unaffected, and the `ee0b83a` readiness change does not fix these — they are separate defects. `plan/prompt.md` (2026-09-27 23:35) puts fixing them next, before the paid window.*

**Mock mode explanation (speaker notes, one sentence):**
The demo runs a mock worker that returns pre-configured responses — the ASR returns the same transcript regardless of what the citizen says, and the model returns the configured scenario, not a response to the actual spoken words. SOURCE(`tools/demo/README.md`): "The mock workers play fixed scripts and do not understand speech"; real workers need `./tools/demo/demo.sh start --real-workers` with the worker URLs exported in your own shell.

**Speaker notes (what to emphasise):**
- Source inspection means a human read the code and confirmed the logic is implemented — it is not a runtime test
- Test verification means the Go integration tests pass — it is not a browser walkthrough
- OBSERVED(`1271257`) means the headless browser harness ran that check in the integrated stack; it is not a human-presented demo and not a real phone
- Real-model NOT_RUN means the AWS GPU instance was last confirmed stopped (2026-09-27); no real inference was performed in this session
- No human language evaluation (Hindi, Malayalam speech quality or transcript accuracy) has been recorded
- The Hindi/Malayalam dialog wording is a DRAFT awaiting native review; that is a separate, still-open item from speech quality

**Sources:** `plan/evidence/prototype-presenter-guide.md` §2 (capability table, classifications); `real-inference-launch-check.md` §4 (real-inference checklist, all NOT_RUN); `prototype_scenarios_test.go:440–471` at `6861d42` (test-verified claims, carried forward); `ee0b83a`; `debc39e`; `1271257`; `3c28504`; `b15cc4a`; `tools/demo/README.md`

---

## Slide 6 — Roadmap and Readiness

**Headline:** From prototype to controlled demonstration

**Four phases (short, honest — from plan/TODO.md):**

| Phase | Status | What it needs |
|---|---|---|
| Immediate: integrated browser + model demo | In progress | Real-model smoke test on GPU instance; the nine known middle-worker defects (OPEN) fixed before the paid window |
| Near-term: native device work | Code exists | Actual Android/iOS builds; physical device testing |
| Mid-term: regional validation | Blocked | Authorized scenario data; government/source approvals; human language review, including native review of the draft HI/ML wording |
| Later: controlled operational rollout | Not authorised | All prior phases complete; explicit release approval |

**What to say (speaker notes):**
- The full voice pipeline with real models has not been verified; it is planned for an authorized, bounded GPU session
- Human language review (Hindi, Malayalam transcript accuracy, speech intelligibility) is required before claiming quality in those languages
- The nine known middle-worker defects are OPEN and being fixed first; until they are, a real-worker run can report a ready middle stage over a dead endpoint (`plan/prompt.md`, 2026-09-27 23:35)
- The roadmap is a planning tool, not a commitment; completion percentages are not used because the phases are not equal-sized
- External approvals (government data, map rights, facility authority) are gates the team cannot clear alone

**Do not state on-slide or in notes:**
- Any accuracy, latency, adoption, or impact statistics
- Any implied completion of real-model or browser verification that is not yet recorded

**Sources:** `plan/TODO.md` §2–3 (immediate checklist, full roadmap); `real-inference-launch-check.md` §4 (real-inference checklist); `plan/evidence/prototype-presenter-guide.md` §4 (Judge Q&A)

---

## Refresh Before Presenting

Update these from real evidence before the presentation — do not use placeholder values:

| Item | Current state | Refresh source |
|---|---|---|
| Final integrated commit | `2d48319` on `CLEAN` (bring-up complete) | `git log` on `CLEAN` after the next integration |
| Browser acceptance | OBSERVED(`1271257`): 77 PASS, 0 FAIL in the integrated stack | Re-run the harness; plus a real Safari/Chrome human walkthrough, still outstanding |
| Real model execution | NOT_RUN | Authorized GPU session; `real-inference-launch-check.md` §4 results |
| Measured latency | Not recorded | Real-model smoke run, step-by-step timing |
| HI/ML dialog wording | PENDING — draft (`3c28504`) | Native-speaker review of the marked strings |
| Human speech review | NOT_RUN | Hindi / Malayalam native-speaker evaluation |
| Known middle-worker defects | OPEN (`debc39e` repro tests) | From `backend/`: `LC_DEFECT_REPRO=1 go test ./internal/middleworker/...` — all nine `TestDefect_*` must pass, not skip |

---

## Unresolved Factual Placeholders

These claims need real evidence before they can be stated as facts:

- **TTS sample rate** — "native sample rate reported by the TTS adapter"; 22050 Hz is a legacy fallback; actual rate must be confirmed from runtime readiness and WAV metadata during real-model verification
- **TTS device placement (GPU vs CPU)** — not freshly verified for the current integrated revision; must be confirmed during the authorized real-model session
- **Sarvam-30B JSON output cleanliness** — prior probe (2026-09-25) showed token exhaustion with incomplete JSON and hallucinated `speech_key`; clean output not confirmed for current revision
- **IndicConformer CPU vs GPU** — the only recorded real ASR execution ran on CPU; GPU ASR has not been demonstrated
- **End-to-end real-model latency** — no measurement recorded; must be captured during the authorized GPU session
- **Hindi and Malayalam speech quality** — no human evaluation recorded
- **Hindi and Malayalam dialog wording** — DRAFT only (`3c28504`); marked `DRAFT_REQUIRES_NATIVE_REVIEW` and awaiting a native speaker. Separate from speech quality, and separate again from whether the strings render at all
- **Real-worker behaviour on the nine known middle-worker defects** — OPEN; unverified with real weights, and the synthetic demo does not exercise them

---

## Sources Used

- `plan/evidence/prototype-presenter-guide.md` — primary reference (source-inspected at `6861d42`)
- `plan/TODO.md` — roadmap, phase statuses, worker ownership
- `plan/prompt.md` — executor handoff 2026-09-27 23:35 (middle-worker defects are next)
- `plan/evidence/real-inference-launch-check.md` — AWS instance state, real-inference checklist, non-claims
- `ee0b83a` — `backend/internal/httpserver/readiness.go` live per-stage ready flag; `readiness_models_test.go`
- `debc39e` — nine `TestDefect_*` middle-worker lifecycle tests (`LC_DEFECT_REPRO=1`), known defects OPEN
- `1271257` — coordinate-less exercise facility seed; browser harness sections; 77 PASS, 0 FAIL recorded in the commit message
- `3c28504` — dialog/error strings moved to i18n; `DRAFT_REQUIRES_NATIVE_REVIEW` at `i18n.ts:40,80,120`
- `b15cc4a` — `tools/demo/demo.sh`, `tools/demo/README.md` (one-command bring-up, scenario selector)
- `backend/internal/orchestration/orchestrator.go:1043` — `validateProposal`
- `backend/internal/orchestration/orchestrator.go:594` — `stageTemplate`
- `backend/internal/asrworker/manifest.go:81` — `ai4bharat/indic-conformer-600m-multilingual`
- `backend/internal/middleworker/sarvam_config.go` — `SarvamModelID = sarvamai/sarvam-30b`
- `backend/internal/ttsworker/runtime_adapter.go:251–258` — readiness requires valid native sample rate
- `backend/internal/ttsworker/tts.go:87-98` — `TTSCacheKey` struct
- `backend/internal/ttsworker/audio.go:13` — `DefaultOutputSampleRate=22050` (legacy fallback)
- `frontend/v2/src/journey.ts:291` — `evaluateArrivalConfirmation`
- `frontend/v2/src/journey.ts:484` — `resolveChoiceAgainstGuidance` (facility_id + safe_zone_id)
- `prototype_scenarios_test.go:440–471` — idempotency and stale-version test evidence at `6861d42`
- `frontend/v2/src/main.ts:2250,2275` — the only two `checkRuntime()` call sites (bootstrap and `window` `online`)
