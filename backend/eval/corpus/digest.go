package corpus

import (
	"crypto/sha256"
	"encoding/hex"
)

// contentDigest returns the sha256:hex digest for audit purposes. The
// prefix matches the form audio fixtures must use.
func contentDigest(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
