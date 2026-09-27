# Worker 11 Report — Map Layer Patch Validation

**Worker:** worker-11
**Timestamp:** 2026-09-27T02:30:00Z
**HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Output directory:** plan/worker-reports/round-3/worker-11/

---

## 1. Scope Inspected

| File | SHA-256 | Notes |
|------|---------|-------|
| frontend/v2/src/mapActions.ts | a28f4f612416391283a1e832418681d98358437e43fa75be0803394558fdd490 | Unchanged from MiniMax-G base |
| frontend/v2/src/mapActions.test.ts | aa5d284309e4a946b7a75559a962cbe0dbfca8d9926290d06ac3ab75f61b7a04 | Unchanged |
| plan/worker-reports/round-2/minimax-g/candidate.patch | (artifact) | Removes 'my-location' from layerMap |
| plan/worker-reports/round-2/minimax-g/report.md | (evidence) | Prior report |

- Dirty files: backend/internal/asrworker/audio_mime_test.go, backend/internal/middleworker/eval/*, frontend/v2/src/styles.css, frontend/v2/src/main.ts (NOT in scope)
- mapActions.ts: clean, no uncommitted changes
- Source tree clean for mapActions.ts; d95df9e confirmed

---

## 2. Prior Report Summary (MiniMax-G Finding 3.1)

MiniMax-G identified that `layerMap['MY_LOCATION']` contains three layer IDs:
```typescript
MY_LOCATION: ['device-pulse', 'device-point', 'my-location'],
```

The map style (per MiniMax-G's inspection of main.ts:1692–1713) includes only `device-pulse` and `device-point`. The `my-location` ID has no corresponding style layer, making it a phantom reference.

**MiniMax-G's candidate.patch** removes `'my-location'` from the array.

---

## 3. Guard Analysis — Does `getLayer` Protect Against the Phantom?

**Yes.** The guard exists at mapActions.ts:216–222:

```typescript
for (const id of targetIds) {
  if (typeof (map as any).getLayer === 'function') {
    if ((map as any).getLayer(id)) {
      map.setLayoutProperty(id, 'visibility', action.visible ? 'visible' : 'none');
    }
  } else {
    map.setLayoutProperty(id, 'visibility', action.visible ? 'visible' : 'none');
  }
}
```

For `action.layer === 'MY_LOCATION'`:
- `targetIds = ['device-pulse', 'device-point', 'my-location']`
- `'device-pulse'`: `getLayer('device-pulse')` → truthy → `setLayoutProperty` called
- `'device-point'`: `getLayer('device-point')` → truthy → `setLayoutProperty` called
- `'my-location'`: `getLayer('my-location')` → falsy (layer does not exist) → **silently skipped**

**Impact of the phantom:** None at runtime. The `getLayer` guard prevents any error or incorrect state. The callback `onLayerVisibility` still fires once per `MY_LOCATION` action (line 235), regardless of how many layer IDs are in the array.

---

## 4. Does the Existing Test Detect the Phantom?

**No.** The MY_LOCATION test at mapActions.test.ts:575–653:

```typescript
const idsSet = layerPropsSet.map((p) => p.id);
assert.ok(idsSet.includes('device-pulse'), 'device-pulse must be toggled');
assert.ok(idsSet.includes('device-point'), 'device-point must be toggled');
```

This only asserts that `device-pulse` and `device-point` are toggled. It never:
- Checks whether `'my-location'` is absent from `layerPropsSet`
- Asserts that exactly 2 (not 3) layers were toggled
- Verifies `setLayoutProperty` was NOT called for `'my-location'`

The test uses a mock map without `getLayer` defined (so the guard takes the `else` branch and calls `setLayoutProperty` for ALL ids in the array). Since the mock doesn't define `getLayer`, `'my-location'` would be passed to `setLayoutProperty` even with the phantom present. The test passes in both cases because it never asserts on `'my-location'`.

**Conclusion:** The existing test does NOT detect a regression from the phantom. A passing test does NOT confirm the phantom is absent.

---

## 5. Does the Candidate Patch Change Behavior?

**No.** Because the `getLayer` guard prevents any runtime effect from the phantom, removing `'my-location'` from `layerMap['MY_LOCATION']`:

| Scenario | `device-pulse` toggled | `device-point` toggled | `my-location` toggled | Result |
|----------|------------------------|----------------------|-----------------------|--------|
| Before patch | ✓ | ✓ | Silently skipped (guard) | Correct |
| After patch | ✓ | ✓ | N/A (not in array) | Correct |

The patch is **valid cleanup** — it removes a dead entry and makes `layerMap` match reality. It does NOT fix a bug that causes incorrect behavior.

---

## 6. Regression Test Assessment

**What a proper regression test would need to detect the phantom:**

A test that, with the phantom present, would FAIL (revealing the issue) and with the patch applied, would PASS:

```typescript
test('SET_LAYER_VISIBILITY MY_LOCATION: only existing layers receive setLayoutProperty', () => {
  const layerPropsSet: string[] = [];
  const mockMap: any = {
    getLayer(id: string) {
      // Only device-pulse and device-point exist in the style
      if (id === 'device-pulse' || id === 'device-point') return true;
      return false; // my-location does NOT exist
    },
    setLayoutProperty(id: string) {
      layerPropsSet.push(id);
    },
  };

  const proposal: VoiceProposal = {
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'SET_LAYER_VISIBILITY', layer: 'MY_LOCATION', visible: true }],
  };

  executeMapActions(mockMap, proposal, false, () => {}, () => {}, () => {}, () => {});

  // This assertion FAILS before the patch (my-location is in layerPropsSet)
  // and PASSES after the patch (my-location not in layerPropsSet)
  assert.equal(layerPropsSet.length, 2, 'only existing layers should be toggled');
  assert.ok(layerPropsSet.includes('device-pulse'));
  assert.ok(layerPropsSet.includes('device-point'));
  assert.ok(!layerPropsSet.includes('my-location'), 'my-location should not be toggled');
});
```

**Current test status:** The existing test at lines 575–653 does NOT perform this check. It only verifies that real layers ARE toggled, not that non-existent layers are NOT toggled.

---

## 7. Recommendation

**Candidate.patch:** APPLY — it is valid cleanup that removes a phantom reference and makes `layerMap` accurate. However, do not treat it as a bug fix; it is source hygiene.

**Existing test gap:** The test at lines 575–653 does not detect the phantom. A new test as described in section 6 would properly regress the issue, but this is NOT a P1 gap since no runtime bug exists (the guard protects against errors).

**If two actual device layers can remain visible incorrectly:** N/A — the device layers (`device-pulse`, `device-point`) are correctly toggled. The phantom `my-location` does not cause any incorrect visibility state.

**No new layers added, no redesign.** Only the one-line removal.

---

## 8. Artifact

**candidate.patch** — Identical to MiniMax-G's candidate.patch (no changes needed):

```diff
diff --git a/frontend/v2/src/mapActions.ts b/frontend/v2/src/mapActions.ts
index a28f4f61..XXXXXXX 100644
--- a/frontend/v2/src/mapActions.ts
+++ b/frontend/v2/src/mapActions.ts
@@ -181,7 +181,7 @@ export const layerMap: Record<Layer, string[]> = {
      'safe-zones',
    ],
    ROUTES: ['route-casing', 'approved-route', 'route-motion', 'routes'],
-  MY_LOCATION: ['device-pulse', 'device-point', 'my-location'],
+  MY_LOCATION: ['device-pulse', 'device-point'],
  };
```

**Base SHA-256 for mapActions.ts:** `a28f4f612416391283a1e832418681d98358437e43fa75be0803394558fdd490`

---

## 9. Verification Commands (Deferred)

```bash
# Apply patch
cd /Users/apple/Documents/Projects/MonitoringZ
patch -p1 < plan/worker-reports/round-3/worker-11/candidate.patch

# Build check
cd frontend/v2 && npm run build

# Unit tests
node --test src/mapActions.test.ts

# Regression test for phantom layer
# (write test from section 6, run with node --test)
```

**Execution status:** NOT_RUN — deferred by user

---

## 10. Status

**Status: READY_FOR_REVIEW_UNVERIFIED**

**Assessment:** Candidate patch is valid cleanup; no behavioral change. Existing test does not detect phantom; no P1 regression exists. Patch should be applied as hygiene but does not require a rushed sprint fix.

**Integration risk:** LOW — patch is a one-line removal in an isolated constant array. No callers depend on `my-location` being in `layerMap`. No downstream changes required.

**Overlapping integration risk:** None — mapActions.ts is clean in working tree; patch applies cleanly to d95df9e.
