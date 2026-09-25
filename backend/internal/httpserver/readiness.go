package httpserver

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"sthira/backend/internal/store"
)

// SubsystemStatus represents the operational status of an individual subsystem.
type SubsystemStatus string

const (
	StatusReady       SubsystemStatus = "READY"
	StatusNotReady    SubsystemStatus = "NOT_READY"
	StatusDisabled    SubsystemStatus = "DISABLED"
	StatusUnavailable SubsystemStatus = "UNAVAILABLE"
	StatusMismatch    SubsystemStatus = "SCHEMA_MISMATCH"
	StatusSkipped     SubsystemStatus = "SKIPPED"
	StatusOK          SubsystemStatus = "OK"
)

// SubsystemHealth captures readiness details for an individual subsystem.
type SubsystemHealth struct {
	Status SubsystemStatus `json:"status"`
	Detail string          `json:"detail,omitempty"`
}

// MemoryHealth captures non-sensitive runtime memory allocation bounds.
type MemoryHealth struct {
	Status          SubsystemStatus `json:"status"`
	AllocBytes      uint64          `json:"alloc_bytes"`
	TotalAllocBytes uint64          `json:"total_alloc_bytes"`
	SysBytes        uint64          `json:"sys_bytes"`
	NumGC           uint32          `json:"num_gc"`
}

// ReadinessReport captures the comprehensive readiness breakdown of the service.
type ReadinessReport struct {
	Status     string                     `json:"status"`
	Subsystems map[string]SubsystemHealth `json:"subsystems"`
	Memory     MemoryHealth               `json:"memory"`
}

// Summary returns a sanitized string description of the readiness status.
// Never exposes internal hostnames, passwords, or connection strings.
func (r ReadinessReport) Summary() string {
	if r.Status == "READY" {
		return "all subsystems ready"
	}
	var failed []string
	for name, sub := range r.Subsystems {
		if sub.Status != StatusReady && sub.Status != StatusDisabled && sub.Status != StatusSkipped {
			failed = append(failed, name)
		}
	}
	if len(failed) > 0 {
		return fmt.Sprintf("dependencies not ready: %s", strings.Join(failed, ", "))
	}
	return "dependencies not ready"
}

// CheckReadiness performs active, non-blocking probes across all configured subsystems.
// Returns a structured report without leaking private credentials or internal topology.
func (s *Server) CheckReadiness(ctx context.Context) ReadinessReport {
	subsystems := make(map[string]SubsystemHealth)
	allReady := true

	// 1. Prober check (authoritative readiness seam)
	if s.prober == nil {
		subsystems["prober"] = SubsystemHealth{
			Status: StatusUnavailable,
			Detail: "no readiness prober configured",
		}
		allReady = false
	} else {
		if err := s.prober.Probe(ctx); err != nil {
			subsystems["prober"] = SubsystemHealth{
				Status: StatusUnavailable,
				Detail: "readiness probe rejected",
			}
			allReady = false
		} else {
			subsystems["prober"] = SubsystemHealth{
				Status: StatusReady,
				Detail: "verified",
			}
		}
	}

	// 2. Database connectivity
	if s.store == nil {
		subsystems["database"] = SubsystemHealth{
			Status: StatusDisabled,
			Detail: "store not configured",
		}
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if err := s.store.DB().PingContext(pingCtx); err != nil {
			subsystems["database"] = SubsystemHealth{
				Status: StatusUnavailable,
				Detail: "connection ping failed",
			}
			allReady = false
		} else {
			subsystems["database"] = SubsystemHealth{
				Status: StatusReady,
				Detail: "connected",
			}
		}
	}

	// 3. Migrations / Schema revision
	if s.store == nil {
		subsystems["migrations"] = SubsystemHealth{
			Status: StatusSkipped,
			Detail: "store not configured",
		}
	} else {
		migCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		var rev int
		err := s.store.DB().QueryRowContext(migCtx,
			`SELECT COALESCE(MAX(revision), 0) FROM schema_migrations`).Scan(&rev)
		if err != nil {
			subsystems["migrations"] = SubsystemHealth{
				Status: StatusUnavailable,
				Detail: "migration state unreadable",
			}
			allReady = false
		} else if rev < store.SchemaRevision {
			subsystems["migrations"] = SubsystemHealth{
				Status: StatusMismatch,
				Detail: fmt.Sprintf("schema at revision %d, expected %d", rev, store.SchemaRevision),
			}
			allReady = false
		} else {
			subsystems["migrations"] = SubsystemHealth{
				Status: StatusReady,
				Detail: fmt.Sprintf("revision current (%d)", rev),
			}
		}
	}

	// 4. Source activation (distinguishes operational government data from API DB connectivity)
	if s.store == nil {
		subsystems["source_activation"] = SubsystemHealth{
			Status: StatusSkipped,
			Detail: "store not configured",
		}
	} else {
		srcCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		var count int
		err := s.store.DB().QueryRowContext(srcCtx,
			`SELECT COUNT(*) FROM sources WHERE state = 'OPERATIONAL'`).Scan(&count)
		if err != nil {
			subsystems["source_activation"] = SubsystemHealth{
				Status: StatusUnavailable,
				Detail: "source activation unreadable",
			}
		} else if count == 0 {
			subsystems["source_activation"] = SubsystemHealth{
				Status: StatusUnavailable,
				Detail: "no operational sources activated",
			}
		} else {
			subsystems["source_activation"] = SubsystemHealth{
				Status: StatusReady,
				Detail: fmt.Sprintf("%d operational sources", count),
			}
		}
	}

	// 5. Operator Identity Provider (distinguishes operator IdP from local API readiness)
	if s.operatorVerifier == nil {
		subsystems["idp"] = SubsystemHealth{
			Status: StatusDisabled,
			Detail: "operator IdP not configured; operator sessions fail closed",
		}
	} else {
		subsystems["idp"] = SubsystemHealth{
			Status: StatusReady,
			Detail: "operator IdP configured",
		}
	}

	// 6. Voice Models (distinguishes model worker readiness from API server liveness)
	if s.workersHealthFn != nil {
		workers := s.workersHealthFn()
		allHealthy := true
		var issues []string
		for _, w := range workers {
			if !w.Ready {
				allHealthy = false
				issues = append(issues, fmt.Sprintf("%s(not ready)", w.Stage))
			}
		}
		if allHealthy && len(workers) > 0 {
			subsystems["models"] = SubsystemHealth{
				Status: StatusReady,
				Detail: fmt.Sprintf("all %d worker stages ready", len(workers)),
			}
		} else if len(workers) == 0 {
			subsystems["models"] = SubsystemHealth{
				Status: StatusNotReady,
				Detail: "no workers reporting",
			}
		} else {
			subsystems["models"] = SubsystemHealth{
				Status: StatusNotReady,
				Detail: fmt.Sprintf("unready worker stages: %s", strings.Join(issues, ", ")),
			}
		}
	} else if s.voiceProcess != nil {
		subsystems["models"] = SubsystemHealth{
			Status: StatusReady,
			Detail: "voice pipeline configured",
		}
	} else {
		subsystems["models"] = SubsystemHealth{
			Status: StatusDisabled,
			Detail: "voice pipeline not wired",
		}
	}

	// 7. Rate Limiter
	if s.rateLimit == nil {
		subsystems["rate_limiter"] = SubsystemHealth{
			Status: StatusDisabled,
			Detail: "rate limiting disabled",
		}
	} else {
		subsystems["rate_limiter"] = SubsystemHealth{
			Status: StatusReady,
			Detail: "active",
		}
	}

	// 5. Memory bounds
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memHealth := MemoryHealth{
		Status:          StatusOK,
		AllocBytes:      m.Alloc,
		TotalAllocBytes: m.TotalAlloc,
		SysBytes:        m.Sys,
		NumGC:           m.NumGC,
	}

	statusStr := "READY"
	if !allReady {
		statusStr = "NOT_READY"
	}

	return ReadinessReport{
		Status:     statusStr,
		Subsystems: subsystems,
		Memory:     memHealth,
	}
}
