// lifecycle_test.go — worker-lifecycle proofs for the TTS adapter
// subprocess that b3_ipc_test.go and ipc_bounded_test.go do NOT
// already establish: startup failure (missing binary AND a child that
// exits immediately), malformed/short/wrong-shaped READINESS
// envelopes, malformed audio payloads, crash mid-request (bounded
// release, no goroutine leak), caller cancellation against a live
// child, stale-response isolation when a timed-out request id is
// re-used, recovery on a fresh runtime after the child dies, and
// Worker shutdown leaving no orphan child process.
//
// Every helper is a tiny python fake written into t.TempDir() and
// executed from there. No model weights, no downloads, no network, no
// ports. Each fake publishes its own pid through $LC_PID_FILE so
// orphan assertions are made against a real pid, and the child-side
// handshakes use marker files polled with a deadline (no fixed sleeps
// as the primary synchronisation).
package ttsworker

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"sthira/backend/internal/ttsworker/templates"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

// lcTTSReady is the default ready envelope: one language, one voice
// and a native sample rate.
const lcTTSReadyJSON = `{"status": "ready", "revision": "lc-1", ` +
	`"languages": ["hi-IN"], ` +
	`"voices": [{"language": "hi-IN", "name": "default", "revision": "lcv-1"}], ` +
	`"digest_name": "lc", "digest_sha256": "` + lcTTSDigest + `", "sample_rate": 44100}`

// lcPyTTS serves the ready envelope taken from $LC_READY_LINE (or the
// default) and answers every synthesize request, except text "hang"
// which it deliberately never answers. The audio payload is selected
// by $LC_AUDIO ("wav", "b64:<literal>", "raw:<n bytes>") and the
// duration by $LC_DURATION.
const lcPyTTS = `#!/usr/bin/env python3
import base64, json, os, sys, time

WAV = bytes.fromhex(
    "524946462600000057415645666d74201000000001000100200000"
    "080200646174610800000000000000000000000000"
)
DEFAULT_READY = __READY__

def w(o):
    sys.stdout.write(json.dumps(o) + "\n")
    sys.stdout.flush()

def audio_b64():
    spec = os.environ.get("LC_AUDIO") or "wav"
    if spec.startswith("b64:"):
        return spec[4:]
    if spec.startswith("raw:"):
        return base64.b64encode(b"A" * int(spec[4:])).decode("ascii")
    return base64.b64encode(WAV).decode("ascii")

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
        override = os.environ.get("LC_READY_LINE")
        w(json.loads(override) if override else DEFAULT_READY)
    elif op == "synthesize":
        if msg.get("text") == "hang":
            f = open(os.path.join(os.environ["LC_DIR"], "hang-armed"), "w")
            f.write("1")
            f.close()
            continue
        w({"request_id": rid, "audio_b64": audio_b64(),
           "duration_secs": float(os.environ.get("LC_DURATION") or "0.05")})
    elif op == "shutdown":
        if os.environ.get("LC_IGNORE_SHUTDOWN"):
            park()
        w({"status": "shutdown"})
        sys.exit(0)
if os.environ.get("LC_IGNORE_SHUTDOWN"):
    park()
sys.exit(0)
`

// lcPyTTSExitNow publishes its pid and exits non-zero without
// answering the ready probe: a worker process that cannot start.
const lcPyTTSExitNow = `#!/usr/bin/env python3
import os, sys

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))
sys.stderr.write("lc: tts adapter failed to load its model\n")
sys.exit(7)
`

// lcPyTTSBadReady emits exactly one unsolicited stdout line taken from
// $LC_READY_LINE, then stays alive so a rejected handshake is
// attributable to the ENVELOPE and not to the child exiting.
const lcPyTTSBadReady = `#!/usr/bin/env python3
import os, sys

open(os.environ["LC_PID_FILE"], "w").write(str(os.getpid()))
sys.stdout.write(os.environ["LC_READY_LINE"] + "\n")
sys.stdout.flush()
for raw in sys.stdin:
    if '"shutdown"' in raw:
        break
sys.exit(0)
`

// lcPyTTSCrashOnSynthesize dies (os._exit, no reply) while it is
// serving the first synthesize request.
const lcPyTTSCrashOnSynthesize = `#!/usr/bin/env python3
import base64, json, os, sys

WAV = bytes.fromhex(
    "524946462600000057415645666d74201000000001000100200000"
    "080200646174610800000000000000000000000000"
)

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
        w(__READY__)
    elif op == "synthesize":
        os._exit(9)
    elif op == "shutdown":
        sys.exit(0)
sys.exit(0)
`

// lcPyTTSLateOnce answers text "dup" exactly once, late, gated on a
// marker:
//
//	armed      the request arrived and is parked
//	go         the test allows the (late) response to be written
//	stale-sent the late response is on stdout
//
// Before writing the late response the fake deliberately parks the
// NEXT request it reads (text "slow") and never answers it, so the
// orphan response reaches the pipe while another caller is registered
// — exactly the window in which a mis-routed response would be
// delivered to the wrong caller. Every later "dup" is also left
// unanswered, so a stale response can never be masked by a fresh one.
const lcPyTTSLateOnce = `#!/usr/bin/env python3
import base64, json, os, sys, time

DIR = os.environ["LC_DIR"]
WAV = bytes.fromhex(
    "524946462600000057415645666d74201000000001000100200000"
    "080200646174610800000000000000000000000000"
)
B64 = base64.b64encode(WAV).decode("ascii")

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
parked = set()

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
    text = msg.get("text", "")
    if op == "ready":
        w(__READY__)
    elif op == "synthesize":
        if text == "dup":
            if text in seen:
                continue
            seen.add(text)
            touch("armed")
            wait_file("go")
            nxt = json.loads(sys.stdin.readline())
            parked.add(nxt.get("request_id", ""))
            w({"request_id": rid, "audio_b64": B64, "duration_secs": 0.05})
            touch("stale-sent")
        elif rid in parked:
            continue
        else:
            w({"request_id": rid, "audio_b64": B64, "duration_secs": 0.05})
    elif op == "shutdown":
        sys.exit(0)
sys.exit(0)
`

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// lcTTSDigest mirrors the mock digests used elsewhere: 64 zero hex
// characters, i.e. an unverified artifact, which the worker treats as
// "not yet hashed".
const lcTTSDigest = "0000000000000000000000000000000000000000000000000000000000000000"

// lcOpts configures a fake: its source, the readiness line it emits
// (empty means the fake's own default), the audio payload spec and the
// duration it reports.
type lcOpts struct {
	src       string
	readyLine string
	audio     string
	duration  string
	extraEnv  []string
}

// lcConfig writes a fake into t.TempDir() and returns a runtime config
// pointing at it plus the shared scratch dir. The pid file lets the
// test assert on process liveness. __READY__ in a fake's source is
// replaced by the canonical ready envelope.
func lcConfig(t *testing.T, o lcOpts) (AdapterSubprocessConfig, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "lc_fake_tts.py")
	src := strings.ReplaceAll(o.src, "__READY__", lcTTSReadyJSON)
	if err := os.WriteFile(bin, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{
		"LC_PID_FILE=" + filepath.Join(dir, "pid"),
		"LC_DIR=" + dir,
		"LC_AUDIO=" + o.audio,
		"LC_DURATION=" + o.duration,
	}
	if o.readyLine != "" {
		env = append(env, "LC_READY_LINE="+o.readyLine)
	}
	env = append(env, o.extraEnv...)
	return AdapterSubprocessConfig{
		Cmd:      bin,
		Module:   "ignored",
		Workdir:  dir,
		ExtraEnv: env,
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

// lcContext is a live request context: a caller-owned cancel channel
// plus a deadline well outside the per-call budget.
func lcContext() (RequestContext, func()) {
	cancel := make(chan struct{})
	return RequestContext{
		DeadlineMillis: time.Now().Add(30 * time.Second).UnixMilli(),
		Canceled:       cancel,
	}, func() { close(cancel) }
}

// ---------------------------------------------------------------------------
// 1. startup failure
// ---------------------------------------------------------------------------

// A missing interpreter must fail fast with a typed error, and a child
// that exits immediately must do the same well inside the (60s)
// production startup budget — as an error, never as a hang — leaving no
// orphan behind.
func TestLifecycle_TTS_StartupFailureSurfacesAsError(t *testing.T) {
	// (a) bad binary: the configured interpreter does not exist.
	rtBad := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    filepath.Join(t.TempDir(), "no-such-interpreter"),
		Module: "ignored",
	})
	start := time.Now()
	var badErr error
	lcNoPanic(t, "LoadModel", func() { badErr = rtBad.LoadModel() })
	if badErr == nil {
		t.Fatal("a missing interpreter must fail LoadModel")
	}
	if !errors.Is(badErr, ErrRuntimeUnavailable) {
		t.Errorf("want ErrRuntimeUnavailable, got %v", badErr)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("bad binary must fail fast, took %v", d)
	}

	// (b) child starts and exits non-zero without answering ready.
	cfg, dir := lcConfig(t, lcOpts{src: lcPyTTSExitNow})
	rt := NewAdapterSubprocessRuntime(cfg)
	t.Cleanup(func() { _ = rt.Close() })
	start = time.Now()
	var err error
	lcNoPanic(t, "LoadModel", func() { err = rt.LoadModel() })
	if err == nil {
		t.Fatal("a child that exits before the ready probe must fail LoadModel")
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("want ErrRuntimeUnavailable, got %v", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("immediate exit must not wait out the 60s startup budget; took %v", d)
	}
	if rt.Revision() != "" {
		t.Errorf("failed start must not populate revision, got %q", rt.Revision())
	}
	if _, err := rt.Synthesize(RequestContext{}, "hi", "hi-IN", ""); !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("an unloaded runtime must refuse to synthesize, got %v", err)
	}
	lcWaitGone(t, lcPID(t, filepath.Join(dir, "pid")), 5*time.Second)
}

// ---------------------------------------------------------------------------
// 2. malformed readiness
// ---------------------------------------------------------------------------

// A readiness envelope that is not a well-formed "ready" object — bad
// JSON, truncated JSON, wrong shape, wrong field type, a non-ready
// status, a missing field, or a ready without a usable native sample
// rate — must be rejected with a typed error and without a panic, and
// must not orphan the child.
func TestLifecycle_TTS_MalformedReadinessRejectedWithoutPanic(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"not-json", "this is not json"},
		{"truncated-json", `{"status":"ready"`},
		{"array-not-object", `[{"status":"ready","sample_rate":44100}]`},
		{"wrong-field-type", `{"status":42,"sample_rate":44100}`},
		{"blocked-status", `{"status":"blocked","error":"voice weights absent","sample_rate":44100}`},
		{"missing-status", `{"revision":"x","sample_rate":44100,"languages":["hi-IN"]}`},
		{"missing-sample-rate", `{"status":"ready","revision":"lc-1","languages":["hi-IN"],` +
			`"voices":[{"language":"hi-IN","name":"default","revision":"lcv-1"}]}`},
		{"impossible-sample-rate", `{"status":"ready","revision":"lc-1","languages":["hi-IN"],` +
			`"voices":[{"language":"hi-IN","name":"default","revision":"lcv-1"}],"sample_rate":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, dir := lcConfig(t, lcOpts{src: lcPyTTSBadReady, readyLine: tc.line})
			rt := NewAdapterSubprocessRuntime(cfg)
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
			if d := time.Since(start); d > 10*time.Second {
				t.Errorf("rejection must be prompt, took %v", d)
			}
			lcWaitGone(t, lcPID(t, filepath.Join(dir, "pid")), 5*time.Second)
		})
	}
}

// A ready envelope with no languages and no voices loads but leaves the
// runtime unable to serve anything: synthesis must refuse rather than
// guess a voice.
func TestLifecycle_TTS_ReadyWithoutVoicesRefusesSynthesis(t *testing.T) {
	cfg, _ := lcConfig(t, lcOpts{
		src:       lcPyTTS,
		readyLine: `{"status":"ready","revision":"lc-1","sample_rate":44100}`,
	})
	rt := NewAdapterSubprocessRuntime(cfg)
	t.Cleanup(func() { _ = rt.Close() })
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("a ready-shaped envelope must load: %v", err)
	}
	if got := rt.Languages(); len(got) != 0 {
		t.Errorf("languages must stay empty, got %v", got)
	}
	if got := rt.Voices(); len(got) != 0 {
		t.Errorf("voices must stay empty, got %v", got)
	}
	ctx, cancel := lcContext()
	defer cancel()
	if _, err := rt.Synthesize(ctx, "hi", "hi-IN", ""); !errors.Is(err, ErrLanguageUnsupported) {
		t.Errorf("want ErrLanguageUnsupported, got %v", err)
	}
	if _, err := rt.Synthesize(ctx, "hi", "hi-IN", "default"); !errors.Is(err, ErrLanguageUnsupported) {
		t.Errorf("an explicit voice must not bypass the language gate; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 3. malformed output
// ---------------------------------------------------------------------------

// Malformed audio from the adapter must fail the call and must never
// yield a partial or unvalidated payload: undecodable base64, a
// duration past the codec ceiling and a payload past the byte ceiling
// all have to be errors with a nil result.
func TestLifecycle_TTS_MalformedAudioRejectedWithoutPartialAudio(t *testing.T) {
	cases := []struct {
		name     string
		audio    string
		duration string
	}{
		{"invalid-base64", "b64:!!!not base64!!!", "0.05"},
		{"unpadded-base64", "b64:QUJ", "0.05"},
		{"whitespace-base64", "b64:QU J", "0.05"},
		{"duration-past-ceiling", "wav", "999"},
		{"bytes-past-ceiling", "raw:1300000", "0.05"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := lcConfig(t, lcOpts{src: lcPyTTS, audio: tc.audio, duration: tc.duration})
			rt := NewAdapterSubprocessRuntime(cfg)
			if err := rt.LoadModel(); err != nil {
				t.Fatalf("LoadModel: %v", err)
			}
			t.Cleanup(func() { _ = rt.Close() })
			ctx, cancel := lcContext()
			defer cancel()

			var (
				res *SynthResult
				err error
			)
			lcNoPanic(t, "Synthesize", func() { res, err = rt.Synthesize(ctx, "hi", "hi-IN", "") })
			if err == nil {
				t.Fatalf("malformed audio must fail; got %+v", res)
			}
			if !errors.Is(err, ErrRuntimeUnavailable) {
				t.Errorf("want ErrRuntimeUnavailable, got %v", err)
			}
			if res != nil {
				if !res.Empty {
					t.Errorf("no result may be produced for malformed audio, got %d bytes", len(res.WavBytes))
				}
				if len(res.WavBytes) != 0 {
					t.Errorf("no partial audio may leak, got %d bytes", len(res.WavBytes))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. crash mid-request
// ---------------------------------------------------------------------------

// A child that dies while serving must release the in-flight caller
// with a typed error well inside the caller's own (generous) budget,
// must let the demultiplexer exit, must leave no goroutine behind, and
// must not orphan the process.
func TestLifecycle_TTS_CrashMidRequestReleasesCallerWithoutLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	cfg, dir := lcConfig(t, lcOpts{src: lcPyTTSCrashOnSynthesize})
	rt := NewAdapterSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	pid := lcPID(t, filepath.Join(dir, "pid"))
	rt.mu.Lock()
	demuxExit := rt.demux.demuxExit
	rt.mu.Unlock()

	ctx, cancel := lcContext()
	defer cancel()
	start := time.Now()
	res, err := rt.Synthesize(ctx, "hi", "hi-IN", "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("a mid-request crash must surface an error, got %+v", res)
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("want ErrRuntimeUnavailable, got %v", err)
	}
	if res != nil && len(res.WavBytes) != 0 {
		t.Errorf("no audio may survive a crash, got %d bytes", len(res.WavBytes))
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
// cancel returns promptly and the dispatcher keeps serving afterwards
// (a cancel must not poison or kill a healthy child).
func TestLifecycle_TTS_CancelReturnsPromptlyAndKeepsServing(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyTTS})
	rt := NewAdapterSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	ctx, cancel := lcContext()
	type outcome struct {
		res *SynthResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := rt.Synthesize(ctx, "hang", "hi-IN", "")
		done <- outcome{res, err}
	}()
	// The fake never answers "hang"; wait for its marker so the cancel
	// cannot race ahead of the request.
	lcWaitFile(t, dir, "hang-armed", 5*time.Second)
	start := time.Now()
	cancel()
	select {
	case got := <-done:
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("cancel must return promptly, took %v", d)
		}
		if got.err == nil {
			t.Errorf("a cancelled call must fail, got %+v", got.res)
		}
		if got.res != nil && len(got.res.WavBytes) != 0 {
			t.Errorf("no audio may survive a cancelled call, got %d bytes", len(got.res.WavBytes))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Synthesize did not return within 5s of cancel")
	}

	// The child is untouched: a normal request still round-trips.
	okCtx, okCancel := lcContext()
	defer okCancel()
	res, err := rt.Synthesize(okCtx, "hi", "hi-IN", "")
	if err != nil {
		t.Fatalf("a cancel must not poison the dispatcher: %v", err)
	}
	if res.Empty || len(res.WavBytes) == 0 {
		t.Errorf("post-cancel synthesis returned %+v", res)
	}
}

// ---------------------------------------------------------------------------
// 6. stale-response isolation
// ---------------------------------------------------------------------------

// A response for a request that already timed out must be dropped, not
// handed to some other caller: the fake releases the late response for
// the first "dup" only once a second caller (the parked one) is
// registered, and never answers either of them again.
func TestLifecycle_TTS_LateResponseAfterTimeoutNotDeliveredToReusedID(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyTTSLateOnce})
	rt := NewAdapterSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()

	req := func(id, text string) ttsAdapterRequest {
		return ttsAdapterRequest{
			Op: "synthesize", RequestID: id, Text: text,
			Language: "hi-IN", Voice: "default", SampleRate: 44100,
		}
	}

	// 1. First "dup" times out while the child holds the response.
	if _, err := disp.send(req("R-dup", "dup"), "R-dup", 300*time.Millisecond, RequestContext{}); err == nil {
		t.Fatal("the first R-dup must time out")
	}

	// 2. Release the child, then park a concurrent caller: the orphan
	// response now reaches the pipe with the parked caller registered.
	lcWaitFile(t, dir, "armed", 5*time.Second)
	lcTouch(t, dir, "go")

	slowDone := make(chan struct {
		resp ttsAdapterResponse
		err  error
	}, 1)
	go func() {
		resp, err := disp.send(req("R-slow", "slow"), "R-slow", 2*time.Second, RequestContext{})
		slowDone <- struct {
			resp ttsAdapterResponse
			err  error
		}{resp, err}
	}()

	lcWaitFile(t, dir, "stale-sent", 5*time.Second)

	// 3. Barrier: a probe answered AFTER the stale line proves the
	// demultiplexer has consumed (and therefore must have dropped) the
	// orphan response before any id is re-registered.
	if _, err := disp.send(req("R-probe", "probe"), "R-probe", 5*time.Second, RequestContext{}); err != nil {
		t.Fatalf("barrier probe: %v", err)
	}

	// 4. The parked caller must burn its whole 2s budget with no
	// response: an orphan delivered to the wrong waiter would have
	// ended this call early.
	select {
	case got := <-slowDone:
		t.Fatalf("parked caller must not be answered by an orphaned response "+
			"(returned inside its 2s budget): audio=%q err=%v", got.resp.AudioB64, got.err)
	case <-time.After(1200 * time.Millisecond):
		// expected: the parked call is still waiting
	}
	select {
	case got := <-slowDone:
		if got.err == nil || got.resp.AudioB64 != "" {
			t.Errorf("parked caller: audio=%q err=%v", got.resp.AudioB64, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parked caller never returned")
	}

	// 5. Re-use the id. The child will never answer it, so anything
	// other than a timeout means the stale response was delivered.
	start := time.Now()
	resp, err := disp.send(req("R-dup", "dup"), "R-dup", 700*time.Millisecond, RequestContext{})
	if err == nil {
		t.Fatalf("a re-used id must not be answered by the stale response; got %q", resp.AudioB64)
	}
	if resp.AudioB64 != "" {
		t.Errorf("stale audio leaked into the next request: %q", resp.AudioB64)
	}
	if d := time.Since(start); d > 4*time.Second {
		t.Errorf("the re-used id must time out promptly, took %v", d)
	}

	// 6. The stream is still clean afterwards.
	if _, err := disp.send(req("R-final", "final"), "R-final", 5*time.Second, RequestContext{}); err != nil {
		t.Errorf("post-stale request: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 7. recovery after restart
// ---------------------------------------------------------------------------

// After the child dies the old runtime must stay failed (no zombie
// service), and a freshly loaded runtime on the same adapter must serve
// normally again.
func TestLifecycle_TTS_FreshRuntimeServesAfterChildDeath(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyTTS})
	rt := NewAdapterSubprocessRuntime(cfg)
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
	ctx, cancel := lcContext()
	if _, err := rt.Synthesize(ctx, "hi", "hi-IN", ""); !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("a dead child must surface ErrRuntimeUnavailable, got %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	lcWaitGone(t, pid, 5*time.Second)
	// The failed runtime must not resurrect itself.
	if _, err := rt.Synthesize(ctx, "hi", "hi-IN", ""); !errors.Is(err, ErrRuntimeClosed) {
		t.Errorf("closed runtime must stay closed, got %v", err)
	}
	cancel()

	// Restart: a new runtime on the same adapter serves again.
	cfg2, _ := lcConfig(t, lcOpts{src: lcPyTTS})
	rt2 := NewAdapterSubprocessRuntime(cfg2)
	t.Cleanup(func() { _ = rt2.Close() })
	if err := rt2.LoadModel(); err != nil {
		t.Fatalf("restart LoadModel: %v", err)
	}
	okCtx, okCancel := lcContext()
	defer okCancel()
	res, err := rt2.Synthesize(okCtx, "hi", "hi-IN", "")
	if err != nil {
		t.Fatalf("restarted runtime must serve: %v", err)
	}
	if res.Empty || len(res.WavBytes) == 0 {
		t.Errorf("restarted synthesis returned %+v", res)
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
func TestLifecycle_TTS_WorkerShutdownLeavesNoOrphanChild(t *testing.T) {
	cfg, dir := lcConfig(t, lcOpts{src: lcPyTTS, extraEnv: []string{"LC_IGNORE_SHUTDOWN=1"}})
	rt := NewAdapterSubprocessRuntime(cfg)
	if err := rt.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	pid := lcPID(t, filepath.Join(dir, "pid"))
	rt.mu.Lock()
	disp := rt.demux
	rt.mu.Unlock()

	catalog := buildCatalog(t)
	clock := NewStandaloneSourceVersionClock(7)
	w, err := New(Config{
		Inventory: Inventory{
			Parler: ParlerTTS{
				ModelID:  "ai4bharat/indic-parler-tts",
				License:  "Apache-2.0",
				Runtime:  "transformers-4.x",
				Hardware: "cpu",
				Revision: "rev-1",
				Voices:   []string{"default"},
			},
			SupportedLanguages: []string{"hi-IN"},
		},
		Runtime:     rt,
		Catalog:     catalog,
		Renderer:    templates.NewRenderer(catalog),
		Cache:       NewCodec(8*1024*1024, time.Hour, clock),
		Clock:       clock,
		QueueDepth:  2,
		MaxInFlight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if h := w.Health(); !h.Ready {
		t.Fatalf("worker must be ready while serving; got %+v", h)
	}
	if err := lcSignalAlive(pid); err != nil {
		t.Fatalf("child must be alive while the worker serves: %v", err)
	}

	// Shutdown must not block on a child that refuses to cooperate.
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- w.Shutdown() }()
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
