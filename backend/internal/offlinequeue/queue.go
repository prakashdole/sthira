// Package offlinequeue is the P5 reference harness for durable pending offline
// writes. It provides a small disk-backed queue of unconfirmed operations a
// citizen client accumulated while disconnected. Each entry preserves the
// idempotency key and the exact intended payload, plus the snapshot version
// the client confirmed against. Replay submits the operation through the
// existing server contract; pending is never reserved and the queue itself
// never holds capacity.
//
// Threat posture: bearer tokens are NOT persisted in queue entries or logs.
// Entries carry an opaque TokenRef (a string label resolved through a separate
// in-memory TokenProvider). Pending is unconfirmed: nothing in the queue ever
// represents a server-side commitment. A changed selection/payload requires
// renewed explicit confirmation and a NEW idempotency key — the queue refuses
// to silently overwrite the snapshot or payload of an uncertain prior
// submission.
//
// Scope: this is a reference harness for P5 acceptance tests, not a production
// mobile client. It uses the Go standard library only (plan/p5-contract.md
// §2.3); no third-party modules. State lives behind explicit values passed at
// construction. No package-level mutable variables; the clock is injected.
package offlinequeue

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// OpState is the durable state of a pending operation.
//
// State transitions:
//
//	PENDING -> IN_FLIGHT                      (worker dispatches)
//	IN_FLIGHT -> COMMITTED                    (server 2xx)
//	IN_FLIGHT -> FAILED_STALE                 (snapshot drift, hold expired, or 4xx selection error)
//	IN_FLIGHT -> FAILED_PERM                  (4xx other than stale/hold, or max attempts exceeded)
//	IN_FLIGHT -> PENDING_RECONCILIATION       (network/timeout error after dispatch)
//	PENDING -> IN_FLIGHT                      (next drain attempt)
//	PENDING_RECONCILIATION -> IN_FLIGHT       (next drain attempt; selection window ignored — must ask server)
//	PENDING_RECONCILIATION -> COMMITTED       (server confirms prior commit via same idempotency key)
//	PENDING_RECONCILIATION -> FAILED_STALE    (server confirms snapshot drift / hold expired)
//	PENDING_RECONCILIATION -> FAILED_PERM     (server explicitly rejects the prior submission)
//	COMMITTED -> (removed by PurgeCommitted, never auto-retried)
//	FAILED_STALE/Failed_PERM -> (held for inspection; new operation needed)
//
// PENDING_RECONCILIATION preserves uncertainty: a transport failure or
// retry exhaustion is not proof of server rejection. The server may have
// committed the operation and the response was lost before the client
// observed it; only a successful replay (with the same idempotency key)
// or an explicit server rejection resolves the entry. A local selection
// expiry does NOT fail-stale an uncertain entry: the client cannot prove
// the hold expired without asking the server, and the server is the only
// party that can answer.
type OpState string

const (
	StatePending               OpState = "PENDING"
	StateInFlight              OpState = "IN_FLIGHT"
	StatePendingReconciliation OpState = "PENDING_RECONCILIATION" // network error or timeout after dispatch; requires server reconciliation
	StateCommitted             OpState = "COMMITTED"
	StateFailedStale           OpState = "FAILED_STALE" // snapshot drift or hold expiry
	StateFailedPerm            OpState = "FAILED_PERM"  // permanent denial, max attempts, etc.
)

// Sentinel errors surfaced by the queue. Wrapped errors include the
// underlying cause so callers can log a stable code plus a non-secret detail.
var (
	ErrQueueEmpty        = errors.New("offlinequeue: queue is empty")
	ErrOperationConflict = errors.New("offlinequeue: duplicate idempotency key for an existing pending or in-flight operation")
	ErrPayloadChanged    = errors.New("offlinequeue: idempotency key reused with a different payload; explicit confirmation requires a new key")
	ErrNoSuchOperation   = errors.New("offlinequeue: no operation with that id")
	ErrTerminalState     = errors.New("offlinequeue: operation is in a terminal state and cannot transition")
	ErrSelectionExpired  = errors.New("offlinequeue: selection window expired; submit a new operation with a fresh confirmation")
)

// PendingOperation is a durable record of a single unconfirmed operation
// accumulated offline. Once written, the IdempotencyKey, Endpoint, Method,
// TokenRef and Payload are immutable — updating an entry only changes its
// State, RetryCount, LastError, LastStatusCode and CommittedAt.
//
// Bearer tokens are NEVER stored in this struct. TokenRef is an opaque label
// the caller resolves through TokenProvider at dispatch time; the token bytes
// never touch disk. (See plan/p5-contract.md §1.2 invariant 1: public/private
// separation.)
type PendingOperation struct {
	ID             string          `json:"id"`
	CreatedAt      time.Time       `json:"created_at"`
	IdempotencyKey string          `json:"idempotency_key"`
	Endpoint       string          `json:"endpoint"` // e.g. "/api/v3/reservations"
	Method         string          `json:"method"`   // "POST" for reservation.create/stay events
	TokenRef       string          `json:"token_ref"`
	Payload        json.RawMessage `json:"payload"`
	// PayloadHash is sha256 over (IdempotencyKey + "\n" + Payload). It binds
	// the operation to one exact intended submission. The store refuses a
	// re-enqueue with the same key but a different hash (ErrPayloadChanged):
	// changed intent requires a fresh explicit confirmation and a new key.
	PayloadHash string `json:"payload_hash"`
	// SnapshotVersion is the source/package snapshot the client validated
	// against. The server revalidates at commit; drift returns STALE_VERSION.
	// Zero is allowed only for endpoints that do not require a snapshot (none
	// in the P4 reservation/stay contract — kept as an int for forward
	// compatibility with future endpoints).
	SnapshotVersion int `json:"snapshot_version"`
	// SelectionExpiry is the server-asserted window for which the client's
	// confirmed selection remains valid offline. After this time the
	// selection is treated as stale (FAILED_STALE) without contacting the
	// server, exactly mirroring the offline-too-long guidance in
	// plan/architecture.md ("Offline and consistency").
	SelectionExpiry time.Time `json:"selection_expiry"`
	State           OpState   `json:"state"`
	RetryCount      int       `json:"retry_count"`
	LastError       string    `json:"last_error,omitempty"`
	LastStatusCode  int       `json:"last_status_code,omitempty"`
	CommittedAt     time.Time `json:"committed_at,omitempty"`
	// ServerResult is the raw response body the worker captured the LAST time
	// the server returned 2xx (either first success or replay). It is the
	// client-side memo of what the server committed; replay reads do not
	// re-fetch from the server. Set only on a 2xx response.
	ServerResult json.RawMessage `json:"server_result,omitempty"`
}

// hashPayload returns the canonical payload hash binding key + payload.
func hashPayload(idempotencyKey string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(idempotencyKey))
	h.Write([]byte{'\n'})
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// Validate returns nil if op is well-formed enough to enqueue. It does not
// check payload contents (the server does that at commit); it enforces only
// the queue-level invariants: a stable id, non-empty key and endpoint, a
// sensible method, a token ref, a parseable payload and a future
// SelectionExpiry. SnapshotVersion is not enforced here.
func (op *PendingOperation) Validate(now time.Time) error {
	if op == nil {
		return errors.New("offlinequeue: nil operation")
	}
	if strings.TrimSpace(op.ID) == "" {
		return errors.New("offlinequeue: operation id is required")
	}
	if strings.TrimSpace(op.IdempotencyKey) == "" {
		return errors.New("offlinequeue: idempotency_key is required")
	}
	if !strings.HasPrefix(op.Endpoint, "/") {
		return fmt.Errorf("offlinequeue: endpoint %q must be an absolute path", op.Endpoint)
	}
	switch op.Method {
	case "POST":
	default:
		return fmt.Errorf("offlinequeue: only POST is supported (got %q)", op.Method)
	}
	if strings.TrimSpace(op.TokenRef) == "" {
		return errors.New("offlinequeue: token_ref is required (the token itself must never be persisted)")
	}
	if len(op.Payload) == 0 || !json.Valid(op.Payload) {
		return errors.New("offlinequeue: payload is required and must be valid JSON")
	}
	if op.SelectionExpiry.IsZero() {
		return errors.New("offlinequeue: selection_expiry is required so the worker can fail closed when the offline window is too long")
	}
	if !op.SelectionExpiry.After(now) {
		return fmt.Errorf("offlinequeue: selection_expiry %s is already in the past (now=%s)", op.SelectionExpiry.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339))
	}
	if op.State == "" {
		op.State = StatePending
	}
	if op.PayloadHash == "" {
		op.PayloadHash = hashPayload(op.IdempotencyKey, op.Payload)
	}
	return nil
}

// SetCommitted is the single transition to a successful terminal state. It
// records the captured server response (typically the reservation/stay id map)
// and sets CommittedAt. Subsequent UpdateState calls reject terminal entries.
// Allowed from PENDING (first success), IN_FLIGHT (current Submit confirms
// server) and PENDING_RECONCILIATION (idempotent replay confirms a previously
// dropped response).
func (op *PendingOperation) SetCommitted(now time.Time, statusCode int, serverResult json.RawMessage) error {
	switch op.State {
	case StatePending, StateInFlight, StatePendingReconciliation:
	default:
		return ErrTerminalState
	}
	op.State = StateCommitted
	op.CommittedAt = now.UTC()
	op.LastStatusCode = statusCode
	op.LastError = ""
	if len(serverResult) > 0 {
		op.ServerResult = append(op.ServerResult[:0], serverResult...)
	}
	return nil
}

// SetFailedStale marks the operation failed because the server's snapshot or
// selection window drifted (or the worker detected offline-too-long). It is
// terminal: retrying would replay against a different server state. The user
// must confirm a new selection with a new idempotency key.
//
// Allowed from PENDING, IN_FLIGHT and PENDING_RECONCILIATION. Reconciliation
// entries that the server finally confirms as stale (snapshot/hold drift
// caught on replay) transition here without raising a "spurious" duplicate
// commit on the client.
func (op *PendingOperation) SetFailedStale(now time.Time, statusCode int, errMsg string) error {
	switch op.State {
	case StatePending, StateInFlight, StatePendingReconciliation:
	default:
		return ErrTerminalState
	}
	op.State = StateFailedStale
	op.CommittedAt = now.UTC()
	op.LastStatusCode = statusCode
	op.LastError = errMsg
	return nil
}

// SetFailedPerm marks the operation failed permanently (denial, conflict,
// max-attempts-exceeded, etc.). The user must inspect the response, then
// either retry with a new key or correct the request.
//
// Allowed from PENDING, IN_FLIGHT and PENDING_RECONCILIATION. Reconciliation
// entries that the server finally confirms as a permanent rejection (e.g.
// IDEMPOTENCY_CONFLICT for a payload-allowed forgery) transition here.
func (op *PendingOperation) SetFailedPerm(now time.Time, statusCode int, errMsg string) error {
	switch op.State {
	case StatePending, StateInFlight, StatePendingReconciliation:
	default:
		return ErrTerminalState
	}
	op.State = StateFailedPerm
	op.CommittedAt = now.UTC()
	op.LastStatusCode = statusCode
	op.LastError = errMsg
	return nil
}

// SetPendingReconciliation marks the operation as "we dispatched it but
// cannot prove the server's verdict": the request reached the network, the
// transport responded with an error (timeout, dropped connection, reset,
// DNS failure), so we cannot tell whether the server committed the hold or
// rejected it. The queue keeps the original key and payload so the next
// drain can replay the submission with identical intent.
//
// This is NOT a terminal state. It is reversible via SetInFlight (next drain
// retries), or it transitions to a terminal state via SetCommitted,
// SetFailedStale or SetFailedPerm once the server answers on replay.
//
// The caller is responsible for bumping RetryCount via BumpRetry first; this
// method only flips State and records the diagnostic.
func (op *PendingOperation) SetPendingReconciliation(now time.Time, statusCode int, errMsg string) error {
	switch op.State {
	case StatePending, StateInFlight, StatePendingReconciliation:
	default:
		return ErrTerminalState
	}
	op.State = StatePendingReconciliation
	op.LastStatusCode = statusCode
	if errMsg != "" {
		op.LastError = errMsg
	}
	return nil
}

// SetInFlight marks the operation dispatched but not yet acknowledged. It is
// reversible: a worker crash returns the operation to PENDING_RECONCILIATION
// on restart (because the worker cannot prove whether the server committed).
//
// Allowed from PENDING (first dispatch) and PENDING_RECONCILIATION (replay
// after a previous transport failure).
func (op *PendingOperation) SetInFlight() error {
	if op.State != StatePending && op.State != StatePendingReconciliation {
		return fmt.Errorf("offlinequeue: cannot move to IN_FLIGHT from %s", op.State)
	}
	op.State = StateInFlight
	return nil
}

// BumpRetry increments the retry counter and records the transient failure
// without leaving IN_FLIGHT. Caller decides IN_FLIGHT -> PENDING vs terminal.
func (op *PendingOperation) BumpRetry(statusCode int, errMsg string) {
	op.RetryCount++
	op.LastStatusCode = statusCode
	op.LastError = errMsg
}

// IsTerminal reports whether the operation is in a state that should not be
// retried automatically. Stale failures must be re-confirmed by the user;
// permanent failures require inspection. Committed entries can be purged.
//
// PENDING_RECONCILIATION is NOT terminal: the worker (or a future drain)
// must still replay it to learn the server's verdict. Only the server
// (via the next dispatch result) can move a reconciliation entry to a
// terminal state.
func (op *PendingOperation) IsTerminal() bool {
	switch op.State {
	case StateCommitted, StateFailedStale, StateFailedPerm:
		return true
	default:
		return false
	}
}

// SelectionStillValid reports whether the operation's confirmed selection
// remains within the server-asserted validity window. A false answer means the
// queue must fail the operation closed without contacting the server (or, if
// the server is reachable, expect a STALE_VERSION/EXPIRED response).
func (op *PendingOperation) SelectionStillValid(now time.Time) bool {
	if op.SelectionExpiry.IsZero() {
		return false
	}
	return now.UTC().Before(op.SelectionExpiry.UTC())
}
