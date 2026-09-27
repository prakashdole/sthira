# Human Acceptance Checklist — Real Device Browser Verification
**Output:** `plan/worker-reports/round-3/worker-27/checklist.md`
**Worker:** worker-27 | **Assignment:** round-3
**Timestamp:** 2026-09-27T19:05:00Z | **HEAD:** d95df9e

---

## Purpose

Executable-by-human acceptance checklist for five browser behaviors that require real hardware, real browser settings, or real microphone input — and cannot be verified by automated scripts or Playwright viewport emulation. Each row has five fields: **Setup**, **Action**, **Observable Success**, **Failure Record**, and **Evidence** (screenshot or photo).

Intelligibility review of transcribed or synthesized speech is kept separate (Section 3) from UI language switching.

---

## Prerequisites

- App loaded on a **real device** (iPhone/iPad with Safari, or Android with Chrome)
- **Microphone** physically present and functional
- **Browser zoom** resettable to 100% before starting
- Dev server or deployed build reachable on localhost or a real host
- No changes to browser settings beyond what the checklist requests
- Human operator who can speak, listen, tap, and observe

---

## Section 1 — Microphone Permission

### 1A — Permission Granted

| Field | Content |
|-------|---------|
| **Setup** | Reset browser microphone permission for the app (Settings → Safari → Microphone → Allow, or Chrome → Permissions → Microphone → Allow). Reload the app fresh. |
| **Action** | Tap the microphone/voice button. Grant permission when prompted. Speak one clear sentence in any supported language (Hindi, Malayalam, or English). |
| **Observable Success** | Caption/transcript appears below the orb showing what you spoke. Audio guidance begins playing within ~3 s of releasing the button. No crash, no blank screen. |
| **Failure Record** | Transcript absent or wrong language. App hangs with spinner > 5 s. Browser shows a permission error toast. Audio plays from wrong speaker. |
| **Evidence** | Screenshot of transcript caption and orb state immediately after release. Photo of screen if crash. |

### 1B — Permission Denied

| Field | Content |
|-------|---------|
| **Setup** | Reset browser microphone permission to Block/Deny for the app. Reload the app fresh. |
| **Action** | Tap the microphone/voice button. Observe what the browser does with the denied prompt (or whether it asks at all). |
| **Observable Success** | A visible message or icon change indicating `'micPermissionDenied'` or equivalent — NOT a generic spinner that never resolves. No crash. |
| **Failure Record** | App shows a generic spinner that never stops. No feedback to the user that permission was denied. Spinner resolves as if recording succeeded with empty audio. |
| **Evidence** | Screenshot of the UI state after tapping mic with denied permission. |

### 1C — Device Not Found (No Microphone)

| Field | Content |
|-------|---------|
| **Setup** | Use a device with no microphone, or revoke microphone access completely (Settings → Safari → Microphone → Block All). |
| **Action** | Tap the microphone/voice button. |
| **Observable Success** | Feedback indicates device not found (`'micNotFound'`), distinct from permission-denied state. No crash. |
| **Failure Record** | App attempts to record silently and produces empty audio. Same spinner behavior as the granted case. |
| **Evidence** | Screenshot of feedback state. |

---

## Section 2 — Audible Playback and Replay

### 2A — Guidance Audio Plays After First Receipt

| Field | Content |
|-------|---------|
| **Setup** | Complete a voice exchange that returns audio guidance (e.g., after a "Start Route" proposal). Audio guidance should be enabled and unmuted. |
| **Action** | Wait for the guidance audio to begin playing automatically (autoplay or after user gesture). Observe the listen button state. |
| **Observable Success** | Audio plays audibly through the device speaker. A "listen" or replay button becomes available after audio completes. The UI does not freeze during playback. |
| **Failure Record** | No audio heard. Audio plays but UI spinner stays active. Listen button never appears after audio ends. |
| **Evidence** | Screenshot of listen button visible after guidance ends. Photo of waveform/player UI if visible. |

### 2B — Replay Refused After Language Switch

| Field | Content |
|-------|---------|
| **Setup** | Receive audio guidance in Language A (e.g., English). The listen button for that guidance is visible. |
| **Action** | Switch the UI language (e.g., to Hindi or Malayalam). While still on the same screen, tap the listen/replay button for the old Language A guidance. |
| **Observable Success** | Replay is refused — either the listen button disappears, or tapping it produces a message that replay is unavailable after language change, or the UI fetches new guidance in the new language instead. |
| **Failure Record** | Old-language audio plays back after switching to a different UI language. Audio and UI language are now out of sync. |
| **Evidence** | Screenshot of refusal message, or screenshot showing listen button disappeared after language switch. |

### 2C — Replay Available Within Same Language Session

| Field | Content |
|-------|---------|
| **Setup** | Receive audio guidance in Language A. Do not switch language. |
| **Action** | Tap the listen/replay button for the same guidance without changing language. |
| **Observable Success** | Audio replays correctly from the beginning with no error, no fetch, and no prompt. |
| **Failure Record** | Replay fails with an error. Replay triggers a new backend request instead of playing cached/approved audio. |
| **Evidence** | Screenshot of successful replay. |

---

## Section 3 — Intelligibility Review (Separate from Language Switching)

> This section is for speech quality assessment, not UI behavior. Do not mix it with Section 2 language-switching tests. Each row covers one language independently.

### 3A — Hindi (hi-IN) Speech intelligibility

| Field | Content |
|-------|---------|
| **Setup** | Set UI language to Hindi. Speak a Hindi phrase. |
| **Action** | Listen to the returned audio guidance in Hindi. Compare the spoken output to the text shown on screen or in the transcript. |
| **Observable Success** | Audio guidance is clearly spoken, matches the meaning of the proposal, and is comprehensible to a Hindi speaker. No obviously wrong words or hallucinations. |
| **Failure Record** | Guidance says something unrelated to the proposal. Guidance is silent. Guidance is in a different language than Hindi. |
| **Evidence** | Written note of transcript vs. audio correspondence. Flag specific failures. |

### 3B — Malayalam (ml-IN) Speech intelligibility

| Field | Content |
|-------|---------|
| **Setup** | Set UI language to Malayalam. Speak a Malayalam phrase. |
| **Action** | Listen to the returned audio guidance in Malayalam. Compare to transcript or on-screen text. |
| **Observable Success** | Audio guidance is clearly spoken and comprehensible to a Malayalam speaker. Matches the proposal content. |
| **Failure Record** | Guidance is garbled, in wrong language, or says something unrelated. |
| **Evidence** | Written note of failures. |

### 3C — English (en-IN) Speech intelligibility

| Field | Content |
|-------|---------|
| **Setup** | Set UI language to English. Speak an English phrase. |
| **Action** | Listen to the returned audio guidance in English. |
| **Observable Success** | Guidance is clear and matches the proposal. |
| **Failure Record** | Same as 3A/3B. |
| **Evidence** | Written note of failures. |

---

## Section 4 — Actual Safari / Mobile Viewport

> These tests require a real iPhone or iPad. Playwright viewport emulation is NOT sufficient — only a real device confirms CSS layout at these sizes.

### 4A — iPhone Portrait (375 × 667 or similar)

| Field | Content |
|-------|---------|
| **Setup** | Open the app on an iPhone in portrait orientation. Use Safari Web Inspector or visually inspect. |
| **Action** | Load the main guidance screen. Observe the guidance panel, topbar, language switcher, and destination card at this width. |
| **Observable Success** | No horizontal overflow. Text wraps within the viewport. Buttons are large enough to tap with a finger. Language labels (Hindi "हिन्दी", Malayalam "മലയാളം") fit within the language switcher without clipping. No content is hidden behind the keyboard. |
| **Failure Record** | Horizontal scrollbar appears. Malayalam or Hindi label is cut off. Text overflows outside the guidance panel. Touch targets are < 44 px. |
| **Evidence** | Screenshot of the guidance screen at this width. Mark any overflow with a red border annotation. |

### 4B — iPhone Landscape

| Field | Content |
|-------|---------|
| **Setup** | Rotate the same iPhone to landscape. |
| **Action** | Observe the guidance panel and map side by side (if both visible). |
| **Observable Success** | Layout adjusts gracefully. Map remains visible. Guidance panel does not overlap map controls. No clipping of buttons or labels. |
| **Failure Record** | Guidance panel overlaps the map. Some controls become inaccessible in landscape. |
| **Evidence** | Screenshot in landscape. |

### 4C — iPad (if available)

| Field | Content |
|-------|---------|
| **Setup** | Open the app on an iPad. |
| **Action** | Observe layout at 768–1024 px width. |
| **Observable Success** | Layout uses additional width for guidance panel or map side panel. No regressions from iPhone layout. |
| **Failure Record** | Layout looks identical to iPhone (no use of extra width). Unexpected whitespace or truncation. |
| **Evidence** | Screenshot on iPad. |

---

## Section 5 — Keyboard Interaction

> Run on a device with an attached keyboard (iPad with paired keyboard, Android with OTG keyboard, or desktop browser).

### 5A — Tab Order Reaches All Interactive Elements

| Field | Content |
|-------|---------|
| **Setup** | Load the app. Attach a physical keyboard. Press Tab from the browser address bar into the page. |
| **Action** | Press Tab repeatedly. Count the focusable elements. Observe where focus lands on the guidance panel, map toolbar, voice suggestions, and modal dialogs. |
| **Observable Success** | Focus moves in a logical top-to-bottom, left-to-right order. All buttons, links, and inputs are reachable. Focus does not skip over interactive elements or jump to hidden ones. Modals receive focus when opened. |
| **Failure Record** | Focus skips some visible interactive element. Focus lands on a hidden or disabled element. Focus is invisible (no focus ring). Tab from modal cannot return to page content. |
| **Evidence** | Numbered list of focus order: `[1] .topbar button, [2] .language-switcher, [3] .voice-launch, ...`. Note any skips or inversions. |

### 5B — Enter Activates Focused Button

| Field | Content |
|-------|---------|
| **Setup** | Tab to the microphone/voice launch button. |
| **Action** | With focus on the voice button, press Enter or Space. |
| **Observable Success** | The voice action activates — microphone recording starts, or the voice console opens. Same as tapping the button. |
| **Failure Record** | Enter/Space does nothing. Focus visually moves but Enter does not activate. |
| **Evidence** | Note whether Enter activates the button or not. |

### 5C — Escape Closes Modal Dialogs

| Field | Content |
|-------|---------|
| **Setup** | Open a modal dialog (e.g., source details). |
| **Action** | Press Escape. |
| **Observable Success** | Modal closes. Focus returns to the element that opened it, or to the last active element. |
| **Failure Record** | Escape does not close the modal. Modal stays open with no keyboard path to close it. |
| **Evidence** | Note whether Escape closes the modal. |

---

## Section 6 — True Browser Zoom

> These must be verified at the actual browser zoom level. Playwright viewport API does NOT change browser zoom — it only resizes the viewport window. Use browser zoom settings (Cmd+/Cmd- on Mac, Ctrl+/Ctrl- on Windows/Android).

### 6A — UI at 100% Zoom (Baseline)

| Field | Content |
|-------|---------|
| **Setup** | Reset browser zoom to 100%. Load the guidance screen. |
| **Action** | Observe the guidance panel heading, instructions text, and emergency buttons. Note any overflow or clipping. |
| **Observable Success** | All content fits within the viewport. Text is legible at default zoom. |
| **Evidence** | Screenshot at 100% zoom. |

### 6B — UI at 200% Zoom

| Field | Content |
|-------|---------|
| **Setup** | Set browser zoom to 200% (Cmd+0 resets; Cmd++ twice = 200%). Do not resize the window — only the browser zoom setting changes. Load the same guidance screen. |
| **Action** | Observe the guidance panel heading, instructions, language switcher buttons, and emergency action buttons. |
| **Observable Success** | Hindi "हिन्दी" label remains fully visible within the language switcher button. Malayalam "മലയാളം" label does not overflow or clip. Emergency action buttons (e.g., "Evacuate Now") remain visually legible and their tap target is ≥ 44 px on screen. Guidance h1 text is readable without horizontal scrolling. |
| **Failure Record** | Hindi or Malayalam label clips or overflows the language-switcher button at 200% zoom. Emergency button text overlaps adjacent content. Content requires horizontal scroll to read. |
| **Evidence** | Screenshot at 200% zoom with annotations marking any overflow. |

### 6C — UI at 50% Zoom (Wide Viewport)

| Field | Content |
|-------|---------|
| **Setup** | Set browser zoom to 50%. |
| **Action** | Observe the layout. At very low zoom, content may reflow unexpectedly. |
| **Observable Success** | Layout remains usable. Guidance panel and map do not overlap unexpectedly. |
| **Failure Record** | Layout collapses or overlaps at low zoom. |
| **Evidence** | Screenshot at 50% zoom. |

---

## Summary Table

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

---

## Execution Status

**NOT_RUN** — requires human operator with real hardware. No automated script can replace this checklist.

**Time estimate:** ~20–30 minutes for a single operator covering all rows.

**Blockers:**
- Safari remote automation requires enabling "Allow remote automation" in Safari Developer Settings (`prototype-browser-verification.md`). This is a device setting, not an app change.
- Some rows require specific hardware (mic present/absent, physical keyboard).
- Zoom tests (Section 6) require browser zoom setting access, not just viewport resize.

---

*Worker 27 — Human Acceptance Checklist — d95df9e — NOT_RUN*
