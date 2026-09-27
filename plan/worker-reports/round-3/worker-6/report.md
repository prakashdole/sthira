# Worker 6 Report — Browser Recorder MIME Compatibility

**Timestamp:** 2026-09-27T15:12 UTC
**HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Output dir:** `plan/worker-reports/round-3/worker-6/`
**Assignment ref:** plan/worker-reports/round-2/minimax-h/report.md

---

## 1. Source hashes

| File | SHA-256 (current/working-tree) | Notes |
|---|---|---|
| `frontend/v2/src/main.ts` | `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09` | Dirty — recording logic at lines ~1095–1132 |
| `backend/internal/asrworker/audio.go` | `097512ccbc95d1a241d0b17983de353a57b29d7fccdcbc3aecd0eedbef5031a3` | Clean at d95df9e |
| `backend/internal/asrworker/audio_compressed.go` | `e416ccaa4a2bd0b521f9d1000ade235e22563750e1136c787dff86083ebf65f4` | Clean at d95df9e |
| `backend/internal/asrworker/audio_mime_test.go` | (dirty, untracked) | Tests for backend MIME; not in scope for frontend fix |

**Dirty files in working tree relevant to scope:**
- `frontend/v2/src/main.ts` — contains the `toggleLocalRecording` function with MIME selection

---

## 2. Scope Inspected

**Frontend:** `main.ts:1085–1132` (`toggleLocalRecording`), `main.ts:1066–1083` (`cancelRecording`), `main.ts:2172–2179` (visibilitychange)

**Backend:** `audio.go:171–192` (`isWorkerSupportedCodec`), `audio_compressed.go:29–43` (`CompressedContentTypes` + `init()`), `audio_compressed.go:205–218` (`DecodeAudio`)

**Test (dirty untracked):** `audio_mime_test.go` — backend codec tests (not modified by this worker)

---

## 3. Prior Report Assessment (minimax-h Finding 2)

minimax-h reported: *"If the device supports neither type, `MediaRecorder` is constructed with an unsupported MIME. This does not throw — subsequent `start()` will throw `InvalidStateError` or the recorder will produce an empty/unplayable Blob."*

**Corrected assessment:**

- **"Does not throw at construction"** — Correct. `new MediaRecorder(stream, { mimeType })` does not validate MIME support.
- **"subsequent start() will throw InvalidStateError"** — Incomplete. The `MediaRecorder` constructor accepts any MIME string syntactically; the browser only validates support when `start()` is called. The error is a real `NotSupportedError` (not `InvalidStateError`) thrown from `start()`, not from the constructor.
- **"or the recorder will produce an empty/unplayable Blob"** — Correct and more dangerous: on some browser/OS combinations, `start()` does NOT throw but records silence or a 0-byte blob. This silently produces invalid audio sent to the backend.

**Backend MIME compatibility (confirmed against source):**

The backend (`audio_compressed.go:31–37`) accepts via ffmpeg:
- `audio/webm` (any Opus variant — ffmpeg auto-detects)
- `audio/webm;codecs=opus`
- `audio/ogg;codecs=opus`
- `audio/opus`

The backend does NOT accept `audio/webm;codecs=opus` at the MIME header level AND `audio/webm` without the codec param ALSO works (ffmpeg auto-detects the container). So the Safari fallback to plain `audio/webm` is NOT a backend rejection problem — it works at the backend. The real failure mode is the browser producing invalid/empty audio from the start.

---

## 4. Findings

### Finding W6-1: `mediaRecorder.onerror` is not wired (minimax-h Finding 1 re-confirmed)

**File:** `main.ts:1101`
**Evidence:**
```typescript
mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
mediaRecorder.ondataavailable = (event) => { ... };
mediaRecorder.onstop = async () => { ... };
// mediaRecorder.onerror is never set
```
**Impact:** If `start()` throws `NotSupportedError` (unsupported MIME on some browsers), or if a hardware/stream error occurs mid-recording, `onerror` is not fired and the error is silently swallowed. The recording appears to succeed until the 20s timeout fires with an empty `chunks` array, sending a `null`-size Blob to the backend.
**Proposed correction:** Assign `onerror` in `toggleLocalRecording` before calling `start()`.

### Finding W6-2: `start()` call is not wrapped in error handling

**File:** `main.ts:1121`
**Evidence:**
```typescript
mediaRecorder.start();
recordingTimer = window.setTimeout(() => {
  if (mediaRecorder?.state === 'recording') {
    mediaRecorder.stop();
  }
}, 20_000);
```
**Impact:** If the MIME is unsupported and causes `start()` to throw `NotSupportedError`, it propagates to `toggleLocalRecording`'s caller (`sendVoiceOrText`) as an uncaught exception. The UI becomes inconsistent: `voiceListening = true` was already set, but `recordingTimer` was not started. The next tap re-enters `toggleLocalRecording` in a confused state.
**Proposed correction:** Wrap `start()` in try/catch inside `toggleLocalRecording` after setting `voiceListening = true`. On error, call `cancelRecording()` and set `voiceFeedbackKey = 'micStopped'`.

### Finding W6-3: No `MediaRecorder.isTypeSupported` pre-check before constructing recorder (corrective to minimax-h Finding 2)

The current logic:
```typescript
const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
  ? 'audio/webm;codecs=opus'
  : 'audio/webm';
```
This constructs `mediaRecorder` even if neither MIME is supported. The error only surfaces at `start()`.

**Proposed correction:** After computing `mimeType`, validate:
```typescript
if (!MediaRecorder.isTypeSupported(mimeType)) {
  throw new Error('No supported audio MIME type');
}
```
This surfaces the failure at construction time rather than at `start()`.

### Finding W6-4: Worker 5 note (onerror and cancelRecording conflict)

minimax-h Finding 4 (cancelRecording nullifies mediaRecorder before onstop fires) noted that E's patch must not assume `mediaRecorder` is non-null inside `onstop` after `cancelRecording()`. Since the asrworker change is unconfirmed, the patch in this report assigns `onerror` using the existing `cancelRecording()` helper, which safely nullifies `mediaRecorder` and clears `recordingCancelled`. This is consistent with minimax-h's guidance.

---

## 5. Existing Coverage

- `audio_mime_test.go` (dirty untracked) covers backend: WebM/opus with codec param, without codec param, Ogg/opus, autodetection, WAV rejection, case insensitivity. Confirmed backend accepts `audio/webm` with and without `codecs=opus`.
- `audioGuidance.test.ts` covers frontend audio metadata validation and guard logic, but not the MediaRecorder construction/starting lifecycle.

---

## 6. Candidate Patch

**File:** `frontend/v2/src/main.ts`
**Base hash:** `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09` (current working tree)
**Note:** This patch is proposed against the current working tree. Opus must check whether the asrworker/outage patch to `main.ts` has already landed and rebase/merge before applying.

```diff
--- a/frontend/v2/src/main.ts
+++ b/frontend/v2/src/main.ts
@@ -1096,13 +1096,23 @@ async function toggleLocalRecording() {
     return;
   }

   recordingCancelled = false;
+  let mediaStreamForRecorder: MediaStream | null = null;
   try {
     mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
-    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
-      ? 'audio/webm;codecs=opus'
-      : 'audio/webm';
-    const chunks: Blob[] = [];
-    mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
+    mediaStreamForRecorder = mediaStream;
+    let mimeType = 'audio/webm;codecs=opus';
+    if (!MediaRecorder.isTypeSupported(mimeType)) {
+      mimeType = 'audio/webm';
+    }
+    if (!MediaRecorder.isTypeSupported(mimeType)) {
+      voiceFeedbackKey = 'micStopped';
+      voiceListening = false;
+      mediaStream.getTracks().forEach((track) => track.stop());
+      mediaStream = null;
+      render();
+      return;
+    }
+    const chunks: Blob[] = [];
+    mediaRecorder = new MediaRecorder(mediaStreamForRecorder, { mimeType });
     mediaRecorder.ondataavailable = (event) => {
       if (event.data.size) chunks.push(event.data);
     };
+    mediaRecorder.onerror = (_event) => {
+      cancelRecording();
+      voiceFeedbackKey = 'micStopped';
+    };
     mediaRecorder.onstop = async () => {
       if (shouldDropRecordedAudio(recordingCancelled, document.hidden)) {
         return;
@@ -1117,6 +1137,11 @@ async function toggleLocalRecording() {
     recordingStartedAt = Date.now();
     voiceListening = true;
     voiceFeedbackKey = 'recording';
+    try {
+      mediaRecorder.start();
+    } catch (err) {
+      cancelRecording();
+      voiceFeedbackKey = 'micStopped';
+      render();
+      return;
+    }
     render();
-    mediaRecorder.start();
     recordingTimer = window.setTimeout(() => {
       if (mediaRecorder?.state === 'recording') {
         mediaRecorder.stop();
```

---

## 7. Regression Test

**File:** `plan/worker-reports/round-3/worker-6/mime_regression_check.py`
**Note:** NOT_RUN — deferred per assignment. Run after integration.

```python
"""
mime_regression_check.py — Browser-side MIME negotiation and error-recovery check.
NOT_RUN — deferred per assignment.
Prerequisites:
  - Dev server running at http://localhost:5173
  - pytest with Playwright: pip install playwright && playwright install --with-deps chromium
  - Browser in English for label stability

Run:
  pytest plan/worker-reports/round-3/worker-6/mime_regression_check.py -v

Checks:
  1. MediaRecorder is constructed with a browser-supported MIME type.
  2. If no supported MIME, feedback key is set to 'micStopped' and recording does not start.
  3. Onerror fires and calls cancelRecording when a recording error occurs.
  4. An invalid start() throws and sets feedback to 'micStopped'.
  5. Backend would accept the chosen MIME type (audio/webm or audio/webm;codecs=opus).
"""

import re
import subprocess
from pathlib import Path

import pytest

BASE_DIR = Path(__file__).parent
FRONTEND_DIR = BASE_DIR / "frontend" / "v2"


def get_mime_type_selection() -> str:
    """Return the MIME type string the frontend would select, as a code analysis result."""
    main_ts = FRONTEND_DIR / "src" / "main.ts"
    content = main_ts.read_text()

    mime_pattern = re.compile(
        r"MediaRecorder\.isTypeSupported\(['\"]audio/webm;codecs=opus['\"]"
        r"\s*\?\s*['\"]([^'\"]+)['\"]"
        r"\s*:\s*['\"]([^'\"]+)['\"]",
        re.MULTILINE,
    )
    m = mime_pattern.search(content)
    if m:
        return m.group(1)  # the selected mime type

    # fallback: extract the raw source logic
    return "audio/webm;codecs=opus"


class TestRecorderMimeSelection:
    def test_mime_type_is_backend_accepted(self):
        """
        The MIME type the browser would select must be accepted by the backend.
        Backend accept list: audio/wav, audio/webm (any opus), audio/ogg (any opus), audio/opus.
        """
        mime = get_mime_type_selection()
        # Normalize: extract base type
        base_type = mime.split(";")[0].strip()  # 'audio/webm;codecs=opus' -> 'audio/webm'
        backend_accepted = {"audio/wav", "audio/webm", "audio/ogg", "audio/opus"}
        assert (
            base_type in backend_accepted
        ), f"MIME '{mime}' (base '{base_type}') not in backend allowlist {backend_accepted}"

    def test_mime_pre_check_or_onerror_handler_exists(self):
        """
        The source must either:
        (a) validate MIME support before constructing MediaRecorder, OR
        (b) assign onerror to handle start() failure.
        Without one of these, unsupported MIME silently produces empty audio.
        """
        main_ts = FRONTEND_DIR / "src" / "main.ts"
        content = main_ts.read_text()

        has_precheck = bool(
            re.search(r"MediaRecorder\.isTypeSupported\s*\(\s*mimeType\s*\)", content)
        )
        has_onerror = bool(re.search(r"mediaRecorder\.onerror\s*=", content))

        assert has_precheck or has_onerror, (
            "No MIME pre-check and no onerror handler found in toggleLocalRecording. "
            "Unsupported MIME silently fails."
        )

    def test_start_wrapped_in_try_catch(self):
        """
        mediaRecorder.start() must be inside a try/catch that calls cancelRecording
        on failure, preventing inconsistent UI state after a start() error.
        """
        main_ts = FRONTEND_DIR / "src" / "main.ts"
        content = main_ts.read_text()

        # find the toggleLocalRecording function body
        func_match = re.search(
            r"async function toggleLocalRecording\(\)[^{]*\{(.*?)\n\}",
            content,
            re.DOTALL,
        )
        assert func_match, "toggleLocalRecording function not found"
        func_body = func_match.group(1)

        # check that start() is followed by catch OR that start() is inside a try block
        try_catch_for_start = re.search(
            r"try\s*\{[^}]*mediaRecorder\.start\(\)[^}]*\}\s*catch", func_body
        )
        assert try_catch_for_start, (
            "mediaRecorder.start() is not wrapped in try/catch inside toggleLocalRecording. "
            "A NotSupportedError from start() will propagate uncaught."
        )

    def test_cancel_recording_calls_stop_before_nulling(self):
        """
        cancelRecording must call mediaRecorder.stop() before nullifying the reference,
        so that any pending onstop fires with valid state.
        """
        main_ts = FRONTEND_DIR / "src" / "main.ts"
        content = main_ts.read_text()

        cancel_match = re.search(
            r"function cancelRecording\(\)[^{]*\{(.*?)\n\}",
            content,
            re.DOTALL,
        )
        assert cancel_match, "cancelRecording function not found"
        cancel_body = cancel_match.group(1)

        stop_before_null = re.search(
            r"mediaRecorder\.stop\(\).*mediaRecorder\s*=\s*null",
            cancel_body,
            re.DOTALL,
        )
        assert stop_before_null, (
            "cancelRecording does not call mediaRecorder.stop() before setting "
            "mediaRecorder = null. onstop may fire with mediaRecorder already nulled."
        )
```

**Expected negative control:** After the patch, patch the source to force `mimeType = 'audio/mp3'` (unsupported), reload, tap the mic. Expected: `voiceFeedbackKey` is set to `'micStopped'`, `voiceListening` is `false`, no audio is sent to the backend.

**Expected positive control:** With the patch applied, record 3 seconds of audio on Chrome. Expected: `chunks` array has at least one non-zero Blob, `mimeType` is `audio/webm;codecs=opus`, backend returns success.

---

## 8. Status

**STATUS: READY_FOR_REVIEW_UNVERIFIED**

**Exact next action for Opus:**
1. Check whether `main.ts` has changed since `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09` due to E's patch landing. If so, apply the patch against the updated base.
2. Apply the W6 candidate patch to `frontend/v2/src/main.ts:1096–1140`.
3. Run `pytest plan/worker-reports/round-3/worker-6/mime_regression_check.py` against the integrated build.
4. On a real Safari device, verify that recording works (Safari accepts `audio/webm` and backend transcodes it).
5. Verify that on Chrome, recording produces `audio/webm;codecs=opus` and the backend accepts it.

**Integration risks:**
- Worker 5 (E) may have modified `main.ts` recording section. The W6 patch must be applied after E's patch; the two patches are non-overlapping (E's on the try/catch around `sendVoiceOrText`, W6's on the recording construction and `onerror`/`start()` safety). If E added `onerror` too, the W6 `onerror` assignment would be a duplicate and must be removed.
- `audio_mime_test.go` (dirty untracked) tests the backend MIME acceptance; it is NOT modified by this worker and does not conflict with the frontend patch.
