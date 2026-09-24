package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpserver"
	"sthira/backend/internal/store"
)

type readyTestProber struct {
	err error
}

func (p readyTestProber) Probe(ctx context.Context) error {
	return p.err
}

type readinessEnvelope struct {
	Status     string                                `json:"status"`
	Subsystems map[string]httpserver.SubsystemHealth `json:"subsystems"`
	Memory     httpserver.MemoryHealth               `json:"memory"`
}

type readyResponse struct {
	RequestID string               `json:"request_id"`
	Data      readinessEnvelope    `json:"data"`
	Errors    []contracts.APIError `json:"errors"`
}

func TestReadiness_BreakdownWhenReady(t *testing.T) {
	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithProber(readyTestProber{err: nil}),
		httpserver.WithRateLimit(100, 50),
	)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode readiness response: %v", err)
	}

	if resp.Data.Status != "READY" {
		t.Fatalf("expected status READY, got %q", resp.Data.Status)
	}

	// Verify prober subsystem
	proberSub, ok := resp.Data.Subsystems["prober"]
	if !ok || proberSub.Status != httpserver.StatusReady {
		t.Fatalf("expected prober READY, got %+v", proberSub)
	}

	// Verify rate limiter subsystem
	rlSub, ok := resp.Data.Subsystems["rate_limiter"]
	if !ok || rlSub.Status != httpserver.StatusReady {
		t.Fatalf("expected rate_limiter READY, got %+v", rlSub)
	}

	// Verify database disabled without store
	dbSub, ok := resp.Data.Subsystems["database"]
	if !ok || dbSub.Status != httpserver.StatusDisabled {
		t.Fatalf("expected database DISABLED, got %+v", dbSub)
	}

	// Verify memory health
	if resp.Data.Memory.Status != httpserver.StatusOK || resp.Data.Memory.AllocBytes == 0 {
		t.Fatalf("expected memory OK with non-zero alloc, got %+v", resp.Data.Memory)
	}
}

func TestReadiness_RedactionOfInternalProberError(t *testing.T) {
	sensitiveError := errors.New("dial postgres://admin:supersecret@10.0.0.1:5432/sthira refused")
	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithProber(readyTestProber{err: sensitiveError}),
	)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "supersecret") || strings.Contains(body, "10.0.0.1") {
		t.Fatalf("readiness response leaked sensitive internal topology: %s", body)
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode readiness error response: %v", err)
	}
	if len(resp.Errors) == 0 || resp.Errors[0].Code != contracts.ErrDataUnavailable {
		t.Fatalf("expected DATA_UNAVAILABLE error, got %+v", resp.Errors)
	}
}

func TestReadiness_SchemaRevisionLiveDB(t *testing.T) {
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN not set; skipping live DB readiness check")
	}
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithStore(st),
		httpserver.WithProber(readyTestProber{err: nil}),
	)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode readiness response: %v", err)
	}

	migSub, ok := resp.Data.Subsystems["migrations"]
	if !ok || migSub.Status != httpserver.StatusReady {
		t.Fatalf("expected migrations READY, got %+v", migSub)
	}
}

func TestReadiness_SchemaRevisionMismatch(t *testing.T) {
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN not set; skipping live DB readiness check")
	}
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	// Roll back revision 10 to simulate an unmigrated DB
	_, err = st.DB().Exec("DELETE FROM schema_migrations WHERE revision >= 10")
	if err != nil {
		t.Fatalf("failed to delete revision: %v", err)
	}
	defer func() {
		_, _ = st.DB().Exec("INSERT INTO schema_migrations (revision) VALUES (10) ON CONFLICT DO NOTHING")
	}()

	srv := httpserver.New(
		httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithStore(st),
		httpserver.WithProber(readyTestProber{err: nil}),
	)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable on schema mismatch, got %d", rec.Code)
	}

	var resp readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode readiness response: %v", err)
	}

	if len(resp.Errors) == 0 || resp.Errors[0].Code != contracts.ErrDataUnavailable {
		t.Fatalf("expected DATA_UNAVAILABLE error, got %+v", resp.Errors)
	}
	if !strings.Contains(resp.Errors[0].Message, "migrations") {
		t.Fatalf("expected message to mention migrations, got %q", resp.Errors[0].Message)
	}

	report := srv.CheckReadiness(context.Background())
	migSub, ok := report.Subsystems["migrations"]
	if !ok || migSub.Status != httpserver.StatusMismatch {
		t.Fatalf("expected report migrations SCHEMA_MISMATCH, got %+v", migSub)
	}
}


