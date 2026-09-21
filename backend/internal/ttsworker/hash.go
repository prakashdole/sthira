package ttsworker

import "crypto/sha256"
import "encoding/hex"

// sha256Hex returns the lowercase hex-encoded SHA-256 digest of b.
// Kept here so callers don't have to import crypto/sha256 directly.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
