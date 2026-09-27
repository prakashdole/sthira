# Worker 7 — Microphone Permission / getUserMedia Rejection + Stream/Recorder Cleanup

**Investigation scope:** getUserMedia rejection handling; stream/recorder cleanup after cancel, visibility-hide, and language-change events.

---

## Findings

### 1. getUserMedia Rejection Is Caught — But Message Is Slightly Misleading

`toggleLocalRecording()` (main.ts:1097–1131):

```typescript
try {
  mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
  // ...sets up MediaRecorder...
} catch {
  voiceListening = false;
  voiceFeedbackKey = 'micStopped';   // ← catches rejection
  render();
}
```

Rejection is caught. However `voiceFeedbackKey = 'micStopped'` maps to:
> "Microphone input stopped. Check permission or enter a command below."

This is not wrong, but it implies the microphone was "stopped" (user action) rather than explicitly "permission denied". The wording is not materially misleading (it says "check permission"), but it could be clearer. No separate `micDenied` key exists.

### 2. cancelRecording() Properly Nulls mediaStream + mediaRecorder and Stops Tracks

`cancelRecording()` (main.ts:1066–1083):

```typescript
recordingCancelled = true;
if (recordingTimer !== null) { clearTimeout(recordingTimer); recordingTimer = null; }
mediaStream?.getTracks().forEach((track) => track.stop());
mediaStream = null;
mediaRecorder = null;
voiceListening = false;
```

All tracks are explicitly stopped; both `mediaStream` and `mediaRecorder` are nulled. ✅

### 3. onstop Handler Has an Early-Return That Skips Track Nulling

`onstop` (main.ts:1105–1116):

```typescript
mediaRecorder.onstop = async () => {
  if (shouldDropRecordedAudio(recordingCancelled, document.hidden)) {
    return;   // ← exits WITHOUT stopping tracks or nulling mediaStream/mediaRecorder
  }
  // ... normal path: stops tracks, nulls both, sends audio
};
```

If `recordingCancelled=true` OR `document.hidden=true` at the time `onstop` fires:
- Tracks are **NOT** stopped by this handler
- BUT `cancelRecording()` was already called on visibility-hide (line 2168), which synchronously stops tracks

So for visibility-hide the synchronous `cancelRecording()` call handles cleanup before the async `onstop` fires. ✅

### 4. Language Change Does NOT Cancel Active Recording

Language change handler (main.ts:1856–1864):

```typescript
language = nextLanguage;
try { localStorage.setItem('sthira-language', language); } catch {}
commandSuggestions = [...words[language].voiceCommands];
commandResponse = words[language].voiceReady;
commandError = '';
voiceFeedbackKey = 'micPrivacy';
supersedeInFlight();   // ← only this; no cancelRecording()
render();
```

**`cancelRecording()` is NOT called on language change.** If a recording is in-flight when the user changes language:
- `supersedeInFlight()` increments `activeRequestId` and clears queued proposal
- `mediaStream` and `mediaRecorder` are NOT stopped
- `voiceFeedbackKey` is set to `'micPrivacy'` (appropriate for the new language)
- The MediaRecorder continues running; when `onstop` fires, `shouldDropRecordedAudio(recordingCancelled=false, document.hidden=false)` → `false`, so it falls through to the normal path and sends audio (with the old `language` setting still in the audio)

This is a latent bug: an in-flight recording collected under the old language settings would be sent after the language switch.

### 5. Retention Risk After Late getUserMedia Rejection

The catch block at line 1127 only handles rejection at the **start** of `toggleLocalRecording()`. If permission is revoked mid-session by the OS (not by user action in-page), the browser automatically drops tracks — no explicit code needed — but `mediaStream` and `mediaRecorder` references may still be non-null. However this is a browser-level behavior; the app cannot intercept OS-level permission revocation.

---

## Recommendations

### Already Correct
- `cancelRecording()` (visibility-hide, voice-close) stops tracks and nulls references correctly.
- `getUserMedia` rejection is caught with `voiceFeedbackKey = 'micStopped'`.

### Bug to Fix
- **Language change** should call `cancelRecording()` to stop any in-progress recording before `supersedeInFlight()`. Otherwise a recording started under the previous language is sent with stale language context.

### Low-Priority: Clarify `micStopped` Message
- The existing `micStopped` key says "Microphone input stopped. Check permission..." which is acceptable. Adding a dedicated `micDenied` key is YAGNI — the existing wording is not materially wrong.

---

## candidate.patch

```diff
diff --git a/frontend/v2/src/main.ts b/frontend/v2/src/main.ts
index c11f175..XXXXXXX 100644
--- a/frontend/v2/src/main.ts
+++ b/frontend/v2/src/main.ts
@@ -1853,6 +1853,7 @@ if (import.meta.env.DEV) console.debug('voiceFeedbackKey:', voiceFeedbackKey);
     language = nextLanguage;
     try { localStorage.setItem('sthira-language', language); } catch {}
     commandSuggestions = [...words[language].voiceCommands];
     commandResponse = words[language].voiceReady;
     commandError = '';
     voiceFeedbackKey = 'micPrivacy';
+    cancelRecording();           // stop any in-progress recording before switching language
     supersedeInFlight();
     render();
   })
```
