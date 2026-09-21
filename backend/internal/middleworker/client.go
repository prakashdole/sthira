// Bounded HTTP client to a private vLLM endpoint. Strict per-request
// context/output caps, per-call deadline, no retries, no paid fallback,
// fail-closed on oversized/malformed/extra-text responses.

package middleworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Errors surfaced by the Client and the schema decoder. All of them
// are typed so the worker can map them to MiddleState values without
// resorting to string matching.
var (
	ErrUnavailable       = errors.New("middleworker: model unavailable")
	ErrTimeout           = errors.New("middleworker: inference deadline exceeded")
	ErrCanceled          = errors.New("middleworker: inference canceled")
	ErrMalformed         = errors.New("middleworker: response malformed")
	ErrExtraText         = errors.New("middleworker: response contained extra text outside the structured payload")
	ErrSchemaUnsupported = errors.New("middleworker: structured-output schema not supported by the runtime")
	ErrContextExceeded   = errors.New("middleworker: context length exceeded the per-request cap")
	ErrOutputExceeded    = errors.New("middleworker: output exceeded the per-request cap")
	ErrOversized         = errors.New("middleworker: response body exceeded the configured byte cap")
)

// Limits is the bounded parameter set the Client enforces. Any field
// at zero gets DefaultLimits. The orchestrator may pass stricter
// values per call.
type Limits struct {
	// MaxContextTokens is the hard ceiling on input tokens. The
	// worker uses an approximate UTF-8 bytes/4 heuristic for the
	// per-request prompt; the runtime is responsible for the
	// authoritative tokenization. Exceeding the ceiling returns
	// ErrContextExceeded and the runtime is never called.
	MaxContextTokens int
	// MaxOutputTokens is the per-request max_tokens sent to vLLM
	// and the ceiling the Client enforces on the response.
	MaxOutputTokens int
	// MaxResponseBytes caps the raw HTTP response body. A body
	// larger than this fails fast with ErrOversized. Set generously
	// above MaxOutputTokens*8 to allow for JSON wrapping and UTF-8
	// slack; the JSON-schema decode still bounds what lands in the
	// proposal.
	MaxResponseBytes int64
	// MaxRequestBytes caps the outgoing request body.
	MaxRequestBytes int64
	// ConnectTimeout is the per-connection TCP/TLS timeout.
	ConnectTimeout time.Duration
	// PerCallTimeout is the per-request total wall-clock budget.
	// Independent of ConnectTimeout so a slow first byte cannot
	// exhaust the connect budget.
	PerCallTimeout time.Duration
}

// DefaultLimits are conservative defaults. Tuned for a 4B model on a
// 24 GB GPU: ~2 K tokens context, 256 tokens output, 64 KiB response
// body, 64 KiB request body, 2 s connect, 6 s per-call. These are
// BOUNDS, not measurements.
var DefaultLimits = Limits{
	MaxContextTokens: 4096,
	MaxOutputTokens:  256,
	MaxResponseBytes: 64 * 1024,
	MaxRequestBytes:  64 * 1024,
	ConnectTimeout:   2 * time.Second,
	PerCallTimeout:   6 * time.Second,
}

// Client speaks the private vLLM HTTP API. The endpoint URL is the
// orchestrator-supplied base (e.g. http://127.0.0.1:8000). Auth is
// not implemented; private network only.
type Client struct {
	baseURL string
	limits  Limits
	http    *http.Client
	// schemaBytes is the pinned JSON Schema the worker hands to
	// vLLM as the guided-generation grammar. Pre-loaded once.
	schemaBytes []byte
}

// ClientConfig bundles construction.
type ClientConfig struct {
	// BaseURL is the vLLM base URL (no trailing slash).
	BaseURL string
	// Limits is the bound set. Zero values → DefaultLimits.
	Limits Limits
	// SchemaJSON is the pinned JSON Schema for guided generation.
	// The Client does not validate it — that is the worker's job
	// at LoadAndVerify — but it is included verbatim in every
	// request so vLLM enforces the schema at decode time.
	SchemaJSON []byte
}

// NewClient builds a Client. baseURL must be non-empty.
func NewClient(cfg ClientConfig) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("middleworker: client requires baseURL")
	}
	lim := cfg.Limits
	if lim.MaxContextTokens <= 0 {
		lim.MaxContextTokens = DefaultLimits.MaxContextTokens
	}
	if lim.MaxOutputTokens <= 0 {
		lim.MaxOutputTokens = DefaultLimits.MaxOutputTokens
	}
	if lim.MaxResponseBytes <= 0 {
		lim.MaxResponseBytes = DefaultLimits.MaxResponseBytes
	}
	if lim.MaxRequestBytes <= 0 {
		lim.MaxRequestBytes = DefaultLimits.MaxRequestBytes
	}
	if lim.ConnectTimeout <= 0 {
		lim.ConnectTimeout = DefaultLimits.ConnectTimeout
	}
	if lim.PerCallTimeout <= 0 {
		lim.PerCallTimeout = DefaultLimits.PerCallTimeout
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		limits:  lim,
		http: &http.Client{
			Timeout: lim.PerCallTimeout,
			// No Transport tuning: a private loopback needs no
			// connection pool, no keep-alive, no proxy.
		},
		schemaBytes: cfg.SchemaJSON,
	}, nil
}

// chatCompletionRequest is the wire shape of vLLM's
// /v1/chat/completions. The Client includes the schema via
// response_format.json_schema so vLLM runs guided generation.
// NOTE: vLLM's structured-output wire format is documented at
// https://docs.vllm.ai/en/latest/features/structured_outputs/. This
// client pins the documented shape; a vLLM version that drifts from
// it must be re-verified before deployment (see Eval).
type chatCompletionRequest struct {
	Model string `json:"model"`
	// Messages: system + user. The user message contains the typed
	// envelope (request_id, scoped_context, transcript). The system
	// message is the pinned voice-map-system-prompt.
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	// ChatTemplate is an optional per-request template override. It
	// is sent to vLLM only when non-empty, so the configured server
	// startup --chat-template wins by default; the per-request
	// override is for cases where reasoning controls (e.g. the
	// Sarvam-30B enable_thinking=false gate) must be applied at
	// request time rather than server-startup time.
	ChatTemplate string `json:"chat_template,omitempty"`
	// ResponseFormat is vLLM's guided-generation envelope. The
	// schema name is informational; strict=true forces vLLM to
	// reject non-conforming outputs server-side.
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	// Stream must be false; the Client does not support streaming.
	Stream bool `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

// chatCompletionResponse is the wire shape of vLLM's response. Only
// the fields the worker needs are decoded.
type chatCompletionResponse struct {
	Choices []chatChoice `json:"choices"`
	// Usage includes prompt_tokens and completion_tokens when
	// vLLM reports them. The worker reports them as structured
	// output logs and uses them to enforce ErrContextExceeded.
	Usage *chatUsage `json:"usage,omitempty"`
	// Model is the resolved model name (may include revision).
	Model string `json:"model,omitempty"`
}

type chatChoice struct {
	// FinishReason is one of "stop", "length", "content_filter",
	// or "tool_calls". "length" implies OUTPUT_EXCEEDED.
	FinishReason string `json:"finish_reason"`
	// Index is always 0 for a non-streamed response.
	Index int `json:"index"`
	// Message contains the assistant content.
	Message chatMessage `json:"message"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ProposeInput is what the worker hands to the Client. The Client is
// the only thing that talks HTTP to vLLM; everything else (queue,
// concurrency, language allow-list) is the Worker's job.
type ProposeInput struct {
	ModelID      string
	RequestID    string
	SystemPrompt string
	UserPayload  []byte // marshalled RequestEnvelope
	// ChatTemplate is the optional per-request template override
	// (e.g. SarvamChatTemplate with enable_thinking=false). Empty
	// means: rely on the server-side --chat-template.
	ChatTemplate string
}

// ProposeOutput is what the Client returns on success.
type ProposeOutput struct {
	Proposal         Proposal
	ModelRevision    string
	FinishReason     string
	PromptTokens     int
	CompletionTokens int
}

// Propose calls /v1/chat/completions on the pinned vLLM endpoint,
// once, with no retries. The context ctx provides cancellation; the
// per-call deadline is enforced by the http.Client.Timeout AND by an
// explicit pre-flight check.
func (c *Client) Propose(ctx context.Context, in ProposeInput) (*ProposeOutput, error) {
	if in.ModelID == "" {
		return nil, errors.New("middleworker: model_id required")
	}
	if in.RequestID == "" {
		return nil, errors.New("middleworker: request_id required")
	}
	if len(in.UserPayload) == 0 {
		return nil, errors.New("middleworker: user_payload required")
	}
	if int64(len(in.UserPayload)) > c.limits.MaxRequestBytes {
		return nil, fmt.Errorf("middleworker: request body %d > %d", len(in.UserPayload), c.limits.MaxRequestBytes)
	}
	// Cheap UTF-8 length heuristic for the prompt. Tokenization is
	// the runtime's job; this check just enforces the bound
	// before we go on the wire.
	approxContextTokens := (len(in.SystemPrompt) + len(in.UserPayload)) / 4
	if approxContextTokens > c.limits.MaxContextTokens {
		return nil, ErrContextExceeded
	}

	// Pre-flight deadline. The http.Client.Timeout also enforces
	// this; the explicit check catches a parent context that is
	// already past its deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.limits.PerCallTimeout)
		defer cancel()
	}

	body, err := json.Marshal(chatCompletionRequest{
		Model: in.ModelID,
		Messages: []chatMessage{
			{Role: "system", Content: in.SystemPrompt},
			{Role: "user", Content: string(in.UserPayload)},
		},
		MaxTokens:   c.limits.MaxOutputTokens,
		Temperature: 0.0,
		ChatTemplate: in.ChatTemplate,
		ResponseFormat: &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchema{
				Name:   "model_output",
				Strict: true,
				Schema: c.schemaBytes,
			},
		},
		Stream: false,
	})
	if err != nil {
		return nil, fmt.Errorf("middleworker: marshal request: %w", err)
	}
	if int64(len(body)) > c.limits.MaxRequestBytes {
		return nil, fmt.Errorf("middleworker: marshalled body %d > %d", len(body), c.limits.MaxRequestBytes)
	}

	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("middleworker: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Sthira-Request-ID", in.RequestID)

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, ErrCanceled
			}
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusRequestTimeout:
		return nil, ErrTimeout
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode == http.StatusUnprocessableEntity:
		// vLLM rejects schema that it cannot enforce. Surface a
		// typed error so the worker reports SCHEMA_UNSUPPORTED.
		return nil, fmt.Errorf("%w: status 422: %s", ErrSchemaUnsupported, readBounded(resp.Body, 4096))
	case resp.StatusCode == http.StatusBadRequest:
		// Likely a context-length rejection from vLLM itself.
		bs := readBounded(resp.Body, 4096)
		if bytes.Contains(bytes.ToLower(bs), []byte("context")) {
			return nil, ErrContextExceeded
		}
		return nil, fmt.Errorf("%w: status 400: %s", ErrMalformed, bs)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	// Bounded read. Any overage is ErrOversized so a misbehaving
	// server cannot pin the goroutine.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.limits.MaxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrUnavailable, err)
	}
	if int64(len(raw)) > c.limits.MaxResponseBytes {
		return nil, ErrOversized
	}

	var wire chatCompletionResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("%w: decode wire: %v", ErrMalformed, err)
	}
	if len(wire.Choices) == 0 {
		return nil, fmt.Errorf("%w: no choices", ErrMalformed)
	}
	ch := wire.Choices[0]
	if ch.FinishReason == "length" {
		// The runtime ran out of tokens before stopping. Surface
		// OUTPUT_EXCEEDED so the worker can map it explicitly.
		return nil, ErrOutputExceeded
	}
	if ch.Message.Role != "assistant" {
		return nil, fmt.Errorf("%w: unexpected role %q", ErrMalformed, ch.Message.Role)
	}

	// The assistant content is the JSON-encoded Proposal. vLLM in
	// guided-generation mode emits a single JSON document with NO
	// extra text. We re-check that the body has no prose around it
	// and decode strictly.
	proposal, err := decodeStrictProposal([]byte(ch.Message.Content))
	if err != nil {
		return nil, err
	}
	// Sanity: the runtime must echo the request_id we sent. If it
	// does not, the response is misrouted or the schema was not
	// enforced. Fail closed.
	if proposal.RequestID != in.RequestID {
		return nil, fmt.Errorf("%w: request_id mismatch (got %q, want %q)", ErrMalformed, proposal.RequestID, in.RequestID)
	}

	out := &ProposeOutput{
		Proposal:      proposal,
		ModelRevision: wire.Model,
		FinishReason:  ch.FinishReason,
	}
	if wire.Usage != nil {
		out.PromptTokens = wire.Usage.PromptTokens
		out.CompletionTokens = wire.Usage.CompletionTokens
		if wire.Usage.PromptTokens > c.limits.MaxContextTokens {
			return nil, ErrContextExceeded
		}
		if wire.Usage.CompletionTokens > c.limits.MaxOutputTokens {
			return nil, ErrOutputExceeded
		}
	}
	return out, nil
}

// readBounded reads up to n bytes from r. Used for surfacing short
// error bodies.
func readBounded(r io.Reader, n int64) []byte {
	bs, _ := io.ReadAll(io.LimitReader(r, n))
	return bs
}
