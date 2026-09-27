package asrworker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os/exec"
	"testing"
)

func genWebmOpusLocal(t *testing.T) []byte {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:a", "libopus", "-ac", "1", "-ar", "16000", "-b:a", "32k",
		"-f", "webm", "pipe:1")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("gen webm: %v", err)
	}
	return buf.Bytes()
}

func genOggOpusLocal(t *testing.T) []byte {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:a", "libopus", "-ac", "1", "-ar", "16000", "-b:a", "32k",
		"-f", "ogg", "pipe:1")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("gen ogg: %v", err)
	}
	return buf.Bytes()
}

// TestDecodeAudio_WebmWithOpusCodecParamAccepted: audio/webm;codecs=opus
// is routed to the compressed decoder (ffmpeg). The result must contain
// non-empty float32 samples at TargetSampleRate (16 000 Hz), mono, all finite.
func TestDecodeAudio_WebmWithOpusCodecParamAccepted(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t)
	res, err := DecodeAudio(webm, "audio/webm;codecs=opus", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("audio/webm;codecs=opus should decode: %v", err)
	}
	if len(res.Samples) == 0 {
		t.Fatal("samples must be non-empty")
	}
	if res.SampleRate != TargetSampleRate {
		t.Errorf("sample rate: got %d want %d", res.SampleRate, TargetSampleRate)
	}
	if res.ChannelsIn != 1 {
		t.Errorf("channels: got %d want 1 (mono)", res.ChannelsIn)
	}
	for i, s := range res.Samples {
		f := float64(s)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			t.Errorf("sample %d is not finite: %g", i, s)
		}
	}
	// Plausible duration: 1 s of 16 kHz mono = 16000 samples ≈ 1 s.
	if res.DurationSecs < 0.9 || res.DurationSecs > 1.5 {
		t.Errorf("duration: got %v, expected ~1 s", res.DurationSecs)
	}
}

// TestDecodeAudio_OggWithOpusCodecParamAccepted: audio/ogg;codecs=opus
// is routed to the compressed decoder (ffmpeg). Same structural checks as WebM.
func TestDecodeAudio_OggWithOpusCodecParamAccepted(t *testing.T) {
	ffmpegOrSkip(t)
	ogg := genOggOpusLocal(t)
	res, err := DecodeAudio(ogg, "audio/ogg;codecs=opus", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("audio/ogg;codecs=opus should decode: %v", err)
	}
	if len(res.Samples) == 0 {
		t.Fatal("samples must be non-empty")
	}
	if res.SampleRate != TargetSampleRate {
		t.Errorf("sample rate: got %d want %d", res.SampleRate, TargetSampleRate)
	}
	for _, s := range res.Samples {
		f := float64(s)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			t.Error("all samples must be finite")
			break
		}
	}
}

// TestDecodeAudio_WebmWithoutCodecParamAccepted: audio/webm (no codecs param)
// is also routed to the compressed decoder. Browser may send this fallback.
func TestDecodeAudio_WebmWithoutCodecParamAccepted(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t)
	res, err := DecodeAudio(webm, "audio/webm", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("audio/webm without codecs param should decode: %v", err)
	}
	if len(res.Samples) == 0 {
		t.Fatal("samples must be non-empty")
	}
	if res.SampleRate != TargetSampleRate {
		t.Errorf("sample rate: got %d want %d", res.SampleRate, TargetSampleRate)
	}
}

// TestDecodeAudio_AutodetectionAcceptsWebmBytesDeclaredAsOggOpus: when the
// MIME type declares audio/ogg;codecs=opus but the actual bytes are WebM/Opus,
// ffmpeg auto-detects the real container format from the byte stream and
// decodes successfully. This is observed behavior: ffmpeg relies on container
// magic bytes rather than the HTTP Content-Type header. The worker has no
// MIME-content consistency check. No contract requires MIME-to-bytes alignment.
func TestDecodeAudio_AutodetectionAcceptsWebmBytesDeclaredAsOggOpus(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t)
	res, err := DecodeAudio(webm, "audio/ogg;codecs=opus", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("ffmpeg auto-detects webm bytes despite ogg MIME label: %v", err)
	}
	if len(res.Samples) == 0 {
		t.Fatal("samples must be non-empty after autodetection")
	}
}

// TestDecodeAudio_RejectsWavWithNonPCMFormat: WAV with IEEE float samples
// (format 3) is not PCM 16-bit and must be rejected with a DecodeError whose
// reason contains "not PCM".
func TestDecodeAudio_RejectsWavWithNonPCMFormat(t *testing.T) {
	pcm := make([]byte, 16000)
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(36+len(pcm))))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(3))  // IEEE float
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(16000))
	binary.Write(buf, binary.LittleEndian, uint32(16000*4))
	binary.Write(buf, binary.LittleEndian, uint16(4))
	binary.Write(buf, binary.LittleEndian, uint16(32))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(pcm)))
	buf.Write(pcm)
	_, err := DecodeWAV(buf.Bytes(), "audio/wav", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected rejection of IEEE float WAV (format 3)")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DecodeError, got %T", err)
	}
	if !bytes.Contains([]byte(de.Reason), []byte("not PCM")) {
		t.Errorf("error reason: %q", de.Reason)
	}
}

// TestDecodeAudio_RejectsWavWithALawFormat: WAV with A-law encoding (format 6)
// must be rejected.
func TestDecodeAudio_RejectsWavWithALawFormat(t *testing.T) {
	pcm := make([]byte, 16000)
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(36+len(pcm))))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(6))  // A-law
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(16000))
	binary.Write(buf, binary.LittleEndian, uint32(16000*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(8))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(pcm)))
	buf.Write(pcm)
	_, err := DecodeWAV(buf.Bytes(), "audio/wav", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected rejection of A-law WAV (format 6)")
	}
}

// TestDecodeAudio_RejectsWavWithMuLawFormat: WAV with mu-law encoding (format 7)
// must be rejected.
func TestDecodeAudio_RejectsWavWithMuLawFormat(t *testing.T) {
	pcm := make([]byte, 16000)
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(36+len(pcm))))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(7))  // mu-law
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(16000))
	binary.Write(buf, binary.LittleEndian, uint32(16000*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(8))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(pcm)))
	buf.Write(pcm)
	_, err := DecodeWAV(buf.Bytes(), "audio/wav", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("expected rejection of mu-law WAV (format 7)")
	}
}

// TestDecodeAudio_EmptyCompressedBytesRejected: nil compressed bytes must be
// rejected before any decoder is invoked.
func TestDecodeAudio_EmptyCompressedBytesRejected(t *testing.T) {
	ffmpegOrSkip(t)
	_, err := DecodeCompressed(nil, "audio/ogg", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("nil compressed bytes should be rejected")
	}
}

// TestDecodeAudio_EmptyWebmBytesRejected: nil WebM bytes must be rejected.
func TestDecodeAudio_EmptyWebmBytesRejected(t *testing.T) {
	_, err := DecodeAudio(nil, "audio/webm", DefaultDecodeLimits())
	if err == nil {
		t.Fatal("nil webm bytes should be rejected")
	}
}

// TestDecodeAudio_ContentTypeCaseInsensitive: uppercase MIME
// "AUDIO/WEBM;CODECS=OPUS" is accepted and routed correctly.
func TestDecodeAudio_ContentTypeCaseInsensitive(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t)
	res, err := DecodeAudio(webm, "AUDIO/WEBM;CODECS=OPUS", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("uppercase MIME should be accepted: %v", err)
	}
	if len(res.Samples) == 0 {
		t.Fatal("samples must be non-empty")
	}
	if res.SampleRate != TargetSampleRate {
		t.Errorf("sample rate: got %d want %d", res.SampleRate, TargetSampleRate)
	}
}
