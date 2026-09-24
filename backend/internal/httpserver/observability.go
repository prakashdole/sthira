package httpserver

import (
	"net/http"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
)

// observabilityRoute is the public path for low-cardinality operational
// metrics. Token-guarded; fails closed (503) when no token is configured
// and 401 on missing/invalid token.
const observabilityRoute = "/api/v3/observability/metrics"

// PipelineMetricsSnapshotter is the read interface for low-cardinality
// pipeline metrics. Production wires *orchestration.Metrics (which already
// exposes Snapshot() returning orchestration.MetricsSnapshot). Nil means
// pipeline metrics are absent from the snapshot.
type PipelineMetricsSnapshotter interface {
	Snapshot() orchestration.MetricsSnapshot
}

// WorkerHealthSummary is a low-cardinality row per pipeline stage. It
// carries only stage, ready/warm flags and the worker's reported language
// allow-list. No request IDs, transcripts, or audio bytes.
type WorkerHealthSummary struct {
	Stage     string   `json:"stage"`
	Ready     bool     `json:"ready"`
	Warm      bool     `json:"warm"`
	Languages []string `json:"languages,omitempty"`
}

// handleObservability implements GET /api/v3/observability/metrics.
// The endpoint returns operational metadata suitable for a metrics scraper:
// server start time, pipeline stage counts and timings, DB connection-pool
// statistics (when a durable store is wired) and worker health summaries
// (when a voice orchestrator is wired).
//
// The endpoint NEVER returns citizen identifiers, bearer tokens, GPS
// coordinates, audio bytes, transcripts, or session IDs. The token is read
// from cfg.PprofToken; when empty the endpoint fails closed (503) so it is
// safe to leave unconfigured in production.
func (s *Server) handleObservability(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.cfg.PprofToken == "" {
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDependencyUnavailable,
			"observability surface not configured", "", false)
		return
	}
	if !s.authorizeObservabilityToken(r) {
		s.writeError(w, r, http.StatusUnauthorized, contracts.ErrUnauthorized,
			"observability token required", "", false)
		return
	}

	uptime := int64(time.Since(s.startedAt).Seconds())
	if uptime < 0 {
		uptime = 0
	}

	data := map[string]any{
		"started_at":     s.startedAt.UTC().Format(time.RFC3339),
		"now":            time.Now().UTC().Format(time.RFC3339),
		"uptime_seconds": uptime,
	}

	if s.rateLimit != nil {
		st := s.rateLimit.Stats()
		data["rate_limiter"] = map[string]any{
			"enabled":    true,
			"rps":        st.RPS,
			"burst":      st.Burst,
			"active_ips": st.ActiveIPs,
			"allowed":    st.Allowed,
			"blocked":    st.Blocked,
		}
	} else {
		data["rate_limiter"] = map[string]any{
			"enabled": false,
		}
	}

	if s.metrics != nil {
		data["pipeline"] = pipelineSnapshotToMap(s.metrics.Snapshot())
	}

	if s.store != nil && s.store.DB() != nil {
		st := s.store.DB().Stats()
		data["db_pool"] = map[string]any{
			"max_open":         st.MaxOpenConnections,
			"open":             st.OpenConnections,
			"in_use":           st.InUse,
			"idle":             st.Idle,
			"wait_count":       st.WaitCount,
			"wait_duration_ms": st.WaitDuration.Milliseconds(),
		}
	}

	if s.workersHealthFn != nil {
		if workers := s.workersHealthFn(); len(workers) > 0 {
			data["workers"] = workers
		}
	}

	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, data)
}

// authorizeObservabilityToken accepts the secret via X-Observability-Token or
// Authorization: Bearer <token>. A missing/empty cfg.PprofToken makes the
// endpoint fail closed (handled at the caller); this helper assumes a
// configured token.
func (s *Server) authorizeObservabilityToken(r *http.Request) bool {
	if s.cfg.PprofToken == "" {
		return false
	}
	token := r.Header.Get("X-Observability-Token")
	if token == "" {
		auth := r.Header.Get("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			token = auth[7:]
		}
	}
	return token != "" && token == s.cfg.PprofToken
}

func pipelineSnapshotToMap(s orchestration.MetricsSnapshot) map[string]any {
	stages := map[string]any{}
	for k, v := range s.Counts {
		key := string(k.Stage) + ":" + k.Code
		stages[key] = map[string]any{
			"count":  v.Count,
			"sum_ms": time.Duration(v.SumNS).Milliseconds(),
			"max_ms": time.Duration(v.MaxNS).Milliseconds(),
		}
	}
	return map[string]any{
		"total_observations": s.Total,
		"queue_reject":       s.QueueReject,
		"stale_drop":         s.StaleDrop,
		"worker_ready":       s.WorkerReady,
		"worker_not_ready":   s.WorkerNotReady,
		"stages":             stages,
	}
}
