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
	var health contracts.WorkerHealth
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&health); err != nil {
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBytes)).Decode(out); err != nil {
		return fmt.Errorf("decode worker response: %w", err)
	}
	return nil
}
