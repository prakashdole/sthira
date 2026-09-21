package ttsworker

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// runtime_adapter.go implements the real SubprocessRuntime backed by
// the Indic Parler-TTS Python adapter.
//
// Architecture mirrors the ASR adapter: long-lived Python subprocess,
// JSONL IPC, mutex-serialized I/O, bounded stderr.
//
// Adapter protocol:
//
//   Startup:  {"op":"ready"}
//   Ready:    {"status":"ready","revision":"...","languages":["hi-IN","ml-IN"],
//              "voices":[{"language":"hi-IN","name":"default","revision":"..."}],
//              "digest_name":"...","digest_sha256":"..."}
//
//   Request:  {"op":"synthesize","text":"...","language":"hi-IN",
//              "voice":"default","sample_rate":22050}
//   Response: {"audio_b64":"<base64 PCM16LE WAV>","duration_secs":1.5}
//              or {"error":"..."}
//
//   Shutdown: {"op":"shutdown"}
//   Ack:      {"status":"shutdown"}

// ttsAdapterRequest is the wire shape sent to the Python TTS adapter.
type ttsAdapterRequest struct {
	Op         string `json:"op"`
	Text       string `json:"text,omitempty"`
	Language   string `json:"language,omitempty"`
	Voice      string `json:"voice,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

// ttsAdapterResponse is the wire shape received from the Python
// TTS adapter.
type ttsAdapterResponse struct {
	// Startup fields.
	Status       string          `json:"status,omitempty"`
	Revision     string          `json:"revision,omitempty"`
	Languages    []string        `json:"languages,omitempty"`
	Voices       []ttsVoiceEntry `json:"voices,omitempty"`
	DigestName   string          `json:"digest_name,omitempty"`
	DigestSHA256 string          `json:"digest_sha256,omitempty"`

	// Synthesize fields.
	AudioB64     string  `json:"audio_b64,omitempty"`
	DurationSecs float64 `json:"duration_secs,omitempty"`

	// Error.
	Error string `json:"error,omitempty"`
}

type ttsVoiceEntry struct {
	Language string `json:"language"`
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

const ttsAdapterStartupTimeout = 60 * time.Second
const ttsAdapterPerCallTimeout = 15 * time.Second

// AdapterSubprocessRuntime is the real Parler-TTS runtime backed by
// a Python subprocess. It implements the ttsworker.Runtime interface.
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
	voiceMap   map[string]string // language → default voice name
	proc       *exec.Cmd
	stdin      io.WriteCloser
	scanner    *bufio.Scanner
}

// AdapterSubprocessConfig configures the Python TTS adapter.
type AdapterSubprocessConfig struct {
	Cmd      string   // python3 executable (default "python3")
	Module   string   // python -m target (e.g. "sthira_v2.speech_tts")
	Workdir  string   // optional CWD
	ExtraEnv []string // env vars
}

// DefaultTTSAdapterConfig returns the default configuration for
// the Indic Parler-TTS adapter.
func DefaultTTSAdapterConfig() AdapterSubprocessConfig {
	return AdapterSubprocessConfig{
		Cmd:    "python3",
		Module: "sthira_v2.speech_tts",
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
	defer r.mu.Unlock()

	if r.closed {
		return ErrRuntimeClosed
	}
	if r.proc != nil {
		return errors.New("model already loaded")
	}

	if _, err := exec.LookPath(r.cmd); err != nil {
		return fmt.Errorf("%w: %s not found: %v", ErrRuntimeUnavailable, r.cmd, err)
	}

	args := []string{"-u", "-m", r.module, "--adapter-mode"}
	cmd := exec.Command(r.cmd, args...)
	if r.workdir != "" {
		cmd.Dir = r.workdir
	}
	cmd.Env = append(os.Environ(), r.extraEnv...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf ttsStderrBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start: %v", ErrRuntimeUnavailable, err)
	}

	readyReq, _ := json.Marshal(ttsAdapterRequest{Op: "ready"})
	readyReq = append(readyReq, '\n')
	if _, err := stdin.Write(readyReq); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("%w: write ready: %v", ErrRuntimeUnavailable, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	readyCh := make(chan ttsAdapterResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		if scanner.Scan() {
			var resp ttsAdapterResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				errCh <- fmt.Errorf("decode startup: %w", err)
				return
			}
			readyCh <- resp
		} else {
			if err := scanner.Err(); err != nil {
				errCh <- fmt.Errorf("read: %w", err)
			} else {
				errCh <- fmt.Errorf("subprocess closed; stderr: %s", stderrBuf.String())
			}
		}
	}()

	select {
	case resp := <-readyCh:
		if resp.Status != "ready" {
			_ = cmd.Process.Kill()
			return fmt.Errorf("%w: status %q error: %s",
				ErrRuntimeUnavailable, resp.Status, resp.Error)
		}
		r.revision = resp.Revision
		r.digestName = resp.DigestName
		r.digestSHA = resp.DigestSHA256
		r.languages = append([]string(nil), resp.Languages...)
		r.voiceMap = make(map[string]string)
		for _, v := range resp.Voices {
			r.voices = append(r.voices, VoiceInfo{
				Language: v.Language,
				Name:     v.Name,
				Revision: v.Revision,
			})
			if _, ok := r.voiceMap[v.Language]; !ok {
				r.voiceMap[v.Language] = v.Name
			}
		}
		r.proc = cmd
		r.stdin = stdin
		r.scanner = scanner
		return nil

	case err := <-errCh:
		_ = cmd.Process.Kill()
		return fmt.Errorf("%w: %v", ErrRuntimeUnavailable, err)

	case <-time.After(ttsAdapterStartupTimeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("%w: startup timeout; stderr: %s",
			ErrRuntimeUnavailable, stderrBuf.String())
	}
}

// Synthesize implements Runtime. Sends a synthesis request to the
// Python adapter and receives base64-encoded audio. The audio bytes
// are decoded from WAV by the worker's audio module.
func (r *AdapterSubprocessRuntime) Synthesize(ctx RequestContext, text string, language string, voice string) (*SynthResult, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrRuntimeClosed
	}
	if r.proc == nil {
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

	req := ttsAdapterRequest{
		Op:         "synthesize",
		Text:       text,
		Language:   language,
		Voice:      voice,
		SampleRate: DefaultOutputSampleRate,
	}
	reqBytes, _ := json.Marshal(req)
	reqBytes = append(reqBytes, '\n')

	if _, err := r.stdin.Write(reqBytes); err != nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("%w: write: %v", ErrRuntimeUnavailable, err)
	}

	respCh := make(chan ttsAdapterResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		if r.scanner.Scan() {
			var resp ttsAdapterResponse
			if err := json.Unmarshal(r.scanner.Bytes(), &resp); err != nil {
				errCh <- fmt.Errorf("decode: %w", err)
				return
			}
			respCh <- resp
		} else {
			if err := r.scanner.Err(); err != nil {
				errCh <- fmt.Errorf("read: %w", err)
			} else {
				errCh <- errors.New("subprocess closed")
			}
		}
	}()
	r.mu.Unlock()

	// Wait for response or cancellation.
	select {
	case resp := <-respCh:
		if resp.Error != "" {
			return nil, fmt.Errorf("%w: %s", ErrRuntimeUnavailable, resp.Error)
		}
		if resp.AudioB64 == "" {
			return &SynthResult{Empty: true}, nil
		}
		// Decode the audio to verify it's valid.
		audioBytes, err := base64.StdEncoding.DecodeString(resp.AudioB64)
		if err != nil {
			return nil, fmt.Errorf("%w: decode audio: %v", ErrRuntimeUnavailable, err)
		}
		if len(audioBytes) > MaxOutputBytes {
			return nil, fmt.Errorf("%w: audio %d > %d bytes", ErrRuntimeUnavailable, len(audioBytes), MaxOutputBytes)
		}
		if resp.DurationSecs > MaxOutputDurationSeconds {
			return nil, fmt.Errorf("%w: duration %.1f > %.1f", ErrRuntimeUnavailable, resp.DurationSecs, MaxOutputDurationSeconds)
		}
		// Return non-empty result. The worker will encode/hash the
		// audio through WavOutput.
		return &SynthResult{
			Empty:    false,
			WavBytes: audioBytes,
		}, nil

	case <-ctx.Canceled:
		return nil, ErrRuntimeUnavailable

	case <-time.After(ttsAdapterPerCallTimeout):
		return nil, ErrDeadlineExceeded
	}
}

// Close implements Runtime.
func (r *AdapterSubprocessRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.stdin != nil {
		shutReq, _ := json.Marshal(ttsAdapterRequest{Op: "shutdown"})
		shutReq = append(shutReq, '\n')
		_, _ = r.stdin.Write(shutReq)
		_ = r.stdin.Close()
		r.stdin = nil
	}
	if r.proc != nil && r.proc.Process != nil {
		_ = r.proc.Process.Kill()
		_ = r.proc.Wait()
		r.proc = nil
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
