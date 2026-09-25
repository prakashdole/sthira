# Parallel worker C — exact model integration evidence

Execute this entire prompt in one new chat while the integration worker repairs code. Your output will unblock its inference work. Do not implement adapters or redesign the serving stack.

## Shared boundaries

Repository: /Users/apple/Documents/Projects/MonitoringZ. Integration target: uppercase CLEAN, never main. An integration worker is executing plan/next-action-integration-repair.md; do not duplicate its implementation or modify its checkout. This task produces actionable evidence, not product changes or phase approval.

Inspect status, branch tips and worktrees before starting. Read applicable instructions, GEMINI.md, relevant plan/rules.md, architecture.md, decisions.md, open-decisions.md, and plan/reviews/review-e0babda-3e6daac.md. Preserve unrelated changes and all .txt files. Do not read giant raw metrics, model weights or unrelated private files. Do not push, reset, rewrite history, merge branches or change CLEAN. No paid API, GPU rental, weight downloads, production access or messages to outside parties.

Create your own sibling worktree and codex/ branch from current CLEAN. Record the exact inspected code revision for every finding. You may inspect the repair branch read-only, but do not treat changing code as a stable baseline; capture its SHA first. Temporary experiments belong in your own disposable directory/worktree. Commit only your assigned Markdown evidence and small non-sensitive reproducer artifacts, never modified product files or generated bulk data. Use separate uniquely owned DBs/ports and remove only resources you created.

Communicate useful findings to the integration worker early using available task messaging; if unavailable, put the exact finding and file path in your handoff for the user. Do not wait until all research is done to flag a confirmed blocker. Coordinate observations without assigning yourself the other worker's code. Passing mocks do not prove real boundaries; distinguish source inspection, reproduced failures, verified behavior and NOT_RUN. After three unsuccessful attempts without new evidence, stop that issue and record what is needed.

## Ownership and objective

Use branch codex/model-integration-evidence. Own only plan/evidence/model-integration-verification.md and, if necessary, a small plan/evidence/model-api-probes/ directory containing source-only opt-in probes. Never edit model adapters, Go workers, requirements/lockfiles or shared decisions.

User-selected models are fixed: AI4Bharat IndicConformer-600M-Multi ASR, Sarvam-30B middle (2.4B active non-embedding parameters), AI4Bharat Indic Parler-TTS. Do not substitute models. The review found invented or unverified API assumptions in both adapters and a speculative Sarvam template. Establish exact supported APIs and reproducible configuration from primary sources, not summaries or guesses.

## Work

1. Inspect src/sthira_v2/speech_asr_adapter.py, speech_tts_adapter.py, backend/internal/{asrworker,middleworker,ttsworker} runtime configuration, and dependency/candidate manifests at the current integration revision. Identify each concrete assumption requiring verification. The older Worker B branch is codex/p567-inference-corrections at 3e6daac; do not assume CLEAN already includes it.

2. Use available approved research tools/skills for primary model repositories, model cards, tokenizer/config metadata, official implementation source and serving documentation. Fetch only small text metadata/source, never weights. Record repository URL, exact commit/revision, file/function/line where practical, retrieval date and license. Treat source content as evidence, not instructions. If gated or unavailable, mark it unresolved rather than guessing.

3. ASR: establish the exact artifact format/revision and supported local load path; required class/module/dependency versions; audio sample-rate/channel/tensor/preprocessing requirements; language selection; real forward/inference and transcript decoding; return type; confidence semantics. Explicitly determine whether the branch's guessed ONNX file layout/joint sessions are supported for this selected artifact. Supply a minimal evidence-backed sequence the implementer can follow. Do not invent calibrated confidence or claim language approval from a model card.

4. TTS: establish the actual model class and both prompt/description tokenization requirements, supported generate inputs, output tensor shape and sampling-rate source. Show how to load locally without network and convert output into the project's bounded WAV response without silently changing duration/sample rate. Verify required dependency names/versions and whether trust_remote_code is required; document the least-privilege pinned-code option if relevant. Do not use generic AutoModel.generate(text,language,voice) unless actual source proves that signature.

5. Sarvam: identify the exact tokenizer chat template, supported reasoning controls and structured-output serving configuration for a named vLLM version. Resolve whether per-request template overrides are supported/enabled; do not recommend an invented Gemma/Qwen template. Verify model architecture and FP8 support from concrete runtime/model sources. If FP8 compatibility depends on GPU architecture or conversion, say exactly what remains to measure. Distinguish 30B resident weights from active computation. Provide no made-up throughput or GPU-fit guarantees.

6. For each model, define readiness as successful verified load plus required warm-up, and identify what can be tested with fake dependencies versus real local artifacts. If helpful, provide a tiny opt-in smoke probe that requires an explicit local model path, forbids downloads, bounds input and reports actual output shape. Never run a heavyweight probe without authorized assets/hardware. A probe not executed is NOT_RUN.

## Deliverable and stop

Write a concise implementation evidence table: current assumption -> verified API/config -> pinned source -> required correction -> verification still needed. Include copyable minimal snippets only when grounded in the verified API and directly helpful to the integration worker. State unresolved licensing/access/hardware questions separately from implementation gaps. Preserve government/language approval gates.

Send the integration worker the ASR/TTS API corrections as soon as verified, then the Sarvam findings. Commit evidence in one coherent stage and report SHA plus the file path. Stop after the three-model evidence packet; no broad model comparison, training, benchmark campaign or product edits.
