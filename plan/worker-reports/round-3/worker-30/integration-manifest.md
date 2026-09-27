# Integration Manifest — Round 3 (Worker 30)

**HEAD:** `d95df9e609454877a91b7c82146b3c6368181b17`
**Generated:** 2026-09-27T02:45:00Z

---

## Working Tree: Already Applied and Ready to Commit

| File | SHA-256 (base) | Change | Worker | Ready |
|-------|----------------|--------|--------|-------|
| `backend/internal/httpserver/stay_http_integration_test.go` | `9698428f9c...` | +47 lines: `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` | MiniMax-A | ✓ |
| `frontend/v2/src/main.ts` | `fd1f98f...` | -6 lines: removed 503/504 `resolvePlace` fallback in `sendVoiceOrText` | MiniMax-E | ✓ |

**Recommended commit 1:** B4 test
```
git add backend/internal/httpserver/stay_http_integration_test.go
git commit -m "test: add PostDenialIdempotencyKeySurvives for B4 idempotency gap"
```

**Recommended commit 2:** Voice outage fix
```
git add frontend/v2/src/main.ts
git commit -m "fix: remove resolvePlace fallback on voice pipeline 503/504"
```

---

## Working Tree: Dirty Files Needing Stage + Test

| File | Change nature | Test command | Prereq |
|-------|--------------|--------------|--------|
| `backend/internal/asrworker/audio_mime_test.go` | ASR subprocess + audio MIME handling improvements | `go test -v -race -count=1 -run 'TestDecodeAudio_|TestIPC_Bounded_|TestB3_ASR_|TestSubprocessRuntime_' ./internal/asrworker/...` | ffmpeg in PATH |
| `backend/internal/middleworker/eval/main.go` | Eval main improvements | `go test -v ./internal/middleworker/eval/...` | None |
| `backend/internal/middleworker/eval/main_test.go` | Eval test improvements | `go test -v ./internal/middleworker/eval/...` | None |
| `backend/internal/middleworker/eval/corpus.go` | Eval corpus | offline corpus-check | None |
| `backend/internal/middleworker/eval/corpus/synthetic_v2.jsonl` | Corpus data | `corpus-check -corpus synthetic_v2.jsonl` | None |
| `backend/internal/ttsworker/audio_mime_test.go` | TTS subprocess + audio MIME handling | `go test -v -race -count=1 ./internal/ttsworker/...` | ffmpeg in PATH |

**Note:** `frontend/v2/src/styles.css` appears in `git status` as modified but produces no content diff — mode/timestamp change only. Ignore for integration purposes.

---

## Round-3 Worker Artifact Summary

| Worker | Status | Artifact path | Recommendation |
|--------|--------|---------------|---------------|
| worker-11 | READY | `plan/worker-reports/round-3/worker-11/candidate.patch` | Review — map layer phantom removal |
| worker-18 | NO_CHANGE_NEEDED | `plan/worker-reports/round-3/worker-18/report.md` | B4 test already applied; no action |
| worker-26 | NO_CHANGE_NEEDED | `plan/worker-reports/round-3/worker-26/presenter-script.md` | Merge into `prototype-presenter-guide.md §3` |
| worker-1 through worker-10, worker-12 through worker-17, worker-19 through worker-25, worker-27, worker-28 | PENDING_REVIEW | `plan/worker-reports/round-3/worker-N/` | Review individually |

---

## Round-3 Worker: No-Change Reports

These workers found the existing implementation satisfactory:

| Worker | Finding |
|--------|---------|
| worker-18 | B4 gap test already in working tree; no patch needed |
| worker-26 | Existing presenter script adequate; presenter-script.md is new optional artifact |

---

## Untracked Files Needing Decision

| File | Recommendation |
|------|---------------|
| `backend/internal/httpserver/citizen_ownership_regression_test.go` | Rename to `citizen_ownership_coverage_map.go` — it is documentation, not a test |
| `backend/internal/middleworker/eval/eval` | Delete or add to `.gitignore` — compiled binary |

---

## Integration Order

```
1. [OPT] Stage dirty ASR tests → test → commit
2. [OPT] Stage dirty TTS tests → test → commit
3. [OPT] Stage dirty eval files → test → commit
4. [COMMIT] B4 idempotency test (stay_http_integration_test.go)
5. [COMMIT] Voice outage fix (main.ts)
6. [REVIEW] worker-11 candidate.patch (map layer phantom)
7. [REVIEW] worker-26 presenter-script.md
8. [AWAIT] Remaining worker reports → review individually
```

---

## K Corrections (from MiniMax-K report)

| K claim | Actual | Correction |
|---------|--------|------------|
| "9 modified files" | 8 with content changes, 1 (styles.css) mode-only | No action — distinction noted |
| "Outage location vs recorder location" | Outage is API-level (503 response), not MediaRecorder | K's framing accurate |
| worker-18 PENDING | worker-18 has since completed with NO_CHANGE_NEEDED | Updated above |

---

*Status: NOT_RUN — manifest only. All test execution deferred by user.*
