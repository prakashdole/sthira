# MiniMax E Report — Fix Misleading Voice Outage Fallback

**Worker:** minimax-e
**Timestamp:** 2026-09-27T00:00:00Z (assignment start)
**HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Source file SHA-256 (main.ts):** `ed9dbcd5bbc7a63c46f42d1d63d03157d9644830842213324bbfca11d2294253`
**Output directory:** plan/worker-reports/round-2/minimax-e/

---

## 1. Scope Inspected

- `frontend/v2/src/main.ts` — full file (2197 lines)
- `frontend/v2/src/i18n.ts` — localization strings for assistantUnavailable, backendUnavailable, commandUnavailable
- `frontend/v2/src/audioGuidance.ts` — buildVoicePipelineRequest
- Git status, recent commits d95df9e–e066056

---

## 2. Existing Work vs. New Contribution

| Item | Status |
|------|--------|
| assistantUnavailable / backendUnavailable UI path (lines 1316–1321, 1358–1360, 1369–1373) | Already implemented |
| voiceFeedbackKey = 'backendUnavailable' rendering in render() | Already implemented |
| resolvePlace() legitimate callers (selectCandidatePlace line 659) | Preserved, not changed |
| **Bug: 503/504 transcript fallback to resolvePlace (lines 1299–1304)** | **NEW: removed** |
| **Candidate patch** | **plan/worker-reports/round-2/minimax-e/candidate.patch** |
| **Regression script** | **plan/worker-reports/round-2/minimax-e/voice-outage-fallback.reticle.js** |

---

## 3. Findings

### 3.1 Bug: Misleading "Location not found" during model outage

**File:** `frontend/v2/src/main.ts:1298–1306`
**Evidence (lines 1298–1306):**
```typescript
if (!res.ok) {
  if (res.status === 503 || res.status === 504) {
    if (input.kind === 'transcript' && input.text.trim()) {
      commandPending = false;
      render();
      await resolvePlace(input.text.trim(), reqId);  // <-- BUG
      return;
    }
    throw new Error('MODEL_UNAVAILABLE');
  }
  throw new Error(`Voice pipeline returned ${res.status}`);
}
```

**Impact:** When the voice pipeline returns 503 or 504 (ASR/model outage), a transcript submitted via the text input falls through to `resolvePlace()`. This performs a jurisdiction place lookup against `/api/v3/places/resolve`. Since the input text is not a valid place name, it returns a 404 and sets `commandResponse = 'Location "<text>" not found in jurisdiction DEMO-EXERCISE.'` (line 693). This is semantically incorrect — the model is down, not the place absent. The user submitted a question/command, not a place name.

**Affected callers of sendVoiceOrText:**
- `toggleLocalRecording` (line 1115): audio input — correctly throws MODEL_UNAVAILABLE, goes to catch → `commandUnavailable` + `backendUnavailable` ✓
- Suggestion button click (line 1893): transcript — **hits the bug fallback** ✗
- Form submit (line 1903): transcript — **hits the bug fallback** ✗

### 3.2 Existing correct path for other errors

When `processVoiceEnvelope` returns `outcome.kind === 'ERROR'` (line 1316–1321):
```typescript
commandPending = false;
commandError = words[language].assistantUnavailable;  // "Voice Map Control is unavailable. The displayed route and emergency call option still work."
voiceFeedbackKey = 'backendUnavailable';
render();
```

The `commandError` renders as `<p class="command-error" role="alert">` in the voice console (line 1558 in render). This is the correct, already-implemented UI path for model outages.

### 3.3 Resolution

The transcript-specific fallback to `resolvePlace()` on 503/504 was introduced to handle the case where the voice pipeline fails but the text might be a place query. However, a 503/504 means the service itself is unavailable — retrying or redirecting to a different endpoint does not solve the outage. Both audio and transcript inputs must show `assistantUnavailable` / `commandUnavailable` feedback.

**The fix:** Remove the `if (input.kind === 'transcript' && input.text.trim())` branch inside the 503/504 handler. Both audio and transcript inputs then throw `MODEL_UNAVAILABLE`, which is caught by the existing catch block (line 1368–1373) that sets `commandError = words[language].commandUnavailable` and `voiceFeedbackKey = 'backendUnavailable'`.

**What is NOT changed:**
- `resolvePlace()` itself — still used legitimately in `selectCandidatePlace` (line 659) when a user explicitly selects from ambiguous place candidates.
- Request generation cancellation via `activeRequestId` — already checked inside `resolvePlace` (line 663) and not affected.
- Language handling — not touched.

### 3.4 Uncertainties

- The `resolvePlace` fallback on 503/504 may have been intended to handle a mixed scenario where ASR works but the LLM fails. However, 503/504 from `/api/v3/voice/process` indicates the entire pipeline is unavailable, not just the LLM component.
- If a future partial-outage scenario requires differentiated handling (ASR up, LLM down), a more granular error code would be needed. The current fix assumes 503/504 = whole voice pipeline down.

---

## 4. Candidate Patch

**File:** `plan/worker-reports/round-2/minimax-e/candidate.patch`
**Base SHA-256:** `ed9dbcd5bbc7a63c46f42d1d63d03157d9644830842213324bbfca11d2294253`
**Integration dependency:** None — only `frontend/v2/src/main.ts` changes; no new files, no backend changes, no schema changes.

---

## 5. Verification

**Source inspection:** Performed (main.ts, i18n.ts, audioGuidance.ts, git log)
**Execution tests:** NOT_RUN — deferred by user

**Later verification commands (Opus executes after integration):**

```bash
# Start dev server
cd frontend/v2 && npm run dev

# Run the Reticle regression script
npx reticle run --script plan/worker-reports/round-2/minimax-e/voice-outage-fallback.reticle.js

# Alternative: manual test
# 1. Open app at http://localhost:5173
# 2. Inject 503 for /api/v3/voice/process (browser CDP or test hook)
# 3. Open voice console, type "Show my route" in text input
# 4. Submit — expect "Voice Map Control is unavailable..." in command-error element
# 5. Expect NO "Location not found" message
# 6. Remove 503 injection, submit again — expect normal response
```

**Expected negative control:** Without the fix, submitting text during a 503/504 results in `commandResponse = 'Location "Show my route" not found in jurisdiction DEMO-EXERCISE.'` — this is absent after the fix.

---

## 6. Status

**Status: READY_FOR_REVIEW_UNVERIFIED**

**Next action for Opus:**
1. Apply `candidate.patch` to `frontend/v2/src/main.ts`
2. Verify the app builds (`cd frontend/v2 && npm run build`)
3. Run the Reticle regression script to confirm: (a) 503 on voice pipeline + text submit → `command-error` element with "unavailable" text and no "not found"; (b) 200 restored → normal command flow resumes
4. Commit with message: `fix(frontend): remove misleading resolvePlace fallback for voice pipeline 503/504`

**Critical blocker:** None — the fix is surgical (7 lines removed) and preserves all legitimate `resolvePlace` callers and the existing `assistantUnavailable`/`backendUnavailable` UI path.
