# Round 2 integration record (Opus) — A–F, H–L; G pending

Date: 2026-09-27 · HEAD d95df9e · Execution tests deferred by user. Everything below is UNVERIFIED.
No commits made: nothing here is execution-verified.

## Source hash check
All base hashes cited by workers match the pre-integration tree (main.ts ed9dbcd5…, styles.css 1951333f…,
stay_http_integration_test.go 9698428f…, stay_handlers.go 7b745f4f…, idempotency.go 54114bdc…,
tts audio_mime_test.go f35b88f1…, eval blobs 165f9f15/104537b6/7e8b937d/16ad9c86 — C labelled these git blob SHA-1s as SHA-256).
Pre-existing dirty files (ASR/TTS audio_mime tests, eval/*, styles.css, citizen_ownership_regression_test.go)
have mtimes 14:38–14:52, before the prompts (15:02). No worker edited shared source.
Both `candidate.patch` files (A, E) are malformed (`git apply --check`: corrupt patch); applied by hand.

## Integrated into working tree (UNVERIFIED)
| Worker | Change | Post-integration SHA-256 | Checks run |
|---|---|---|---|
| E | main.ts: removed 503/504 transcript → `resolvePlace` fallback; outage now reaches existing `commandUnavailable`/`backendUnavailable` catch path. `selectCandidatePlace` (only other caller) untouched. | main.ts 00dae4e6… | `tsc --noEmit` OK (compile only) |
| A | stay_http_integration_test.go: `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives`. Rewrote worker comments that wrongly called A's first ARRIVE a "replay"; added `held==0` assertion. | 053ed7b9… | `go vet ./internal/httpserver/` OK (compile only) |

## Pre-existing dirty work — reviewed, left as-is (UNVERIFIED, not committed)
- C: eval delta consistent; corpus has 13 lines = `wantV2`. Follow-up: add test for model omitting `visible`.
- D: ASR audio_mime_test delta OK. D's report cites `minimax-d/candidate.patch`, which does not exist (no patch needed).
- I: TTS audio_mime_test delta OK. Optional follow-up: isolated per-call-timeout test.
- styles.css (Gemini-owned): no change applied, see F.

## Rejected / needs rework
- F-CSS-01 REJECTED: the claim is wrong. `pointer-events:none` on the parent does not block a child set to
  `pointer-events:auto`; the Esri link remains clickable. Don't apply.
- E `voice-outage-fallback.reticle.js` NEEDS_REWORK: uses testids that don't exist in main.ts
  (`voice-open-btn`, `command-error-msg`, `command-submit`) and never injects the 503 (commented out).
- L `stale-response-language-switch.spec.ts` NEEDS_REWORK: never submits a command, so no request is
  in flight; uses selectors that don't exist (`[data-lang=…]`, `.caption-text`, `.command-response`,
  `.facility-name`); uses `waitForTimeout` as its only sync; `.catch(() => '')` makes assertions vacuous.
  Result: it would pass whether or not the guard works. Its source findings (supersedeInFlight/shouldDropResponse) are accurate.
- K queue SUPERSEDED by the queue below: it lists I/J/L as PENDING and cites an F candidate.patch that does not exist.
- J: F9 is moot because the runbook already says `max_tokens 512`. F3 is confirmed (sthira-exercise main.go:290–295).
  launch-checklist.md is not yet merged into plan/evidence/real-inference-launch-check.md.
- H: no patch. Confirmed `MediaRecorder.onerror` unset and the MIME fallback unvalidated (main.ts:1099–1101).
  Finding 5 contradicts itself: hide does clear `lastApprovedAudio` via supersedeInFlight, but a playing Audio element isn't paused.

## Post-reset test queue (all NOT_RUN)
1. A: `cd backend && STHIRA_TEST_DSN=… go test -count=1 -run 'TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives|TestHTTPCrossSessionDenial|TestHTTPDuplicateConfirmationReplay|TestB02_|TestHTTPArriveDepartFlow' ./internal/httpserver/` → PASS.
2. C: `cd backend && go test -count=1 ./internal/middleworker/eval/ ./internal/middleworker/` → PASS. Also run corpus-check, and benchmark mode must exit 2.
3. D: `cd backend && go test -race -count=1 ./internal/asrworker/` (ffmpeg on PATH) → PASS.
4. I: `cd backend && go test -race -count=1 ./internal/ttsworker/` → PASS.
5. E: `cd frontend/v2 && npm test && npm run build`. Manual check: route `/api/v3/voice/process` to 503/504, submit "Show my route",
   and expect `.command-error` = commandUnavailable with no "Location … not found". Restore and expect a normal response.
   Negative control: at pre-E main.ts (ed9dbcd5…), "not found" appears.
6. B: `python3 plan/worker-reports/round-2/minimax-b/browser_accept.py http://127.0.0.1:18492/` (stack + playwright) → all PASS.
   Unconfirmed: whether `[data-action="route"]` alone creates the reservation, or whether `start-route`/`reservation-confirm-submit` is required.
7. F: `layout-check.spec.ts` at 375/1024/1440 plus a manual 200% zoom check. Its `.map-disclaimer` link assertion should expect clickable.
8. L and E browser regressions: rewrite first (see above), then run with their negative controls.
9. H: real-device checklist items 1–10 from H's report.
10. G: pending. Integrate after its report is final.
