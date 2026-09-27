# opus handoff — round-3 + mac-final integration

Status: INTEGRATED (local commits on CLEAN, nothing pushed). Workstation migration cancelled.
Final code revision `0d290da`; harness `e7e2171`; docs committed after these (see `git log`).

## Checkpoint recovery (historical, 15:49 IST)

Worker 20 ran `git checkout -- eval/main_test.go` and workers created two stashes that removed tracked
dirty work. The nine tracked files were restored from unreachable stash `2e96cfb` with
`git restore --source=2e96cfb --worktree` (no reset, no stash pop). `fb5c358` differed only by Worker 10's
CSS patch (rejected), a HEAD `main_test.go` and the W28 doc edit.

## Stage commits

Evidence below is current (2026-09-27 17:00–18:00 IST, this Mac) unless marked historical.
"Staged tree" = the exact index content exported with `git checkout-index` and served by its own Vite.

| Commit | Scope | Evidence |
|---|---|---|
| `a79c461` | ASR decoder / TTS WAV boundary tests | historical: asrworker/ttsworker `go test -race` ok, 14 + 17 PASS |
| `7100466` | Eval per-case context + oracle negatives | historical: middleworker vet/test ok; new tests fail with the fix removed |
| `2444365` | Ownership regression: foreign denial leaves owner stay and key untouched | current: see "Backend" below |
| `e883692` | Voice outage shows `assistantUnavailable`, no place-lookup fallback | current: `outage` 4/4; real worker stop/restart 2/2 |
| `11ddcfc` | Recorder generation; getUserMedia-after-cancel, `onerror`, `start()`/constructor failure release the mic; MIME from backend allow-list (none → mic never opened); visible `micStopped`; language switch cancels recording | staged tree: tsc 0, `npm test` 90/90, build 0, browser `recorder` 10/10, `core` 4/4. Negative (HEAD `e883692`): b, e, f, g, h, i FAIL |
| `1fa3fed` | Pending `styles.css` (Gemini delta from `2e96cfb`) accepted | staged tree: `layout` 18/18. Previous CSS: guidance panel `display:none` below 48rem (the guidance card never becomes visible at 375 px); at 1024/1280/1440 px Directions/Listen/ISL/Ask-by-voice are covered by the 112 link (`elementFromPoint`) |
| `086db86` | `AudioPlaybackGuard.claimPlayback()` owns the playing clip; `invalidate()` stops it | staged tree: 92/92, `audio` 8/8, `audio-denied` 3/3. Negatives: HEAD keeps playing after new request / language switch; reversed claim order (naive pause-all) stops the new clip (browser + 2 unit FAIL) |
| `74dd911` | Escape closes the topmost overlay (one listener at bootstrap); focus moves into dialogs and returns to the opener across re-renders | staged tree: `dialog` 5/5, `recorder` 10/10, `audio` 8/8. Negatives: HEAD 0/4 cycles; listener moved into `bindInteractions` closes two overlays per Escape |
| `102176c` | Reservation outcome-unknown / destination-locked message rendered outside the confirm dialog | staged tree: `reservation` 11/11. Negative: HEAD shows no message after a committed-but-lost response |
| `0d290da` | Voice `RECENTER` clears `mapTilted`, terrain and `aria-pressed` | staged tree: `map` 6/6. Negative: HEAD pitch 65 / terrain / pressed after voice recenter |
| `e7e2171` | `plan/evidence/browser/prototype_accept.py` sections: language, reservation, recorder, audio, audio-denied, dialog, map, layout | final combined run below |

### Final combined acceptance at `0d290da` (+ harness `e7e2171`)

- `npx tsc --noEmit` 0; `npm test` 92/92; `npm run build` 0; hooks absent from `dist` (0 matches).
- `prototype_accept.py … core outage language reservation recorder audio audio-denied dialog map layout`:
  **73 PASS, 0 FAIL, exit 0**. Real worker outage (mock-workers killed → 503 + assistant-unavailable;
  restarted → 200 + template): 2/2.
- Stack (owned): `/tmp/sthira-opus-final/bin/{mock-workers -scenario destination-choice,sthira-exercise}`
  built from HEAD, Vite on the checkout, ports 18690–18694, disposable DB `sthira_opusfinal_1790509210`
  (migrations 0001–0010, `ON_ERROR_STOP`). Chromium: Playwright 1.61 headless shell (installed cache).
- Instrumentation limits: fake Chromium microphone and API wrappers; the audio section replaces only the
  approved WAV bytes with a 6 s clip and recomputes size/SHA-256; the map section reads (never sets) the
  MapLibre instance via `?sthira-test-hooks=1`; voice `RECENTER` is a network-rewritten action. Autoplay
  denial is produced by `--autoplay-policy=user-gesture-required` plus a 6 s response delay.

### Backend

`go test -tags integration -count=1 -v ./internal/httpserver/` with a fresh migrated `STHIRA_TEST_DSN` and
`STHIRA_TEST_ADMIN_DSN=postgres://localhost:5432/postgres`: exit 0, 211 top-level PASS, 240 `--- PASS`
lines incl. subtests, 0 FAIL, 0 SKIP. One of those was the assertion-free coverage-map test (removed
below). A first run with a socket-only admin DSN failed 3 C05 tests with "Admin DSN must specify a
hostname" — an invocation error in the rehearsal script's requirement, not a product failure.
**238 vs 211 reconciled:** 211 is the top-level count; 238 (at `41a0278`) and 240 (now) count every
`--- PASS` line including subtests. No backend code changed in this session.

## Mac-final workers

| Worker | Disposition | Reason |
|---|---|---|
| mac-final 1 (recorder) | accepted with corrections → `11ddcfc` | Boolean `recordingCancelled` still let a cancelled recorder's late `onstop` submit and let a late getUserMedia win; replaced by a generation with recorder-local stream. `micStopped` was never rendered (dead `voiceFeedbackKey`), now shown via `commandError`. `audio/wav` dropped from the recorder list (MediaRecorder does not produce it). Its 6 tests only covered the MIME helper |
| mac-final 2 (playback) | accepted with corrections → `086db86` | Correct ownership idea; reduced `setActive(gen)`/`releaseActive`/getter to one `claimPlayback()` so ordering cannot be misused; its 8 unit checks were isolated-copy only and are replaced by the committed tests and browser checks |
| mac-final 3 (dialog/recenter/CSS) | accepted with corrections | Escape/focus findings confirmed and implemented (`74dd911`). Its "voice recenter consistent" verdict checked only `recenterMap()`, not the voice `RECENTER` path (fixed `0d290da`). CSS verdict confirmed by hit-testing, not source reading. `main.ts.scratch`/`styles.css.scratch` not used |
| mac-final 4 (acceptance) | accepted with corrections | Exit-code finding on round-3 W2 `outage_accept.py` accepted by inspection. Its lost-response check aborted before the server received the request, so it could not prove retry after an uncertain outcome; replaced by commit-then-drop plus a DB count. Script not committed |

## Round-3 workers 1–30

Totals: accepted 2 · accepted with corrections 11 · rejected 7 · no change needed 7 · missing/blocked 3.

| # | Assignment | Disposition |
|---|---|---|
| 1 | Core browser harness | accepted with corrections — journeys rewritten into `e7e2171`; script not committed |
| 2 | Outage regression | accepted with corrections — fix is `e883692`; its script exits 0 on failures, not committed |
| 3 | Delayed-language regression | accepted with corrections — old check did not distinguish old/new code; `language` section uses distinct EN/HI templates |
| 4 | Layout script | accepted with corrections — `layout` section adds hit-testing and HI/ML |
| 5 | Recorder errors (`onerror`) | accepted with corrections — stale base; idea in `11ddcfc` |
| 6 | Recorder MIME | rejected — "no change needed" contradicted: HEAD opened the mic with no supported type (check h) |
| 7 | Mic permission/stream cleanup | rejected — "no change needed" contradicted: HEAD left the mic live after cancel and start failure (checks b, g) |
| 8 | Playback cancellation | rejected — pausing `pending` is a no-op for the playing clip |
| 9 | Dialog accessibility | rejected — keydown listeners added per render; defect reimplemented in `74dd911` |
| 10 | CSS `.map-disclaimer` | rejected — `pointer-events:auto` under a `none` parent is correct; hit-test shows the link clickable and the box transparent |
| 11 | Remove `my-location` layer id | rejected as unnecessary — `getLayer`-guarded no-op |
| 12 | Recenter/tilt state | accepted with corrections — voice RECENTER desync fixed in `0d290da`; layer part duplicated W11 |
| 13 | Map asset/render probe | accepted with corrections — `map` section checks rendered features |
| 14 | Arrival geolocation | accepted with corrections — covered by `reservation` (emulated GPS → server ARRIVE → DB `ARRIVED`) |
| 15 | Lost reservation response | accepted with corrections — commit-then-drop browser check; its Node tests not committed |
| 16 | Pending stay restore | accepted with corrections — reload check in `reservation` |
| 17 | Cross-citizen patch review | no change needed — covered by `2444365` |
| 18 | Denied mutation semantics | no change needed — covered by `2444365` |
| 19 | Eval omitted visibility | no change needed — covered by `7100466` |
| 20 | Eval per-case context | rejected — 0-byte patch; ran a destructive `git checkout` |
| 21 | Eval negative status/skip | missing — no report or artifacts anywhere under `plan/worker-reports` (rechecked) |
| 22 | ASR MIME evidence | no change needed |
| 23 | TTS timeout gap | no change needed — probe not run |
| 24 | Speech metadata regression | blocked — Go test artifact not executed or integrated this session |
| 25 | Evidence/fixture review | accepted — coverage-map test has no assertions; removed from the tree |
| 26 | Presenter script | no change needed (document) |
| 27 | Human checklist | no change needed (document) |
| 28 | Launch doc delta | accepted with corrections — invented `approved_voices.json` path and unsupported 44,100 Hz removed; voices-file-unset is a BLOCKED probe, not "0 voices" |
| 29 | Command runner | missing — no report or artifacts (rechecked) |
| 30 | Integration manifest | accepted as planning input; superseded by this record |

## Debris (not committed)

Moved out of the repo to `/tmp/sthira-opus-final/debris/` with `SHA256SUMS` (tmp is not durable; copy if
wanted): `Oops.rej`, `Oops.rej.orig` (W28 rejects), `mapActions.ts.{bak,orig}` (identical to HEAD),
`mapActions.ts.{copy,rej}` (W11), `backend/internal/middleworker/eval/eval` (Mach-O build output),
`backend/internal/httpserver/citizen_ownership_regression_test.go` (assertion-free coverage map).
Untracked and left as-is: `plan/prompts/`, `plan/worker-reports/{round-2,round-3,mac-final,*.md}`,
`plan/reviews/opus-checkpoint-review-round-3.md`, `.ai/`.

## Remaining (not verified here)

Real microphone and speaker, Safari (remote automation disabled), human speech review, real models
(BLOCKED_HARDWARE / no paid window), true 200 % browser zoom, backgrounding via real tab hide.
Lower-priority findings not fixed: `voiceFeedbackKey` is written but never rendered; a re-render while
typing keeps focus in the command input but clears its text; `focusRoute` ignores a manual pitch.
