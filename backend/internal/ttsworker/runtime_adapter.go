// runtime_adapter.go implements the real AdapterSubprocessRuntime
// backed by the Indic Parler-TTS Python adapter.
//
// Architecture mirrors the ASR adapter (see runtime_ipc.go): one
// goroutine owns the subprocess stdout and routes responses to
// matching pending requests via request_id; the whole exchange
// is bounded by a per-call deadline and the caller's context.
//
// Adapter protocol:
//
//	Startup:  {"op":"ready"}
//	Ready:    {"status":"ready"|"blocked", "revision":"...",
//	           "languages":["hi-IN","ml-IN"],
//	           "voices":[{"language":"hi-IN","name":"default",
//	                       "revision":"..."}],
//	           "digest_name":"...","digest_sha256":"..."}
//
//	Request:  {"op":"synthesize","request_id":"R-1","text":"...",
//	           "language":"hi-IN","voice":"default","sample_rate":22050}
//	Response: {"request_id":"R-1","audio_b64":"<base64 PCM16LE WAV>",
//	           "duration_secs":1.5} or {"request_id":"R-1",
//	           "error":"..."}
//
//	Shutdown: {"op":"shutdown"}
//	Ack:      {"status":"shutdown"}
package ttsworker

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	ttsAdapterStartupTimeout = 60 * time.Second
	ttsAdapterPerCallTimeout = 15 * time.Second
)

// ttsAdapterRequest is the wire shape sent to the Python TTS adapter.
type ttsAdapterRequest struct {
	Op         string `json:"op"`
	RequestID  string `json:"request_id,omitempty"`
	Text       string `json:"text,omitempty"`
	Language   string `json:"language,omitempty"`
	Voice      string `json:"voice,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

// ttsAdapterResponse is the wire shape received from the Python
// TTS adapter.
type ttsAdapterResponse struct {
	Status       string          `json:"status,omitempty"`
	Revision     string          `json:"revision,omitempty"`
	Languages    []string        `json:"languages,omitempty"`
	Voices       []ttsVoiceEntry `json:"voices,omitempty"`
	DigestName   string          `json:"digest_name,omitempty"`
	DigestSHA256 string          `json:"digest_sha256,omitempty"`
	SampleRate   int             `json:"sample_rate,omitempty"`

	RequestID    string  `json:"request_id,omitempty"`
	AudioB64     string  `json:"audio_b64,omitempty"`
	DurationSecs float64 `json:"duration_secs,omitempty"`

	Error string `json:"error,omitempty"`
}

type ttsVoiceEntry struct {
	Language string `json:"language"`
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

// ipcDispatcher is the TTS-side equivalent of the ASR dispatcher.
// It is intentionally package-private and self-contained so the
// TTS worker module stays stdlib-only.
type ttsIPCDispatcher struct {
	mu sync.Mutex

	proc  *exec.Cmd
	stdin interface {
		Write(p []byte) (int, error)
		Close() error
	}
	scanner lineScanner

	pending map[string]chan ttsAdapterResponse

	closed bool

	stderrBufPtr *ttsStderrBuf

	demuxExit chan struct{}

	demuxErr error
}

// lineScanner is a minimal interface so the dispatcher is testable
// without a real subprocess. The real implementation is a
// bufio.Scanner over the subprocess stdout pipe.
type lineScanner interface {
	Scan() bool
	Bytes() []byte
	Err() error
}

// AdapterSubprocessRuntime is the real Parler-TTS runtime backed
// by a Python subprocess.
type AdapterSubprocessRuntime struct {
	mu         sync.Mutex
	closed     bool
	cmd        string
	module     string
	workdir    string
	extraEnv   []string
	revision   string
	digestName string
	digestSHA  string
	sampleRate int
	languages  []string
	voices     []VoiceInfo
	voiceMap   map[string]string
	demux      *ttsIPCDispatcher
}

// AdapterSubprocessConfig configures the Python TTS adapter.
type AdapterSubprocessConfig struct {
	Cmd      string
	Module   string
	Workdir  string
	ExtraEnv []string
}

// DefaultTTSAdapterConfig returns the default configuration for
// the Indic Parler-TTS adapter.
func DefaultTTSAdapterConfig() AdapterSubprocessConfig {
	return AdapterSubprocessConfig{
		Cmd:    "python3",
		Module: "sthira_v2.speech_tts_adapter",
	}
}

// NewAdapterSubprocessRuntime creates a new adapter runtime. Does NOT
// spawn the subprocess; call LoadModel() to start.
func NewAdapterSubprocessRuntime(cfg AdapterSubprocessConfig) *AdapterSubprocessRuntime {
	cmd := cfg.Cmd
	if cmd == "" {
		cmd = "python3"
	}
	return &AdapterSubprocessRuntime{
		cmd:      cmd,
		module:   cfg.Module,
		workdir:  cfg.Workdir,
		extraEnv: cfg.ExtraEnv,
	}
}

// LoadModel spawns the Python subprocess and waits for "ready".
func (r *AdapterSubprocessRuntime) LoadModel() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return ErrRuntimeClosed
	}
	if r.demux != nil {
		r.mu.Unlock()
		return errors.New("model already loaded")
	}
	if _, err := exec.LookPath(r.cmd); err != nil {
		r.mu.Unlock()
		return fmt.Errorf("%w: %s not found: %v", ErrRuntimeUnavailable, r.cmd, err)
	}
	r.mu.Unlock()

	args := []string{"-u", "-m", r.module, "--adapter-mode"}
	cmd := exec.Command(r.cmd, args...) // #nosec G204
	if r.workdir != "" {
		cmd.Dir = r.workdir
	}
	cmd.Env = append(os.Environ(), r.extraEnv...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("tts: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("tts: stdout pipe: %w", err)
	}
	var stderrBuf ttsStderrBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start: %v", ErrRuntimeUnavailable, err)
	}

	// Scanner buffer must hold the largest legitimate response:
	// 12 s of native-rate (up to 48 kHz) mono PCM16 WAV ≈ 1.15 MiB,
	// base64-expands to ≈ 1.54 MiB plus JSON overhead. Cap 2 MiB;
	// truly oversized/malformed lines abort the demux at the
	// scanner boundary (tested in b3).
	scanner := newStdScanner(stdout, 2<<20)

	disp := &ttsIPCDispatcher{
		proc:         cmd,
		stdin:        stdin,
		scanner:      scanner,
		pending:      make(map[string]chan ttsAdapterResponse),
		stderrBufPtr: &stderrBuf,
		demuxExit:    make(chan struct{}),
	}

	// Register startup correlation BEFORE the reader can deliver
	// it (mirrors the ASR dispatcher repair): an adapter that
	// answers the ready probe instantly must not be mistaken for
	// an unsolicited response.
	startupCh := make(chan ttsAdapterResponse, 1)
	disp.mu.Lock()
	disp.pending["__startup__"] = startupCh
	disp.mu.Unlock()

	go disp.runDemux()

	probeBytes, _ := json.Marshal(ttsAdapterRequest{Op: "ready"})
	probeBytes = append(probeBytes, '\n')
	started := time.Now()
	if err := disp.writeStdin(probeBytes, ttsAdapterStartupTimeout, nil); err != nil {
		_ = disp.Close()
		return fmt.Errorf("%w: write probe: %v", ErrRuntimeUnavailable, err)
	}

	var resp ttsAdapterResponse
	select {
	case resp = <-startupCh:
	case <-time.After(ttsAdapterStartupTimeout - time.Since(started)):
		_ = disp.Close()
		return fmt.Errorf("%w: startup timeout; stderr: %s",
			ErrRuntimeUnavailable, stderrBuf.String())
	}
	if resp.Status != "ready" {
		_ = disp.Close()
		return fmt.Errorf("%w: status %q error: %s",
			ErrRuntimeUnavailable, resp.Status, resp.Error)
	}
	// The adapter must advertise its model-native sample rate
	// (Indic Parler-TTS writes audio at model.config.sampling_rate;
	// any other rate changes playback speed). A ready envelope
	// without a plausible rate is a protocol violation and keeps
	// the worker unready.
	if resp.SampleRate <= 0 || resp.SampleRate > MaxOutputSampleRate {
		_ = disp.Close()
		return fmt.Errorf("%w: runtime reported ready without a usable native sample rate (got %d, want 1..%d)",
			ErrRuntimeUnavailable, resp.SampleRate, MaxOutputSampleRate)
	}

	voiceMap := make(map[string]string)
	var voices []VoiceInfo
	for _, v := range resp.Voices {
		voices = append(voices, VoiceInfo(v))
		if _, ok := voiceMap[v.Language]; !ok {
			voiceMap[v.Language] = v.Name
		}
	}

	r.mu.Lock()
	r.demux = disp
	r.revision = resp.Revision
	r.digestName = resp.DigestName
	r.digestSHA = resp.DigestSHA256
	r.sampleRate = resp.SampleRate
	r.languages = append([]string(nil), resp.Languages...)
	r.voices = voices
	r.voiceMap = voiceMap
	r.mu.Unlock()
	return nil
}

// Synthesize implements Runtime. Sends a synthesis request to the
// Python adapter and receives base64-encoded audio. The audio
// bytes are decoded from WAV by the worker's audio module.
func (r *AdapterSubprocessRuntime) Synthesize(ctx RequestContext, text string, language string, voice string) (*SynthResult, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrRuntimeClosed
	}
	if r.demux == nil {
		r.mu.Unlock()
		return nil, ErrRuntimeUnavailable
	}
	// Verify language support.
	supported := false
	for _, l := range r.languages {
		if l == language {
			supported = true
			break
		}
	}
	if !supported {
		r.mu.Unlock()
		return nil, ErrLanguageUnsupported
	}

	if voice == "" {
		voice = r.voiceMap[language]
	}
	if voice == "" {
		r.mu.Unlock()
		return nil, ErrVoiceUnsupported
	}
	rate := r.sampleRate
	requestID := fmt.Sprintf("R-%d", nextSynthID())
	req := ttsAdapterRequest{
		Op:         "synthesize",
		RequestID:  requestID,
		Text:       text,
		Language:   language,
		Voice:      voice,
		SampleRate: rate,
	}
	disp := r.demux
	r.mu.Unlock()

	resp, err := disp.send(req, requestID, ttsAdapterPerCallTimeout, ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrRuntimeUnavailable, resp.Error)
	}
	if resp.SampleRate != 0 && resp.SampleRate != rate {
		return nil, fmt.Errorf("%w: adapter returned rate %d, negotiated %d",
			ErrRuntimeUnavailable, resp.SampleRate, rate)
	}
	if resp.AudioB64 == "" {
		return &SynthResult{Empty: true}, nil
	}
	audioBytes, err := base64.StdEncoding.DecodeString(resp.AudioB64)
	if err != nil {
		return nil, fmt.Errorf("%w: decode audio: %v", ErrRuntimeUnavailable, err)
	}
	if int64(len(audioBytes)) > int64(MaxOutputBytes) {
		return nil, fmt.Errorf("%w: audio %d > %d bytes", ErrRuntimeUnavailable, len(audioBytes), MaxOutputBytes)
	}
	if resp.DurationSecs > MaxOutputDurationSeconds {
		return nil, fmt.Errorf("%w: duration %.1f > %.1f", ErrRuntimeUnavailable, resp.DurationSecs, MaxOutputDurationSeconds)
	}
	return &SynthResult{
		Empty:    false,
		WavBytes: audioBytes,
	}, nil
}

// Close implements Runtime.
func (r *AdapterSubprocessRuntime) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	disp := r.demux
	r.demux = nil
	r.mu.Unlock()
	if disp != nil {
		return disp.Close()
	}
	return nil
}

// Languages implements Runtime.
func (r *AdapterSubprocessRuntime) Languages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.languages...)
}

// Revision implements Runtime.
func (r *AdapterSubprocessRuntime) Revision() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.revision
}

// Voice implements Runtime.
func (r *AdapterSubprocessRuntime) Voice(language string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.voiceMap[language]
}

// Voices implements Runtime.
func (r *AdapterSubprocessRuntime) Voices() []VoiceInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]VoiceInfo(nil), r.voices...)
}

// --- dispatcher implementation ---------------------------------
//
// Boundedness mirrors asrworker/runtime_ipc.go: startup
// correlation is registered before the reader starts; every stdin
// write runs under a deadline that starts BEFORE the write (a
// wedged child fills the OS pipe and would otherwise hang Request
// and Close); demuxErr is only touched under the mutex; and Close
// reaps the child unconditionally so it never waits on a wedged
// child's cooperation.

// writeStdin writes one JSONL request under a budget covering the
// write itself. Budget expiry means the child cannot drain input
// at all, so the whole dispatcher is uncertain: the child is
// killed, which releases the blocked write and the goroutine.
// Caller cancellation returns without killing (the response is
// dropped by deregistration; the writer goroutine ends when the
// child drains or Close kills it).
func (d *ttsIPCDispatcher) writeStdin(data []byte, budget time.Duration, cancel <-chan struct{}) error {
	d.mu.Lock()
	w := d.stdin
	if d.closed || w == nil {
		d.mu.Unlock()
		return fmt.Errorf("%w: dispatcher closed", ErrRuntimeUnavailable)
	}
	d.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := w.Write(data)
		done <- err
	}()

	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			d.markUncertain(fmt.Errorf("write: %v", err))
			return fmt.Errorf("%w: write: %v", ErrRuntimeUnavailable, err)
		}
		return nil
	case <-timer.C:
		d.markUncertain(errors.New("tts ipc: stdin write deadline; child not draining input"))
		<-done
		return fmt.Errorf("%w: stdin write deadline", ErrRuntimeUnavailable)
	case <-cancel:
		return fmt.Errorf("%w: canceled during stdin write", ErrRuntimeUnavailable)
	}
}

// markUncertain records a protocol-lifetime failure under the
// mutex, releases every pending caller, and kills the child so
// the demultiplexer observes EOF and blocked writers unblock.
// Idempotent; safe from any goroutine.
func (d *ttsIPCDispatcher) markUncertain(reason error) {
	d.mu.Lock()
	if d.demuxErr == nil {
		d.demuxErr = reason
	}
	for id, ch := range d.pending {
		select {
		case ch <- ttsAdapterResponse{Error: "subprocess reset"}:
		default:
		}
		delete(d.pending, id)
	}
	proc := d.proc
	d.mu.Unlock()
	if proc != nil && proc.Process != nil {
		_ = proc.Process.Kill()
	}
}

func (d *ttsIPCDispatcher) runDemux() {
	defer close(d.demuxExit)
	defer func() {
		d.mu.Lock()
		for id, ch := range d.pending {
			select {
			case ch <- ttsAdapterResponse{Error: "subprocess terminated"}:
			default:
			}
			delete(d.pending, id)
		}
		if d.demuxErr == nil {
			if err := d.scanner.Err(); err != nil {
				d.demuxErr = fmt.Errorf("tts ipc: scanner: %w", err)
			} else {
				d.demuxErr = errors.New("tts ipc: subprocess closed stdout")
			}
		}
		d.mu.Unlock()
	}()
	for d.scanner.Scan() {
		line := d.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp ttsAdapterResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "ttsworker ipc: malformed response: %v\n", err)
			d.markUncertain(fmt.Errorf("tts ipc: malformed response: %w", err))
			return
		}
		d.mu.Lock()
		if resp.RequestID == "" {
			// Only valid for the startup envelope, which is
			// registered before this goroutine starts.
			if startupCh, ok := d.pending["__startup__"]; ok {
				delete(d.pending, "__startup__")
				d.mu.Unlock()
				startupCh <- resp // buffer 1; receiver already selected
				continue
			}
			d.mu.Unlock()
			fmt.Fprintln(os.Stderr, "ttsworker ipc: response missing request_id")
			d.markUncertain(errors.New("tts ipc: response missing request_id"))
			return
		}
		ch, ok := d.pending[resp.RequestID]
		if ok {
			delete(d.pending, resp.RequestID)
		}
		d.mu.Unlock()
		if !ok {
			// Late response after cancel/timeout or an unknown /
			// duplicate id: discard; it must never reach another
			// caller.
			continue
		}
		select {
		case ch <- resp:
		default:
		}
	}
	// Scanner finished; the deferred cleanup classifies it.
}

// send writes a single JSONL request and awaits the matching
// response. The per-call budget covers the write AND the wait;
// no mutex is held across the blocking write.
func (d *ttsIPCDispatcher) send(req ttsAdapterRequest, requestID string, perCallTimeout time.Duration, ctx RequestContext) (ttsAdapterResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return ttsAdapterResponse{}, fmt.Errorf("marshal: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	started := time.Now()
	respCh := make(chan ttsAdapterResponse, 1)

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return ttsAdapterResponse{}, ErrRuntimeClosed
	}
	if d.demuxErr != nil {
		err := d.demuxErr
		d.mu.Unlock()
		return ttsAdapterResponse{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, err)
	}
	d.pending[requestID] = respCh
	d.mu.Unlock()

	deregister := func() {
		d.mu.Lock()
		delete(d.pending, requestID)
		d.mu.Unlock()
	}
	remaining := func() time.Duration {
		left := perCallTimeout - time.Since(started)
		if left < 0 {
			left = 0
		}
		return left
	}
	var cancel <-chan struct{}
	if ctx.Canceled != nil {
		cancel = ctx.Canceled
	}
	if err := d.writeStdin(reqBytes, remaining(), cancel); err != nil {
		deregister()
		return ttsAdapterResponse{}, err
	}

	timer := time.NewTimer(remaining())
	defer timer.Stop()
	select {
	case resp := <-respCh:
		if resp.RequestID != requestID && resp.RequestID != "__startup__" {
			return ttsAdapterResponse{}, fmt.Errorf("%w: id mismatch got=%q want=%q",
				ErrRuntimeUnavailable, resp.RequestID, requestID)
		}
		return resp, nil
	case <-timer.C:
		deregister()
		return ttsAdapterResponse{}, fmt.Errorf("%w: per-call deadline", ErrRuntimeUnavailable)
	case <-cancelOrNever(ctx.Canceled):
		deregister()
		return ttsAdapterResponse{}, ErrRuntimeUnavailable
	}
}

// cancelOrNever lets a nil cancel channel block forever instead
// of spinning select.
func cancelOrNever(ch <-chan struct{}) <-chan struct{} {
	if ch != nil {
		return ch
	}
	return make(chan struct{})
}

// Close terminates the subprocess and waits for the demultiplexer
// to drain. The graceful shutdown write is best-effort under a
// 1s budget; the kill+wait then guarantees bounded Close even
// against a wedged child. Idempotent.
func (d *ttsIPCDispatcher) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	stdin := d.stdin
	proc := d.proc
	d.stdin = nil
	d.mu.Unlock()

	if stdin != nil {
		shutReq, _ := json.Marshal(ttsAdapterRequest{Op: "shutdown"})
		shutReq = append(shutReq, '\n')
		writeDone := make(chan struct{})
		go func() {
			_, _ = stdin.Write(shutReq)
			close(writeDone)
		}()
		select {
		case <-writeDone:
		case <-time.After(time.Second):
			// Wedged child; kill below unblocks the writer.
		}
		_ = stdin.Close()
	}
	if proc != nil && proc.Process != nil {
		_ = proc.Process.Kill()
		_ = proc.Wait()
	}
	if d.demuxExit != nil {
		<-d.demuxExit
	}
	return nil
}

// ttsStderrBuf is a bounded buffer for subprocess stderr.
type ttsStderrBuf struct {
	mu   sync.Mutex
	data []byte
}

func (b *ttsStderrBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	const limit = 4096
	remaining := limit - len(b.data)
	if remaining > 0 {
		n := len(p)
		if n > remaining {
			n = remaining
		}
		b.data = append(b.data, p[:n]...)
	}
	return len(p), nil
}

func (b *ttsStderrBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}

// nextSynthID is a process-local counter used to generate unique
// request IDs for Synthesize calls.
var (
	synthIDMu  sync.Mutex
	synthIDSeq uint64
)

func nextSynthID() uint64 {
	synthIDMu.Lock()
	defer synthIDMu.Unlock()
	synthIDSeq++
	return synthIDSeq
}
