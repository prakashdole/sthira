# Worker 23 — TTS Full-Call Timeout Gap

## Assignment
Investigate the TTS full-call timeout gap against `d95df9e`, referencing minimax-i Round 2 report.

## Evidence Reviewed

### Source
- `runtime_adapter.go:42` — `ttsAdapterPerCallTimeout = 15 * time.Second`
- `runtime_adapter.go:553–612` — `send()` method: timeout applied to combined write+wait
- `runtime_adapter.go:597–608` — explicit timer wrapping the response wait
- `worker.go:331` — caller-supplied deadline via `makeContext(req.DeadlineMillis)`
- `ipc_bounded_test.go:184` — `TestTTSIPC_CancelThenRetry`: 30s slow subprocess response correctly cancelled at 200ms, subsequent retry succeeds
- `ipc_bounded_test.go:96` — `TestTTSIPC_WriteDeadlineAndBoundedClose`: write bounded separately from close
- `b3_ipc_test.go:119` — `TestB3_TTS_CancelThenRetry`: concurrent cancel+retry verified
- `backend/internal/ttsworker/go.mod` — nested module (`go 1.23`) inside ttsworker dir

### minimax-i Report (Round 2)
Covers: WAV test handoff, subprocess lifecycle, `audio_mime_test.go` dirty state.
Does **not** claim a timeout gap in `send()` or `Synthesize()`.

## Finding: No Gap in Current Source

The `send()` method at `runtime_adapter.go:553` applies `ttsAdapterPerCallTimeout` (15s) to the
combined write-and-wait operation. The timer starts after the lock is released (line 560),
covers the JSONL write via `remaining()` (line 592), and the response wait is on the
same timer (line 606). Late responses for cancelled/timeout'd requests are dropped
via the `default` branch in the demux loop (line 544).

**Timeout mechanism is correct:**
1. `remaining()` computes how much time is left after the write completes
2. `timer := time.NewTimer(remaining())` fires if the subprocess doesn't respond in time
3. `case <-timer.C` deregisters and returns `ErrRuntimeUnavailable` wrapped with "per-call deadline"
4. Cancellation via `ctx.Canceled` also deregisters and returns `ErrRuntimeUnavailable`

**Cancellation is proven correct by tests:**
- `TestTTSIPC_CancelThenRetry` (ipc_bounded_test.go:184): sends a 30s-slow request, cancels at 200ms,
  retries and gets correct response — proving stale responses don't leak to subsequent calls
- `TestB3_TTS_CancelThenRetry` (b3_ipc_test.go:119): same pattern, passes
- `TestTTSIPC_WriteDeadlineAndBoundedClose`: write timeout bounded, close bounded

All 14 TTS IPC tests pass at `d95df9e`.

## Note on test execution
The `ttsworker/` subdir has its own `go.mod` (go 1.23). Tests must run from inside
that directory: `cd backend/internal/ttsworker && go test ./`

## Status: NO_CHANGE_NEEDED
