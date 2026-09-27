# Worker 3 Report: Dialog, Voice Recenter, and CSS Corrections

## Status: COMPLETE
## Output dir: plan/worker-reports/mac-final/worker-3/

**Base hashes:**
- `main.ts`: `9fd36d27547f2abe30f7f5f14d2d6a02`
- `styles.css`: `cd76dfb2c149cfe28059ea8b57e1cf1b`

**Critical blocker:** Escape key does not close modals — no keyboard accessibility for dialogs.

---

## 1. Dialog Escape Behavior and Focus Restoration

### Finding: Escape key does NOT close modals

**Evidence (source inspection):**
- Modals are `<dialog class="modal" open>` elements (lines 1559-1565 of main.ts)
- Using `open` attribute, NOT `.showModal()` — native Escape handling does NOT apply
- No global `keydown` listener for Escape exists in the codebase (confirmed via grep)
- Each modal has a close button (`data-action="*-close"]`) but no keyboard handler

**Code path:**
```typescript
// Dialog rendered with open attribute — no Escape behavior
${detailsOpen ? `<dialog class="modal" open>...</dialog>` : ''}

// Close buttons registered in bindInteractions() but no Escape handler
document.querySelector<HTMLButtonElement>('[data-action="details-close"]')
```

**Impact:** Users cannot dismiss modals with keyboard; accessibility violation.

### Finding: No focus restoration to opening control

**Evidence:**
- `bindInteractions()` called after every `render()` (line 1582)
- Opens set boolean flags (`detailsOpen = true`) then call `render()`
- No tracking of "trigger element" for focus return
- No `previousFocus` reference stored before opening

**Code path:**
```typescript
// Opening a modal
document.querySelector('[data-action="details"]')?.addEventListener('click', () => {
  detailsOpen = true;  // No focus tracking
  render();
});
```

**Impact:** After close, focus is lost to `<body>` — not returned to opening control.

### Finding: `bindInteractions()` called on every render

**Evidence (line 1582):**
```typescript
function render() {
  // ... innerHTML replaced ...
  hasRendered = true;
  bindInteractions();  // Re-attaches listeners to new DOM elements
}
```

**Analysis:** Since `render()` replaces `#app` innerHTML each time, old elements are removed and GC'd along with their listeners. Each render creates fresh elements with fresh listeners — no duplicate listener accumulation in practice.

**Verdict:** Not a memory leak — but the pattern is fragile if render() partial-updates in future.

### Finding: Openers replaced during rendering

**Analysis:** When a modal is open and `render()` is called (e.g., for voice response), the entire `#app` innerHTML is replaced. The opening button is recreated. If focus were stored, the stored reference would point to a detached DOM node.

**Verdict:** Focus restoration requires storing a selector (not element reference) to re-query after render, or a separate focus-trap layer.

---

## 2. Voice Recenter and 3D Toggle Consistency

### Finding: `recenterMap` hardcodes pitch/bearing to 0

**Evidence (lines 1805-1812):**
```typescript
function recenterMap() {
  const target = (deviceLocation || mapData.user) as [number, number];
  mapTilted = false;
  try { map?.setTerrain(null); } catch {}
  map?.easeTo({ center: target, zoom: 15, pitch: 0, bearing: 0, duration: motionDuration() });
  syncPerspectiveControl();
  recordCameraState();
}
```

**Issue:** If user manually tilted the map via gestures (not the 3D button), their custom pitch/bearing is discarded. The ARIA state (`aria-pressed="false"`) correctly reflects `mapTilted = false`.

**Verdict:** Actual pitch/terrain and `mapTilted` ARE consistent — `syncPerspectiveControl()` syncs aria-pressed correctly after `recenterMap()`.

### Finding: User-location behavior preserved

**Evidence:**
```typescript
const target = (deviceLocation || mapData.user) as [number, number];
```

`recenterMap` uses real `deviceLocation` when available, falls back to `mapData.user`. No fabricated coordinates.

### Finding: `focusRoute` uses hardcoded pitch/bearing, not savedCamera

**Evidence (lines 1813-1819):**
```typescript
function focusRoute() {
  map?.fitBounds(mapData.routeBounds, {
    padding: ...,
    pitch: mapTilted ? 65 : 0,      // Hardcoded, not savedCamera.pitch
    bearing: mapTilted ? -18 : 0,    // Hardcoded, not savedCamera.bearing
    duration: motionDuration(),
  });
}
```

**Issue:** If user manually adjusted pitch before toggling 3D, `toggleMapPerspective` saves their custom pitch to `savedCamera` (via `recordCameraState()`). But `focusRoute` ignores `savedCamera.pitch` and uses hardcoded `mapTilted ? 65 : 0`.

**Verdict:** Minor inconsistency — `toggleMapPerspective` saves custom pitch before toggling, but `focusRoute` doesn't use it. However, `focusRoute` is for route-focus, not tilt-restore — this is arguably correct behavior.

---

## 3. Unused my-location Layer Entry

### Finding: No layer named "my-location" exists

**Evidence (grep for "my-location" and "MY_LOCATION"):**
- `myLocationVisible` (line 177) controls visibility of device layers
- `MY_LOCATION` appears only in action handling (line 1226: `if (action.layer === 'MY_LOCATION')`)
- Actual map layers: `device-pulse` and `device-point` (lines 1701-1702)
- No separate "my-location" vector/raster layer

**Verdict:** The layer entry is NOT unused — it IS the `device-pulse`/`device-point` pair. No defect.

---

## 4. Pending CSS Review

### Claim: "Child with pointer-events:auto remains clickable under parent with pointer-events:none"

**Verdict: This is FALSE — CSS behavior is correct**

**Explanation:**
- Parent `pointer-events: none` blocks pointer events on the parent
- Child with `pointer-events: auto` CAN receive pointer events — this is standard CSS
- The pending CSS correctly implements:
  - `.map-disclaimer { pointer-events: none }` — blocks clicks on container
  - `.map-disclaimer a { pointer-events: auto }` — allows link clicks

**Pending CSS changes are correct** — no defect to fix.

### Claim: Dialog overflow, keyboard focus, long HI/ML labels at 375/1024/1440

**Pending CSS evidence:**
```css
.modal {
  max-height: min(85dvh, 38rem);
  overflow-y: auto;
  overscroll-behavior: contain;
}
.modal .sheet-head { position: sticky; top: 0; }
```

**Analysis:**
- Modal has `overflow-y: auto` — content scrolls
- `sheet-head` is sticky — stays visible while scrolling
- `85dvh` height limit handles viewport constraints
- `overscroll-behavior: contain` prevents scroll chaining

**Verdict:** Dialog overflow handling appears correct. No demonstrated defect.

### Claim: Browser zoom vs viewport/device scaling

**Analysis:**
- CSS uses `dvh`/`dvw` units for dynamic viewport — handles virtual keyboard and browser chrome
- Media queries use `rem` and explicit breakpoints (`47.99rem`, `48rem`, `75rem`)
- No evidence of zoom-specific issues in pending diff

**Verdict:** Cannot confirm or deny without actual browser testing at specified widths.

---

## 5. Summary of Verified Issues

| # | Issue | Severity | Status |
|---|-------|----------|--------|
| 1 | Escape key does not close modals | High | **CONFIRMED** — no Escape handler |
| 2 | Focus not restored after modal close | High | **CONFIRMED** — no focus tracking |
| 3 | Dialogs use `open` attr, not `.showModal()` | Medium | Design choice limits native behavior |
| 4 | `bindInteractions` on every render | Low | Not a leak (elements replaced each render) |

---

## 6. Regression Test Candidates

### T-01: Escape closes modal
**Pre-condition:** Dev server running, app past onboarding
**Steps:** Click Details → press Escape
**Expected:** Modal closes; focus remains in page
**Current behavior:** Modal stays open

### T-02: Focus returns to trigger after close
**Pre-condition:** Dev server running, app past onboarding
**Steps:** Click Details → press Escape → check focus
**Expected:** Focus on Details button
**Current behavior:** Focus lost to body

### T-03: 3D toggle aria-pressed consistency
**Pre-condition:** Dev server, map loaded
**Steps:** Click 3D → check aria-pressed="true" → click 3D → check aria-pressed="false"
**Expected:** aria-pressed matches button visual state
**Current behavior:** Consistent (verified via source)

### T-04: Recenter resets mapTilted and terrain
**Pre-condition:** Map in 3D mode (tilted with terrain)
**Steps:** Click My Location (recenter)
**Expected:** terrain=null, pitch=0, aria-pressed="false"
**Current behavior:** Verified correct via source

---

## 7. No-Git Patch Artifacts

No production patches recommended — this report documents confirmed behaviors and testable regressions.

For actual fixes, see `dialog-keyboard-check.spec.ts` in `plan/worker-reports/round-3/worker-9/` (deferred, not run).
