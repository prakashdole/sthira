# Worker B — inference, audio transport, evaluation and P7 evidence

Send this entire file to the second new chat. Execute the work; do not merely return a plan. Run in parallel with Worker A.

## Common instructions — read before editing

Repository: `/Users/apple/Documents/Projects/MonitoringZ`. Integration branch is uppercase `CLEAN`, never main. Reviewed implementation base is `b0b9772`. This is bounded correction work on P5/P6/P7, not a new phase or rewrite. Another worker runs concurrently; obey ownership below.

First inspect status, branches, worktrees and HEAD. Read GEMINI.md and applicable instructions, then plan/prd.md, rules.md, trd.md, architecture.md, source-register.md, decisions.md, open-decisions.md, phases.md and plan.md in that order. Read relevant sections selectively, not giant outputs. Then read plan/reviews/review-b0b9772.md, plan/next-action-single-worker.md and its handoff for historical requirements; this prompt governs the current assignment. Trace callers and the actual contract before changing code. Do not read weights or raw metrics.

Use an isolated sibling worktree and a `codex/` branch from the same captured CLEAN revision containing these prompts. Record base SHA; verify b0b9772 is its ancestor. Do not work concurrently in the primary checkout. Preserve unrelated files/worktrees/staged work. Do not edit any .txt file, push, reset, amend, rewrite history or touch main. Commit coherent verified stages with only your own files. Do not globally update shared ledgers while the other lane runs. Put your evidence in your assigned Markdown handoff.

User-selected models: IndicConformer-600M-Multi ASR, Sarvam-30B (2.4B active non-embedding) middle, Indic Parler-TTS. Do not revert to Qwen or infer that active parameters determine weight memory. Model outputs propose constrained display actions; they cannot authorize routes, capacity, arrivals or emergency calls. Preserve synthetic/operational isolation and fail-closed behavior. No paid calls, rented GPU or model-weight downloads without existing explicit authorization. Missing runtime assets are NOT_RUN evidence, not successful inference and not an excuse for unimplemented integration code.

Reuse existing helpers/contracts. Fix root causes, avoid parallel schemas/frameworks. For each reproduced defect, add a regression exercising the real affected boundary, demonstrate the intended failure before the fix when practical, then verify the fix. Fake inference is allowed to test transport, clearly labelled; fake replacement HTTP handlers cannot prove actual worker/public HTTP conformance. Do not invent authoritative data, policy or translation approval to make tests green.

Use unique owned disposable test databases and ports. Apply the repository migration runner through the current revision; configure STHIRA_TEST_DSN and STHIRA_TEST_ADMIN_DSN where required, and STHIRA_PYTHONPATH for process tests. Never truncate/drop shared databases. Cleanup only resources you created. Retain bounded JSON/Markdown summaries and failures, not millions of metric lines. Preserve command exit codes (no pipelines masking test failure). Record exact revision, environment, executed/failed/skipped/uncompiled tests and unavailable checks. After three unsuccessful attempts on the same issue without new evidence, report it and continue independent work.

## Worktree and ownership

Create/reuse only your own `codex/p567-inference-corrections` branch in a sibling worktree. Own separate modules backend/internal/asrworker, middleworker, ttsworker, backend/eval, speech_asr_adapter.py and speech_tts_adapter.py with directly related Python tests/dependencies, loadmodel, and scripts for speech serving, evaluation, smoke/security/SBOM. Own plan/worker-b-corrections-handoff.md and bounded lane-specific evidence files.

Do not edit backend/cmd/sthira, backend/internal/orchestration/store/httpserver/contracts/offlinedelivery/offlinequeue, shared OpenAPI, migrations or shared plan ledgers during parallel work. A owns those and final integration. Read their public contracts now and adapt your modules to them. If a contract is truly unsatisfiable, send A the exact proposed change; do not create a second protocol. Keep cross-module tests within your lane where possible, or provide A a precise integration test requirement for its test host. No third worker is needed.

## B1 — real compressed audio decoding

Read asrworker/audio_compressed.go, audio.go and server dispatch. Reproduce with real ffmpeg-generated one-second Ogg/Opus: DecodeAudio currently returns `decode: wav data chunk truncated` because ffmpeg writes an unknown-length WAV data chunk to nonseekable stdout. Existing rejection tests do not cover successful compressed decoding.

Use a bounded decoding strategy appropriate for streaming output; reuse existing decoders and do not loosen ordinary WAV truncation validation. Enforce decoded duration, byte/sample/channel/rate bounds as well as compressed-input limits, cancellation and subprocess cleanup. Avoid unbounded ffmpeg output or temporary-file leakage. Test actual valid Ogg and WebM decode through the actual worker HTTP handler, ordinary valid WAV, truncated WAV, oversized/long decoded output, malformed input and cancellation. Assert actual usable PCM/transcription input, not merely status below 500.

## B2 — implement selected ASR/TTS adapters honestly

Read both Python adapters, Go runtime adapters, dependency pins and selected model documentation. Current Python modules have no load/inference branch and always report BLOCKED; TTS default command points to nonexistent sthira_v2.speech_tts instead of the adapter module. Implement real local-artifact loading and inference for IndicConformer-600M-Multi and Indic Parler-TTS, using the actual supported APIs and pinned compatible versions. Verify APIs from installed code or primary documentation; never invent model methods. Correct module invocation and configuration paths.

Load once per process; health ready only after successful load. Enforce language support, request/audio/text bounds, output format/sample rate, device/config errors and no raw-audio retention. Missing artifacts/dependencies should fail explicitly; do not auto-download weights. Keep protocol stdout machine-readable and diagnostics on stderr without sensitive content. Preserve zero-retention default and bounded inference failure behavior.

Test the real adapter entry points with deterministic fake model dependencies for protocol/lifecycle coverage. Separately supply an opt-in test for actual local artifacts and record NOT_RUN if absent. A fake runtime pass proves transport only. Do not claim a functional real inference branch if it is still a hardcoded refusal. If primary APIs/assets cannot be established, name the specific engineering gap rather than filling it with placeholders.

## B3 — subprocess correlation, concurrency and cancellation

Both runtime adapters currently spawn a Scanner reader goroutine then release the exchange mutex before receiving a response. Concurrent pool calls can read shared stdout and consume another request's result; canceled readers survive. TTS lacks request correlation. Fix once at the IPC boundary, reusing existing primitives: serialize the complete exchange with cancellable admission or implement the minimum fully correlated bounded dispatcher. Prefer serialization unless demonstrated throughput needs otherwise. On timeout/cancel/protocol uncertainty, safely terminate/reap/reset or otherwise prove resynchronization; never hand a late response to the next request.

Make request IDs round-trip in ASR and TTS. Reject missing/wrong IDs, extra/trailing protocol records where invalid, malformed and overlimit responses. Derive TTS response size bounds from permitted audio size plus base64/JSON overhead: current 256 KiB Scanner cap rejects valid audio beyond roughly 4.45 seconds while 12 seconds is allowed. Do not simply remove the cap.

Regressions using real subprocess transport: concurrent distinct requests get their own results under `go test -race`; blocked admission cancels; cancel then immediate retry cannot consume the first response; process exit/malformed response resets cleanly; wrong ID rejects; near-limit permitted audio succeeds and overlimit fails; shutdown leaves no live child/readers. Keep tests bounded and deterministic.

## B4 — actual worker health/wire and Sarvam serving configuration

TTS /health currently returns internal runtime_languages/inventory/metrics instead of common WorkerHealth.supported_languages and other agreed fields. Make actual servers conform to the existing backend client contract. Compare ASR/middle/TTS request/response envelopes, nested scoped context, IDs, languages, errors and audio payloads. Test actual server constructors with fake inference runtimes. Supply A exact constructor/test-host instructions; hand-written replacement servers do not prove compatibility.

Read middleworker/sarvam_config.go and all callers. Current SarvamChatTemplate is Qwen-style and only tested as a string; live requests/startup do not apply the claimed thinking controls. Use the real pinned Sarvam tokenizer/chat template and supported vLLM options. Wire constrained response and reasoning controls into the actual outbound request/serving path, with tests inspecting that path. Do not infer Sarvam behavior from Qwen names. Keep reasoning out of the action JSON and cap output consistently with backend limits.

Record model/tokenizer revision, serving version and supported quantization/hardware prerequisites. FP8 config strings are not proof of support or capacity. Keep GPU correctness/per-language accuracy/latency/concurrency benchmarks NOT_RUN until measured with authorized hardware. No download/rental to bypass this boundary.

## B5 — evaluation against actual public contracts

Read backend/eval/provider/http.go and actual contracts/pipeline.go plus httpserver voice handlers. Current provider decodes flat responses instead of data envelopes, sends invented context/case_id shapes, omits jurisdiction, and expects status/intent/actions strings instead of PipelineResponse state/validated_proposal. Existing HTTP tests copy those invented shapes.

Align each configured endpoint with its actual public or private protocol; do not silently interchange them. Public pipeline requests must carry explicitly supplied jurisdiction/current source version and real input/render fields. Decode standard errors/envelopes, correlate IDs and enforce bounded strict reads. No fallback invented jurisdiction/version. Interpret validated actions and audio without translating a refusal into model-quality success.

Replace false-green provider tests with actual handler conformance using a small real-backend test host coordinated with A. Fake inference only; use real scoped context and DB where the handler requires them. Cover valid text/voice response, unavailable worker, wrong language, stale context, denied/missing scope, bad envelope and cancellation. Keep simulated-runtime evidence separate from real language/model evaluation. Provide a runnable command and config for later real local model runs, not fabricated accuracy numbers.

## B6 — bounded business smoke and honest security evidence

Inspect loadmodel/k6/smoke_real.js and scripts/run_k6_smoke_real.sh. Existing smoke mostly probes health and invalid inputs; checks lack failure thresholds and status<500 accepts transport status 0. Add one bounded successful business flow against the real seeded backend (for example the existing isolated stay/publication flow), and explicit expected statuses/response invariants. Respect authority gates: isolated test fixtures cannot become production defaults. Add k6 check/failure thresholds so broken responses or connection failure make the command fail. Do not generate another huge raw metrics dump.

Use dynamically allocated/verified owned ports, initialized cleanup state and ownership flags, startup readiness wait with timeout, and cleanup only owned processes/databases. Test startup failure and preexisting-port/resource cases without killing unrelated services. Preserve exit codes and compact output. If A's lifecycle fixes are needed for a publication flow, finish independent smoke harness work now and identify the integration command for A.

Inspect scripts/run_security_checks.sh and SBOM generation. Pin/reproducibly identify tool versions, configure scanner exit behavior for findings, and distinguish absent tool, scanner crash, findings and clean result. A NOT_RUN scanner must not produce an overall clean security claim. Verify the script's failure propagation with bounded deterministic fixtures. Do not broaden this into an unrequested full security audit or install paid tooling.

Fix the reviewed gofmt issue in ttsworker/worker.go; A handles offlinequeue formatting. Preserve plan/evidence/security-checks.txt (it records FAIL); write current evidence as Markdown/JSON. Validate SPDX output structure and real dependency identities; remove fictitious root-module proxy URL/empty-version metadata or use a suitable existing maintained generator. Do not invent licenses, package versions or completed scans. Actual vulnerability scans need the pinned tool/database; record missing prerequisites honestly.

## Verification, handoff and stop

Run gofmt, go vet, go build and tests for all five owned modules. Run IPC race tests and relevant Python adapter tests. Use actual-server protocol/evaluation regressions and the bounded smoke when prerequisites are available. No GPU/model quality claims from fake runtimes. Record each row B1–B6 with exact test/command, revision and PASS/FAIL/BLOCKED/NOT_RUN; attach compact evidence and actual dependencies needed for remaining checks.

Commit coherent verified stages on your own branch. Supply A the ordered commit IDs, base SHA, cross-boundary contract notes, tests it must run after integration and any unverified claims. Do not cherry-pick A's work or edit its files during parallel execution. A integrates and reconciles shared ledgers after both lanes finish. Keep your worktree intact. Stop before P8; do not mark P6/P7 or gate B complete merely because component tests pass.
