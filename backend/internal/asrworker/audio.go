package asrworker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// Wire formats this worker actually decodes. The private protocol
// accepts a payload that arrives as bounded compressed bytes; the
// worker validates the declared Content-Type against this allow list
// BEFORE attempting decode. Anything else is rejected with a
// structured "AUDIO_UNAVAILABLE" state and reason="unsupported codec".
//
// The public handler's allow-list (see contracts.IsSupportedTranscriptionContentType)
// is broader (audio/wav, audio/webm, audio/ogg); this worker narrows
// it to codecs the runtime can really consume. The orchestrator must
// pre-transcode before calling /transcribe.
var WorkerSupportedContentTypes = []string{"audio/wav"}

// AudioDecodeResult is the typed result of decoding a bounded
// compressed audio blob into a single mono buffer at the requested
// target sample rate.
//
// Samples is a freshly-allocated []float32 in the range [-1, 1]. The
// caller's lifetime owns this slice; once it goes out of scope the GC
// reclaims it. Raw audio bytes are NOT kept by the worker.
type AudioDecodeResult struct {
	Samples       []float32 // mono, in [-1,1]
	SampleRate    int       // post-resample rate (always == TargetSampleRate)
	OriginalRate  int       // rate from the WAV header before resample
	ChannelsIn    int       // channels in the source file
	DurationSecs  float64   // len(Samples) / TargetSampleRate
	BytesConsumed int       // bytes consumed from the input (== len(audioBytes))
	ContentType   string    // the wire content-type
}

// AudioDecodeLimits bounds the worker's audio intake. These are the
// private-worker counterpart to the contracts.Max* limits. They are
// independent from the public handler's limits so a misconfigured
// orchestrator cannot trick the worker into unbounded work.
type AudioDecodeLimits struct {
	// CompressedBytes is the upper bound on raw input bytes.
	CompressedBytes int64
	// DecodedMonoSeconds caps the post-resample duration.
	DecodedMonoSeconds float64
	// MinSampleRate / MaxSampleRate bound the source sample rate.
	// Both pre-resample and post-resample are clamped to this range.
	MinSampleRate int
	MaxSampleRate int
	// MaxChannels is the upper bound on the channel count in the
	// source. Mono (1) is the only supported mode downstream.
	MaxChannels int
	// MonoOnly forces a mono downmix for multi-channel sources.
	MonoOnly bool
}

// DefaultDecodeLimits mirrors the public handler limits plus a
// stricter MonoOnly contract. Any source with >1 channels is rejected
// when MonoOnly=true; we never silently downmix a multi-channel file
// because the runtime expects mono and the silence between channels
// may be ambiguous.
func DefaultDecodeLimits() AudioDecodeLimits {
	return AudioDecodeLimits{
		CompressedBytes:    MaxCompressedBytes,
		DecodedMonoSeconds: MaxDecodedSeconds,
		MinSampleRate:      8000,
		MaxSampleRate:      48000,
		MaxChannels:        1,
		MonoOnly:           true,
	}
}

// MaxCompressedBytes is the private-worker byte cap. Matches
// contracts.MaxTranscriptionCompressedBytes; duplicated here so the
// private worker module does not import the orchestrator's contracts
// package.
const MaxCompressedBytes int64 = 512 * 1024

// MaxDecodedSeconds is the post-resample duration cap.
const MaxDecodedSeconds float64 = 20.0

// TargetSampleRate is the rate the ASR runtime expects (16 kHz
// mono float32). The reference resamples to 16 kHz; the worker
// hard-codes that because the runtime is locked to CTC at 16 kHz.
const TargetSampleRate = 16000

// DecodeError is a typed error from the audio pipeline. The HTTP
// handler maps this to a typed Transcript state.
type DecodeError struct {
	Reason string
	Cause  error
}

func (e *DecodeError) Error() string { return "decode: " + e.Reason }
func (e *DecodeError) Unwrap() error { return e.Cause }

// DecodeWAV decodes a 16-bit signed little-endian PCM mono-or-mono-compatible
// WAV. The function NEVER caches, NEVER writes to disk, NEVER keeps
// references to the input slice. It returns a freshly-allocated
// float32 buffer plus the original rate so the runtime can resample
// at its preferred rate (we target 16 kHz downstream).
//
// Limits are enforced against the post-decode / post-resample size;
// we refuse oversized audio at the earliest signal we can measure it.
//
// Errors are typed:
//   - *DecodeError typed "oversize" / "truncated" / "malformed_header"
//   - errors.Is(err, ErrUnsupportedCodec) when content_type is not in
//     WorkerSupportedContentTypes.
func DecodeWAV(audioBytes []byte, contentType string, limits AudioDecodeLimits) (*AudioDecodeResult, error) {
	if !isWorkerSupportedCodec(contentType) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCodec, contentType)
	}
	if int64(len(audioBytes)) > limits.CompressedBytes {
		return nil, &DecodeError{Reason: "audio bytes exceed limit",
			Cause: fmt.Errorf("got %d > %d", len(audioBytes), limits.CompressedBytes)}
	}
	if len(audioBytes) == 0 {
		return nil, &DecodeError{Reason: "audio bytes are empty"}
	}
	samples, srcRate, channels, err := decodeRIFFWAV(audioBytes, limits)
	if err != nil {
		return nil, err
	}
	if srcRate < limits.MinSampleRate || srcRate > limits.MaxSampleRate {
		return nil, &DecodeError{Reason: "sample rate out of bounds",
			Cause: fmt.Errorf("got %d allowed [%d,%d]", srcRate, limits.MinSampleRate, limits.MaxSampleRate)}
	}
	if channels > limits.MaxChannels {
		if limits.MonoOnly {
			return nil, &DecodeError{Reason: "multi-channel audio is rejected",
				Cause: fmt.Errorf("got %d channels, mono only", channels)}
		}
	}
	// Resample to TargetSampleRate.
	resampled := resampleLinear(samples, srcRate, TargetSampleRate)
	duration := float64(len(resampled)) / float64(TargetSampleRate)
	if duration > limits.DecodedMonoSeconds {
		// Drop the buffer before returning.
		for i := range resampled {
			resampled[i] = 0
		}
		return nil, &DecodeError{Reason: "decoded duration exceeds limit",
			Cause: fmt.Errorf("got %.2fs > %.2fs", duration, limits.DecodedMonoSeconds)}
	}
	return &AudioDecodeResult{
		Samples:       resampled,
		SampleRate:    TargetSampleRate,
		OriginalRate:  srcRate,
		ChannelsIn:    channels,
		DurationSecs:  duration,
		BytesConsumed: len(audioBytes),
		ContentType:   contentType,
	}, nil
}

// ErrUnsupportedCodec is returned by DecodeWAV when the content-type
// does not name a codec this worker actually decodes. The HTTP layer
// maps this to TranscriptionAudioUnavailable.
var ErrUnsupportedCodec = errors.New("unsupported audio codec")

func isWorkerSupportedCodec(ct string) bool {
	for _, t := range WorkerSupportedContentTypes {
		if t == ct {
			return true
		}
	}
	return false
}

// decodeRIFFWAV parses a RIFF/WAVE file and returns the float32
// mono samples, source sample rate, and channel count.
//
// Supported formats:
//   - PCM 16-bit signed little-endian (audio format == 1)
//
// Other format tags (e.g. IEEE float, A-law, μ-law, MP3-in-WAV) are
// rejected as "unsupported_pcm". The PCM 16-bit LE constraint
// matches the public handler contract; we never silently accept an
// arbitrary encoding.
//
// Truncation: the WAV declares a data chunk size; we compare the
// declared remaining bytes to what's actually present and refuse
// truncated inputs.
func decodeRIFFWAV(b []byte, limits AudioDecodeLimits) (samples []float32, sampleRate, channels int, err error) {
	if len(b) < 44 {
		return nil, 0, 0, &DecodeError{Reason: "wav header truncated", Cause: io.ErrUnexpectedEOF}
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, 0, &DecodeError{Reason: "wav missing RIFF/WAVE signature"}
	}
	// Walk chunks until we find fmt and data.
	pos := 12
	var audioFormat uint16
	var bitsPerSample uint16
	var byteRate uint32
	var blockAlign uint16
	var dataStart, dataLen int
	for pos+8 <= len(b) {
		id := string(b[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(b[pos+4 : pos+8]))
		if pos+8+size > len(b) && id != "data" {
			// Malformed: chunk declares more bytes than the file.
			return nil, 0, 0, &DecodeError{Reason: "wav chunk overruns file",
				Cause: fmt.Errorf("chunk %s size %d at offset %d", id, size, pos)}
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, 0, &DecodeError{Reason: "wav fmt chunk too short",
					Cause: fmt.Errorf("got %d bytes", size)}
			}
			fmtChunk := b[pos+8 : pos+8+size]
			audioFormat = binary.LittleEndian.Uint16(fmtChunk[0:2])
			channels = int(binary.LittleEndian.Uint16(fmtChunk[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(fmtChunk[4:8]))
			byteRate = binary.LittleEndian.Uint32(fmtChunk[8:12])
			blockAlign = binary.LittleEndian.Uint16(fmtChunk[12:14])
			bitsPerSample = binary.LittleEndian.Uint16(fmtChunk[14:16])
		case "data":
			dataStart = pos + 8
			dataLen = size
			// Don't return yet: read on in case there is junk
			// after; but for this header we only care about the
			// first data chunk, which is the canonical layout.
		}
		pos += 8 + size
		if size%2 == 1 && pos < len(b) {
			pos++ // pad byte
		}
		if dataLen > 0 && audioFormat != 0 {
			break
		}
	}
	if audioFormat == 0 {
		return nil, 0, 0, &DecodeError{Reason: "wav missing fmt chunk"}
	}
	if dataStart == 0 {
		return nil, 0, 0, &DecodeError{Reason: "wav missing data chunk"}
	}
	if audioFormat != 1 {
		return nil, 0, 0, &DecodeError{Reason: "wav audio format not PCM16_LE",
			Cause: fmt.Errorf("got %d", audioFormat)}
	}
	if bitsPerSample != 16 {
		return nil, 0, 0, &DecodeError{Reason: "wav bits per sample not 16",
			Cause: fmt.Errorf("got %d", bitsPerSample)}
	}
	if channels < 1 {
		return nil, 0, 0, &DecodeError{Reason: "wav declared zero channels"}
	}
	if int(blockAlign) != channels*int(bitsPerSample)/8 {
		return nil, 0, 0, &DecodeError{Reason: "wav block align mismatched",
			Cause: fmt.Errorf("block %d vs channels*bits %d", blockAlign, channels*int(bitsPerSample)/8)}
	}
	_ = byteRate
	// Verify truncation.
	wantEnd := dataStart + dataLen
	if wantEnd > len(b) {
		return nil, 0, 0, &DecodeError{Reason: "wav data chunk truncated",
			Cause: fmt.Errorf("declared %d bytes at %d, file ends at %d", dataLen, dataStart, len(b))}
	}
	pcm := b[dataStart:wantEnd]
	frameSize := channels * 2
	if len(pcm)%frameSize != 0 {
		return nil, 0, 0, &DecodeError{Reason: "wav pcm not aligned to frame",
			Cause: fmt.Errorf("mod %d", frameSize)}
	}
	frameCount := len(pcm) / frameSize
	// Multi-channel: take channel 0 only (first channel). The
	// runtime is mono-only; we never silently average channels
	// because that loses information.
	if channels == 1 {
		out := make([]float32, frameCount)
		for i := 0; i < frameCount; i++ {
			s := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
			out[i] = float32(s) / 32768.0
		}
		return out, sampleRate, channels, nil
	}
	if channels > 1 && limits.MonoOnly {
		return nil, 0, channels, &DecodeError{Reason: "multi-channel audio rejected (mono only)",
			Cause: fmt.Errorf("channels=%d", channels)}
	}
	// channels > 1 with MonoOnly=false: collapse to channel 0 only.
	out := make([]float32, frameCount)
	for i := 0; i < frameCount; i++ {
		s := int16(binary.LittleEndian.Uint16(pcm[i*frameSize : i*frameSize+2]))
		out[i] = float32(s) / 32768.0
	}
	return out, sampleRate, 1, nil
}

// resampleLinear uses linear interpolation to resample mono samples
// from srcRate to dstRate. The downsampler path here is a simple box
// filter with linear interpolation; that is sufficient for the ASR
// front-end because the runtime is CTC, which is robust to mild
// spectral tilt. For full-band music this would be wrong; for
// narrow-band speech it is the cheapest standard-library-equivalent
// pass.
//
// Caller MUST zero out the returned slice when finished if it wants
// to overwrite the samples; the worker does this on the
// oversize-duration path to avoid a transient short-lived
// buffer holding decoded audio.
//
// Boundaries:
//   - srcRate < 1 or dstRate < 1 → returns the input unchanged.
//   - Empty input → empty output.
func resampleLinear(samples []float32, srcRate, dstRate int) []float32 {
	if len(samples) == 0 || srcRate == dstRate {
		return samples
	}
	if srcRate < 1 || dstRate < 1 {
		return samples
	}
	out := make([]float32, int(math.Ceil(float64(len(samples))*float64(dstRate)/float64(srcRate))))
	if len(out) == 0 {
		return out
	}
	ratio := float64(srcRate) / float64(dstRate)
	for i := range out {
		srcIdx := float64(i) * ratio
		i0 := int(srcIdx)
		i1 := i0 + 1
		if i1 >= len(samples) {
			i1 = len(samples) - 1
		}
		w := srcIdx - float64(i0)
		if i0 >= len(samples) {
			// Past the end of input (round-up ceil). Repeat last sample.
			out[i] = samples[len(samples)-1]
			continue
		}
		out[i] = float32((1-w)*float64(samples[i0]) + w*float64(samples[i1]))
	}
	return out
}

// SilenceDetector reports whether the buffer is all-zero or
// numerically negligible. Used by the stub runtime only to produce
// a "no speech detected" response when silence is the entire input.
// This is a band-aid for the stub path; a real runtime performs its
// own silence / VAD detection and would never go through this.
func SilenceDetector(samples []float32) bool {
	for _, s := range samples {
		if math.Abs(float64(s)) > 1e-6 {
			return false
		}
	}
	return true
}

// WipeBuffer zeros a float32 slice. Used by the worker to scrub the
// decoded audio buffer once downstream is done. The caller MUST
// guarantee no other goroutine holds a reference.
func WipeBuffer(samples []float32) {
	for i := range samples {
		samples[i] = 0
	}
}
