# MiniMax B — Reusable Browser Acceptance Harness
**Report**: `plan/worker-reports/round-2/minimax-b/report.md`
**Worker**: MiniMax B
**Timestamp**: 2026-09-27T00:00:00Z
**HEAD**: `d95df9e609454877a91b7c82146b3c6368181b17`
**Assignment**: Round 2, MiniMax B

---

## 1. Scope Inspected

| Area | Source | Evidence |
|------|--------|----------|
| Frontend selectors | `frontend/v2/src/main.ts:1380–1600` | All `data-action`, `data-language`, `data-onboarding-*` attributes confirmed in HTML template literals |
| Prototype verification record | `plan/evidence/prototype-browser-verification.md` | 9/9 PASS at `e066056`; duplicate-start and arrival checks defined there |
| Existing accept.py | `/tmp/w3keep/accept.py` | Baseline: hooks check, onboarding, language switch, duplicate-start, GPS arrival, 390px overflow |
| Existing outage.py | `/tmp/w3keep/outage.py` | Kill/mock restart cycle — not in scope for this harness |
| Existing browser_verify_v3.py | `/tmp/browser_verify_v3.py` | Broader HTTP + browser suite at `6861d42`; overlays used as reference only |

**Dirty files** (unrelated to this work):
- `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/ttsworker/audio_mime_test.go` — audio MIME tests
- `backend/internal/middleworker/eval/` — eval corpus and main_test changes
- `frontend/v2/src/styles.css` — style changes

---

## 2. Deliverable

| Artifact | Path |
|----------|------|
| Reusable Playwright harness | `plan/worker-reports/round-2/minimax-b/browser_accept.py` |
| This report | `plan/worker-reports/round-2/minimax-b/report.md` |

---

## 3. Findings

### 3.1 Confirmed Selectors (from `main.ts`)

All selectors used in the harness are verified against `main.ts` source-of-truth:

| Selector | Element | Line |
|----------|---------|------|
| `[data-action="onboarding-start"]` | Begin voice onboarding button | 1391 |
| `[data-onboarding-language="EN"]` | Language option EN | 1393 |
| `[data-action="onboarding-language-next"]` | Language step continue | 1395 |
| `[data-action="onboarding-complete"]` | Location step complete / start guidance | 1400–1401 |
| `[data-language="EN"]` / `HI` | Runtime language switcher | 1450 |
| `[data-action="voice-open"]` | Voice console launcher (header + mobile dock) | 1449, 1551 |
| `[data-action="voice-close"]` | Voice console close | 1555 |
| `#command-input` | Text input in voice console form | 1560 |
| `[data-command-form] button[type="submit"]` | Submit button for text command | 1560 |
| `[data-action="route"]` | Start Route primary action | 1531 |
| `[data-action="directions-close"]` | Directions sheet close | 1564 |
| `[data-action="start-tracking"]` | GPS tracking start button | 446 |
| `[data-action="arrival-open"]` | Near-destination arrival trigger | 1468 |
| `[data-action="arrival-yes"]` | Confirm arrival button | 1567 |
| `.voice-console` | Voice console panel | 1554 |

### 3.2 Duplicate-Start Protection Logic

The existing `accept.py` uses a mutable `posts` list captured via `page.on("request")` for POSTs to `/api/v3/reservations`. This works but mixes request capture (network) with the in-process list (mutation).

**Improvement in new harness**: Capture via `page.on("response")` for the same URL pattern, which separates network status (response arrived) from UI state. The `posts_to_reservations` list is append-only after both clicks, and the comparison is `new_posts <= 1` — the `<= 1` accounts for the possibility that the first click itself generates 0 or 1 POST (some flows are HTTP-driven before the UI updates).

The `accept.py` baseline at `e066056` confirmed this passes: 1 POST, stay pinned to `FACDEMO-1`.

### 3.3 GPS / Arrival Sequence

The `trackingButtonHtml` function (`main.ts:439`) shows three journey states:
- `'TRACKING'` or `'NEAR_DESTINATION'` → show `stop-tracking`
- `'ARRIVAL_REPORTED'` or `'ROUTE_REVOKED'` → show nothing
- otherwise → show `start-tracking`

With Playwright's `geolocation=SHELTER` context option, `navigator.geolocation.getCurrentPosition()` inside `startTracking()` (`main.ts:958`) will use the emulated coordinates. The app's proximity evaluator (`applyPositionUpdate` → `evaluateProximity` → state transition to `NEAR_DESTINATION`) then triggers the arrival UI.

The `confirmArrival()` function (`main.ts:901`) POSTs to `/api/v3/reservations/{activeStayId}/events` with `{type: 'ARRIVE'}`. On `res.ok`, it sets `journeyState = 'ARRIVAL_REPORTED'` and shows `"Recorded at"`.

**Key uncertainty**: The 20-second timeout for `arrival-open` appearance is an empirical bound. In the demo environment with mock-workers at `destination-choice` scenario and shelter coords, the GPS → near transition should be near-immediate. A longer timeout may be needed if the proximity evaluation has a polling interval.

### 3.4 Excluded Scenarios

| Excluded | Owner |
|----------|-------|
| Outage detection + recovery | E/F |
| Layout / horizontal overflow at 375/390/1024/1440 px | G |
| Audio playback, replay after language switch | H |
| Camera / MapLibre layer toggles (red zones, relocation zones) | L |
| Delayed-language race (switch HI while EN request pending) | C |

---

## 4. Proposed Patch

**Base hashes** (for reference; this is a new standalone script, not a source patch):

```
frontend/v2/src/main.ts:d95df9e
plan/evidence/prototype-browser-verification.md:e066056
/tmp/w3keep/accept.py:e066056 baseline
```

No source file is modified. The harness is a new artifact.

If Opus needs a source delta to wire this into the repo (e.g., as `acceptance/browser_accept.py`), the only change would be copying `browser_accept.py` into the repo with a minimal `pyproject.toml` or `requirements-dev.txt` entry for `playwright` and `httpx`.

---

## 5. Verification Plan

**Execution tests NOT_RUN — deferred by user (usage reset).**

### Later Commands

```bash
# 1. Install dependencies (one-time)
pip install playwright httpx
playwright install chromium  # or: npx playwright install chromium

# 2. Stack must be running (Vite dev server + sthira-exercise + mock-workers)
#    On the demo shelter (SHELTER = {longitude: 76.105, latitude: 11.570})
#    Expected: FACDEMO-1 is the destination, ~2 km from shelter coords

# 3. Run against default (1280×860, 127.0.0.1:18492)
python3 browser_accept.py

# 4. Run against mobile viewport
python3 browser_accept.py http://127.0.0.1:18492/ --viewport=390,844

# 5. Run against Vite dev port
python3 browser_accept.py http://127.0.0.1:5173/
```

### Expected Outcomes

| Check | Expected |
|-------|----------|
| window hooks absent without opt-in | PASS |
| onboarding completes to guidance card | PASS |
| guidance destination rendered from backend | PASS |
| text command submitted and processed | PASS |
| route start creates stay in session storage | PASS (stay_id non-null, fac=FACDEMO-1) |
| repeated Start Route creates no second reservation | PASS (new_posts ≤ 1, stay unchanged) |
| arrival recorded after server acknowledgement | PASS ("Recorded at" visible) |
| no uncaught page errors | PASS |
| API reservation+arrival spot-check | PASS (httpx) |

### Negative Controls

| Scenario | Expected |
|----------|----------|
| `?sthira-test-hooks=1` NOT set, `window.setActiveStayId` defined | FAIL (correct — hooks must be absent without opt-in) |
| Clicking route twice without server running | FAIL with HTTP 0 or 503 |
| Arrival without prior reservation (fresh session) | FAIL with "No verified stay reservation found" error text |

---

## 6. Stack Setup Recipe

```bash
# --- One-time setup ---
brew install node go playwright  # macOS; Linux: nix/apt as appropriate
cd /Users/apple/Documents/Projects/MonitoringZ
npm install --prefix frontend/v2
go install ./cmd/sthira-exercise
playwright install chromium

# --- Start backend services (one terminal) ---
# PostgreSQL must be running; use the disposable DB pattern from integration tests
cd backend
go run ./cmd/sthira-exercise -db-url "$DATABASE_URL" &
STHIRA_PID=$!

# mock-workers in destination-choice scenario
go run ./cmd/mock-workers -scenario destination-choice -port 50616 &
MOCK_PID=$!

# --- Start frontend (second terminal) ---
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run dev -- --port 18492 &
FRONTEND_PID=$!

# --- Run harness ---
python3 /Users/apple/Documents/Projects/MonitoringZ/plan/worker-reports/round-2/minimax-b/browser_accept.py http://127.0.0.1:18492/

# --- Tear down ---
kill $STHIRA_PID $MOCK_PID $FRONTEND_PID
```

---

## 7. Status

**READINESS: READY_FOR_REVIEW_UNVERIFIED**

All checks are grounded in verified source selectors and the existing `accept.py` baseline that passed at `e066056`. The new harness adds: separate network/UI concern via response capture, CLI viewport override, optional httpx API spot-check, machine-readable failure lines, and cleaner modular functions for reuse.

**Next action for Opus**: Review `browser_accept.py` for correctness and integration placement. No source changes required unless repo-wiring is desired.
