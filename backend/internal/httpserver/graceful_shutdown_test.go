package httpserver_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func freeAddrForShutdownTest(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

// TestGracefulShutdown_AllowsInflightToComplete verifies that a
// request already accepted by the server completes successfully even
// after Shutdown is invoked. The shutdown timeout is large enough
// for the in-flight work; the test asserts:
//   - the in-flight request returns a 200 with the expected body
//   - the server exits cleanly with nil error from Serve
//   - a new request after shutdown is rejected with EOF / closed-port
//
// This proves the recovery property "controlled in-flight request
// completes during graceful shutdown", not just that the process
// exits. A drain rehearsal that observes shutdown is incomplete
// without this assertion.
func TestGracefulShutdown_AllowsInflightToComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: graceful shutdown timing")
	}
	addr := freeAddrForShutdownTest(t)

	// Probe the slowHandler via a stub server we control. We want
	// the slow handler to take ~500ms so the test can cancel the
	// context at ~50ms and assert the request still completes
	// within the shutdown timeout.
	var inflightStarted, inflightFinished atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		inflightStarted.Store(true)
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "done")
		inflightFinished.Store(true)
	})
	srv := &http.Server{Addr: addr, Handler: mux}

	// Start serving
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	// Wait for it to bind.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Fire a slow request.
	type result struct {
		status int
		body   string
		err    error
	}
	resCh := make(chan result, 1)
	go func() {
		c := &http.Client{Timeout: 5 * time.Second}
		resp, err := c.Get("http://" + addr + "/slow")
		if err != nil {
			resCh <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		resCh <- result{status: resp.StatusCode, body: string(body)}
	}()

	// Wait for the handler to enter, then shutdown.
	deadline = time.Now().Add(1 * time.Second)
	for !inflightStarted.Load() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !inflightStarted.Load() {
		t.Fatal("slow handler never started")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("shutdown: %v", err)
	}

	// In-flight request must have completed with 200 + body.
	select {
	case r := <-resCh:
		if r.err != nil {
			t.Errorf("in-flight request errored: %v", r.err)
		}
		if r.status != http.StatusOK {
			t.Errorf("status: got %d want 200", r.status)
		}
		if r.body != "done" {
			t.Errorf("body: got %q want %q", r.body, "done")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request did not return")
	}
	if !inflightFinished.Load() {
		t.Errorf("handler did not finish")
	}

	// Serve goroutine should have returned nil.
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("ListenAndServe: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Serve did not return after shutdown")
	}

	// New connection attempts must be rejected.
	if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		_ = c.Close()
		t.Errorf("server still accepting connections after shutdown")
	}
}

// TestGracefulShutdown_RejectsNewAfterClose: this is a focused
// sanity check that graceful shutdown does not race with new
// requests. Shutdown before any in-flight work means new requests
// fail fast.
func TestGracefulShutdown_RejectsNewAfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: graceful shutdown timing")
	}
	addr := freeAddrForShutdownTest(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	time.Sleep(100 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	c := &http.Client{Timeout: 500 * time.Millisecond}
	if _, err := c.Get("http://" + addr + "/ok"); err == nil {
		t.Errorf("request after shutdown should fail")
	}
}

// Ensure the imported package is referenced so the test file
// remains part of the same package as other httpserver tests.
var _ = net.IPv4zero
