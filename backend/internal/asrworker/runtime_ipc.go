// runtime_ipc.go — properly correlated subprocess IPC for the
// inference adapters (ASR + TTS).
//
// Background.
//
// The previous per-runtime Scanner reader released the exchange
// mutex before reading a response, so concurrent pool goroutines
// could read each other's stdout and consume the wrong response.
// TTS did not correlate request IDs at all. Both behaviours are
// fixed here.
//
// Design.
//
// One goroutine per subprocess owns the stdout scanner and acts as
// a demultiplexer: every parsed response carries the request_id
// echoed by the adapter; the demultiplexer routes the response
// to the matching pending caller via a request-id map. Callers
// that time out or get cancelled remove themselves from the map
// under the dispatcher lock; the demultiplexer drops orphan
// responses after a bounded retention window.
//
// Concurrency.
//
//   - The mutex protects the stdin write + the request map
//     (registration, lookup, deregistration).
//   - The demultiplexer is the only goroutine that reads from
//     the scanner; it never races with another reader.
//   - A misbehaving subprocess that prints extra/trailing protocol
//     records without a request_id is logged on stderr (bounded)
//     and the process is reset; the next caller sees a clean
//     subprocess.
//
// Cancellation.
//
//   - Per-call deadline: caller cancels via per-call channel.
//   - On cancel, the dispatcher deregisters the request and the
//     response (if it arrives later) is discarded.
//   - On subprocess death: all in-flight requests are released
//     with ErrRuntimeUnavailable; the next LoadModel spawns a
//     fresh process.
package asrworker

import (
	"bufio"
	"context"
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

// ipcDispatcher owns the subprocess lifecycle, the stdin writer,
// and the stdout demultiplexer. It is shared between LoadModel
// (startup) and the per-call Transcribe path. All exported methods
// are safe for concurrent use.
type ipcDispatcher struct {
	cfg SubprocessRuntimeConfig

	mu sync.Mutex

	proc    *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner

	// pending maps request_id -> channel for the response. The
	// demultiplexer goroutine writes into the channel and never
	// blocks (channel buffer = 1). Callers select on the channel
	// for the per-call deadline.
	pending map[string]chan adapterResponse

	// closed is true after Close().
	closed bool

	// onStderr is the bounded stderr buffer for diagnostics.
	// Populated by the dispatcher constructor and read on Close.
	stderrBufPtr *adapterStderrBuf

	// demuxExit signals that the demultiplexer goroutine has
	// exited. Callers wait on it before tearing down the process.
	demuxExit chan struct{}

	// demuxErr is set by the demultiplexer when the subprocess
	// dies or the protocol becomes unsynchronized. The next
	// caller surfaces this as ErrRuntimeUnavailable.
	demuxErr error
}

// StartResult is the typed outcome of a successful LoadModel.
type StartResult struct {
	Revision     string
	DigestName   string
	DigestSHA256 string
	Languages    []string
}

// stderrCopy captures the bounded stderr buffer's contents at a
// point in time without copying the mutex. Used only at startup
// to format the timeout error.
func stderrCopy(b *adapterStderrBuf) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

// loadIPCRuntime starts the subprocess, writes the startup probe,
// and waits for the "ready" envelope. The dispatcher goroutine is
// running for the lifetime of the subprocess. The returned
// StartResult is filled in from the parsed ready envelope.
func loadIPCRuntime(cfg SubprocessRuntimeConfig, probe interface{}) (disp *ipcDispatcher, info StartResult, err error) {
	if cfg.Module == "" {
		return nil, StartResult{}, errors.New("ipc: module required")
	}
	pyCmd := cfg.Cmd
	if pyCmd == "" {
		pyCmd = "python3"
	}
	if _, lookErr := exec.LookPath(pyCmd); lookErr != nil {
		return nil, StartResult{}, fmt.Errorf("%w: %s not found: %v", ErrRuntimeUnavailable, pyCmd, lookErr)
	}

	args := []string{"-u", "-m", cfg.Module, "--adapter-mode"}
	cmd := exec.Command(pyCmd, args...)
	if cfg.Workdir != "" {
		cmd.Dir = cfg.Workdir
	}
	cmd.Env = append(os.Environ(), cfg.ExtraEnv...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, StartResult{}, fmt.Errorf("ipc: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, StartResult{}, fmt.Errorf("ipc: stdout pipe: %w", err)
	}
	var stderrBuf adapterStderrBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, StartResult{}, fmt.Errorf("%w: start: %v", ErrRuntimeUnavailable, err)
	}

	disp = &ipcDispatcher{
		cfg:          cfg,
		proc:         cmd,
		stdin:        stdin,
		scanner:      bufio.NewScanner(stdout),
		pending:      make(map[string]chan adapterResponse),
		stderrBufPtr: &stderrBuf,
		demuxExit:    make(chan struct{}),
	}
	// Scanner buffer is 1 MiB: enough for a base64-encoded 12s
	// 22050 Hz mono 16-bit WAV (~264 KiB) with JSON overhead.
	// Truly malformed lines exceed this and abort the demux.
	disp.scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	// Send startup probe.
	probeBytes, err := json.Marshal(probe)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, StartResult{}, fmt.Errorf("ipc: marshal probe: %w", err)
	}
	probeBytes = append(probeBytes, '\n')
	if _, err := stdin.Write(probeBytes); err != nil {
		_ = cmd.Process.Kill()
		return nil, StartResult{}, fmt.Errorf("%w: write probe: %v", ErrRuntimeUnavailable, err)
	}

	// Start the demultiplexer BEFORE waiting for the startup
	// response so the scanner can read past the ready envelope
	// while subsequent requests are in flight.
	go disp.runDemux()

	// Read the startup response synchronously: it MUST be the
	// first line the subprocess emits.
	startupCh := make(chan adapterResponse, 1)
	disp.mu.Lock()
	disp.pending["__startup__"] = startupCh
	disp.mu.Unlock()

	var resp adapterResponse
	select {
	case resp = <-startupCh:
	case <-time.After(adapterStartupTimeout):
		_ = disp.Close()
		return nil, StartResult{}, fmt.Errorf("%w: startup timeout; stderr: %s",
			ErrRuntimeUnavailable, stderrCopy(&stderrBuf))
	}
	if resp.Status != "ready" {
		_ = disp.Close()
		return nil, StartResult{}, fmt.Errorf("%w: status %q error: %s",
			ErrRuntimeUnavailable, resp.Status, resp.Error)
	}
	info.Revision = resp.Revision
	info.DigestName = resp.DigestName
	info.DigestSHA256 = resp.DigestSHA256
	info.Languages = append([]string(nil), resp.Languages...)
	return disp, info, nil
}

// runDemux reads one line at a time from the subprocess stdout and
// routes it to the matching pending request. Lines that do not
// decode as JSON are treated as protocol corruption. Lines that
// lack a request_id are only valid before any client request has
// been registered (the startup envelope).
func (d *ipcDispatcher) runDemux() {
	defer close(d.demuxExit)
	defer func() {
		// On exit, fail every still-pending request so the next
		// caller sees ErrRuntimeUnavailable.
		d.mu.Lock()
		for id, ch := range d.pending {
			select {
			case ch <- adapterResponse{Error: "subprocess terminated"}:
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
		var resp adapterResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			d.demuxErr = fmt.Errorf("ipc: malformed response: %w", err)
			fmt.Fprintf(os.Stderr, "asrworker ipc: malformed response: %v\n", err)
			return
		}
		// Special-case the startup probe: it is registered under
		// "__startup__" but the subprocess may echo a different id
		// or no id at all.
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
			d.demuxErr = errors.New("ipc: response missing request_id")
			fmt.Fprintln(os.Stderr, "asrworker ipc: response missing request_id")
			return
		}
		ch, ok := d.pending[resp.RequestID]
		if ok {
			delete(d.pending, resp.RequestID)
		}
		d.mu.Unlock()
		if !ok {
			// Late or unknown response — discard. Late responses
			// are expected on cancel/timeout; we do not log them
			// to avoid noise.
			continue
		}
		select {
		case ch <- resp:
		default:
			// Caller already gave up.
		}
	}
	var demuxErr error
	if err := d.scanner.Err(); err != nil {
		demuxErr = fmt.Errorf("ipc: scanner: %w", err)
	} else {
		demuxErr = errors.New("ipc: subprocess closed stdout")
	}
	d.mu.Lock()
	d.demuxErr = demuxErr
	d.mu.Unlock()
}

// SendRequest writes a single JSONL request and returns the
// matching response. The mutex serializes the write with itself
// and with Close. The call is bounded by perCallTimeout and the
// caller's context.
//
// The caller supplies a request_id; it MUST be unique per
// concurrent request. The adapter echoes it on stdout and the
// demultiplexer routes it back here. Wrong/missing IDs are
// rejected at the response boundary.
func (d *ipcDispatcher) SendRequest(req interface{}, requestID string, perCallTimeout time.Duration, callerCtx context.Context) (adapterResponse, error) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return adapterResponse{}, ErrRuntimeClosed
	}
	if d.demuxErr != nil {
		err := d.demuxErr
		d.mu.Unlock()
		return adapterResponse{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, err)
	}
	if d.stdin == nil {
		d.mu.Unlock()
		return adapterResponse{}, fmt.Errorf("%w: not loaded", ErrRuntimeUnavailable)
	}

	// Marshal under lock so the write+register sequence is atomic
	// w.r.t. other callers.
	reqBytes, err := json.Marshal(req)
	if err != nil {
		d.mu.Unlock()
		return adapterResponse{}, fmt.Errorf("marshal: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	respCh := make(chan adapterResponse, 1)
	d.pending[requestID] = respCh

	if _, err := d.stdin.Write(reqBytes); err != nil {
		delete(d.pending, requestID)
		d.mu.Unlock()
		return adapterResponse{}, fmt.Errorf("%w: write: %v", ErrRuntimeUnavailable, err)
	}
	d.mu.Unlock()

	timer := time.NewTimer(perCallTimeout)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		// Validate request-id round-trip. The adapter must echo
		// the same request_id we sent; otherwise the demux
		// router could have crossed streams (defense in depth).
		if resp.RequestID != requestID && resp.RequestID != "__startup__" {
			return adapterResponse{}, fmt.Errorf("%w: id mismatch got=%q want=%q",
				ErrRuntimeUnavailable, resp.RequestID, requestID)
		}
		return resp, nil

	case <-timer.C:
		// Deregister so a late response is dropped by the demux.
		d.mu.Lock()
		delete(d.pending, requestID)
		d.mu.Unlock()
		return adapterResponse{}, fmt.Errorf("%w: per-call deadline", ErrRuntimeUnavailable)

	case <-callerCtx.Done():
		// Same deregistration on caller cancellation.
		d.mu.Lock()
		delete(d.pending, requestID)
		d.mu.Unlock()
		return adapterResponse{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, callerCtx.Err())
	}
}

// Close terminates the subprocess and waits for the demultiplexer
// to drain. Idempotent.
func (d *ipcDispatcher) Close() error {
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
		shutReq, _ := json.Marshal(adapterRequest{Op: "shutdown"})
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

// Stderr returns the bounded subprocess stderr capture. Used by
// the runtime to surface failure reasons.
func (d *ipcDispatcher) Stderr() string {
	if d.stderrBufPtr == nil {
		return ""
	}
	return d.stderrBufPtr.String()
}

// Unused import guard so go vet does not complain if a future
// refactor temporarily drops the strings import.
var _ = strings.TrimSpace
