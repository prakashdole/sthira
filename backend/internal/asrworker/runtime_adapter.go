// runtime_adapter.go implements the real SubprocessRuntime backed
// by the IndicConformer-600M-Multi Python adapter.
//
// Architecture (B3-corrected):
//
//   - On LoadModel(), the runtime spawns a long-lived Python
//     subprocess running the adapter module.
//   - Communication is line-delimited JSON over stdin/stdout (JSONL).
//   - One goroutine owns the subprocess stdout and routes each
//     response to the matching pending request via request_id.
//   - The whole exchange (write + register + await) is bounded by
//     a per-call deadline and a caller-supplied context.
//   - The subprocess is killed on Close() or on unrecoverable
//     protocol error.
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
package asrworker

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	adapterStartupTimeout = 30 * time.Second
	adapterPerCallTimeout = 15 * time.Second
)

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

// SubprocessRuntime is the typed subprocess-backed Runtime. The
// worker constructs it via NewSubprocessRuntime and calls
// LoadModel once before exposing the worker to traffic.
type SubprocessRuntime struct {
	cfg    SubprocessRuntimeConfig
	mu     sync.Mutex
	closed bool

	// demux is the shared IPC dispatcher. Nil until LoadModel
	// succeeds.
	demux *ipcDispatcher

	revision   string
	digest     string
	digestName string
	languages  []string
}

// LoadModel spawns the Python subprocess and waits for the "ready"
// response. Populates revision, digest, and supported languages.
// On any failure the subprocess is reaped and the runtime stays
// in the not-loaded state.
func (s *SubprocessRuntime) LoadModel() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrRuntimeClosed
	}
	if s.demux != nil {
		s.mu.Unlock()
		return errors.New("model already loaded")
	}
	s.mu.Unlock()

	probe := adapterRequest{Op: "ready"}
	disp, info, err := loadIPCRuntime(s.cfg, probe)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.demux = disp
	s.revision = info.Revision
	s.digest = info.DigestSHA256
	s.digestName = info.DigestName
	s.languages = append([]string(nil), info.Languages...)
	s.mu.Unlock()
	return nil
}

// transcribeViaAdapter sends a transcribe request to the loaded
// Python subprocess.
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

	resp, err := s.demux.SendRequest(adReq, req.RequestID, deadline, ctx)
	if err != nil {
		return TranscribeResult{}, err
	}
	if resp.Error != "" {
		return TranscribeResult{}, fmt.Errorf("%w: %s", ErrRuntimeUnavailable, resp.Error)
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

// Transcribe implements Runtime. Dispatches to the loaded Python
// adapter when available; returns ErrRuntimeUnavailable otherwise.
func (s *SubprocessRuntime) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return TranscribeResult{}, ErrRuntimeClosed
	}
	loaded := s.demux != nil
	s.mu.Unlock()

	if loaded {
		return s.transcribeViaAdapter(ctx, req)
	}
	return TranscribeResult{}, fmt.Errorf("%w: subprocess runtime not loaded; call LoadModel first", ErrRuntimeUnavailable)
}

// SupportedLanguages implements Runtime. Returns the languages
// reported by the Python adapter after LoadModel, or nil if not
// loaded.
func (s *SubprocessRuntime) SupportedLanguages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.languages...)
}

// Revision implements Runtime. Empty until LoadModel populates it.
func (s *SubprocessRuntime) Revision() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}

// Digest implements Runtime. Empty until LoadModel populates it.
func (s *SubprocessRuntime) Digest() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.digestName, s.digest
}

// Close implements Runtime. Sends shutdown to the subprocess if
// loaded, then kills the process. Idempotent.
func (s *SubprocessRuntime) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	demux := s.demux
	s.demux = nil
	s.mu.Unlock()
	if demux != nil {
		return demux.Close()
	}
	return nil
}

// NewSubprocessRuntime returns the structural stub. Constructing it
// does NOT spawn anything; the subprocess spawns on LoadModel().
// Until then, Transcribe returns ErrRuntimeUnavailable so the worker
// surfaces UNAVAILABLE to the caller without ever invoking real
// inference.
func NewSubprocessRuntime(cfg SubprocessRuntimeConfig) *SubprocessRuntime {
	return &SubprocessRuntime{
		cfg:      cfg,
		revision: "",
		digest:   "",
	}
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
	return string(b.data)
}

// Unused import guard for encoding/json (kept available for
// future envelope changes).
var _ = json.Marshal
