package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/store"
)

// VoiceWiringConfig specifies dependencies and parameters for wiring the
// P6 voice pipeline into the HTTP server.
type VoiceWiringConfig struct {
	Store                   *store.Store
	Logger                  *slog.Logger
	ASRURL                  string
	ASRToken                string
	MiddleURL               string
	MiddleToken             string
	TTSURL                  string
	TTSToken                string
	HealthRefreshInterval   time.Duration
	AllowSyntheticTemplates bool
	Templates               orchestration.TemplateRegistry
	Validator               orchestration.VoiceValidator
	Limits                  orchestration.Limits
}

// OrchestratorSnapshotter adapts Orchestrator metrics to PipelineMetricsSnapshotter.
type OrchestratorSnapshotter struct {
	Orch *orchestration.Orchestrator
}

// Snapshot returns the current metrics snapshot from the orchestrator.
func (s OrchestratorSnapshotter) Snapshot() orchestration.MetricsSnapshot {
	if s.Orch == nil {
		return orchestration.MetricsSnapshot{}
	}
	if snap, ok := s.Orch.PipelineMetricsSnapshot(); ok {
		return snap
	}
	return orchestration.MetricsSnapshot{}
}

// WorkerHealthSummaries returns a function suitable for WithWorkersHealth
// that produces public low-cardinality summaries of each pipeline stage.
func WorkerHealthSummaries(orch *orchestration.Orchestrator) func() []WorkerHealthSummary {
	return func() []WorkerHealthSummary {
		if orch == nil {
			return nil
		}
		stages := orch.WorkerHealthStages()
		if len(stages) == 0 {
			return nil
		}
		out := make([]WorkerHealthSummary, 0, len(stages))
		for _, s := range stages {
			out = append(out, WorkerHealthSummary{
				Stage:     string(s.Stage),
				Ready:     s.Health.Ready,
				Warm:      s.Health.Warm,
				Languages: append([]string(nil), s.Health.SupportedLanguages...),
			})
		}
		return out
	}
}

// WireVoicePipeline constructs the HTTP worker clients, runs startup health probes,
// launches the background worker health refresh loop, constructs the orchestrator,
// and returns the httpserver options to register voice processing, metrics, and health.
func WireVoicePipeline(ctx context.Context, cfg VoiceWiringConfig) ([]Option, *orchestration.Orchestrator, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if cfg.ASRURL == "" || cfg.MiddleURL == "" || cfg.TTSURL == "" || cfg.Store == nil {
		logger.Info("P6 voice workers not fully configured; voice pipeline stays unavailable (503)")
		return nil, nil, nil
	}

	asrClient := orchestration.NewHTTPWorkerClient(cfg.ASRURL, cfg.ASRToken, nil)
	midClient := orchestration.NewHTTPWorkerClient(cfg.MiddleURL, cfg.MiddleToken, nil)
	ttsClient := orchestration.NewHTTPWorkerClient(cfg.TTSURL, cfg.TTSToken, nil)
	workers := orchestration.NewWorkers(asrClient, midClient, ttsClient)

	// Warm and check worker health on startup via SnapshotHealth.
	healthCtx, cancelHealth := context.WithTimeout(ctx, 5*time.Second)
	for _, stage := range []orchestration.Stage{orchestration.StageASR, orchestration.StageMiddle, orchestration.StageTTS} {
		h, err := workers.SnapshotHealth(healthCtx, stage)
		if err != nil {
			logger.Warn("initial worker health check failed", "stage", stage, "error", err)
		} else if !h.Ready || !h.Warm {
			logger.Warn("worker not ready or not warm", "stage", stage, "ready", h.Ready, "warm", h.Warm)
		} else {
			logger.Info("worker healthy and warm", "stage", stage, "languages", h.SupportedLanguages)
		}
	}
	cancelHealth()

	// Bounded refresh loop: re-probe each worker on a steady interval until shutdown.
	refreshInterval := cfg.HealthRefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = 10 * time.Second
	}
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rctx, rcancel := context.WithTimeout(ctx, 3*time.Second)
				for _, stage := range []orchestration.Stage{orchestration.StageASR, orchestration.StageMiddle, orchestration.StageTTS} {
					h, err := workers.SnapshotHealth(rctx, stage)
					if err != nil {
						logger.Debug("worker health refresh failed", "stage", stage, "error", err)
						continue
					}
					if !h.Ready || !h.Warm {
						logger.Warn("worker not ready during refresh", "stage", stage, "ready", h.Ready, "warm", h.Warm)
					}
				}
				rcancel()
			}
		}
	}()

	limits := cfg.Limits
	if limits.ASRMaxInflight <= 0 {
		limits = orchestration.DefaultLimits()
	}

	templates := cfg.Templates
	if templates == nil {
		templates = orchestration.DefaultTemplateRegistry()
	}

	validator := cfg.Validator
	if validator == nil {
		validator = orchestration.NewProductionValidator()
	}

	resolver := store.NewScopedContextResolver(cfg.Store)

	orch, err := orchestration.NewOrchestrator(orchestration.PipelineConfig{
		Limits:                  limits,
		Workers:                 workers,
		Resolver:                resolver,
		Validator:               validator,
		Templates:               templates,
		AllowSyntheticTemplates: cfg.AllowSyntheticTemplates,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to construct voice orchestrator: %w", err)
	}

	voiceHandler := NewVoiceProcessHandler(orch, limits)
	opts := []Option{
		WithVoiceProcess(voiceHandler),
		WithMetricsSnapshotter(OrchestratorSnapshotter{Orch: orch}),
		WithWorkersHealth(WorkerHealthSummaries(orch)),
	}

	logger.Info("P6 voice orchestrator wired with private workers",
		"synthetic_templates_allowed", cfg.AllowSyntheticTemplates)

	return opts, orch, nil
}
