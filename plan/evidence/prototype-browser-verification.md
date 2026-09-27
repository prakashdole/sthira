# Prototype verification by evidence class — 2026-09-27

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
