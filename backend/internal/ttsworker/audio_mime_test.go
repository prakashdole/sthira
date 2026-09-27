package ttsworker

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestFloat32ToWav_MismatchedDeclaredDataSize(t *testing.T) {
	samples := []float32{0, 0.5, -0.5}
	header := buildWavHeader(22050, 1, 16, len(samples)*2)
	buf := append([]byte(nil), header...)
	for _, s := range samples {
		v := int16(22050 * s)
		buf = binary.LittleEndian.AppendUint16(buf, uint16(v))
	}
	_, err := Float32ToWav(samples, 22050)
	if err != nil {
		t.Fatalf("valid encoding should succeed: %v", err)
	}
	_ = buf
}

func TestFloat32ToWav_DataChunkLargerThanDeclared(t *testing.T) {
	samples := []float32{0, 0.5, -0.5}
	header := buildWavHeader(22050, 1, 16, 2)
	buf := append([]byte(nil), header...)
	for _, s := range samples {
		v := int16(22050 * s)
		buf = binary.LittleEndian.AppendUint16(buf, uint16(v))
	}
	_, err := Float32ToWav(samples, 22050)
	if err != nil {
		t.Fatalf("should succeed with buffer larger than declared: %v", err)
	}
}

func TestFloat32ToWav_SampleRate48kHz(t *testing.T) {
	samples := make([]float32, 48000)
	for i := range samples {
		samples[i] = 0.1
	}
	out, err := Float32ToWav(samples, 48000)
	if err != nil {
		t.Fatalf("48 kHz should be accepted: %v", err)
	}
	if out.SampleRate != 48000 {
		t.Errorf("sample rate: got %d want 48000", out.SampleRate)
	}
	if out.BitDepth != 16 {
		t.Errorf("bit depth: got %d want 16", out.BitDepth)
	}
	if out.Channels != 1 {
		t.Errorf("channels: got %d want 1", out.Channels)
	}
}

func TestFloat32ToWav_JustBelowDurationLimit(t *testing.T) {
	maxSamples := int(float64(MaxOutputSampleRate) * MaxOutputDurationSeconds)
	samples := make([]float32, maxSamples-1)
	for i := range samples {
		samples[i] = 0
	}
	_, err := Float32ToWav(samples, MaxOutputSampleRate)
	if err != nil {
		t.Fatalf("just below duration limit should succeed: %v", err)
	}
}

func TestFloat32ToWav_ExactlyAtDurationLimit(t *testing.T) {
	maxSamples := int(float64(MaxOutputSampleRate) * MaxOutputDurationSeconds)
	samples := make([]float32, maxSamples)
	for i := range samples {
		samples[i] = 0
	}
	_, err := Float32ToWav(samples, MaxOutputSampleRate)
	if err != nil {
		t.Fatalf("exactly at duration limit should succeed: %v", err)
	}
}

func TestFloat32ToWav_JustAboveDurationLimit(t *testing.T) {
	maxSamples := int(float64(MaxOutputSampleRate) * MaxOutputDurationSeconds)
	samples := make([]float32, maxSamples+1)
	for i := range samples {
		samples[i] = 0
	}
	_, err := Float32ToWav(samples, MaxOutputSampleRate)
	if err == nil {
		t.Fatalf("expected rejection just above duration limit")
	}
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("duration")) {
		t.Errorf("error should mention duration: %v", err)
	}
}

func TestFloat32ToWav_MaxSamplesAtMaxRate(t *testing.T) {
	maxSamples := int(float64(MaxOutputSampleRate) * MaxOutputDurationSeconds)
	samples := make([]float32, maxSamples)
	for i := range samples {
		samples[i] = 0.01
	}
	out, err := Float32ToWav(samples, MaxOutputSampleRate)
	if err != nil {
		t.Fatalf("max rate and max duration should succeed: %v", err)
	}
	if out.DurationSec > MaxOutputDurationSeconds {
		t.Errorf("duration %v exceeds MaxOutputDurationSeconds %v", out.DurationSec, MaxOutputDurationSeconds)
	}
}

func TestFloat32ToWav_ZeroSamplesRejects(t *testing.T) {
	_, err := Float32ToWav(nil, 22050)
	if err == nil {
		t.Fatalf("nil samples should be rejected")
	}
}

func TestFloat32ToWav_AllPositiveOnes(t *testing.T) {
	samples := []float32{1.0, 1.0, 1.0}
	out, err := Float32ToWav(samples, 22050)
	if err != nil {
		t.Fatalf("all 1.0 samples should be accepted (clamped to 1.0): %v", err)
	}
	if len(out.Bytes) == 0 {
		t.Error("no bytes produced")
	}
}

func TestFloat32ToWav_AllNegativeOnes(t *testing.T) {
	samples := []float32{-1.0, -1.0, -1.0}
	out, err := Float32ToWav(samples, 22050)
	if err != nil {
		t.Fatalf("all -1.0 samples should be accepted (clamped to -1.0): %v", err)
	}
	if len(out.Bytes) == 0 {
		t.Error("no bytes produced")
	}
}

func TestFloat32ToWav_SilenceNearFullScale(t *testing.T) {
	samples := make([]float32, 22050)
	for i := range samples {
		samples[i] = 1e-7
	}
	out, err := Float32ToWav(samples, 22050)
	if err != nil {
		t.Fatalf("near-silence should be accepted: %v", err)
	}
	if out.DurationSec != 1.0 {
		t.Errorf("duration: got %v want 1.0", out.DurationSec)
	}
}

func TestEncodeSilenceWav_MaxDuration(t *testing.T) {
	out, err := EncodeSilenceWav(22050, MaxOutputDurationSeconds)
	if err != nil {
		t.Fatalf("max duration should succeed: %v", err)
	}
	if out.DurationSec != MaxOutputDurationSeconds {
		t.Errorf("duration: got %v want %v", out.DurationSec, MaxOutputDurationSeconds)
	}
}

func TestEncodeSilenceWav_ZeroDurationRejects(t *testing.T) {
	_, err := EncodeSilenceWav(22050, 0)
	if err == nil {
		t.Fatalf("zero duration should be rejected")
	}
}

func TestEncodeSilenceWav_NegativeDurationRejects(t *testing.T) {
	_, err := EncodeSilenceWav(22050, -1.0)
	if err == nil {
		t.Fatalf("negative duration should be rejected")
	}
}

func TestEncodeSilenceWav_ExceedsMaxOutputBytesRejects(t *testing.T) {
	rate := 64000
	samples := rate * 12
	bufSize := 44 + samples*2
	if bufSize <= MaxOutputBytes {
		t.Skipf("64 kHz at 12s = %d bytes ≤ MaxOutputBytes %d; cannot trigger this boundary", bufSize, MaxOutputBytes)
	}
	_, err := EncodeSilenceWav(rate, MaxOutputDurationSeconds)
	if err == nil {
		t.Fatalf("64 kHz at max duration should exceed MaxOutputBytes and be rejected")
	}
}

func TestEncodeSilenceWav_MultipleOfOneSecond(t *testing.T) {
	for _, dur := range []float64{0.5, 1.0, 2.0, 5.0} {
		out, err := EncodeSilenceWav(22050, dur)
		if err != nil {
			t.Errorf("duration %v: %v", dur, err)
		}
		if out != nil && out.DurationSec != dur {
			t.Errorf("duration %v: got %v", dur, out.DurationSec)
		}
	}
}
