package asrworker

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// runtime_adapter.go implements the real SubprocessRuntime backed by
// the IndicConformer-600M-Multi Python adapter.
//
// Architecture:
//   - On LoadModel(), the runtime spawns a long-lived Python subprocess
//     running the adapter module.
//   - Communication is line-delimited JSON over stdin/stdout (JSONL).
//   - Each Transcribe call sends a request and reads a response.
//   - A mutex serializes subprocess I/O; request concurrency is the
//     Worker's job.
//   - The subprocess is killed on Close() or on unrecoverable error.
//   - The runtime never retries a failed subprocess; the Worker surfaces
//     UNAVAILABLE and the orchestrator must restart.
//
// Adapter protocol (stdin→subprocess, subprocess→stdout):
//
//   Startup:  {"op":"ready"}
//   Ready:    {"status":"ready","revision":"...","languages":["hi-IN","ml-IN"],
//              "digest_name":"...","digest_sha256":"..."}
//
//   Request:  {"op":"transcribe","request_id":"R-1","language":"hi-IN",
//              "samples_b64":"<base64 float32 LE>","sample_rate":16000,
//              "duration_secs":1.5}
//   Response: {"request_id":"R-1","text":"...","confidence":null,
//              "alternatives":[]}
//
//   Shutdown: {"op":"shutdown"}
//   Ack:      {"status":"shutdown"}

// adapterRequest is the wire shape sent to the Python adapter.
type adapterRequest struct {
	Op           string  `json:"op"`
	RequestID    string  `json:"request_id,omitempty"`
	Language     string  `json:"language,omitempty"`
	SamplesB64   string  `json:"samples_b64,omitempty"`
	SampleRate   int     `json:"sample_rate,omitempty"`
	DurationSecs float64 `json:"duration_secs,omitempty"`
}

// adapterResponse is the wire shape received from the Python adapter.
type adapterResponse struct {
	Status       string   `json:"status,omitempty"`
	Revision     string   `json:"revision,omitempty"`
	Languages    []string `json:"languages,omitempty"`
	DigestName   string   `json:"digest_name,omitempty"`
	DigestSHA256 string   `json:"digest_sha256,omitempty"`

	RequestID    string               `json:"request_id,omitempty"`
	Text         string               `json:"text,omitempty"`
	Confidence   *float64             `json:"confidence"`
	Alternatives []adapterAlternative `json:"alternatives,omitempty"`

	Error string `json:"error,omitempty"`
}

type adapterAlternative struct {
	Text       string   `json:"text"`
	Confidence *float64 `json:"confidence"`
}

const adapterStartupTimeout = 30 * time.Second
const adapterPerCallTimeout = 15 * time.Second

// LoadModel spawns the Python subprocess and waits for the "ready"
// response. Populates revision, digest, and supported languages.
func (s *SubprocessRuntime) LoadModel() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrRuntimeClosed
	}
	if s.proc != nil {
		return errors.New("model already loaded")
	}

	pyCmd := s.cfg.Cmd
	if pyCmd == "" {
		pyCmd = "python3"
	}
	if _, err := exec.LookPath(pyCmd); err != nil {
		return fmt.Errorf("%w: %s not found: %v", ErrRuntimeUnavailable, pyCmd, err)
	}

	args := []string{"-u", "-m", s.cfg.Module, "--adapter-mode"}
	cmd := exec.Command(pyCmd, args...)
	if s.cfg.Workdir != "" {
		cmd.Dir = s.cfg.Workdir
	}
	cmd.Env = append(os.Environ(), s.cfg.ExtraEnv...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf adapterStderrBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start: %v", ErrRuntimeUnavailable, err)
	}

	// Send startup probe.
	readyReq, _ := json.Marshal(adapterRequest{Op: "ready"})
	readyReq = append(readyReq, '\n')
	if _, err := stdin.Write(readyReq); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("%w: write ready: %v", ErrRuntimeUnavailable, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	readyCh := make(chan adapterResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		if scanner.Scan() {
			var resp adapterResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				errCh <- fmt.Errorf("decode startup: %w", err)
				return
			}
			readyCh <- resp
		} else {
			if err := scanner.Err(); err != nil {
				errCh <- fmt.Errorf("read startup: %w", err)
			} else {
				errCh <- fmt.Errorf("subprocess closed stdout; stderr: %s", stderrBuf.String())
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
		s.revision = resp.Revision
		s.digest = resp.DigestSHA256
		s.digestName = resp.DigestName
		s.languages = append([]string(nil), resp.Languages...)
		s.proc = cmd
		s.stdin = stdin
		s.scanner = scanner
		return nil

	case err := <-errCh:
		_ = cmd.Process.Kill()
		return fmt.Errorf("%w: %v", ErrRuntimeUnavailable, err)

	case <-time.After(adapterStartupTimeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("%w: startup timeout; stderr: %s",
			ErrRuntimeUnavailable, stderrBuf.String())
	}
}

// transcribeViaAdapter sends a transcribe request to the loaded
// Python subprocess. Called from the overridden Transcribe method
// when proc != nil.
func (s *SubprocessRuntime) transcribeViaAdapter(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	if !hasLanguage(s, req.Language) {
		return TranscribeResult{}, fmt.Errorf("%w: %s", ErrLanguageUnsupported, req.Language)
	}

	adReq := adapterRequest{
		Op:           "transcribe",
		RequestID:    req.RequestID,
		Language:     req.Language,
		SamplesB64:   encodeFloat32B64(req.Samples),
		SampleRate:   req.SampleRate,
		DurationSecs: req.DurationSecs,
	}
	reqBytes, err := json.Marshal(adReq)
	if err != nil {
		return TranscribeResult{}, fmt.Errorf("marshal: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	deadline := adapterPerCallTimeout
	if !req.Deadline.IsZero() {
		remaining := time.Until(req.Deadline)
		if remaining <= 0 {
			return TranscribeResult{}, fmt.Errorf("%w: deadline passed", ErrRuntimeUnavailable)
		}
		if remaining < deadline {
			deadline = remaining
		}
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return TranscribeResult{}, ErrRuntimeClosed
	}
	if s.stdin == nil {
		s.mu.Unlock()
		return TranscribeResult{}, fmt.Errorf("%w: not loaded", ErrRuntimeUnavailable)
	}

	if _, err := s.stdin.Write(reqBytes); err != nil {
		s.mu.Unlock()
		return TranscribeResult{}, fmt.Errorf("%w: write: %v", ErrRuntimeUnavailable, err)
	}

	respCh := make(chan adapterResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		if s.scanner.Scan() {
			var resp adapterResponse
			if err := json.Unmarshal(s.scanner.Bytes(), &resp); err != nil {
				errCh <- fmt.Errorf("decode: %w", err)
				return
			}
			respCh <- resp
		} else {
			if err := s.scanner.Err(); err != nil {
				errCh <- fmt.Errorf("read: %w", err)
			} else {
				errCh <- errors.New("subprocess closed")
			}
		}
	}()
	s.mu.Unlock()

	select {
	case resp := <-respCh:
		if resp.Error != "" {
			return TranscribeResult{}, fmt.Errorf("%w: %s", ErrRuntimeUnavailable, resp.Error)
		}
		if resp.RequestID != req.RequestID {
			return TranscribeResult{}, fmt.Errorf("%w: id mismatch %q!=%q",
				ErrRuntimeUnavailable, resp.RequestID, req.RequestID)
		}
		result := TranscribeResult{
			Text:       resp.Text,
			Confidence: resp.Confidence,
		}
		for _, alt := range resp.Alternatives {
			result.Alternatives = append(result.Alternatives, TranscriptAlternative{
				Text:       alt.Text,
				Confidence: alt.Confidence,
			})
		}
		return result, nil

	case err := <-errCh:
		return TranscribeResult{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, err)

	case <-ctx.Done():
		return TranscribeResult{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, ctx.Err())

	case <-time.After(deadline):
		return TranscribeResult{}, fmt.Errorf("%w: per-call deadline", ErrRuntimeUnavailable)
	}
}

// encodeFloat32B64 encodes float32 samples as base64-encoded
// little-endian bytes. This is the inverse of the Python adapter's
// np.frombuffer(base64.b64decode(s), dtype="<f4").
func encodeFloat32B64(samples []float32) string {
	buf := make([]byte, len(samples)*4)
	for i, s := range samples {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(s))
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// adapterStderrBuf is a bounded buffer for subprocess stderr.
type adapterStderrBuf struct {
	mu   sync.Mutex
	data []byte
}

func (b *adapterStderrBuf) Write(p []byte) (int, error) {
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

func (b *adapterStderrBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}

// Ensure the SubprocessRuntime interface is compatible.
var _ io.Closer = (*SubprocessRuntime)(nil)
