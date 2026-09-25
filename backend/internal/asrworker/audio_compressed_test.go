package asrworker

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestDecodeAudio_RejectsUnknownCodec ensures the dispatch entry
// point rejects a content type it does not understand.
func TestDecodeAudio_RejectsUnknownCodec(t *testing.T) {
	_, err := DecodeAudio([]byte{0, 1, 2}, "audio/aac", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected error for unknown codec")
	}
	if !errors.Is(err, ErrUnsupportedCodec) {
		t.Errorf("expected ErrUnsupportedCodec, got %v", err)
	}
}

// TestDecodeAudio_DispatchesToWAV verifies the dispatch returns
// the same shape as DecodeWAV when given audio/wav input. We pass
// a tiny but valid WAV.
func TestDecodeAudio_DispatchesToWAV(t *testing.T) {
	wav := silenceBuffer(t, 100, 16_000)
	res, err := DecodeAudio(wav, "audio/wav", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("DecodeAudio wav: %v", err)
	}
	if res == nil || res.SampleRate != TargetSampleRate {
		t.Fatalf("bad result: %+v", res)
	}
}

// TestDecodeCompressed_FailsWhenFFmpegMissing enforces the
// fail-closed behavior when the external transcoder is unavailable.
// We set TranscoderBinary to a guaranteed-missing path.
func TestDecodeCompressed_FailsWhenFFmpegMissing(t *testing.T) {
	prev := TranscoderBinary
	TranscoderBinary = "/nonexistent/sthira-transcoder-" + t.Name()
	t.Cleanup(func() { TranscoderBinary = prev })

	_, err := DecodeCompressed([]byte{0, 1, 2}, "audio/ogg", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected error when transcoder is missing")
	}
	if !errors.Is(err, ErrTranscoderUnavailable) {
		t.Errorf("expected ErrTranscoderUnavailable, got %v", err)
	}
}

// TestDecodeCompressed_RejectsEmptyBytes checks the size-zero
// boundary.
func TestDecodeCompressed_RejectsEmptyBytes(t *testing.T) {
	prev := TranscoderBinary
	TranscoderBinary = "/nonexistent/sthira-transcoder-" + t.Name()
	t.Cleanup(func() { TranscoderBinary = prev })

	_, err := DecodeCompressed(nil, "audio/ogg", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected error for empty bytes")
	}
	if !errors.Is(err, ErrUnsupportedCodec) && !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-bytes error, got %v", err)
	}
}

// TestDecodeCompressed_RejectsOversizeBytes checks the size cap is
// applied before subprocess spawn.
func TestDecodeCompressed_RejectsOversizeBytes(t *testing.T) {
	prev := TranscoderBinary
	TranscoderBinary = "/nonexistent/sthira-transcoder-" + t.Name()
	t.Cleanup(func() { TranscoderBinary = prev })

	huge := make([]byte, int(DefaultDecodeLimits().CompressedBytes)+1)
	_, err := DecodeCompressed(huge, "audio/ogg", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected error for oversize input")
	}
}

// TestIsCompressedCodec_TrueOnOpusFormats exercises the codec check
// for all declared compressed codecs.
func TestIsCompressedCodec_TrueOnOpusFormats(t *testing.T) {
	for _, ct := range CompressedContentTypes {
		if !isCompressedCodec(ct) {
			t.Errorf("expected true for %q", ct)
		}
	}
}

// TestIsCompressedCodec_FalseOnWAV ensures WAV is not in the
// compressed set.
func TestIsCompressedCodec_FalseOnWAV(t *testing.T) {
	if isCompressedCodec("audio/wav") {
		t.Errorf("WAV must not be a compressed codec")
	}
	if isCompressedCodec("audio/mp3") {
		t.Errorf("MP3 must not be a compressed codec")
	}
}

// TestDecodeAudio_RejectsEmptyForCompressed mirrors the WAV empty
// guard for compressed codecs. The transcoder probe happens first
// when transcoder is missing; when present, the empty check should
// also fire. With a missing transcoder we expect the upstream error.
func TestDecodeAudio_RejectsCompressedEmptyWithMissingTranscoder(t *testing.T) {
	prev := TranscoderBinary
	TranscoderBinary = "/nonexistent/sthira-transcoder-" + t.Name()
	t.Cleanup(func() { TranscoderBinary = prev })

	_, err := DecodeAudio(nil, "audio/ogg", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestLimitedWriter_TruncatesAndReportsFull ensures the bounded
// stdout writer truncates but reports the full write length so the
// subprocess sees its output was consumed.
func TestLimitedWriter_TruncatesAndReportsFull(t *testing.T) {
	buf := new(bytes.Buffer)
	w := &limitedWriter{buf: buf, limit: 5}
	payload := []byte("ABCDEFGHIJK")
	n, err := w.Write(payload)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(payload) {
		t.Errorf("reported n: got %d want %d", n, len(payload))
	}
	if got := buf.String(); got != "ABCDE" {
		t.Errorf("buf: got %q want %q", got, "ABCDE")
	}
}

func TestIsWorkerSupportedCodec_BrowserMIMETypes(t *testing.T) {
	valid := []string{
		"audio/wav",
		"audio/webm",
		"audio/webm;codecs=opus",
		"audio/webm; codecs=opus",
		"audio/webm; codecs=\"opus\"",
		"audio/ogg",
		"audio/ogg;codecs=opus",
		"audio/ogg; codecs=opus",
		"audio/opus",
	}
	for _, ct := range valid {
		if !isWorkerSupportedCodec(ct) {
			t.Errorf("expected %q to be supported by worker", ct)
		}
	}

	invalid := []string{
		"audio/webm;codecs=vorbis",
		"audio/aac",
		"audio/mp3",
		"audio/mp4",
		"text/plain",
	}
	for _, ct := range invalid {
		if isWorkerSupportedCodec(ct) {
			t.Errorf("expected %q to be rejected by worker", ct)
		}
	}
}

func TestDecodeAudio_RejectsUnsupportedCodecParam(t *testing.T) {
	_, err := DecodeAudio([]byte{1, 2, 3}, "audio/webm;codecs=vorbis", DefaultDecodeLimits())
	if !errors.Is(err, ErrUnsupportedCodec) {
		t.Errorf("expected ErrUnsupportedCodec for vorbis, got %v", err)
	}
}
