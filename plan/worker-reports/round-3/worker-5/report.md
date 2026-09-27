# Worker 5 Report — Browser Microphone Recorder Error Handling

**Worker:** worker-5 | **Assignment:** round-3
**Timestamp:** 2026-09-27 | **HEAD:** d95df9e (working tree dirty)
**Output dir:** plan/worker-reports/round-3/worker-5/

---

## 1. HEAD, Timestamp, Source Hashes, Dirty Files

- **HEAD:** `d95df9e609454877a91b7c82146b3c6368181b17`
- **main.ts SHA-256 (current working tree):** `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09`
- **Dirty source files in scope:**
  - `frontend/v2/src/main.ts` — contains 503/504 fix (minimax-e patch applied, not committed), MediaRecorder.onerror missing (this work)
  - `frontend/v2/src/styles.css` — unrelated dirty work
- **Dirty untracked files (not in scope):** `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/ttsworker/audio_mime_test.go`, `backend/internal/middleworker/eval/`, `backend/internal/httpserver/citizen_ownership_regression_test.go`

---

## 2. Prior Report (minimax-h) Assessment

minimax-h report (round-2/minimax-h/report.md) identified `MediaRecorder.onerror is not assigned` as **Finding 1** with severity REQUIRED. The report correctly located the gap at `main.ts:1101` (now line ~1101) and proposed assigning an error handler that calls `cancelRecording()`.

**Verification against current code:** Confirmed. `main.ts:1101` constructs a `MediaRecorder`, sets `ondataavailable` and `onstop` but **never sets `onerror`**. The working tree has advanced since minimax-h's inspection (SHA changed from `ed9dbcd5...` to `00dae4e6...`) due to the 503/504 fix, but the onerror gap remains identical — the 503/504 patch is in a different section (lines 1298-1303) and does not interact with the MediaRecorder lifecycle.

---

## 3. Scope Inspected

| File | Lines | Finding |
|------|-------|---------|
| `frontend/v2/src/main.ts` | 1066–1132 (cancelRecording, toggleLocalRecording) | onerror missing |
| `frontend/v2/src/i18n.ts` | full | micStopped key confirmed |
| `frontend/v2/src/audioGuidance.ts` | N/A | not in scope |
| `frontend/v2/package.json` | test runner section | Node.js strip-types test runner |

---

## 4. Existing Coverage vs. New Contribution

### Existing
- `cancelRecording()` cleans up timer, stream tracks, calls `stop()` in try/catch, sets `mediaRecorder=null`
- `onstop` handler checks `shouldDropRecordedAudio(recordingCancelled, document.hidden)` — drops audio if cancelled
- `voiceFeedbackKey = 'micStopped'` used in catch block for `getUserMedia` failure
- All closure state (`recordingCancelled`, `recordingTimer`, `mediaStream`, `mediaRecorder`) is module-scoped and correctly shared between `cancelRecording` and `toggleLocalRecording`

### New Contribution
- `mediaRecorder.onerror` assignment in `toggleLocalRecording` — calls `cancelRecording()`, sets `voiceFeedbackKey='micStopped'`, renders
- No new localized strings needed — reuses existing `'micStopped'` key
- No MIME selection, permission, or playback code changed
- No changes to `onstop` processing or `shouldDropRecordedAudio` guard

---

## 5. Findings with Evidence

### Finding 1: MediaRecorder.onerror is unhandled (confirmed)

**File:** `main.ts:1101`
**Evidence:**
```typescript
mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
mediaRecorder.ondataavailable = (event) => { ... }; // set
mediaRecorder.onstop = async () => { ... };          // set
// mediaRecorder.onerror is NEVER set                  ← BUG
```

**Impact:** If the OS/hardware fires a MediaRecorder error event (track loss, encoding failure, device disconnect) during recording:
- `onerror` is null → error is silently ignored
- The timeout fires at 20s and calls `stop()` which fires `onstop`
- `onstop` checks `shouldDropRecordedAudio(recordingCancelled, document.hidden)` — but `recordingCancelled` is `false` (normal timeout path)
- A potentially empty/truncated Blob may be sent to the backend as audio
- The user sees no error feedback — the UI appears to hang in `recording` state

**Proposed correction (this patch):**
```typescript
mediaRecorder.onerror = () => {
  cancelRecording();
  voiceFeedbackKey = 'micStopped';
  render();
};
```

**Idempotency / recursive stop protection:**
- `cancelRecording()` sets `mediaRecorder = null` synchronously before calling `stop()` in a try/catch
- If `stop()` triggers `onstop` asynchronously, `onstop` checks `shouldDropRecordedAudio(recordingCancelled=true)` → drops the audio
- If `onerror` fires again after `cancelRecording()` has already run (e.g., a late error event), `cancelRecording()` is called again with `mediaRecorder = null` — the `if (mediaRecorder)` check is a no-op, no double-stop occurs

### Finding 2: Confirm cancelRecording is correctly idempotent

**File:** `main.ts:1066–1083`
```typescript
function cancelRecording() {
  recordingCancelled = true;
  if (recordingTimer !== null) { clearTimeout(recordingTimer); recordingTimer = null; }
  if (mediaStream) { mediaStream.getTracks().forEach((track) => track.stop()); mediaStream = null; }
  if (mediaRecorder) {
    try { if (mediaRecorder.state === 'recording') mediaRecorder.stop(); } catch {}
    mediaRecorder = null;  // ← set null BEFORE calling stop(), which fires onstop async
  }
  voiceListening = false;
}
```

- `recordingCancelled = true` is set first, so when `onstop` fires it will correctly drop the audio via `shouldDropRecordedAudio`
- `mediaRecorder = null` before `stop()` means a second `cancelRecording()` call is a no-op on the recorder
- The timeout at line 1122 checks `mediaRecorder?.state === 'recording'` — if null, optional chaining returns `undefined !== 'recording'` so timeout is also a no-op

---

## 6. Candidate Patch

**File:** `plan/worker-reports/round-3/worker-5/candidate.patch`
**Base SHA-256:** `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09`
**Integration dependency:** None — does not conflict with the 503/504 fix (different lines, different function). However, both patches touch `main.ts` and must be applied together. The 503/504 fix is already in the working tree (SHA `00dae4e6...`).

**Patch diff:**
```diff
--- a/frontend/v2/src/main.ts
+++ b/frontend/v2/src/main.ts
@@ -1098,6 +1098,11 @@ async function toggleLocalRecording() {
     const chunks: Blob[] = [];
     mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
     mediaRecorder.ondataavailable = (event) => {
       if (event.data.size) chunks.push(event.data);
     };
+    mediaRecorder.onerror = () => {
+      cancelRecording();
+      voiceFeedbackKey = 'micStopped';
+      render();
+    };
     mediaRecorder.onstop = async () => {
       if (shouldDropRecordedAudio(recordingCancelled, document.hidden)) {
         return;
```

---

## 7. Regression Artifact

**File:** `plan/worker-reports/round-3/worker-5/test_mediarecorder_error.py`
**Framework:** Python Playwright (available at `/Users/apple/.agent-reach-venv/bin/playwright`)
**Status:** NOT_RUN — deferred by user

### What it tests:
1. App loads; fake `MediaRecorder` (fires error 100ms after `start()`) is injected via `page.evaluate`
2. User clicks mic button; recording starts; fake error fires
3. After error, `cancelRecording()` is called; `voiceFeedbackKey='micStopped'` set; UI updated
4. **Assertion:** `.voice-stage.is-listening` class is removed (proves `voiceListening=false` after error)
5. **Assertion:** No `POST /api/v3/voice/process` network request was made (proves audio was dropped by `shouldDropRecordedAudio`)
6. **Assertion:** Voice console remains open and functional after error (recovery)

### Expected failure BEFORE patch:
- `.is-listening` remains visible — `voiceFeedbackKey` never updated because `onerror` is null
- No error feedback shown

### Expected pass AFTER patch:
- `.is-listening` removed immediately after fake error fires
- Zero audio POST requests
- Console still functional

---

## 8. Verification Commands

**Source inspection:** Performed
**Execution tests:** NOT_RUN — deferred by user

```bash
# Start dev server (one terminal)
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run dev

# Run regression (second terminal)
cd /Users/apple/Documents/Projects/MonitoringZ
python plan/worker-reports/round-3/worker-5/test_mediarecorder_error.py

# Apply both patches together (Opus)
cd /Users/apple/Documents/Projects/MonitoringZ
patch -p1 < plan/worker-reports/round-3/worker-5/candidate.patch
# (The 503/504 fix from minimax-e is already in the working tree at SHA 00dae4e6)

# Type-check after patch
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npx tsc --noEmit
```

**Expected negative control:** Before patch, `is_listening` class persists for 20+ seconds (timeout duration) even after the fake recorder error fires.

---

## 9. Integration Risks

- **Conflicting changes:** Both `candidate.patch` (this work) and the minimax-e 503/504 patch touch `main.ts`. The 503/504 fix is already in the working tree at SHA `00dae4e6...`. This patch adds 5 lines to a different function (`toggleLocalRecording`, lines 1101-1127). No line-number conflict.
- **Dependencies:** None. Uses existing `cancelRecording`, `voiceFeedbackKey='micStopped'`, and `render()`.
- **State interaction:** The `onerror` → `cancelRecording()` path correctly sets `recordingCancelled=true` before calling `stop()`, ensuring `shouldDropRecordedAudio` returns `true` in `onstop` and drops the partial audio.

---

## 10. Status

**STATUS: READY_FOR_REVIEW_UNVERIFIED**

**Next action for Opus:**
1. Verify `frontend/v2/src/main.ts` SHA-256 is `00dae4e6...` (confirms 503/504 fix is present)
2. Apply `plan/worker-reports/round-3/worker-5/candidate.patch`
3. Run `python test_mediarecorder_error.py` against the running dev server
4. Verify TypeScript compiles cleanly after both patches
5. Run existing unit tests: `cd frontend/v2 && node --experimental-strip-types --test src/*.test.ts`
