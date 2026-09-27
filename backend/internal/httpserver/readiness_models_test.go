// Regression: /health/ready must stop answering 200 READY when a model
// worker dies. Readiness has to follow the live per-stage ready flag the
// orchestrator gates dispatch on, not the cached health snapshot: a killed
// worker keeps its last successful snapshot forever, so snapshot-derived
// readiness reported a dead model as ready while POST /api/v3/voice/process
// correctly returned 503 MODEL_UNAVAILABLE.
//
// Every phase drives exactly one Workers.SnapshotHealth round over the
// three stages — the same work the STHIRA_WORKER_HEALTH_REFRESH loop
// (10s default) does per tick — so the test never sleeps.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
)

// modelStages is the pipeline's worker stages, in pipeline order.
var modelStages = []orchestration.Stage{orchestration.StageASR, orchestration.StageMiddle, orchestration.StageTTS}

// errModelWorkerUnused fails the inference methods, which readiness never calls.
var errModelWorkerUnused = errors.New("inference not expected in a readiness test")

// modelFakeWorker is the minimal WorkerClient fake. up=false models a
// worker whose process is gone: Health fails, exactly as the production
// client does when the worker's /health cannot be reached.
type modelFakeWorker struct {
	up atomic.Bool
}

func (w *modelFakeWorker) Health(context.Context) (contracts.WorkerHealth, bool) {
	if !w.up.Load() {
		return contracts.WorkerHealth{}, false
	}
	return contracts.WorkerHealth{Ready: true, Warm: true, SupportedLanguages: []string{"en-IN"}}, true
}

func (w *modelFakeWorker) Transcribe(context.Context, contracts.ASRWorkerRequest) (contracts.ASRWorkerResponse, error) {
	return contracts.ASRWorkerResponse{}, errModelWorkerUnused
}

func (w *modelFakeWorker) Propose(context.Context, contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
	return contracts.MiddleWorkerResponse{}, errModelWorkerUnused
}

func (w *modelFakeWorker) Synthesize(context.Context, contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
	return contracts.TTSWorkerResponse{}, errModelWorkerUnused
}

// TestReadinessModelsFollowLiveWorkerReadyFlag walks the three observable
// phases: healthy -> one worker dead -> recovered.
func TestReadinessModelsFollowLiveWorkerReadyFlag(t *testing.T) {
	asr, mid, tts := &modelFakeWorker{}, &modelFakeWorker{}, &modelFakeWorker{}
	asr.up.Store(true)
	mid.up.Store(true)
	tts.up.Store(true)
	workers := orchestration.NewWorkers(asr, mid, tts)
	srv := New(DefaultConfig("127.0.0.1:0"),
		WithProber(okProber{}),
		WithWorkersHealth(WorkerHealthSummaries(workers)),
	)

	refresh := func() {
		t.Helper()
		// Mirrors the production refresh loop: a failed probe is logged and
		// the remaining stages are still probed.
		for _, stage := range modelStages {
			_, _ = workers.SnapshotHealth(context.Background(), stage)
		}
	}
	serveReady := func() *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
		return rec
	}

	// Phase 1: all three stages healthy.
	refresh()
	if got := srv.CheckReadiness(context.Background()).Status; got != "READY" {
		t.Fatalf("phase 1 status = %q, want READY", got)
	}
	if rec := serveReady(); rec.Code != http.StatusOK {
		t.Fatalf("phase 1 /health/ready = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	// Phase 2: the middle worker's process is gone. Readiness must fail
	// closed even though its last health snapshot still says ready.
	mid.up.Store(false)
	refresh()
	report := srv.CheckReadiness(context.Background())
	if report.Status != "NOT_READY" {
		t.Fatalf("phase 2 status = %q with models %+v, want NOT_READY", report.Status, report.Subsystems["models"])
	}
	models, ok := report.Subsystems["models"]
	if !ok || models.Status != StatusNotReady {
		t.Fatalf("phase 2 models subsystem = %+v, want NOT_READY", models)
	}
	if !strings.Contains(models.Detail, "middle") {
		t.Fatalf("phase 2 models detail = %q, want it to name the middle stage", models.Detail)
	}
	rec := serveReady()
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("phase 2 /health/ready = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Errors []contracts.APIError `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("phase 2 body is not a valid envelope: %v", err)
	}
	if resp.Data.Status != "" || len(resp.Errors) == 0 || resp.Errors[0].Code != contracts.ErrDataUnavailable {
		t.Fatalf("phase 2 must report an error, not data: %+v", resp)
	}

	// Phase 3: the worker recovers.
	mid.up.Store(true)
	refresh()
	if got := srv.CheckReadiness(context.Background()).Status; got != "READY" {
		t.Fatalf("phase 3 status = %q, want READY", got)
	}
	if rec := serveReady(); rec.Code != http.StatusOK {
		t.Fatalf("phase 3 /health/ready = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}
