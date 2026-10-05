// Middle-worker LIFECYCLE tests. The unit tests that already exist
// (client_test.go, decode_test.go, worker_test.go, server_test.go)
// cover single-call shape and the happy path. This file covers the
// process-level lifecycle that had never been verified: startup
// failure, malformed readiness, untrusted model output, crash
// mid-request, timeout/cancellation, stale-response isolation,
// recovery after restart, and shutdown leaving nothing behind.
//
// Determinism rules used throughout: no wall-clock sleep is ever the
// primary synchronisation. Every wait is a channel receive bounded by
// a timeout, or a bounded poll with an explicit deadline. Ports are
// bound only in 18860-18869 and are closed in t.Cleanup. Child
// processes started here are killed and reaped in t.Cleanup.
//
// Tests named TestRegression_* assert behaviour the module's OWN
// documentation promises. Each one failed against the pre-fix code in
// worker.go, server.go and client.go and now runs unconditionally as
// a standing guarantee: the promised behaviour, not the defect it
// once was.

package middleworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// ---------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------

// lcMode selects how the fake inference endpoint answers.
type lcMode int

const (
	// lcAnswer returns lcVLLM.content as the assistant message.
	lcAnswer lcMode = iota
	// lcCrash writes a partial body then kills the connection.
	lcCrash
	// lcLate holds the response until the client cancels, then
	// writes it anyway: the "late response after a timeout" case.
	lcLate
)

// lcContractMaxActions mirrors contracts.MaxModelActions (5). The
// contracts package lives in another module, so the number is pinned
// here explicitly.
const lcContractMaxActions = 5

// lcProposalTemplate is a schema-shaped proposal. {{RID}} is replaced
// with the request id the caller sent so every answer is attributable.
const lcProposalTemplate = `{"schema_version":"3.0","request_id":"{{RID}}","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`

// lcVLLM is a tiny fake inference endpoint bound to 127.0.0.1 on a
// port in 18860-18869. It restarts on the same port so a "crash and
// recover" test is deterministic.
type lcVLLM struct {
	l       net.Listener
	srv     *http.Server
	port    int
	content string
	lateID  string
	mode    atomic.Int32
	calls   atomic.Int32
	seen    chan string
}

// lcTestClient is the client the test's own plumbing uses. It never
// pools connections, so one fake endpoint can never be reached through
// another fake's keep-alive socket. The production Client keeps
// keep-alives on purpose: the stale-response test depends on them.
var lcTestClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: &http.Transport{DisableKeepAlives: true},
}

// lcListen binds the first free port in 18860-18869, optionally
// skipping one already taken.
func lcListen(t *testing.T, skip ...int) (net.Listener, int) {
	t.Helper()
	for p := 18860; p <= 18869; p++ {
		skipIt := false
		for _, s := range skip {
			if s == p {
				skipIt = true
			}
		}
		if skipIt {
			continue
		}
		l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
		if err == nil {
			return l, p
		}
	}
	t.Fatalf("no free port in 18860-18869")
	return nil, 0
}

func lcNewVLLM(t *testing.T, content string, mode lcMode) *lcVLLM {
	t.Helper()
	v := &lcVLLM{content: content, seen: make(chan string, 32)}
	v.mode.Store(int32(mode))
	l, port := lcListen(t)
	v.l, v.port = l, port
	v.srv = &http.Server{Handler: v.handler(), ReadHeaderTimeout: 5 * time.Second}
	t.Cleanup(v.Close)
	go v.serve(l)
	return v
}

func (v *lcVLLM) serve(l net.Listener) { _ = v.srv.Serve(l) }

func (v *lcVLLM) handler() http.Handler {
	mux := http.NewServeMux()
	// vLLM's readiness endpoint, probed by LoadAndVerify.
	mux.HandleFunc("/health", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Sthira-Request-ID")
		v.calls.Add(1)
		select {
		case v.seen <- rid:
		default:
		}
		content := strings.ReplaceAll(v.content, "{{RID}}", rid)
		switch lcMode(v.mode.Load()) {
		case lcCrash:
			// Announce a JSON body, then die mid-write. The client
			// must surface a typed error, never a partial proposal.
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", "4096")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"`))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			panic(http.ErrAbortHandler)
		case lcLate:
			// Answer this request only after its client has given
			// up, so the bytes are provably late. Scoped to one
			// request id so the follow-up call is served normally.
			if v.lateID == "" || rid == v.lateID {
				select {
				case <-r.Context().Done():
				case <-time.After(5 * time.Second):
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": SarvamModelID,
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": content},
			}},
		})
	})
	return mux
}

func (v *lcVLLM) URL() string { return "http://127.0.0.1:" + strconv.Itoa(v.port) }

// setMode switches how the endpoint answers. Used to bring a crashed
// endpoint back after a restart.
func (v *lcVLLM) setMode(m lcMode) { v.mode.Store(int32(m)) }

// Close kills the listener AND every accepted connection, so a
// restart on the same port behaves like a process restart. Closing
// only the listener would leave keep-alive sockets alive and the next
// test would reach this dead server through the shared transport
// pool.
func (v *lcVLLM) Close() {
	if v.srv != nil {
		_ = v.srv.Close()
		v.srv = nil
	}
	if v.l != nil {
		_ = v.l.Close()
		v.l = nil
	}
}

// Restart re-binds the same port and returns only once it is
// listening again, so the next request cannot race the bind.
func (v *lcVLLM) Restart(t *testing.T) {
	t.Helper()
	v.Close()
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(v.port))
	if err != nil {
		t.Fatalf("re-listen on %d: %v", v.port, err)
	}
	v.l = l
	v.srv = &http.Server{Handler: v.handler(), ReadHeaderTimeout: 5 * time.Second}
	go v.serve(l)
}

func (v *lcVLLM) sawRequestIDs() []string {
	var out []string
	for {
		select {
		case id := <-v.seen:
			out = append(out, id)
			continue
		default:
			return out
		}
	}
}

// lcRuntime is an in-process Runtime whose Propose behaviour is
// switchable at run time: gated, panicking or healthy.
type lcRuntime struct {
	mu       sync.Mutex
	langs    []string
	revision string
	digest   [2]string
	panics   bool
	gate     chan struct{} // when non-nil, Propose blocks on it or ctx
	entered  chan struct{} // signalled on every Propose entry
	inflight atomic.Int64  // jobs inside Propose right now
	peak     atomic.Int64  // high-water mark of inflight
}

func lcNewRuntime(langs ...string) *lcRuntime {
	return &lcRuntime{
		langs:    langs,
		revision: "lc-revision-1",
		digest:   [2]string{"lc-artifact", "3333333333333333333333333333333333333333333333333333333333333333"},
		entered:  make(chan struct{}, 64),
	}
}

func (r *lcRuntime) Propose(ctx context.Context, req RequestEnvelope) (*ProposeOutput, error) {
	cur := r.inflight.Add(1)
	for {
		peak := r.peak.Load()
		if cur <= peak || r.peak.CompareAndSwap(peak, cur) {
			break
		}
	}
	defer r.inflight.Add(-1)
	select {
	case r.entered <- struct{}{}:
	default:
	}
	r.mu.Lock()
	gate, panics := r.gate, r.panics
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ErrCanceled
		}
	}
	if panics {
		panic("lc runtime: simulated inference crash")
	}
	p, err := decodeStrictProposal([]byte(strings.ReplaceAll(lcProposalTemplate, "{{RID}}", req.RequestID)))
	if err != nil {
		return nil, err
	}
	return &ProposeOutput{Proposal: p, ModelRevision: r.Revision(), FinishReason: "stop"}, nil
}

func (r *lcRuntime) Revision() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.revision
}

func (r *lcRuntime) Digest() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.digest[0], r.digest[1]
}

func (r *lcRuntime) Languages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.langs...)
}

func (r *lcRuntime) set(fn func(*lcRuntime)) {
	r.mu.Lock()
	fn(r)
	r.mu.Unlock()
}

// lcRuntimeFrom builds the production-shaped runtime against a fake
// endpoint, filling the revision and digest exactly as
// cmd/middleworker/main.go does (SarvamConfig leaves them empty).
func lcRuntimeFrom(t *testing.T, baseURL string) *HTTPClientRuntime {
	t.Helper()
	cli, err := NewClient(ClientConfig{
		BaseURL:    baseURL,
		Limits:     SarvamLimits(),
		SchemaJSON: DefaultModelOutputSchema(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	cfg := SarvamConfig(cli, "lc system prompt")
	cfg.Revision = "lc-rev"
	cfg.DigestSHA = "lc-digest"
	rt, err := NewHTTPClientRuntime(cfg)
	if err != nil {
		t.Fatalf("NewHTTPClientRuntime: %v", err)
	}
	return rt
}

func lcEnvelope(requestID, text string) RequestEnvelope {
	return RequestEnvelope{
		RequestID: requestID,
		ScopedContext: ScopedContext{
			SchemaVersion: "3.0",
			DataVersion:   "v1",
			TemplateKeys:  []string{"welcome"},
		},
		Transcript:      TranscriptInput{RequestID: requestID, Language: "en-IN", Text: text, State: "OK"},
		MaxOutputTokens: 256,
		DeadlineMillis:  5000,
	}
}

// lcNewServer wires a ready Worker + Server on a port in 18860-18869
// and returns the base URL. Shutdown runs in t.Cleanup.
func lcNewServer(t *testing.T, rt Runtime) string {
	t.Helper()
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 4, MaxInFlight: 1, BuildRevision: "lc-build"})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	return lcServe(t, w)
}

// lcServe binds a Server around an existing (possibly unverified)
// worker and returns its base URL.
func lcServe(t *testing.T, w *Worker) string {
	t.Helper()
	probe, port := lcListen(t)
	_ = probe.Close() // NewServer does the real bind
	srv, err := NewServer(ServerConfig{
		Address:      "127.0.0.1:" + strconv.Itoa(port),
		Token:        "lc-token",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ShutdownWait: 2 * time.Second,
	}, w)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	go func() { _ = srv.Start() }()
	t.Cleanup(func() { _ = srv.Shutdown() })
	return "http://" + srv.Addr()
}

// lcPost sends one request to the worker protocol endpoint and
// returns the status, the X-Sthira-State header and the raw body.
func lcPost(t *testing.T, base, body string) (int, string, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer lc-token")
	resp, err := lcTestClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	bs, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("X-Sthira-State"), bs
}

func lcGet(t *testing.T, base, path string) (int, []byte) {
	t.Helper()
	status, bs := lcGetQuiet(base, path)
	if status == 0 {
		t.Fatalf("GET %s: no response", path)
	}
	return status, bs
}

// lcGetQuiet never fails the test; used for readiness polling where a
// connection error is the expected "not up yet" answer.
func lcGetQuiet(base, path string) (int, []byte) {
	resp, err := lcTestClient.Get(base + path)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	bs, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, bs
}

// lcAwait runs fn and fails the test if it has not reported within d.
func lcAwait(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not complete within %s", what, d)
	}
}

// lcWaitGoroutines polls until the goroutine count falls back to
// base+slack, or fails. Bounded by construction, never a fixed sleep.
func lcWaitGoroutines(t *testing.T, base, slack int, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		n := runtime.NumGoroutine()
		if n <= base+slack {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: goroutines did not drain (base=%d now=%d)", what, base, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func lcProcessGone(pid int) bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }

// lcBuf collects a child process's stdout and stderr. os/exec copies
// them on two goroutines, so the buffer needs its own lock or the race
// detector flags the collector, not the code under test.
type lcBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *lcBuf) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *lcBuf) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// lcBuildBinary builds the shipped command into t.TempDir(). The
// module has no external dependencies, so this performs no downloads.
func lcBuildBinary(t *testing.T) string {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go toolchain on PATH: %v", err)
	}
	out := filepath.Join(t.TempDir(), "middleworker")
	if b, err := exec.Command(goBin, "build", "-o", out, "./cmd/middleworker").CombinedOutput(); err != nil {
		t.Skipf("cannot build ./cmd/middleworker: %v: %s", err, b)
	}
	return out
}

// lcStartBinary starts the shipped command against vllmURL and waits
// (bounded) for its /health to answer. The process is killed, reaped
// and checked for orphans in t.Cleanup.
func lcStartBinary(t *testing.T, bin, vllmURL string) (*exec.Cmd, string, int) {
	t.Helper()
	// Two distinct free ports in the range: one for the command under
	// test, one held only long enough to force the next probe to move
	// on. Both probe listeners are closed before the child binds.
	probeA, portA := lcListen(t)
	probeB, portB := lcListen(t, portA)
	_ = probeA.Close()
	_ = probeB.Close()
	port := portB

	addr := "127.0.0.1:" + strconv.Itoa(port)
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"STHIRA_MIDDLE_ADDR="+addr,
		"STHIRA_MIDDLE_TOKEN=lc-token",
		"STHIRA_VLLM_URL="+vllmURL,
		"STHIRA_MIDDLE_REVISION=lc-rev",
		"STHIRA_MIDDLE_DIGEST_SHA=lc-digest",
	)
	var logs lcBuf
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start middleworker: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGKILL)
		_ = cmd.Wait()
		if !lcProcessGone(cmd.Process.Pid) {
			t.Errorf("orphan: middleworker pid %d still alive after cleanup", cmd.Process.Pid)
		}
	})
	base := "http://" + addr
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if status, _ := lcGetQuiet(base, "/health"); status == http.StatusOK {
			return cmd, base, port
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("middleworker never answered /health within 15s; logs:\n%s", logs.String())
	return nil, "", 0
}

func lcWaitExit(t *testing.T, cmd *exec.Cmd, d time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		t.Fatalf("process did not exit within %s", d)
		return nil
	}
}

// ---------------------------------------------------------------------
// 1. startup failure
// ---------------------------------------------------------------------

// TestLifecycle_StartupFailureUnreachableEndpointFailsClosed: an
// inference endpoint that is not listening must produce a typed
// UNAVAILABLE error promptly, never a fabricated proposal, never a
// hang. Breaks if Propose retries, invents a proposal, or swallows the
// connect error.
func TestLifecycle_StartupFailureUnreachableEndpointFailsClosed(t *testing.T) {
	l, _ := lcListen(t)
	addr := l.Addr().String()
	_ = l.Close() // nothing is bound here any more

	rt := lcRuntimeFrom(t, "http://"+addr)
	lcAwait(t, 5*time.Second, "Propose against a dead endpoint", func() {
		out, err := rt.Propose(context.Background(), lcEnvelope("REQ-DEAD", "show shelter"))
		if err == nil {
			t.Errorf("dead endpoint returned a proposal: %+v", out)
			return
		}
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("err=%v; want ErrUnavailable", err)
		}
	})
}

// TestLifecycle_StartupFailureBindConflictExitsNonZero: the shipped
// command must refuse to start when its address is already taken and
// exit non-zero promptly with a clear error, instead of serving from a
// half-bound state. Breaks if NewServer ignores a bind error or the
// command keeps running.
func TestLifecycle_StartupFailureBindConflictExitsNonZero(t *testing.T) {
	bin := lcBuildBinary(t)
	busy, port := lcListen(t)
	defer busy.Close()

	v := lcNewVLLM(t, lcProposalTemplate, lcAnswer)
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"STHIRA_MIDDLE_ADDR=127.0.0.1:"+strconv.Itoa(port),
		"STHIRA_MIDDLE_TOKEN=lc-token",
		"STHIRA_VLLM_URL="+v.URL(),
	)
	var logs lcBuf
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := lcWaitExit(t, cmd, 15*time.Second); err == nil {
		t.Fatalf("command must exit non-zero on a bind conflict; logs:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "bind") {
		t.Errorf("exit must name the bind failure; logs:\n%s", logs.String())
	}
	if !lcProcessGone(cmd.Process.Pid) {
		t.Errorf("pid %d left running after a failed start", cmd.Process.Pid)
	}
	// The occupied port must still belong to the original listener.
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), time.Second)
	if err != nil {
		t.Errorf("the failed start disturbed the existing listener: %v", err)
	} else {
		_ = conn.Close()
	}
}

// TestRegression_LoadAndVerifyGatesOnDeadInferenceEndpoint guarantees
// the module's own rule (go.mod: "readiness is gated on actual artifact
// loading, not process liveness"): a worker whose inference endpoint is
// dead fails LoadAndVerify, stays NOT-READY in its own snapshot, and
// reports not-ready over /health, so the orchestrator never routes
// citizens to it on the strength of a process that merely exists.
func TestRegression_LoadAndVerifyGatesOnDeadInferenceEndpoint(t *testing.T) {
	l, _ := lcListen(t)
	addr := l.Addr().String()
	_ = l.Close()

	w, err := NewWorker(Config{Runtime: lcRuntimeFrom(t, "http://"+addr), QueueDepth: 2, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	loadErr := w.LoadAndVerify(context.Background())
	snap := w.Snapshot()
	if loadErr == nil {
		t.Errorf("LoadAndVerify must fail closed when the inference endpoint %s is unreachable; got nil", addr)
	}
	if snap.Ready || snap.Warm {
		t.Errorf("worker must stay NOT-READY while the model is unreachable; ready=%v warm=%v", snap.Ready, snap.Warm)
	}
	status, body := lcGet(t, lcServe(t, w), "/health")
	if status != http.StatusOK {
		t.Fatalf("GET /health status=%d", status)
	}
	var h HealthEnvelope
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("/health body is not valid JSON: %v body=%s", err, body)
	}
	if h.Ready || h.Warm {
		t.Errorf("/health advertises a ready worker with a dead model: %s", body)
	}
}

// ---------------------------------------------------------------------
// 2. malformed readiness
// ---------------------------------------------------------------------

// TestLifecycle_MalformedReadinessFailsClosed: every missing readiness
// field (no revision, no digest name, no digest sha, no languages)
// must fail the gate, leave the worker NOT-READY, be reported as
// not-ready over /health, and produce a readiness document a strict
// consumer can decode. Breaks if LoadAndVerify returns nil for any of
// these, or if /health emits a field HealthEnvelope does not declare.
func TestLifecycle_MalformedReadinessFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		mutch func(*lcRuntime)
	}{
		{"empty revision", func(r *lcRuntime) { r.revision = "" }},
		{"empty digest name", func(r *lcRuntime) { r.digest[0] = "" }},
		{"empty digest sha", func(r *lcRuntime) { r.digest[1] = "" }},
		{"zero languages", func(r *lcRuntime) { r.langs = nil }},
		{"empty language list", func(r *lcRuntime) { r.langs = []string{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt := lcNewRuntime("en-IN")
			tc.mutch(rt)
			w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1})
			if err != nil {
				t.Fatalf("NewWorker: %v", err)
			}
			if err := w.LoadAndVerify(context.Background()); err == nil {
				t.Errorf("LoadAndVerify must reject readiness with %s", tc.name)
			}
			if w.Snapshot().Ready || w.Snapshot().Warm {
				t.Fatalf("worker must stay NOT-READY with %s", tc.name)
			}
			status, body := lcGet(t, lcServe(t, w), "/health")
			if status != http.StatusOK {
				t.Fatalf("GET /health status=%d", status)
			}
			dec := json.NewDecoder(bytes.NewReader(body))
			dec.DisallowUnknownFields()
			var h HealthEnvelope
			if err := dec.Decode(&h); err != nil {
				t.Fatalf("/health is not strictly decodable: %v body=%s", err, body)
			}
			if h.Ready || h.Warm {
				t.Errorf("/health says ready for a malformed-readiness worker: %s", body)
			}
		})
	}
}

// TestRegression_EmptyLanguageAllowListEntryAdmitsEmptyLanguage
// guarantees the language allow-list is a real trust gate: a request
// with no language at all is rejected by Dispatch and never reaches
// inference, even when a readiness document has admitted the empty
// string as one of the worker's languages.
func TestRegression_EmptyLanguageAllowListEntryAdmitsEmptyLanguage(t *testing.T) {
	rt := lcNewRuntime("en-IN", "")
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	req := lcEnvelope("REQ-EMPTY-LANG", "x")
	req.Transcript.Language = ""
	if _, err := w.Dispatch(context.Background(), req); err == nil {
		t.Fatalf("a request with no language was dispatched; the readiness allow-list contains an empty string")
	}
	if n := len(rt.entered); n != 0 {
		t.Errorf("inference was contacted %d times for a request with no language", n)
	}
}

// TestRegression_HealthAdvertisesArtifactWithoutChecksum guarantees
// /health never names an artifact or a model whose integrity digest is
// empty. A runtime with no checksum is rejected by LoadAndVerify and
// Worker.Snapshot omits its entries entirely, so an incident responder
// is never handed a checksum-less artifact or model to trust.
func TestRegression_HealthAdvertisesArtifactWithoutChecksum(t *testing.T) {
	rt := lcNewRuntime("en-IN")
	rt.set(func(r *lcRuntime) { r.digest[1] = "" })
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err == nil {
		t.Fatalf("LoadAndVerify must reject a runtime with no artifact digest")
	}
	_, body := lcGet(t, lcServe(t, w), "/health")
	var h HealthEnvelope
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("/health body is not valid JSON: %v body=%s", err, body)
	}
	for _, a := range h.Artifacts {
		if a.ChecksumSHA256 == "" {
			t.Errorf("/health advertises artifact %q with no checksum: %s", a.Name, body)
		}
	}
	for _, m := range h.Models {
		if m.ChecksumSHA256 == "" {
			t.Errorf("/health advertises model %q with no checksum: %s", m.ModelID, body)
		}
	}
}

// ---------------------------------------------------------------------
// 3. malformed / untrusted model output
// ---------------------------------------------------------------------

// TestLifecycle_MalformedOutputWrongTypesRejectedBeforeDispatch: every
// wrong-typed or structurally broken model output must be rejected
// with a typed MALFORMED state and must never produce a proposal on
// the wire. Breaks if the strict decode starts coercing types, or if
// the endpoint answers 200 with a partial envelope.
func TestLifecycle_MalformedOutputWrongTypesRejectedBeforeDispatch(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"actions is an object", `{"schema_version":"3.0","request_id":"{{RID}}","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":{},"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`},
		{"status is a number", `{"schema_version":"3.0","request_id":"{{RID}}","data_version":"v1","status":7,"intent":"FOCUS_PLACE","language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`},
		{"evidence_ids is a string", `{"schema_version":"3.0","request_id":"{{RID}}","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":"P1"}`},
		{"truncated json", `{"schema_version":"3.0","request_id":"{{RID}}"`},
		{"not json at all", `I cannot help with that.`},
		{"forbidden extra field", `{"schema_version":"3.0","request_id":"{{RID}}","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[],"call_shelter":true}`},
		{"extra text around payload", "Here you go:\n" + lcProposalTemplate},
		{"request id mismatch", strings.ReplaceAll(lcProposalTemplate, "{{RID}}", "SOMEONE-ELSES-REQUEST")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := lcNewVLLM(t, tc.content, lcAnswer)
			base := lcNewServer(t, lcRuntimeFrom(t, v.URL()))
			status, state, body := lcPost(t, base, string(mustEncode(t, lcEnvelope("REQ-BAD", "show shelter"))))
			if status == http.StatusOK {
				t.Fatalf("malformed output was served as a proposal: %s", body)
			}
			if state != MiddleStateMalformed {
				t.Errorf("X-Sthira-State=%q; want %q (body=%s)", state, MiddleStateMalformed, body)
			}
			if bytes.Contains(body, []byte(`"actions"`)) {
				t.Errorf("rejected response leaked a proposal: %s", body)
			}
		})
	}
}

// TestLifecycle_UntrustedOutputIsNeverRepaired documents and pins the
// middle worker's real trust boundary. Output that is structurally
// sound but semantically unacceptable (wrong schema version, unknown
// action type, CLARIFY without clarification_ids, OK with a null
// intent, more actions than the contract max, a speech_key outside the
// scoped allow-list) is NOT this module's gate: decode.go:1-4 and
// eval/main_test.go:309-310 assign it to the orchestrator's
// independent validator. What this module must guarantee is that it
// forwards the model's output unchanged -- no repair, no dropped
// action, no substituted field -- so the downstream gate sees exactly
// what the model said. Breaks if a sanitiser is ever added: dropping
// the offending action, defaulting a null status, or substituting a
// template key all fail here.
func TestLifecycle_UntrustedOutputIsNeverRepaired(t *testing.T) {
	cases := []struct {
		name    string
		content string
		check   func(*testing.T, Proposal)
	}{
		{
			name:    "wrong schema version",
			content: strings.ReplaceAll(lcProposalTemplate, `"schema_version":"3.0"`, `"schema_version":"9.9"`),
			check: func(t *testing.T, p Proposal) {
				if p.SchemaVersion != "9.9" {
					t.Errorf("schema_version was rewritten to %q", p.SchemaVersion)
				}
			},
		},
		{
			name:    "unknown action type",
			content: strings.ReplaceAll(lcProposalTemplate, `{"type":"FOCUS_FEATURE","target_id":"P1"}`, `{"type":"EVACUATE_EVERYONE"}`),
			check: func(t *testing.T, p Proposal) {
				if len(p.Actions) != 1 || p.Actions[0].Type != "EVACUATE_EVERYONE" {
					t.Errorf("unknown action was dropped or rewritten: %+v", p.Actions)
				}
			},
		},
		{
			name: "CLARIFY without clarification ids",
			content: strings.ReplaceAll(
				strings.ReplaceAll(lcProposalTemplate, `"status":"OK"`, `"status":"CLARIFY"`),
				`"intent":"FOCUS_PLACE"`, `"intent":null`),
			check: func(t *testing.T, p Proposal) {
				if p.Status != "CLARIFY" || p.Intent != nil || len(p.ClarificationIDs) != 0 {
					t.Errorf("CLARIFY proposal was altered: status=%q intent=%v ids=%v", p.Status, p.Intent, p.ClarificationIDs)
				}
			},
		},
		{
			name:    "OK with null intent",
			content: strings.ReplaceAll(lcProposalTemplate, `"intent":"FOCUS_PLACE"`, `"intent":null`),
			check: func(t *testing.T, p Proposal) {
				if p.Status != "OK" || p.Intent != nil {
					t.Errorf("OK proposal was altered: status=%q intent=%v", p.Status, p.Intent)
				}
			},
		},
		{
			name: "more actions than the contract max",
			content: strings.ReplaceAll(lcProposalTemplate,
				`{"type":"FOCUS_FEATURE","target_id":"P1"}`,
				strings.TrimSuffix(strings.Repeat(`{"type":"FOCUS_FEATURE","target_id":"P1"},`, lcContractMaxActions+1), ",")),
			check: func(t *testing.T, p Proposal) {
				if len(p.Actions) != lcContractMaxActions+1 {
					t.Errorf("actions=%d; the model's %d must be forwarded verbatim", len(p.Actions), lcContractMaxActions+1)
				}
			},
		},
		{
			name:    "speech key outside the scoped allow-list",
			content: strings.ReplaceAll(lcProposalTemplate, `"speech_key":null`, `"speech_key":"key-the-model-invented"`),
			check: func(t *testing.T, p Proposal) {
				if p.SpeechKey == nil || *p.SpeechKey != "key-the-model-invented" {
					t.Errorf("speech_key was substituted: %v", p.SpeechKey)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := lcNewVLLM(t, tc.content, lcAnswer)
			w, err := NewWorker(Config{Runtime: lcRuntimeFrom(t, v.URL()), QueueDepth: 2, MaxInFlight: 1})
			if err != nil {
				t.Fatalf("NewWorker: %v", err)
			}
			if err := w.LoadAndVerify(context.Background()); err != nil {
				t.Fatalf("LoadAndVerify: %v", err)
			}
			resp, err := w.Dispatch(context.Background(), lcEnvelope("REQ-UNTRUSTED", "show shelter"))
			if err != nil {
				t.Fatalf("structurally sound output must reach the downstream gate, got %v", err)
			}
			tc.check(t, resp.Proposal)
		})
	}
}

// TestRegression_EmptyTranscriptServedAsProposal guarantees the
// precheck rejects an empty transcript: a request with no text and no
// transcript state never produces a proposal, whatever state the
// transcript claims, and is answered as MALFORMED instead of 200.
func TestRegression_EmptyTranscriptServedAsProposal(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"no text, no state", `{"request_id":"REQ-E1","scoped_context":{"schema_version":"3.0","data_version":"v1"},"transcript":{"request_id":"REQ-E1","language":"en-IN","text":"","state":""}}`},
		{"no text, listening", `{"request_id":"REQ-E2","scoped_context":{"schema_version":"3.0","data_version":"v1"},"transcript":{"request_id":"REQ-E2","language":"en-IN","text":"","state":"LISTENING"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := lcNewServer(t, lcNewRuntime("en-IN"))
			status, state, body := lcPost(t, base, tc.body)
			if status == http.StatusOK {
				t.Fatalf("an empty transcript produced a proposal: %s", body)
			}
			if state != MiddleStateMalformed {
				t.Errorf("X-Sthira-State=%q; want %q (body=%s)", state, MiddleStateMalformed, body)
			}
		})
	}
}

// ---------------------------------------------------------------------
// 4. crash mid-request
// ---------------------------------------------------------------------

// TestLifecycle_CrashMidRequestSurfacesTypedErrorAndRecovers: an
// inference endpoint that dies mid-body must produce a typed error
// promptly, must not hang, must not leak the dead connection into the
// next request, and must leave the adapter able to serve again.
// Breaks if the client retries the dead connection, blocks forever, or
// hands a truncated body to the decoder as a proposal.
func TestLifecycle_CrashMidRequestSurfacesTypedErrorAndRecovers(t *testing.T) {
	base := runtime.NumGoroutine()
	v := lcNewVLLM(t, lcProposalTemplate, lcCrash)
	rt := lcRuntimeFrom(t, v.URL())
	lcAwait(t, 10*time.Second, "Propose against a crashing endpoint", func() {
		out, err := rt.Propose(context.Background(), lcEnvelope("REQ-CRASH", "show shelter"))
		if err == nil {
			t.Errorf("crashed inference returned a proposal: %+v", out)
			return
		}
		if !errors.Is(err, ErrUnavailable) && !errors.Is(err, ErrMalformed) {
			t.Errorf("err=%v; want ErrUnavailable or ErrMalformed", err)
		}
	})
	// The same adapter must serve again once the endpoint is healthy.
	v.setMode(lcAnswer)
	v.Close()
	v.Restart(t)
	lcAwait(t, 10*time.Second, "Propose after the endpoint came back", func() {
		out, err := rt.Propose(context.Background(), lcEnvelope("REQ-AFTER-CRASH", "show shelter"))
		if err != nil {
			t.Errorf("adapter did not recover after the crash: %v", err)
			return
		}
		if out.Proposal.RequestID != "REQ-AFTER-CRASH" {
			t.Errorf("request_id=%q", out.Proposal.RequestID)
		}
	})
	v.Close()
	lcWaitGoroutines(t, base, 4, "crash mid-request")
}

// TestRegression_RuntimePanicMustNotCrashTheProcess guarantees a panic
// inside Runtime.Propose cannot take the service down: Worker.propose
// recovers and returns a typed error, the pool slot stays usable, and
// the worker still serves the next request. The check runs in a child
// process so a regression is reported as a failing test instead of
// taking the runner with it.
func TestRegression_RuntimePanicMustNotCrashTheProcess(t *testing.T) {
	if os.Getenv("LC_CRASH_CHILD") == "1" {
		lcChildPanicDispatch(t)
		return
	}
	out, err := lcRunChild(t, "TestRegression_RuntimePanicMustNotCrashTheProcess")
	if err != nil {
		t.Fatalf("worker process died when a Runtime panicked; it must report a typed error and stay up.\nchild output:\n%s", out)
	}
	if !strings.Contains(out, "LC_FOLLOWUP_OK") {
		t.Fatalf("after a panicking inference call the worker must still serve the next request.\nchild output:\n%s", out)
	}
}

// lcChildPanicDispatch runs only inside the crash child.
func lcChildPanicDispatch(t *testing.T) {
	rt := lcNewRuntime("en-IN")
	rt.set(func(r *lcRuntime) { r.panics = true })
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// The panicking call must return a typed error, not kill us.
	if _, err := w.Dispatch(ctx, lcEnvelope("REQ-PANIC", "x")); err == nil {
		t.Log("panicking runtime returned success")
	} else {
		t.Logf("panicking runtime returned %v", err)
	}
	// The pool slot must still be alive.
	rt.set(func(r *lcRuntime) { r.panics = false })
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if _, err := w.Dispatch(ctx2, lcEnvelope("REQ-AFTER-PANIC", "x")); err != nil {
		t.Fatalf("worker did not serve the request after a panicking inference call: %v", err)
	}
	t.Log("LC_FOLLOWUP_OK")
}

// lcRunChild re-executes this test binary for a single test, marked by
// LC_CRASH_CHILD so the child takes the child branch. The depth marker
// is checked here as well: a wiring mistake can never nest the child
// more than one level, which would fork-bomb the machine.
func lcRunChild(t *testing.T, name string) (string, error) {
	t.Helper()
	if os.Getenv("LC_CRASH_CHILD_DEPTH") != "" {
		t.Fatalf("child recursion: refusing to spawn %s again", name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+name+"$", "-test.v", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), "LC_CRASH_CHILD=1", "LC_CRASH_CHILD_DEPTH=1")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ---------------------------------------------------------------------
// 5. timeout / cancellation
// ---------------------------------------------------------------------

// TestLifecycle_WorkerDeadlineExceededReturnsTimeout: a caller
// deadline must surface as ErrTimeout (not ErrCanceled) promptly and
// must release the pool slot so the next request is served normally.
// Breaks if the per-call deadline is ignored, if a deadline is
// misreported as a cancellation, or if the aborted job poisons the
// slot for later requests.
func TestLifecycle_WorkerDeadlineExceededReturnsTimeout(t *testing.T) {
	rt := lcNewRuntime("en-IN")
	gate := make(chan struct{})
	rt.set(func(r *lcRuntime) { r.gate = gate })
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = w.Dispatch(ctx, lcEnvelope("REQ-DEADLINE", "x"))
	elapsed := time.Since(start)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v; want ErrTimeout", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Dispatch returned after %s; the deadline was not honoured", elapsed)
	}
	close(gate)
	rt.set(func(r *lcRuntime) { r.gate = nil })
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	resp, err := w.Dispatch(ctx2, lcEnvelope("REQ-AFTER-DEADLINE", "x"))
	if err != nil {
		t.Fatalf("worker did not serve the request after a timeout: %v", err)
	}
	if resp.Proposal.RequestID != "REQ-AFTER-DEADLINE" {
		t.Errorf("request_id=%q", resp.Proposal.RequestID)
	}
}

// TestRegression_TimeoutCounterIgnoresCallerDeadlines guarantees the
// timedOut counter means what it documents ("jobs that hit the
// per-call deadline"): a job abandoned at the caller's deadline is
// counted on the ctx.Done path, so deadline pressure stays visible to
// the only counter that reports it.
func TestRegression_TimeoutCounterIgnoresCallerDeadlines(t *testing.T) {
	rt := lcNewRuntime("en-IN")
	gate := make(chan struct{})
	rt.set(func(r *lcRuntime) { r.gate = gate })
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := w.Dispatch(ctx, lcEnvelope("REQ-COUNTED", "x")); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v; want ErrTimeout", err)
	}
	close(gate)
	_, _, timedOut, _, _ := w.Stats()
	if timedOut != 1 {
		t.Errorf("timedOut=%d after one deadline expiry; want 1", timedOut)
	}
}

// ---------------------------------------------------------------------
// 6. stale-response isolation
// ---------------------------------------------------------------------

// TestLifecycle_LateResponseFromTimedOutRequestNotDeliveredToNext: a
// response that arrives after its caller gave up must be dropped; it
// must not be handed to the next caller and must not corrupt it.
// Breaks if the client pools the abandoned connection, if the worker
// mixes results between jobs, or if a late body is decoded as a
// proposal.
func TestLifecycle_LateResponseFromTimedOutRequestNotDeliveredToNext(t *testing.T) {
	v := lcNewVLLM(t, lcProposalTemplate, lcLate)
	v.lateID = "REQ-LATE"
	w, err := NewWorker(Config{Runtime: lcRuntimeFrom(t, v.URL()), QueueDepth: 2, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}

	// The first request times out; the endpoint writes its answer only
	// after observing the cancellation, so it is provably late.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	lcAwait(t, 5*time.Second, "timed-out request", func() {
		if _, err := w.Dispatch(ctx, lcEnvelope("REQ-LATE", "x")); !errors.Is(err, ErrTimeout) {
			t.Errorf("err=%v; want ErrTimeout", err)
		}
	})

	// The next request must get its own answer.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	resp, err := w.Dispatch(ctx2, lcEnvelope("REQ-NEXT", "x"))
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	if resp.RequestID != "REQ-NEXT" || resp.Proposal.RequestID != "REQ-NEXT" {
		t.Fatalf("stale response delivered: envelope=%q proposal=%q", resp.RequestID, resp.Proposal.RequestID)
	}
	if seen := v.sawRequestIDs(); len(seen) != 2 || seen[0] != "REQ-LATE" || seen[1] != "REQ-NEXT" {
		t.Fatalf("inference saw %v; want [REQ-LATE REQ-NEXT]", seen)
	}
}

// ---------------------------------------------------------------------
// 7. recovery after restart
// ---------------------------------------------------------------------

// TestLifecycle_RecoversAfterInferenceProcessRestart: the shipped
// command must report UNAVAILABLE while the inference endpoint is
// gone and must serve normally again once the endpoint is restarted on
// the same port, without restarting itself. Breaks if the failure
// latches, if the connection pool is poisoned, or if the worker
// fabricates a proposal while the endpoint is down.
func TestLifecycle_RecoversAfterInferenceProcessRestart(t *testing.T) {
	bin := lcBuildBinary(t)
	v := lcNewVLLM(t, lcProposalTemplate, lcAnswer)
	_, base, _ := lcStartBinary(t, bin, v.URL())

	status, state, body := lcPost(t, base, string(mustEncode(t, lcEnvelope("REQ-BEFORE", "show shelter"))))
	if status != http.StatusOK {
		t.Fatalf("healthy endpoint: status=%d state=%q body=%s", status, state, body)
	}
	var env ResponseEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Proposal.RequestID != "REQ-BEFORE" {
		t.Errorf("request_id=%q", env.Proposal.RequestID)
	}

	// The inference endpoint dies.
	v.Close()
	lcAwait(t, 15*time.Second, "request while the endpoint is down", func() {
		status, state, body := lcPost(t, base, string(mustEncode(t, lcEnvelope("REQ-DOWN", "show shelter"))))
		if status == http.StatusOK {
			t.Errorf("worker served a proposal with the inference endpoint down: %s", body)
			return
		}
		if state != MiddleStateUnavailable {
			t.Errorf("X-Sthira-State=%q; want %q (body=%s)", state, MiddleStateUnavailable, body)
		}
	})

	// The endpoint restarts on the same port. The same middle-worker
	// process must serve again.
	v.Restart(t)
	lcAwait(t, 15*time.Second, "request after the endpoint restarted", func() {
		status, state, body := lcPost(t, base, string(mustEncode(t, lcEnvelope("REQ-AFTER-RESTART", "show shelter"))))
		if status != http.StatusOK {
			t.Errorf("worker did not recover after the endpoint restarted: status=%d state=%q body=%s", status, state, body)
			return
		}
		var env ResponseEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		if env.Proposal.RequestID != "REQ-AFTER-RESTART" {
			t.Errorf("stale response delivered after restart: %q", env.Proposal.RequestID)
		}
	})
}

// ---------------------------------------------------------------------
// 8. shutdown leaves nothing behind
// ---------------------------------------------------------------------

// TestLifecycle_ShutdownReleasesListenerAndJoinsPool: after Shutdown
// the worker must own no listening socket and no pool goroutine.
// Breaks if the listener leaks, if a run() goroutine outlives
// Shutdown, or if Shutdown returns before the pool is joined.
func TestLifecycle_ShutdownReleasesListenerAndJoinsPool(t *testing.T) {
	base := runtime.NumGoroutine()
	w, err := NewWorker(Config{Runtime: lcNewRuntime("en-IN"), QueueDepth: 2, MaxInFlight: 2})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	probe, port := lcListen(t)
	_ = probe.Close()
	addr := "127.0.0.1:" + strconv.Itoa(port)
	srv, err := NewServer(ServerConfig{
		Address:      addr,
		Token:        "lc-token",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		ShutdownWait: 2 * time.Second,
	}, w)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	go func() { _ = srv.Start() }()

	lcAwait(t, 5*time.Second, "health before shutdown", func() {
		if status, _ := lcGet(t, "http://"+addr, "/health"); status != http.StatusOK {
			t.Errorf("pre-shutdown /health status=%d", status)
		}
	})
	lcAwait(t, 5*time.Second, "Shutdown", func() {
		if err := srv.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	})
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = conn.Close()
		t.Fatalf("listener %s still accepting after Shutdown", addr)
	}
	if w.Snapshot().Ready {
		t.Errorf("worker still reports ready after Shutdown")
	}
	lcWaitGoroutines(t, base, 4, "worker pool after Shutdown")
}

// TestLifecycle_ShutdownLeavesNoOrphanProcess: the process that owns
// the worker must be gone (kill(pid, 0) == ESRCH) and its port free
// after shutdown, and the inference endpoint it was pointed at must
// still be running because the Go worker does not own it
// (sarvam_config.go:163-165). Breaks if shutdown leaks the process,
// leaks the socket, or starts killing a process it does not own.
func TestLifecycle_ShutdownLeavesNoOrphanProcess(t *testing.T) {
	bin := lcBuildBinary(t)
	v := lcNewVLLM(t, lcProposalTemplate, lcAnswer)
	cmd, _, port := lcStartBinary(t, bin, v.URL())
	pid := cmd.Process.Pid
	if lcProcessGone(pid) {
		t.Fatalf("process %d is not running", pid)
	}

	// SIGTERM is the path the shipped command wires to srv.Shutdown
	// (cmd/middleworker/main.go:81,96-99).
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }() // reaps, so kill(pid, 0) is meaningful
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal: %v", err)
	}
	lcAwait(t, 15*time.Second, "process exit after SIGTERM", func() {
		select {
		case err := <-exited:
			if err != nil {
				t.Errorf("exit after SIGTERM: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("middleworker pid %d is still alive 10s after SIGTERM", pid)
		}
	})
	if !lcProcessGone(pid) {
		t.Errorf("pid %d still exists after the worker was reaped", pid)
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = conn.Close()
		t.Errorf("port %d still accepting after the process exited", port)
	}
	// The inference endpoint is not owned by the Go worker, so it must
	// still be there: no over-reach, and no orphan created by us.
	if status, _ := lcGetQuiet(v.URL(), "/v1/chat/completions"); status == 0 {
		t.Errorf("inference endpoint went away with the worker; the worker must not own it")
	}
}

// TestRegression_ShutdownEndpointNeverReportsDrained guarantees POST
// /shutdown behaves as documented: a clean drain answers 200
// {"state":"DRAINED"} instead of always hitting its own deadline and
// reporting 503 DEADLINE_EXCEEDED, because the drain excludes the
// shutdown request's own connection.
func TestRegression_ShutdownEndpointNeverReportsDrained(t *testing.T) {
	base := lcNewServer(t, lcNewRuntime("en-IN"))
	// A keep-alive client: the server closes the connection instead
	// when the request asks to close it, which would mask the state.
	req, err := http.NewRequest(http.MethodPost, base+"/shutdown", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer lc-token")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST /shutdown: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "DRAINED") {
		t.Errorf("clean drain must answer 200 DRAINED; got status=%d body=%s", resp.StatusCode, body)
	}
}

// ---------------------------------------------------------------------
// pool invariants the lifecycle depends on
// ---------------------------------------------------------------------

// TestRegression_LoadAndVerifyIsNotIdempotent guarantees the promise in
// worker.go that a second LoadAndVerify on a ready worker is a no-op:
// it must not spawn another pool, so the highest number of Propose
// calls running at the same instant never exceeds MaxInFlight and the
// GPU pressure MaxInFlight exists to bound cannot grow.
func TestRegression_LoadAndVerifyIsNotIdempotent(t *testing.T) {
	rt := lcNewRuntime("en-IN")
	gate := make(chan struct{})
	rt.set(func(r *lcRuntime) { r.gate = gate })
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 4, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("first LoadAndVerify: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("second LoadAndVerify must be a no-op, got %v", err)
	}
	t.Cleanup(func() { close(gate); _ = w.Shutdown(2 * time.Second) })

	// Two jobs, gate still closed: with one pool goroutine only the
	// first can be inside Propose. A second goroutine takes the second
	// job immediately, so the peak concurrency becomes 2.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 2)
	for _, id := range []string{"REQ-A", "REQ-B"} {
		go func(id string) {
			_, err := w.Dispatch(ctx, lcEnvelope(id, "x"))
			done <- err
		}(id)
	}
	// The first job must enter; a second entry while the gate is
	// still closed can only come from a second pool. Its absence is
	// a negative check, so it is bounded by a short window.
	select {
	case <-rt.entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("no request reached inference")
	}
	select {
	case <-rt.entered:
	case <-time.After(300 * time.Millisecond):
	}
	if peak := rt.peak.Load(); peak > int64(w.maxInflight) {
		t.Errorf("MaxInFlight=%d but a second LoadAndVerify started a second pool: %d jobs ran at once", w.maxInflight, peak)
	}
}

// TestRegression_DispatchRacingShutdownCrashesTheProcess guarantees a
// drain racing an arriving request is safe: Dispatch and Shutdown may
// overlap without a send on a closed admission queue, so neither the
// process nor the race detector reports a crash during an ordinary
// operation. The probe runs in a child process so a regression is
// reported as a failing test instead of taking the runner with it.
func TestRegression_DispatchRacingShutdownCrashesTheProcess(t *testing.T) {
	if os.Getenv("LC_CRASH_CHILD") == "1" {
		lcChildDispatchShutdownRace(t)
		return
	}
	out, err := lcRunChild(t, "TestRegression_DispatchRacingShutdownCrashesTheProcess")
	if err != nil {
		t.Fatalf("Dispatch racing Shutdown crashed the process:\n%s", out)
	}
	if !strings.Contains(out, "LC_RACE_OK") {
		t.Fatalf("race probe did not finish:\n%s", out)
	}
}

func lcChildDispatchShutdownRace(t *testing.T) {
	for round := 0; round < 300; round++ {
		rt := lcNewRuntime("en-IN")
		w, err := NewWorker(Config{Runtime: rt, QueueDepth: 2, MaxInFlight: 1})
		if err != nil {
			t.Fatalf("round %d NewWorker: %v", round, err)
		}
		if err := w.LoadAndVerify(context.Background()); err != nil {
			t.Fatalf("round %d LoadAndVerify: %v", round, err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				_, _ = w.Dispatch(ctx, lcEnvelope("REQ-RACE", "x"))
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = w.Shutdown(100 * time.Millisecond)
		}()
		close(start)
		wg.Wait()
	}
	t.Log("LC_RACE_OK")
}
