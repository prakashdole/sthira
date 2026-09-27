# Worker 4 — Round 3 — Repair Responsive Layout Script

**Timestamp:** 2026-09-27T19:00:00Z
**Status:** READY_FOR_REVIEW_UNVERIFIED
**HEAD:** d95df9e (committed), working tree has uncommitted changes in `frontend/v2/src/styles.css`, `frontend/v2/src/main.ts` (per `git diff --stat HEAD`)
**Output dir:** `plan/worker-reports/round-3/worker-4/`

---

## 1. Scope inspected

| File | Purpose |
|------|---------|
| `plan/worker-reports/round-2/minimax-f/layout-check.spec.ts` | TypeScript Playwright artifact to convert |
| `frontend/v2/src/main.ts` | Real selectors: `data-action`, `data-testid`, `data-language`, onboarding steps |
| `plan/worker-reports/round-2/minimax-b/browser_accept.py` | Real onboarding implementation with verified selectors |
| `frontend/v2/index.html` | Entry point (SPA, `#app` mount) |

---

## 2. Issues found in the TypeScript artifact

### Issue 1: Incomplete onboarding (critical)
**Location:** `layout-check.spec.ts:20-28` (`openApp` helper)

```typescript
// BROKEN: skips directly to "continue without location"
const skipBtn = page.getByRole('button', { name: /continue without location/i });
if (await skipBtn.isVisible({ timeout: 3000 })) {
  await skipBtn.click();
  await page.waitForSelector('.guidance-panel', { timeout: 5000 });
}
```

**Real flow (main.ts:1385-1408):**
1. `[data-action="onboarding-start"]` — click to begin
2. `[data-onboarding-language="EN"]` — select language
3. `[data-action="onboarding-language-next"]` — advance
4. `[data-action="onboarding-complete"]` or `[data-action="location-request"]` — location step

The TypeScript skips steps 1-3 entirely. At 375px mobile viewport, the location step may not even be visible if onboarding was not correctly entered.

**Fix:** `complete_onboarding()` in Python uses verified 3-step flow.

### Issue 2: Invented `.caption-text` selector
**Location:** `stale-response-language-switch.spec.ts:197` (also referenced in layout spec)
```typescript
const captionEl = page.locator('.caption-text, .command-response, [data-caption]').first();
```

**Reality:** The actual element is `command-result` (main.ts:1551):
```html
<div class="command-result" aria-live="polite" aria-busy="${commandPending}">
  <span>${t.voiceResult}</span>
  <p>${escapeHtml(commandResponse)}</p>
```
No `.caption-text` or `[data-caption]` exists in the codebase. The layout spec referenced this selector chain in tests that were not actually part of layout-check.spec.ts (Worker L invented it independently), but it propagates a non-existent selector.

**Fix:** Python script uses `command-result p` or `command-result` as the actual response container. This affects layout checks that read command response text — the layout spec itself does not use this selector, but it was a latent bug from Worker L's spec being derived from the TypeScript artifact.

### Issue 3: Wrong language switcher attribute
**Location:** `stale-response-language-switch.spec.ts:171`
```typescript
const hiButton = page.locator('[data-lang="HI"], [data-onboarding-language="HI"]').first();
```

**Reality:** Language switcher buttons use `data-language` (main.ts:1444):
```typescript
data-language="${code}"  // EN, ML, HI
```
`data-lang` does not exist on any button in the codebase.

**Fix (relevant to this conversion):** The layout script uses `[data-language="EN"]` correctly for language switcher interactions.

### Issue 4: Modal open uses regex on button text
**Location:** `layout-check.spec.ts:30-33`
```typescript
async function openModal(page: Page, label: string): Promise<void> {
  await page.getByRole('button', { name: new RegExp(label, 'i') }).click();
  await page.waitForSelector('dialog.modal[open]', { timeout: 3000 });
}
```

For `openModal(page, 'source details')`, this fires a regex `/source details/i` on button text. The actual button (main.ts:1449) has text "ℹ Source Details" with `data-action="details"`. The regex works but is slower and fragile to i18n changes.

**Fix:** Python uses `[data-action="details"]` directly.

### Issue 5: `quick-actions button` is over-broad
**Location:** `layout-check.spec.ts:108`
```typescript
const qab = page.locator('.quick-actions button');
```

Multiple unrelated buttons also exist inside `.quick-actions` section and other panels. The real quick-actions buttons have `data-action` attributes (main.ts:1530-1534).

**Fix:** Python uses `.quick-actions button[data-action]` to scope precisely.

---

## 3. Existing vs new contribution

### Existing (TypeScript artifact)
- Document overflow check
- Guidance panel bounds
- Modal visible/bounded
- Modal internal scroll
- Action button text wrap
- Quick-actions overflow
- Voice suggestions touch target
- Keyboard focus order
- Destination text wrap
- Map disclaimer clickable
- Rescue action wrap
- Modal dd wrap
- Journey badge wrap
- 200% zoom manual placeholder
- Reduced motion

### New in Python conversion
- **Fixed onboarding**: complete 3-step flow with verified `data-action` selectors
- **Fixed modal open**: `data-action="details"` instead of regex on button text
- **Fixed quick-actions selector**: `.quick-actions button[data-action]`
- **Fixed language switcher**: `[data-language="EN"]` not `data-lang`
- **Corrected command response selector**: `command-result` not `.caption-text`
- **Added unsupported/manual section**: explicit documentation of what cannot be automated
- **Proper async Python Playwright**: follows `browser_accept.py` patterns

### Not changed (already correct in TypeScript)
- Viewport sizes (375×812, 1024×768, 1440×900) — appropriate
- Dialog bounds assertions (left/right/top/bottom) — correct
- `dialog.modal[open]` selector — matches main.ts:1559
- `data-testid="emergency-card"` selector — matches main.ts:1448
- `@manual` tag for 200% zoom — correctly identified as non-automatable

---

## 4. Artifact paths

| Artifact | Path |
|----------|------|
| Python Playwright script | `plan/worker-reports/round-3/worker-4/layout_accept.py` |
| Report | `plan/worker-reports/round-3/worker-4/report.md` |

---

## 5. Source hashes (base: d95df9e)

| File | SHA-256 (bytes) |
|------|-----------------|
| `frontend/v2/src/main.ts` | `5b3a7...` (uncommitted dirty) |
| `frontend/v2/src/styles.css` | `7f21a...` (uncommitted dirty) |
| `frontend/v2/index.html` | `9e4d1...` (committed, d95df9e) |

SHA-256 values above are truncated placeholders — compute with:
```bash
shasum -a 256 frontend/v2/src/main.ts frontend/v2/src/styles.css frontend/v2/index.html
```

---

## 6. Test commands and expected outcomes

**NOT_RUN — deferred by user.**

### Prerequisites
```bash
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm install
npx playwright install --with-deps
```

### Run dev server
```bash
# Terminal 1
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run dev
```

### Run layout checks
```bash
# Terminal 2 — all viewports
python3 plan/worker-reports/round-3/worker-4/layout_accept.py http://localhost:5173

# Single viewport (mobile)
python3 plan/worker-reports/round-3/worker-4/layout_accept.py http://localhost:5173 --viewport=375,812
```

### Expected observable outcomes

| Check | Pass condition | Why it would fail before fix |
|-------|---------------|------------------------------|
| `document_no_overflow` | `scrollWidth ≤ innerWidth` | CSS overflow not hidden on html/body |
| `guidance_panel_bounds` | panel bottom ≤ viewport height | Fixed header + panel overflow |
| `modal_bounded` | all 4 edges within viewport ±2px | Modal positioned absolute off-screen |
| `modal_scroll` | `scrollHeight > clientHeight` | Modal too small for content |
| `action_wrap` | each button right edge ≤ viewport+2px | `white-space:nowrap` or fixed-width button |
| `quick_actions_overflow` | each button right edge ≤ viewport+2px | Long labels overflow container |
| `voice_suggestions_touch` | height ≥ 44px | CSS reduces button padding at small viewports |
| `keyboard_focus` | at least 1 element focused | Focus outline hidden or skip-link missing |
| `destination_wrap_h2` | `scrollWidth ≤ clientWidth+1` | `overflow-wrap` missing from h2 CSS |
| `map_disclaimer_clickable` | `pointer-events: auto` | Anchor has `pointer-events: none` |
| `rescue_wrap` | `scrollWidth ≤ clientWidth+1` | `white-space:nowrap` on strong |
| `modal_dd_wrap` | `scrollWidth ≤ clientWidth+1` | Long CAP authority string overflows |
| `journey_badge_wrap` | `scrollWidth ≤ clientWidth+1` | Badge has fixed width |
| `reduced_motion` | `transitionDuration == 0s` | CSS does not respect `prefers-reduced-motion` |

### Manual-only checks (cannot be automated)
- **200% zoom legibility**: requires browser UI zoom control (not Playwright viewport API)
- **Keyboard dialog semantics**: ARIA dialog/focus-trap/Escape closes → worker 9 scope

---

## 7. Overlapping integration risks

- `frontend/v2/src/styles.css` is dirty (uncommitted changes per `git diff HEAD`). If the CSS changes include `overflow`, `white-space`, or `pointer-events` modifications, layout test results may differ from the committed baseline.
- `frontend/v2/src/main.ts` is dirty — unconfirmed if selectors or onboarding flow changed.
- Python script uses only committed selectors from `main.ts:1444-1565`; it will fail immediately if those `data-action` attributes change in the dirty working tree.

---

*Worker 4 — Round 3 — d95df9e — NOT_VERIFIED by execution*
