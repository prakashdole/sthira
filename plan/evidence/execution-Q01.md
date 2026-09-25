# Task Q01 Evidence — Regional Scenarios, Languages and Participant-Ready Drill Inputs

**Task**: Q01 — Regional scenarios, languages and participant-ready drill inputs  
**Owner lane**: `plan/drills/`, `backend/internal/catalogue/`  
**Base commit**: `0c96ac1` (CLEAN)  
**Host Environment**: macOS (Darwin arm64, Apple M2), Go 1.27.1.

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation & Verification Result | Status |
| --- | --- | --- |
| **10–15 State Regional Catalogue** | Created canonical `plan/drills/catalogue.json` conforming to `catalogue.Manifest`. Covers 10 disaster-vulnerable states (Kerala KL, Himachal Pradesh HP, Uttarakhand UK, Assam AS, Maharashtra MH, Tamil Nadu TN, Karnataka KA, Odisha OD, Sikkim SK, West Bengal WB) with 20 sourced scenarios. | **PASS** |
| **Official Disaster Event Provenance** | Included 21 historical flood and landslide disaster events with official citations (`gov:cwc:...`, `gov:gsi:bhusanket-...`, `gov:nidm:...`, `gov:imd:...`). Separates real historical disaster evidence from synthetic present-day exercise geometry/timestamps (`SYNTHETIC_EXERCISE`). | **PASS** |
| **Strict Catalogue Validation** | Added `backend/internal/catalogue/regional_test.go` executing `Validate()` and `AssessLaunch()`. All 18 tests in `internal/catalogue` pass cleanly: 0 validation errors, 0 gaps, exactly 10 states (`MinLaunchStates=10`), >=2 scenarios per state. | **PASS** |
| **Language & Qualified Reviewer Matrix** | Documented in `plan/drills/language-matrix.md`. Clarifies that only `ml-IN` and `hi-IN` are currently enabled for ASR/TTS (backed by P6 synthetic benchmarks); remaining 8 regional languages are marked PLANNED pending O03/O11. Strictly enforces that English text is NEVER mislabeled as Sign Language; ISL requires verified video media. | **PASS** |
| **Mandatory Facilitator Stop Rule** | Documented verbatim safety briefing in `plan/drills/facilitator-protocol.md`. Enforces immediate termination triggers if participants show distress or confusion. Forbids unauthorized outreach, cold-calling emergency dispatchers (112), or recruiting minors without guardian/IRB consent. | **PASS** |
| **7 Participant Drill Task Scripts** | Documented structured prompts and pass criteria in `plan/drills/task-scripts.md` covering: (1) immediate stay, (2) 7–30 day temporary stay, (3) ambiguous place name resolution, (4) dynamic route revocation & transfer, (5) offline cold start, (6) caregiver party size booking, (7) explicit arrival confirmation. | **PASS** |
| **Parallel Agent Isolation** | Staged only `plan/drills/`, `backend/internal/catalogue/regional_test.go`, and `plan/evidence/execution-Q01.md`. Zero modifications to other agent's active files or `.txt` files. | **PASS** |

---

## 2. Test Execution Details

### A. Catalogue Manifest Validation Test
```
$ cd backend && go test -v ./internal/catalogue/...
=== RUN   TestValidateValidManifest
--- PASS: TestValidateValidManifest (0.00s)
=== RUN   TestValidateEmptyManifestIsGapNotError
--- PASS: TestValidateEmptyManifestIsGapNotError (0.00s)
=== RUN   TestValidateHistoricalScenarioRequiresEventRef
--- PASS: TestValidateHistoricalScenarioRequiresEventRef (0.00s)
=== RUN   TestValidateHistoricalScenarioRejectsUnknownEvent
--- PASS: TestValidateHistoricalScenarioRejectsUnknownEvent (0.00s)
=== RUN   TestValidateSyntheticMustNotReferenceEvent
--- PASS: TestValidateSyntheticMustNotReferenceEvent (0.00s)
=== RUN   TestValidateScenarioStateMustMatchOwner
--- PASS: TestValidateScenarioStateMustMatchOwner (0.00s)
=== RUN   TestValidateDuplicateStateRejected
--- PASS: TestValidateDuplicateStateRejected (0.00s)
=== RUN   TestValidateDuplicateEventRejected
--- PASS: TestValidateDuplicateEventRejected (0.00s)
=== RUN   TestValidateHistoricalEventRequiresSourceRef
--- PASS: TestValidateHistoricalEventRequiresSourceRef (0.00s)
=== RUN   TestValidateInvalidEvidenceKind
--- PASS: TestValidateInvalidEvidenceKind (0.00s)
=== RUN   TestValidateInvalidExerciseTime
--- PASS: TestValidateInvalidExerciseTime (0.00s)
=== RUN   TestValidateStateWithNoLanguagesIsGap
--- PASS: TestValidateStateWithNoLanguagesIsGap (0.00s)
=== RUN   TestValidateDuplicateScenarioIDAcrossStates
--- PASS: TestValidateDuplicateScenarioIDAcrossStates (0.00s)
=== RUN   TestValidateCrossStateHistoricalReferenceRejected
--- PASS: TestValidateCrossStateHistoricalReferenceRejected (0.00s)
=== RUN   TestAssessLaunchInsufficientStates
--- PASS: TestAssessLaunchInsufficientStates (0.00s)
=== RUN   TestAssessLaunchInsufficientScenariosPerState
--- PASS: TestAssessLaunchInsufficientScenariosPerState (0.00s)
=== RUN   TestStructuralValidityIndependentOfLaunchAcceptance
--- PASS: TestStructuralValidityIndependentOfLaunchAcceptance (0.00s)
=== RUN   TestRegionalDrillCatalogue
--- PASS: TestRegionalDrillCatalogue (0.00s)
PASS
ok  	sthira/backend/internal/catalogue	0.378s
```

### B. Artifact Index
- `plan/drills/catalogue.json`: 10 states, 21 historical events, 20 scenarios.
- `plan/drills/language-matrix.md`: BCP-47 language support table, reviewer qualifications, ISL boundary rules.
- `plan/drills/facilitator-protocol.md`: Verbatim briefing script, immediate stop triggers, zero-surveillance guarantees.
- `plan/drills/task-scripts.md`: 7 participant tasks with objective pass/fail metrics.

---

## 3. Summary & Next Steps
- Task Q01 drill inputs and regional catalogue are prepared, validated, and ready for participant drills (Q02).
- Gaps in live regional reviewer recruitment (O03) and official multi-state route authority (O05) remain recorded as open external gates without fabricating fictitious completion.
