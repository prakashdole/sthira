// ipc_bounded_test.go — adversarial boundedness proofs for the
// subprocess dispatcher (prompt item 4): a child that never reads
// must not hang Request or Close; startup correlation must be
// registered before the reader can deliver; malformed/duplicate/
// unknown responses, cancellation-then-retry, process exit and
// concurrent close must all terminate and reap within budgets.
// These spawn REAL helper processes; run them under -race in the
// nested asrworker module.
package asrworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// helperASR writes a small python script and returns its path.
func helperASR(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "helper_asr.py")
	if err := os.WriteFile(bin, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

const (
	pyNonReader = `#!/usr/bin/env python3
import sys, time
# Never reads stdin after startup; pipe fills on large writes.
sys.stdout.write('{"status":"ready","revision":"nr-1","languages":["hi-IN"],"digest_name":"x","digest_sha256":"0"}\n')
sys.stdout.flush()
time.sleep(300)
`
	pyReadyAtImport = `#!/usr/bin/env python3
import sys, time
# Emits ready BEFORE consuming the probe line from stdin: proves the
# startup correlation was registered before the reader ran.
sys.stdout.write('{"status":"ready","revision":"race-1","languages":["hi-IN"],"digest_name":"x","digest_sha256":"0"}\n')
sys.stdout.flush()
for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    if '"shutdown"' in line:
        break
sys.exit(0)
`
	pyChaos = `#!/usr/bin/env python3
import json, sys
def w(o):
    sys.stdout.write(json.dumps(o) + "\n")
    sys.stdout.flush()
w({"status":"ready","revision":"chaos-1","languages":["hi-IN"],"digest_name":"x","digest_sha256":"0"})
for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    msg = json.loads(line)
    op = msg.get("op")
    rid = msg.get("request_id", "")
    if op == "transcribe":
        if rid == "R-chaos":
            w({"request_id": rid, "text": "first-wins"})       # valid
            w({"request_id": rid, "text": "duplicate"})        # duplicate
            w({"request_id": "ghost", "text": "unknown"})      # unknown id
            w({"request_id": rid, "text": "late dup"})         # late duplicate
            continue
        if rid == "R-junk":
            sys.stdout.write("this is not json\n"); sys.stdout.flush()
            continue
        if rid == "R-anon":
            w({"text": "no id here"})                          # unsolicited
            continue
        w({"request_id": rid, "text": "echo:" + rid})
    elif op == "shutdown":
        w({"status":"shutdown"}); break
sys.exit(0)
`
	pySlowThenOK = `#!/usr/bin/env python3
import json, sys, threading, time
def w(o):
    sys.stdout.write(json.dumps(o) + "\n"); sys.stdout.flush()
w({"status":"ready","revision":"slow-1","languages":["hi-IN"],"digest_name":"x","digest_sha256":"0"})
def slow(rid):
    time.sleep(30)
    w({"request_id": rid, "text": "late"})
for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    msg = json.loads(line)
    op = msg.get("op"); rid = msg.get("request_id", "")
    if op == "transcribe":
        if rid == "R-slow":
            threading.Thread(target=slow, args=(rid,), daemon=True).start()
        else:
            w({"request_id": rid, "text": "echo:" + rid})
    elif op == "shutdown":
        break
sys.exit(0)
`
	pyExitOnRequest = `#!/usr/bin/env python3
import json, os, sys
sys.stdout.write('{"status":"ready","revision":"exit-1","languages":["hi-IN"],"digest_name":"x","digest_sha256":"0"}\n')
sys.stdout.flush()
for raw in sys.stdin:
    line = raw.strip()
    if '"transcribe"' in line:
        sys.stdout.write('{"request_id": "R-exit", "text": "bye"}\n')
        sys.stdout.flush()
        os._exit(0)
`
)

func transcribeReq(rid string) TranscribeRequest {
	return TranscribeRequest{RequestID: rid, Language: "hi-IN", Samples: []float32{0.1}, SampleRate: 16000}
}

// 1. Startup race: a child that answers ready immediately (before
// the probe would be consumed) must still load. The old order
// (start demux, then register) raced and dropped the line as
// "missing request_id".
func TestIPC_Bounded_StartupRace(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pyReadyAtImport), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel must survive an instant ready: %v", err)
	}
	_ = rt.Close()
}

// 2. Non-reading child: a large stdin write against a child that
// never drains must fail INSIDE the write budget, and Close must
// then be bounded and reap the process.
func TestIPC_Bounded_WriteDeadlineAndBoundedClose(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pyNonReader), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("startup (ready emitted by child) failed: %v", err)
	}
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()
	if disp == nil {
		t.Fatal("no dispatcher")
	}
	// Payload larger than the 64 KiB pipe buffer guarantees the
	// write itself blocks (the child has read nothing since ready).
	big := make([]byte, 0, 256*1024)
	for i := 0; i < 256; i++ {
		big = append(big, []byte(strings.Repeat("x", 1024))...)
	}
	start := time.Now()
	err := disp.writeStdin(big, 300*time.Millisecond, nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("write to non-reading child must fail")
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("want ErrRuntimeUnavailable, got %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("write must be bounded by its budget; took %v", elapsed)
	}
	// After the kill-to-unblock path, Close must be bounded and must
	// reap the child.
	start = time.Now()
	if err := rt.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Close must not wait on child cooperation; took %v", d)
	}
	disp.mu.Lock()
	state := disp.proc.ProcessState
	disp.mu.Unlock()
	if state == nil {
		t.Error("child not reaped (ProcessState nil after Close)")
	}
}

// 3. Malformed response resets the dispatcher for everybody.
func TestIPC_Bounded_MalformedResponseResets(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pyChaos), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rt.Transcribe(ctx, transcribeReq("R-junk"))
	if err == nil || !errors.Is(err, ErrRuntimeUnavailable) {
		t.Fatalf("malformed response must surface unavailable, got %v", err)
	}
	// And every later caller must fail fast, not hang.
	start := time.Now()
	_, err = rt.Transcribe(context.Background(), transcribeReq("R-after-junk"))
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("post-corruption request must fail fast: %v (%v)", err, time.Since(start))
	}
}

// 4. Duplicate + unknown + late responses never cross-deliver: the
// first valid line answers, everything else is dropped, and the
// stream stays clean for the next caller.
func TestIPC_Bounded_DuplicateAndUnknownDropped(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pyChaos), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := rt.Transcribe(ctx, transcribeReq("R-chaos"))
	if err != nil {
		t.Fatalf("chaos batch: %v", err)
	}
	if res.Text != "first-wins" {
		t.Errorf("cross-delivery: got %q want %q", res.Text, "first-wins")
	}
	res2, err := rt.Transcribe(context.Background(), transcribeReq("R-clean"))
	if err != nil || res2.Text != "echo:R-clean" {
		t.Fatalf("post-chaos request: %v %q", err, res2.Text)
	}
}

// 5. Unsolicited (id-less) line after startup poisons the stream
// and every caller is released.
func TestIPC_Bounded_AnonResponseResets(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pyChaos), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := rt.Transcribe(ctx, transcribeReq("R-anon")); !errors.Is(err, ErrRuntimeUnavailable) {
		t.Fatalf("want unavailable for unsolicited line, got %v", err)
	}
}

// 6. Cancellation followed by retry: a canceled in-flight request
// must not consume the NEXT caller's response.
func TestIPC_Bounded_CancelThenRetry(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pySlowThenOK), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	if _, err := rt.Transcribe(ctx, transcribeReq("R-slow")); err == nil {
		t.Fatal("canceled slow request must fail")
	}
	res, err := rt.Transcribe(context.Background(), transcribeReq("R-retry"))
	if err != nil || res.Text != "echo:R-retry" {
		t.Fatalf("retry after cancel leaked: %v %q", err, res.Text)
	}
}

// 7. Process exit mid-flight releases all pending callers.
func TestIPC_Bounded_ChildExitReleasesPending(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pyExitOnRequest), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			_, errs[i] = rt.Transcribe(ctx, transcribeReq(fmt.Sprintf("R-exit-%d", i)))
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, e := range errs {
		if e == nil || errors.Is(e, ErrRuntimeUnavailable) {
			ok++
		}
	}
	if ok != len(errs) {
		t.Errorf("every caller must terminate cleanly on child exit: %v", errs)
	}
	// Subsequent requests surface the dead state immediately.
	if _, err := rt.Transcribe(context.Background(), transcribeReq("R-final")); err == nil {
		t.Error("expected error after exit")
	}
}

// 8. Concurrent Close vs in-flight requests: everything terminates
// without hang or panic.
func TestIPC_Bounded_ConcurrentClose(t *testing.T) {
	rt := NewSubprocessRuntime(SubprocessRuntimeConfig{
		Cmd: helperASR(t, pyChaos), Module: "ignored",
		StartupTimeout: 5 * time.Second,
	})
	if err := rt.LoadModel(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			_, _ = rt.Transcribe(ctx, transcribeReq(fmt.Sprintf("R-cc-%d", i)))
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
		t.Fatal("concurrent close/traffic must terminate within 10s")
	}
}
