package asrworker

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Compressed audio decoding via an external decoder (ffmpeg or
// opusdec). The worker accepts Ogg/Opus, WebM/Opus and WAV at the
// boundary. WAV is handled in-process (see DecodeWAV); compressed
// formats are transcoded to mono 16-bit PCM WAV by a bounded
// subprocess and then decoded by DecodeWAV.
//
// Why subprocess: the asrworker module has no external Go dependencies
// (go.mod is stdlib-only). A Go Opus decoder would add a CGo binding
// or a large pure-Go port, both of which violate the module's
// isolation constraint. ffmpeg is widely available, well-audited, and
// runs in a child process with strict resource limits.
//
// Security: the input is passed via stdin (no temp files, no shell
// expansion). The subprocess runs with a wall-clock deadline and
// bounded stdout/stderr sizes.

// CompressedContentTypes lists the codecs the worker transcodes via
// subprocess before WAV decode.
var CompressedContentTypes = []string{
	"audio/ogg",
	"audio/webm",
	"audio/opus",
	"audio/ogg; codecs=opus",
	"audio/webm; codecs=opus",
}

func init() {
	// Extend WorkerSupportedContentTypes to include compressed
	// codecs so the codec check accepts them.
	WorkerSupportedContentTypes = append(WorkerSupportedContentTypes, CompressedContentTypes...)
}

// isCompressedCodec reports whether the content-type requires
// subprocess transcode.
func isCompressedCodec(ct string) bool {
	ct = strings.TrimSpace(strings.ToLower(ct))
	for _, t := range CompressedContentTypes {
		if ct == t {
			return true
		}
	}
	return false
}

// TranscoderBinary is the external transcoder. Tests can override.
var TranscoderBinary = "ffmpeg"

// transcoderMaxStdout bounds the WAV output from ffmpeg. 20s at
// 16 kHz mono 16-bit = 640,000 sample bytes + 44 header ≈ 640 KiB.
// We allow 1 MiB to cover rounding.
const transcoderMaxStdout = 1 << 20 // 1 MiB

// transcoderMaxStderr bounds the diagnostic output.
const transcoderMaxStderr = 8 * 1024

// transcoderDeadline is the wall-clock limit for the subprocess.
const transcoderDeadline = 10 * time.Second

// ErrTranscoderUnavailable is returned when the external transcoder
// binary is not installed.
var ErrTranscoderUnavailable = errors.New("external transcoder not available")

// ErrTranscodeFailed is returned on non-zero exit or output error.
var ErrTranscodeFailed = errors.New("transcode failed")

// DecodeCompressed transcodes a compressed audio blob (Ogg/Opus or
// WebM/Opus) to PCM WAV via ffmpeg, then decodes the WAV. The
// contentType must be one of CompressedContentTypes; callers should
// have already checked isCompressedCodec.
//
// The subprocess receives compressed bytes on stdin and writes mono
// 16-bit LE WAV to stdout. No temporary files are created.
//
// Resource limits:
//   - CompressedBytes: enforced before subprocess spawn
//   - transcoderDeadline: wall-clock subprocess timeout
//   - transcoderMaxStdout: stdout byte cap
//   - DecodedMonoSeconds: enforced by downstream DecodeWAV
func DecodeCompressed(audioBytes []byte, contentType string, limits AudioDecodeLimits) (*AudioDecodeResult, error) {
	if int64(len(audioBytes)) > limits.CompressedBytes {
		return nil, &DecodeError{Reason: "compressed audio bytes exceed limit",
			Cause: fmt.Errorf("got %d > %d", len(audioBytes), limits.CompressedBytes)}
	}
	if len(audioBytes) == 0 {
		return nil, &DecodeError{Reason: "audio bytes are empty"}
	}

	// Verify ffmpeg is available.
	if _, err := exec.LookPath(TranscoderBinary); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrTranscoderUnavailable, TranscoderBinary, err)
	}

	// Build the ffmpeg command:
	//   ffmpeg -nostdin -hide_banner -loglevel error
	//          -i pipe:0
	//          -f wav -acodec pcm_s16le -ac 1 -ar 16000
	//          pipe:1
	//
	// -i pipe:0 reads from stdin.
	// Output is mono 16-bit LE WAV at 16 kHz written to stdout.
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-f", "wav",
		"-acodec", "pcm_s16le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", TargetSampleRate),
		"pipe:1",
	}

	cmd := exec.Command(TranscoderBinary, args...)
	cmd.Stdin = bytes.NewReader(audioBytes)

	// Bounded stdout capture.
	var stdout bytes.Buffer
	stdout.Grow(transcoderMaxStdout)
	cmd.Stdout = &limitedWriter{buf: &stdout, limit: transcoderMaxStdout}

	// Bounded stderr capture.
	var stderr bytes.Buffer
	stderr.Grow(transcoderMaxStderr)
	cmd.Stderr = &limitedWriter{buf: &stderr, limit: transcoderMaxStderr}

	// Start with deadline.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: start: %v", ErrTranscodeFailed, err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("%w: exit: %v; stderr: %s",
				ErrTranscodeFailed, err, strings.TrimSpace(stderr.String()))
		}
	case <-time.After(transcoderDeadline):
		_ = cmd.Process.Kill()
		<-done
		return nil, fmt.Errorf("%w: deadline exceeded (%v)",
			ErrTranscodeFailed, transcoderDeadline)
	}

	if stdout.Len() == 0 {
		return nil, &DecodeError{Reason: "transcoder produced no output"}
	}

	// Decode the WAV produced by ffmpeg.
	result, err := DecodeWAV(stdout.Bytes(), "audio/wav", limits)
	if err != nil {
		return nil, err
	}
	// Override the content type to reflect the original compressed
	// input.
	result.ContentType = contentType
	result.BytesConsumed = len(audioBytes)
	return result, nil
}

// limitedWriter wraps a bytes.Buffer with a byte cap. Writes beyond
// the limit are silently discarded; the actual content up to the
// limit is preserved.
type limitedWriter struct {
	buf     *bytes.Buffer
	limit   int
	written int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.written
	if remaining <= 0 {
		return len(p), nil // silently discard
	}
	n := len(p)
	if n > remaining {
		n = remaining
	}
	w.buf.Write(p[:n])
	w.written += n
	return len(p), nil
}

// DecodeAudio is the unified entry point for audio decoding. It
// dispatches to DecodeWAV for WAV input and DecodeCompressed for
// Ogg/Opus/WebM input. This is the function the worker calls
// instead of DecodeWAV directly when compressed codecs are enabled.
func DecodeAudio(audioBytes []byte, contentType string, limits AudioDecodeLimits) (*AudioDecodeResult, error) {
	ct := strings.TrimSpace(strings.ToLower(contentType))
	if !isWorkerSupportedCodec(ct) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCodec, contentType)
	}
	if isCompressedCodec(ct) {
		return DecodeCompressed(audioBytes, contentType, limits)
	}
	return DecodeWAV(audioBytes, contentType, limits)
}
