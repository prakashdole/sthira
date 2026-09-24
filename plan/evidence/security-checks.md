Security / static check evidence
Date: 2026-09-24T11:00:38Z
Modules: backend, backend/internal/asrworker, backend/internal/ttsworker,
         backend/internal/middleworker, backend/eval, loadmodel

=== backend (backend) ===
2026-09-24T11:00:38Z | backend | go vet | PASS | go1.27.1
2026-09-24T11:00:38Z | backend | gofmt | PASS | go1.27.1
No vulnerabilities found.
2026-09-24T11:00:39Z | backend | govulncheck | PASS | Go: go1.27.1 (/Users/apple/go/bin/govulncheck)
    detail: path=/Users/apple/go/bin/govulncheck
[gosec] 2026/09/24 16:30:39 Including rules: default
[gosec] 2026/09/24 16:30:39 Excluding rules: default
[gosec] 2026/09/24 16:30:39 Including analyzers: default
[gosec] 2026/09/24 16:30:39 Excluding analyzers: default
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineresources
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/templates
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinepkg
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/cmd/eval-run
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/sthira
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/drills
[gosec] 2026/09/24 16:30:39 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinequeue
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration
[gosec] 2026/09/24 16:30:40 Checking package: offlineresources
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineresources/deps.go
[gosec] 2026/09/24 16:30:40 Checking package: offlinepkg
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinepkg/canonical.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineresources/descriptor.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinepkg/parse.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinepkg/trust.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinepkg/types.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinepkg/validate.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineresources/pack.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineresources/style.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineresources/types.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineresources/validator.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker
[gosec] 2026/09/24 16:30:40 Checking package: contracts
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/context.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/envelope.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/errors.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/sthira-exercise
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/corpus
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/geometry.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/model.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/pipeline.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/scoped.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/transcription.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/tts.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/contracts/worker_health.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/report
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/capfeed
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/scenarioprep
[gosec] 2026/09/24 16:30:40 Checking package: httpserver
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/accesslog.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/auth.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/config.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/crashhook_disabled.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/handlers.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/observability.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/operator.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/operator_handlers.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/pprof.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/publication_adapter.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/ratelimit.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/server.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/stay_handlers.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpserver/voice_process.go
[gosec] 2026/09/24 16:30:40 Checking package: offlinequeue
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinequeue/dispatcher.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinequeue/queue.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinequeue/store.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinequeue/worker.go
[gosec] 2026/09/24 16:30:40 Checking package: main
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/sthira/main.go
[gosec] 2026/09/24 16:30:40 Checking package: store
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/audit.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/choice.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/context.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/expiry.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/idempotency.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/operatorgrant.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/place.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/publication.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/publisher.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/readiness.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/reservation.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/scoped.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/session.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/source.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval
[gosec] 2026/09/24 16:30:40 Checking package: orchestration
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/admission.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/stay.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/audio_meta.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/correlation.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/doc.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/http_client.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/metrics.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/model_strict.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/orchestrator.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/store/store.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpjson
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/sourceact
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/registry.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/synthesize.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/orchestrationtest
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/types.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/workers.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/runner
[gosec] 2026/09/24 16:30:40 Checking package: capfeed
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/capfeed/cap.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/capfeed/lifecycle.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/capfeed/transport.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/catalogue
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineclient
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/opkg
[gosec] 2026/09/24 16:30:40 Checking package: scenarioprep
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/scenarioprep/bundle.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/scenarioprep/index.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/scenarioprep/path.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/scenarioprep/prepare.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/scenarioprep/validate.go
[gosec] 2026/09/24 16:30:40 Checking package: main
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/sthira-exercise/main.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/scenario-prep
[gosec] 2026/09/24 16:30:40 Checking package: httpjson
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/httpjson/decode.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/provider
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/eval
[gosec] 2026/09/24 16:30:40 Checking package: sourceact
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/sourceact/activation.go
[gosec] 2026/09/24 16:30:40 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinedelivery
[gosec] 2026/09/24 16:30:40 Checking package: opkg
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/opkg/package.go
[gosec] 2026/09/24 16:30:40 Checking package: catalogue
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/catalogue/catalogue.go
[gosec] 2026/09/24 16:30:40 Checking package: orchestrationtest
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/orchestration/orchestrationtest/fakes.go
[gosec] 2026/09/24 16:30:40 Checking package: main
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/scenario-prep/bundle.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/scenario-prep/init.go
[gosec] 2026/09/24 16:30:40 Checking package: offlineclient
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineclient/client.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineclient/errors.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineclient/storage.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineclient/sync.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlineclient/transport.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/scenario-prep/main.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/scenario-prep/report.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/cmd/scenario-prep/validate.go
[gosec] 2026/09/24 16:30:40 Checking package: offlinedelivery
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinedelivery/cache.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinedelivery/handler.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinedelivery/range.go
[gosec] 2026/09/24 16:30:40 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/offlinedelivery/types.go
Results:


[1;36mSummary:[0m
  Gosec  : dev
  Files  : 96
  Lines  : 25020
  Nosec  : 39
  Issues : [1;32m0[0m
2026-09-24T11:00:40Z | backend | gosec | PASS | Version: dev (/Users/apple/go/bin/gosec)
    detail: path=/Users/apple/go/bin/gosec

2026-09-24T11:00:41Z | backend | staticcheck | PASS | staticcheck 2026.2.1 (0.8.1) (/Users/apple/go/bin/staticcheck)
    detail: path=/Users/apple/go/bin/staticcheck

=== asrworker (backend/internal/asrworker) ===
2026-09-24T11:00:41Z | asrworker | go vet | PASS | go1.27.1
2026-09-24T11:00:41Z | asrworker | gofmt | PASS | go1.27.1
No vulnerabilities found.
2026-09-24T11:00:42Z | asrworker | govulncheck | PASS | Go: go1.27.1 (/Users/apple/go/bin/govulncheck)
    detail: path=/Users/apple/go/bin/govulncheck
[gosec] 2026/09/24 16:30:42 Including rules: default
[gosec] 2026/09/24 16:30:42 Excluding rules: default
[gosec] 2026/09/24 16:30:42 Including analyzers: default
[gosec] 2026/09/24 16:30:42 Excluding analyzers: default
[gosec] 2026/09/24 16:30:42 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker
[gosec] 2026/09/24 16:30:42 Checking package: asrworker
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/audio.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/audio_compressed.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/doc.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/manifest.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/runtime.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/runtime_adapter.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/runtime_ipc.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/server.go
[gosec] 2026/09/24 16:30:42 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/asrworker/worker.go
Results:


[1;36mSummary:[0m
  Gosec  : dev
  Files  : 9
  Lines  : 3033
  Nosec  : 7
  Issues : [1;32m0[0m
2026-09-24T11:00:42Z | asrworker | gosec | PASS | Version: dev (/Users/apple/go/bin/gosec)
    detail: path=/Users/apple/go/bin/gosec

2026-09-24T11:00:42Z | asrworker | staticcheck | PASS | staticcheck 2026.2.1 (0.8.1) (/Users/apple/go/bin/staticcheck)
    detail: path=/Users/apple/go/bin/staticcheck

=== ttsworker (backend/internal/ttsworker) ===
2026-09-24T11:00:42Z | ttsworker | go vet | PASS | go1.27.1
2026-09-24T11:00:42Z | ttsworker | gofmt | PASS | go1.27.1
No vulnerabilities found.
2026-09-24T11:00:43Z | ttsworker | govulncheck | PASS | Go: go1.27.1 (/Users/apple/go/bin/govulncheck)
    detail: path=/Users/apple/go/bin/govulncheck
[gosec] 2026/09/24 16:30:43 Including rules: default
[gosec] 2026/09/24 16:30:43 Excluding rules: default
[gosec] 2026/09/24 16:30:43 Including analyzers: default
[gosec] 2026/09/24 16:30:43 Excluding analyzers: default
[gosec] 2026/09/24 16:30:43 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker
[gosec] 2026/09/24 16:30:43 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/templates
[gosec] 2026/09/24 16:30:43 Checking package: templates
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/templates/errors.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/templates/templates.go
[gosec] 2026/09/24 16:30:43 Checking package: ttsworker
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/audio.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/cache.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/doc.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/hash.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/inventory.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/runtime.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/runtime_adapter.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/runtime_ipc.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/server.go
[gosec] 2026/09/24 16:30:43 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker/worker.go
Results:


[1;36mSummary:[0m
  Gosec  : dev
  Files  : 12
  Lines  : 3217
  Nosec  : 4
  Issues : [1;32m0[0m
2026-09-24T11:00:43Z | ttsworker | gosec | PASS | Version: dev (/Users/apple/go/bin/gosec)
    detail: path=/Users/apple/go/bin/gosec

2026-09-24T11:00:44Z | ttsworker | staticcheck | PASS | staticcheck 2026.2.1 (0.8.1) (/Users/apple/go/bin/staticcheck)
    detail: path=/Users/apple/go/bin/staticcheck

=== middleworker (backend/internal/middleworker) ===
2026-09-24T11:00:44Z | middleworker | go vet | PASS | go1.27.1
2026-09-24T11:00:44Z | middleworker | gofmt | PASS | go1.27.1
No vulnerabilities found.
2026-09-24T11:00:45Z | middleworker | govulncheck | PASS | Go: go1.27.1 (/Users/apple/go/bin/govulncheck)
    detail: path=/Users/apple/go/bin/govulncheck
[gosec] 2026/09/24 16:30:45 Including rules: default
[gosec] 2026/09/24 16:30:45 Excluding rules: default
[gosec] 2026/09/24 16:30:45 Including analyzers: default
[gosec] 2026/09/24 16:30:45 Excluding analyzers: default
[gosec] 2026/09/24 16:30:45 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker
[gosec] 2026/09/24 16:30:45 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/eval
[gosec] 2026/09/24 16:30:45 Checking package: middleworker
[gosec] 2026/09/24 16:30:45 Checking package: main
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/client.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/eval/main.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/decode.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/doc.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/runtime.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/runtime_http.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/runtime_stub.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/runtime_subprocess.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/sarvam_config.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/server.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/wire.go
[gosec] 2026/09/24 16:30:45 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/internal/middleworker/worker.go
Results:


[1;36mSummary:[0m
  Gosec  : dev
  Files  : 12
  Lines  : 2214
  Nosec  : 0
  Issues : [1;32m0[0m
2026-09-24T11:00:45Z | middleworker | gosec | PASS | Version: dev (/Users/apple/go/bin/gosec)
    detail: path=/Users/apple/go/bin/gosec

2026-09-24T11:00:45Z | middleworker | staticcheck | PASS | staticcheck 2026.2.1 (0.8.1) (/Users/apple/go/bin/staticcheck)
    detail: path=/Users/apple/go/bin/staticcheck

=== eval (backend/eval) ===
2026-09-24T11:00:45Z | eval | go vet | PASS | go1.27.1
2026-09-24T11:00:45Z | eval | gofmt | PASS | go1.27.1
No vulnerabilities found.
2026-09-24T11:00:46Z | eval | govulncheck | PASS | Go: go1.27.1 (/Users/apple/go/bin/govulncheck)
    detail: path=/Users/apple/go/bin/govulncheck
[gosec] 2026/09/24 16:30:46 Including rules: default
[gosec] 2026/09/24 16:30:46 Excluding rules: default
[gosec] 2026/09/24 16:30:46 Including analyzers: default
[gosec] 2026/09/24 16:30:46 Excluding analyzers: default
[gosec] 2026/09/24 16:30:46 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/provider
[gosec] 2026/09/24 16:30:46 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/runner
[gosec] 2026/09/24 16:30:46 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/report
[gosec] 2026/09/24 16:30:46 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/cmd/eval-run
[gosec] 2026/09/24 16:30:46 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/corpus
[gosec] 2026/09/24 16:30:46 Import directory: /Users/apple/Documents/Projects/MonitoringZ/backend/eval
[gosec] 2026/09/24 16:30:46 Checking package: evalroot
[gosec] 2026/09/24 16:30:46 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/doc.go
[gosec] 2026/09/24 16:30:47 Checking package: corpus
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/corpus/digest.go
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/corpus/load.go
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/corpus/schema.go
[gosec] 2026/09/24 16:30:47 Checking package: report
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/report/report.go
[gosec] 2026/09/24 16:30:47 Checking package: provider
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/provider/b64.go
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/provider/deterministic.go
[gosec] 2026/09/24 16:30:47 Checking package: runner
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/runner/runner.go
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/runner/testdata.go
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/provider/http.go
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/provider/provider.go
[gosec] 2026/09/24 16:30:47 Checking package: main
[gosec] 2026/09/24 16:30:47 Checking file: /Users/apple/Documents/Projects/MonitoringZ/backend/eval/cmd/eval-run/main.go
Results:


[1;36mSummary:[0m
  Gosec  : dev
  Files  : 12
  Lines  : 2986
  Nosec  : 2
  Issues : [1;32m0[0m
2026-09-24T11:00:47Z | eval | gosec | PASS | Version: dev (/Users/apple/go/bin/gosec)
    detail: path=/Users/apple/go/bin/gosec

2026-09-24T11:00:47Z | eval | staticcheck | PASS | staticcheck 2026.2.1 (0.8.1) (/Users/apple/go/bin/staticcheck)
    detail: path=/Users/apple/go/bin/staticcheck

=== loadmodel (loadmodel) ===
2026-09-24T11:00:47Z | loadmodel | go vet | PASS | go1.27.1
2026-09-24T11:00:47Z | loadmodel | gofmt | PASS | go1.27.1
No vulnerabilities found.
2026-09-24T11:00:48Z | loadmodel | govulncheck | PASS | Go: go1.27.1 (/Users/apple/go/bin/govulncheck)
    detail: path=/Users/apple/go/bin/govulncheck
[gosec] 2026/09/24 16:30:48 Including rules: default
[gosec] 2026/09/24 16:30:48 Excluding rules: default
[gosec] 2026/09/24 16:30:48 Including analyzers: default
[gosec] 2026/09/24 16:30:48 Excluding analyzers: default
[gosec] 2026/09/24 16:30:48 Import directory: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/cmd/dummyd
[gosec] 2026/09/24 16:30:48 Import directory: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/cmd/loadtestd
[gosec] 2026/09/24 16:30:48 Import directory: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/loadfixtures
[gosec] 2026/09/24 16:30:48 Import directory: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/loadfixtures/dummyd
[gosec] 2026/09/24 16:30:48 Import directory: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/loadfixtures/internal
[gosec] 2026/09/24 16:30:48 Import directory: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/voice/voiceload
[gosec] 2026/09/24 16:30:48 Import directory: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/cmd/voiceloadd
[gosec] 2026/09/24 16:30:48 Checking package: internal
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/loadfixtures/internal/util.go
[gosec] 2026/09/24 16:30:48 Checking package: main
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/cmd/loadtestd/main.go
[gosec] 2026/09/24 16:30:48 Checking package: loadfixtures
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/loadfixtures/server.go
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/loadfixtures/store.go
[gosec] 2026/09/24 16:30:48 Checking package: main
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/cmd/dummyd/main.go
[gosec] 2026/09/24 16:30:48 Checking package: main
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/cmd/voiceloadd/main.go
[gosec] 2026/09/24 16:30:48 Checking package: dummyd
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/loadfixtures/dummyd/dummy_worker.go
[gosec] 2026/09/24 16:30:48 Checking package: voiceload
[gosec] 2026/09/24 16:30:48 Checking file: /Users/apple/Documents/Projects/MonitoringZ/loadmodel/voice/voiceload/voiceload.go
Results:


[1;36mSummary:[0m
  Gosec  : dev
  Files  : 8
  Lines  : 1581
  Nosec  : 7
  Issues : [1;32m0[0m
2026-09-24T11:00:48Z | loadmodel | gosec | PASS | Version: dev (/Users/apple/go/bin/gosec)
    detail: path=/Users/apple/go/bin/gosec

2026-09-24T11:00:48Z | loadmodel | staticcheck | PASS | staticcheck 2026.2.1 (0.8.1) (/Users/apple/go/bin/staticcheck)
    detail: path=/Users/apple/go/bin/staticcheck

[90m4:44PM[0m [32mINF[0m [1m409 commits scanned.[0m
[90m4:44PM[0m [32mINF[0m [1mscanned ~2294810376 bytes (2.29 GB) in 3m52s[0m
[90m4:44PM[0m [32mINF[0m [1mno leaks found[0m
2026-09-24T11:14:15Z | repo | gitleaks | PASS | gitleaks version version is set by build process (/Users/apple/go/bin/gitleaks)
    detail: path=/Users/apple/go/bin/gitleaks

Report Summary

┌──────────────────────────────────────┬───────┬─────────────────┬─────────┐
│                Target                │ Type  │ Vulnerabilities │ Secrets │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ backend/eval/go.mod                  │ gomod │        0        │    -    │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ backend/go.mod                       │ gomod │        0        │    -    │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ backend/internal/asrworker/go.mod    │ gomod │        0        │    -    │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ backend/internal/middleworker/go.mod │ gomod │        0        │    -    │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ backend/internal/ttsworker/go.mod    │ gomod │        0        │    -    │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ deploy/go-backend/migrate/go.mod     │ gomod │        0        │    -    │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ frontend/v2/package-lock.json        │  npm  │        0        │    -    │
├──────────────────────────────────────┼───────┼─────────────────┼─────────┤
│ loadmodel/go.mod                     │ gomod │        0        │    -    │
└──────────────────────────────────────┴───────┴─────────────────┴─────────┘
Legend:
- '-': Not scanned
- '0': Clean (no security findings detected)
2026-09-24T11:14:19Z | repo | trivy | PASS | 2026-09-24T16:44:15+05:30	INFO	Loaded	file_path="trivy.yaml" (/opt/homebrew/bin/trivy)
    detail: rc0, no findings table entries; path=/opt/homebrew/bin/trivy
2026-09-24T11:14:19Z | repo | spdx-validate | PASS | python3
    detail: validated 6 SBOM documents

RESULT: PASS (every required check ran and passed)
