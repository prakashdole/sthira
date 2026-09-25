// HTTPClientRuntime — production-shaped call through Client to the
// pinned private vLLM endpoint. The runtime owns the pinned model_id,
// revision, digest, language list and system prompt; the Client owns
// transport and structured-output decoding.

package middleworker

import (
	"context"
	"encoding/json"
	"errors"
)

// HTTPClientRuntime is the production-shaped runtime. It composes a
// Client (transport) with a model identifier (artifact). It does NOT
// load the model or perform real inference; the actual weights live
// in the private vLLM process the Client calls.
type HTTPClientRuntime struct {
	client     *Client
	modelID    string
	revision   string
	digestName string
	digestSHA  string
	langs      []string
	system     string
	chatKwargs map[string]any // Jinja vars for the server-loaded template (e.g. enable_thinking=false)
}

// HTTPClientRuntimeConfig bundles construction. The model_id is the
// Sarvam-30B model identifier; revision and digest are
// recorded for /health. system is the pinned voice-map-system-prompt
// (see plan/voice-map-system-prompt.md).
type HTTPClientRuntimeConfig struct {
	Client     *Client
	ModelID    string
	Revision   string
	DigestName string
	DigestSHA  string
	Languages  []string
	System     string
	// ChatTemplateKwargs carries Jinja variables (e.g.
	// {"enable_thinking": false}) to the template the SERVER
	// loaded from the model repo. It is not a template override.
	ChatTemplateKwargs map[string]any
}

// NewHTTPClientRuntime validates the config and returns a runtime.
// An empty revision is allowed at construction; LoadAndVerify is the
// gate that warms the worker.
func NewHTTPClientRuntime(cfg HTTPClientRuntimeConfig) (*HTTPClientRuntime, error) {
	if cfg.Client == nil {
		return nil, errors.New("http client runtime: Client required")
	}
	if cfg.ModelID == "" {
		return nil, errors.New("http client runtime: ModelID required")
	}
	if cfg.System == "" {
		return nil, errors.New("http client runtime: System prompt required")
	}
	return &HTTPClientRuntime{
		client:     cfg.Client,
		modelID:    cfg.ModelID,
		revision:   cfg.Revision,
		digestName: cfg.DigestName,
		digestSHA:  cfg.DigestSHA,
		langs:      append([]string(nil), cfg.Languages...),
		system:     cfg.System,
		chatKwargs: cfg.ChatTemplateKwargs,
	}, nil
}

// Propose marshals the typed request envelope, hands it to Client,
// and surfaces typed errors. Cancellation is honored at every
// boundary. The Runtime is the seam where the wire envelope is
// constructed; the Client is the seam where the wire envelope is
// sent and decoded.
func (h *HTTPClientRuntime) Propose(ctx context.Context, req RequestEnvelope) (*ProposeOutput, error) {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, ErrCanceled
		}
		return nil, ErrTimeout
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return h.client.Propose(ctx, ProposeInput{
		ModelID:            h.modelID,
		RequestID:          req.RequestID,
		SystemPrompt:       h.system,
		UserPayload:        payload,
		ChatTemplateKwargs: h.chatKwargs,
		MaxOutputTokens:    req.MaxOutputTokens,
	})
}

// Revision returns the configured revision.
func (h *HTTPClientRuntime) Revision() string { return h.revision }

// Digest returns the configured artifact name and SHA-256.
func (h *HTTPClientRuntime) Digest() (string, string) { return h.digestName, h.digestSHA }

// Languages returns the configured language list.
func (h *HTTPClientRuntime) Languages() []string { return append([]string(nil), h.langs...) }

// System returns the pinned system prompt.
func (h *HTTPClientRuntime) System() string { return h.system }

// ModelID returns the configured model identifier.
func (h *HTTPClientRuntime) ModelID() string { return h.modelID }
