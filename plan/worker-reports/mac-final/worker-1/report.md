# Worker 1 — Microphone Recording Lifecycle Fixes
**Report:** `plan/worker-reports/mac-final/worker-1/report.md`
**Worker:** worker-1 (final round)
**Timestamp:** 2026-09-27
**Status:** READY_FOR_INTEGRATION
**HEAD:** `e8836922d90a153e2d3e571efde2381b28aa1a43`

---

## 1. Base HEAD and File Hashes

| File | SHA-256 (base) |
|------|----------------|
| `frontend/v2/src/main.ts` | `27603c3c0b20cfdabe35c293c1ace937ec7c2ebcd7f36fbabb2414dbfeec27a0` |
| `frontend/v2/src/audioGuidance.ts` | `fb664b91634f6a520979a851a27d7d3f71b834fe089dd85a9b9f975fd4b1a844` |
| `frontend/v2/src/audioGuidance.test.ts` | `bd2c740664f916afdd861e5cc244ba825ab0b1703960ec1a43f8af209a9fb1dd` |

**Integration branch:** `CLEAN` — no staged changes to scoped files at HEAD.

Dirty/untracked files (pre-existing, not modified):
- `frontend/v2/src/styles.css` — unrelated
- `backend/internal/asrworker/audio_mime_test.go` — backend MIME tests, out of scope
- `backend/internal/httpserver/citizen_ownership_regression_test.go` — untracked

---

## 2. Root Cause Analysis

All gaps stem from incomplete `MediaRecorder` lifecycle management in `toggleLocalRecording` (main.ts:1085–1132) and missing cleanup on language change.

### Gap 1: MediaRecorder.onerror unhandled
`mediaRecorder.onerror` was never assigned. If the OS/hardware fires an error event during recording, `onerror` is `null` → error is silently ignored. The 20s timeout fires and calls `stop()`, which triggers `onstop`. Since `recordingCancelled=false`, `shouldDropRecordedAudio` returns `false` and a potentially empty/truncated Blob is sent to the backend.

**Fix:** Assign `mediaRecorder.onerror` that calls `cancelRecording()`, sets `voiceFeedbackKey='micStopped'`, and renders.

### Gap 2: Constructor/start failure cleanup
`new MediaRecorder(stream, { mimeType })` does not throw even when the MIME type is unsupported. The error surfaces at `start()` as `NotSupportedError`. The original `start()` call was not wrapped in try/catch. If it threw, `voiceListening` was already set to `true` but `recordingTimer` was not started, leaving the UI in an inconsistent state.

**Fix:** Wrap `mediaRecorder.start()` in try/catch. On error, call `cancelRecording()` and set `voiceFeedbackKey='micStopped'`.

### Gap 3: Unsupported recording MIME types not checked against backend
The original MIME selection:
```js
const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
  ? 'audio/webm;codecs=opus'
  : 'audio/webm';
```
Neither check validates against the backend allow-list. If a browser supports only `audio/ogg;codecs=opus`, the code would select `audio/webm` which the browser might not support (but might still return an empty Blob).

**Fix:** Add `pickSupportedRecorderMimeType(isTypeSupported)` helper in `audioGuidance.ts` that iterates the backend allow-list in priority order and returns the first browser-supported type, or `null` if none. Call it before `getUserMedia`; if `null`, bail out with `micStopped`.

### Gap 4: getUserMedia resolving after cancellation
When `toggleLocalRecording` awaits `getUserMedia`, the user can cancel via `voice-close` button or page hide. After the `await` resolves, the code proceeds to construct the `MediaRecorder` and call `start()`. The `recordingCancelled` flag is already `true` from `cancelRecording()` but the post-await code ignores it.

**Fix:** After the `await` resolves, check `recordingCancelled || document.hidden`. If true, stop the stream tracks and return early without creating the recorder.

### Gap 5: Language change during recording
The language change handler (main.ts:1856) calls `supersedeInFlight()` but NOT `cancelRecording()`. An in-progress recording continues after a language switch. When `onstop` fires, `shouldDropRecordedAudio(recordingCancelled=false, document.hidden=false)` returns `false`, so audio is sent with the old language context tag still in scope.

**Fix:** Add `cancelRecording()` before `supersedeInFlight()` in the language change handler.

### Gap 6: Cancelled/obsolete onstop callbacks must not submit audio
**Already correct.** `onstop` opens with `if (shouldDropRecordedAudio(recordingCancelled, document.hidden)) { return; }`. `cancelRecording()` sets `recordingCancelled=true` synchronously before calling `stop()`, so by the time `onstop` fires, the guard correctly drops the audio.

### Gap 7: Stream tracks and timers cleaned up without breaking normal recording
- `cancelRecording()` clears the timer, stops all tracks, nulls `mediaStream` and `mediaRecorder`.
- Normal `onstop` path stops tracks, nulls references, sends audio.
- Cancelled `onstop` path: early return — tracks already stopped by `cancelRecording()`.
- No double-stop: `cancelRecording()` nulls `mediaRecorder` before calling `stop()`, so a second `cancelRecording()` is a no-op on the recorder.

---

## 3. Exact Changes

### `frontend/v2/src/audioGuidance.ts` (+27 lines)
Added `pickSupportedRecorderMimeType(isTypeSupported)` function with `BACKEND_SUPPORTED_MIME_TYPES` constant that mirrors `contracts.SupportedTranscriptionContentTypes`.

### `frontend/v2/src/main.ts` (+32 lines net)

**Import (line 55):** Added `pickSupportedRecorderMimeType` to the `audioGuidance` import.

**`toggleLocalRecording` rewrite:**
1. Call `pickSupportedRecorderMimeType` to select MIME type. If `null`, bail with `micStopped`.
2. After `await getUserMedia`: check `recordingCancelled || document.hidden` and bail early (stop tracks first).
3. Assign `mediaRecorder.onerror`: calls `cancelRecording()`, sets `voiceFeedbackKey='micStopped'`, renders.
4. `mediaRecorder.start()` wrapped in try/catch. On error: `cancelRecording()`, `micStopped`, return.
5. `catch` block cleans up `streamRef` tracks.
6. `mediaStream = stream` assigned after the cancellation check.

**Language change handler (line 1893):** Added `cancelRecording()` before `supersedeInFlight()`.

---

## 4. Commands, Exit Statuses and Observed Results

### Regression tests (new function)
```
cd /tmp/opencode/worker1-scratch
node --experimental-strip-types --test recording_lifecycle.test.ts
```
Result: **6 pass, 0 fail**

### Existing test suite (89 tests)
```
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
node --experimental-strip-types --test \
  src/journey.test.ts src/operator.test.ts src/i18n.test.ts \
  src/mapActions.test.ts src/emergency.test.ts src/audioGuidance.test.ts
```
Result: **89 pass, 0 fail**

### Patch verification
```
cd /Users/apple/Documents/Projects/MonitoringZ
patch -p0 < plan/worker-reports/mac-final/worker-1/candidate.patch
```
Result: **"patching file 'frontend/v2/src/audioGuidance.ts'"** and **"patching file 'frontend/v2/src/main.ts'"** — both clean.

---

## 5. What Remains Unverified

| Item | Reason |
|------|--------|
| Actual MediaRecorder error event fires and `onerror` fires correctly | Requires browser automation with fake `MediaRecorder` that fires error events. Python Playwright harness exists from worker-5 (round-3); not re-run in this isolated scratch. |
| `audio/webm;codecs=opus` actually works on Chrome/Firefox against real backend | Requires live stack with ASR worker; no mock ASR in browser context. |
| `cancelRecording()` race between `onstop` async and synchronous cancel path | Inspected statically; would need a deterministic async test harness to reproduce reliably. |
| End-to-end recording on Safari with `audio/webm` fallback | Safari-specific; not available in automation. |

These require live-stack browser tests or Playwright with injected fake recorders (worker-5's `test_mediarecorder_error.py` covers this).

---

## 6. Patch Dependencies and Integration

**Base hashes (confirmed at HEAD e883692):**
```
frontend/v2/src/main.ts         27603c3c0b20cfdabe35c293c1ace937ec7c2ebcd7f36fbabb2414dbfeec27a0
frontend/v2/src/audioGuidance.ts fb664b91634f6a520979a851a27d7d3f71b834fe089dd85a9b9f975fd4b1a844
```

**Dependencies:** None — does not conflict with existing dirty state (styles.css, asrworker MIME tests).

**Integration:**
```bash
cd /Users/apple/Documents/Projects/MonitoringZ
patch -p0 < plan/worker-reports/mac-final/worker-1/candidate.patch

# Type-check after patch
cd frontend/v2 && npx tsc --noEmit

# Run existing tests
node --experimental-strip-types --test \
  src/journey.test.ts src/operator.test.ts src/i18n.test.ts \
  src/mapActions.test.ts src/emergency.test.ts src/audioGuidance.test.ts

# Run new regression
cd /tmp/opencode/worker1-scratch
node --experimental-strip-types --test recording_lifecycle.test.ts
```

**Patch files delivered:**
- `plan/worker-reports/mac-final/worker-1/candidate.patch` — unified patch (repo-relative paths)
- `plan/worker-reports/mac-final/worker-1/recording_lifecycle.test.ts` — regression test (6 cases, all pass)

---

## 7. Artifacts

| Artifact | Path |
|----------|------|
| Unified patch | `plan/worker-reports/mac-final/worker-1/candidate.patch` |
| Regression test | `plan/worker-reports/mac-final/worker-1/recording_lifecycle.test.ts` |
| This report | `plan/worker-reports/mac-final/worker-1/report.md` |
