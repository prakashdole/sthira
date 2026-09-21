package asrworker

import (
	"encoding/binary"
	"testing"
)

// buildRIFF wraps an fmt chunk and a data chunk into a RIFF/WAVE
// container. The declared chunk sizes may intentionally disagree with
// the number of body bytes actually written, to probe the decoder's
// handling of hostile size fields. Pass an empty id to omit a chunk.
func buildRIFF(fmtID string, fmtSize uint32, fmtBody []byte, dataID string, dataSize uint32, dataBody []byte) []byte {
	buf := make([]byte, 0, 12+8+len(fmtBody)+8+len(dataBody))
	buf = append(buf, "RIFF"...)
	riffSizeAt := len(buf)
	buf = append(buf, 0, 0, 0, 0)
	buf = append(buf, "WAVE"...)
	if fmtID != "" {
		buf = append(buf, fmtID...)
		buf = binary.LittleEndian.AppendUint32(buf, fmtSize)
		buf = append(buf, fmtBody...)
	}
	if dataID != "" {
		buf = append(buf, dataID...)
		buf = binary.LittleEndian.AppendUint32(buf, dataSize)
		buf = append(buf, dataBody...)
	}
	binary.LittleEndian.PutUint32(buf[riffSizeAt:], uint32(len(buf)-8))
	return buf
}

func pcmFmtBody() []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[0:], 1)     // PCM
	binary.LittleEndian.PutUint16(b[2:], 1)     // mono
	binary.LittleEndian.PutUint32(b[4:], 16000) // sample rate
	binary.LittleEndian.PutUint32(b[8:], 32000) // byte rate
	binary.LittleEndian.PutUint16(b[12:], 2)    // block align
	binary.LittleEndian.PutUint16(b[14:], 16)   // bits per sample
	return b
}

// decodeWAVMustNotPanic fails the test loudly if DecodeWAV panics on
// the supplied bytes; a returned error is the expected, safe outcome.
func decodeWAVMustNotPanic(t *testing.T, in []byte) (err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DecodeWAV panicked on malformed input: %v", r)
		}
	}()
	_, err = DecodeWAV(in, "audio/wav", DefaultDecodeLimits())
	return err
}

// TestDecodeWAV_Malformed_UnknownSizeFmt_NoPanic reproduces the
// 44-byte RIFF/WAVE whose fmt chunk declares the streaming sentinel
// 0xFFFFFFFF. Before the fix the decoder sliced
// b[pos+8 : pos+8+int(size)] and panicked. The unknown-length
// sentinel must only be honoured for the data chunk, never fmt.
func TestDecodeWAV_Malformed_UnknownSizeFmt_NoPanic(t *testing.T) {
	in := buildRIFF("fmt ", 0xFFFFFFFF, make([]byte, 24), "", 0, nil)
	if len(in) != 44 {
		t.Fatalf("want the 44-byte reproduction, got %d bytes", len(in))
	}
	if err := decodeWAVMustNotPanic(t, in); err == nil {
		t.Fatalf("expected rejection of unknown-size fmt chunk, got nil error")
	}
}

// TestDecodeWAV_Malformed_OversizeKnownFmt_NoPanic: a fmt chunk whose
// known size runs far past EOF must be rejected before any slice.
func TestDecodeWAV_Malformed_OversizeKnownFmt_NoPanic(t *testing.T) {
	in := buildRIFF("fmt ", 0x7FFFFFFF, pcmFmtBody(), "", 0, nil)
	if err := decodeWAVMustNotPanic(t, in); err == nil {
		t.Fatalf("expected oversize fmt rejection, got nil error")
	}
}

// TestDecodeWAV_Malformed_OversizeKnownData_NoPanic: a data chunk that
// declares more bytes than the file holds must fail the truncation
// check without a huge allocation or slice panic.
func TestDecodeWAV_Malformed_OversizeKnownData_NoPanic(t *testing.T) {
	in := buildRIFF("fmt ", 16, pcmFmtBody(), "data", 0x7FFFFFFF, []byte{1, 2, 3, 4})
	if err := decodeWAVMustNotPanic(t, in); err == nil {
		t.Fatalf("expected truncated data rejection, got nil error")
	}
}

// TestDecodeRIFFWAV_ShortHeaderBelowSentinel_NoPanic: a tiny input that
// still passes the 44-byte gate but has a truncated chunk table must
// not index out of range.
func TestDecodeRIFFWAV_TrailingJunkAfterData_NoPanic(t *testing.T) {
	in := buildRIFF("fmt ", 16, pcmFmtBody(), "data", 4, []byte{1, 0, 2, 0})
	in = append(in, "junkjunkjunk"...) // trailing bytes after a valid data chunk
	// Must decode (4 bytes = 2 mono int16 samples) or reject cleanly,
	// never panic.
	_ = decodeWAVMustNotPanic(t, in)
}

// FuzzDecodeWAVChunkSizes drives the RIFF chunk-size fields with
// arbitrary values to prove no combination of fmt/data size fields
// panics or over-allocates the decoder. Bounded seed corpus; the entry
// CompressedBytes gate plus the uint64 overflow checks keep the fuzz
// target cheap and safe.
func FuzzDecodeWAVChunkSizes(f *testing.F) {
	f.Add(buildRIFF("fmt ", 16, pcmFmtBody(), "data", 4, []byte{1, 0, 2, 0}))
	f.Add(buildRIFF("fmt ", 0xFFFFFFFF, make([]byte, 24), "", 0, nil))
	f.Add(buildRIFF("fmt ", 0x7FFFFFFF, pcmFmtBody(), "data", 0xFFFFFFFF, []byte{1, 2, 3, 4}))
	f.Add(buildRIFF("fmt ", 0, nil, "data", 0, nil))
	f.Add(buildRIFF("fmt ", 16, pcmFmtBody(), "LIST", 0xFFFFFFFF, []byte{0}))
	f.Add([]byte("RIFF"))
	f.Add([]byte("RIFF\x00\x00\x00\x00WAVE"))
	f.Add([]byte("RIFF\x04\x00\x00\x00WAV"))

	lim := DefaultDecodeLimits()
	lim.CompressedBytes = 1 << 20
	f.Fuzz(func(t *testing.T, in []byte) {
		if int64(len(in)) > lim.CompressedBytes {
			t.Skip("oversize input handled by the entry gate")
		}
		// Any error is an acceptable outcome; a panic fails the fuzz run.
		_, _ = DecodeWAV(in, "audio/wav", lim)
	})
}
