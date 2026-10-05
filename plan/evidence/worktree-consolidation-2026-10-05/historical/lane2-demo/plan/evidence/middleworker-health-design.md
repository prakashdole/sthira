# Middle-Worker /health — Design Decision Record

## Item 6 evidence

### What was observed

The worker's `/health` endpoint returns `ready:true,warm:true` with a model list even when vLLM is unreachable at `127.0.0.1:9999`. The blocker asked whether this is a write-scope defect (fix + regression test) or an honest design (document no-change).

### Finding: Config-state health, not connectivity health

The ready/warm state is determined by **configuration validation only**, not by live vLLM connectivity. This is intentional.

### LoadAndVerify gate (worker.go:159–182)

```go
func (w *Worker) LoadAndVerify(_ context.Context) error {
    if w.runtime.Revision() == "" {
        return errors.New("worker: runtime revision is empty")
    }
    name, sha := w.runtime.Digest()
    if name == "" || sha == "" {
        return errors.New("worker: runtime artifact digest is empty")
    }
    langs := w.runtime.Languages()
    if len(langs) == 0 {
        return errors.New("worker: runtime reported zero languages")
    }
    w.readyMu.Lock()
    w.warm = true
    w.ready = true
    w.readyMu.Unlock()
    // Start the worker pool. Each goroutine pulls from the bounded queue.
    for i := 0; i < w.maxInflight; i++ {
        w.wg.Add(1)
        go w.run()
    }
    return nil
}
```

The three checks are **all configuration checks**:
1. `runtime.Revision() != ""` — did the runtime report a revision string?
2. `runtime.Digest()` returns non-empty name and SHA — did the runtime report artifact identity?
3. `runtime.Languages()` reports at least one language — did the runtime report an allow-list?

**None of these probe network reachability.** If they all pass, `warm=true, ready=true` is set.

### Snapshot returns config-based model info (worker.go:325–369)

```go
func (w *Worker) Snapshot() HealthEnvelope {
    // ...
    rev := w.runtime.Revision()
    dname, dsha := w.runtime.Digest()
    langs := w.runtime.Languages()
    models := []ModelInfo(nil)
    if rev != "" {
        models = []ModelInfo{{
            ModelID:        runtimeModelID(w.runtime),
            Revision:       rev,
            ChecksumSHA256: dsha,
            // ...
        }}
    }
    return HealthEnvelope{
        Ready:  ready,
        Warm:   warm,
        Models: models,
        // ...
    }
}
```

Model list is derived from **runtime config**, not from probing vLLM.

### Connectivity failures surface at Dispatch, not at health

When a request is dispatched and vLLM is unreachable:

**client.go:319–327** — the HTTP client returns `ErrUnavailable`:
```go
resp, err := c.http.Do(req)
if err != nil {
    if ctx.Err() != nil {
        // ...
    }
    return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)  // wrapped ErrUnavailable
}
```

**server.go / handlePropose** — `ErrUnavailable` maps to `MODEL_UNAVAILABLE`:
```go
// The server maps client errors to MiddleState:
// ErrUnavailable → MODEL_UNAVAILABLE
// ErrTimeout    → TIMEOUT
// ErrCanceled   → CANCELED
// ErrMalformed  → MALFORMED
```

The worker remains `ready=true, warm=true` after any number of `ErrUnavailable` responses. The Stats() counter would increment timedOut or completed depending on the exact error path, but the health envelope is unaffected.

### Stats() counters (worker.go:378–380)

```go
func (w *Worker) Stats() (saturated, completed, timedOut, malformed uint64, inflight int64) {
    return w.saturated.Load(), w.completed.Load(), w.timedOut.Load(), w.malformed.Load(), w.inflight.Load()
}
```

**No `skipped` counter.** The blocker question referenced a skip counter that does not exist. (This aligns with Worker 21 finding — neither the worker nor the eval package has skip counting.)

### Design rationale

This is **config-state health**, not **liveness health**. The rationale:
1. **Configuration is verified at startup** — `LoadAndVerify` runs once at worker start and confirms the runtime is configured with identity (revision, digest, languages). This is a correctness gate, not a connectivity gate.
2. **Connectivity is a runtime concern** — whether the vLLM endpoint responds depends on network path, DNS, firewall, server process state. Checking every request would add latency to every health poll.
3. **The request path handles failures explicitly** — `ErrUnavailable` is a typed error that maps to `MODEL_UNAVAILABLE`, which the orchestrator handles explicitly. The worker's health is not the signal for vLLM availability.

### Verdict: NO-CHANGE — Honest Design

This is working as designed. `/health` reflects configuration validity; request failure handling is a separate concern managed at the `Dispatch` layer with typed errors.

**No fix needed. No regression test needed.** The behavior is documented here for future engineers who may expect `/health` to ping vLLM.

If a future requirement calls for liveness probing (pinging vLLM on each `/health` request or on a background interval), that would be a new capability requiring explicit design decision and a new decision record (Proposed → Accepted).
