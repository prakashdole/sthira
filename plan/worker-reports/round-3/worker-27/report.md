# Worker 27 — Human Acceptance Checklist for Real Device Browser Verification
**Worker:** worker-27 | **Assignment:** round-3
**Timestamp:** 2026-09-27T19:05:00Z | **HEAD:** d95df9e
**Output dir:** `plan/worker-reports/round-3/worker-27/`

---

## 1. Scope and sources inspected

| Source | Relevance |
|--------|-----------|
| `plan/evidence/prototype-browser-verification.md` | Existing human-remaining items; Safari blocker note |
| `plan/worker-reports/round-2/minimax-h/report.md` | Synthetic test gaps and real-device checklist items (MediaRecorder, getUserMedia) |
| `plan/worker-reports/round-2/minimax-f/report.md` | Responsive CSS, 200% zoom gap, keyboard Tab order |
| `plan/worker-reports/round-2/minimax-j/report.md` | Pipeline launch runbook; intelligibility review is separate from UI switching |
| `plan/worker-reports/round-2/minimax-f/layout-check.spec.ts` | Automated Playwright script — this checklist does NOT duplicate it |

---

## 2. What this checklist covers vs. what automated scripts cover

The automated `layout-check.spec.ts` (minimax-f) covers:
- Horizontal overflow at 375/1024/1440 px viewport widths
- Modal scroll, bounded dialog, focus ring visibility
- Touch target height ≥ 44 px via bounding box
- Map disclaimer pointer-events
- `prefers-reduced-motion` media query

This checklist covers **only what automated scripts cannot verify**:

| Gap | Why automated fails |
|-----|---------------------|
| Real microphone permission | `getUserMedia` prompt can only be granted/denied by a human on the actual device |
| Audible playback quality | No audio quality metric in DOM; requires human ear |
| Replay refusal after language switch | Requires human to observe replay state after a live language-change action |
| Actual Safari viewport | Playwright viewport API does not exercise real iOS Safari rendering |
| Real browser zoom | Playwright viewport API resizes the window; browser zoom setting is a separate control |
| Keyboard Tab order on real device | Desktop browser Tab focus requires a physical keyboard; automated focus checks use `keyboard.press('Tab')` which does not test the real tab顺序 |
| Real hardware MediaRecorder errors | `MediaRecorder.onerror` (Finding 1 in minimax-h) requires physically disconnecting a mic mid-recording |

---

## 3. Checklist structure

Each row has five fields: **Setup**, **Action**, **Observable Success**, **Failure Record**, **Evidence**.

Sections:
- **§1 Microphone**: 1A granted, 1B denied, 1C device not found
- **§2 Playback/Replay**: 2A first audio plays, 2B replay refused after language switch, 2C replay within same language
- **§3 Intelligibility**: 3A Hindi, 3B Malayalam, 3C English — kept separate from language-switching tests
- **§4 Viewport**: 4A iPhone portrait, 4B iPhone landscape, 4C iPad
- **§5 Keyboard**: 5A Tab order, 5B Enter activates, 5C Escape closes modal
- **§6 Zoom**: 6A 100% baseline, 6B 200% real browser zoom, 6C 50% zoom

---

## 4. Relationship to prior reports

- **minimax-h** (MediaRecorder lifecycle) — Findings 1–3 identified gaps that this checklist's §1 (mic permission) and §2 (playback) exercise in a real-browser context. Finding 1 (`MediaRecorder.onerror` not wired) is tested implicitly by §1C (device removed mid-recording).
- **minimax-f** (CSS/responsive) — The 200% zoom gap (§3.6 in minimax-f) maps directly to §6B here. Tab order in minimax-f's Playwright script uses `keyboard.press('Tab')`; §5A tests real keyboard Tab order on actual hardware.
- **prototype-browser-verification.md** — Section "Remaining manual acceptance (owner, Safari or phone, about 15 minutes)" items map to: item 1 → §1A/1B here; item 2 → §2A/2B; item 3 → §4A/4C and §5A.
- **minimax-j** — intelligibility notes (`DRAFT_REQUIRES_NATIVE_REVIEW`) map to §3A/3B/3C here.

---

## 5. No source changes, no patch

No production source, test files, or planning documents were modified. This worker produced a checklist only.

---

## 6. Execution status

**NOT_RUN** — requires human operator with real hardware. No automated script can replace this checklist.

**Time estimate:** ~20–30 minutes for a single operator covering all rows.

**Specific blockers:**
- Safari remote automation requires enabling "Allow remote automation" in Safari Developer Settings — this is documented in `prototype-browser-verification.md` §Safari blocker and is a device setting, not an app change.
- §6B (200% browser zoom) requires actual browser zoom setting, not Playwright viewport resize.
- §1C (device not found) requires physically absent/disconnected mic or a fully revoked permission state.

---

## 7. Artifact

| Artifact | Path |
|----------|------|
| Human acceptance checklist | `plan/worker-reports/round-3/worker-27/checklist.md` |
| This report | `plan/worker-reports/round-3/worker-27/report.md` |

---

## 8. Status

**Status: NO_CHANGE_NEEDED**

No source or test changes needed. The checklist is the deliverable. It is designed to be executed by a human operator on real hardware after the usage reset.

**Next action for Opus:** Distribute `checklist.md` to the operator performing manual acceptance. No integration needed — the checklist is independent of any code change.

---

*Worker 27 — Human Acceptance Checklist — d95df9e — NOT_RUN*
