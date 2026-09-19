// Command sthira runs the bounded /api/v3 backend boundary.
//
// This is the P1 foundation slice: health/version/contract endpoints and the
// constrained middle-model validation boundary. It uses only the standard
// library. Live government integrations, durable storage and model serving are
// later phases and remain disabled.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sthira/backend/internal/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	addr := os.Getenv("STHIRA_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	cfg := httpserver.DefaultConfig(addr)
	// No ReadinessProber is wired in P1: there are no real dependencies yet, so
	// /health/ready correctly reports BLOCKED (503) rather than a false READY.
	opts := []httpserver.Option{httpserver.WithLogger(logger)}

	// Optional demo context for manual smoke testing only. It loads the golden
	// fixture IDs as the active context so a valid proposal validates. It is
	// never enabled by default and is visibly non-operational.
	if os.Getenv("STHIRA_DEMO_CONTEXT") == "1" {
		opts = append(opts, httpserver.WithKnownIDs(
			"PLACE-DEMO-1", "PLACE-DEMO-2", "FACILITY-DEMO-1", "FACILITY-DEMO-2",
		))
		logger.Warn("demo context enabled: synthetic known IDs loaded (non-operational)")
	}

	srv := httpserver.New(cfg, opts...)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Serve(ctx); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped cleanly")
}
