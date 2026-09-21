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
// responses.
//
// Concurrency.
//
//   - The mutex protects the request map and the dispatcher state.
//     It is NEVER held across a stdin write: a wedged child that
//     stops reading fills the OS pipe and blocks os.File.Write,
//     and a mutex held across that write would hang every other
//     caller and Close(). Writers instead take a bounded path
//     (see Boundedness).
//   - The demultiplexer is the only goroutine that reads from the
//     scanner; it never races with another reader.
//   - demuxErr is written only under the mutex.
//
// Boundedness.
//
//   - Startup correlation is registered BEFORE the reader can
//     deliver it, so an adapter that answers the ready probe
//     instantly is never mistaken for an unsolicited response.
//   - Every stdin write (startup probe, per-call request,
//     shutdown) runs in a detached goroutine guarded by a
//     deadline that starts before the write. If the budget
//     elapses the dispatcher kills the child, which converts the
//     blocked write into an error and releases the goroutine.
//     Request and Close therefore return inside bounded time
//     even against a non-reading child.
//   - Malformed/duplicate/mismatched/unsolicited responses,
//     cancellation and child exit all release or reset the
//     affected streams without leaking responses across requests;
//     the dead dispatcher is replaced by the next LoadModel.
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
	// caller surfaces this as ErrRuntimeUnavailable. ALWAYS
	// read/written under mu.
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

// writeStdin performs a single JSONL write against the dispatcher's
// stdin with a deadline that covers the WRITE ITSELF, not just the
// response wait. A child that stops reading eventually fills the
// OS pipe buffer; os.File.Write would otherwise block forever and
// (previously) with d.mu held, wedging every caller and Close().
//
// The write runs in a detached goroutine. Two outcomes release it:
//   - budget expiry: the child cannot drain input, so it cannot
//     serve ANY request; the dispatcher kills it (markUncertain),
//     which converts the blocked write into an error and reaps
//     the goroutine. The whole dispatcher then fails closed.
//   - caller cancellation while the write is still pending: the
//     caller is allowed to return immediately; the child is NOT
//     killed (the write may be legitimately in progress), the
//     orphan response is dropped by the deregistered pending
//     entry, and the writer goroutine terminates when the child
//     drains or when Close kills it. Either way nothing outlives
//     the dispatcher.
func (d *ipcDispatcher) writeStdin(data []byte, budget time.Duration, cancel <-chan struct{}) error {
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
		d.markUncertain(errors.New("stdin write deadline; child not draining input"))
		<-done // the kill makes the write fail; do not leak the goroutine
		return fmt.Errorf("%w: stdin write deadline", ErrRuntimeUnavailable)
	case <-cancel:
		return fmt.Errorf("%w: canceled during stdin write", ErrRuntimeUnavailable)
	}
}

// markUncertain records a protocol-lifetime failure under the
// mutex, releases every pending caller, and kills the child so
// the demultiplexer observes EOF and the writer goroutines
// unblock. Idempotent; safe from any goroutine.
func (d *ipcDispatcher) markUncertain(reason error) {
	d.mu.Lock()
	if d.demuxErr == nil {
		d.demuxErr = reason
	}
	for id, ch := range d.pending {
		select {
		case ch <- adapterResponse{Error: "subprocess reset"}:
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

// loadIPCRuntime starts the subprocess, registers the startup
// correlation, starts the demultiplexer, and writes the startup
// probe under the startup budget. The returned StartResult is
// filled in from the parsed ready envelope.
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

	startupBudget := cfg.StartupTimeout
	if startupBudget <= 0 {
		startupBudget = adapterStartupTimeout
	}

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

	// Register startup correlation BEFORE the reader exists. The
	// adapter may answer the ready probe immediately; the old
	// order (start demux, then register) could drop that first
	// line as "response missing request_id" and poison startup.
	startupCh := make(chan adapterResponse, 1)
	disp.mu.Lock()
	disp.pending["__startup__"] = startupCh
	disp.mu.Unlock()

	go disp.runDemux()

	probeBytes, err := json.Marshal(probe)
	if err != nil {
		_ = disp.Close()
		return nil, StartResult{}, fmt.Errorf("ipc: marshal probe: %w", err)
	}
	probeBytes = append(probeBytes, '\n')
	handshakeStarted := time.Now()
	if err := disp.writeStdin(probeBytes, startupBudget, nil); err != nil {
		_ = disp.Close()
		return nil, StartResult{}, err
	}

	var resp adapterResponse
	select {
	case resp = <-startupCh:
	case <-time.After(startupBudget - time.Since(handshakeStarted)):
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
// lack a request_id are only valid for the startup envelope (which
// is registered BEFORE this goroutine starts, so the ordering is
// deterministic).
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
		if d.demuxErr == nil {
			if err := d.scanner.Err(); err != nil {
				d.demuxErr = fmt.Errorf("ipc: scanner: %w", err)
			} else {
				d.demuxErr = errors.New("ipc: subprocess closed stdout")
			}
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
			// Protocol corruption: stop reading and reset the
			// streams. markUncertain releases every pending
			// caller and kills the child so no response can be
			// cross-delivered later.
			fmt.Fprintf(os.Stderr, "asrworker ipc: malformed response: %v\n", err)
			d.markUncertain(fmt.Errorf("ipc: malformed response: %w", err))
			return
		}
		d.mu.Lock()
		if resp.RequestID == "" {
			// Only the startup envelope may be unsolicited; the
			// startup correlation is registered before this
			// goroutine runs, so if it is not (or no longer)
			// pending, any later id-less line is unsynchronized
			// protocol output.
			if startupCh, ok := d.pending["__startup__"]; ok {
				delete(d.pending, "__startup__")
				d.mu.Unlock()
				startupCh <- resp // buffer 1; receiver already selected
				continue
			}
			d.mu.Unlock()
			fmt.Fprintln(os.Stderr, "asrworker ipc: response missing request_id")
			d.markUncertain(errors.New("ipc: response missing request_id"))
			return
		}
		ch, ok := d.pending[resp.RequestID]
		if ok {
			delete(d.pending, resp.RequestID)
		}
		d.mu.Unlock()
		if !ok {
			// Late or unknown response — discard. Late responses
			// are expected on cancel/timeout; duplicates from a
			// misbehaving child land here too. Neither may reach
			// a different caller.
			continue
		}
		select {
		case ch <- resp:
		default:
			// Caller already gave up.
		}
	}
	// Scanner finished (EOF or error); the deferred cleanup
	// classifies it.
}

// SendRequest writes a single JSONL request and returns the
// matching response. The deadline covers registration, the WRITE
// (which can block on a wedged child) and the response wait; the
// caller's context participates in all three. No mutex is held
// across the blocking write.
//
// The caller supplies a request_id; it MUST be unique per
// concurrent request. The adapter echoes it on stdout and the
// demultiplexer routes it back here. Wrong/missing IDs are
// rejected at the response boundary.
func (d *ipcDispatcher) SendRequest(req interface{}, requestID string, perCallTimeout time.Duration, callerCtx context.Context) (adapterResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return adapterResponse{}, fmt.Errorf("marshal: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	started := time.Now()
	respCh := make(chan adapterResponse, 1)

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
	// Register BEFORE the write so the response can never arrive
	// uncorrelated.
	d.pending[requestID] = respCh
	d.mu.Unlock()

	deregister := func() {
		d.mu.Lock()
		delete(d.pending, requestID)
		d.mu.Unlock()
	}

	// The per-call budget covers registration + write + wait as a
	// whole: the write guard gets the remaining budget, and so
	// does the response wait.
	remaining := func() time.Duration {
		left := perCallTimeout - time.Since(started)
		if left < 0 {
			left = 0
		}
		return left
	}

	if err := d.writeStdin(reqBytes, remaining(), callerCtx.Done()); err != nil {
		deregister()
		if ctxErr := callerCtx.Err(); ctxErr != nil {
			return adapterResponse{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, ctxErr)
		}
		return adapterResponse{}, err
	}

	timer := time.NewTimer(remaining())
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
		deregister()
		return adapterResponse{}, fmt.Errorf("%w: per-call deadline", ErrRuntimeUnavailable)

	case <-callerCtx.Done():
		deregister()
		return adapterResponse{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, callerCtx.Err())
	}
}

// Close terminates the subprocess and waits for the demultiplexer
// to drain. Bounded: the shutdown write is best-effort under a
// short budget; whatever the child's state, the process is then
// killed and reaped, so Close never hangs on a wedged child.
// Idempotent.
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
	d.mu.Unlock()

	if stdin != nil {
		// Best-effort graceful shutdown: bounded to 1s and skipped
		// entirely on the first sign of a wedged child (writeStdin
		// kills + unblocks itself on overrun; here we simply do
		// not want Close to wait on cooperation).
		shutReq, _ := json.Marshal(adapterRequest{Op: "shutdown"})
		shutReq = append(shutReq, '\n')
		writeDone := make(chan struct{})
		go func() {
			_, _ = stdin.Write(shutReq)
			close(writeDone)
		}()
		select {
		case <-writeDone:
		case <-time.After(time.Second):
			// Child is wedged; fall through to kill.
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
