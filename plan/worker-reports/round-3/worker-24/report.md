# Worker 24 — Speech Metadata Mismatch Regression
**Worker:** worker-24 | **Assignment:** round-3
**Timestamp:** 2026-09-27T13:00:00Z | **HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Output dir:** plan/worker-reports/round-3/worker-24/

---

## 1. Scope Inspected

| File | SHA-256 (base @ d95df9e) | Dirty |
|------|--------------------------|-------|
| `backend/internal/orchestration/orchestrator.go` | `00b772d9533bef020e79acd005c7c95a3ae0d8dc2fa3ecbee6df2e293b39f93e` | no |
| `backend/internal/orchestration/language_binding_test.go` | `5cdfed1de58996766d2830c9b1edfba0b6da11e24e5401dd3ff4122cf3fde73e` | no |
| `backend/internal/orchestration/audio_meta_test.go` | (same file, shared test infra) | no |
| `backend/internal/contracts/worker_health.go` | (TTSWorkerRequest/Response defs) | no |
| `backend/internal/contracts/pipeline.go` | (PipelineAudio def) | no |
| `backend/internal/contracts/tts.go` | `943a492f70734ff2b2832016afdddbeb50ca8ba2f2ff79289feb682fea0cf61e` | no |
| `backend/internal/ttsworker/server_test.go` | `31f69a06e9bcad4930264d793f3cfa444c0a8614acd6e9bd3ab0b054e95a2135` | no |

### Prior Evidence
- `plan/worker-reports/round-2/minimax-h/report.md` (frontend audio lifecycle, not directly applicable to backend boundary)
- Existing language binding tests in `language_binding_test.go` (backend)
- Existing audio metadata tests in `audio_meta_test.go` (backend)

---

## 2. Existing Coverage vs. New Contribution

### Existing coverage (orchestrator.go boundary validation)

| Validation | Location | Covered by test |
|-----------|----------|-----------------|
| `resp.Language != language` rejection | orchestrator.go:758 | `TestLanguageBinding_WorkerMislabelRejected` (both language AND speech_key wrong) |
| `resp.SpeechKey != tpl.SpeechKey` rejection | orchestrator.go:758 | ❌ **NO — only covered when BOTH fields are wrong** |
| Settings vs RIFF header mismatch | orchestrator.go:782–807 | `TestStageTTS_RejectsMismatchedSettings` |
| Worker returns 22050Hz, surfaces correctly | orchestrator.go:793–806 | `TestStageTTS_PropagatesReturnedSettingsNotRequestSide` |
| TTS worker Language mislabel (cross-language proposal) | orchestrator.go:758 | `TestLanguageBinding_CrossLanguageMismatchNeverSynthesizes` |
| TTS worker Language + SpeechKey both wrong | orchestrator.go:758 | `TestLanguageBinding_WorkerMislabelRejected` |

**Key finding: `resp.SpeechKey` validation at line 758 is tested only in the compound case where both Language AND SpeechKey are wrong together. The isolated case — worker returns the correct Language but a wrong SpeechKey — has no test.**

### Existing test gap

`TestLanguageBinding_WorkerMislabelRejected` (language_binding_test.go:160–169) sets `ttsLanguageOverride = langEN` which overrides ONLY Language. Both Language and SpeechKey are wrong in that scenario.

No test exercises:
- Worker returns **correct Language** + **wrong SpeechKey** → orchestrator must reject (line 758)
- Worker returns **wrong Language** + **correct SpeechKey** → orchestrator must reject (line 758)

### Contribution: new test artifact
`language_binding_speechkey_test.go` — adds `TestLanguageBinding_WorkerMisattestedSpeechKeyRejected`: fake TTS returns Language=hi-IN (correct) but SpeechKey="wrong_speech_key" (wrong). Verifies `out.Audio == nil` and `hasStage(StageTTS)`.

---

## 3. Findings with Evidence

### Finding W24-1: `resp.SpeechKey` mismatch rejection has no isolated negative test
**File:** `backend/internal/orchestration/language_binding_test.go`
**Evidence — `orchestrator.go:758`:**
```go
if resp.Language != language || resp.SpeechKey != tpl.SpeechKey {
    return nil, pipelineError(contracts.PipelineModelUnavailable, 500, StageFailure{
        Stage: StageTTS, Code: contracts.ErrInternal,
        Reason: "tts response language/speech_key does not match request",
    })
}
```
The Language and SpeechKey checks are in the same compound `||` condition. If the worker returns Language=hi-IN (correct) but SpeechKey="wrong_key" (wrong), the condition is true and audio is rejected — but this specific combination is not tested.

**Existing `TestLanguageBinding_WorkerMislabelRejected`** sets `ttsLanguageOverride = langEN` (affects Language only). It never tests the isolated SpeechKey mismatch.

**Observable failure before the test:** The rejection code path exists and is correct, but there is no regression guard preventing a future refactor that separates the two checks and inadvertently drops the SpeechKey check.

**Proposed test:** `TestLanguageBinding_WorkerMisattestedSpeechKeyRejected` in `language_binding_speechkey_test.go`. Verifies that when a TTS worker returns the correct Language but a mismatched SpeechKey, the orchestrator returns nil audio and StageTTS failure.

---

## 4. Other Boundary Validations Assessed

### `resp.ContentType` — NOT validated by orchestrator
The `TTSWorkerResponse.ContentType` field (tts.go:137, contract: "audio/wav") is **not checked** in `stageTTS`. It is propagated directly to `PipelineAudio.ContentType` (orchestrator.go:811). However, this is **NOT a finding** because:
- `readWAVHeader` (audio_meta.go:24) validates that actual bytes are valid PCM WAV (RIFF/WAVE/fmt magic, fmtSize≥16, format=1 PCM, bitDepth=16)
- If a worker returns wrong ContentType with valid WAV bytes: ContentType is propagated but audio is valid
- If a worker returns ContentType="audio/wav" with non-WAV bytes: `readWAVHeader` rejects with "wav: missing RIFF/WAVE/fmt magic"
- If a worker returns ContentType="audio/mp3" with MP3 bytes: rejected the same way

**Conclusion:** The ContentType field is redundant with the RIFF header check. No functional gap exists.

### `resp.ByteSize` — NOT validated against actual bytes
`PipelineAudio.ByteSize` (pipeline.go:93) is set from `resp.ContentType` — the worker's attestation. The orchestrator does not cross-check ByteSize against the actual decoded audio length. However:
- `readWAVHeader` validates `byteLen` against `len(b)` at audio_meta.go:77
- The final `PipelineAudio.ByteSize` is used by the client for display/caching; it is not used for any security decision

**Conclusion:** No gap requiring a test.

### `resp.AudioID` — generated from checksum, not from worker
`PipelineAudio.AudioID` is set at orchestrator.go:810 as `AudioID: checksum` where `checksum` is the SHA-256 of the actual decoded bytes. The worker's `resp.AudioID` field (worker_health.go) is **not used** in the orchestrator output. This is correct — the orchestrator is the trust boundary, not the worker.

**Conclusion:** No gap.

---

## 5. Artifact: Missing Negative Test

**File:** `plan/worker-reports/round-3/worker-24/language_binding_speechkey_test.go`
**Status:** NOT_RUN (deferred by user)

```go
// TestLanguageBinding_WorkerMisattestedSpeechKeyRejected:
// TTS worker returns correct Language but wrong SpeechKey.
// Orchestrator must reject at stageTTS (orchestrator.go:758).
//
// Before fix: (N/A — code is correct, test is the missing regression guard)
// After fix: (N/A — no code change, test is new coverage)
```

The test uses the existing `langRig` test infrastructure from `language_binding_test.go`. The fake TTS hook returns Language=req.Language (correct) but SpeechKey="wrong_speech_key" (mismatch). The assertion checks that `out.Audio == nil` and `hasStage(StageTTS)`.

---

## 6. Execution Tests NOT_RUN (deferred by user)

```bash
# Run the new test alongside existing language binding tests
cd /Users/apple/Documents/Projects/MonitoringZ/backend
go test -v -run TestLanguageBinding_WorkerMisattestedSpeechKeyRejected ./internal/orchestration/

# Run all language binding + audio metadata tests
cd /Users/apple/Documents/Projects/MonitoringZ/backend
go test -v -run 'TestLanguageBinding|TestStageTTS' ./internal/orchestration/
```

**Expected observable outcome (when run):** Test passes — `out.Audio == nil` and `hasStage(StageTTS) == true`. If the SpeechKey check at line 758 is ever removed or broken, this test fails and catches the regression.

---

## 7. Status

**STATUS:** NO_CHANGE_NEEDED (source is correct; test artifact adds regression coverage)

### Integration note
The new test file is in `plan/worker-reports/round-3/worker-24/`, separate from the production test file at `backend/internal/orchestration/language_binding_test.go`. Opus should integrate the test content into `language_binding_test.go` (or a new `_speechkey_test.go` in the same package) before claiming full boundary coverage.

### No patch
No production source change. The orchestrator's `resp.SpeechKey` check at line 758 is correct. The new test is the deliverable.

---

*Report generated by worker-24. No execution tests performed. Source-inspection-only.*
