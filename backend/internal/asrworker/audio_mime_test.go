package asrworker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os/exec"
	"testing"
)

func genWebmOpusLocal(t *testing.T, seconds float64) []byte {
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

func genOggOpusLocal(t *testing.T, seconds float64) []byte {
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

func TestDecodeAudio_AcceptsWebmWithOpusCodecParam(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t, 1.0)
	_, err := DecodeAudio(webm, "audio/webm;codecs=opus", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("DecodeAudio with audio/webm;codecs=opus: %v", err)
	}
}

func TestDecodeAudio_AcceptsOggWithOpusCodecParam(t *testing.T) {
	ffmpegOrSkip(t)
	ogg := genOggOpusLocal(t, 1.0)
	_, err := DecodeAudio(ogg, "audio/ogg;codecs=opus", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("DecodeAudio with audio/ogg;codecs=opus: %v", err)
	}
}

func TestDecodeAudio_WebmWithoutCodecParam_RoutesToCompressed(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t, 1.0)
	_, err := DecodeAudio(webm, "audio/webm", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("DecodeAudio with audio/webm (no codecs param): %v", err)
	}
}

func TestDecodeAudio_MismatchedMimeBytesCrossCodecDecodes(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t, 1.0)
	_, err := DecodeAudio(webm, "audio/ogg;codecs=opus", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("ffmpeg auto-detects format from bytes; MIME mismatch does not error: %v", err)
	}
}

func TestDecodeAudio_RejectsWavWithNonPCMFormat(t *testing.T) {
	pcm := make([]byte, 16000)
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(36+len(pcm))))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(3))
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
		t.Fatalf("expected rejection of IEEE float WAV (format 3)")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DecodeError, got %T", err)
	}
	if !bytes.Contains([]byte(de.Reason), []byte("not PCM")) {
		t.Errorf("error reason: %q", de.Reason)
	}
}

func TestDecodeAudio_RejectsWavWithALawFormat(t *testing.T) {
	pcm := make([]byte, 16000)
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(36+len(pcm))))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(6))
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
		t.Fatalf("expected rejection of A-law WAV (format 6)")
	}
}

func TestDecodeAudio_RejectsWavWithMuLawFormat(t *testing.T) {
	pcm := make([]byte, 16000)
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	buf.Write(binary.LittleEndian.AppendUint32(nil, uint32(36+len(pcm))))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(7))
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
		t.Fatalf("expected rejection of mu-law WAV (format 7)")
	}
}

func TestDecodeAudio_EmptyCompressedBytes_Rejected(t *testing.T) {
	ffmpegOrSkip(t)
	_, err := DecodeCompressed(nil, "audio/ogg", DefaultDecodeLimits())
	if err == nil {
		t.Fatalf("expected rejection of nil compressed bytes")
	}
}

func TestDecodeAudio_EmptyWebmBytes_Rejected(t *testing.T) {
	_, err := DecodeAudio(nil, "audio/webm", DefaultDecodeLimits())
	if err == nil {
		t.Fatalf("expected rejection of nil webm bytes")
	}
}

func TestDecodeAudio_ContentTypeNormalization(t *testing.T) {
	ffmpegOrSkip(t)
	webm := genWebmOpusLocal(t, 0.5)
	res, err := DecodeAudio(webm, "AUDIO/WEBM;CODECS=OPUS", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("case-insensitive content-type: %v", err)
	}
	if len(res.Samples) == 0 {
		t.Error("no samples decoded")
	}
}
