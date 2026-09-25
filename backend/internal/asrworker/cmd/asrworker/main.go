// Command asrworker runs the private IndicConformer-600M ASR worker service.
// It wraps the real Python adapter subprocess and exposes the private HTTP protocol.
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
	"time"

	"sthira/backend/internal/asrworker"
)

func main() {
	addr := os.Getenv("STHIRA_ASR_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8001"
	}
	tok := os.Getenv("STHIRA_ASR_TOKEN")

	pyCmd := os.Getenv("STHIRA_ASR_PYTHON")
	if pyCmd == "" {
		pyCmd = "python3"
	}
	module := os.Getenv("STHIRA_ASR_ADAPTER")
	if module == "" {
		module = "sthira_v2.speech_asr_adapter"
	}
	workdir := os.Getenv("STHIRA_ASR_WORKDIR")

	queueDepth := 8
	if q := os.Getenv("STHIRA_ASR_QUEUE_DEPTH"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			queueDepth = v
		}
	}
	maxInflight := 2
	if m := os.Getenv("STHIRA_ASR_MAX_INFLIGHT"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			maxInflight = v
		}
	}

	rt := asrworker.NewSubprocessRuntime(asrworker.SubprocessRuntimeConfig{
		Cmd:     pyCmd,
		Module:  module,
		Workdir: workdir,
	})
	if err := rt.LoadModel(); err != nil {
		log.Fatalf("failed to load asr adapter: %v", err)
	}
	defer rt.Close()

	worker, err := asrworker.NewWorker(asrworker.Config{
		Inventory:   asrworker.Inventory{AllowedLanguages: rt.SupportedLanguages(), LastScannedAt: time.Now()},
		Runtime:     rt,
		QueueDepth:  queueDepth,
		MaxInFlight: maxInflight,
	})
	if err != nil {
		log.Fatalf("failed to create asr worker: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Model must be loaded and verified at startup. Missing weights or failure fails closed.
	if err := worker.LoadAndVerify(ctx); err != nil {
		log.Fatalf("failed to load asr model (failing closed): %v", err)
	}

	srv, err := asrworker.NewServer(asrworker.ServerConfig{
		Address: addr,
		Token:   tok,
	}, worker)
	if err != nil {
		log.Fatalf("failed to create asr server: %v", err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Cancel(shutdownCtx)
		_ = worker.Shutdown(shutdownCtx)
	}()

	log.Printf("asrworker listening on %s (private)", addr)
	if err := srv.Start(ctx, addr); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		log.Fatalf("asrworker server exited: %v", err)
	}
}
