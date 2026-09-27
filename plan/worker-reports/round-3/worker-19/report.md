# Worker 19 Report — Visibility Oracle Omitted Field Regression

**Worker:** worker-19 | **Assignment:** round-3
**Timestamp:** 2026-09-27 | **HEAD:** d95df9e
**Output dir:** plan/worker-reports/round-3/worker-19/

---

## 1. HEAD, Source Hashes, Dirty Files

| File | SHA-256 (current working tree) |
|------|-------------------------------|
| `backend/internal/middleworker/eval/main.go` | `c41dec7aa68c3bfca8676962ff29ae9ff28cdca1674d8f0052867febe3c4971d` |
| `backend/internal/middleworker/eval/main_test.go` | `1b688e2803d80222304cbddcc14114040872d3b3c01f50aff81f1bf12d56a673` |
| `backend/internal/middleworker/wire.go` | `954b4bf27152e5e41cc5d3f8de9871a8ffde54767bc9dd204663dad6d4082137` |

**Dirty files (relevant scope):**
- `backend/internal/middleworker/eval/main.go` — dirty working tree (unrelated to this assignment)
- `backend/internal/middleworker/eval/main_test.go` — dirty working tree (dirty delta present since minimax-c)

---

## 2. Prior Minimax-C Report Assessment

minimax-c identified an **untested gap** (Finding 3.2):

> "Untested gap — model emits SET_LAYER_VISIBILITY with missing visible field: The wire decoder accepts `{"type":"SET_LAYER_VISIBILITY","layer":"RED_ZONES"}` (visible=nil after decode). The semantic oracle would correctly reject this because `want.Visible!=nil && got.Visible==nil` at line 562. However, **no test exercises this specific path**."

The C report's recommendation: "no corrective patch needed — the semantic oracle logic is correct; noted for test completeness."

**Verification against current source:** The C report is correct. The semantic oracle at `main_test.go:559-566` handles all three states:
```go
switch {
case want.Visible == nil && got.Visible != nil:   // missing vs present → reject
case want.Visible != nil && got.Visible == nil:   // present vs missing → reject ← COVERS THIS
case want.Visible != nil && got.Visible != nil && *want.Visible != *got.Visible:  // false vs true → reject
}
```

The gap is confirmed: no test exercises the `want.Visible!=nil && got.Visible==nil` branch with a real oracle call and a SYN-020-derived fixture.

---

## 3. Scope Inspected

| File | Relevant Section | Finding |
|------|-----------------|---------|
| `wire.go` | Action struct line 145-155 | `Visible *bool` with `omitempty` — three states preserved |
| `eval/main_test.go` | `semanticExpectationCheck` lines 526-599 | Visibility switch at lines 559-566 handles all three states |
| `eval/main_test.go` | `TestOracle_RejectsWrongVisibility` lines 339-352 | Tests false→true flip, not omitted path |
| `eval/main_test.go` | `wrongProposal` + `mustLoadCase` helpers | Used to build test mutations |
| `eval/corpus.go` | `synthesizeProposal` + `ActionSpec` | `Visible *bool json:"visible,omitempty"` marshals to/from JSON correctly |
| `eval/corpus/synthetic_v2.jsonl` | SYN-020 | `visible:false` baseline; confirmed present in current working tree |

---

## 4. Existing Coverage vs. New Contribution

| Test | Status |
|------|--------|
| `TestOracle_WireShapeAcceptsEveryCase` (round-trips all corpus cases) | Existing |
| `TestOracle_RejectsWrongVisibility` (visible=false → true flip) | Existing — tests one wrong branch |
| `semanticExpectationCheck` switch cases (lines 559-566) | Logic present, nil-vs-false branch not exercised by any named test |
| `TestOracle_RejectsOmittedVisibility` | **NEW** — tests nil (model) vs &false (corpus) |
| `TestOracle_RejectsOmittedVisibilityVsTrue` | **NEW** — tests nil (model) vs &true (corpus) |
| `TestOracle_AcceptsExplicitFalse` (positive control) | **NEW** — confirms &false is not confused with nil |

---

## 5. Regression Test Artifact

**File:** `plan/worker-reports/round-3/worker-19/test_visibility_omitted.go`

### What it tests

Three tests use the real `semanticExpectationCheck` oracle and `synthesizeProposal` helpers from the `main` package:

1. **`TestOracle_RejectsOmittedVisibility`** — loads SYN-020 (`visible:false` expected), synthesizes the correct proposal, then sets `Visible=nil` to simulate a model emitting `{"type":"SET_LAYER_VISIBILITY","layer":"RED_ZONES"}` (no visible field). Asserts the oracle rejects with an error mentioning "visible".

2. **`TestOracle_RejectsOmittedVisibilityVsTrue`** — synthetic case expecting `visible:true`, mutated to `Visible=nil`. Same oracle rejection.

3. **`TestOracle_AcceptsExplicitFalse`** — positive control: corpus and proposal both have `visible:false`. Asserts the oracle **accepts** exact match (not confused with nil).

### Why this is NOT a duplicate of `TestOracle_RejectsWrongVisibility`

`TestOracle_RejectsWrongVisibility` flips `visible=false` → `visible=true` (explicit-to-explicit, wrong value). The switch branch is:
```go
case want.Visible != nil && got.Visible != nil && *want.Visible != *got.Visible
```
This tests `*want.Visible != *got.Visible` but NOT the nil-vs-non-nil distinction.

The new tests set `got.Visible = nil` (pointer is nil, not pointing to a bool), exercising the branch:
```go
case want.Visible != nil && got.Visible == nil
```
These are genuinely different code paths — one compares bool values, the other checks pointer nil.

### Failure mode detected

Before the fix: N/A — the semantic oracle logic was already correct; only the test was missing.

After the fix: the regression proves the oracle correctly rejects model output that omits the visibility field when the corpus expects it to be explicitly set.

---

## 6. Integration Risks

- **No patch** — only a test addition. The semantic oracle logic is already correct.
- **File placement:** The test file must be placed in `backend/internal/middleworker/eval/` as `test_visibility_omitted_test.go` and compiled as part of the `main` package.
- **Dependencies:** `semanticExpectationCheck`, `synthesizeProposal`, `loadCorpusJSONL`, `mustLoadCase` — all internal helpers in `main_test.go`. External package tests in `package main` can access them.
- **Corpus path:** The test uses `os.Getwd()` + relative path to find `synthetic_v2.jsonl`. Must be run from `backend/internal/middleworker/eval/` or the working directory when run via `go test`.

---

## 7. Verification Commands

**Source inspection:** Performed (main_test.go, wire.go, corpus.go, synthetic_v2.jsonl)
**Execution tests:** NOT_RUN — deferred by user

```bash
# Apply test to eval package (move file)
cp plan/worker-reports/round-3/worker-19/test_visibility_omitted.go \
   backend/internal/middleworker/eval/test_visibility_omitted_test.go

# Run just the new tests
cd backend/internal/middleworker/eval
go test -v -run "TestOracle_RejectsOmittedVisibility|TestOracle_RejectsOmittedVisibilityVsTrue|TestOracle_AcceptsExplicitFalse" .

# Run all eval tests (confirms no regression)
cd backend/internal/middleworker/eval
go test -v ./...

# Build check
cd backend/internal/middleworker/eval && go build .
```

**Expected output (after placement):**
```
TestOracle_RejectsOmittedVisibility          PASS
TestOracle_RejectsOmittedVisibilityVsTrue    PASS
TestOracle_AcceptsExplicitFalse               PASS
```

**Expected failure before the test existed:** N/A — the test is new, not testing a fix. The existing `TestOracle_RejectsWrongVisibility` already PASSES, confirming the `*Visible != *Visible` path works. The new tests add coverage for the `nil` pointer path.

---

## 8. Status

**STATUS: READY_FOR_REVIEW_UNVERIFIED**

**Next action for Opus:**
1. Move `plan/worker-reports/round-3/worker-19/test_visibility_omitted.go` to `backend/internal/middleworker/eval/test_visibility_omitted_test.go`
2. Run `go test -v -run "TestOracle_RejectsOmittedVisibility|TestOracle_RejectsOmittedVisibilityVsTrue|TestOracle_AcceptsExplicitFalse" .` from `backend/internal/middleworker/eval/`
3. Confirm all three pass
4. Commit as part of the eval test suite (no production code change)
