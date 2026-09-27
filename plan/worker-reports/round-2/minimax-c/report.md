# MiniMax C Report — Offline Eval Contract Review
**Timestamp:** 2026-09-27T00:00:00Z
**HEAD:** d95df9e (docs: record handoff commit id)
**Assignment scope:** backend/internal/middleworker/eval/{main.go,main_test.go,corpus.go,corpus/synthetic_v2.jsonl}, wire.go/decode_test.go, prior C handoff
**Status:** READY_FOR_REVIEW_UNVERIFIED

---

## 1. Source hashes and dirty files

| File | Object hash (SHA-256) |
|------|----------------------|
| backend/internal/middleworker/eval/main.go | `165f9f15cb9eb840115912ed9f450b4dd99a90a2` |
| backend/internal/middleworker/eval/main_test.go | `104537b683e28087e2cf0300fca1d987aa096620` |
| backend/internal/middleworker/eval/corpus.go | `7e8b937d48fbfdd6a1f3f4bbe131d6cacdd18c12` |
| backend/internal/middleworker/eval/corpus/synthetic_v2.jsonl | `16ad9c8655087e91d8a3ed3788f624dce8dc2d6f` |
| backend/internal/middleworker/wire.go | `d3397453f0643b1487e79fa757c80c7e706cd2dd` |
| backend/internal/middleworker/decode.go | `3ee4fe4683d04f1885ea9672d94ee88343841c19` |
| backend/internal/middleworker/decode_test.go | `ec32be3694121932dac96b1925f68f3a0cdafcb4` |

Dirty working-tree files (eval package only):
- `backend/internal/middleworker/eval/corpus.go` (+59 lines)
- `backend/internal/middleworker/eval/main.go` (+206 lines, −97)
- `backend/internal/middleworker/eval/main_test.go` (+425 lines, −97)
- `backend/internal/middleworker/eval/corpus/synthetic_v2.jsonl` (+26 lines, −26)

---

## 2. Existing vs. new contribution

**Already integrated (5d57c9b):** SET_LAYER_VISIBILITY `layer`+`*visible` added to `wire.go` Action struct; `decode_test.go` covers Show/Hide/MissingVisible/MarshalRoundTrip/ExtraFieldRejected. Layer and *Visible fields exist and round-trip. Do not redo.

**This review covers:** the new dirty layer on top of 5d57c9b — the eval driver, corpus-check oracle, semantic expectation check, negative-example tests, and corpus augmentation.

---

## 3. Findings

### 3.1 corpus.go — ExpectedContextIDs and validatePerCaseContext (CLEAN)
- `ExpectedContextIDs` field added to `CaseV2` struct at correct JSON field position.
- `validatePerCaseContext` is called from `runCorpusCheck` (main.go:390) after global fixture ref check.
- Empty `ExpectedContextIDs` (all non-semantic camera-only and abstention cases) correctly bypass the per-case check.
- Cases with non-empty context: SYN-019 (`["SZ-1","RZ-1","ROUTE-1"]`), SYN-021 (`["SZ-1"]`), SYN-022 (`["FACILITY-DEMO-1","FACILITY-DEMO-2"]`), SYN-024 (`["FACILITY-DEMO-1"]`), SYN-028 (`["PLACE-DEMO-1"]`). All actions/evidence IDs in each case are within that case's own context set.
- **Finding:** No defect. The per-case binding is correctly implemented.

### 3.2 SET_LAYER_VISIBILITY — visible=false vs. missing vs. true (CLEAN with one untested gap)

**Wire decoder (decode.go + wire.go):** `Layer string` and `Visible *bool` are in the `Action` struct. `json:"layer,omitempty"` and `json:"visible,omitempty"` allow the three states to round-trip distinctly:
- `visible:true` → JSON `"visible":true`, `*Visible=&true`
- `visible:false` → JSON `"visible":false`, `*Visible=&false`
- absent → JSON field omitted, `*Visible=nil`

`DisallowUnknownFields` in `decodeStrictProposal` rejects extra fields on SET_LAYER_VISIBILITY (covered by `TestDecodeStrictProposal_SetLayerVisibility_ExtraFieldRejected`).

**Corpus (synthetic_v2.jsonl):** SYN-020 carries `"visible":false` explicitly. `synthesizeProposal` produces the correct JSON with the field present. The corpus note accurately reflects the post-5d57c9b state.

**Semantic oracle (semanticExpectationCheck, main_test.go:553-566):** The three-state distinction is enforced:
```go
switch {
case want.Visible == nil && got.Visible != nil:  // missing vs. present → reject
case want.Visible != nil && got.Visible == nil:  // present vs. missing → reject
case want.Visible != nil && got.Visible != nil && *want.Visible != *got.Visible:  // false vs. true → reject
}
```

**Negative test (TestOracle_RejectsWrongVisibility):** Flips SYN-020's visible=false to true; correctly rejected. **Finding:** The flip path is tested and correct.

**Untested gap — model emits SET_LAYER_VISIBILITY with missing visible field:** The wire decoder accepts `{"type":"SET_LAYER_VISIBILITY","layer":"RED_ZONES"}` (visible=nil after decode). The semantic oracle would correctly reject this because `want.Visible!=nil && got.Visible==nil` at line 562. However, **no test exercises this specific path** (visible field silently dropped by the model vs. corpus expecting `visible:false`). This is a real failure mode. No patch required this round; noted for test completeness.

### 3.3 Negative/non-OK outcomes (CLEAN)
- SYN-023 (DATA_UNAVAILABLE, no actions): covered by `TestOracle_RejectsNonOKWithActions` (action added → reject) and `TestOracle_WireShapeAcceptsEveryCase` (correct proposal accepted).
- SYN-025 (DATA_UNAVAILABLE, route invention): correct status + nil intent, accepted by wire oracle.
- SYN-027 (UNSUPPORTED, weather): correct status + nil intent, accepted by wire oracle.
- Non-OK wire-shape: `TestOracle_RejectsNonOKWithActions` verifies semantic oracle enforces the "no actions on non-OK" rule.
- **Finding:** Correct. All non-OK paths have positive and negative coverage.

### 3.4 REVIEWED labels (CLEAN — no defect)
| Case | review_status | Basis |
|------|--------------|-------|
| SYN-017 | REVIEWED | English self-check baseline; no native gate needed |
| SYN-019 | REVIEWED | FIT_FEATURES on standard fixture set |
| SYN-023 | REVIEWED | DATA_UNAVAILABLE for unknown ID; abstention path |
| SYN-024 | REVIEWED | DESTINATION_PREVIEW on known facility |
| SYN-025 | REVIEWED | DATA_UNAVAILABLE for route invention |
| SYN-027 | REVIEWED | UNSUPPORTED for weather; no native speaker needed |

The remaining 8 cases carry DRAFT_REQUIRES_NATIVE_REVIEW (all non-English). `semanticExpectationCheck` does not assert `ReviewStatus` — this is intentional; review_status is an external-gate metadata field, not a semantic property of the proposal. **No defect.**

### 3.5 corpus.go — synthesizeProposal visible=false handling (CLEAN)
For `ExpectedActions` with `Visible=&false`, `json.Marshal` produces `"visible":false` in the JSON. Wire decoder reads it back as `*Visible=&false`. Semantic oracle compares `*want.Visible` vs `*got.Visible` with `!=`. Correct.

### 3.6 Redundant Layer field check in semanticExpectationCheck (MINOR — no incorrect behavior)
`main_test.go:589`: `if got.Layer != want.Layer` is executed in the general action loop **after** the same check already executed at line 555 inside the SET_LAYER_VISIBILITY block. For any non-SET_LAYER_VISIBILITY action type, `got.Layer` is always empty string (no field emits it), and `want.Layer` is also empty, so the check is always a no-op for those types. For SET_LAYER_VISIBILITY, it is redundant with line 555. **No incorrect behavior** — the redundant check is dead code but produces the same result. No corrective patch warranted.

### 3.7 wantV2=13 hardcoded count (OBSERVATION — not a defect)
`main_test.go:112`: `const wantV2 = 13`. If a case is silently removed from synthetic_v2.jsonl, `TestCorpusV2_LoadsAndCounts` fails with a count mismatch rather than silently skipping. This is correct defensive behavior. The constant is accurate for the current corpus.

### 3.8 Benchmark UNIMPLEMENTED (DOCUMENTED CORRECTLY)
`runBenchmark` unconditionally exits 2. The `BenchmarkCapabilitySpec` block in `main.go` is thorough, correctly scoped, and documents the exact contract Opus must satisfy. Hardware/artifacts prerequisites are documented in `HARDWARE_BLOCKER.md`. **Benchmark remains UNIMPLEMENTED as documented — not a defect.**

### 3.9 corpus-check mode integration (CLEAN)
`runCorpusCheck` calls `loadCorpusJSONL` → `uniqueCaseIDs` → `validateFixtureRefs` → `validatePerCaseContext`. Exit codes: 0 on success, 1 on validation error, 2 on usage error. Consistent with the documented contract. No network, no model. Correct.

---

## 4. Candidate patch

No corrective patch required. The dirty delta is internally consistent and correct. All findings are observations or untested gaps, not defects.

---

## 5. Verification

**Source inspection performed:**
- `decodeStrictProposal` (decode.go:29): uses `DisallowUnknownFields`, preserves `*bool` for Visible
- `wire.go` Action struct: has `Layer string` and `Visible *bool` with `omitempty`
- `semanticExpectationCheck` (main_test.go:526): three-state visibility check with explicit nil/!nil/distinct-value branches
- `TestOracle_RejectsWrongVisibility` (main_test.go:339): negative example covering visible=false→true
- `validatePerCaseContext` (corpus.go:202): empty-context skip, per-ID membership check
- `runCorpusCheck` (main.go:372): calls validatePerCaseContext at line 390
- `synthesizeProposal` (corpus.go:247): produces `"visible":false` JSON for `Visible=&false`
- `TestOracle_WireShapeAcceptsEveryCase` (main_test.go:226): covers SYN-020 round-trip

**Execution tests:** NOT_RUN — deferred by user.

**Required later commands (for Opus):**
```bash
# Verify eval package builds
cd backend/internal/middleworker/eval && go build .

# Run offline eval checks (no network, no model)
middleworker-eval -mode harness-check
middleworker-eval -mode corpus-check -corpus eval/corpus/synthetic_v2.jsonl
middleworker-eval -mode manifest

# Run Go tests (unit-only, no real inference)
cd backend/internal/middleworker/eval && go test -v ./...

# Run wire decoder tests (covers SET_LAYER_VISIBILITY)
cd backend/internal/middleworker && go test -v -run "DecodeStrictProposal_SetLayerVisibility" ./...

# Negative control: benchmark mode must exit 2
middleworker-eval -mode benchmark -endpoint http://localhost:8000 -corpus eval/corpus/synthetic_v2.jsonl
# Expected: exit 2 with "UNIMPLEMENTED" message
```

**Expected negative control for new regression logic:** When the model emits `{"type":"SET_LAYER_VISIBILITY","layer":"RED_ZONES"}` (no visible field) against SYN-020's expectation of `visible:false`, the semantic oracle must reject with `errSemantic("action 0 visible=nil want false")`. The current `TestOracle_RejectsWrongVisibility` tests the `true` flip, not the missing-field path; adding a dedicated test for missing-visible is recommended as follow-up.

---

## 6. Status

**READ_FOR_REVIEW_UNVERIFIED**

No blocking defects found. The dirty layer is consistent with the 5d57c9b SET_LAYER_VISIBILITY integration. The only substantive gap is an untested negative path (model omits visible field vs. corpus expects explicit false). Benchmark remains correctly documented as UNIMPLEMENTED.

**Exact next action for Opus:** Confirm no corrective patch needed; apply the dirty delta as-is after reviewing this report. The untested visible-omission path (Finding 3.2) can be addressed separately as a test-completeness follow-up; it does not block integration.
