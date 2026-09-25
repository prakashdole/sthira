// ipc_bounded_test.go — adversarial boundedness proofs for the TTS
// dispatcher, mirroring asrworker/ipc_bounded_test.go: startup
// correlation registered before the reader can deliver; a wedged
// (non-reading) child cannot hang send/Close; malformed,
// unsolicited, duplicate and unknown responses reset without
// cross-delivery; cancel-then-retry stays clean; concurrent close
// terminates. REAL helper subprocesses; run under -race in the
// nested ttsworker module.
package ttsworker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func helperTTS(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "helper_tts.py")
	if err := os.WriteFile(bin, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

const (
	pyTTSReadyAtImport = `#!/usr/bin/env python3
import sys, time
sys.stdout.write('{"status":"ready","revision":"race-1","languages":["hi-IN"],"voices":[{"language":"hi-IN","name":"v1","revision":"r1"}],"digest_name":"x","digest_sha256":"0","sample_rate":44100}\n')
sys.stdout.flush()
time.sleep(300)
`
	pyTTSNonReader = `#!/usr/bin/env python3
import sys, time
sys.stdout.write('{"status":"ready","revision":"nr-1","languages":["hi-IN"],"voices":[{"language":"hi-IN","name":"v1","revision":"r1"}],"digest_name":"x","digest_sha256":"0","sample_rate":44100}\n')
sys.stdout.flush()
time.sleep(300)
`
	pyTTSCaos = `#!/usr/bin/env python3
import json, sys, threading, time
def w(o):
    sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()
w({"status":"ready","revision":"chaos-1","languages":["hi-IN","ml-IN"],
   "voices":[{"language":"hi-IN","name":"v1","revision":"r1"}],
   "digest_name":"x","digest_sha256":"0","sample_rate":44100})
def slow(rid):
    time.sleep(30)
    w({"request_id": rid, "audio_b64": "", "duration_secs": 0.0})
for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    msg = json.loads(line)
    op = msg.get("op"); rid = msg.get("request_id", "")
    if op == "synthesize":
        if rid == "R-chaos":
            w({"request_id": rid, "audio_b64": "Zmlyc3Q=", "duration_secs": 0.1})
            w({"request_id": rid, "audio_b64": "ZHVw", "duration_secs": 0.1})
            w({"request_id": "ghost", "audio_b64": "Z2hvc3Q=", "duration_secs": 0.1})
            continue
        if rid == "R-junk":
            sys.stdout.write("not json at all\n"); sys.stdout.flush()
            continue
        if rid == "R-anon":
            w({"audio_b64": "bm9uZQ==", "duration_secs": 0.1})
            continue
        if rid == "R-slow":
            threading.Thread(target=slow, args=(rid,), daemon=True).start()
            continue
        w({"request_id": rid, "audio_b64": "ZWNobyE=", "duration_secs": 0.1})
    elif op == "shutdown":
        break
sys.exit(0)
`
)

func loadCao(t *testing.T, src string) *AdapterSubprocessRuntime {
	t.Helper()
	rt := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{Cmd: helperTTS(t, src), Module: "ignored"})
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	return rt
}

func TestTTSIPC_StartupRace(t *testing.T) {
	rt := loadCao(t, pyTTSReadyAtImport)
	_ = rt.Close()
}

func TestTTSIPC_WriteDeadlineAndBoundedClose(t *testing.T) {
	rt := loadCao(t, pyTTSNonReader)
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()
	big := []byte(strings.Repeat("y", 256*1024))
	big = append(big, '\n')
	start := time.Now()
	if err := disp.writeStdin(big, 300*time.Millisecond, nil); err == nil {
		t.Fatal("write to non-reading child must fail")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("write must be bounded; took %v", d)
	}
	start = time.Now()
	if err := rt.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Close must not wait on a wedged child; took %v", d)
	}
	disp.mu.Lock()
	reaped := disp.proc != nil && disp.proc.ProcessState != nil
	disp.mu.Unlock()
	if !reaped {
		t.Error("child not reaped after Close")
	}
}

func TestTTSIPC_MalformedResets(t *testing.T) {
	rt := loadCao(t, pyTTSCaos)
	defer rt.Close()
	ctx := RequestContext{DeadlineMillis: time.Now().Add(5 * time.Second).UnixMilli()}
	if _, err := rt.Synthesize(ctx, "hi", "hi-IN", "v1"); err != nil {
		t.Fatalf("clean synth before chaos: %v", err)
	}
	if _, err := rt.Synthesize(ctx, "junk", "hi-IN", "v1"); err == nil {
		// R-junk only triggers with request id R-junk; Synthesize
		// auto-generates ids, so drive the dispatcher directly.
	}
	// Drive the dispatcher directly for the named cases.
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()
	junk := ttsAdapterRequest{Op: "synthesize", RequestID: "R-junk", Text: "x", Language: "hi-IN", Voice: "v1", SampleRate: 44100}
	if _, err := disp.send(junk, "R-junk", 5*time.Second, ctx); err == nil {
		t.Fatal("malformed response must fail")
	}
	ok := ttsAdapterRequest{Op: "synthesize", RequestID: "R-after-junk", Text: "x", Language: "hi-IN", Voice: "v1", SampleRate: 44100}
	start := time.Now()
	if _, err := disp.send(ok, "R-after-junk", 5*time.Second, ctx); err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("post-corruption send must fail fast: %v (%v)", err, time.Since(start))
	}
}

func TestTTSIPC_DuplicateAndUnknownDropped(t *testing.T) {
	rt := loadCao(t, pyTTSCaos)
	defer rt.Close()
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()
	chaos := ttsAdapterRequest{Op: "synthesize", RequestID: "R-chaos", Text: "x", Language: "hi-IN", Voice: "v1", SampleRate: 44100}
	resp, err := disp.send(chaos, "R-chaos", 5*time.Second, RequestContext{})
	if err != nil {
		t.Fatalf("chaos: %v", err)
	}
	if resp.AudioB64 != "Zmlyc3Q=" {
		t.Errorf("cross-delivery: got %q", resp.AudioB64)
	}
	clean := ttsAdapterRequest{Op: "synthesize", RequestID: "R-clean", Text: "x", Language: "hi-IN", Voice: "v1", SampleRate: 44100}
	resp2, err := disp.send(clean, "R-clean", 5*time.Second, RequestContext{})
	if err != nil || resp2.AudioB64 != "ZWNobyE=" {
		t.Fatalf("post-chaos: %v %q", err, resp2.AudioB64)
	}
}

func TestTTSIPC_AnonResponseResets(t *testing.T) {
	rt := loadCao(t, pyTTSCaos)
	defer rt.Close()
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()
	anon := ttsAdapterRequest{Op: "synthesize", RequestID: "R-anon", Text: "x", Language: "hi-IN", Voice: "v1", SampleRate: 44100}
	if _, err := disp.send(anon, "R-anon", 5*time.Second, RequestContext{}); err == nil {
		t.Fatal("unsolicited id-less response must poison the stream")
	}
}

func TestTTSIPC_CancelThenRetry(t *testing.T) {
	rt := loadCao(t, pyTTSCaos)
	defer rt.Close()
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()
	canceled := make(chan struct{})
	slow := ttsAdapterRequest{Op: "synthesize", RequestID: "R-slow", Text: "x", Language: "hi-IN", Voice: "v1", SampleRate: 44100}
	go func() { time.Sleep(200 * time.Millisecond); close(canceled) }()
	ctx := RequestContext{Canceled: canceled}
	if _, err := disp.send(slow, "R-slow", 10*time.Second, ctx); err == nil {
		t.Fatal("canceled slow request must fail")
	}
	retry := ttsAdapterRequest{Op: "synthesize", RequestID: "R-retry", Text: "x", Language: "hi-IN", Voice: "v1", SampleRate: 44100}
	resp, err := disp.send(retry, "R-retry", 5*time.Second, RequestContext{})
	if err != nil {
		t.Fatalf("retry must not consume the late response: %v", err)
	}
	if resp.AudioB64 != "ZWNobyE=" {
		t.Errorf("retry got wrong audio: %q", resp.AudioB64)
	}
}

func TestTTSIPC_ConcurrentClose(t *testing.T) {
	rt := loadCao(t, pyTTSCaos)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cc := contextWithTimeout(4 * time.Second)
			defer cc()
			_, _ = rt.Synthesize(ctx, "x", "hi-IN", "v1")
		}(i)
	}
	time.Sleep(30 * time.Millisecond)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = rt.Close() }()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent traffic + close must terminate")
	}
}

// contextWithTimeout builds a worker RequestContext plus a cancel.
func contextWithTimeout(d time.Duration) (RequestContext, func()) {
	cancel := make(chan struct{})
	return RequestContext{
		DeadlineMillis: time.Now().Add(d).UnixMilli(),
		Canceled:       cancel,
	}, func() { close(cancel) }
}

var _ = json.Marshal
