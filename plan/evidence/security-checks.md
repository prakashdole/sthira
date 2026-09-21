Security / static check evidence
Date: 2026-09-21T11:47:45Z
Modules: backend, backend/internal/asrworker, backend/internal/ttsworker,
         backend/internal/middleworker, backend/eval, loadmodel

=== backend (backend) ===
2026-09-21T11:47:46Z | backend | go vet | PASS | go1.27.1
internal/httpserver/graceful_shutdown_test.go
internal/offlinequeue/worker.go
internal/offlinequeue/worker_regression_test.go
2026-09-21T11:47:46Z | backend | gofmt | FAIL | go1.27.1
    detail: files need formatting: internal/httpserver/graceful_shutdown_test.go internal/offlinequeue/worker.go internal/offlinequeue/worker_regression_test.go 
2026-09-21T11:47:46Z | backend | govulncheck | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | backend | gosec | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | backend | staticcheck | NOT_RUN | 
    detail: binary not installed

=== asrworker (backend/internal/asrworker) ===
2026-09-21T11:47:46Z | asrworker | go vet | PASS | go1.27.1
2026-09-21T11:47:46Z | asrworker | gofmt | PASS | go1.27.1
2026-09-21T11:47:46Z | asrworker | govulncheck | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | asrworker | gosec | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | asrworker | staticcheck | NOT_RUN | 
    detail: binary not installed

=== ttsworker (backend/internal/ttsworker) ===
2026-09-21T11:47:46Z | ttsworker | go vet | PASS | go1.27.1
2026-09-21T11:47:46Z | ttsworker | gofmt | PASS | go1.27.1
2026-09-21T11:47:46Z | ttsworker | govulncheck | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | ttsworker | gosec | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | ttsworker | staticcheck | NOT_RUN | 
    detail: binary not installed

=== middleworker (backend/internal/middleworker) ===
2026-09-21T11:47:46Z | middleworker | go vet | PASS | go1.27.1
2026-09-21T11:47:46Z | middleworker | gofmt | PASS | go1.27.1
2026-09-21T11:47:46Z | middleworker | govulncheck | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | middleworker | gosec | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:46Z | middleworker | staticcheck | NOT_RUN | 
    detail: binary not installed

=== eval (backend/eval) ===
2026-09-21T11:47:46Z | eval | go vet | PASS | go1.27.1
2026-09-21T11:47:47Z | eval | gofmt | PASS | go1.27.1
2026-09-21T11:47:47Z | eval | govulncheck | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:47Z | eval | gosec | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:47Z | eval | staticcheck | NOT_RUN | 
    detail: binary not installed

=== loadmodel (loadmodel) ===
2026-09-21T11:47:47Z | loadmodel | go vet | PASS | go1.27.1
2026-09-21T11:47:47Z | loadmodel | gofmt | PASS | go1.27.1
2026-09-21T11:47:47Z | loadmodel | govulncheck | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:47Z | loadmodel | gosec | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:47Z | loadmodel | staticcheck | NOT_RUN | 
    detail: binary not installed

2026-09-21T11:47:47Z | repo | gitleaks | NOT_RUN | 
    detail: binary not installed
2026-09-21T11:47:47Z | repo | trivy | NOT_RUN | 
    detail: binary not installed

RESULT: FAIL (see above; exit non-zero)
