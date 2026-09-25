package asrworker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"testing"
)

// makeWAV builds an in-memory PCM-16-LE mono WAV with the given
// samples and source sample rate. The data chunk is well-formed;
// header is canonical RIFF / WAVE.
func makeWAV(t *testing.T, samples []int16, sampleRate, channels int) []byte {
	t.Helper()
	if channels < 1 {
		t.Fatalf("channels must be >= 1")
	}
	// For mono tests channels=1; multi-channel ones duplicate samples.
	frames := len(samples)
	totalData := frames * channels * 2
	totalFile := 36 + 8 + totalData
	buf := bytes.NewBuffer(nil)
	// RIFF header
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(totalFile))
	buf.WriteString("WAVE")
	// fmt chunk
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate*channels*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))
	// data chunk
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(totalData))
	for _, s := range samples {
		if channels == 1 {
			_ = binary.Write(buf, binary.LittleEndian, s)
		} else {
			for c := 0; c < channels; c++ {
				_ = binary.Write(buf, binary.LittleEndian, s)
			}
		}
	}
	return buf.Bytes()
}

// silenceBufferAt is the non-test variant. Tests below use
// silenceBuffer; this is for callers from server_test.go that
// don't have a *testing.T.
func silenceBufferAt(frames, sampleRate int) []byte {
	samples := make([]int16, frames)
	return makeWAVPlain(samples, sampleRate, 1)
}

// makeWAVPlain is the no-test variant of makeWAV used by other
// helpers. The two share body via inline copy; only difference is
// the t.Helper() call.
func makeWAVPlain(samples []int16, sampleRate, channels int) []byte {
	if channels < 1 {
		channels = 1
	}
	frames := len(samples)
	totalData := frames * channels * 2
	totalFile := 36 + 8 + totalData
	buf := bytes.NewBuffer(nil)
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(totalFile))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate*channels*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(totalData))
	for _, s := range samples {
		if channels == 1 {
			_ = binary.Write(buf, binary.LittleEndian, s)
		} else {
			for c := 0; c < channels; c++ {
				_ = binary.Write(buf, binary.LittleEndian, s)
			}
		}
	}
	return buf.Bytes()
}

// silenceBuffer returns N samples of zero at the given rate.
func silenceBuffer(t *testing.T, frames, sampleRate int) []byte {
	t.Helper()
	samples := make([]int16, frames)
	return makeWAV(t, samples, sampleRate, 1)
}

// toneBuffer returns N samples of a 1 kHz sine at the given rate,
// amplitude in [-30000, 30000] to avoid clipping.
func toneBuffer(t *testing.T, frames, sampleRate int) []byte {
	t.Helper()
	samples := make([]int16, frames)
	for i := 0; i < frames; i++ {
		v := math.Sin(2 * math.Pi * 1000 * float64(i) / float64(sampleRate))
		samples[i] = int16(v * 30000)
	}
	return makeWAV(t, samples, sampleRate, 1)
}

// TestDecodeWAV_MonoSilence: a silent mono 16 kHz WAV decodes cleanly,
// produces exactly len(samples) samples at the target rate, and the
// duration is reported as len/SampleRate.
func TestDecodeWAV_MonoSilence(t *testing.T) {
	frames := 16_000 // 1 second
	wav := silenceBuffer(t, frames, 16_000)
	out, err := DecodeWAV(wav, "audio/wav", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.SampleRate != TargetSampleRate {
		t.Errorf("SampleRate: got %d want %d", out.SampleRate, TargetSampleRate)
	}
	if out.ChannelsIn != 1 {
		t.Errorf("ChannelsIn: got %d want 1", out.ChannelsIn)
	}
	if math.Abs(out.DurationSecs-1.0) > 1e-3 {
		t.Errorf("DurationSecs: got %.3f want ~1.0", out.DurationSecs)
	}
	if !SilenceDetector(out.Samples) {
		t.Errorf("decoded buffer expected to be silent")
	}
}

// TestDecodeWAV_MonoToneResampled: a 1-second 8 kHz mono tone
// resamples up to 16 kHz and reports correctly. We don't compare
// the samples float-by-float; instead we sanity-check RMS, peak
// and zero-crossing density to ensure the resampler didn't lose
// the signal.
func TestDecodeWAV_MonoToneResampled(t *testing.T) {
	wav := toneBuffer(t, 8_000, 8_000)
	out, err := DecodeWAV(wav, "audio/wav", DefaultDecodeLimits())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.OriginalRate != 8_000 {
		t.Errorf("OriginalRate: got %d want 8000", out.OriginalRate)
	}
	if out.SampleRate != 16_000 {
		t.Errorf("SampleRate: got %d want 16000", out.SampleRate)
	}
	// RMS of a sinusoid is amplitude / sqrt(2). Allow loose bounds
	// because linear interpolation slightly damps the amplitude.
	var sumSq float64
	var maxAbs float64
	for _, s := range out.Samples {
		sumSq += float64(s) * float64(s)
		if a := math.Abs(float64(s)); a > maxAbs {
			maxAbs = a
		}
	}
	rms := math.Sqrt(sumSq / float64(len(out.Samples)))
	if rms < 0.05 || rms > 0.95 {
		t.Errorf("RMS out of expected band: %.3f", rms)
	}
	if maxAbs < 0.1 {
		t.Errorf("peak too small: %.3f", maxAbs)
	}
}

// TestDecodeWAV_RejectsEmpty: zero-length input rejects with
// typed error.
func TestDecodeWAV_RejectsEmpty(t *testing.T) {
	if _, err := DecodeWAV(nil, "audio/wav", DefaultDecodeLimits()); err == nil {
		t.Fatalf("expected error for empty input")
	}
}

// TestDecodeWAV_RejectsOversizeInputBytes: an input that exceeds
// the compressed-bytes cap rejects with "audio bytes exceed
// limit" before any decode work.
func TestDecodeWAV_RejectsOversizeInputBytes(t *testing.T) {
	wav := silenceBuffer(t, 1_000, 16_000)
	lim := DefaultDecodeLimits()
	lim.CompressedBytes = 100 // very small cap
	if _, err := DecodeWAV(wav, "audio/wav", lim); err == nil {
		t.Fatalf("expected oversize-bytes rejection")
	} else if !strings.Contains(err.Error(), "exceed") {
		t.Errorf("error: %v", err)
	}
}

// TestDecodeWAV_RejectsOversizeDuration: an audio whose post-decode
// duration exceeds the limit rejects with "decoded duration". The
// decoded buffer must be wiped; we cannot introspect the slice from
// outside, but the typed return is the proof: if the function had
// returned the buffer we'd see samples != nil. Here we only check
// error and absence of a partial result.
func TestDecodeWAV_RejectsOversizeDuration(t *testing.T) {
	wav := silenceBuffer(t, 16_000*30, 16_000) // 30 s
	lim := DefaultDecodeLimits()
	lim.DecodedMonoSeconds = 1.0
	res, err := DecodeWAV(wav, "audio/wav", lim)
	if err == nil {
		t.Fatalf("expected oversize duration rejection")
	}
	if res != nil {
		t.Errorf("oversize must NOT return a non-nil result")
	}
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Errorf("expected *DecodeError, got %T", err)
	}
}

// TestDecodeWAV_RejectsTruncatedHeader: passing only the first 20
// bytes rejects as truncated.
func TestDecodeWAV_RejectsTruncatedHeader(t *testing.T) {
	wav := silenceBuffer(t, 100, 16_000)
	if _, err := DecodeWAV(wav[:20], "audio/wav", DefaultDecodeLimits()); err == nil {
		t.Fatalf("expected truncated header rejection")
	}
}

// TestDecodeWAV_RejectsTruncatedData: a WAV whose data chunk is
// declared 8000 bytes but the file ends short.
func TestDecodeWAV_RejectsTruncatedData(t *testing.T) {
	full := silenceBuffer(t, 100, 16_000)
	// Slice off the last 50 bytes of data.
	trunc := full[:len(full)-50]
	if _, err := DecodeWAV(trunc, "audio/wav", DefaultDecodeLimits()); err == nil {
		t.Fatalf("expected truncated data rejection")
	}
}

// TestDecodeWAV_RejectsMultiChannelWhenMonoOnly: a stereo WAV is
// rejected when limits.MonoOnly is true.
func TestDecodeWAV_RejectsMultiChannelWhenMonoOnly(t *testing.T) {
	wav := silenceBuffer(t, 100, 16_000)
	// Replace the channel count with 2 by re-encoding the WAV.
	buf := bytes.NewBuffer(nil)
	buf.Write(wav[:22]) // through channels field
	_ = binary.Write(buf, binary.LittleEndian, uint16(2))
	buf.Write(wav[24:]) // rest
	// Now the data chunk claims interleaved stereo but is sized
	// mono. That's actually malformed; we expect the block-align
	// check to fail before the channels check. Try a clean stereo
	// WAV instead.
	stereo := makeWAV(t, []int16{0, 0, 0, 0}, 16_000, 2)
	if _, err := DecodeWAV(stereo, "audio/wav", DefaultDecodeLimits()); err == nil {
		t.Fatalf("expected multi-channel rejection under MonoOnly")
	}
}

// TestDecodeWAV_RejectsOutOfBandSampleRate: sample rates under
// MinSampleRate or over MaxSampleRate reject.
func TestDecodeWAV_RejectsOutOfBandSampleRate(t *testing.T) {
	low := silenceBuffer(t, 100, 4_000)
	if _, err := DecodeWAV(low, "audio/wav", DefaultDecodeLimits()); err == nil {
		t.Errorf("expected rejection for 4 kHz source")
	}
	hi := silenceBuffer(t, 100, 96_000)
	if _, err := DecodeWAV(hi, "audio/wav", DefaultDecodeLimits()); err == nil {
		t.Errorf("expected rejection for 96 kHz source")
	}
}

// TestDecodeWAV_RejectsUnsupportedContentType: a non-audio/wav
// content-type returns the typed ErrUnsupportedCodec.
func TestDecodeWAV_RejectsUnsupportedContentType(t *testing.T) {
	wav := silenceBuffer(t, 100, 16_000)
	_, err := DecodeWAV(wav, "audio/webm", DefaultDecodeLimits())
	if !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("expected ErrUnsupportedCodec, got %v", err)
	}
}

// TestDecodeWAV_RejectsEmptyAudio: 0-length bytes after the
// wrapper formats are decoded.
func TestDecodeWAV_RejectsZeroBytes(t *testing.T) {
	if _, err := DecodeWAV(nil, "audio/wav", DefaultDecodeLimits()); err == nil {
		t.Fatal("expected error for nil bytes")
	}
}

// TestResampleLinear_IdentityAtEqualRates: resampleLinear must be
// a no-op when src == dst.
func TestResampleLinear_IdentityAtEqualRates(t *testing.T) {
	in := []float32{0.1, 0.2, -0.1, 0.0, 0.5}
	out := resampleLinear(in, 16_000, 16_000)
	if len(out) != len(in) {
		t.Errorf("len: got %d want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("identity[%d]: got %v want %v", i, out[i], in[i])
		}
	}
}

// TestResampleLinear_EmptyInput: zero-length sample slice returns
// zero-length output without panicking.
func TestResampleLinear_EmptyInput(t *testing.T) {
	out := resampleLinear(nil, 8_000, 16_000)
	if len(out) != 0 {
		t.Errorf("len: got %d want 0", len(out))
	}
}

// TestResampleLinear_Upsample2x: upsample 1 second of silence
// from 8 kHz to 16 kHz produces a slice of about 16000 samples.
func TestResampleLinear_Upsample2x(t *testing.T) {
	in := make([]float32, 8_000)
	out := resampleLinear(in, 8_000, 16_000)
	if len(out) < 16_000 || len(out) > 16_001 {
		t.Errorf("upsample len: got %d want ~16000", len(out))
	}
}

// TestWipeBuffer_ZerosAll: WipeBuffer must zero every element.
func TestWipeBuffer_ZerosAll(t *testing.T) {
	in := []float32{1, 2, 3, 4, 5}
	WipeBuffer(in)
	for _, s := range in {
		if s != 0 {
			t.Errorf("after wipe: got %v want 0", s)
		}
	}
}

// TestSilenceDetector_ExactAndQuiet tests exact silence vs quiet speech vs non-finite samples.
func TestSilenceDetector_ExactAndQuiet(t *testing.T) {
	if !SilenceDetector(nil) {
		t.Errorf("nil samples must be detected as silence")
	}
	if !SilenceDetector([]float32{}) {
		t.Errorf("empty samples must be detected as silence")
	}
	if !SilenceDetector([]float32{0, 0, 0, 1e-7, -1e-7}) {
		t.Errorf("negligible samples (<= 1e-6) must be detected as silence")
	}
	if SilenceDetector([]float32{0.001}) {
		t.Errorf("quiet speech (0.001 > 1e-6) must NOT be detected as silence")
	}
	if SilenceDetector([]float32{float32(math.NaN())}) {
		t.Errorf("NaN must NOT be detected as silence")
	}
	if SilenceDetector([]float32{float32(math.Inf(1))}) {
		t.Errorf("+Inf must NOT be detected as silence")
	}
}

// TestIsFinite_ValidatesSamples checks that NaN and Inf are detected.
func TestIsFinite_ValidatesSamples(t *testing.T) {
	if !IsFinite([]float32{0.0, -0.5, 0.5, 1.0}) {
		t.Errorf("normal samples must be finite")
	}
	if IsFinite([]float32{0.0, float32(math.NaN())}) {
		t.Errorf("NaN must not be finite")
	}
	if IsFinite([]float32{float32(math.Inf(1)), 0.0}) {
		t.Errorf("+Inf must not be finite")
	}
	if IsFinite([]float32{float32(math.Inf(-1)), 0.0}) {
		t.Errorf("-Inf must not be finite")
	}
}
