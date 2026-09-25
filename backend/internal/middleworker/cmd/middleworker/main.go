// Command middleworker runs the private Sarvam-30B FP8 MoE middle worker service.
// It wraps the upstream vLLM inference server and exposes the private HTTP protocol.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"sthira/backend/internal/middleworker"
)

func main() {
	addr := os.Getenv("STHIRA_MIDDLE_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8002"
	}
	tok := os.Getenv("STHIRA_MIDDLE_TOKEN")

	vllmURL := os.Getenv("STHIRA_VLLM_URL")
	if vllmURL == "" {
		vllmURL = "http://127.0.0.1:8000"
	}

	revision := os.Getenv("STHIRA_MIDDLE_REVISION")
	if revision == "" {
		revision = "sarvam-30b-fp8-v1"
	}
	digestSHA := os.Getenv("STHIRA_MIDDLE_DIGEST_SHA")
	if digestSHA == "" {
		digestSHA = "pinned"
	}

	queueDepth := 8
	if q := os.Getenv("STHIRA_MIDDLE_QUEUE_DEPTH"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			queueDepth = v
		}
	}
	maxInflight := 2
	if m := os.Getenv("STHIRA_MIDDLE_MAX_INFLIGHT"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			maxInflight = v
		}
	}

	client, err := middleworker.NewClient(middleworker.ClientConfig{
		BaseURL:    vllmURL,
		SchemaJSON: middleworker.DefaultModelOutputSchema(),
		Limits:     middleworker.SarvamLimits(),
	})
	if err != nil {
		log.Fatalf("failed to create middle client: %v", err)
	}

	cfg := middleworker.SarvamConfig(client, "")
	cfg.Revision = revision
	cfg.DigestSHA = digestSHA

	rt, err := middleworker.NewHTTPClientRuntime(cfg)
	if err != nil {
		log.Fatalf("failed to create http client runtime: %v", err)
	}

	worker, err := middleworker.NewWorker(middleworker.Config{
		Runtime:       rt,
		QueueDepth:    queueDepth,
		MaxInFlight:   maxInflight,
		BuildRevision: cfg.Revision,
		SystemHint:    "Sarvam-30B FP8 MoE",
	})
	if err != nil {
		log.Fatalf("failed to create middle worker: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := worker.LoadAndVerify(ctx); err != nil {
		log.Fatalf("failed to load middle worker (failing closed): %v", err)
	}

	srv, err := middleworker.NewServer(middleworker.ServerConfig{
		Address: addr,
		Token:   tok,
	}, worker)
	if err != nil {
		log.Fatalf("failed to create middle server: %v", err)
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown()
	}()

	log.Printf("middleworker listening on %s (private)", addr)
	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("middleworker server exited: %v", err)
	}
}
