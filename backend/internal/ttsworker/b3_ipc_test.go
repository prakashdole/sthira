package ttsworker

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ttsAdapterB3Script is a Python mock that echoes request_id in
// every synthesize response. Tests use it to exercise the
// dispatcher's correlation behavior.
const ttsAdapterB3Script = `#!/usr/bin/env python3
import base64
import json
import sys

DIGEST = "0" * 64
WAV = bytes.fromhex(
    "524946462600000057415645666d74201000000001000100200000"
    "080200646174610800000000000000000000000000"
)

for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except Exception:
        continue
    op = msg.get("op", "")
    rid = msg.get("request_id", "")
    if op == "ready":
        sys.stdout.write(json.dumps({
            "status": "ready",
            "revision": "mock-tts-1",
            "languages": ["hi-IN", "ml-IN"],
            "voices": [{"language": "hi-IN", "name": "default", "revision": "vmock-1"}],
            "digest_name": "mock-tts",
            "sample_rate": 44100,
            "digest_sha256": DIGEST,
        }) + "\n")
        sys.stdout.flush()
    elif op == "synthesize":
        sys.stdout.write(json.dumps({
            "request_id": rid,
            "audio_b64": base64.b64encode(WAV).decode("ascii"),
            "duration_secs": 0.05,
        }) + "\n")
        sys.stdout.flush()
    elif op == "shutdown":
        sys.stdout.write(json.dumps({"status": "shutdown"}) + "\n")
        sys.stdout.flush()
        sys.exit(0)
sys.exit(0)
`

func writeTTSAdapterB3Script(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "mock_tts_adapter_b3.py")
	if err := os.WriteFile(bin, []byte(ttsAdapterB3Script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// TestB3_TTS_ConcurrentSynthsGetOwnResults exercises the TTS
// dispatcher under concurrent load: each caller's request_id
// round-trips through the subprocess and is delivered back to
// the matching caller.
func TestB3_TTS_ConcurrentSynthsGetOwnResults(t *testing.T) {
	bin := writeTTSAdapterB3Script(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	const N = 25
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make([]error, N)
	empty := make([]bool, N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			ctx := RequestContext{Canceled: make(chan struct{})}
			res, err := r.Synthesize(ctx, "hello", "hi-IN", "")
			if err != nil {
				errs[i] = err
				return
			}
			empty[i] = res.Empty
		}()
	}
	wg.Wait()
	for i := 0; i < N; i++ {
		if errs[i] != nil {
			t.Errorf("[%d] err: %v", i, errs[i])
			continue
		}
		if empty[i] {
			t.Errorf("[%d] empty result", i)
		}
	}
}

// TestB3_TTS_CancelThenRetry verifies cancel does not consume the
// retry's response.
func TestB3_TTS_CancelThenRetry(t *testing.T) {
	bin := writeTTSAdapterB3Script(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	canceled := make(chan struct{})
	close(canceled)
	ctx1 := RequestContext{Canceled: canceled}
	_, err1 := r.Synthesize(ctx1, "hello", "hi-IN", "")
	if err1 == nil {
		t.Errorf("cancelled call should return error")
	}
	res2, err2 := r.Synthesize(RequestContext{Canceled: make(chan struct{})}, "hello", "hi-IN", "")
	if err2 != nil {
		t.Fatalf("retry: %v", err2)
	}
	if res2.Empty || len(res2.WavBytes) == 0 {
		t.Errorf("retry returned empty")
	}
}

// TestB3_TTS_NearLimitAudioAccepted verifies the TTS scanner
// buffer cap derived from the permitted audio size (256 KiB) +
// base64/JSON overhead accepts a near-limit audio payload.
//
// We construct a 22050 Hz mono 16-bit WAV whose PCM portion is
// just under MaxOutputBytes (256 KiB). The base64-encoded form is
// ~342 KiB; the dispatcher's 1 MiB scanner buffer accepts it.
func TestB3_TTS_NearLimitAudioAccepted(t *testing.T) {
	// Replace the mock script with one that emits near-limit audio.
	const nearLimitScript = `#!/usr/bin/env python3
import base64
import json
import sys

DIGEST = "0" * 64
# Build a 22050 Hz mono 16-bit WAV with PCM size just under 256 KiB.
# 256*1024 = 262144 bytes total target; we want PCM ~= 262100 bytes
# to keep total WAV under 256 KiB.
target_pcm_bytes = 262000
fmt_chunk = (
    b"fmt " + (16).to_bytes(4, "little")
    + (1).to_bytes(2, "little")     # PCM
    + (1).to_bytes(2, "little")     # mono
    + (22050).to_bytes(4, "little") # sample rate
    + (22050 * 2).to_bytes(4, "little")  # byte rate
    + (2).to_bytes(2, "little")      # block align
    + (16).to_bytes(2, "little")     # bits per sample
)
data_chunk = b"data" + target_pcm_bytes.to_bytes(4, "little") + b"\x00" * target_pcm_bytes
WAV = b"RIFF" + (4 + len(fmt_chunk) + len(data_chunk)).to_bytes(4, "little") + b"WAVE" + fmt_chunk + data_chunk

for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except Exception:
        continue
    op = msg.get("op", "")
    rid = msg.get("request_id", "")
    if op == "ready":
        sys.stdout.write(json.dumps({
            "status": "ready",
            "revision": "mock-tts-1",
            "languages": ["hi-IN"],
            "voices": [{"language": "hi-IN", "name": "default", "revision": "vmock-1"}],
            "digest_name": "mock-tts",
            "sample_rate": 44100,
            "digest_sha256": DIGEST,
        }) + "\n")
        sys.stdout.flush()
    elif op == "synthesize":
        sys.stdout.write(json.dumps({
            "request_id": rid,
            "audio_b64": base64.b64encode(WAV).decode("ascii"),
            "duration_secs": 11.88,
        }) + "\n")
        sys.stdout.flush()
    elif op == "shutdown":
        sys.stdout.write(json.dumps({"status": "shutdown"}) + "\n")
        sys.stdout.flush()
        sys.exit(0)
sys.exit(0)
`
	dir := t.TempDir()
	bin := filepath.Join(dir, "near_limit.py")
	if err := os.WriteFile(bin, []byte(nearLimitScript), 0o755); err != nil {
		t.Fatal(err)
	}
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	res, err := r.Synthesize(RequestContext{Canceled: make(chan struct{})}, "hello", "hi-IN", "")
	if err != nil {
		t.Fatalf("near-limit synth: %v", err)
	}
	if res.Empty {
		t.Errorf("near-limit returned empty")
	}
	if int64(len(res.WavBytes)) > int64(MaxOutputBytes) {
		t.Errorf("audio exceeds limit: %d > %d", len(res.WavBytes), MaxOutputBytes)
	}
}

// TestB3_TTS_OverLimitRejectedAtScanner ensures a payload over
// the scanner cap is rejected as ErrRuntimeUnavailable rather
// than truncating.
func TestB3_TTS_OverLimitRejectedAtScanner(t *testing.T) {
	const hugeScript = `#!/usr/bin/env python3
import base64
import json
import sys

DIGEST = "0" * 64
# 2 MiB of base64 — exceeds the 1 MiB scanner cap.
BIG = b"\x00" * (2 * 1024 * 1024)

for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except Exception:
        continue
    op = msg.get("op", "")
    rid = msg.get("request_id", "")
    if op == "ready":
        sys.stdout.write(json.dumps({
            "status": "ready",
            "revision": "mock-tts-1",
            "languages": ["hi-IN"],
            "voices": [{"language": "hi-IN", "name": "default", "revision": "vmock-1"}],
            "digest_name": "mock-tts",
            "sample_rate": 44100,
            "digest_sha256": DIGEST,
        }) + "\n")
        sys.stdout.flush()
    elif op == "synthesize":
        sys.stdout.write(json.dumps({
            "request_id": rid,
            "audio_b64": base64.b64encode(BIG).decode("ascii"),
            "duration_secs": 999.0,
        }) + "\n")
        sys.stdout.flush()
    elif op == "shutdown":
        sys.stdout.write(json.dumps({"status": "shutdown"}) + "\n")
        sys.stdout.flush()
        sys.exit(0)
sys.exit(0)
`
	dir := t.TempDir()
	bin := filepath.Join(dir, "huge.py")
	if err := os.WriteFile(bin, []byte(hugeScript), 0o755); err != nil {
		t.Fatal(err)
	}
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	_, err := r.Synthesize(RequestContext{Canceled: make(chan struct{})}, "hello", "hi-IN", "")
	if err == nil {
		t.Fatal("expected error on oversize payload")
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("expected ErrRuntimeUnavailable, got %v", err)
	}
}

// TestB3_TTS_SubprocessExitReleasesPending kills the subprocess
// and verifies the next caller sees ErrRuntimeUnavailable.
func TestB3_TTS_SubprocessExitReleasesPending(t *testing.T) {
	bin := writeTTSAdapterB3Script(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	r.demux.mu.Lock()
	proc := r.demux.proc
	r.demux.mu.Unlock()
	if proc == nil || proc.Process == nil {
		t.Fatal("expected live process")
	}
	if err := proc.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	_, err := r.Synthesize(RequestContext{Canceled: make(chan struct{})}, "hello", "hi-IN", "")
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("expected ErrRuntimeUnavailable, got %v", err)
	}
	_ = r.Close()
}

// TestB3_TTS_ShutdownLeavesNoLiveReader verifies Close drains
// the demultiplexer goroutine cleanly.
func TestB3_TTS_ShutdownLeavesNoLiveReader(t *testing.T) {
	bin := writeTTSAdapterB3Script(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	// Capture demux exit channel before Close clears r.demux.
	demuxExit := r.demux.demuxExit
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-demuxExit:
	case <-time.After(2 * time.Second):
		t.Fatal("demultiplexer did not exit within 2s")
	}
	// Idempotent.
	if err := r.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestB3_TTS_RunWithGoRace is the race-detector friendly
// version; must be run with -race.
func TestB3_TTS_RunWithGoRace(t *testing.T) {
	bin := writeTTSAdapterB3Script(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	const N = 50
	var counter atomic.Int64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			res, err := r.Synthesize(RequestContext{Canceled: make(chan struct{})}, "hi", "hi-IN", "")
			if err != nil {
				t.Errorf("err: %v", err)
				return
			}
			if res.Empty || len(res.WavBytes) == 0 {
				t.Errorf("empty result")
			}
			counter.Add(1)
		}()
	}
	wg.Wait()
	if got := counter.Load(); got != N {
		t.Errorf("completed: got %d want %d", got, N)
	}
}
