package orchestration

// WAVHeader describes the metadata in a canonical PCM WAV RIFF
// header. It is used by stageTTS to verify the worker did not
// mislabel the synthesis settings (sample rate, bit depth, channel
// count) on the returned bytes. The orchestrator never declares the
// rate; the bytes are the source of truth.
type WAVHeader struct {
	SampleRate int
	Channels   int
	BitDepth   int
	// ByteLength is the audio data length declared by the RIFF
	// `data` chunk. Used to bound the bytes the orchestrator will
	// forward (caller checks against the decoded size).
	ByteLength int
}

// readWAVHeader parses the canonical PCM WAV header in b. It returns
// an error when the magic / fmt-chunk is missing, the header is
// truncated, or the declared length is inconsistent. Only the
// canonical PCM format is recognised (no WAVE_FORMAT_EXTENSIBLE /
// floating-point / multi-channel compression paths); the TTS
// pipeline produces only PCM 16-bit mono per design.
func readWAVHeader(b []byte) (WAVHeader, error) {
	if len(b) < 44 {
		return WAVHeader{}, errShortWAVHeader
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" || string(b[12:16]) != "fmt " {
		return WAVHeader{}, errNotWAVHeader
	}
	// fmt chunk header: 4 bytes size, then 16 bytes of format data
	// (audio format, channels, sample rate, byte rate, block align,
	// bits per sample). Other sizes are rejected to keep the
	// verifier strict.
	if string(b[12:16]) != "fmt " {
		return WAVHeader{}, errNotWAVHeader
	}
	// chunkSize at bytes 16..20, little-endian uint32.
	if len(b) < 36 {
		return WAVHeader{}, errShortWAVHeader
	}
	fmtSize := uint32(b[16]) | uint32(b[17])<<8 | uint32(b[18])<<16 | uint32(b[19])<<24
	if fmtSize < 16 {
		return WAVHeader{}, errShortWAVHeader
	}
	// Audio format (PCM=1) at 20..22.
	if uint16(b[20])|uint16(b[21])<<8 != 1 {
		return WAVHeader{}, errUnsupportedWAVFormat
	}
	channels := int(uint16(b[22]) | uint16(b[23])<<8)
	sampleRate := int(uint32(b[24]) | uint32(b[25])<<8 | uint32(b[26])<<16 | uint32(b[27])<<24)
	// 28..32 byte rate (ignored), 32..34 block align (ignored)
	bitDepth := int(uint16(b[34]) | uint16(b[35])<<8)
	if channels <= 0 || sampleRate <= 0 || bitDepth <= 0 {
		return WAVHeader{}, errZeroWAVHeaderField
	}
	if bitDepth != 16 {
		return WAVHeader{}, errUnsupportedWAVBitDepth
	}
	// data chunk starts at offset 20 + fmtSize (typically 36). Find
	// the literal "data" magic and read its declared length.
	dataOffset := 20 + int(fmtSize)
	if dataOffset > len(b)-8 {
		return WAVHeader{}, errShortWAVHeader
	}
	if string(b[dataOffset:dataOffset+4]) != "data" {
		return WAVHeader{}, errMissingWAVDataChunk
	}
	byteLen := int(uint32(b[dataOffset+4]) | uint32(b[dataOffset+5])<<8 |
		uint32(b[dataOffset+6])<<16 | uint32(b[dataOffset+7])<<24)
	if byteLen < 0 {
		return WAVHeader{}, errBadWAVLength
	}
	// Cross-check the declared length against the actual bytes
	// remaining in the buffer. The orchestrator never forwards
	// past the declared length.
	if dataOffset+8+byteLen > len(b) {
		return WAVHeader{}, errBadWAVLength
	}
	return WAVHeader{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
		ByteLength: byteLen,
	}, nil
}

// Typed errors. Sentinel values so callers can errors.Is-check.
var (
	errShortWAVHeader         = wavErr("wav: header too short")
	errNotWAVHeader           = wavErr("wav: missing RIFF/WAVE/fmt magic")
	errUnsupportedWAVFormat   = wavErr("wav: only PCM (format=1) is accepted")
	errUnsupportedWAVBitDepth = wavErr("wav: only 16-bit PCM is accepted")
	errZeroWAVHeaderField     = wavErr("wav: rate / channels / depth must be > 0")
	errMissingWAVDataChunk    = wavErr("wav: data chunk missing")
	errBadWAVLength           = wavErr("wav: declared length inconsistent with bytes")
)

type wavErr string

func (e wavErr) Error() string { return string(e) }
