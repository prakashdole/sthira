package ttsworker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Audio package: pure-Go PCM 16-bit signed little-endian WAV encoder
// and a silence generator for the blocked real-inference stub.
//
// The worker outputs only `audio/wav` PCM 16-bit LE mono at 22050 Hz.
// The encoder refuses to write above MaxOutputBytes or
// MaxOutputDuration; this is the codec/output ceiling the worker's
// prompt requires and is independent of any client-declared size.
//
// Sample rate 22050 Hz matches the documented Parler-TTS / cache
// candidate envelope. Bit depth is 16, channels is 1 (mono). The
// encoder refuses to write float samples outside int16 range and
// refuses a duration estimate that exceeds the declared budget.

const (
	// DefaultOutputSampleRate is the legacy fallback rate used only
	// when no runtime has negotiated a native rate. The Indic
	// Parler-TTS adapter reports its real rate (model.config.
	// sampling_rate, verified from the model card: audio is written
	// at that rate) at ready time; encoding at any other rate
	// changes playback speed, so the runtime path never uses this.
	DefaultOutputSampleRate = 22050
	// MaxOutputSampleRate caps the rate regardless of what the
	// runtime reports. Indic Parler-TTS (Parler-TTS Mini lineage)
	// runs at ~44.1 kHz native, so the previous 24 kHz cap would
	// have rejected the real artifact's own output.
	MaxOutputSampleRate = 48000
	// MaxOutputDurationSeconds is the codec-level ceiling for one
	// response.
	MaxOutputDurationSeconds = 12.0
	// MaxOutputBytes caps the encoded WAV size in bytes, including
	// the 44-byte header: 12 s @ 48 kHz mono PCM16 = 1,152,000.
	MaxOutputBytes = 1200 * 1024
	// MaxSynthesizedTextBytes caps the rendered-text size so a
	// long template cannot produce a runaway audio length.
	MaxSynthesizedTextBytes = 1024
)

// WavOutput is the result of encoding samples to PCM WAV. Bytes is
// the encoded file; ChecksumSHA256 is its SHA-256 in lowercase hex.
// Caller is expected to scrub Bytes (zero and drop references) once
// the audio is handed off.
type WavOutput struct {
	Bytes          []byte
	SampleRate     int
	Channels       int
	BitDepth       int
	ChecksumSHA256 string
	DurationSec    float64
}

// EncodeSilenceWav produces a PCM 16-bit LE mono WAV file containing
// `duration` seconds of zeros at sample rate `sampleRate`. Refuses
// out-of-band parameters with a typed error. Used by the stub runtime
// when the worker is forced to construct a deterministic placeholder
// before the audio buffer is wiped.
//
// The prompt explicitly forbids faking audio as successful speech;
// EncodeSilenceWav is a worker primitive, not a synthesizer.
func EncodeSilenceWav(sampleRate int, duration float64) (*WavOutput, error) {
	if sampleRate <= 0 || sampleRate > MaxOutputSampleRate {
		return nil, fmt.Errorf("sample rate %d invalid (≤ %d)", sampleRate, MaxOutputSampleRate)
	}
	if duration <= 0 || duration > MaxOutputDurationSeconds {
		return nil, fmt.Errorf("duration %g invalid (≤ %g)", duration, MaxOutputDurationSeconds)
	}
	if MaxOutputBytes < 44 {
		return nil, errors.New("limit misconfigured")
	}
	maxSamples := int(float64(sampleRate) * duration)
	if maxSamples <= 0 {
		return nil, errors.New("zero samples requested")
	}
	// Sample count must fit into the byte budget.
	maxSamplesByBytes := (MaxOutputBytes - 44) / 2
	if maxSamples > maxSamplesByBytes {
		return nil, fmt.Errorf("requested samples %d exceeds byte cap %d", maxSamples, maxSamplesByBytes)
	}
	header := buildWavHeader(sampleRate, 1, 16, maxSamples*2)
	buf := make([]byte, 0, len(header)+maxSamples*2)
	buf = append(buf, header...)
	buf = append(buf, make([]byte, maxSamples*2)...) // zero PCM data
	if len(buf) > MaxOutputBytes {
		return nil, fmt.Errorf("encoded size %d > %d", len(buf), MaxOutputBytes)
	}
	return &WavOutput{
		Bytes:          buf,
		SampleRate:     sampleRate,
		Channels:       1,
		BitDepth:       16,
		ChecksumSHA256: sha256Hex(buf),
		DurationSec:    float64(maxSamples) / float64(sampleRate),
	}, nil
}

// Float32ToWav encodes float32 mono samples in [-1,1] to PCM 16-bit
// LE mono WAV. Refuses out-of-band parameters, clipping with a typed
// error rather than silently saturating. Duration is checked BEFORE
// byte count so the documented limit surfaces as "duration" rather
// than as a confusing "samples exceed byte cap".
func Float32ToWav(samples []float32, sampleRate int) (*WavOutput, error) {
	if len(samples) == 0 {
		return nil, errors.New("empty samples")
	}
	if sampleRate <= 0 || sampleRate > MaxOutputSampleRate {
		return nil, fmt.Errorf("sample rate %d invalid (≤ %d)", sampleRate, MaxOutputSampleRate)
	}
	duration := float64(len(samples)) / float64(sampleRate)
	if duration > MaxOutputDurationSeconds {
		return nil, fmt.Errorf("duration %g > %g", duration, MaxOutputDurationSeconds)
	}
	maxSamplesByBytes := (MaxOutputBytes - 44) / 2
	if len(samples) > maxSamplesByBytes {
		return nil, fmt.Errorf("samples %d exceed byte cap %d", len(samples), maxSamplesByBytes)
	}
	for i, s := range samples {
		if s < -1 || s > 1 {
			return nil, fmt.Errorf("sample %d out of range %g", i, s)
		}
	}
	buf := append([]byte(nil), buildWavHeader(sampleRate, 1, 16, len(samples)*2)...)
	for _, s := range samples {
		v := int16(math.Round(float64(s) * 32767))
		buf = binary.LittleEndian.AppendUint16(buf, uint16(v))
	}
	if len(buf) > MaxOutputBytes {
		return nil, fmt.Errorf("encoded size %d > %d", len(buf), MaxOutputBytes)
	}
	return &WavOutput{
		Bytes:          buf,
		SampleRate:     sampleRate,
		Channels:       1,
		BitDepth:       16,
		ChecksumSHA256: sha256Hex(buf),
		DurationSec:    duration,
	}, nil
}

func buildWavHeader(sampleRate, channels, bitDepth int, dataBytes int) []byte {
	h := make([]byte, 0, 44)
	h = append(h, 'R', 'I', 'F', 'F')
	h = binary.LittleEndian.AppendUint32(h, uint32(36+dataBytes))
	h = append(h, 'W', 'A', 'V', 'E')
	h = append(h, 'f', 'm', 't', ' ')
	h = binary.LittleEndian.AppendUint32(h, 16)
	h = binary.LittleEndian.AppendUint16(h, 1) // PCM
	h = binary.LittleEndian.AppendUint16(h, uint16(channels))
	h = binary.LittleEndian.AppendUint32(h, uint32(sampleRate))
	blockAlign := uint16(channels * bitDepth / 8)
	byteRate := uint32(sampleRate) * uint32(blockAlign)
	h = binary.LittleEndian.AppendUint32(h, byteRate)
	h = binary.LittleEndian.AppendUint16(h, blockAlign)
	h = binary.LittleEndian.AppendUint16(h, uint16(bitDepth))
	h = append(h, 'd', 'a', 't', 'a')
	h = binary.LittleEndian.AppendUint32(h, uint32(dataBytes))
	return h
}
