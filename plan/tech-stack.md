# Technology stack and selection gates

2026-09-19 · Target architecture, not installed software. Decisions marked **proposed** require the named gate. Source links and verification limits: [source-register.md](source-register.md).

## Current implementation versus target

| Layer | Observed current repository | Target / decision |
| --- | --- | --- |
| Application API | Python FastAPI; `src/sthira/api/app.py` mounts `/api/v2` | **Accepted:** Go application service; standard `net/http`, `context`, `encoding/json`, `log/slog`; supported stable toolchain pinned at P1 |
| Domain/storage | Python contracts, in-memory API services; SQLAlchemy/Alembic/PostGIS scaffold | **Proposed:** PostgreSQL + PostGIS; explicit SQL transactions via pgx; versioned SQL migrations; real DB integration tests |
| Citizen client | TypeScript/Vite/MapLibre GL JS; older vanilla JS client still present | **Accepted:** Android and iPhone, locally installed assets. Framework deferred to P8 |
| Frontend candidates | No Kotlin/native app | **Proposed:** native Kotlin Android + Swift/SwiftUI iOS, or Kotlin Multiplatform shared logic with platform UI; evaluate Flutter only if it improves both-platform delivery without missing device budgets |
| Maps | Web MapLibre with remote Esri raster imagery | **Proposed:** MapLibre Native Android/iOS, restrained vector style, legal self-hosted/contracted tiles and downloadable regions; no satellite background required |
| Local storage | Browser cache/localStorage/service worker | **Proposed:** platform SQLite/Room-equivalent storage, app-private files, OS keystore/keychain for secrets. Pick exact client library in P8 |
| Place lookup | Fixed demo aliases | Curated multilingual gazetteer indexed by jurisdiction; PostgreSQL lookup and regional on-device aliases. No public geocoder per voice request |
| Middle model | Deterministic parser plus optional Azure GPT-4.1 mini explanation; legacy Bedrock adapter | **Adopted candidate (Evaluation BLOCKED / NOT_EVALUATED: hardware/GPU unavailable, O03/O11 open):** Sarvam-30B (sarvamai/sarvam-30b, Apache-2.0, 2.4B active non-embedding params in 30B MoE, FP8 quantized weights ~30 GB, non-thinking / reasoning disabled). Supersedes earlier Qwen3-4B-Instruct-2507 candidate (preserved as historical). Synthetic harness passes 20/20 |
| Middle serving | API process/client adapters | **Proposed:** vLLM / SGLang on separate private GPU workers; pinned image/model/tokenizer/quantization (FP8); single private loopback API protocol |
| ASR | IndicConformer local adapter enables Hindi/Malayalam | **Proposed candidate (Evaluation BLOCKED / NOT_EVALUATED: hardware unavailable, O03 open):** IndicConformer; verify every selected state language and separate English coverage. Loopback private worker protocol verified |
| TTS | Indic Parler-TTS local runtime; small approved-text WAV cache | **Proposed candidate (Evaluation BLOCKED / NOT_EVALUATED: hardware unavailable, O11 open):** Indic Parler-TTS; evaluate latency/intelligibility per language; pre-generate approved common audio, synthesize only validated template text |
| Static/package delivery | Web assets plus bundled fixtures | Object storage + CDN or equivalent regional cache, signed immutable packages, ETag/deltas. Provider and India hosting requirements open |
| Async work | Python in-memory outbox concepts | Start with durable PostgreSQL jobs/outbox and bounded workers; add message broker only after measured need |
| Cache | Process-local objects | HTTP/CDN cache first. Add shared Redis only for demonstrated need, never as capacity source of truth |
| Operations | Docker demo + GitHub Actions | Reproducible OCI images; CI gates; initial container deployment with real HA/failover evidence. Kubernetes only when operator/platform requirements justify it |
| Telemetry | Basic request metadata | OpenTelemetry-compatible traces and metrics, Prometheus/Grafana as proposed OSS tooling; no voice/location payloads in telemetry |
| Assurance | pytest, TypeScript build and demo checks | Go tests/race/fuzz/vet, Staticcheck, govulncheck/gosec, Trivy/Gitleaks, pprof/benchstat/k6, ZAP and scoped Strix; [assurance.md](assurance.md) |

## Why Go, and what it does not solve

Go is the user's backend choice and fits bounded concurrent I/O and small deployable services. It does not automatically make a system safe or one-million-user capable. Database contention, image/voice bytes, per-user polling, uncached tiles and model inference can dominate cost. JavaScript/TypeScript UI code normally executes on the device; switching the frontend language alone does not remove server load. Native/local assets help startup and network use, but browser apps can also cache assets; choose on measured device behavior and maintainability.

Do not translate every Python module line for line. Preserve required semantics and regression evidence. Keep Python/native code only where established ASR/TTS/vLLM dependencies require it, isolated from the Go product API. Rewriting ML runtimes in Go is outside scope.

## Model decision

On 2026-09-21, the user explicitly selected **Sarvam-30B** (`sarvamai/sarvam-30b`) in **FP8** precision as the middle model, superseding the earlier Qwen-first/~5–6B total-model preference (Decision D59). Sarvam-30B is an Apache-2.0 Mixture-of-Experts (MoE) model with 128 experts (top-6 routing), 2.4B active non-embedding parameters, and state-of-the-art coverage across 22 Indian languages. The earlier Qwen3-4B-Instruct-2507 evidence is preserved as historical.

Capacity planning for Sarvam-30B must account for total resident weights: approximately 60 GB decimal BF16 weights or approximately **30 GB in FP8** (the selected format), or 15 GB ideal 4-bit weights before scales, KV cache, activations, runtime workspace and concurrency overhead. Active parameter count (2.4B) determines compute/FLOPs per token, **NOT** resident weight memory. A single 24 GB GPU is insufficient to host the resident FP8 weights without multi-GPU tensor parallelism (e.g. 2x 24 GB or 1x 48 GB/80 GB A100/H100). Never promise that a 2.4B-active MoE model fits in memory like a 2.4B dense model.

Inference runtime must disable thinking (`enable_thinking=false` in chat template) and enforce strict structured JSON generation so internal reasoning cannot leak into the JSON action channel or exceed token budgets. Keep context near 2–4K tokens and outputs at most 256 tokens. No weights download or paid cloud inference is permitted without explicit authorization; hardware/budget gaps remain reported as BLOCKED_HARDWARE / NOT_EVALUATED.

## Selection records required

Before a dependency is installed: record purpose, alternatives, exact version/digest, license, maintenance/security posture and upgrade owner. Model records also include weights/tokenizer hash, remote-code requirements, language matrix, hardware, quantization and evaluation results. The frontend decision requires Android and iPhone prototypes of map download, offline boot, audio capture and screen-reader flow after gate B; a Kotlin preference cannot settle iOS or MapLibre integration without those checks.
