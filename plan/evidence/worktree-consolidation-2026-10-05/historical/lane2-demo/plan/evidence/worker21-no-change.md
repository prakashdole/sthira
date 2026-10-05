# Worker 21 — Model Evaluation Corpus: NO_CHANGE

## Item 4 evidence

### What the blocker asked

The original blocker asked whether the eval package conflates skip/ERROR with success, and whether a missing skip counter means skipped cases can inflate the success tally.

### Finding: NO skip counter exists

The eval package (`backend/internal/middleworker/eval/`) has **no counting variables** for pass, fail, skip, error, or synthesized outcomes. The code uses `t.Errorf()` for all failure reporting, not atomic counter increments.

### File-by-file counting points

| File | Lines | Mechanism | Purpose |
|------|-------|-----------|---------|
| `main.go` | — | None | Contains `runBenchmark`, `runCorpus`, `runHarness` — no counter variables |
| `corpus.go` | — | None | `loadCorpusJSONL`, `synthesizeProposal`, `uniqueCaseIDs`, `validateFixtureRefs`, `validatePerCaseContext` — no counters |
| `main_test.go` | All tests | `t.Errorf()` | All 21 test functions use `t.Errorf()` for failures, not counter increments |

**Conclusion:** There is no `passed`, `failed`, `skipped`, `error`, or `synthesized` counter variable anywhere in the eval package. The blocker question ("skip counter not implemented") is moot — there is nothing to implement, because counting is not the mechanism.

### Non-OK cases are tested, not tallied

The v2 corpus (13 cases) contains three non-OK cases:

| Case | Expected status | Oracle path |
|------|----------------|-------------|
| SYN-023 | `DATA_UNAVAILABLE` | Semantic oracle (negative example + wire-shape) |
| SYN-025 | `DATA_UNAVAILABLE` | Semantic oracle |
| SYN-027 | `UNSUPPORTED` | Semantic oracle |

These are exercised by:
- `TestOracle_WireShapeAcceptsEveryCase` — verifies non-OK proposals decode cleanly
- `TestOracle_SemanticExpectationsAcceptCorrectProposal` — verifies oracle accepts the case's own expected output (including non-OK)
- `TestOracle_RejectsNonOKWithActions` — negative example: DATA_UNAVAILABLE with a stray action must be rejected
- `TestOracle_RejectsWrongStatus` — negative example: OK→CLARIFY must be rejected

No counter is incremented for these; they are tested via `t.Errorf()`, which causes test failures if violated.

### All 21 tests pass

```
ok      sthira/backend/internal/middleworker/eval   0.152s
```

```
TestEval_HarnessCheckPrintsBlocker                    PASS
TestEval_ManifestModeWritesFile                       PASS
TestEval_BenchmarkModeIsUnimplemented                 PASS
TestCorpusV2_LoadsAndCounts                          PASS
TestCorpusV2_UniqueIDsAndNoCrossFileDuplicates       PASS
TestCorpusV2_NoInventedFixtureIDs                    PASS
TestCorpusV2_PerCaseContext                          PASS
TestCorpusV2_SyntheticProposalMatchesShapeContract    PASS
TestOracle_WireShapeAcceptsEveryCase                  PASS
TestOracle_SemanticExpectationsAcceptCorrectProposal  PASS (13 sub-tests: SYN-016..SYN-028)
TestOracle_RejectsWrongStatus                        PASS
TestOracle_RejectsForbiddenTarget                     PASS
TestOracle_RejectsWrongVisibility                     PASS
TestOracle_RejectsUnauthorizedExtraAction            PASS
TestOracle_RejectsNonOKWithActions                   PASS
TestOracle_RejectsMalformedJSON                      PASS
TestOracle_RejectsExtraText                          PASS
TestOracle_RejectsUnknownField                       PASS
TestOracle_RejectsRequestIDMismatch                  PASS
TestOracle_RejectsOmittedVisibility                  PASS
TestCorpusContext_RejectsGloballyKnownIDOutsideCase   PASS
```

Exit code: 0. All assertions hold.

### Verdict: NO_CHANGE

Worker 21 requires no changes. The eval package correctly tests corpus cases without a tally mechanism. The non-OK cases are exercised through the semantic oracle and wire-shape tests. There is no counter that could double-count or misattribute skip/ERROR outcomes.
