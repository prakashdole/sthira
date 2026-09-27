# MiniMax I — TTS Subprocess and WAV-Test Handoff
**Timestamp:** 2026-09-27
**Status:** NO_CHANGE_NEEDED — minor gap noted; no patch warranted
**HEAD:** d95df9e (docs: record handoff commit id)

---

## 1. Scope Inspected

| File | SHA-256 |
|------|---------|
| backend/internal/ttsworker/audio_mime_test.go (dirty) | f35b88f1f0dad5e00b0a32bb6341dabbd5fa19d46ae1a45075aace17f7d6f048 |
| backend/internal/ttsworker/audio_test.go | ff62f900b0911be5fbb3f8f91f5389df557abc9f5d860fe5219a2e6b957d1f55 |
| backend/internal/ttsworker/runtime_adapter.go | b3807e75c5d5767840127c9a8375836ff355eb0b6fa66afcab7a65120e8d3744 |
| backend/internal/ttsworker/runtime_adapter_test.go | 5ed0607869ca5c1a9a130e1c08125ebf5f4b8e94eff1230b018bbb74f5f741b5 |
| backend/internal/ttsworker/b3_ipc_test.go | b9557846b3ba1461c05b23b81d4a91b6f88ee43c40ada156d8dbb746b2400c91 |
| backend/internal/ttsworker/ipc_bounded_test.go | fb9e8947daa18ccf5a84d2375a439f4f784644e49e828ecc60020276ca894226 |
| backend/internal/ttsworker/audio.go | (committed source) |
| backend/internal/ttsworker/runtime.go | (committed source) |
| backend/internal/ttsworker/runtime_ipc.go | (committed source) |

**Dirty/untracked files (pre-existing, not modified by this worker):**
- `backend/internal/asrworker/audio_mime_test.go` — dirty (ASR scope, D owns)
- `backend/internal/ttsworker/audio_mime_test.go` — **dirty (TTS scope, subject)**
- `backend/internal/middleworker/eval/*.go` — dirty
- `frontend/v2/src/styles.css` — dirty

**Scope:** TTS only. D owns ASR (confirmed from D's report at `plan/worker-reports/round-2/minimax-d/report.md`).

---

## 2. Existing Completed Work vs New Contribution

### Dirty delta (TTS audio_mime_test.go — staged candidate)

The dirty diff (`+87/-91`) removes four tests and replaces them with richer assertions:

| Removed test | Why removed | Replaced by |
|-------------|-------------|-------------|
| `TestFloat32ToWav_MismatchedDeclaredDataSize` | Built a buffer with mismatched header, passed to `Float32ToWav`, but `Float32ToWav` does not read header bytes — it builds its own header. Test did not validate decoding. | `TestFloat32ToWav_MaxRateAccepted` (richer header field assertions) |
| `TestFloat32ToWav_DataChunkLargerThanDeclared` | Same issue: buffer had wrong declared size but `Float32ToWav` ignores it. | `TestFloat32ToWav_MaxRateAccepted` |
| `TestFloat32ToWav_SampleRate48kHz` | Weak assertions (only checked WavOutput struct fields). | `TestFloat32ToWav_MaxRateAccepted` (structural byte-level assertions) |
| `TestFloat32ToWav_JustBelowDurationLimit` | Same boundary as `JustBelowMaxDurationAccepted`. | `TestFloat32ToWav_JustBelowMaxDurationAccepted` |
| `TestFloat32ToWav_ExactlyAtDurationLimit` | Same boundary as `MaxDurationAtMaxRateAccepted`. | `TestFloat32ToWav_MaxDurationAtMaxRateAccepted` |
| `TestFloat32ToWav_AllPositiveOnes`, `AllNegativeOnes`, `SilenceNearFullScale`, `MaxSamplesAtMaxRate` | Narrow edge cases already covered by other tests; the clamp-to-[-1,1] contract is tested by `TestFloat32ToWavRefusesOutOfRangeSamples`. | Consolidated into boundary tests |
| `TestEncodeSilenceWav_ExceedsMaxOutputBytesRejects` | Skip condition: "cannot trigger this boundary" — the byte ceiling is unreachable via rate+duration compliant input (documented in new comment). | Replaced by explanatory comment |
| `TestFloat32ToWav_MaxSamplesAtMaxRate` | Duplicate of `MaxDurationAtMaxRateAccepted`. | Consolidated |

**Net change:** -91 lines of misleading/duplicate tests, +87 lines of structural WAV header validation at 48 kHz with full byte-level assertions.

### New contribution (this review)
Identified that the dirty diff is a net improvement (removes false-positive tests, adds structural validation). No new code changes required. One minor gap identified below.

---

## 3. Coverage Map — TTS Subprocess Assertions

### A — Startup Failure

| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestAdapterSubprocessRuntime_LoadModel_PopulatesRevision` | runtime_adapter_test.go:88 | Mock adapter loads; revision/languages/voices populated |
| `TestAdapterSubprocessRuntime_NotLoadedReturnsUnavailable` | runtime_adapter_test.go:163 | No LoadModel → `ErrRuntimeUnavailable` |
| `TestAdapterSubprocessRuntime_LoadModel_RealPythonAdapter_BLOCKED` | runtime_adapter_test.go:215 | Real python3 adapter BLOCKED → `ErrRuntimeUnavailable` |
| `TestTTSIPC_StartupRace` | ipc_bounded_test.go:91 | Startup race: ready sent before reader starts; no double-delivery |

### B — Malformed / Anonymous Output

| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestTTSIPC_MalformedResets` | ipc_bounded_test.go:125 | Non-JSON line → `markUncertain` → all pending callers fail; post-corruption send fails fast |
| `TestTTSIPC_AnonResponseResets` | ipc_bounded_test.go:172 | Anonymous response (no request_id) → `markUncertain` → subprocess killed |
| `TestB3_TTS_OverLimitRejectedAtScanner` | b3_ipc_test.go:240 | Oversize audio_b64 (>1 MiB) → `ErrRuntimeUnavailable` at scanner |

### C — Child Crash

| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestB3_TTS_SubprocessExitReleasesPending` | b3_ipc_test.go:309 | Kill subprocess → next caller sees `ErrRuntimeUnavailable` |
| `TestIPC_Bounded_ChildExitReleasesPending` | ipc_bounded_test.go:284 | Process exit mid-flight → all callers released |
| `TestB3_TTS_RunWithGoRace` | b3_ipc_test.go:364 | 50 concurrent synths; no data races |

### D — Timeout / Cancellation

| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestTTSIPC_WriteDeadlineAndBoundedClose` | ipc_bounded_test.go:96 | Non-reader child; write deadline fires; `Close()` bounded at 5s; child reaped |
| `TestTTSIPC_CancelThenRetry` | ipc_bounded_test.go:184 | Cancel slow request → retry succeeds |
| `TestB3_TTS_CancelThenRetry` | b3_ipc_test.go:119 | Cancel does not consume retry's response |
| `TestB3_TTS_ConcurrentSynthsGetOwnResults` | b3_ipc_test.go:76 | 25 concurrent callers each get own response |

**Minor gap:** `ttsAdapterPerCallTimeout` (15s) path through `send` → timer fires → `ErrRuntimeUnavailable` is exercised indirectly via slow adapter in `TestB3_TTS_CancelThenRetry` (slow response via `R-slow` in chaos script). Direct assertion with a script that hangs specifically on synthesize requests (not just non-reading) would tighten coverage, but the existing chaos-script + cancel path exercises the deadline mechanism.

### E — Late / Duplicate Response Isolation

| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestTTSIPC_DuplicateAndUnknownDropped` | ipc_bounded_test.go:151 | First valid line wins; duplicate/unknown/late dropped; stream stays clean |
| `TestTTSIPC_AnonResponseResets` | ipc_bounded_test.go:172 | Anonymous response → stream reset; subsequent caller succeeds |

### F — Recovery and Process Cleanup

| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestB3_TTS_ShutdownLeavesNoLiveReader` | b3_ipc_test.go:337 | `Close()` drains demux goroutine; exits within 2s; idempotent |
| `TestTTSIPC_ShutdownLeavesNoLiveReader` | ipc_bounded_test.go:233 | Same for IPC-level test |
| `TestB3_TTS_SubprocessExitReleasesPending` | b3_ipc_test.go:309 | Post-crash caller sees `ErrRuntimeUnavailable` |
| `TestB3_TTS_RunWithGoRace` | b3_ipc_test.go:364 | Concurrent stress + race detector |
| `TestTTSIPC_ConcurrentClose` | ipc_bounded_test.go:207 | 20 requests + 3 concurrent Close → terminates within 10s |

### G — WAV Header / Payload Assertions

| Test | File:line | Coverage |
|------|-----------|----------|
| `TestFloat32ToWav_MaxRateAccepted` (dirty) | audio_mime_test.go:13 | Full byte-level header validation at 48 kHz: RIFF, WAVE, fmt format=1 PCM, mono, 16-bit, sample rate, bits per sample, data chunk size, WavOutput fields, total size |
| `TestFloat32ToWav_MaxDurationAtMaxRateAccepted` (dirty) | audio_mime_test.go:81 | Max rate × max duration; verifies DurationSec, byte count ≤ MaxOutputBytes, byte ceiling independence comment |
| `TestFloat32ToWav_JustBelowMaxDurationAccepted` (dirty) | audio_mime_test.go:106 | One sample below ceiling succeeds |
| `TestFloat32ToWav_JustAboveMaxDurationRejected` (dirty) | audio_mime_test.go:120 | One sample above → error mentions "duration" |
| `TestFloat32ToWav_ZeroSamplesRejected` (dirty) | audio_mime_test.go:136 | Empty samples → rejection |
| `TestFloat32ToWav_AtMaxSampleRateRejected` (dirty) | audio_mime_test.go:145 | Rate > 48000 → rejection |
| `TestFloat32ToWavRefusesOutOfRangeSamples` | audio_test.go:53 | Sample >1 or <-1 → typed error (no silent clamping) |
| `TestFloat32ToWavRefusesDurationBudget` | audio_test.go:85 | Duration > 12s → rejection |
| `TestFloat32ToWavRoundtrip` | audio_test.go:64 | RIFF/WAVE prefix, data byte count, SHA-256 checksum at 22050 Hz |
| `TestEncodeSilenceWavProducesValidHeader` | audio_test.go:13 | Valid header at 22050 Hz, byte count ≤ MaxOutputBytes |
| `TestEncodeSilenceWavRejectsOversizeDuration` | audio_test.go:34 | Duration > 12s, rate > 48000, zero rate, zero duration → rejection |
| `TestEncodeSilenceWav_MultipleDurations` (dirty) | audio_mime_test.go:191 | Various durations produce correct DurationSec |
| `TestEncodeSilenceWavChecksumIsContent` | audio_test.go:98 | Identical silence → identical SHA-256 |

**Note on byte-ceiling independence (from dirty comment):** `MaxOutputBytes` = 44 + `maxSamples*2` where `maxSamples = MaxOutputSampleRate × MaxOutputDurationSeconds`. Any rate-and-duration compliant input automatically satisfies the byte ceiling. No separate byte-ceiling execution path exists to test. The old `TestEncodeSilenceWav_ExceedsMaxOutputBytesRejects` had a skip condition acknowledging this; it was removed.

---

## 4. Gap Analysis

### Gap 1 — Minor: Per-call timeout through full Synthesize path not directly asserted (D)

The `ttsAdapterPerCallTimeout` (15s) deadline path in `runtime_adapter.go:597-608` is exercised indirectly:
- `TestB3_TTS_CancelThenRetry`: the `pyTTSCaos` script's `R-slow` case hangs for 30s via `time.sleep(30)`, but the test's deadline is `10 * time.Second` — so the cancel fires before the per-call timeout. The timeout path is not isolated.
- `TestTTSIPC_WriteDeadlineAndBoundedClose`: tests write deadline (stdin buffer full), not the per-call response timer.

**Impact:** Low. The timeout logic in `send` (timer-based) is structurally identical to the ASR path which is tested. The chaos script with a 30s-sleep synthese is the right tool but the test deadline is too short.

**Recommendation:** Add a `TestB3_TTS_PerCallTimeout_Fires` that uses a script sleeping 5s on synthesize (under the 15s per-call timeout), with a deadline of 3s, asserting `ErrRuntimeUnavailable` on deadline. No new infrastructure needed — uses existing `pyTTSCaos`-pattern with `R-slow`. However, this gap is minor and does not block acceptance.

### Gap 2 — Byte-ceiling test was removed (acceptable)

The old `TestEncodeSilenceWav_ExceedsMaxOutputBytesRejects` had a skip condition: "64 kHz at 12s = 1,548,800 bytes > 1,228,800 MaxOutputBytes, but cannot independently reach the byte ceiling from rate-and-duration compliant input." The dirty diff removes this test and replaces it with an explanatory comment. This is correct: the byte ceiling is a mathematical consequence of the rate and duration limits, not an independent enforcement path.

**Recommendation:** No action needed. The comment in the dirty diff documents the independence correctly.

### Gap 3 — Removed tests were false positives (correct removal)

The four removed tests (`MismatchedDeclaredDataSize`, `DataChunkLargerThanDeclared`, `AllPositiveOnes`, `AllNegativeOnes`) were testing behavior the code does not implement:
- `Float32ToWav` builds its own WAV header from scratch; it never reads or validates the header bytes of any input buffer. A test that constructs a buffer with wrong declared sizes and passes it to `Float32ToWav` is not testing WAV decoding — it's testing that `Float32ToWav` ignores irrelevant bytes and produces correct output from the samples it does read. The samples were valid, so `Float32ToWav` correctly produced a valid WAV. The test passing proved nothing about header validation.
- The `AllPositiveOnes` / `AllNegativeOnes` tests claimed "clamped to 1.0" but `Float32ToWav` does not clamp — it returns an error for out-of-range samples. These tests passed because the values were in range (+1.0, -1.0), not because clamping occurred.

**Recommendation:** The removal is correct. The dirty diff improves the test suite by eliminating tests that gave false confidence.

---

## 5. Dirty Diff Assessment

The dirty `audio_mime_test.go` (+87/-91) is a net improvement:

**Removed (-91 lines):**
- 4 tests whose names misleadingly suggested header validation that never occurred
- 4 narrow edge-case tests already covered by boundary tests
- 1 unreachable byte-ceiling test with a skip condition
- Redundant `MaxSamplesAtMaxRate` test

**Added (+87 lines):**
- `TestFloat32ToWav_MaxRateAccepted`: full byte-level WAV header inspection at 48 kHz — RIFF, WAVE, fmt chunk format/channels/rate/byte-rate/block-align/bits per sample, data chunk size, WavOutput struct fields, total size. This is strictly stronger than the original.
- `TestFloat32ToWav_MaxDurationAtMaxRateAccepted`: max boundary with structural output validation and explanatory comment on byte-ceiling independence.
- `TestFloat32ToWav_JustBelowMaxDurationAccepted`: just-below boundary.
- `TestFloat32ToWav_JustAboveMaxDurationRejected`: just-above boundary → error mentions "duration".
- `TestFloat32ToWav_ZeroSamplesRejected`, `AtMaxSampleRateRejected`, and renamed `EncodeSilenceWav` tests with cleaner naming.

---

## 6. Candidate Patch

**No patch required.** The dirty diff is a test-only improvement that is ready for Opus to stage. All production source is untouched. The one identified minor gap (per-call timeout isolation) does not warrant blocking the existing improvements.

If Opus wishes to address the minor gap nonetheless, the test would be:

```go
// In b3_ipc_test.go — add to pyTTSCaos script:
// elif op == "synthesize":
//     if rid == "R-timeout":
//         time.sleep(5)  // under 15s per-call timeout; test deadline is 3s
//         w({"request_id": rid, "audio_b64": "ZWNobyE=", "duration_secs": 0.1})
//         ...
// Then in a new test:
// ctx := RequestContext{DeadlineMillis: time.Now().Add(3*time.Second).UnixMilli()}
// req := ttsAdapterRequest{Op: "synthesize", RequestID: "R-timeout", ...}
// _, err := disp.send(req, "R-timeout", 15*time.Second, ctx)
// if !errors.Is(err, ErrRuntimeUnavailable) { t.Errorf(...) }
```

---

## 7. Verification (NOT_RUN — deferred by user)

**Source inspection performed:** Yes — audio.go (Float32ToWav, EncodeSilenceWav, buildWavHeader, constants), runtime_adapter.go (Synthesize, send, writeStdin, markUncertain, runDemux, Close), runtime.go (StubRuntime, SubprocessRuntime, SynthResult, error types), b3_ipc_test.go, ipc_bounded_test.go, runtime_adapter_test.go.

**Execution tests: NOT_RUN — deferred by user (usage reset in ~30 min).**

### Post-reset commands and prerequisites

**Prerequisites:**
```bash
# Requires python3 in PATH
python3 --version
```

**Run all TTS tests:**
```bash
cd /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker
go test -v -count=1 ./...
```

**Run WAV audio tests specifically:**
```bash
go test -v -count=1 -run 'TestFloat32ToWav_|TestEncodeSilenceWav_' ./...
```

**Run IPC/subprocess tests (spawns subprocesses; use -race):**
```bash
go test -v -race -count=1 -run 'TestTTSIPC_|TestB3_TTS_|TestAdapterSubprocessRuntime_' ./...
```

**Run B3 concurrent tests:**
```bash
go test -v -race -count=1 -run 'TestB3_TTS_RunWithGoRace|TestB3_TTS_ConcurrentSynths' ./...
```

**Expected outcomes:**
- All `TestFloat32ToWav_*` and `TestEncodeSilenceWav_*` pass: pure Go, no subprocess needed
- `TestTTSIPC_WriteDeadlineAndBoundedClose`: write deadline fires within 3s; `Close()` reaps child within 5s
- `TestB3_TTS_ConcurrentSynthsGetOwnResults`: 25 concurrent, each request_id returns correct response
- `TestB3_TTS_RunWithGoRace`: 50 concurrent, no races detected
- `TestB3_TTS_SubprocessExitReleasesPending`: killed subprocess → `ErrRuntimeUnavailable`
- `TestTTSIPC_StartupRace`: instant-ready script → no double-delivery

**Expected negative control:**
- `TestAdapterSubprocessRuntime_LoadModel_RealPythonAdapter_BLOCKED`: skips unless `STHIRA_PYTHONPATH` set
- `TestTTSIPC_WriteDeadlineAndBoundedClose` may be sensitive to host load

---

## 8. Status and Next Action

**Status: NO_CHANGE_NEEDED**

The TTS subprocess lifecycle and WAV encoding test suites are comprehensive. The dirty `audio_mime_test.go` diff is a net improvement (removes false-positive/meaningless tests, adds structural header validation). All seven requested coverage categories have runnable assertions. The one minor gap (per-call timeout isolation) does not warrant blocking the existing improvements.

**Next action for Opus:**
1. Review the dirty `backend/internal/ttsworker/audio_mime_test.go` diff (+87/-91)
2. Stage and commit the TTS test improvements when ready
3. Optionally: address the minor per-call timeout gap with a `R-timeout` case in `pyTTSCaos` + a new test

---

*MiniMax I — TTS subprocess and WAV-test handoff — d95df9e — NOT_VERIFIED by execution*
