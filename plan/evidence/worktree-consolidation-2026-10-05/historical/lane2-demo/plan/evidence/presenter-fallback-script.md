# Presenter Fallback Script — One Paragraph Per Failure

**File:** `plan/evidence/presenter-fallback-script.md`
**Stack:** lane2-demo worktree, exercise backend + mock-workers + Vite on 127.0.0.1:18710/5173
**Tested:** 2026-09-27, lane2-demo `dee8e18` + CLEAN `0ceafc6`

---

## (a) Workers Down

**Trigger:** Stop the Go backend (`kill <pid>`) or stop mock-workers.

**On-screen text:**
> "Voice Map Control is unavailable. The displayed route and emergency call option still work."

(EN; HI: "वॉयस मैप कंट्रोल उपलब्ध नहीं है। दिखाया गया मार्ग और आपात कॉल विकल्प काम करेंगे।"; ML: "വോയ്സ് മാപ്പ് നിയന്ത്രണം ലഭ്യമല്ല. കാണിച്ച വഴിയും അടിയന്തര കോൾ ഓപ്ഷനും പ്രവർത്തികും.")

**HTTP response:** 503; `"error":"MODEL_UNAVAILABLE"`; body: `"voice pipeline is unavailable: model worker is not configured"`

**Other UI state:** Route displayed on map. 112 link reachable. Layer toggles, 3D, language switch still functional. Reservation/arrival/voice commands fail with the above banner.

**Recovery:** Restart backend or workers → `curl /health/ready` returns `200` with `"status":"READY"` → next request succeeds.

---

## (b) Network Loss (Browser Offline)

**Trigger:** Playwright `ctx.set_offline(True)` (or OS/airplane mode in a real browser).

**On-screen text:**
> "Offline, saved demo guidance"

(EN; HI: "ऑफ़लाइन, सहेजा गया डेमो मार्गदर्शन"; ML: "ഓഫ്‌ലൈൻ, സംരക്ഷിച്ച ഡെമോ മാർഗനിർദേശം")

**Console:** No JavaScript errors. No error banner beyond the `offline` status banner.

**Other UI state:** Map may continue to display cached tiles. 112 link still reachable ("Trapped or cut off? Call 112 for rescue"). Layer toggles still interactive (5 aria-pressed elements confirmed). Guidance text and route displayed from in-memory state.

**Recovery:** `ctx.set_offline(False)` → network reconnects → `checkRuntime()` calls `/health/ready`, receives 200 → runtime returns `'demo'` → banner clears. App auto-recovers within ~2 s.

---

## (c) Autoplay Blocked

**Trigger:** Browser autoplay policy blocks audio playback without prior user interaction.

**On-screen text:** No error banner. The listen/replay button appears after a tap gesture. Audio does not play automatically; user must tap to play.

**Proof:** `prototype_accept.py` section `audio-denied` (line 506): autoplay refused → replay tap required → `claimPlayback` order verified.

**Other UI state:** Guidance text always visible as fallback. 112 reachable. No spinner stuck.

**Recovery:** Tap the listen button to play audio. Or: change language → replay refused (correct) → new guidance in new language plays after user tap.

---

## (d) Microphone Permission Denied

**Trigger:** Browser microphone permission set to Block/Deny for the app.

**On-screen text:** No generic spinner. The UI shows the mic button in an error state with a message indicating the mic is unavailable.

**Proof:** `prototype_accept.py` section `recorder` (line 321): permission denial → `error` event fires → `micStopped` message shown → mic released immediately. No stale audio sent.

**Observable:** After tapping mic with permission denied, the UI does NOT hang. A visible message indicates the failure within ~1 s. The guidance text remains accessible.

**Recovery:** Grant microphone permission in browser settings and reload the page.

---

## Observed Banner Summary

| Scenario | i18n key | EN text |
|---|---|---|
| Browser offline | `offline` | "Offline, saved demo guidance" |
| Backend/workers down | `assistantUnavailable` | "Voice Map Control is unavailable. The displayed route and emergency call option still work." |
| Backend up, workers unreachable (503) | `blocked` | "Demo ready, live sources not connected" |
| Voice command fails (per-command) | `backendUnavailable` | "Voice Map Control is unavailable." |
| No speech recognition | `recognitionUnavailable` | "Local speech recognition is unavailable. Enter a command below instead." |
| Mic denied | `micStopped` | "Microphone input stopped. Check permission or enter a command below." |

All six keys from `frontend/v2/src/i18n.ts:31-32`.
