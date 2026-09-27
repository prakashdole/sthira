package ttsworker

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TestFloat32ToWav_MaxRateAccepted: Float32ToWav accepts MaxOutputSampleRate
// (48 kHz) and produces a structurally valid PCM 16-bit mono WAV with correct
// header fields and byte layout.
func TestFloat32ToWav_MaxRateAccepted(t *testing.T) {
	samples := make([]float32, 48000) // 1 s
	for i := range samples {
		samples[i] = 0.1
	}
	out, err := Float32ToWav(samples, 48000)
	if err != nil {
		t.Fatalf("48 kHz should be accepted: %v", err)
	}
	// Structural assertions on the encoded bytes.
	if !bytes.HasPrefix(out.Bytes, []byte("RIFF")) {
		t.Fatal("missing RIFF tag")
	}
	if !bytes.Contains(out.Bytes[8:44], []byte("WAVE")) {
		t.Fatal("missing WAVE tag")
	}
	// fmt chunk: PCM (format 1), mono, 16-bit.
	fmtFormat := binary.LittleEndian.Uint16(out.Bytes[20:22])
	if fmtFormat != 1 {
		t.Errorf("format: got %d want 1 (PCM)", fmtFormat)
	}
	channels := binary.LittleEndian.Uint16(out.Bytes[22:24])
	if channels != 1 {
		t.Errorf("channels: got %d want 1", channels)
	}
	sampleRate := binary.LittleEndian.Uint32(out.Bytes[24:28])
	if sampleRate != 48000 {
		t.Errorf("sample rate: got %d want 48000", sampleRate)
	}
	bitsPerSample := binary.LittleEndian.Uint16(out.Bytes[34:36])
	if bitsPerSample != 16 {
		t.Errorf("bits per sample: got %d want 16", bitsPerSample)
	}
	// Data chunk: size must equal 2 * sample count.
	dataSize := binary.LittleEndian.Uint32(out.Bytes[40:44])
	if dataSize != uint32(2*len(samples)) {
		t.Errorf("data chunk size: got %d want %d", dataSize, 2*len(samples))
	}
	// WavOutput fields.
	if out.SampleRate != 48000 {
		t.Errorf("WavOutput.SampleRate: got %d want 48000", out.SampleRate)
	}
	if out.Channels != 1 {
		t.Errorf("WavOutput.Channels: got %d want 1", out.Channels)
	}
	if out.BitDepth != 16 {
		t.Errorf("WavOutput.BitDepth: got %d want 16", out.BitDepth)
	}
	if out.DurationSec != 1.0 {
		t.Errorf("DurationSec: got %v want 1.0", out.DurationSec)
	}
	if len(out.Bytes) != int(44+2*len(samples)) {
		t.Errorf("total size: got %d want %d", len(out.Bytes), 44+2*len(samples))
	}
}

// TestFloat32ToWav_MaxDurationAtMaxRateAccepted: encoding exactly
// MaxOutputSampleRate for exactly MaxOutputDurationSeconds produces a valid
// WAV that fits within MaxOutputBytes. At this boundary:
//   - duration == MaxOutputDurationSeconds (checked first, passes)
//   - byte count == 44 + maxSamples*2 == 1 152 044 bytes, which is below
//     MaxOutputBytes (1 228 800).
//
// The byte ceiling cannot be reached independently from a rate-and-duration-
// compliant input: any attempt to exceed MaxOutputBytes would require either
// a rate > MaxOutputSampleRate (rejected first) or a duration >
// MaxOutputDurationSeconds (rejected before the byte check). There is no
// separate byte-ceiling execution path to test.
func TestFloat32ToWav_MaxDurationAtMaxRateAccepted(t *testing.T) {
	maxSamples := int(float64(MaxOutputSampleRate) * MaxOutputDurationSeconds)
	samples := make([]float32, maxSamples)
	for i := range samples {
		samples[i] = 0
	}
	out, err := Float32ToWav(samples, MaxOutputSampleRate)
	if err != nil {
		t.Fatalf("max rate × max duration should be accepted: %v", err)
	}
	if out.DurationSec != MaxOutputDurationSeconds {
		t.Errorf("duration: got %v want %v", out.DurationSec, MaxOutputDurationSeconds)
	}
	if len(out.Bytes) > MaxOutputBytes {
		t.Errorf("output %d bytes exceeds MaxOutputBytes %d", len(out.Bytes), MaxOutputBytes)
	}
	// byte ceiling relationship: 1 152 044 < 1 228 800; ceiling not independently reachable.
	expectedBytes := 44 + maxSamples*2
	if len(out.Bytes) != expectedBytes {
		t.Errorf("byte count: got %d want %d", len(out.Bytes), expectedBytes)
	}
}

// TestFloat32ToWav_JustBelowMaxDurationAccepted: one sample below the
// maximum duration should succeed.
func TestFloat32ToWav_JustBelowMaxDurationAccepted(t *testing.T) {
	maxSamples := int(float64(MaxOutputSampleRate) * MaxOutputDurationSeconds)
	samples := make([]float32, maxSamples-1)
	for i := range samples {
		samples[i] = 0
	}
	_, err := Float32ToWav(samples, MaxOutputSampleRate)
	if err != nil {
		t.Fatalf("just below max duration should succeed: %v", err)
	}
}

// TestFloat32ToWav_JustAboveMaxDurationRejected: one sample above the
// maximum duration must be rejected with a message referencing "duration".
func TestFloat32ToWav_JustAboveMaxDurationRejected(t *testing.T) {
	maxSamples := int(float64(MaxOutputSampleRate) * MaxOutputDurationSeconds)
	samples := make([]float32, maxSamples+1)
	for i := range samples {
		samples[i] = 0
	}
	_, err := Float32ToWav(samples, MaxOutputSampleRate)
	if err == nil {
		t.Fatal("expected rejection above max duration")
	}
	if !strings.Contains(err.Error(), "duration") {
		t.Errorf("error should mention duration: %v", err)
	}
}

// TestFloat32ToWav_ZeroSamplesRejected: empty sample slice is rejected.
func TestFloat32ToWav_ZeroSamplesRejected(t *testing.T) {
	_, err := Float32ToWav(nil, 22050)
	if err == nil {
		t.Fatal("nil samples should be rejected")
	}
}

// TestFloat32ToWav_AboveMaxSampleRateRejected: sample rate above
// MaxOutputSampleRate is rejected.
func TestFloat32ToWav_AboveMaxSampleRateRejected(t *testing.T) {
	samples := []float32{0}
	_, err := Float32ToWav(samples, MaxOutputSampleRate+1)
	if err == nil {
		t.Fatal("sample rate above MaxOutputSampleRate should be rejected")
	}
}

// TestEncodeSilenceWav_MaxDurationAccepted: silence at 22.05 kHz for exactly
// MaxOutputDurationSeconds produces a valid WAV.
func TestEncodeSilenceWav_MaxDurationAccepted(t *testing.T) {
	out, err := EncodeSilenceWav(22050, MaxOutputDurationSeconds)
	if err != nil {
		t.Fatalf("max duration should succeed: %v", err)
	}
	if out.DurationSec != MaxOutputDurationSeconds {
		t.Errorf("duration: got %v want %v", out.DurationSec, MaxOutputDurationSeconds)
	}
}

// TestEncodeSilenceWav_ZeroDurationRejected: zero duration is rejected.
func TestEncodeSilenceWav_ZeroDurationRejected(t *testing.T) {
	_, err := EncodeSilenceWav(22050, 0)
	if err == nil {
		t.Fatal("zero duration should be rejected")
	}
}

// TestEncodeSilenceWav_NegativeDurationRejected: negative duration is rejected.
func TestEncodeSilenceWav_NegativeDurationRejected(t *testing.T) {
	_, err := EncodeSilenceWav(22050, -1.0)
	if err == nil {
		t.Fatal("negative duration should be rejected")
	}
}

// TestEncodeSilenceWav_ZeroSampleRateRejected: zero sample rate is rejected.
func TestEncodeSilenceWav_ZeroSampleRateRejected(t *testing.T) {
	_, err := EncodeSilenceWav(0, 1.0)
	if err == nil {
		t.Fatal("zero sample rate should be rejected")
	}
}

// TestEncodeSilenceWav_MultipleDurations: various durations produce correct
// DurationSec values.
func TestEncodeSilenceWav_MultipleDurations(t *testing.T) {
	for _, dur := range []float64{0.5, 1.0, 2.0, 5.0} {
		out, err := EncodeSilenceWav(22050, dur)
		if err != nil {
			t.Errorf("duration %v: %v", dur, err)
			continue
		}
		if out != nil && out.DurationSec != dur {
			t.Errorf("duration %v: got %v", dur, out.DurationSec)
		}
	}
}

// Note: Byte-ceiling independence. The MaxOutputBytes ceiling (1 228 800 bytes)
// equals 44 + maxSamples*2 where maxSamples = MaxOutputSampleRate ×
// MaxOutputDurationSeconds. Any input that satisfies the rate and duration
// limits automatically satisfies the byte limit. An input that exceeds the byte
// limit must exceed either the rate limit (rejected first) or the duration
// limit (rejected before byte check). The byte ceiling therefore cannot be
// triggered by a rate-and-duration-compliant input, and no separate
// byte-ceiling execution exists to test independently.
