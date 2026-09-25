# Task B01 Evidence — Strict Speech / Source / Template Authorization

**Task**: B01 — Strict speech/source/template authorization  
**Owner lane**: `backend/internal/{contracts,orchestration,store,ttsworker,httpserver}`, `backend/migrations/`  
**Base commit**: `6772a5b` (CLEAN)  
**Host Environment**: macOS (Darwin arm64), Go 1.27.1, local PostgreSQL 18.6 (`STHIRA_TEST_ADMIN_DSN='postgres://apple@localhost/postgres?sslmode=disable'`).

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation & Verification Result | Status |
| --- | --- | --- |
| **Exact source/jurisdiction/language/version/template-SHA binding, fail closed** | `scoped.go:readApprovedSpeechKeys` returns only complete approvals; `contracts.IsSpeechKeyApprovedForLanguage` rejects empty key/language, `*` wildcard, nil-map fallback. Check order: SyntheticOnly → zero/exact TemplateVersion → zero/exact SourceVersion → digest. | **PASS** |
| **Digest over approved canonical template bytes (not rendered substitutions)** | SHA-256 hex over canonical `tpl.Text` only. Known digests: `Destination choices are displayed on screen.` → `46b41677adde0539476adbad6937dabb20f6b3b404a676c92e6f4029f70c1411`; `Welcome, citizen.` → `db76e6ebe7435317ff8d37f6bb1d8aaf9bab4468d5c156103dcfe95891008a56`; `Synthetic only.` → `4f7301c8599d75d99b9886fc6558e3cdcdf140e4f0e0b09d70bb8a15b36293dc`. `DigestString` exported from `orchestrationtest`; `sha256HexOfString` in `orchestration/registry.go`. | **PASS** |
| **NULL/empty/zero cannot authorize; no wildcard / TemplateKeys fallback** | Fail-closed in `IsSpeechKeyApprovedForLanguage` and `readApprovedSpeechKeys`. Zero versions rejected by orchestrator. | **PASS** |
| **Quarantine migration for incomplete active rows (no invented approver)** | `0010_p6_template_binding_quarantine.sql`: `SET revoked_at = GREATEST(approved_at, now())` on active rows with NULL language/source_id/template_sha256; CHECK `approved_translations_active_complete` requires revoked-or-complete. Historical revoked rows keep NULL fields for audit. `SchemaRevision = 10`. | **PASS** |
| **Revocation/version change mid-inference suppresses stale output** | `SnapshotRevalidate` re-checks approval + digest; digest change on disk → stale reject (covered by `TestB01_ExactApprovalAuthorizes`). | **PASS** |
| **Synthetic reviewed phrases isolated from operational approval** | Synthetic path uses SYNTHETIC_DEMO gates only; operational `approved_translations` rows required for non-synthetic speech. `TestSynthesize_UnapprovedSyntheticTranslation` green. | **PASS** |
| **Cache keys bind versions/digests/language/voice/settings; cannot resurrect withdrawn instructions** | ttsworker `TemplateSHA256` in cache identity; Validate rejects zero/empty SHA; orchestration rejects before synthesise. | **PASS** |
| **Acceptance: wrong source/jurisdiction/language/digest; zero versions; NULL legacy; revoked; withdrawal mid-inference; stale cache; valid exact approval** | Covered by store/contracts/httpserver acceptance tests (below). DB→context→validator→public HTTP path exercised. | **PASS** |
| **Outbound TTS does not run for unauthorized text** | `TestB01_VoiceSpeech_DigestMismatchRejected`: public `POST /api/v3/voice/speech` returns non-200 and `TTSWorker.SynthesizeCalls()==0`. | **PASS** |
| **Operator grant revalidation + fail-closed no-IdP intact** | Existing operator grant tests remain green in full suite. | **PASS** |
| **Exact approved-content owner still required** | External content ownership remains O03/O11 — not claimed done here. | **OPEN (O03/O11)** |

---

## 2. Test Execution Details

### A. Focused B01 acceptance tests
```
$ STHIRA_TEST_ADMIN_DSN=... go test -v -count=1 -run 'B01_|VerifierReads' ./internal/store/ ./internal/httpserver/
=== RUN   TestB01_NullLanguageRowCannotAuthorizeActive
--- PASS: TestB01_NullLanguageRowCannotAuthorizeActive (2.61s)
=== RUN   TestB01_ExactApprovalAuthorizes
--- PASS: TestB01_ExactApprovalAuthorizes (1.18s)
ok  	sthira/backend/internal/store	3.907s
=== RUN   TestB01_VoiceSpeech_DigestMismatchRejected
--- PASS: TestB01_VoiceSpeech_DigestMismatchRejected (0.00s)
ok  	sthira/backend/internal/httpserver	0.129s

$ go test -v -count=1 -run 'VerifierReads' ./internal/store/
=== RUN   TestScopedContext_VerifierReadsViaAcceptableRoute
--- PASS: TestScopedContext_VerifierReadsViaAcceptableRoute (1.68s)

$ go test -v -count=1 -run 'Synthesize_Unapproved' ./internal/orchestration/
=== RUN   TestSynthesize_UnapprovedSyntheticTranslation
--- PASS: TestSynthesize_UnapprovedSyntheticTranslation (0.00s)

$ go test -v -count=1 -run 'SpeechKeyApproved' ./internal/contracts/
--- PASS: TestEnforce_LanguageSpecificSpeechKeyApproval
--- PASS: TestEnforce_SilentActionDoesNotRequireSpeechApproval
```
Also: `TestIsSpeechKeyApprovedForLanguage_FailClosed` (empty key/language, `*`, nil-map) in `contracts/b01_approval_test.go`.

### B. Full module verification
```
$ gofmt -l internal/   # only other workers' WIP files flagged (drills/offlineclient/scenarioprep); B01 files clean
$ go vet ./...         # exit 0
$ go test ./... -count=1
ok  	sthira/backend/internal/capfeed … sthira/backend/internal/store	(16 packages)
# TEST_EXIT=0 — full suite green

# separate module:
$ cd internal/ttsworker && go test ./... -count=1 && go test -race ./... -count=1
ok  	sthira/backend/internal/ttsworker
ok  	sthira/backend/internal/ttsworker/templates

# race on B01-critical main-module packages:
$ go test -race ./internal/store/ ./internal/contracts/ ./internal/httpserver/ ./internal/orchestration/ -count=1
ok  store / contracts / httpserver / orchestration

$ git diff --check   # exit 0
```

---

## 3. Implementation Summary

| File / area | Change |
| --- | --- |
| `backend/migrations/0010_p6_template_binding_quarantine.sql` | Quarantine incomplete active approvals; CHECK `approved_translations_active_complete` |
| `backend/internal/store/store.go` | `SchemaRevision = 10` |
| `backend/internal/store/scoped.go` | Fail-closed `readApprovedSpeechKeys` (returns digests), `Resolve`, `SnapshotRevalidate` |
| `backend/internal/contracts/context.go` | `ApprovedTemplateSHA`; fail-closed `IsSpeechKeyApprovedForLanguage` |
| `backend/internal/orchestration/{orchestrator,synthesize,registry}.go` | Version+digest checks; `sha256HexOfString` |
| `backend/internal/orchestration/orchestrationtest/fakes.go` | `BuildScopedContext`, `DigestString`, `Worker.SynthesizeCalls()` |
| `backend/internal/ttsworker/{cache,worker}.go` | `TemplateSHA256` bound into cache identity; Validate rejects zero/empty |
| Acceptance tests | `contracts/b01_approval_test.go`, `store/b01_acceptance_test.go`, `httpserver/b01_digest_test.go` |
| `backend/README.md` | Document revision 10 / migrations 0001–0010 |

**Not staged**: `backend/internal/drills/system_assurance_test.go` (other worker), other workers' gofmt-dirty test files.

**Open**: O03/O11 exact approved-content owner; R07 real-model demo still `BLOCKED_HARDWARE`.

---

## 4. Commands to re-run

```bash
export STHIRA_TEST_ADMIN_DSN='postgres://apple@localhost/postgres?sslmode=disable'
cd backend
go vet ./... && go test ./... -count=1
cd internal/ttsworker && go test -race ./... -count=1
```
