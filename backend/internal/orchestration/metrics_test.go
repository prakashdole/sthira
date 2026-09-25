package orchestration

import (
	"testing"
	"time"

	"sthira/backend/internal/contracts"
)

func TestMetrics_RecordTiming(t *testing.T) {
	m := NewMetrics()
	m.ObserveStageStart(StageASR)
	m.ObserveStageEnd(StageASR, "OK", 50*time.Millisecond)
	m.ObserveStageStart(StageASR)
	m.ObserveStageEnd(StageASR, "TIMEOUT", 3*time.Second)

	snap := m.Snapshot()
	sum := snap.Counts[stageKey{Stage: StageASR, Code: "OK"}]
	if sum.Count != 1 || sum.SumNS != int64(50*time.Millisecond) {
		t.Errorf("OK summary = %+v; want count=1 sum=50ms", sum)
	}
	sum2 := snap.Counts[stageKey{Stage: StageASR, Code: "TIMEOUT"}]
	if sum2.MaxNS != int64(3*time.Second) {
		t.Errorf("TIMEOUT max = %d; want 3s", sum2.MaxNS)
	}
}

func TestMetrics_StaleDropCounters(t *testing.T) {
	m := NewMetrics()
	m.ObserveStaleDrop(StageASR)
	m.ObserveStaleDrop(StageASR)
	m.ObserveStaleDrop(StageMiddle)
	snap := m.Snapshot()
	if snap.StaleDrop[StageASR] != 2 {
		t.Errorf("ASR stale drops = %d; want 2", snap.StaleDrop[StageASR])
	}
	if snap.StaleDrop[StageMiddle] != 1 {
		t.Errorf("Middle stale drops = %d; want 1", snap.StaleDrop[StageMiddle])
	}
}

func TestMetrics_QueueRejectCounters(t *testing.T) {
	m := NewMetrics()
	m.ObserveQueueReject(StageMiddle)
	snap := m.Snapshot()
	if snap.QueueReject[StageMiddle] != 1 {
		t.Errorf("Middle queue reject = %d; want 1", snap.QueueReject[StageMiddle])
	}
}

func TestMetrics_WorkerHealthCounters(t *testing.T) {
	m := NewMetrics()
	m.ObserveWorkerHealth(StageASR, true, true)
	m.ObserveWorkerHealth(StageASR, true, false)
	m.ObserveWorkerHealth(StageMiddle, false, false)
	snap := m.Snapshot()
	if snap.WorkerReady[StageASR] != 1 {
		t.Errorf("ASR ready = %d; want 1", snap.WorkerReady[StageASR])
	}
	if snap.WorkerNotReady[StageASR] != 1 {
		t.Errorf("ASR not-ready = %d; want 1", snap.WorkerNotReady[StageASR])
	}
	if snap.WorkerNotReady[StageMiddle] != 1 {
		t.Errorf("Middle not-ready = %d; want 1", snap.WorkerNotReady[StageMiddle])
	}
}

func TestPipelineStateCode(t *testing.T) {
	cases := map[contracts.PipelineState]string{
		contracts.PipelineOK:               "OK",
		contracts.PipelineClarify:          "CLARIFY",
		contracts.PipelineUnsupported:      "UNSUPPORTED",
		contracts.PipelineDataUnavailable:  "DATA_UNAVAILABLE",
		contracts.PipelineModelUnavailable: "UNAVAILABLE",
		contracts.PipelineCanceled:         "CANCELED",
	}
	for s, want := range cases {
		if got := PipelineStateCode(s); got != want {
			t.Errorf("PipelineStateCode(%s) = %s, want %s", s, got, want)
		}
	}
	if got := PipelineStateCode(""); got != "UNKNOWN" {
		t.Errorf("PipelineStateCode(unknown) = %s, want UNKNOWN", got)
	}
}

func TestNopMetrics_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NopMetrics panicked: %v", r)
		}
	}()
	var m MetricsRecorder = NopMetrics{}
	m.ObserveStageStart(StageASR)
	m.ObserveStageEnd(StageASR, "OK", time.Millisecond)
	m.ObserveStaleDrop(StageASR)
	m.ObserveQueueReject(StageASR)
	m.ObserveWorkerHealth(StageASR, true, true)
}
