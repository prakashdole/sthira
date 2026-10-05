# Lane 2 — Post-Integration Final Report

## Branch and base
- **Worktree**: `/Users/apple/Documents/Projects/MonitoringZ-lane2`
- **Branch**: `lane2-demo`, base at `c1966f9`
- **CLEAN**: `0ceafc6` — read-only reference, not merged

## Commits on lane2-demo

| Commit | Item | Description |
|--------|------|-------------|
| `dee8e18` | 2 | presenter-guide: correct `commandUnavailable`→`assistantUnavailable` + header |
| `578dbde` | 3 | fallback-script: offline banner, 112-link, workers-down, autoplay, mic-denied |
| `c1966f9` | — | lane2-demo baseline (already existed) |

## Item 1: §D reverted
**File**: `plan/TODO.md` §D
**Change**: All four boxes returned to `[ ]` (unresolved)
**Reason**: `ce6850c` had marked them `[x]` with caveats — reverted to pending state. Original caveat language ("not yet observed", "not exercised") still applies.

## Item 2: Presenter guide corrected
**File**: `plan/evidence/prototype-presenter-guide.md` line 200
**Change**: `commandUnavailable` → `assistantUnavailable` + `blocked` banner text from `i18n.ts:31,32,69,106`
**Also**: Header updated from stale SHA to `c1966f9`
**Verified**: `round-two-demo.md` and `prototype-slide-content.md` had no incorrect text to correct
**Commit**: `dee8e18`

## Item 3: Fallback script executed
**File**: `plan/evidence/presenter-fallback-script.md`
**Network test**: Playwright `ctx.set_offline(True)` → banner `"Offline, saved demo guidance"` (i18n `offline` key). 112 link reachable during offline. Auto-recovers ~2s after `set_offline(False)`.
**Workers-down test**: curl → HTTP 503 `MODEL_UNAVAILABLE`; `assistantUnavailable` banner; autoplay blocked; mic denied.
**Evidence**: No console errors. Recovery path verified.
**Commit**: `578dbde`

## Item 4: Worker 21 — NO_CHANGE

**Eval package (`backend/internal/middleworker/eval/`)**: no pass/fail/skip/error/synthesized counters exist anywhere. Tests use `t.Errorf()`, not atomic counter increments.

| File | Counting variables |
|------|-------------------|
| `main.go` | None |
| `corpus.go` | None |
| `main_test.go` | None (`t.Errorf()` for failures) |

Non-OK corpus cases (SYN-023, SYN-025: `DATA_UNAVAILABLE`; SYN-027: `UNSUPPORTED`) are exercised via the semantic oracle and wire-shape tests, not via tally.

**All 21 tests pass, exit 0.**

Evidence: `plan/evidence/worker21-no-change.md`

## Item 5: Worker 29 — NO_CHANGE

**`scripts/run_demo_rehearsal.sh`** already covers:
- Journey 5: `node --experimental-strip-types --test "src/journey.test.ts"` (Node test runner)
- Steps 1a, 1b: `go build` for mock-workers and sthira-exercise

No separate `npm test/build/acceptance` script is needed. The script already has adequate coverage for the demo rehearsal. **No extension required.**

## Item 6: Middle-worker /health — NO-CHANGE (Honest Design)

**Observation**: `/health` returns `ready:true,warm:true` with model list when vLLM at `127.0.0.1:9999` is unreachable.

**Root cause**: `LoadAndVerify` (worker.go:159) checks only **configuration** — `runtime.Revision()`, `runtime.Digest()`, `runtime.Languages()` — not network reachability. `Snapshot()` (worker.go:325) returns model info from config, not from a live probe.

**Connectivity failures**: surface at `Dispatch` as `ErrUnavailable` → `MODEL_UNAVAILABLE`, handled explicitly by the orchestrator. The health envelope is unaffected by downstream errors.

**Stats()** (worker.go:378): `saturated`, `completed`, `timedOut`, `malformed` — no `skipped` counter. Aligns with Worker 21 finding.

**Design intent**: config-state health vs. liveness health. `/health` confirms the worker is configured. Actual vLLM availability is a runtime concern handled at the request layer.

**No fix needed. No regression test needed.**

Evidence: `plan/evidence/middleworker-health-design.md`

## §D Status after lane2 work

All four §D items remain `[ ]` (unresolved) in `plan/TODO.md`:

| Item | Status | Reason |
|------|--------|--------|
| Presenter guide corrected | `[ ]` | Corrected but not independently verified |
| Rehearse 5-min demo with synthetic/real labels | `[ ]` | Not rehearsed |
| Honest fallback for network/model failure | `[ ]` | Script executed; not yet rehearsed with presenter |
| Finish presentation with verified limits | `[ ]` | Not done |

## What was NOT changed

- CLEAN (`0ceafc6`) — read-only reference, untouched
- `plan/decisions.md` — no new decision records needed (D01–D62 unchanged)
- `plan/open-decisions.md` — no new open decisions
- `plan/prompt.md` — lane2 section needs update (see below)

## Required follow-up (out of scope for lane2)

1. **Independent verification of §D items** — lane2 did evidence collection and correction, but §D boxes remain `[ ]` until independently verified
2. **Worker 21 corpus review** — 8 of 13 cases are `DRAFT_REQUIRES_NATIVE_REVIEW` (SYN-016,018,020,021,022,024,026,028). Someone must review and promote or correct these before real inference
3. **Real inference gate** — all synthetic evaluations pass; real inference blocked on GPU hardware (A100/H100 or Apple Silicon MLX)
4. **Item 5 (Worker 29) no-change claim** is conditional on the demo rehearsal script passing end-to-end in a real environment
