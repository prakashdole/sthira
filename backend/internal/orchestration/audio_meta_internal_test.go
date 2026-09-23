package orchestration

import (
	"strings"
	"testing"
)

// TestReadWAVHeader_AcceptsCanonicalPCM: the inspector accepts a
// well-formed PCM 16-bit LE mono WAV and reports the actual rate /
// channels / depth.
func TestReadWAVHeader_AcceptsCanonicalPCM(t *testing.T) {
	wav := make([]byte, 44)
	copy(wav[0:4], "RIFF")
	wav[8], wav[9], wav[10], wav[11] = 'W', 'A', 'V', 'E'
	copy(wav[12:16], "fmt ")
	wav[16], wav[17], wav[18], wav[19] = 16, 0, 0, 0
	wav[20], wav[21] = 1, 0                               // PCM
	wav[22], wav[23] = 1, 0                               // mono
	wav[24], wav[25], wav[26], wav[27] = 0x80, 0x3E, 0, 0 // 16000 LE
	wav[28], wav[29], wav[30], wav[31] = 0, 0x7D, 0, 0    // 32000 byte rate
	wav[32], wav[33] = 2, 0                               // block align
	wav[34], wav[35] = 16, 0                              // bits per sample
	copy(wav[36:40], "data")
	wav[40], wav[41], wav[42], wav[43] = 0, 0, 0, 0

	h, err := readWAVHeader(wav)
	if err != nil {
		t.Fatalf("readWAVHeader: %v", err)
	}
	if h.SampleRate != 16000 || h.Channels != 1 || h.BitDepth != 16 {
		t.Errorf("header = %+v, want 16000/1/16", h)
	}
}

// TestReadWAVHeader_RejectsNonPCMFormat: only PCM (format=1) is
// accepted; the worker pipeline never produces WAVE_FORMAT_EXTENSIBLE
// or float samples.
func TestReadWAVHeader_RejectsNonPCMFormat(t *testing.T) {
	wav := make([]byte, 44)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	wav[16], wav[17], wav[18], wav[19] = 16, 0, 0, 0
	wav[20], wav[21] = 3, 0 // IEEE FLOAT
	wav[22], wav[23] = 1, 0
	wav[24], wav[25], wav[26], wav[27] = 0x80, 0x3E, 0, 0
	wav[34], wav[35] = 16, 0
	copy(wav[36:40], "data")

	if _, err := readWAVHeader(wav); err == nil || !strings.Contains(err.Error(), "PCM") {
		t.Fatalf("non-PCM must be rejected, got %v", err)
	}
}

// TestReadWAVHeader_Rejects24Bit: only 16-bit PCM is in the
// accepted envelope. 24-bit would imply a different cache key
// shape and is out of policy.
func TestReadWAVHeader_Rejects24Bit(t *testing.T) {
	wav := make([]byte, 44)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	wav[16], wav[17], wav[18], wav[19] = 16, 0, 0, 0
	wav[20], wav[21] = 1, 0
	wav[22], wav[23] = 1, 0
	wav[24], wav[25], wav[26], wav[27] = 0x80, 0x3E, 0, 0
	wav[34], wav[35] = 24, 0
	copy(wav[36:40], "data")

	if _, err := readWAVHeader(wav); err == nil {
		t.Fatalf("24-bit must be rejected")
	}
}

// TestReadWAVHeader_RejectsTruncated: header shorter than 44 bytes
// is rejected. Corrupt / oversized audio must NOT panic.
func TestReadWAVHeader_RejectsTruncated(t *testing.T) {
	for _, n := range []int{0, 12, 36, 43} {
		if _, err := readWAVHeader(make([]byte, n)); err == nil {
			t.Errorf("len=%d must be rejected", n)
		}
	}
}

// TestReadWAVHeader_RejectsZeroFields: a header that reports zero
// sample rate / channels / bit depth is rejected (defensive).
func TestReadWAVHeader_RejectsZeroFields(t *testing.T) {
	wav := make([]byte, 44)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	copy(wav[12:16], "fmt ")
	wav[16], wav[17], wav[18], wav[19] = 16, 0, 0, 0
	wav[20], wav[21] = 1, 0                         // PCM
	wav[22], wav[23] = 0, 0                         // zero channels
	wav[24], wav[25], wav[26], wav[27] = 0, 0, 0, 0 // zero rate
	wav[34], wav[35] = 16, 0
	copy(wav[36:40], "data")

	if _, err := readWAVHeader(wav); err == nil {
		t.Fatalf("zero fields must be rejected")
	}
}
