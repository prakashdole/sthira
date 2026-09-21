// StubRuntime — deterministic runtime for tests. Reads a JSON fixture
// file and returns it as a successful proposal. Never reports Ready
// in production.

package middleworker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
)

// StubRuntime serves a single pre-baked proposal. The fixture path
// must be a valid JSON file containing a Proposal. The runtime does
// no validation; the worker (and the orchestrator's validator) are
// responsible for that.
type StubRuntime struct {
	fixturePath string
	revision    string
	digestName  string
	digestSHA   string
	langs       []string
}

// NewStubRuntime builds a runtime serving the given fixture file.
// The file is read once at construction; production code MUST NOT
// pass a mutable file here.
func NewStubRuntime(fixturePath string) (*StubRuntime, error) {
	if fixturePath == "" {
		return nil, errors.New("stub runtime: fixturePath required")
	}
	if _, err := os.Stat(fixturePath); err != nil {
		return nil, err
	}
	return &StubRuntime{
		fixturePath: fixturePath,
		revision:    "stub-revision-0",
		digestName:  "stub-fixture",
		digestSHA:   "0000000000000000000000000000000000000000000000000000000000000000",
		langs:       []string{"en-IN"},
	}, nil
}

// SetLanguages overrides the supported languages reported in /health.
// Used by tests.
func (s *StubRuntime) SetLanguages(langs []string) {
	s.langs = append([]string(nil), langs...)
}

// Propose reads the fixture and returns it as a successful proposal.
// ctx cancellation is honored by returning ErrCanceled (never
// partially decoded state).
func (s *StubRuntime) Propose(ctx context.Context, req RequestEnvelope) (*ProposeOutput, error) {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, ErrCanceled
		}
		return nil, ErrTimeout
	}
	bs, err := os.ReadFile(s.fixturePath)
	if err != nil {
		return nil, err
	}
	proposal, err := decodeStrictProposal(bs)
	if err != nil {
		return nil, err
	}
	if proposal.RequestID != req.RequestID {
		return nil, errRequestIDMismatch{proposalID: proposal.RequestID, sent: req.RequestID}
	}
	return &ProposeOutput{
		Proposal:      proposal,
		ModelRevision: s.revision,
		FinishReason:  "stop",
	}, nil
}

// Revision returns the stub's revision string.
func (s *StubRuntime) Revision() string { return s.revision }

// Digest returns the stub's digest name and SHA-256.
func (s *StubRuntime) Digest() (string, string) { return s.digestName, s.digestSHA }

// Languages returns the configured language list.
func (s *StubRuntime) Languages() []string { return append([]string(nil), s.langs...) }

// errRequestIDMismatch is the typed sentinel a stub returns when its
// fixture carries a request_id that does not match the call. The
// worker maps it to MiddleStateMalformed.
type errRequestIDMismatch struct {
	proposalID string
	sent       string
}

func (e errRequestIDMismatch) Error() string {
	return "stub fixture request_id " + e.proposalID + " != sent " + e.sent
}

// IsRequestIDMismatch reports whether err is the typed mismatch
// sentinel.
func IsRequestIDMismatch(err error) bool {
	var e errRequestIDMismatch
	return errors.As(err, &e)
}

// JSON returns the raw bytes of a Proposal. Used by tests for
// fixture generation.
func (p Proposal) JSON() ([]byte, error) { return json.Marshal(p) }
