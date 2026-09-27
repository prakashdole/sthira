// lifecycle_test.go — worker-lifecycle proofs for the ASR subprocess
// runtime that b3_ipc_test.go and ipc_bounded_test.go do NOT already
// establish: immediate child exit at startup, malformed/short/
// wrong-shaped READINESS envelopes, a wrong-shaped response body,
// crash mid-request (bounded release, no goroutine leak), caller
// cancellation and per-call deadlines against a live child, stale
// -response isolation when a timed-out request id is re-used,
// recovery on a fresh runtime after the child dies, and Worker
// shutdown leaving no orphan child process.
//
// Every helper is a tiny python fake written into t.TempDir() and
// executed from there. No model weights, no downloads, no network, no
// ports. Each fake publishes its own pid through $LC_PID_FILE so
// orphan assertions are made against a real pid, and the child-side
// handshakes use marker files polled with a deadline (no fixed sleeps
// as the primary synchronisation).
package asrworker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

// lcPyEcho serves the ready envelope given by $LC_READY_LINE (default:
// a complete one) and echoes every transcribe request, except R-hang
// which it deliberately never answers.
const lcPyEcho = `#!/usr/bin/env python3
import json, os, sys, time

DIGEST = "0" * 64
DEFAULT_READY = {
    "status": "ready", "revision": "lc-1", "languages": ["hi-IN"],
    "digest_name": "lc", "digest_sha256": DIGEST,
}

def w(o):
    sys.stdout.write(json.dumps(o) + "\n")
    sys.stdout.flush()

def park():
    # A wedged child: keeps running after stdin closes and after a
    # shutdown op, so only a kill can stop it.
    while True:
        time.sleep(1)

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))

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
        w(json.loads(os.environ.get("LC_READY_LINE") or json.dumps(DEFAULT_READY)))
    elif op == "transcribe":
        if rid == "R-hang":
            f = open(os.path.join(os.environ["LC_DIR"], "hang-armed"), "w")
            f.write("1")
            f.close()
            continue
        w({"request_id": rid, "text": "echo:" + rid})
    elif op == "shutdown":
        if os.environ.get("LC_IGNORE_SHUTDOWN"):
            park()
        w({"status": "shutdown"})
        sys.exit(0)
if os.environ.get("LC_IGNORE_SHUTDOWN"):
    park()
sys.exit(0)
`

// lcPyExitNow publishes its pid and exits non-zero without answering
// the ready probe: a worker process that cannot start.
const lcPyExitNow = `#!/usr/bin/env python3
import os, sys

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))
sys.stderr.write("lc: no model, refusing to serve\n")
sys.exit(7)
`

// lcPyBadReady emits exactly one unsolicited stdout line taken from
// $LC_READY_LINE, then stays alive so a rejected handshake is
// attributable to the ENVELOPE and not to the child exiting.
const lcPyBadReady = `#!/usr/bin/env python3
import os, sys

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))
sys.stdout.write(os.environ["LC_READY_LINE"] + "\n")
sys.stdout.flush()
for raw in sys.stdin:
    if '"shutdown"' in raw:
        break
sys.exit(0)
`

// lcPyWrongShape answers transcribe with syntactically valid JSON of
// the wrong shape (a bare array, not an object).
const lcPyWrongShape = `#!/usr/bin/env python3
import json, os, sys

DIGEST = "0" * 64

def w(o):
    sys.stdout.write(json.dumps(o) + "\n")
    sys.stdout.flush()

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))

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
        w({"status": "ready", "revision": "lc-1", "languages": ["hi-IN"],
           "digest_name": "lc", "digest_sha256": DIGEST})
    elif op == "transcribe":
        sys.stdout.write('[{"request_id": "' + rid + '"}]\n')
        sys.stdout.flush()
    elif op == "shutdown":
        sys.exit(0)
sys.exit(0)
`

// lcPyCrashOnTranscribe dies (os._exit, no reaping help, no reply)
// while it is serving the first transcribe request.
const lcPyCrashOnTranscribe = `#!/usr/bin/env python3
import json, os, sys

DIGEST = "0" * 64

def w(o):
    sys.stdout.write(json.dumps(o) + "\n")
    sys.stdout.flush()

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))

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
        w({"status": "ready", "revision": "lc-1", "languages": ["hi-IN"],
           "digest_name": "lc", "digest_sha256": DIGEST})
    elif op == "transcribe":
        os._exit(9)
    elif op == "shutdown":
        sys.exit(0)
sys.exit(0)
`

// lcPyLateOnce answers R-dup exactly once, late, gated on a marker:
//
//	armed      the request arrived and is parked
//	go         the test allows the (late) response to be written
//	stale-sent the late response is on stdout
//
// Before writing the late response the fake deliberately parks the
// NEXT request it reads (R-slow) and never answers it, so the orphan
// response reaches the pipe while another caller is registered —
// exactly the window in which a mis-routed response would be
// delivered to the wrong caller. Every later R-dup is also left
// unanswered, so a stale response can never be masked by a fresh one.
const lcPyLateOnce = `#!/usr/bin/env python3
import json, os, sys, time

DIR = os.environ["LC_DIR"]
DIGEST = "0" * 64

def w(o):
    sys.stdout.write(json.dumps(o) + "\n")
    sys.stdout.flush()

def touch(name):
    f = open(os.path.join(DIR, name), "w")
    f.write("1")
    f.close()

def wait_file(name, limit=15.0):
    p = os.path.join(DIR, name)
    end = time.time() + limit
    while time.time() < end:
        if os.path.exists(p):
            return True
        time.sleep(0.01)
    return False

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))
seen = set()
never = set()

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
        w({"status": "ready", "revision": "lc-1", "languages": ["hi-IN"],
           "digest_name": "lc", "digest_sha256": DIGEST})
    elif op == "transcribe":
        if rid == "R-dup":
            if rid in seen:
                continue
            seen.add(rid)
            touch("armed")
            wait_file("go")
            parked = json.loads(sys.stdin.readline())
            never.add(parked.get("request_id", ""))
            w({"request_id": rid, "text": "stale:" + rid})
            touch("stale-sent")
        elif rid in never:
            continue
        else:
            w({"request_id": rid, "text": "echo:" + rid})
    elif op == "shutdown":
        w({"status": "shutdown"})
        sys.exit(0)
sys.exit(0)
`

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// lcOpts configures a fake: its source, the readiness line it emits
// (empty means the fake's own default), and any extra env.
type lcOpts struct {
	src       string
	readyLine string
	extraEnv  []string
}

// lcConfig writes a fake into t.TempDir(), returns a runtime config
// pointing at it plus the shared scratch dir. The startup budget is
// short so a hang surfaces in seconds, and the pid file lets the test
// assert on process liveness.
func lcConfig(t *testing.T, o lcOpts) (SubprocessRuntimeConfig, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "lc_fake.py")
	if err := os.WriteFile(bin, []byte(o.src), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{
		"LC_PID_FILE=" + filepath.Join(dir, "pid"),
		"LC_DIR=" + dir,
	}
	if o.readyLine != "" {
		env = append(env, "LC_READY_LINE="+o.readyLine)
	}
	env = append(env, o.extraEnv...)
	return SubprocessRuntimeConfig{
		Cmd:            bin,
		Module:         "ignored",
		Workdir:        dir,
		ExtraEnv:       env,
		StartupTimeout: 3 * time.Second,
	}, dir
}

// lcPID reads the child's published pid, polling until it appears.
func lcPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil {
			if pid, perr := strconv.Atoi(strings.TrimSpace(string(b))); perr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child never published its pid at %s", path)
	return 0
}

// lcSignalAlive reports the raw error from signalling pid 0 times;
// errors.Is(err, syscall.ESRCH) means the process is gone AND reaped.
func lcSignalAlive(pid int) error { return syscall.Kill(pid, 0) }

// lcWaitGone polls (bounded by a deadline) until the pid is gone.
func lcWaitGone(t *testing.T, pid int, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if errors.Is(lcSignalAlive(pid), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("orphan child %d still alive after %v (kill(0) = %v)", pid, d, lcSignalAlive(pid))
}

// lcWaitFile polls (bounded) until the child publishes a marker.
func lcWaitFile(t *testing.T, dir, name string, d time.Duration) {
	t.Helper()
	path := filepath.Join(dir, name)
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child never wrote marker %s within %v", name, d)
}

// lcTouch creates a marker the fake is polling for.
func lcTouch(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// lcNoPanic turns a panic into a test failure with context.
func lcNoPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", what, r)
		}
	}()
	fn()
}

// lcReq is a non-silent transcribe request (the runtime short-circuits
// silence before it reaches the adapter).
func lcReq(rid string) TranscribeRequest {
	return TranscribeRequest{
		RequestID:  rid,
		Language:   "hi-IN",
		Samples:    []float32{0.1, 0.2, 0.3, -0.4},
		SampleRate: 16000,
	}
}

// ---------------------------------------------------------------------------
// 1. startup failure
// ---------------------------------------------------------------------------

// A child that exits immediately (bad start / no model) must surface a
// clear typed error well inside the startup budget, and must leave no
// orphan behind.
func TestLifecycle_ASR_ImmediateChildExitFailsStartupPromptly(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyExitNow})
	rt := NewSubprocessRuntime(cfg)
	t.Cleanup(func() { _ = rt.Close() })

	start := time.Now()
	var err error
	lcNoPanic(t, "LoadModel", func() { err = rt.LoadModel() })
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a child that exits before the ready probe must fail LoadModel")
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("want ErrRuntimeUnavailable, got %v", err)
	}
	if elapsed > 8*time.Second {
		t.Errorf("startup failure must not hang; took %v", elapsed)
	}
	// The failed start must not strand the child: it published its pid
	// before exiting, and the dispatcher's Close reaps it.
	lcWaitGone(t, lcPID(t, filepath.Join(dir, "pid")), 5*time.Second)
	// And the runtime stays unloaded (no half-open state).
	if r := rt.Revision(); r != "" {
		t.Errorf("failed start must not populate revision, got %q", r)
	}
	if _, err := rt.Transcribe(context.Background(), lcReq("R-after-fail")); err == nil {
		t.Error("a runtime whose child never started must refuse to transcribe")
	}
}

// ---------------------------------------------------------------------------
// 2. malformed readiness
// ---------------------------------------------------------------------------

// A readiness envelope that is not a well-formed "ready" object — bad
// JSON, truncated JSON, wrong shape, wrong field type, a non-ready
// status, or missing fields — must be rejected with a typed error and
// without a panic, and must not orphan the child.
func TestLifecycle_ASR_MalformedReadinessRejectedWithoutPanic(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"not-json", "this is not json"},
		{"truncated-json", `{"status":"ready"`},
		{"array-not-object", `[{"status":"ready"}]`},
		{"wrong-field-type", `{"status":42}`},
		{"blocked-status", `{"status":"blocked","error":"model weights absent"}`},
		{"missing-status", `{"revision":"x","languages":["hi-IN"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, dir := lcConfig(t, lcOpts{src: lcPyBadReady, readyLine: tc.line})
			rt := NewSubprocessRuntime(cfg)
			t.Cleanup(func() { _ = rt.Close() })

			start := time.Now()
			var err error
			lcNoPanic(t, "LoadModel", func() { err = rt.LoadModel() })
			if err == nil {
				t.Fatalf("readiness %q must be rejected", tc.line)
			}
			if !errors.Is(err, ErrRuntimeUnavailable) {
				t.Errorf("want ErrRuntimeUnavailable, got %v", err)
			}
			if d := time.Since(start); d > 8*time.Second {
				t.Errorf("rejection must be prompt, took %v", d)
			}
			lcWaitGone(t, lcPID(t, filepath.Join(dir, "pid")), 5*time.Second)
		})
	}
}

// A syntactically valid "ready" that is missing ONE artifact field
// still loads the adapter, but the worker's readiness gate — the layer
// that owns readiness — must refuse it and name the missing field.
func TestLifecycle_ASR_ReadyWithoutMetadataLeavesWorkerUnready(t *testing.T) {
	cases := []struct {
		name      string
		readyLine string
		want      string
	}{
		{
			"no-revision",
			`{"status":"ready","languages":["hi-IN"],"digest_name":"lc","digest_sha256":"` + HashZero + `"}`,
			"revision",
		},
		{
			"no-digest",
			`{"status":"ready","revision":"lc-1","languages":["hi-IN"]}`,
			"digest",
		},
		{
			"no-languages",
			`{"status":"ready","revision":"lc-1","digest_name":"lc","digest_sha256":"` + HashZero + `"}`,
			"language",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := lcConfig(t, lcOpts{src: lcPyEcho, readyLine: tc.readyLine})
			rt := NewSubprocessRuntime(cfg)
			t.Cleanup(func() { _ = rt.Close() })
			if err := rt.LoadModel(); err != nil {
				t.Fatalf("a ready-shaped envelope must load: %v", err)
			}
			w, err := NewWorker(Config{
				Inventory:  Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN"}},
				Runtime:    rt,
				QueueDepth: 2,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = w.LoadAndVerify(context.Background())
			if err == nil {
				t.Fatalf("LoadAndVerify must refuse a runtime missing %s", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("failure must name the missing %s field; got %v", tc.want, err)
			}
			if h := w.Snapshot(); h.Ready || h.Warm {
				t.Errorf("health must stay not-ready, got ready=%v warm=%v", h.Ready, h.Warm)
			}
			_ = w.Shutdown(context.Background())
		})
	}
}

// ---------------------------------------------------------------------------
// 3. malformed output
// ---------------------------------------------------------------------------

// A response body that is valid JSON but the wrong shape is protocol
// corruption: the call must fail fast, must never surface partial
// text, and must fail the dispatcher CLOSED for every later caller
// (see runtime_ipc.go: "stop reading and reset the streams ... the
// dispatcher fails closed") rather than silently skipping the line.
func TestLifecycle_ASR_WrongShapedResponseBodyErrorsWithoutPartialText(t *testing.T) {
	cfg, _ := lcConfig(t, lcOpts{src: lcPyWrongShape})
	rt := NewSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	req := lcReq("R-shape")
	req.Deadline = time.Now().Add(3 * time.Second)
	start := time.Now()
	res, err := rt.Transcribe(context.Background(), req)
	if err == nil {
		t.Fatalf("a wrong-shaped response body must fail; got text %q", res.Text)
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("want ErrRuntimeUnavailable, got %v", err)
	}
	if res.Text != "" {
		t.Errorf("no partial text may survive a malformed response, got %q", res.Text)
	}
	if res.Confidence != nil {
		t.Errorf("no confidence may survive a malformed response, got %v", *res.Confidence)
	}
	// Corrupted protocol is detected, not waited out: the failure must
	// land well inside the caller's own deadline.
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("a malformed response must fail fast, not burn the deadline; took %v", d)
	}
	// Fail closed: the stream is poisoned, so the next caller must not
	// be served by a child whose response framing is untrustworthy.
	if _, err := rt.Transcribe(context.Background(), lcReq("R-after-shape")); err == nil {
		t.Error("a dispatcher that saw a corrupted frame must fail closed")
	}
}

// ---------------------------------------------------------------------------
// 4. crash mid-request
// ---------------------------------------------------------------------------

// A child that dies while serving must release the in-flight caller
// with a typed error well inside the caller's own (generous) budget,
// must let the demultiplexer exit, must leave no goroutine behind, and
// must not orphan the process.
func TestLifecycle_ASR_CrashMidRequestReleasesCallerWithoutLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	cfg, dir := lcConfig(t, lcOpts{src: lcPyCrashOnTranscribe})
	rt := NewSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	pid := lcPID(t, filepath.Join(dir, "pid"))
	rt.mu.Lock()
	demuxExit := rt.demux.demuxExit
	rt.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	res, err := rt.Transcribe(ctx, lcReq("R-crash"))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("a mid-request crash must surface an error, got text %q", res.Text)
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("want ErrRuntimeUnavailable, got %v", err)
	}
	if res.Text != "" {
		t.Errorf("no text may survive a crash, got %q", res.Text)
	}
	if elapsed > 5*time.Second {
		t.Errorf("crash must release the caller promptly, took %v (caller budget was 30s)", elapsed)
	}
	select {
	case <-demuxExit:
	case <-time.After(3 * time.Second):
		t.Fatal("demultiplexer did not exit after the child died")
	}
	if err := rt.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	lcWaitGone(t, pid, 5*time.Second)

	// Bounded wait for the goroutine count to fall back to the
	// pre-adapter baseline: a leaked demux or writer goroutine keeps
	// it permanently above.
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > baseline {
		t.Errorf("goroutine leak after crash+close: %d > baseline %d", got, baseline)
	}
}

// ---------------------------------------------------------------------------
// 5. timeout / cancellation
// ---------------------------------------------------------------------------

// A child that never answers must not pin the caller: an explicit
// cancel and a per-call deadline both return promptly, and the
// dispatcher keeps serving afterwards (a cancel must not poison or
// kill a healthy child).
func TestLifecycle_ASR_CancelAndDeadlineReturnPromptly(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyEcho})
	rt := NewSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// (a) explicit cancel, released by the child's "hang-armed"
	// marker so the race between response and cancel is not a timing
	// lottery.
	ctx, cancel := context.WithCancel(context.Background())
	type outcome struct {
		res TranscribeResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := rt.Transcribe(ctx, lcReq("R-hang"))
		done <- outcome{res, err}
	}()
	lcWaitFile(t, dir, "hang-armed", 5*time.Second)
	start := time.Now()
	cancel()
	select {
	case got := <-done:
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("cancel must return promptly, took %v", d)
		}
		if got.err == nil {
			t.Errorf("a cancelled call must fail, got text %q", got.res.Text)
		}
		if got.res.Text != "" {
			t.Errorf("no text may survive a cancelled call, got %q", got.res.Text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Transcribe did not return within 5s of cancel")
	}

	// (b) per-call deadline on the same unresponsive id.
	dlReq := lcReq("R-hang")
	dlReq.Deadline = time.Now().Add(200 * time.Millisecond)
	start = time.Now()
	deadlineDone := make(chan error, 1)
	go func() {
		_, err := rt.Transcribe(context.Background(), dlReq)
		deadlineDone <- err
	}()
	select {
	case err := <-deadlineDone:
		if err == nil {
			t.Fatal("an unanswered request with a passed deadline must fail")
		}
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("per-call deadline must bound the call, took %v", d)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the per-call deadline did not bound the call within 5s")
	}

	// (c) the child is untouched: a normal request still round-trips.
	res, err := rt.Transcribe(context.Background(), lcReq("R-after-cancel"))
	if err != nil {
		t.Fatalf("a cancel must not poison the dispatcher: %v", err)
	}
	if res.Text != "echo:R-after-cancel" {
		t.Errorf("post-cancel text: got %q", res.Text)
	}
}

// ---------------------------------------------------------------------------
// 6. stale-response isolation
// ---------------------------------------------------------------------------

// A response for a request that already timed out must be dropped, not
// handed to some other caller: the fake releases the late response for
// R-dup only once a second caller (R-slow) is registered, and never
// answers either of them again.
func TestLifecycle_ASR_LateResponseAfterTimeoutNotDeliveredToReusedID(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyLateOnce})
	rt := NewSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// 1. First R-dup times out while the child holds the response.
	first := lcReq("R-dup")
	first.Deadline = time.Now().Add(300 * time.Millisecond)
	if _, err := rt.Transcribe(context.Background(), first); err == nil {
		t.Fatal("the first R-dup must time out")
	}

	// 2. Release the child, then park a concurrent caller: the orphan
	// response now reaches the pipe with R-slow registered.
	lcWaitFile(t, dir, "armed", 5*time.Second)
	lcTouch(t, dir, "go")

	slow := lcReq("R-slow")
	slow.Deadline = time.Now().Add(2 * time.Second)
	slowDone := make(chan struct {
		res TranscribeResult
		err error
	}, 1)
	go func() {
		res, err := rt.Transcribe(context.Background(), slow)
		slowDone <- struct {
			res TranscribeResult
			err error
		}{res, err}
	}()

	lcWaitFile(t, dir, "stale-sent", 5*time.Second)

	// 3. Barrier: a probe answered AFTER the stale line proves the
	// demultiplexer has consumed (and therefore must have dropped) the
	// orphan response before any id is re-registered.
	probe, err := rt.Transcribe(context.Background(), lcReq("R-probe"))
	if err != nil {
		t.Fatalf("barrier probe: %v", err)
	}
	if probe.Text != "echo:R-probe" {
		t.Fatalf("barrier probe text: %q", probe.Text)
	}

	// 4. The parked caller must burn its whole 2s budget with no
	// response: an orphan delivered to the wrong waiter would have
	// ended this call early.
	select {
	case got := <-slowDone:
		t.Fatalf("R-slow must not be answered by an orphaned response "+
			"(returned inside its 2s budget): text=%q err=%v", got.res.Text, got.err)
	case <-time.After(1200 * time.Millisecond):
		// expected: the parked call is still waiting
	}
	select {
	case got := <-slowDone:
		if got.err == nil || got.res.Text != "" {
			t.Errorf("parked caller: text=%q err=%v", got.res.Text, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parked caller never returned")
	}

	// 5. Re-use the id. The child will never answer it, so anything
	// other than a timeout means the stale response was delivered.
	second := lcReq("R-dup")
	second.Deadline = time.Now().Add(700 * time.Millisecond)
	start := time.Now()
	res, err := rt.Transcribe(context.Background(), second)
	if err == nil {
		t.Fatalf("a re-used id must not be answered by the stale response; got %q", res.Text)
	}
	if res.Text != "" {
		t.Errorf("stale response leaked into the next request: %q", res.Text)
	}
	if d := time.Since(start); d > 4*time.Second {
		t.Errorf("the re-used id must time out promptly, took %v", d)
	}

	// 6. The stream is still clean afterwards.
	final, err := rt.Transcribe(context.Background(), lcReq("R-final"))
	if err != nil || final.Text != "echo:R-final" {
		t.Errorf("post-stale request: err=%v text=%q", err, final.Text)
	}
}

// ---------------------------------------------------------------------------
// 7. recovery after restart
// ---------------------------------------------------------------------------

// After the child dies the old runtime must stay failed (no zombie
// service), and a freshly loaded runtime on the same adapter must serve
// normally again.
func TestLifecycle_ASR_FreshRuntimeServesAfterChildDeath(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyEcho})
	rt := NewSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	pid := lcPID(t, filepath.Join(dir, "pid"))
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()
	if err := disp.proc.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	// Bounded wait for the dispatcher to record the death.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		disp.mu.Lock()
		failed := disp.demuxErr != nil
		disp.mu.Unlock()
		if failed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := rt.Transcribe(context.Background(), lcReq("R-dead")); !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("a dead child must surface ErrRuntimeUnavailable, got %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	lcWaitGone(t, pid, 5*time.Second)
	// The failed runtime must not resurrect itself.
	if _, err := rt.Transcribe(context.Background(), lcReq("R-zombie")); !errors.Is(err, ErrRuntimeClosed) {
		t.Errorf("closed runtime must stay closed, got %v", err)
	}

	// Restart: a new runtime on the same adapter serves again.
	cfg2, _ := lcConfig(t, lcOpts{src: lcPyEcho})
	rt2 := NewSubprocessRuntime(cfg2)
	t.Cleanup(func() { _ = rt2.Close() })
	if err := rt2.LoadModel(); err != nil {
		t.Fatalf("restart LoadModel: %v", err)
	}
	res, err := rt2.Transcribe(context.Background(), lcReq("R-restart"))
	if err != nil {
		t.Fatalf("restarted runtime must serve: %v", err)
	}
	if res.Text != "echo:R-restart" {
		t.Errorf("restarted text: got %q", res.Text)
	}
	if rt2.Revision() != "lc-1" {
		t.Errorf("restarted revision: %q", rt2.Revision())
	}
}

// ---------------------------------------------------------------------------
// 8. shutdown kills the owned child
// ---------------------------------------------------------------------------

// Worker.Shutdown owns the runtime: the adapter child it started must
// be dead (ESRCH on a real pid) and reaped once Shutdown returns, even
// when the child ignores the shutdown op and only a kill can stop it.
func TestLifecycle_ASR_WorkerShutdownLeavesNoOrphanChild(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyEcho, extraEnv: []string{"LC_IGNORE_SHUTDOWN=1"}})
	rt := NewSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	pid := lcPID(t, filepath.Join(dir, "pid"))
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()

	w, err := NewWorker(Config{
		Inventory:   Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN"}},
		Runtime:     rt,
		QueueDepth:  2,
		MaxInFlight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	res, err := w.Dispatch(context.Background(), lcReq("R-owned"))
	if err != nil {
		t.Fatalf("Dispatch through the worker: %v", err)
	}
	if res.Text != "echo:R-owned" {
		t.Fatalf("worker dispatch text: %q", res.Text)
	}
	if err := lcSignalAlive(pid); err != nil {
		t.Fatalf("child must be alive while the worker serves: %v", err)
	}

	// Shutdown must not block on a child that refuses to cooperate.
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- w.Shutdown(context.Background()) }()
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return within 10s against a shutdown-ignoring child")
	}
	lcWaitGone(t, pid, 5*time.Second)

	// Reaped, not merely signalled: the dispatcher waited on the child.
	disp.mu.Lock()
	state := disp.proc.ProcessState
	disp.mu.Unlock()
	if state == nil {
		t.Error("child was signalled but never reaped")
	}
}
