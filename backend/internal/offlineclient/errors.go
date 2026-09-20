// Package offlineclient is the P5 protocol test client: a disk-backed,
// resumable, verifiable downloader for the public incident card, signed
// manifest and optional map/audio resources defined by plan/p5-contract.md.
//
// It is a test/reference harness for the P5 delivery protocol, not a
// production mobile frontend. The native Android/iOS clients are P8.
//
// Trust posture: the client never trusts content merely because the bytes
// arrived. Every artifact must pass checksum verification AND signature
// verification under a TrustStore authorized for the artifact's
// jurisdiction before activation. A failed refresh never destroys a
// previously valid active state — the previous bytes are preserved until
// the new ones have been verified and atomically swapped in.
//
// Untrusted clocks: monotonic elapsed time (from time.Since via the injected
// Now function) is the only clock the client uses for expiry decisions.
// Wall-clock rollback, NTP jumps and user tampering cannot extend the
// server-asserted validity window.
package offlineclient

import "errors"

// Errors returned by the public API. Internal helpers may wrap these.
var (
	// ErrNoActiveState is returned by state queries when the storage
	// directory contains no verified manifest/card (cold start, lost
	// storage, or activation was interrupted before state.json was
	// written).
	ErrNoActiveState = errors.New("offlineclient: no verified active state")

	// ErrInterrupted is returned when an in-progress sync is interrupted
	// before activation. The .part files persist on disk and the next
	// Sync call will resume or restart them.
	ErrInterrupted = errors.New("offlineclient: download or activation interrupted")

	// ErrPartial is returned when a download is truncated mid-stream.
	// The partial bytes are persisted to .part for the next Sync.
	ErrPartial = errors.New("offlineclient: partial download")

	// ErrRangeNotSupported is returned when the server does not advertise
	// Accept-Ranges and a resume was requested. The client falls back to
	// a full re-download in this case.
	ErrRangeNotSupported = errors.New("offlineclient: server does not support byte ranges")

	// ErrTooLarge is returned when a downloaded artifact exceeds the
	// configured byte ceiling (default 100 MiB for resources).
	ErrTooLarge = errors.New("offlineclient: artifact exceeds configured byte ceiling")

	// ErrClockRolledBack is returned when monotonic time observation is
	// negative relative to the recorded sync instant — i.e. monotonic
	// time itself is broken, not just wall-clock. The client surfaces
	// this as a hard failure because no time-based decision can be
	// trusted while it holds.
	ErrClockRolledBack = errors.New("offlineclient: monotonic clock rolled back")

	// ErrStorageUnavailable is returned when the configured StorageDir
	// cannot be read or written. The caller can treat this as a hard
	// failure and prompt the user to free space.
	ErrStorageUnavailable = errors.New("offlineclient: storage directory unavailable")
)
