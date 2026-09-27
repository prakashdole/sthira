# Worker 22 — ASR MIME claims and evidence cleanup
**Timestamp:** 2026-09-27T19:00:00Z
**Status:** NO_CHANGE_NEEDED
**HEAD:** d95df9e (docs: record handoff commit id)

---

## 1. Checkpoint and scope

| Item | Value |
|------|-------|
| HEAD | d95df9e |
| Dirty file | `backend/internal/asrworker/audio_mime_test.go` (+102/-32) |
| Scope | ASR MIME codec claims — asserted behaviors vs production code |

**Files inspected:**
- `backend/internal/asrworker/audio_mime_test.go` (dirty diff + committed baseline)
- `backend/internal/asrworker/audio_compressed.go`
- `backend/internal/asrworker/audio_compressed_test.go`
- `backend/internal/asrworker/audio.go` (WAV decode)
- `plan/worker-reports/round-2/minimax-d/report.md` (prior ASR coverage analysis)

---

## 2. MIME contract vs production code

### What the code declares as supported

`audio_compressed.go:31–37` defines `CompressedContentTypes`:

```go
"audio/ogg",
"audio/webm",
"audio/opus",
"audio/ogg; codecs=opus",
"audio/webm; codecs=opus",
```

`isCompressedCodec` (`audio_compressed.go:47–63`) accepts:
- `audio/webm` and `audio/ogg` (any non-empty codecs param or no param — but rejects non-opus codecs like vorbis)
- `audio/opus` (raw Opus)

Browser-facing MIME types are normalized via `isWorkerSupportedCodec` which extends `WorkerSupportedContentTypes` with `CompressedContentTypes` in `init()`.

### What the tests assert

The dirty `audio_mime_test.go` asserts five concrete behaviors:

| Test | Asserts |
|------|---------|
| `TestDecodeAudio_WebmWithOpusCodecParamAccepted` | `audio/webm;codecs=opus` → non-empty finite mono 16 kHz samples, duration ≈ 1 s |
| `TestDecodeAudio_OggWithOpusCodecParamAccepted` | `audio/ogg;codecs=opus` → same structural checks |
| `TestDecodeAudio_WebmWithoutCodecParamAccepted` | `audio/webm` (no codecs) → routes to ffmpeg, non-empty mono 16 kHz |
| `TestDecodeAudio_AutodetectionAcceptsWebmBytesDeclaredAsOggOpus` | Bytes declare `audio/ogg;codecs=opus` but bytes are WebM → ffmpeg auto-detects and decodes successfully. Comment documents: "ffmpeg relies on container magic bytes rather than the HTTP Content-Type header. No contract requires MIME-to-bytes alignment." |
| `TestDecodeAudio_RejectsWavWithNonPCMFormat/ALaw/MuLaw` | WAV format 3/6/7 → `DecodeError{Reason: "not PCM"}` |

### Are claims consistent with code?

**Yes.** Each assertion maps 1:1 to a production code behavior:

- `audio/webm;codecs=opus` / `audio/ogg;codecs=opus` → `isCompressedCodec == true` → `DecodeCompressed` → ffmpeg with `-acodec pcm_s16le -ac 1 -ar 16000`
- `audio/webm` (no param) → `isCompressedCodec` (line 53) returns `true` because no `codecs` param means "any" (or param is empty string which is falsy in the `codec != ""` check on line 54 but returns `true` before that check)
- MIME/bytes mismatch → ffmpeg ignores the declared MIME, reads container magic bytes → decode succeeds
- WAV non-PCM → `audio.go` `decodeWAVFormat` rejects format != 1

No claims are inflated. The autodetection test explicitly documents the non-contract in a comment.

---

## 3. Dirty delta quality

The dirty `audio_mime_test.go` diff (+102/-32) improves the committed baseline:

- **`seconds` parameter removed** from `genWebmOpusLocal`/`genOggOpusLocal` — generation always 1 s at 440 Hz, hardcoded. Callers updated. Eliminates a parameter that was always `1.0` at every call site.
- **Rich structural assertions** added to all three happy-path tests: non-empty samples, `SampleRate == 16000`, `ChannelsIn == 1`, per-sample `IsNaN`/`IsInf` finite check, duration range `0.9–1.5 s`.
- **Test rename** `MismatchedMimeBytesCrossCodecDecodes` → `AutodetectionAcceptsWebmBytesDeclaredAsOggOpus` — name now describes what is tested, not just that a mismatch exists.
- **Comments added** to rejection tests documenting format numbers (IEEE float A-law mu-law).
- **`t.Fatalf` → `t.Fatal`** in rejection tests (same logic, minor style cleanup).

All assertions check real ffmpeg output. The test helpers invoke real `ffmpeg` locally to generate the audio bytes; the assertions verify the decoded `AudioDecodeResult`. This is the correct pattern: the tests consume the actual encoder/decoder pipeline.

---

## 4. Gap analysis

### No gaps in MIME claims

Every MIME type in `CompressedContentTypes` has a corresponding test exercising the decode path. The autodetection case is explicitly documented as a non-contract (ffmpeg magic-byte detection, not MIME enforcement). No MIME type listed in the production code lacks a test.

### One gap: `audio/opus` (raw) is in `CompressedContentTypes` but has no dedicated test

`audio/opus` is listed in `CompressedContentTypes` and `isCompressedCodec("audio/opus")` returns `true`, but there is no `TestDecodeAudio_RawOpusAccepted` in `audio_mime_test.go`. This is a minor gap — raw Opus is a browser-recorded format (e.g., `MediaRecorder` with `mimeType: "audio/opus"` on supported browsers), but WebM/Opus and Ogg/Opus cover the same codec and the raw path shares the same `DecodeCompressed` code path. The `audio/opus` dispatch is tested indirectly via `TestIsCompressedCodec_TrueOnOpusFormats` in `audio_compressed_test.go` which iterates `CompressedContentTypes` including `"audio/opus"`.

**Verdict:** Low severity. Raw Opus could be added, but the codec path is covered. Not worth a patch at this time — add if `audio/opus` becomes a reported issue.

### No other missing assertions

- Sample bounds (CompressedBytes, DecodedMonoSeconds) are enforced at the `DecodeCompressed`/`DecodeWAV` entry points, not re-verified in MIME tests. This is the correct separation per minimax-d's analysis.
- Process limits (stdout cap, stderr cap, wall-clock deadline) are tested in `ipc_bounded_test.go` and `runtime_adapter_test.go`, not in audio MIME tests.

---

## 5. Candidate patch

**No patch required.** The dirty delta is a pure test improvement. The production source is untouched. No source-only recommendation requiring a patch was identified.

---

## 6. Verification (NOT_RUN — deferred by user)

Execution tests are **NOT_RUN** per user directive.

**Later verification commands:**

```bash
# ASR audio MIME tests (requires ffmpeg)
cd /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker
go test -v -run 'TestDecodeAudio_' -count=1 ./...

# ASR compressed codec dispatch tests
go test -v -run 'TestDecodeCompressed_|TestIsCompressedCodec_|TestIsWorkerSupportedCodec' -count=1 ./...
```

**Expected outcomes:**
- All `TestDecodeAudio_*` pass: real ffmpeg decode produces non-empty finite mono 16 kHz samples
- `TestDecodeAudio_AutodetectionAcceptsWebmBytesDeclaredAsOggOpus` passes despite MIME/bytes mismatch

**Required setup:** `ffmpeg` in PATH.

---

## 7. Status and next action

**Status: NO_CHANGE_NEEDED**

The ASR MIME codec claims in production code are consistent with the test assertions. The dirty `audio_mime_test.go` improvements are ready for Opus to review and stage. One minor gap (raw `audio/opus` no dedicated test) noted but not worth a patch at this time.

**Next action for Opus:**
- Confirm the raw `audio/opus` gap is acceptable (codec path covered via WebM/Ogg tests)
- Stage and commit the ASR test improvements when ready

---

*Worker 22 — ASR MIME claims and evidence cleanup — d95df9e — NOT_VERIFIED by execution*
