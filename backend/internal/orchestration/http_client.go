package orchestration

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

	"sthira/backend/internal/contracts"
)

// HTTPWorkerClient is an HTTP-backed implementation of WorkerClient.
// It sends typed requests to private worker loopback endpoints with
// bearer-token authentication.
type HTTPWorkerClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewHTTPWorkerClient builds an HTTPWorkerClient for the given baseURL and bearer token.
func NewHTTPWorkerClient(baseURL, token string, client *http.Client) *HTTPWorkerClient {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPWorkerClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: client,
	}
}

// Health probes the worker's GET /health endpoint.
func (c *HTTPWorkerClient) Health(ctx context.Context) (contracts.WorkerHealth, bool) {
	if c == nil || c.baseURL == "" {
		return contracts.WorkerHealth{}, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return contracts.WorkerHealth{}, false
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return contracts.WorkerHealth{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return contracts.WorkerHealth{}, false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return contracts.WorkerHealth{}, false
	}
	if err := checkNoDuplicateKeys(raw); err != nil {
		return contracts.WorkerHealth{}, false
	}
	var health contracts.WorkerHealth
	if err := json.Unmarshal(raw, &health); err != nil {
		return contracts.WorkerHealth{}, false
	}
	return health, true
}

// Transcribe sends an ASR request to POST /transcribe.
func (c *HTTPWorkerClient) Transcribe(ctx context.Context, req contracts.ASRWorkerRequest) (contracts.ASRWorkerResponse, error) {
	if c == nil || c.baseURL == "" {
		return contracts.ASRWorkerResponse{State: contracts.TranscriptionUnavailable}, ErrModelUnavailable
	}
	var out contracts.ASRWorkerResponse
	err := c.postJSON(ctx, "/transcribe", req, &out, 512*1024)
	if err != nil {
		return contracts.ASRWorkerResponse{State: contracts.TranscriptionUnavailable}, err
	}
	if req.RequestID != "" && out.RequestID != "" && out.RequestID != req.RequestID {
		return contracts.ASRWorkerResponse{State: contracts.TranscriptionUnavailable}, fmt.Errorf("worker response request_id %q does not match request %q", out.RequestID, req.RequestID)
	}
	return out, nil
}

// Propose sends a middle-model request to POST /v1/chat/completions.
func (c *HTTPWorkerClient) Propose(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
	if c == nil || c.baseURL == "" {
		return contracts.MiddleWorkerResponse{}, ErrModelUnavailable
	}
	var out contracts.MiddleWorkerResponse
	err := c.postJSON(ctx, "/v1/chat/completions", req, &out, 512*1024)
	if err != nil {
		return contracts.MiddleWorkerResponse{}, err
	}
	if req.RequestID != "" && out.RequestID != "" && out.RequestID != req.RequestID {
		return contracts.MiddleWorkerResponse{}, fmt.Errorf("worker response request_id %q does not match request %q", out.RequestID, req.RequestID)
	}
	return out, nil
}

// Synthesize sends a TTS request to POST /synthesize.
func (c *HTTPWorkerClient) Synthesize(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
	if c == nil || c.baseURL == "" {
		return contracts.TTSWorkerResponse{State: contracts.TTSUnavailable}, ErrModelUnavailable
	}
	var out contracts.TTSWorkerResponse
	err := c.postJSON(ctx, "/synthesize", req, &out, 2*1024*1024)
	if err != nil {
		return contracts.TTSWorkerResponse{State: contracts.TTSUnavailable}, err
	}
	if req.RequestID != "" && out.RequestID != "" && out.RequestID != req.RequestID {
		return contracts.TTSWorkerResponse{State: contracts.TTSUnavailable}, fmt.Errorf("worker response request_id %q does not match request %q", out.RequestID, req.RequestID)
	}
	return out, nil
}

func (c *HTTPWorkerClient) postJSON(ctx context.Context, path string, in any, out any, maxBytes int64) error {
	data, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return ErrPipelineCanceled
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrModelTimeout
		}
		return fmt.Errorf("worker request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return ErrModelUnavailable
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrQueueSaturated
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("worker returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	// max+1 detection: read up to (maxBytes + 1) so an exactly-at-cap
	// response passes, but a response that exceeds the cap by even
	// one byte is detected BEFORE the truncated envelope is decoded.
	limited := io.LimitReader(resp.Body, maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("read worker response: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		// Try to drain the rest so the connection can be reused,
		// but the response is rejected regardless.
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("worker response exceeds %d bytes (oversize)", maxBytes)
	}
	if err := checkNoDuplicateKeys(raw); err != nil {
		return fmt.Errorf("validate response JSON: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode worker response: %w", err)
	}
	return nil
}

func checkNoDuplicateKeys(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errors.New("empty JSON data")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	if err := walkJSONTokens(dec, 0, 32); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return errors.New("trailing data after top-level JSON value")
	}
	return nil
}

func walkJSONTokens(dec *json.Decoder, depth, maxDepth int) error {
	if maxDepth > 0 && depth > maxDepth {
		return fmt.Errorf("depth %d exceeds maximum depth %d", depth, maxDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("truncated JSON")
		}
		return err
	}
	switch delim := tok.(type) {
	case json.Delim:
		switch delim {
		case '{':
			seen := make(map[string]struct{})
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := keyTok.(string)
				if !ok {
					return fmt.Errorf("expected string key in JSON object, got %T", keyTok)
				}
				if _, exists := seen[key]; exists {
					return fmt.Errorf("duplicate key in JSON object: %q", key)
				}
				seen[key] = struct{}{}
				if err := walkJSONTokens(dec, depth+1, maxDepth); err != nil {
					return err
				}
			}
			closing, err := dec.Token()
			if err != nil {
				return err
			}
			if cDelim, ok := closing.(json.Delim); !ok || cDelim != '}' {
				return fmt.Errorf("expected '}', got %v", closing)
			}
		case '[':
			for dec.More() {
				if err := walkJSONTokens(dec, depth+1, maxDepth); err != nil {
					return err
				}
			}
			closing, err := dec.Token()
			if err != nil {
				return err
			}
			if cDelim, ok := closing.(json.Delim); !ok || cDelim != ']' {
				return fmt.Errorf("expected ']', got %v", closing)
			}
		default:
			return fmt.Errorf("unexpected delimiter %v", delim)
		}
	}
	return nil
}
