# MiniMax L — Language switching and stale-response regression
**Timestamp:** 2026-09-27T18:50:00Z
**Status:** READY_FOR_REVIEW_UNVERIFIED
**HEAD:** d95df9e (docs: record handoff commit id)

---

## 1. Checkpoint and scope

| Item | Value |
|------|-------|
| HEAD | d95df9e |
| Relevant dirty files | `frontend/v2/src/styles.css` (out of scope) |
| Scope inspected | `frontend/v2/src/main.ts`, `frontend/v2/src/journey.ts`, `frontend/v2/src/audioGuidance.ts`, `frontend/v2/src/journey.test.ts`, `frontend/v2/src/audioGuidance.test.ts` |
| Assignment scope | Language and delayed response behavior; E owns outage, H audio lifecycle, F layout |

**Scope inspected:**
- `frontend/v2/src/main.ts` — request generation, `supersedeInFlight()`, `sendVoiceOrText()`, language selection in onboarding and topbar, `dispatchVoiceProposal()` SET_LANGUAGE handling
- `frontend/v2/src/journey.ts` — `shouldDropResponse()` guard, `isGuidanceActionCurrent()`, `canChangeSelection()`
- `frontend/v2/src/audioGuidance.ts` — `AudioPlaybackGuard`, `isAudioValidForReplay()`, `processVoiceEnvelope()`
- Existing unit tests: `journey.test.ts`, `audioGuidance.test.ts`
- Playwright pattern from `plan/worker-reports/round-2/minimax-f/layout-check.spec.ts`

---

## 2. Existing completed work vs new contribution

### Existing coverage

**Unit-level stale-response guards (journey.test.ts:265-280, audioGuidance.test.ts:345-416):**
- `shouldDropResponse()` unit tests: checks active-ID mismatch and document-hidden guards
- `AudioPlaybackGuard` unit tests: language/version/freshness invalidation
- `isGuidanceActionCurrent()` tests: data_version mismatch blocks guidance actions

**These unit tests pass in isolation but do NOT constitute a browser-level regression check for the language-switch scenario.**

### New contribution

**Primary artifact:** `plan/worker-reports/round-2/minimax-l/stale-response-language-switch.spec.ts`

A single Playwright regression script covering:
1. **EN→HI mid-flight**: delayed EN response is dropped after language switch; caption stays HI, destination unchanged, fresh HI request succeeds
2. **Onboarding language switch**: same protection during onboarding flow
3. **EN→ML mid-flight** (discriminating evidence): language tag, caption, destination all preserved; ML confirmation
4. **Negative control** (tagged `@negative-control`, not executed this round): without a switch, the delayed EN response IS accepted and caption shows EN text — defines what "accepted" looks like to contrast with the regressions above

**Key design decisions:**
- Uses `page.route` for true network-level response delay (not timers or mocking)
- No `data-language` attribute as the sole evidence — asserts actual `document.documentElement.lang`, caption text content, and destination text
- Captures initial destination before the race condition to assert it is unchanged after
- Distinguishes visually: EN text `"Here are your safe shelter options"` vs HI text `यहाँ आपके सुरक्षित आश्रय विकल्प हैं।`
- Negative control is defined but not executed this round (as specified)

---

## 3. Source-backed findings

### Finding 1: supersedeInFlight() correctly invalidates all in-flight state

**Evidence:** `main.ts:244-250`

```typescript
function supersedeInFlight() {
  activeRequestId++;
  queuedVoiceProposal = null;
  commandPending = false;
  audioGuard.invalidate();
  lastApprovedAudio = undefined;
}
```

**Impact:** Language switches (both onboarding selection at `:1407` and `SET_LANGUAGE` dispatch at `:1235-1241`) call `supersedeInFlight()`, incrementing `activeRequestId`. All subsequent responses with the old `responseRequestId` are dropped by `shouldDropResponse()` at `main.ts:1294` and `:1312`.

### Finding 2: shouldDropResponse() is the primary gate

**Evidence:** `journey.ts:364-368`

```typescript
export function shouldDropResponse(activeRequestId: number, responseRequestId: number, isHidden: boolean): boolean {
  if (isHidden) return true;
  if (activeRequestId !== responseRequestId) return true;
  return false;
}
```

**Impact:** Both `sendVoiceOrText()` call sites check this guard after parsing the response envelope. An incrementing `activeRequestId` makes stale responses unrecoverable. No code path bypasses this check for voice responses.

### Finding 3: AudioPlaybackGuard provides async playback protection

**Evidence:** `audioGuidance.ts:265-318`

The guard tracks generations. When `invalidate()` is called (via `supersedeInFlight()`), `currentGeneration` increments and `canPlayPending()` returns `false` for any pending audio from the old generation.

**Impact:** Even if a delayed audio metadata response passes the request-ID check, `verifyAndPlayAudio()` at `main.ts:730-742` re-checks `audioGuard.currentGeneration` before playback. Late audio cannot start after a language switch.

### Finding 4: isGuidanceActionCurrent() prevents stale proposal side effects

**Evidence:** `journey.ts:584-586`

```typescript
export function isGuidanceActionCurrent(proposalDataVersion: string | undefined, currentDataVersion: string): boolean {
  return typeof proposalDataVersion === 'string' && proposalDataVersion !== '' && proposalDataVersion === currentDataVersion;
}
```

**Impact:** `dispatchVoiceProposal()` at `main.ts:1228-1231` checks `isGuidanceActionCurrent()` before executing non-map actions (SHOW_CHOICES, OPEN_PANEL). A proposal with a stale `data_version` is rejected with `"Response was built for different guidance data"`.

### Finding 5: destination selection is pinned by accepted stay

**Evidence:** `journey.ts:589-591`

```typescript
export function canChangeSelection(acceptedStayFacilityId: string | null, nextFacilityId: string | null | undefined): boolean {
  return !acceptedStayFacilityId || acceptedStayFacilityId === nextFacilityId;
}
```

**Impact:** `onDestinationSelectionChanged()` at `main.ts:326-349` calls `canChangeSelection()`. An accepted stay (from reservation) locks the destination; stale responses that attempt to call `applyDestinationChoices()` cannot overwrite the locked selection.

---

## 4. Gap analysis

### No high-value missing regression patches identified

All seven coverage categories from the unit tests map to the browser-level protections above. The existing code is correctly structured. No speculative patches required.

### Existing browser-level test gap

The unit tests for `shouldDropResponse` and `AudioPlaybackGuard` pass in isolation. The Playwright script provides the missing browser-level regression coverage that exercises the full integration path (network → parse → guard check → state mutation → DOM update).

---

## 5. Artifact paths

| Artifact | Path |
|----------|------|
| Playwright regression | `plan/worker-reports/round-2/minimax-l/stale-response-language-switch.spec.ts` |
| Report | `plan/worker-reports/round-2/minimax-l/report.md` |

---

## 6. Verification (NOT_RUN — deferred by user)

**Setup required:**
```bash
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm install
npx playwright install --with-deps
```

**Dev server:**
```bash
# Terminal 1
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run dev
```

**Run the regression:**
```bash
# Terminal 2
npx playwright test \
  plan/worker-reports/round-2/minimax-l/stale-response-language-switch.spec.ts \
  --reporter=line
```

**Expected outcomes:**

| Test | Expected result | Why |
|------|----------------|-----|
| `delayed EN response is dropped after switching to HI` | PASS | `supersedeInFlight()` increments `activeRequestId`; EN response ID mismatch → dropped |
| `onboarding language switch also supersedes in-flight response` | PASS | Same mechanism; onboarding language change calls `supersedeInFlight()` |
| `ML language switch is also protected` | PASS | ML switch calls `supersedeInFlight()`; caption, destination, lang tag all preserved |
| `@negative-control without language switch` | PASS (when run) | No `supersedeInFlight()` call; delayed response is processed and caption shows EN text |

**Negative control:** Run with `@negative-control` tag to confirm the delay mechanism itself works — if this fails, the test infrastructure (network interception) is broken, not the regression.

**Expected negative control result:**
- Without a language switch, the delayed EN response IS accepted
- Caption shows `"Here are your safe shelter options."`
- HTML lang is `"en"`
- Destination selection unchanged

---

## 7. Status and next action

**Status: READY_FOR_REVIEW_UNVERIFIED**

The language-switch stale-response regression coverage is complete at the source level. All five protection mechanisms (`supersedeInFlight()`, `shouldDropResponse()`, `AudioPlaybackGuard`, `isGuidanceActionCurrent()`, `canChangeSelection()`) are mapped with file:line evidence. The Playwright script provides the missing browser-level discrimination test that the unit tests alone cannot provide.

**Next action for Opus:**
- Review `plan/worker-reports/round-2/minimax-l/stale-response-language-switch.spec.ts`
- Verify the network interception pattern is correctly structured for the voice/process endpoint
- Stage and run the Playwright tests against a live dev server
- Confirm ML language test distinguishes ML from HI/EN evidence (language tag assertion checks `['ml']` specifically)

---

*MiniMax L — language switching and stale-response — d95df9e — NOT_VERIFIED by execution*
