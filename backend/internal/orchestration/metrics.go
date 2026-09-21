package orchestration

import (
	"sync"
	"sync/atomic"
	"time"

	"sthira/backend/internal/contracts"
)

// Metrics is the in-process, low-cardinality metrics sink. It tracks
// timing, queue rejection, and stale-drop counts per stage. It does
// NOT record audio bytes, transcripts, locations, request IDs, or
// credential material; those are scrubbed upstream.
//
// Cardinality is bounded by the stage × code × outcome cross-product,
// which is small by construction. Tests can read the counters via
// Snapshot.
type Metrics struct {
	mu      sync.RWMutex
	stageID Stage
	// timings records per-(stage, code) durations in a sliding
	// window of size 64. The window is small because the orchestrator
	// only needs approximate percentiles for the metrics scraper.
	timings map[stageKey]*timingHistogram
	// counters are the cumulative observation counts.
	counters atomic.Uint64
	// queueReject counts.
	queueReject map[Stage]uint64
	// staleDrop counts.
	staleDrop map[Stage]uint64
	// workerHealth counts (ready and not-ready).
	workerReady    map[Stage]uint64
	workerNotReady map[Stage]uint64
}

type stageKey struct {
	Stage Stage
	Code  string
}

type timingHistogram struct {
	count uint64
	sumNS int64
	maxNS int64
	// ring buffer of last 64 durations in nanoseconds.
	buf    [64]int64
	cursor int
}

// NewMetrics returns an empty in-process metrics sink.
func NewMetrics() *Metrics {
	return &Metrics{
		timings:        map[stageKey]*timingHistogram{},
		queueReject:    map[Stage]uint64{},
		staleDrop:      map[Stage]uint64{},
		workerReady:    map[Stage]uint64{},
		workerNotReady: map[Stage]uint64{},
	}
}

// ObserveStageStart implements MetricsRecorder. It records the
// observation count and the start time; ObserveStageEnd computes the
// delta. The start time is not stored across stages because each
// stage uses a fresh histogram entry.
func (m *Metrics) ObserveStageStart(stage Stage) {
	if m == nil {
		return
	}
	m.counters.Add(1)
	m.stageID = stage
}

// ObserveStageEnd records the (stage, code, duration) tuple. code is
// one of: "OK", "TIMEOUT", "UNAVAILABLE", "REJECTED", "STALE",
// "VALIDATION", "CANCELED". Any other string is treated as an unknown
// outcome and stored as-is (test-only).
func (m *Metrics) ObserveStageEnd(stage Stage, code string, d time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	k := stageKey{Stage: stage, Code: code}
	h, ok := m.timings[k]
	if !ok {
		h = &timingHistogram{}
		m.timings[k] = h
	}
	h.count++
	ns := d.Nanoseconds()
	h.sumNS += ns
	if ns > h.maxNS {
		h.maxNS = ns
	}
	h.buf[h.cursor] = ns
	h.cursor = (h.cursor + 1) % len(h.buf)
}

// ObserveStaleDrop implements MetricsRecorder.
func (m *Metrics) ObserveStaleDrop(stage Stage) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.staleDrop[stage]++
}

// ObserveQueueReject implements MetricsRecorder.
func (m *Metrics) ObserveQueueReject(stage Stage) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueReject[stage]++
}

// ObserveWorkerHealth implements MetricsRecorder.
func (m *Metrics) ObserveWorkerHealth(stage Stage, ready, warm bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if ready && warm {
		m.workerReady[stage]++
	} else {
		m.workerNotReady[stage]++
	}
}

// Snapshot returns a low-cardinality snapshot of the metrics. The
// returned map is a copy; callers may freely mutate it. The timing
// histogram is summarized as (count, sum, max) and a 64-element ring
// buffer of recent durations.
type MetricsSnapshot struct {
	Counts         map[stageKey]TimingSummary
	QueueReject    map[Stage]uint64
	StaleDrop      map[Stage]uint64
	WorkerReady    map[Stage]uint64
	WorkerNotReady map[Stage]uint64
	Total          uint64
}

// TimingSummary is a low-cardinality summary of a timing histogram.
type TimingSummary struct {
	Count  uint64
	SumNS  int64
	MaxNS  int64
	Recent []int64
}

// Snapshot returns a copy of the current metrics state.
func (m *Metrics) Snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := MetricsSnapshot{
		Counts:         map[stageKey]TimingSummary{},
		QueueReject:    map[Stage]uint64{},
		StaleDrop:      map[Stage]uint64{},
		WorkerReady:    map[Stage]uint64{},
		WorkerNotReady: map[Stage]uint64{},
		Total:          m.counters.Load(),
	}
	for k, h := range m.timings {
		out.Counts[k] = TimingSummary{
			Count:  h.count,
			SumNS:  h.sumNS,
			MaxNS:  h.maxNS,
			Recent: append([]int64(nil), h.buf[:]...),
		}
	}
	for k, v := range m.queueReject {
		out.QueueReject[k] = v
	}
	for k, v := range m.staleDrop {
		out.StaleDrop[k] = v
	}
	for k, v := range m.workerReady {
		out.WorkerReady[k] = v
	}
	for k, v := range m.workerNotReady {
		out.WorkerNotReady[k] = v
	}
	return out
}

// PipelineStateCode converts a contracts.PipelineState to the
// canonical "code" string used in metrics.
func PipelineStateCode(state contracts.PipelineState) string {
	switch state {
	case contracts.PipelineOK:
		return "OK"
	case contracts.PipelineClarify:
		return "CLARIFY"
	case contracts.PipelineUnsupported:
		return "UNSUPPORTED"
	case contracts.PipelineDataUnavailable:
		return "DATA_UNAVAILABLE"
	case contracts.PipelineModelUnavailable:
		return "UNAVAILABLE"
	case contracts.PipelineCanceled:
		return "CANCELED"
	default:
		return "UNKNOWN"
	}
}
