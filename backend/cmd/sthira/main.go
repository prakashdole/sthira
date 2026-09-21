// Command sthira runs the bounded /api/v3 backend boundary.
//
// Foundation mode (no STHIRA_DATABASE_DSN) serves health/version/contract
// endpoints and the constrained middle-model validation boundary, with
// readiness reporting BLOCKED. When STHIRA_DATABASE_DSN is set the server opens
// the durable store, wires the migration-aware readiness prober and the P4
// destination/stay routes, and closes the pool on shutdown.
//
// Dependency readiness is not operational source readiness: a reachable,
// correctly-migrated database proves the dependency only. Operational guidance
// additionally requires an OPERATIONAL authorized source; a working database
// never enables unapproved guidance. Live government integrations and model
// serving remain disabled.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sthira/backend/internal/httpserver"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	addr := os.Getenv("STHIRA_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	cfg := httpserver.DefaultConfig(addr)
	opts := []httpserver.Option{httpserver.WithLogger(logger)}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Durable storage. When a DSN is provided, open the store and wire the
	// migration-aware readiness prober; otherwise readiness stays BLOCKED.
	var st *store.Store
	if dsn := os.Getenv("STHIRA_DATABASE_DSN"); dsn != "" {
		var err error
		st, err = store.Open(dsn)
		if err != nil {
			logger.Error("failed to open store", "error", err)
			os.Exit(1)
		}
		defer func() { _ = st.Close() }()
		opts = append(opts,
			httpserver.WithProber(store.NewReadinessProber(st.DB(), store.SchemaRevision)),
			httpserver.WithStore(st),
			// Persisted context resolver: voice-commands validates against the
			// current authorized OPERATIONAL package snapshot, failing closed when
			// none exists. Distinct from the static demo resolver below.
			httpserver.WithPersistedContextResolver(st),
		)
		logger.Info("durable store wired", "schema_revision", store.SchemaRevision)

		// Operator issuance: no trusted identity/MFA verifier is wired in this
		// binary, so POST /api/v3/operations/sessions fails closed. Wiring a real
		// verifier requires an external identity-provider decision (see
		// plan/open-decisions.md); until then no operator token can be minted here.
		logger.Info("no trusted operator identity verifier configured; operator issuance fails closed")

		// Bounded, retry-safe expiry worker: expires RESERVED holds past their
		// expiry so held capacity returns to free. Runs against the DB (not a
		// process-local lock) so it races safely with arrival/transfer.
		expiry := store.NewExpiryWorker(st, store.NewStayStore(store.ChainAuditor{}))
		go expiry.Run(ctx)
	} else {
		logger.Info("no STHIRA_DATABASE_DSN; foundation mode, readiness BLOCKED")
	}

	// P6 Voice Pipeline wiring:
	// When private worker URLs are configured (STHIRA_ASR_URL, STHIRA_MIDDLE_URL, STHIRA_TTS_URL)
	// and durable store is available, wire the full voice orchestrator.
	// Otherwise, incomplete model configuration stays unavailable (fail closed 503).
	asrURL := os.Getenv("STHIRA_ASR_URL")
	asrTok := os.Getenv("STHIRA_ASR_TOKEN")
	midURL := os.Getenv("STHIRA_MIDDLE_URL")
	midTok := os.Getenv("STHIRA_MIDDLE_TOKEN")
	ttsURL := os.Getenv("STHIRA_TTS_URL")
	ttsTok := os.Getenv("STHIRA_TTS_TOKEN")

	if asrURL != "" && midURL != "" && ttsURL != "" && st != nil {
		asrClient := orchestration.NewHTTPWorkerClient(asrURL, asrTok, nil)
		midClient := orchestration.NewHTTPWorkerClient(midURL, midTok, nil)
		ttsClient := orchestration.NewHTTPWorkerClient(ttsURL, ttsTok, nil)
		workers := orchestration.NewWorkers(asrClient, midClient, ttsClient)

		resolver := store.NewScopedContextResolver(st)
		validator := orchestration.NewProductionValidator()
		templates := orchestration.DefaultTemplateRegistry()

		orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
			Limits:    orchestration.DefaultLimits(),
			Workers:   workers,
			Resolver:  resolver,
			Validator: validator,
			Templates: templates,
		})
		if err != nil {
			logger.Error("failed to construct voice orchestrator", "error", err)
			os.Exit(1)
		}
		voiceHandler := httpserver.NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
		opts = append(opts, httpserver.WithVoiceProcess(voiceHandler))
		logger.Info("P6 voice orchestrator wired with private workers")
	} else {
		logger.Info("P6 voice workers not fully configured; voice pipeline stays unavailable (503)")
	}

	// Optional demo context for manual smoke testing only. It wires a static
	// server-side snapshot resolver with the golden fixture IDs so a valid
	// proposal validates against a server-resolved context. It is never enabled
	// by default and is visibly non-operational. Without it the voice-commands
	// endpoint fails closed (503).
	if os.Getenv("STHIRA_DEMO_CONTEXT") == "1" {
		opts = append(opts, httpserver.WithContextResolver(httpserver.StaticContextResolver(httpserver.ContextSnapshot{
			DataVersion:  "demo-1",
			Jurisdiction: "DEMO",
			KnownIDs: map[string]bool{
				"PLACE-DEMO-1": true, "PLACE-DEMO-2": true,
				"FACILITY-DEMO-1": true, "FACILITY-DEMO-2": true,
			},
			EnabledLanguages: map[string]bool{"en-IN": true, "hi-IN": true, "ml-IN": true},
		})))
		logger.Warn("demo context enabled: static server-side snapshot resolver (non-operational)")
	}

	srv := httpserver.New(cfg, opts...)

	if err := srv.Serve(ctx); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped cleanly")
}
