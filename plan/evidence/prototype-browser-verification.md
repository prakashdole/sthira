# Prototype verification by evidence class — 2026-09-27

## Current: `329633b`, 2026-09-27 ~19:10 IST

Same harness and stack type (fresh disposable DB, mock-workers destination-choice, ports 18790–18792,
Python with Playwright from `~/.venvs/scrapling`). All 11 sections incl. new `draft`: **76 PASS, 0 FAIL,
exit 0**. `npm test` 92/92, tsc/build 0, hooks absent from `dist`. `draft` (3 checks): a GPS update
while tracking replaces `#command-input` and the typed text survives; at `c1966f9` the text is lost
(FAIL ''). asrworker/ttsworker/middleworker module tests ok; `corpus-check` v1/v2 ok.

## Previous: mac-final integration at `0d290da` (harness `e7e2171`), 2026-09-27 ~17:55 IST

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

## Remaining manual acceptance (owner, Safari or phone, about 15 minutes)

1. Real microphone permission, both granted and denied. Speak one command; check the transcript and the caption.
2. Audio playback after a tap gesture. Replay must be refused after a language switch.
3. Phone-width layout, keyboard Tab order, and long Malayalam and Hindi text.
