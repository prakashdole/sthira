# Prototype verification by evidence class — 2026-09-27

## Current: mac-final integration at `0d290da` (harness `e7e2171`), 2026-09-27 ~17:55 IST

Headless Chromium (Playwright 1.61 headless shell, installed cache), owned synthetic stack
(`sthira-exercise` + `mock-workers -scenario destination-choice` built from HEAD, disposable DB with
migrations 0001–0010), Vite on the checkout. Command:
`STHIRA_ACCEPT_DB=<disposable db> python3 plan/evidence/browser/prototype_accept.py http://127.0.0.1:18692/ core outage language reservation recorder audio audio-denied dialog map layout`
→ **73 PASS, 0 FAIL, exit 0**. Real worker outage (process stopped → 503 + assistant-unavailable,
restarted → 200 + template): 2/2. `npm test` 92/92, tsc/build 0, test hooks absent from `dist`.

| Section | Checks | What is observed |
|---|---|---|
| core / outage | 4 / 4 | hooks absent; backend destination; 503/504 honest message; recovery |
| language | 4 | late en-IN 200 ignored after switch; fresh hi-IN template shown |
| reservation | 11 | server commits, response dropped → visible "not confirmed"; byte-identical retry → 200 replay; DB +1 stay; repeat Start Route no POST; GPS near → ARRIVE 200, DB `ARRIVED`; reload keeps the stay |
| recorder | 10 | stop submits one webm/opus request; cancel during getUserMedia, cancel+restart, language switch, `error` event, `start()` failure, no supported MIME, permission denial: mic released, nothing stale sent, message shown |
| audio / audio-denied | 8 / 3 | autoplay keeps playing; new request, language switch, offline stop it; replay refused after switch; browser-denied autoplay never plays and tap replays |
| dialog | 5 | 4 open/re-render/Escape cycles return focus; one Escape closes only the top overlay; focus returns to the same opener instance |
| map | 6 | rendered route features; 2× 3D/recenter agreement of pitch, terrain, aria-pressed; voice RECENTER; red-zone layer shown/hidden with drawn features |
| layout | 18 | 375/1024/1440 × EN/HI/ML: no overflow; route, quick actions, 112, voice entry hit-testable; map disclaimer passes gestures, link clickable; details dialog fits |

Negative controls (same harness): HEAD `e883692` fails the new recorder, audio, dialog, voice-recenter and
reservation-message checks; a listener-per-render variant fails the one-Escape check; a reversed
`claimPlayback` order fails "autoplay keeps playing"; the previous CSS hides the guidance card at 375 px
and covers the quick actions at 1024–1440 px.
Limits: fake microphone and API wrappers, a size/checksum-correct replacement WAV, network-rewritten
RECENTER, read-only map access via `?sthira-test-hooks=1`. Not run: real microphone/speaker, Safari,
real models, true 200 % zoom, real tab backgrounding.

## Historical sections (below)

Coordinator record at `e066056` (frontend) / `5d57c9b` (HEAD when written). It supersedes two
earlier drafts from the verification worker. The first draft marked two items PASS that were
defects at its revision: late results after a language switch, and mutable test hooks in the
production build. Both were fixed in `aa97e69`. The second draft ran a real browser, but at
`6861d42` plus uncommitted edits. Its findings are kept below as historical.

| Class | Status | What was actually run |
|---|---|---|
| Frontend unit tests | PASS | `npm test` 89/89; `npm run build` PASS (tsc + vite) |
| Production bundle | PASS | `grep 'setActiveStayId\|setLastApprovedAudio\|sthira-test-hooks' dist/assets/*.js` → 0 matches |
| DB/HTTP integration | PASS | `go test -tags integration ./internal/httpserver/ -count=1` on a fresh disposable DB: 238 PASS, 0 FAIL, 0 SKIP, exit 0 (at `41a0278`; no backend handler changed afterwards). `TestProto*` was re-run after the new check that each proposal's `data_version` matches guidance: PASS |
| Browser (headless Chromium) | PASS 9/9 at `e066056` | See the next section |
| Browser outage/recovery | PASS 2/2 | Stopped mock workers → 503 with no success message. Restarted them → next request 200 |
| Mock plumbing (curl via Vite proxy) | PASS | Guidance `PKGDEMO-1:1`. en-IN and hi-IN proposals carry `data_version=PKGDEMO-1:1` and `SHOW_CHOICES [FACDEMO-1]`. Reserve 201; identical retry 200 with the same stay; same key with a different payload 409 `IDEMPOTENCY_CONFLICT`; GET → `RESERVED FACDEMO-1`; ARRIVE 200; unknown stay 404 |
| Adapter/decoder | PASS (local) | ASR `DecodeAudio`: webm/ogg Opus via the local ffmpeg; non-PCM WAV rejected. Webm bytes labelled `audio/ogg` are accepted because ffmpeg detects the format from content (recorded behaviour, not a guarantee). TTS WAV boundaries. Middle-worker strict decode of `SET_LAYER_VISIBILITY` (the test does not build against the pre-fix struct) |
| Eval corpus | PASS (offline) | `corpus-check`: v1 15 cases, v2 13 cases, unique IDs, only known fixtures. 4 oracle subcases SKIP by design |
| Safari | NOT_RUN | See "Safari blocker" below |
| Real microphone | NOT_RUN | Needs a person and a real device |
| Real models | NOT_RUN | No paid session authorized |
| Human speech review | NOT_RUN | hi-IN and ml-IN corpus text is `DRAFT_REQUIRES_NATIVE_REVIEW` |

## Browser acceptance (headless Chromium, real UI)

- **Browser:** Playwright Chromium headless shell, already in `~/Library/Caches/ms-playwright`; nothing was installed.
- **Stack:**
  - frontend: `git archive HEAD` copy served by Vite;
  - backend: `sthira-exercise`, Postgres (disposable DB) and `mock-workers -scenario destination-choice`.
- **Instrumentation:** a 2.5 s delay injected at the network layer, and emulated browser geolocation at the demo shelter. Only visible controls were used. `window` hooks were confirmed absent.
- **Scripts:** kept outside the repo at `/tmp/w3keep/{accept,outage}.py`.

| Check | At `e066056` | Same script against `fc0128b` (before the fixes) |
|---|---|---|
| Test hooks absent without opt-in | PASS | FAIL |
| Guidance destination rendered from backend | PASS | PASS |
| Language switch while a request is pending clears the spinner immediately | PASS | FAIL (input stayed disabled) |
| Late EN response ignored after switching to HI | PASS | PASS (does not distinguish old and new code in this scenario) |
| Current request after the switch applies normally | PASS | PASS |
| Repeated Start Route creates no second reservation | PASS (1 POST; stay pinned to FACDEMO-1) | FAIL (2 POSTs) |
| Arrival recorded after the server acknowledges it (GPS near → confirm) | PASS | FAIL (tracking never started) |
| 390 px viewport: no horizontal overflow | PASS | PASS |
| No uncaught page errors | PASS | PASS |

**Observed limitations**
- MapLibre asset requests from the symlinked `node_modules` returned 403 in the temporary copy. This is a verification setup artifact.
- During a worker outage, a typed command falls back to place lookup and shows `Location "…" not found`. This is honest, but it is not a clear "assistant unavailable" message.

## Safari blocker (observed 2026-09-27)

`/usr/bin/safaridriver -p 15123` starts. `POST /session` with
`{"capabilities":{"alwaysMatch":{"browserName":"safari"}}}` then returns
`session not created: You must enable 'Allow remote automation' in the Developer section of Safari Settings`.
The setting was not changed. The verification worker had earlier downloaded Playwright WebKit into `/tmp/pw-browsers`; that launch closed immediately and the cause was not established.

## Historical (verification worker, `6861d42` plus uncommitted edits)

Headless Chromium covered onboarding, the text command, the 3D, layer and recenter controls, the source-details panel, EN/HI switching, and viewports 375, 1024 and 1440 with no overflow and no console errors. The worker's outage check failed because its own request format was wrong. This run did not include any of the coordinator fixes.

## Owner Acceptance Checklist (worker-27)

**Source:** `plan/worker-reports/round-3/worker-27/checklist.md` (original repo, HEAD d95df9e)
**Status: NOT_RUN** — requires human operator with real hardware; no automated substitute.

**Blockers for Safari rows:**
- Safari remote automation requires enabling "Allow remote automation" in Safari Developer Settings (device setting, not an app change).
- Some rows require specific hardware (mic present/absent, physical keyboard, real iPhone/iPad).
- Zoom tests require browser zoom setting access (Cmd+/Cmd-); Playwright viewport API resizes the window, not the browser zoom.

**Time estimate:** ~20–30 minutes for a single operator covering all rows.

### Section 1 — Microphone Permission

**1A — Permission Granted**
- Setup: Reset browser microphone permission for the app. Reload fresh.
- Action: Tap the mic button, grant permission, speak one sentence in Hindi, Malayalam, or English.
- Observable Success: Transcript appears below the orb. Audio guidance plays within ~3 s of release. No crash.
- Failure Record: Transcript absent/wrong language. Spinner > 5 s. Permission error toast. Audio from wrong speaker.
- Evidence: Screenshot of transcript + orb state after release.

**1B — Permission Denied**
- Setup: Set browser mic permission to Block/Deny for the app. Reload fresh.
- Action: Tap the mic button.
- Observable Success: Visible feedback indicating `'micPermissionDenied'` — NOT a generic spinner.
- Failure Record: Generic spinner that never resolves. No feedback that permission was denied.
- Evidence: Screenshot of UI state after tapping with denied permission.

**1C — Device Not Found (No Microphone)**
- Setup: Device with no mic, or Block All microphone access.
- Action: Tap the mic button.
- Observable Success: Feedback indicates `'micNotFound'`, distinct from permission-denied.
- Failure Record: App silently attempts to record with empty audio. Same spinner as granted case.
- Evidence: Screenshot of feedback state.

### Section 2 — Audible Playback and Replay

**2A — Guidance Audio Plays After First Receipt**
- Setup: Complete a voice exchange that returns audio guidance (e.g., "Start Route" proposal).
- Action: Wait for audio to begin playing automatically. Observe listen button state.
- Observable Success: Audio audible through speaker. Listen/replay button appears after audio completes. UI does not freeze.
- Failure Record: No audio. UI spinner stays active. Listen button never appears.
- Evidence: Screenshot of listen button after guidance ends.

**2B — Replay Refused After Language Switch**
- Setup: Receive audio guidance in Language A. Listen button is visible.
- Action: Switch UI language to Hindi or Malayalam. Tap the old-language listen button.
- Observable Success: Replay refused — button disappears, message shown, or new-language guidance fetched instead.
- Failure Record: Old-language audio plays after language switch. Audio and UI language out of sync.
- Evidence: Screenshot of refusal message or button disappearance.

**2C — Replay Available Within Same Language Session**
- Setup: Receive audio guidance in Language A. Do not switch language.
- Action: Tap listen/replay button without changing language.
- Observable Success: Audio replays correctly from beginning. No new request.
- Failure Record: Replay fails or triggers a new backend request.
- Evidence: Screenshot of successful replay.

### Section 3 — Intelligibility Review

**3A — Hindi (hi-IN) Speech Intelligibility**
- Setup: UI language set to Hindi. Speak a Hindi phrase.
- Action: Listen to returned Hindi audio guidance. Compare to transcript/on-screen text.
- Observable Success: Audio is clear, matches proposal meaning, comprehensible to Hindi speaker. No hallucinations.
- Failure Record: Guidance unrelated to proposal. Silent. In wrong language.
- Evidence: Written note of transcript vs. audio correspondence.

**3B — Malayalam (ml-IN) Speech Intelligibility**
- Same structure as 3A, for Malayalam.

**3C — English (en-IN) Speech Intelligibility**
- Same structure as 3A, for English.

### Section 4 — Real Safari / Mobile Viewport

**4A — iPhone Portrait (375 × 667 or similar)**
- Setup: Open app on iPhone in portrait. Use Safari Web Inspector or visual inspection.
- Observable Success: No horizontal overflow. Text wraps. Buttons ≥ 44 px tap target. Hindi/Malayalam labels in language switcher fit without clipping. No content hidden behind keyboard.
- Failure Record: Horizontal scrollbar. Label clipped. Text overflows. Touch targets < 44 px.
- Evidence: Screenshot with red-border annotation on any overflow.

**4B — iPhone Landscape**
- Observable Success: Layout adjusts gracefully. Map remains visible. Guidance panel does not overlap map controls.
- Failure Record: Guidance panel overlaps map. Some controls inaccessible.
- Evidence: Screenshot in landscape.

**4C — iPad (if available)**
- Observable Success: Layout uses additional width for side panel. No regressions from iPhone layout.
- Failure Record: Layout identical to iPhone. Unexpected whitespace or truncation.
- Evidence: Screenshot on iPad.

### Section 5 — Keyboard Interaction

**5A — Tab Order Reaches All Interactive Elements**
- Setup: Load app with attached physical keyboard. Press Tab from address bar into page.
- Action: Press Tab repeatedly. Observe focus order on guidance panel, map toolbar, voice suggestions, modals.
- Observable Success: Logical top-to-bottom, left-to-right order. All buttons/links/inputs reachable. Focus visible. Modals receive focus.
- Failure Record: Focus skips visible elements. Focus lands on hidden/disabled elements. No focus ring.
- Evidence: Numbered focus order list.

**5B — Enter Activates Focused Button**
- Action: Tab to mic/voice launch button. Press Enter or Space.
- Observable Success: Voice action activates (recording starts or console opens).
- Failure Record: Enter/Space does nothing. Focus moves but Enter does not activate.
- Evidence: Note whether Enter activates the button.

**5C — Escape Closes Modal Dialogs**
- Setup: Open a modal dialog (e.g., source details).
- Action: Press Escape.
- Observable Success: Modal closes. Focus returns to opener or last active element.
- Failure Record: Escape does not close modal. No keyboard path to close.
- Evidence: Note whether Escape closes the modal.

### Section 6 — True Browser Zoom

> Playwright viewport API resizes the window, not browser zoom. Use browser zoom settings (Cmd+/Cmd- on Mac).

**6A — UI at 100% Zoom (Baseline)**
- Observable Success: All content fits viewport. Text legible. No overflow.
- Evidence: Screenshot at 100% zoom.

**6B — UI at 200% Zoom**
- Setup: Browser zoom to 200% (Cmd++ twice). Do not resize window.
- Observable Success: Hindi/Malayalam labels fully visible in language switcher. Emergency buttons remain legible with ≥ 44 px tap target. Guidance h1 readable without horizontal scroll.
- Failure Record: Labels clip/overflow at 200% zoom. Emergency button text overlaps. Horizontal scroll required.
- Evidence: Screenshot at 200% zoom with annotations.

**6C — UI at 50% Zoom (Wide Viewport)**
- Observable Success: Layout usable. Guidance panel and map do not overlap unexpectedly.
- Failure Record: Layout collapses or overlaps at low zoom.
- Evidence: Screenshot at 50% zoom.

### Summary Table

| # | Section | Item | Device Required | Hardware Required |
|---|---------|------|-----------------|-------------------|
| 1A | Mic | Permission granted | Any | Mic + speaker |
| 1B | Mic | Permission denied | Any | — |
| 1C | Mic | Device not found | Any | — |
| 2A | Playback | First guidance audio | Any | Speaker |
| 2B | Playback | Replay refused after lang switch | Any | Speaker |
| 2C | Playback | Replay within same language | Any | Speaker |
| 3A | Intelligibility | Hindi speech quality | Any | — |
| 3B | Intelligibility | Malayalam speech quality | Any | — |
| 3C | Intelligibility | English speech quality | Any | — |
| 4A | Viewport | iPhone portrait | iPhone/Safari | — |
| 4B | Viewport | iPhone landscape | iPhone/Safari | — |
| 4C | Viewport | iPad | iPad/Safari | — |
| 5A | Keyboard | Tab order | Any + keyboard | Physical keyboard |
| 5B | Keyboard | Enter activates | Any + keyboard | Physical keyboard |
| 5C | Keyboard | Escape closes modal | Any + keyboard | Physical keyboard |
| 6A | Zoom | 100% baseline | Any | — |
| 6B | Zoom | 200% real zoom | Any | — |
| 6C | Zoom | 50% real zoom | Any | — |
