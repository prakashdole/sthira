# Worker 2 — real inference, bounded IPC, eval and P7 verification

Repository: `/Users/apple/Documents/Projects/MonitoringZ`. This is implementation, not another planning-only task. Read applicable instructions, GEMINI.md and its listed product/rules/architecture/decisions/phase documents, then `plan/reviews/review-three-workers-2026-09-21.md` and `plan/next-action-integration-repair.md`. Read relevant callers, not huge metrics or weights. Do not start P8.

Baseline: CLEAN `4095400`; repair branch `codex/p567-integration-repair` at `2e2d5c8`; model evidence `64b9ea9`; auth evidence `9c372f0`. Recheck actual refs and dirty files. Preserve all existing worktrees and unrelated edits. No main changes, pushing, reset, amend, rebase, history rewriting, shared-DB truncation, model downloads, paid inference or GPU rental. Do not modify or newly import .txt files. Commit coherent verified stages locally. Imported historical commits are not evidence of acceptance.

Two workers run concurrently in isolated worktrees. Worker 1 owns shared backend authority/contracts/orchestration and final integration; Worker 2 owns inference modules, Python adapters, eval, loadmodel and operational scripts. Do not edit the other's files. If a shared contract needs changing, send the exact proposed shape and regression requirement to Worker 1 (or put it in your handoff for the user to relay); do independent work meanwhile. No duplicate final integration or competing ledger edits. Keep source/fixture evidence concise and never commit raw verbose test logs.

Keep government source, route, stay-policy, map-license, translation and IdP approvals open without owner evidence. Fixed models: IndicConformer-600M-Multi, Sarvam-30B (2.4B active non-embedding parameters), Indic Parler-TTS. Missing implementation is internal work; missing weights/hardware only blocks real execution measurements. Synthetic runtime tests prove protocol, not model quality. No invented approvals, confidence, geography, capacities or routes.

## 1. Start independently

Create isolated `codex/inference-runtime-closure` from reviewed repair SHA **2e2d5c8** (recheck it). Do not work on CLEAN or reuse another worker's checkout. Read `plan/integration-repair-handoff.md` on this baseline, the model evidence at `64b9ea9` and original integration-repair Stages 3–5. Stage 1 public pipeline/WAV fixes are already present; preserve them. Report only your new commits for Worker 1 to integrate.

Own backend/internal/asrworker, ttsworker, middleworker, backend/eval, src/sthira_v2/speech_*adapter.py, associated Python tests/dependency metadata, loadmodel, operational scripts and your evidence files. Do not edit backend shared contracts/httpserver/orchestration/store/cmd/migrations or final ledgers. Send precise integration needs to Worker 1. Use `plan/inference-runtime-handoff.md` as your compact tracker.

## 2. Validate research before turning it into code

C's report is guidance, NOT an accepted specification. It calls mutable main a pinned commit and uses a Bhili ASR repository to describe the selected multilingual artifact. Do not substitute that model. Confirm exact selected repository, immutable revision, artifact inventory, model class/call signature, language mapping, confidence semantics and license from primary source or installed pinned code. Record inaccessible/gated evidence honestly. Its Sarvam/vLLM request-template and reasoning claims also require verification against the exact supported version. Do not blindly run or copy the illustrative probes, manually reconstructed CTC decoder, hotpatch or dynamic import.

No weights/downloads/paid compute. Small source/config documentation retrieval is in scope; use applicable research skills. Pin justified dependencies, verify provenance of locally executed model code, disable opportunistic network loading and never execute arbitrary unverified model-dir Python as trusted code.

## 3. Implement real ASR/TTS adapters and readiness

ASR still returns `[unverified:real-inference-stub]`; TTS still guesses generic AutoModel.generate(text,language,voice) and .wav. Implement the real selected model APIs using verified source. Remove fabricated transcript/confidence and invented capabilities. Preserve unknown confidence through contracts with Worker 1's coordination; never silently turn it into 1.0 or a false calibrated score.

For Indic Parler verify the dedicated class, prompt/description tokenizers, local text encoder assets, supported generate arguments, output shape and model-native sample rate. Encode audio at the real rate, not a hardcoded value that changes playback speed. Verify descriptions/voices/languages against actual supported artifacts and approved configuration; model support does not imply government language approval.

READY requires successful local initialization and a bounded valid warm-up; dummy files, import errors, malformed weights or warm-up failures must remain unready. Advertise only justified loaded capabilities. Tests must exercise actual Python entry points with injected fake libraries/models to prove load->warm->ready ordering and real invocation/result decoding; mocks are protocol evidence only. Keep an opt-in local-artifact check reporting NOT_RUN without artifacts. Implement the API even when heavyweight execution is unavailable; do not call missing implementation a GPU blocker.

Sarvam: remove speculative active templates. Bind exact tokenizer template, reasoning behavior and structured-output configuration to supported pinned serving code. Assert the actual outbound request and launch flags; don't infer that a Jinja variable disables thinking if unused. Keep 30B weight residency distinct from 2.4B active compute. No throughput/accuracy claims without measurement.

## 4. Make IPC cancellation and shutdown truly bounded

Trace both adapters' admission locks, stdin.Write, response demux, startup and Close. Deadline must cover waiting for admission and blocked writes, not begin after writing. A child that never reads with a full pipe must not hang Request or Close. Register startup correlation before the reader can deliver it. Synchronize demux errors. Malformed/duplicate/mismatched responses, cancellation and child exit must terminate/reap/reset uncertain streams without cross-request response leakage. Prefer the existing implementation with minimal fixes over another abstraction layer.

Use actual helper subprocesses, deterministic handshakes/deadlines and -race: full pipe/non-reading child, startup race, malformed/duplicate/unknown response, cancellation followed by retry, process exit, concurrent close and large valid TTS response. Verify no leaked child/readers. Run in the respective nested Go module; root module commands do not cover these.

## 5. Repair eval against actual servers

Middle provider still needs endpoint-specific schema/envelope proof: no source_version/data_version_hint in a public PipelineRequest, no flat decoding of a data envelope, no empty successful outcome with nil error. TTS must distinguish public speech from private synthesis, require actual text and versions, never invent version 1. Verify all current fields rather than changing public contracts just to fit the harness.

Build conformance using real ASR/middle/TTS server constructors with fake runtimes. Point eval at actual handlers and coordinate the public voice host with Worker 1. Hand-written servers duplicating the provider's imagined schema are insufficient. Cover actual transcript/audio/proposal, correlation, malformed 200, stale data, unsupported language, refusal, cancellation, unavailable->recovered. Classify plumbing versus real-language quality evidence explicitly.

## 6. Finish bounded P7 harnesses

Smoke: initialize and track owned process/DB resources before traps; safe cleanup when failure occurs before server startup. Unique names not just second-resolution timestamps; never drop an existing database or kill a reused PID. Seed isolated successful business data and assert semantic candidate fields, not body length. Fail readiness timeout, HTTP status 0, empty/error/malformed 200 and every required failed check. Preserve actual places schema (jurisdiction is accepted). Coordinate seed contracts with Worker 1; no production synthetic bypass. Keep short smoke and summarized metrics, not millions of lines.

Security: aggregate required NOT_RUN checks as incomplete, never overall clean PASS. Verify scanner findings exit behavior (including Trivy), tool crash versus findings, version pin/provenance and deterministic fake-tool failure propagation. Run available real scanners within scope; missing tools stay NOT_RUN. Test the harness without expensive installation unless justified/authorized. Validate SPDX metadata without invented dependencies/licenses. Keep evidence Markdown/JSON and do not modify .txt evidence or rewrite git history to remove prior raw metrics.

## 7. Verify and hand off

Per coherent stage, run relevant Go/Python regressions, formatting/vet/build and inspect diffs before local commits. Run all owned Go module suites and IPC -race tests. Supply commands, versions and actual results; tests skipped or not compiled are not passing. Record exact source revisions for model APIs, fake-vs-real inference coverage, missing hardware/approval and honest scanner/load limits. Send Worker 1 your new ordered commits and shared-contract needs, then support its final integrated conformance run. Do not declare P5/P6/P7 done or start P8. Finish all independently executable internal items, not only research; blocked heavy measurements do not excuse stubs or false readiness.
