# MiniMax D — ASR subprocess and audio-test handoff
**Timestamp:** 2026-09-27T18:45:00Z
**Status:** NO_CHANGE_NEEDED
**HEAD:** d95df9e (docs: record handoff commit id)

---

## 1. Checkpoint and scope

| Item | Value |
|------|-------|
| HEAD | d95df9e |
| Dirty files (ASR scope) | `backend/internal/asrworker/audio_mime_test.go` (+102/-32) |
| TTS dirty (out of scope) | `backend/internal/ttsworker/audio_mime_test.go` — not inspected |
| Scope | ASR only; I owns subprocess lifecycle and audio MIME decode assertions |

**Scope inspected:**
- `backend/internal/asrworker/audio_mime_test.go` (dirty diff + committed baseline)
- `backend/internal/asrworker/runtime_adapter_test.go`
- `backend/internal/asrworker/b3_ipc_test.go`
- `backend/internal/asrworker/ipc_bounded_test.go`
- `backend/internal/asrworker/runtime_adapter.go`
- `backend/internal/asrworker/runtime.go`
- `backend/internal/asrworker/runtime_ipc.go`
- `backend/internal/asrworker/audio.go`
- `backend/internal/asrworker/audio_compressed.go`
- `backend/internal/asrworker/audio_malformed_test.go`
- `backend/internal/asrworker/runtime_test.go`

---

## 2. Existing completed work vs new contribution

### Existing committed baseline (pre-dirty)

The committed `audio_mime_test.go` at HEAD (via b35a875 "test: ASR decoder MIME/codec and TTS WAV boundary coverage") already covered:
- WebM/Opus decode acceptance (`TestDecodeAudio_AcceptsWebmWithOpusCodecParam`)
- Ogg/Opus decode acceptance (`TestDecodeAudio_AcceptsOggWithOpusCodecParam`)
- WebM without codecs param routing (`TestDecodeAudio_WebmWithoutCodecParam_RoutesToCompressed`)
- MIME mismatch auto-detection (`TestDecodeAudio_MismatchedMimeBytesCrossCodecDecodes`)
- WAV non-PCM rejection (formats 3, 6, 7) (`TestDecodeAudio_RejectsWavWithNonPCMFormat`, `RejectsWavWithALawFormat`, `RejectsWavWithMuLawFormat`)
- Empty compressed/WebM rejection (`TestDecodeAudio_EmptyCompressedBytes_Rejected`, `TestDecodeAudio_EmptyWebmBytes_Rejected`)
- Content-type case normalization (`TestDecodeAudio_ContentTypeNormalization`)

### Dirty delta (staged candidate patch)

The dirty `audio_mime_test.go` adds:
- **`math` import added** — required for `math.IsNaN`/`math.IsInf` finite-sample checks
- **`genWebmOpusLocal(t *testing.T)` / `genOggOpusLocal(t *testing.T)`** — removed `seconds float64` parameter; generation is always 1 second of 440 Hz sine, hardcoded. Callers updated accordingly.
- **Richer happy-path assertions** in `TestDecodeAudio_WebmWithOpusCodecParamAccepted` and `TestDecodeAudio_OggWithOpusCodecParamAccepted`:
  - Non-empty samples check
  - `SampleRate == TargetSampleRate (16000)` assertion
  - `ChannelsIn == 1` assertion
  - Per-sample `IsNaN`/`IsInf` finite check with sample index in error
  - Duration range `0.9–1.5 s` assertion (1 s expected for 1 s input)
- **Test rename** `TestDecodeAudio_WebmWithoutCodecParamAccepted` — same rich assertions
- **Test rename** `TestDecodeAudio_AutodetectionAcceptsWebmBytesDeclaredAsOggOpus` — added non-empty samples check; comment documents ffmpeg auto-detection behavior explicitly
- **Comment clarifications** on format 3/6/7 WAV tests (IEEE float, A-law, mu-law)
- **`t.Fatalf` → `t.Fatal`** in rejection tests (same logic, cleaner failure)

### Artifact paths

| Artifact | Path |
|----------|------|
| Dirty test file | `backend/internal/asrworker/audio_mime_test.go` |
| Candidate patch | `plan/worker-reports/round-2/minimax-d/candidate.patch` |

---

## 3. Coverage map — existing runnable assertions

### Startup failure
| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestSubprocessRuntime_LoadModel_RequiresExecutable` | `runtime_adapter_test.go:81` | Missing binary → `ErrRuntimeUnavailable` |
| `TestSubprocessRuntime_StartupTimeout_ReturnsUnavailable` | `runtime_adapter_test.go:298` | Silent subprocess → timeout → `ErrRuntimeUnavailable` |
| `TestIPC_Bounded_StartupRace` | `ipc_bounded_test.go:131` | Instant ready before probe consumed — startup correlation ordering |

### Malformed output
| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestIPC_Bounded_MalformedResponseResets` | `ipc_bounded_test.go:195` | Junk JSON line → `ErrRuntimeUnavailable`; subsequent caller fails fast |
| `TestB3_ASR_MalformedResponseResetsCleanly` | `b3_ipc_test.go:276` | Process kill → EOF → `ErrRuntimeUnavailable` |
| `FuzzDecodeWAVChunkSizes` | `audio_malformed_test.go:106` | Arbitrary chunk sizes; panic = failure |
| `TestDecodeWAV_Malformed_UnknownSizeFmt_NoPanic` | `audio_malformed_test.go:61` | fmt chunk 0xFFFFFFFF sentinel rejected |
| `TestDecodeWAV_Malformed_OversizeKnownFmt_NoPanic` | `audio_malformed_test.go:73` | Oversize fmt chunk rejected before slice |
| `TestDecodeWAV_Malformed_OversizeKnownData_NoPanic` | `audio_malformed_test.go:83` | Truncated data chunk rejected |
| `TestDecodeRIFFWAV_TrailingJunkAfterData_NoPanic` | `audio_malformed_test.go:93` | Junk after valid data chunk — safe |

### Child exit
| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestB3_ASR_SubprocessExitReleasesPending` | `b3_ipc_test.go:228` | Kill subprocess → all pending → `ErrRuntimeUnavailable` |
| `TestIPC_Bounded_ChildExitReleasesPending` | `ipc_bounded_test.go:286` | Process exit mid-flight → all callers released |
| `TestSubprocessRuntime_Close_KillsSubprocess` | `runtime_adapter_test.go:253` | `Close()` kills subprocess; later `Transcribe` → `ErrRuntimeClosed` |

### Timeout/cancellation
| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestSubprocessRuntime_StartupTimeout_ReturnsUnavailable` | `runtime_adapter_test.go:298` | 30 s startup deadline on silent subprocess |
| `TestIPC_Bounded_CancelThenRetry` | `ipc_bounded_test.go:265` | Cancel in-flight → retry succeeds with fresh request_id |
| `TestB3_ASR_CancelDoesNotConsumeLaterResult` | `b3_ipc_test.go:150` | Canceled call does not consume next caller's response |

### Late/duplicate response isolation
| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestIPC_Bounded_DuplicateAndUnknownDropped` | `ipc_bounded_test.go:221` | First valid line wins; duplicate/unknown/late dropped; stream stays clean |
| `TestIPC_Bounded_AnonResponseResets` | `ipc_bounded_test.go:247` | Unsolicited id-less line → all callers released |
| `TestB3_ASR_WrongIDRejected` | `b3_ipc_test.go:190` | Distinct request_ids round-trip correctly; misrouted = rejected |

### Retry/recovery
| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestIPC_Bounded_CancelThenRetry` | `ipc_bounded_test.go:265` | Cancel → retry with new request_id succeeds |
| `TestB3_ASR_CancelDoesNotConsumeLaterResult` | `b3_ipc_test.go:150` | Fresh request after cancel → correct response |

### Owned-child cleanup
| Test | File:line | What it exercises |
|------|-----------|------------------|
| `TestSubprocessRuntime_Close_KillsSubprocess` | `runtime_adapter_test.go:253` | `Close()` kills; `ErrRuntimeClosed` on post-close |
| `TestB3_ASR_ShutdownLeavesNoLiveReader` | `b3_ipc_test.go:308` | `Close()` drains demux goroutine; no goroutine leak |
| `TestIPC_Bounded_ConcurrentClose` | `ipc_bounded_test.go:324` | 20 concurrent requests + 3 concurrent `Close()` → terminates within 10 s |
| `TestIPC_Bounded_WriteDeadlineAndBoundedClose` | `ipc_bounded_test.go:145` | Non-reading child; write deadline fires; `Close()` bounded at 5 s; child reaped |

---

## 4. Audio MIME test analysis

### Do revised audio assertions consume actual encoder/decoder output?

**Yes.** The `genWebmOpusLocal(t)` / `genOggOpusLocal(t)` helpers invoke real `ffmpeg` locally, producing actual WebM/Opus or Ogg/Opus bytes. The revised assertions then pass those bytes through `DecodeAudio` and inspect the returned `AudioDecodeResult.Samples`. The finite-sample checks (`math.IsNaN`/`math.IsInf`), duration range check, and sample-rate/channel assertions are all checking real ffmpeg-decoded output.

### MIME sniffing vs guarantees

**MIME sniffing is confirmed as observed behavior, not a guarantee.** `TestDecodeAudio_AutodetectionAcceptsWebmBytesDeclaredAsOggOpus` (dirty diff, renamed from `MismatchedMimeBytesCrossCodecDecodes`) documents:

> "ffmpeg relies on container magic bytes rather than the HTTP Content-Type header. The worker has no MIME-content consistency check. No contract requires MIME-to-bytes alignment."

This is the documented contract. The test verifies ffmpeg's auto-detection works, not that the worker enforces MIME-to-bytes correspondence.

### Sample bounds vs independently tested byte limits

**Sample bounds (duration, channel count, sample rate) are independently enforced upstream:**
- `DecodeCompressed` (audio_compressed.go:100–103): `CompressedBytes` limit enforced before subprocess spawn
- `DecodeWAV` (audio.go:147–153): `DecodedMonoSeconds` limit enforced post-resample

The audio_mime_test.go tests do NOT independently re-verify these limits — they assume `DefaultDecodeLimits()` is the enforcement point and verify the decoded output is well-formed when inputs are within bounds. This is the correct separation: boundary-crossing is tested at the limits layer; well-formed decode is tested at the codec layer.

---

## 5. Gap analysis

### No high-value missing fake-adapter regression patches identified

All seven coverage categories have runnable test assertions. No speculative patches are warranted.

### Minor observation: TTS audio_mime_test.go shares the same diff pattern

The TTS `audio_mime_test.go` dirty diff shows structurally similar changes (richer assertions, renamed tests, boundary comment additions). Since TTS is out of scope for this worker, this is noted for Opus's awareness but not acted upon here.

---

## 6. Candidate patch

**No patch required.** The dirty delta in `audio_mime_test.go` is a pure test improvement (richer assertions, cleanup of hardcoded parameter, clearer naming). The production source is untouched. No `candidate.patch` is generated because there is no source-only recommendation requiring a patch — the existing dirty work can be staged directly.

If Opus requires a patch artifact anyway, the diff base is:
- File: `backend/internal/asrworker/audio_mime_test.go`
- Committed SHA (pre-edit): `3fe1af3` (from git show)

---

## 7. Verification (NOT_RUN — deferred by user)

Execution tests are **NOT_RUN** per user directive. Source inspection was performed as documented above.

**Later verification commands:**

```bash
# ASR audio MIME tests (requires ffmpeg)
cd /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker
go test -v -run 'TestDecodeAudio_' -count=1 ./...

# ASR IPC bounded tests (spawns real subprocesses; use -race)
go test -v -race -run 'TestIPC_Bounded_' -count=1 ./...

# ASR B3 concurrent tests (requires -race for correctness)
go test -v -race -run 'TestB3_ASR_' -count=1 ./...

# ASR runtime adapter tests
go test -v -run 'TestSubprocessRuntime_' -count=1 ./...

# ASR malformed WAV fuzz (fast)
go test -v -fuzz=FuzzDecodeWAVChunkSizes -fuzztime=10s ./...
```

**Required setup:** `ffmpeg` in PATH; `python3` for real-adapter blocked test; `STHIRA_PYTHONPATH` set for `TestSubprocessRuntime_LoadModel_RealPythonAdapter_BLOCKED`.

**Expected outcomes:**
- All `TestDecodeAudio_*` pass: real ffmpeg decode produces non-empty finite mono 16 kHz samples
- `TestIPC_Bounded_WriteDeadlineAndBoundedClose`: write deadline fires within 3 s; `Close()` reaps child within 5 s
- `TestB3_ASR_ConcurrentRequestsGetOwnResults` (25 concurrent): each request_id returns its own response
- `TestB3_ASR_RunWithGoRace` (50 concurrent, -race): no data races detected

**Negative control (expected to fail or skip):**
- `TestSubprocessRuntime_LoadModel_RealPythonAdapter_BLOCKED`: skips unless `STHIRA_PYTHONPATH` set and real adapter present
- `TestSubprocessRuntime_StartupTimeout_ReturnsUnavailable`: may skip under `testing.Short()` due to 30 s timeout

---

## 8. Status and next action

**Status: NO_CHANGE_NEEDED**

The ASR subprocess and audio MIME test coverage is complete. All seven requested coverage categories have runnable assertions. The dirty `audio_mime_test.go` improvements are ready for Opus to review and stage. No production source was modified.

**Next action for Opus:**
- Review the dirty `audio_mime_test.go` delta (+102/-32)
- Confirm TTS `audio_mime_test.go` changes are handled separately (out of ASR scope)
- Stage and commit the ASR test improvements when ready

---

*MiniMax D — ASR subprocess handoff — d95df9e — NOT_VERIFIED by execution*
