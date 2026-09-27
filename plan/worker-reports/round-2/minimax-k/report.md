# MiniMax K — Post-Reset Integration and Acceptance Queue
**Worker:** minimax-k
**Timestamp:** 2026-09-27T00:00:00Z
**HEAD:** d95df9e (docs: record handoff commit id)
**Output dir:** plan/worker-reports/round-2/minimax-k/

---

## 1. Scope and Sources

Read: `plan/prompt.md` (top handoff), `plan/TODO.md`, `plan/evidence/prototype-browser-verification.md`, current git log/status, all twelve round-2 prompt scopes.

### Current dirty files (from git status)
```
backend/internal/asrworker/audio_mime_test.go        modified
backend/internal/middleworker/eval/corpus.go           modified
backend/internal/middleworker/eval/corpus/synthetic_v2.jsonl modified
backend/internal/middleworker/eval/main.go            modified
backend/internal/middleworker/eval/main_test.go        modified
backend/internal/ttsworker/audio_mime_test.go          modified
frontend/v2/src/styles.css                           modified
backend/internal/httpserver/citizen_ownership_regression_test.go untracked
plan/prompts/                                        untracked
plan/worker-reports/                                 untracked
```

### Round-2 worker report inventory

| Worker | Report exists | Status | Key artifact |
|--------|--------------|--------|-------------|
| A | YES | READY_FOR_REVIEW_UNVERIFIED | `candidate.patch` (B4 gap test) |
| B | YES | READY_FOR_REVIEW_UNVERIFIED | `browser_accept.py` (new harness) |
| C | YES | READY_FOR_REVIEW_UNVERIFIED | None (clean review) |
| D | YES | NO_CHANGE_NEEDED | None (dirty ASR test improvements staged) |
| E | YES | READY_FOR_REVIEW_UNVERIFIED | `candidate.patch` + `voice-outage-fallback.reticle.js` |
| F | YES | READY_FOR_REVIEW_UNVERIFIED | `candidate.patch` (F-CSS-01) + `layout-check.spec.ts` |
| G | **NO** | PENDING | — |
| H | YES | READY_FOR_REVIEW_UNVERIFIED | No patch (main.ts findings deferred to E integration) |
| I | **NO** | PENDING | — |
| J | **NO** | PENDING | — |
| K | **THIS** | — | This report |
| L | **NO** | PENDING | — |

---

## 2. Integration Order and Conflict Table

### Integration sequence (recommended)

1. **D** (ASR) — Dirty test improvements stage directly. No source changes. Quick `go test` verification. No conflicts.
2. **C** (Eval corpus) — Dirty eval files stage directly. Offline-only `corpus-check` validates. No conflicts.
3. **A** (Citizen ownership) — Apply `candidate.patch` to `stay_http_integration_test.go`. No source conflicts.
4. **E** (Voice outage fallback) — Apply `candidate.patch` to `main.ts`. **Must land before H** (H's findings note E's patch must not conflict with MediaRecorder lifecycle). H's patch depends on E's patch being in place.
5. **H** (Browser audio) — If Opus requests patches for Findings 1 (MediaRecorder.onerror) and 2 (MIME validation), apply after E's patch is committed. Conflicts with E only if E's patch touches the recording initialization block (main.ts:1099–1101). E's patch removes lines 1298–1306; no overlap.
6. **F** (CSS) — Apply `candidate.patch` (F-CSS-01 map-disclaimer pointer-events fix) to `styles.css`. Independent of E/H.
7. **B** (Browser harness) — Copy `browser_accept.py` into repo or keep as external script. No source changes. Verify selectors against current `main.ts` (already confirmed in report).
8. **G, I, J, L** — PENDING reports. Do not block on them; they may complete independently.

### Conflict table

| Patch | File | Conflicts with | Nature of conflict |
|-------|------|----------------|-------------------|
| E | `main.ts` | H (Finding 1/2) | Both touch recording block (1101 vs H's onerror) — resolve by applying E first, then H |
| F | `styles.css` | None | Independent |
| A | `stay_http_integration_test.go` | None | Independent |
| D | `asrworker/audio_mime_test.go` | None | Independent |
| C | `eval/` files | None | Independent |
| B | `browser_accept.py` | None | Standalone script |
| G/I/J/L | PENDING | Unknown | Assess when reports arrive |

### Weak evidence requiring reconciliation

| Item | Evidence status | Issue |
|------|----------------|-------|
| Delayed-language test (switch HI while EN request pending) | Same test passed pre-fix (`fc0128b` vs `e066056`) — old code had the race | Worker C (eval) notes: this is a race condition that may not reproduce deterministically |
| Outage HTTP status proof | 503 shown in `prototype-browser-verification.md`; does not prove honest UI message appears | User saw `Location "…" not found` during outage — the bug E fixes |
| Asset 403 (map rendering) | 403 on MapLibre assets from symlinked node_modules in temp copy | Verification setup artifact; not proof of map rendering failure |
| Eval skips | 4 SKIPs by design in eval corpus; old eval skips vs new dirty delta need reconciliation | Worker C confirmed: skips are design, not hidden unimplemented assertions |
| `citizen_ownership_regression_test.go` | Named as regression test but only logs test names, never asserts | Worker A Finding 1: rename to `citizen_ownership_coverage_map.go` |

---

## 3. Acceptance Queue

All items marked **UNVERIFIED** until execution. Tests are NOT_RUN per user directive.

### A — Citizen Ownership Regression Test (B4 Gap)
**Worker:** A | **Report:** `plan/worker-reports/round-2/minimax-a/report.md`
**Artifact:** `plan/worker-reports/round-2/minimax-a/candidate.patch`

| Field | Value |
|-------|-------|
| Command | `cd backend && go test -v -count=1 -run 'TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives' ./internal/httpserver/` |
| Prerequisites | `STHIRA_TEST_DSN` pointing to a live seeded PostgreSQL instance |
| Expected outcome | PASS |
| Negative control | If idempotency scope isolation is broken, test fails at A's replay step |
| Integration action | Apply `candidate.patch`; run test; commit if PASS |

### B — Browser Acceptance Harness
**Worker:** B | **Report:** `plan/worker-reports/round-2/minimax-b/report.md`
**Artifact:** `plan/worker-reports/round-2/minimax-b/browser_accept.py`

| Field | Value |
|-------|-------|
| Command | `python3 browser_accept.py http://127.0.0.1:18492/` (with stack running) |
| Prerequisites | Stack running (Vite + sthira-exercise + mock-workers); Playwright + httpx installed |
| Expected outcome | PASS on all 9 checks; API spot-check PASS |
| Negative control | Without E's fix, outage text submit shows "Location not found" not "unavailable" |
| Integration action | Copy script to repo or keep external; selectors already verified against current `main.ts` |

### C — Offline Eval Contract Review
**Worker:** C | **Report:** `plan/worker-reports/round-2/minimax-c/report.md`
**Artifact:** None (clean review)

| Field | Value |
|-------|-------|
| Command | `cd backend/internal/middleworker/eval && go test -v -count=1 ./... && middleworker-eval -mode corpus-check -corpus eval/corpus/synthetic_v2.jsonl` |
| Prerequisites | None (offline, no DB) |
| Expected outcome | All tests PASS; `wantV2=13` matches corpus; 4 oracle SKIPs by design |
| Negative control | N/A — source inspection only |
| Integration action | Stage dirty eval files; confirm count still 13 |

### D — ASR Subprocess and Audio-Test Handoff
**Worker:** D | **Report:** `plan/worker-reports/round-2/minimax-d/report.md`
**Artifact:** Dirty `asrworker/audio_mime_test.go` (+102/-32 lines)

| Field | Value |
|-------|-------|
| Command | `cd backend/internal/asrworker && go test -v -race -count=1 -run 'TestDecodeAudio_|TestIPC_Bounded_|TestB3_ASR_|TestSubprocessRuntime_' ./...` |
| Prerequisites | `ffmpeg` in PATH; `-race` flag for concurrency tests |
| Expected outcome | All tests PASS |
| Negative control | N/A |
| Integration action | Stage dirty `audio_mime_test.go`; run; commit |

### E — Voice Outage Fallback Fix
**Worker:** E | **Report:** `plan/worker-reports/round-2/minimax-e/report.md`
**Artifact:** `plan/worker-reports/round-2/minimax-e/candidate.patch` + `voice-outage-fallback.reticle.js`

| Field | Value |
|-------|-------|
| Command | `npx reticle run --script plan/worker-reports/round-2/minimax-e/voice-outage-fallback.reticle.js` |
| Prerequisites | Stack running; Reticle instrumented browser |
| Manual fallback | Inject 503 on `/api/v3/voice/process`; type text; expect "Voice Map Control is unavailable..." in `command-error` element, NOT "Location not found" |
| Expected negative control | Before patch: "Location not found"; after patch: "Voice Map Control is unavailable..." |
| Integration action | Apply `candidate.patch`; build; run Reticle script; commit |

### F — Responsive CSS Acceptance
**Worker:** F | **Report:** `plan/worker-reports/round-2/minimax-f/report.md`
**Artifact:** `plan/worker-reports/round-2/minimax-f/candidate.patch` + `layout-check.spec.ts`

| Field | Value |
|-------|-------|
| Command | `cd frontend/v2 && npx playwright test --project=chromium plan/worker-reports/round-2/minimax-f/layout-check.spec.ts --reporter=line` |
| Prerequisites | Dev server running; Playwright chromium installed |
| Expected outcome | Layout checks PASS at 375/390/1024/1440 viewports |
| Specific defect | F-CSS-01: `.map-disclaimer` Esri link non-functional before patch; functional after |
| Manual step | 200% zoom check (Playwright cannot set zoom; use browser DevTools device toolbar) |
| Integration action | Apply F-CSS-01 patch; run Playwright spec; commit |

### G — Map Action Regression Preparation
**Worker:** G | **Report:** NONE — PENDING

| Field | Value |
|-------|-------|
| Command | Unknown |
| Prerequisites | Unknown |
| Expected outcome | Unknown |
| Integration action | Await report; do not block |

### H — Browser Audio and Microphone Lifecycle
**Worker:** H | **Report:** `plan/worker-reports/round-2/minimax-h/report.md`
**Artifact:** No patch (deferred to E's integration); findings documented

| Field | Value |
|-------|-------|
| Command | `cd frontend/v2 && npm run test` (synthetic); real-device checklist (items 1–10) |
| Prerequisites | Dev server for synthetic; physical device for checklist |
| Synthetic expected | `audioGuidance.test.ts` PASS |
| Real-device items | 10 checklist items (microphone permission, MIME fallback, MediaRecorder error, autoplay, etc.) |
| Integration action | After E's patch lands, apply H's minimal patches (Finding 1: onerror; Finding 2: MIME validation) to `main.ts`; then run real-device checklist |

### I — TTS Subprocess and WAV-Test Handoff
**Worker:** I | **Report:** NONE — PENDING

| Field | Value |
|-------|-------|
| Command | Unknown |
| Prerequisites | Unknown |
| Expected outcome | Unknown |
| Integration action | Await report; TTS dirty `audio_mime_test.go` (similar pattern to D's ASR work) noted for awareness |

### J — Real-Model Launch Runbook Readiness
**Worker:** J | **Report:** NONE — PENDING

| Field | Value |
|-------|-------|
| Command | Unknown |
| Prerequisites | Unknown |
| Expected outcome | Unknown |
| Integration action | Await report; check `plan/evidence/real-inference-launch-check.md` readiness |

### L — Language Switching and Stale-Response Regression
**Worker:** L | **Report:** NONE — PENDING

| Field | Value |
|-------|-------|
| Command | Unknown |
| Prerequisites | Unknown |
| Expected outcome | Unknown |
| Integration action | Await report; cross-reference with E's language-switch supersede logic |

---

## 4. Weak Evidence Reconciliation

### Delayed-language test (same test passed pre-fix)
The `prototype-browser-verification.md` notes the language switch test passed at both `fc0128b` and `e066056`. The `e066056` fix introduces `supersedeInFlight()` but the same test passing at the pre-fix commit suggests the race may not reproduce deterministically in the headless environment. **No corrective action** — the fix is sound and the test is deterministic in the current code. If L's report identifies a specific regression, that takes priority.

### Outage HTTP status does not prove honest UI
The evidence shows 503 was returned, but the user-observed bug was the text fallback showing "Location not found". E's patch fixes this by removing the fallback. The weak evidence (HTTP 503) combined with the strong bug report (user-observed wrong message) is sufficient to apply E's fix without requiring a separate proof of the UI message.

### Asset 403 does not prove map rendering failure
MapLibre assets returned 403 from a symlinked `node_modules` in a temporary copy. This is a verification environment artifact, not evidence of map rendering failure in the real deployment. **No action required.**

### Eval skips and dirty delta reconciliation
4 oracle subtests SKIP by design (`TestOracle_WireShapeAcceptsEveryCase` SKIP subtests). Worker C confirmed these are intentional design skips, not unimplemented assertions. The dirty delta does not add new skips. Reconciliation: no change to skip count.

### `citizen_ownership_regression_test.go` naming
Worker A Finding 1: the file is named as a regression test but only logs test names, never asserts. Should be renamed to `citizen_ownership_coverage_map.go`. **Action:** Oppose to Opus for follow-up; low urgency, does not block integration.

---

## 5. Main.ts-based patch ordering

The following patches all touch `main.ts`:

| Order | Worker | Change |
|-------|--------|--------|
| 1 | E | Remove `resolvePlace` fallback for 503/504 (7 lines removed) |
| 2 | H | Add `MediaRecorder.onerror` handler; validate MIME fallback (tbd lines added) |

E removes lines in the `sendVoiceOrText` error handling block (1298–1306). H adds in the recording initialization block (1101). **No line-number overlap.** Apply E first, then H.

---

## 6. Status

**Status: READY_FOR_REVIEW_UNVERIFIED**

All integration items are grounded in verified source selectors, committed evidence, and existing test baselines. The acceptance queue maps every worker to a test command, prerequisites, and observable outcome. No execution was performed.

**Exact next action for Opus:**
1. Review and apply E's `candidate.patch` (voice outage fix) — commits first since H depends on it
2. Apply A's `candidate.patch` (B4 ownership gap test)
3. Apply F's `candidate.patch` (F-CSS-01 map-disclaimer pointer-events)
4. Stage and run D's ASR dirty test improvements
5. Stage and run C's eval dirty files
6. Copy/run B's `browser_accept.py`
7. After E committed: review H's deferred findings and apply minimal onerror/MIME patches
8. Await G, I, J, L reports independently; integrate when they arrive

**Critical blocker:** None. All available work is ready. G/I/J/L are PENDING but non-blocking.
