package offlinequeue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPDispatcher is the seam through which the worker submits a queued
// operation. It is the integration point a real mobile client replaces with
// its own HTTP client; the harness ships a stdlib-net/http implementation.
//
// PostJSON takes the resolved bearer token bytes (NOT a TokenRef; the caller
// resolves the ref first so the token never enters the queue). It returns
// the HTTP status code, the response body bytes (always the raw /api/v3
// envelope bytes, including 4xx/5xx envelopes) and a transport-level error
// (network failure, context cancellation, etc.).
type HTTPDispatcher interface {
	PostJSON(ctx context.Context, method, path, bearerToken string, body []byte) (statusCode int, respBody []byte, err error)
}

// TokenProvider resolves a TokenRef label to the actual bearer token bytes
// for dispatch. The harness ships an in-memory map; a real mobile client
// would back this with the OS secure storage (Keychain on iOS, EncryptedSharedPreferences
// on Android). Token material MUST NOT be logged, persisted, or copied.
type TokenProvider interface {
	BearerTokenFor(ctx context.Context, ref string) (string, error)
}

// MemoryTokenStore is the in-memory TokenProvider used by the reference
// harness and tests. It is not persisted to disk; the worker dies, the
// tokens vanish. The store is keyed by the TokenRef the operation carries,
// not by session id, so tests can carry multiple tokens per ref without
// colliding with the production wiring.
//
// MemoryTokenStore is NOT safe for cross-process sharing and is NOT a
// substitute for OS secure storage.
type MemoryTokenStore struct {
	mu     sync.Mutex
	tokens map[string]string
}

// NewMemoryTokenStore creates an empty in-memory token store.
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{tokens: map[string]string{}}
}

// Put associates a token with a ref. The token is stored by reference; the
// store does NOT defensively copy. Callers SHOULD pass a fresh value.
func (m *MemoryTokenStore) Put(ref, token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[ref] = token
}

// Forget removes the token associated with ref, if any. Use when the
// session is revoked.
func (m *MemoryTokenStore) Forget(ref string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, ref)
}

// BearerTokenFor returns the token for ref or ErrUnknownTokenRef if absent.
func (m *MemoryTokenStore) BearerTokenFor(ctx context.Context, ref string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tok, ok := m.tokens[ref]
	if !ok {
		return "", ErrUnknownTokenRef
	}
	return tok, nil
}

// ErrUnknownTokenRef is returned when no token is bound to the requested ref.
var ErrUnknownTokenRef = errors.New("offlinequeue: no token for the given TokenRef")

// HTTPClientDispatcher is the stdlib-based HTTPDispatcher used by the
// reference harness and integration tests. It accepts an absolute base URL
// (the wire endpoint) and a custom *http.Client (so tests can inject timeouts
// or transports). It does not parse the response envelope; the worker reads
// the raw status code and body bytes and decides what to do.
//
// The dispatcher refuses non-absolute paths and any path that contains ".."
// or a scheme; the queue contract is "endpoint as path under the configured
// base URL", and accepting a scheme-bearing path would silently turn the
// queue into an SSRF gadget.
type HTTPClientDispatcher struct {
	BaseURL string
	Client  *http.Client
}

// NewHTTPClientDispatcher constructs a dispatcher with sensible defaults.
func NewHTTPClientDispatcher(baseURL string, client *http.Client) *HTTPClientDispatcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPClientDispatcher{BaseURL: baseURL, Client: client}
}

// PostJSON validates the endpoint, then issues the request. ctx cancels the
// in-flight call (the http.Client honors cancellation through its transport).
func (d *HTTPClientDispatcher) PostJSON(ctx context.Context, method, path, bearerToken string, body []byte) (int, []byte, error) {
	if d.BaseURL == "" {
		return 0, nil, errors.New("offlinequeue: dispatcher BaseURL is required")
	}
	if err := validateEndpoint(path); err != nil {
		return 0, nil, err
	}
	u, err := url.Parse(d.BaseURL)
	if err != nil {
		return 0, nil, fmt.Errorf("offlinequeue: parse base url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}

// validateEndpoint guards against SSRF and protocol confusion by rejecting
// anything that is not an absolute path under a /api/ prefix.
func validateEndpoint(p string) error {
	if p == "" {
		return errors.New("offlinequeue: endpoint is empty")
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("offlinequeue: endpoint %q must start with /", p)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("offlinequeue: endpoint %q must not contain '..'", p)
	}
	if strings.Contains(p, "://") {
		return fmt.Errorf("offlinequeue: endpoint %q must not contain a scheme", p)
	}
	if !strings.HasPrefix(p, "/api/") {
		return fmt.Errorf("offlinequeue: endpoint %q must be under /api/", p)
	}
	return nil
}

// errorEnvelope mirrors the /api/v3 envelope shape so the worker can extract
// the stable error code without importing the contracts package (the queue
// stays decoupled from the server package per plan/p5-contract.md §2.2).
// It is a struct-of-strings; no behavior beyond extraction.
type errorEnvelope struct {
	Errors []struct {
		Code string `json:"code"`
	} `json:"errors"`
}

// extractServerErrorCode returns the first stable error code in a 4xx/5xx
// envelope, or "" if the body is not a recognisable envelope.
func extractServerErrorCode(body []byte) string {
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}
	for _, e := range env.Errors {
		if e.Code != "" {
			return e.Code
		}
	}
	return ""
}
