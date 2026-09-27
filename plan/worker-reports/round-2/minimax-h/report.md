# MiniMax H — Browser Audio and Microphone Lifecycle
**Worker:** minimax-h | **Assignment:** round-2
**Timestamp:** 2026-09-27T12:00:00Z | **HEAD:** d95df9e
**Output dir:** plan/worker-reports/round-2/minimax-h/

---

## 1. Scope Inspected

| File | Lines | SHA-256 (base) | Dirty |
|------|-------|----------------|-------|
| `frontend/v2/src/audioGuidance.ts` | 482 | (clean @ d95df9e) | no |
| `frontend/v2/src/audioGuidance.test.ts` | 489 | (clean @ d95df9e) | no |
| `frontend/v2/src/main.ts` | 2197 | (clean @ d95df9e) | no |
| `frontend/v2/src/journey.ts` | 609 | (clean @ d95df9e) | no |

Dirty untracked files in repo (not in scope): `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/ttsworker/audio_mime_test.go`, `backend/internal/middleworker/eval/`, `frontend/v2/src/styles.css`, `backend/internal/httpserver/citizen_ownership_regression_test.go`.

---

## 2. Existing Coverage vs. New Contribution

### Existing Complete Coverage
- `buildAudioMetadata` — full field validation (content_type prefix, byte ceiling 768 KiB, hex checksum, positive integers, settings sub-object)
- `verifyAudioIntegrity` — cryptographic verification (base64 decode, byte count match, SHA-256 via Web Crypto API, fail-closed on unavailable crypto)
- `isAudioValidForReplay` — freshness=current, language match, data_version match, positive provenance fields
- `AudioPlaybackGuard` — generation counter; `canPlayPending` cross-checks all 5 conditions; `invalidate()` clears pending
- `processVoiceEnvelope` — pipeline state machine, silent camera actions, CLARIFY/ERROR classification, audio metadata pass-through
- `shouldDropRecordedAudio` — boolean OR of cancelled + hidden; tested for all 4 combinations
- `shouldDropResponse` — superseding by request ID + hidden document drop (journey.ts:364)
- `evaluateReadinessState` — fail-closed backend health evaluation
- Visibility-change → `cancelRecording()` + `supersedeInFlight()` at main.ts:2172
- `sendVoiceOrText` flow with `shouldDropResponse` at lines 1294, 1312
- Offline handler invalidates guard and clears `lastApprovedAudio` at line 2189
- All invalidation sites for `audioGuard` and `lastApprovedAudio` (language change, data version change, revokeRoute, supersedeInFlight, offline)

### Synthetic Test Gaps Found (verified by reading tests)

| Gap | Synthetic Coverage | Real-Device Checklist Item |
|-----|-------------------|---------------------------|
| `getUserMedia` permission denial细分 | ❌ none | REQUIRED |
| `getUserMedia` device-not-found细分 | ❌ none | REQUIRED |
| `getUserMedia` hardware-error细分 | ❌ none | REQUIRED |
| `MediaRecorder.onerror` not wired | ❌ none | REQUIRED |
| MIME fallback not validated | ❌ none | REQUIRED |
| `mediaRecorder.ondataavailable` empty-chunk guard | ❌ none | REQUIRED |
| Blob URL lifetime (data URI — no URL.createObjectURL used) | ✅ N/A — data URI | N/A |
| Autoplay `NotAllowedError` → user gesture replay flow | partial | REQUIRED |
| `lastApprovedAudio` cleared on visibility hide | ❌ not cleared | REQUIRED |
| `AudioPlaybackGuard.consumePending()` caller | partial | REQUIRED |
| Navigation/pagehide cleanup of playback Audio element | ❌ none | REQUIRED |

---

## 3. Findings with Evidence

### Finding 1: `MediaRecorder.onerror` is not assigned
**File:** `main.ts:1101`
**Evidence:**
```typescript
mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
mediaRecorder.ondataavailable = (event) => { ... };
mediaRecorder.onstop = async () => { ... };
// mediaRecorder.onerror is never set
```
**Impact:** If MediaRecorder encounters a hardware故障, encoding error, or stream track loss during recording, the error is silently ignored. The recording appears to succeed (timeout fires, `onstop` fires) but the Blob may be empty or truncated.
**Proposed correction:** Add `mediaRecorder.onerror = (event) => { ... }` that calls `cancelRecording()` and sets `voiceFeedbackKey = 'micStopped'`. See checklist item 4.
**Uncertainties:** Exact Safari/iOS Safari MediaRecorder error taxonomy; whether `event` is `MediaRecorderErrorEvent` with `name` property.

### Finding 2: Fallback MIME type is not validated
**File:** `main.ts:1099`
**Evidence:**
```typescript
const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
  ? 'audio/webm;codecs=opus'
  : 'audio/webm';
```
If the device supports neither type, `MediaRecorder` is constructed with an unsupported MIME. This does not throw — subsequent `start()` will throw `InvalidStateError` or the recorder will produce an empty/unplayable Blob.
**Proposed correction:** After the fallback, validate:
```typescript
if (!MediaRecorder.isTypeSupported(mimeType)) {
  // Fall through to catch block with specific error
  throw new Error('No supported audio MIME type');
}
```
**Uncertainties:** Whether `MediaRecorder.isTypeSupported` is reliable cross-browser for all MIME strings; whether Safari supports `audio/webm` at all.

### Finding 3: Permission error types not differentiated
**File:** `main.ts:1127-1130`
**Evidence:**
```typescript
} catch {
  voiceListening = false;
  voiceFeedbackKey = 'micStopped';
  render();
}
```
`getUserMedia` can throw `NotAllowedError` (denied), `NotFoundError` (no mic), `NotReadableError` (hardware busy), `OverconstrainedError` (settings impossible). All collapse to the same `'micStopped'` feedback. A user who denied permission needs different UX than a user whose device is missing.
**Proposed correction:** Catch the error, check `err.name`, set `voiceFeedbackKey` to a more specific key (e.g., `'micPermissionDenied'`, `'micNotFound'`) and surface a targeted message.
**Uncertainties:** Whether UX should differentiate; whether `voiceFeedbackKey` values for these states exist in `words` object.

### Finding 4: `cancelRecording()` nullifies `mediaRecorder` synchronously before `onstop` fires
**File:** `main.ts:1076-1082`
**Evidence:**
```typescript
if (mediaRecorder) {
  try {
    if (mediaRecorder.state === 'recording') mediaRecorder.stop();
  } catch {}
  mediaRecorder = null; // ← synchronous, but onstop fires async
}
voiceListening = false;
```
`mediaRecorder.stop()` returns immediately; `onstop` fires in a later microtask. If `cancelRecording` is called a second time (e.g., `visibilitychange` fires while `toggleLocalRecording` stop path is in flight), `mediaRecorder` is already `null` and the second `cancelRecording` call's `mediaRecorder.state` check is a no-op — but that's benign. The real risk is if something reads `mediaRecorder` in the window between `stop()` and `onstop`.
**Severity:** Low — no evidence of actual corruption. The `catch {}` swallows any `InvalidStateError` from double-stop. The `onstop` guard at line 1106 correctly checks `shouldDropRecordedAudio` using `recordingCancelled` (captured closure variable), not `mediaRecorder.state`.
**Note for E:** E's outage patch to `main.ts` must not assume `mediaRecorder` is non-null inside the `onstop` callback after `cancelRecording()`.

### Finding 5: `lastApprovedAudio` is NOT cleared on visibility hide
**File:** `main.ts:2172-2179`
**Evidence:**
```typescript
document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    cancelRecording();
    stopTracking();
    supersedeInFlight();
  }
  render();
});
```
`supersedeInFlight()` clears `lastApprovedAudio = undefined` (line 249). So visibility hide DOES clear it. However, the `Audio` element itself (created at line 747) is not paused or cleaned up — it continues playing if already started. This is a gap for pagehide/navigation.
**Proposed correction:** On `visibilitychange` hidden, if an `Audio` element from `verifyAndPlayAudio` is playing, call `.pause()` on it. See checklist item 10.

### Finding 6: Audio element created as data URI has no cleanup concern
**File:** `main.ts:746-747`
**Evidence:**
```typescript
const audio = new Audio(`data:${mime};base64,${audioInfo.audio_b64}`);
```
This uses a data URI, not `URL.createObjectURL()`. There is no Blob URL registry to revoke. The `Audio` element goes out of scope when `verifyAndPlayAudio` resolves (unless `setPending` stored it). The guard's `pending.audio` reference is cleared by `consumePending()` or `invalidate()`. No URL revocation needed.
**Status:** No gap.

---

## 4. Candidate Patch

No patch generated. All findings are in `main.ts` which E owns for an active outage patch. Patches would interfere with E's work. All corrections are documented as source-supported recommendations for E or for Opus's integration review.

If Opus requests a patch after E's work lands, the minimal patch would address:
1. `MediaRecorder.onerror` assignment (Finding 1)
2. MIME type validation after fallback (Finding 2)

---

## 5. Verification: Execution Tests NOT_RUN (deferred by user)

### Synthetic tests: NOT_RUN
No `node --test` or `vitest` executed per assignment constraints.

### Later commands (requires setup)
```bash
# Run audioGuidance unit tests (synthetic, no browser)
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run test -- src/audioGuidance.test.ts

# Run all frontend tests
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run test

# Type-check
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npx tsc --noEmit
```

### Real-device checklist (observable outcomes on device/simulator)

For each item, load the app in a real iOS Safari or Android Chrome browser.

| # | Checklist item | Action | Expected outcome |
|---|---------------|--------|-----------------|
| 1 | Microphone permission denied | Tap mic, deny browser permission prompt | Toast/key `'micPermissionDenied'` shown; no crash |
| 2 | Microphone device not found | Physically disconnect/cover mic, tap mic | Toast/key `'micNotFound'` shown; no crash |
| 3 | MediaRecorder error mid-recording | Start recording, suspend app (background it), resume | Recording cancelled cleanly; no orphan MediaRecorder |
| 4 | Unsupported MIME fallback | Force an unsupported MIME by patching `isTypeSupported` | Falls back to `'audio/webm'` or shows error, not silent hang |
| 5 | Autoplay blocked, tap to replay | Deny autoplay, receive audio guidance, tap listen button | Audio plays; `AudioPlaybackGuard.consumePending()` called |
| 6 | Language change during pending autoplay | While autoplay is pending (blocked), change language | Pending audio is dropped; new language audio plays or not |
| 7 | `lastApprovedAudio` cleared on visibility | Receive guidance audio, background app (home button) | Audio invalidated on return to foreground; re-fetch required |
| 8 | Pagehide while audio playing | Start audio playback, press home/switch away | Audio paused by pagehide or continues correctly |
| 9 | Replay stale audio via modal | Receive guidance, navigate away, return, tap listen on stale audio | Shows `'approvedAudioUnavailable'` or fetches fresh audio |
| 10 | 20s recording timeout | Hold recording for 20+ seconds | Recording stops, audio sent to backend, no MediaRecorder leak |

**Expected negative control for new regression logic:**
- Patch Finding 1 (onerror): Start recording, unplug mic mid-record → should trigger error handler, call `cancelRecording()`, NOT produce a spurious audio Blob.

---

## 6. Status

**STATUS:** READY_FOR_REVIEW_UNVERIFIED

### Integration dependencies
- E's outage patch to `main.ts` must be reviewed for conflicts with Findings 1–3 (MediaRecorder lifecycle)
- Any patch to `main.ts` recording section must preserve the `shouldDropRecordedAudio(recordingCancelled, document.hidden)` guard in `onstop`
- `AudioPlaybackGuard` and `audioGuidance.ts` are clean; no integration risk

### Exact next action for Opus
1. Read E's patch to `main.ts` and confirm it does not conflict with Findings 1–3
2. Apply or delegate the two-line MIME validation fix (Finding 2) and the `onerror` handler (Finding 1) to `main.ts:1101`
3. After E's patch lands, run real-device checklist items 1–10 and confirm all pass
4. If E's patch changes recording lifecycle, re-run synthetic tests in `audioGuidance.test.ts`

---

*Report generated by minimax-h. No execution tests performed. All findings are source-inspection-only.*
