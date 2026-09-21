// Package voiceload implements a Go-only voice-load harness. It drives
// the frozen P6 protocol shapes directly against the dummy workers
// (loadfixtures/dummyd) and records p50/p95/p99 latency, throughput,
// queue saturation, error categories and stale/orphan drops.
//
// It does NOT integrate with the W9 orchestration package. That is the
// integration stage's job. This harness is the protocol-shape validator
// and the orchestrator's pre-integration behavioural proxy.
//
// Throughput measured here is NOT GPU throughput. The dummy workers
// are bounded by ServiceTime, not by GPU compute.
package voiceload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerClients is the bundle the harness speaks to. Each role is a
// separate HTTP endpoint exposed by a dummyd worker.
type WorkerClients struct {
	ASR    string // base URL of the ASR dummy worker
	Middle string // base URL of the middle dummy worker
	TTS    string // base URL of the TTS dummy worker
}

// Config configures one voice-load run.
type Config struct {
	Workers WorkerClients
	// ArrivalRate is utterances/s.
	ArrivalRate int
	// Duration is the run length.
	Duration time.Duration
	// RenderTTSFraction is the probability that a request asks for TTS.
	// 0.0 = never; 1.0 = always.
	RenderTTSFraction float64
	// Language is sent in X-Language.
	Language string
	// Jurisdiction is part of the synthetic /voice/process request.
	Jurisdiction string
	// Logger receives structured timing lines.
	Logger *slog.Logger
	// HTTPClient is the client used for every request. Defaults to
	// http.DefaultClient with a 30s timeout.
	HTTPClient *http.Client
}

// Result is the post-run summary.
type Result struct {
	Total           uint64
	OK              uint64
	QueueSaturated  uint64
	ModelUnavailable uint64
	Other           uint64
	Cancelled       uint64
	p50             time.Duration
	p95             time.Duration
	p99             time.Duration
	Max             time.Duration
	Throughput      float64 // requests/s
	StartedAt       time.Time
	EndedAt         time.Time
}

// Run drives the voice workload for cfg.Duration and returns the summary.
func Run(ctx context.Context, cfg Config) Result {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.ArrivalRate <= 0 {
		cfg.ArrivalRate = 20
	}
	if cfg.Duration <= 0 {
		cfg.Duration = 30 * time.Second
	}
	if cfg.Jurisdiction == "" {
		cfg.Jurisdiction = "KL"
	}
	if cfg.Language == "" {
		cfg.Language = "en-IN"
	}

	var (
		ok       atomic.Uint64
		qs       atomic.Uint64
		mu       atomic.Uint64
		other    atomic.Uint64
		cancel   atomic.Uint64
		latencies = make([]time.Duration, 0, 4096)
		latMu     sync.Mutex
		total    atomic.Uint64
	)

	started := time.Now().UTC()
	deadline := started.Add(cfg.Duration)
	tickInterval := time.Second / time.Duration(cfg.ArrivalRate)
	if tickInterval < time.Millisecond {
		tickInterval = time.Millisecond
	}

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	wg := sync.WaitGroup{}
	for {
		select {
		case <-ctx.Done():
			cancel.Add(1)
			goto done
		case <-ticker.C:
			if time.Now().UTC().After(deadline) {
				goto done
			}
			total.Add(1)
			wg.Add(1)
			go func(idx uint64) {
				defer wg.Done()
				t := time.Now().UTC()
				includeTTS := float64(idx%uint64(cfg.ArrivalRate+1))/float64(cfg.ArrivalRate) < cfg.RenderTTSFraction
				code, _ := driveVoiceProcess(ctx, cfg, includeTTS)
				d := time.Since(t)
				switch code {
				case 200:
					ok.Add(1)
				case 503:
					qs.Add(1)
				case 504:
					mu.Add(1)
				default:
					other.Add(1)
				}
				latMu.Lock()
				if len(latencies) < cap(latencies) {
					latencies = append(latencies, d)
				}
				latMu.Unlock()
			}(total.Load())
		}
	}
done:
	wg.Wait()
	ended := time.Now().UTC()

	// Compute percentiles.
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	res := Result{
		Total:      total.Load(),
		OK:         ok.Load(),
		QueueSaturated: qs.Load(),
		ModelUnavailable: mu.Load(),
		Other:      other.Load(),
		Cancelled:  cancel.Load(),
		StartedAt:  started,
		EndedAt:    ended,
	}
	if len(latencies) > 0 {
		res.p50 = latencies[len(latencies)*50/100]
		res.p95 = latencies[len(latencies)*95/100]
		res.p99 = latencies[len(latencies)*99/100]
		res.Max = latencies[len(latencies)-1]
	}
	dur := ended.Sub(started).Seconds()
	if dur > 0 {
		res.Throughput = float64(total.Load()) / dur
	}
	return res
}

// driveVoiceProcess is a single /voice/process call. It speaks the frozen
// envelope shape against the dummyd workers in sequence (ASR -> middle ->
// TTS) so the harness measures the protocol round-trip cost end-to-end.
func driveVoiceProcess(ctx context.Context, cfg Config, includeTTS bool) (int, error) {
	// 1. ASR.
	asrBody := bytes.NewReader([]byte("AAAA")) // 4 bytes; bound enforced by server
	asrReq, err := http.NewRequestWithContext(ctx, "POST", cfg.Workers.ASR+"/transcribe", asrBody)
	if err != nil {
		return 0, err
	}
	asrReq.Header.Set("Content-Type", "audio/wav")
	asrReq.Header.Set("X-Language", cfg.Language)
	asrResp, err := cfg.HTTPClient.Do(asrReq)
	if err != nil {
		// Treat transport errors as cancellation.
		if errors.Is(err, context.Canceled) {
			return 499, err
		}
		return 599, err
	}
	_, _ = io.Copy(io.Discard, asrResp.Body)
	asrResp.Body.Close()
	if asrResp.StatusCode != 200 {
		return asrResp.StatusCode, fmt.Errorf("asr status %d", asrResp.StatusCode)
	}

	// 2. middle.
	midBody, _ := json.Marshal(map[string]any{
		"model":     "synthetic",
		"messages":  []map[string]any{{"role": "user", "content": "synthetic"}},
		"max_tokens": 64,
	})
	midReq, err := http.NewRequestWithContext(ctx, "POST", cfg.Workers.Middle+"/v1/chat/completions", bytes.NewReader(midBody))
	if err != nil {
		return 0, err
	}
	midReq.Header.Set("Content-Type", "application/json")
	midResp, err := cfg.HTTPClient.Do(midReq)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 499, err
		}
		return 599, err
	}
	_, _ = io.Copy(io.Discard, midResp.Body)
	midResp.Body.Close()
	if midResp.StatusCode != 200 {
		return midResp.StatusCode, fmt.Errorf("middle status %d", midResp.StatusCode)
	}

	// 3. TTS (optional).
	if includeTTS {
		ttsBody, _ := json.Marshal(map[string]any{
			"text":     "synthetic",
			"language": cfg.Language,
		})
		ttsReq, err := http.NewRequestWithContext(ctx, "POST", cfg.Workers.TTS+"/synthesize", bytes.NewReader(ttsBody))
		if err != nil {
			return 0, err
		}
		ttsReq.Header.Set("Content-Type", "application/json")
		ttsResp, err := cfg.HTTPClient.Do(ttsReq)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return 499, err
			}
			return 599, err
		}
		_, _ = io.Copy(io.Discard, ttsResp.Body)
		ttsResp.Body.Close()
		if ttsResp.StatusCode != 200 {
			return ttsResp.StatusCode, fmt.Errorf("tts status %d", ttsResp.StatusCode)
		}
	}
	return 200, nil
}

// FormatPercentiles renders a one-line summary for the report.
func (r Result) FormatPercentiles() string {
	return fmt.Sprintf("p50=%s p95=%s p99=%s max=%s",
		r.p50.Round(time.Millisecond),
		r.p95.Round(time.Millisecond),
		r.p99.Round(time.Millisecond),
		r.Max.Round(time.Millisecond),
	)
}

// ApproxMedian returns the latency at quantile q in [0,1].
func ApproxMedian(samples []time.Duration, q float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	idx := int(math.Round(q * float64(len(samples)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	return samples[idx]
}
