package ttsworker

import "bytes"

// bytesEqualLocal is a tiny alias to break the name collision
// between local `bytes` variables and the bytes package in test
// code. Prefer using bytes.Equal directly; this helper exists only
// for tests that need a name-safe helper.
func bytesEqualLocal(a, b []byte) bool { return bytes.Equal(a, b) }
