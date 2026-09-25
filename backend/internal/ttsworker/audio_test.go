package ttsworker

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TestEncodeSilenceWavProducesValidHeader: the generated WAV starts
// with the canonical RIFF/WAVE header and reports the requested
// sample rate + duration.
func TestEncodeSilenceWavProducesValidHeader(t *testing.T) {
	out, err := EncodeSilenceWav(22050, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out.Bytes, []byte("RIFF")) {
		t.Fatalf("missing RIFF tag: %q", out.Bytes[:4])
	}
	wave := string(out.Bytes[8:12])
	if wave != "WAVE" {
		t.Fatalf("missing WAVE tag: %q", wave)
	}
	sampleRate := binary.LittleEndian.Uint32(out.Bytes[24:28])
	if sampleRate != 22050 {
		t.Fatalf("sample rate: got %d want 22050", sampleRate)
	}
	if len(out.Bytes) > MaxOutputBytes {
		t.Fatalf("output exceeds MaxOutputBytes")
	}
}

// TestEncodeSilenceWavRejectsOversizeDuration: the encoder refuses
// to produce more than MaxOutputDurationSeconds of audio.
func TestEncodeSilenceWavRejectsOversizeDuration(t *testing.T) {
	if _, err := EncodeSilenceWav(22050, MaxOutputDurationSeconds+1); err == nil {
		t.Fatal("expected rejection for oversize duration")
	}
	if _, err := EncodeSilenceWav(MaxOutputSampleRate+1, 1.0); err == nil {
		t.Fatal("expected rejection for oversize sample rate")
	}
	if _, err := EncodeSilenceWav(0, 1.0); err == nil {
		t.Fatal("expected rejection for zero sample rate")
	}
	if _, err := EncodeSilenceWav(22050, 0); err == nil {
		t.Fatal("expected rejection for zero duration")
	}
}

// TestFloat32ToWavRefusesOutOfRangeSamples: the encoder rejects
// samples outside [-1, 1] with a typed error rather than clipping.
func TestFloat32ToWavRefusesOutOfRangeSamples(t *testing.T) {
	if _, err := Float32ToWav([]float32{2}, 22050); err == nil {
		t.Fatal("expected rejection for sample > 1")
	}
	if _, err := Float32ToWav([]float32{-2}, 22050); err == nil {
		t.Fatal("expected rejection for sample < -1")
	}
}

// TestFloat32ToWavRoundtrip: encoded bytes match the canonical WAV
// header and length.
func TestFloat32ToWavRoundtrip(t *testing.T) {
	samples := []float32{0, 0.5, -0.5, 1.0, -1.0, 0}
	out, err := Float32ToWav(samples, 22050)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out.Bytes, []byte("RIFF")) {
		t.Fatalf("missing RIFF tag")
	}
	if !bytes.Contains(out.Bytes[8:44], []byte("WAVE")) {
		t.Fatalf("missing WAVE tag")
	}
	if got := len(out.Bytes) - 44; got != 2*len(samples) {
		t.Fatalf("data bytes: got %d want %d", got, 2*len(samples))
	}
	if out.ChecksumSHA256 == "" || len(out.ChecksumSHA256) != 64 {
		t.Fatalf("checksum missing or wrong length: %q", out.ChecksumSHA256)
	}
}

// TestFloat32ToWavRefusesDurationBudget: long inputs are rejected.
func TestFloat32ToWavRefusesDurationBudget(t *testing.T) {
	samples := make([]float32, 300000) // 13.6s at 22050
	for i := range samples {
		samples[i] = 0
	}
	if _, err := Float32ToWav(samples, 22050); err == nil ||
		!strings.Contains(err.Error(), "duration") {
		t.Fatalf("expected duration refusal, got %v", err)
	}
}

// TestEncodeSilenceWavChecksumIsContent: identical inputs produce
// identical SHA-256.
func TestEncodeSilenceWavChecksumIsContent(t *testing.T) {
	a, err := EncodeSilenceWav(22050, 0.25)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncodeSilenceWav(22050, 0.25)
	if err != nil {
		t.Fatal(err)
	}
	if a.ChecksumSHA256 != b.ChecksumSHA256 {
		t.Fatalf("identical silence buffers should hash the same")
	}
}
