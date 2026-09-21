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
	cmd := exec.Command(r.cmd, args...)
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

	// Scanner buffer derived from the maximum permitted audio
	// size plus base64/JSON overhead: 12 seconds of 22050 Hz
	// mono 16-bit PCM = 264,600 sample bytes + 44 header; base64
	// expands by ~33%. Add 20% margin.
	scanner := newStdScanner(stdout, 1<<20)

	disp := &ttsIPCDispatcher{
		proc:         cmd,
		stdin:        stdin,
		scanner:      scanner,
		pending:      make(map[string]chan ttsAdapterResponse),
		stderrBufPtr: &stderrBuf,
		demuxExit:    make(chan struct{}),
	}

	probeBytes, _ := json.Marshal(ttsAdapterRequest{Op: "ready"})
	probeBytes = append(probeBytes, '\n')
	if _, err := stdin.Write(probeBytes); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("%w: write probe: %v", ErrRuntimeUnavailable, err)
	}

	go disp.runDemux()

	startupCh := make(chan ttsAdapterResponse, 1)
	disp.mu.Lock()
	disp.pending["__startup__"] = startupCh
	disp.mu.Unlock()

	var resp ttsAdapterResponse
	select {
	case resp = <-startupCh:
	case <-time.After(ttsAdapterStartupTimeout):
		_ = disp.Close()
		return fmt.Errorf("%w: startup timeout; stderr: %s",
			ErrRuntimeUnavailable, stderrBuf.String())
	}
	if resp.Status != "ready" {
		_ = disp.Close()
		return fmt.Errorf("%w: status %q error: %s",
			ErrRuntimeUnavailable, resp.Status, resp.Error)
	}

	voiceMap := make(map[string]string)
	var voices []VoiceInfo
	for _, v := range resp.Voices {
		voices = append(voices, VoiceInfo{
			Language: v.Language,
			Name:     v.Name,
			Revision: v.Revision,
		})
		if _, ok := voiceMap[v.Language]; !ok {
			voiceMap[v.Language] = v.Name
		}
	}

	r.mu.Lock()
	r.demux = disp
	r.revision = resp.Revision
	r.digestName = resp.DigestName
	r.digestSHA = resp.DigestSHA256
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
	requestID := fmt.Sprintf("R-%d", nextSynthID())
	req := ttsAdapterRequest{
		Op:         "synthesize",
		RequestID:  requestID,
		Text:       text,
		Language:   language,
		Voice:      voice,
		SampleRate: DefaultOutputSampleRate,
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
		d.mu.Unlock()
	}()
	for d.scanner.Scan() {
		line := d.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp ttsAdapterResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			d.demuxErr = fmt.Errorf("tts ipc: malformed response: %w", err)
			fmt.Fprintf(os.Stderr, "ttsworker ipc: malformed response: %v\n", err)
			return
		}
		d.mu.Lock()
		if resp.RequestID == "" {
			if startupCh, ok := d.pending["__startup__"]; ok {
				delete(d.pending, "__startup__")
				d.mu.Unlock()
				select {
				case startupCh <- resp:
				default:
				}
				continue
			}
			d.mu.Unlock()
			d.demuxErr = errors.New("tts ipc: response missing request_id")
			fmt.Fprintln(os.Stderr, "ttsworker ipc: response missing request_id")
			return
		}
		ch, ok := d.pending[resp.RequestID]
		if ok {
			delete(d.pending, resp.RequestID)
		}
		d.mu.Unlock()
		if !ok {
			continue
		}
		select {
		case ch <- resp:
		default:
		}
	}
	var demuxErr error
	if err := d.scanner.Err(); err != nil {
		demuxErr = fmt.Errorf("tts ipc: scanner: %w", err)
	} else {
		demuxErr = errors.New("tts ipc: subprocess closed stdout")
	}
	d.mu.Lock()
	d.demuxErr = demuxErr
	d.mu.Unlock()
}

// send writes a single JSONL request and awaits the matching
// response. The mutex serializes writes with itself and with Close.
func (d *ttsIPCDispatcher) send(req ttsAdapterRequest, requestID string, perCallTimeout time.Duration, ctx RequestContext) (ttsAdapterResponse, error) {
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
	if d.stdin == nil {
		d.mu.Unlock()
		return ttsAdapterResponse{}, fmt.Errorf("%w: not loaded", ErrRuntimeUnavailable)
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		d.mu.Unlock()
		return ttsAdapterResponse{}, fmt.Errorf("marshal: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	respCh := make(chan ttsAdapterResponse, 1)
	d.pending[requestID] = respCh

	if _, err := d.stdin.Write(reqBytes); err != nil {
		delete(d.pending, requestID)
		d.mu.Unlock()
		return ttsAdapterResponse{}, fmt.Errorf("%w: write: %v", ErrRuntimeUnavailable, err)
	}
	d.mu.Unlock()

	timer := time.NewTimer(perCallTimeout)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		if resp.RequestID != requestID && resp.RequestID != "__startup__" {
			return ttsAdapterResponse{}, fmt.Errorf("%w: id mismatch got=%q want=%q",
				ErrRuntimeUnavailable, resp.RequestID, requestID)
		}
		return resp, nil
	case <-timer.C:
		d.mu.Lock()
		delete(d.pending, requestID)
		d.mu.Unlock()
		return ttsAdapterResponse{}, fmt.Errorf("%w: per-call deadline", ErrRuntimeUnavailable)
	case <-ctx.Canceled:
		d.mu.Lock()
		delete(d.pending, requestID)
		d.mu.Unlock()
		return ttsAdapterResponse{}, ErrRuntimeUnavailable
	}
}

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
	d.proc = nil
	d.mu.Unlock()

	if stdin != nil {
		shutReq, _ := json.Marshal(ttsAdapterRequest{Op: "shutdown"})
		shutReq = append(shutReq, '\n')
		_, _ = stdin.Write(shutReq)
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
