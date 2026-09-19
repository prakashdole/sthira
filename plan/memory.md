# Project handoff memory

Updated 2026-09-19; concise planning context, not a completion certificate.

- User wants plans only in this pass: update every Markdown file under `plan/`; no Go implementation and no runtime-file deletions.
- Target immediate evacuation and 7–30 day temporary relocation; no permanent relocation, hazard/red-zone or safe-land prediction.
- Government data supplies operational truth. Route authority explicitly remains open; see O05 in [open-decisions.md](open-decisions.md).
- One million **total** users. Peak/voice traffic and monthly hosting budget still need measurement/approval.
- Android and iPhone from launch. Decide frontend after P7 backend handoff; do not silently select Android-only Kotlin.
- 10–15 demo states, 2–3 event scenarios per state, their regional languages. User provides demo zones later. Proposed states/languages: [feature.md](feature.md).
- Voice-first ASR → constrained middle model → optional TTS. Local UI/maps/fallback; no on-device AI requirement. Small labelled chat control remains accessible.
- Start model evaluation with Qwen3-4B-Instruct-2507; not yet selected by product benchmark. Preserve existing IndicConformer/Indic Parler assets as candidates; language coverage must be verified.
- Go owns application logic. Established model runtimes may remain isolated Python/native services; do not port neural network frameworks to Go.
- Repository has 206 tracked paths at inspection, including 89 Python source files. Both legacy and v2 are mounted by the current Python entry point. V2 uses in-memory application services despite persistence scaffolding; current voice flow is Hindi/Malayalam ASR with optional Azure explanation and MapLibre web UI.
- Existing completed Python slices are useful reference evidence, not completed Go stages. [changes.md](changes.md) separates them from the new P0–P12 ledger.
- Use [prompt.md](prompt.md) for the next phase. Update evidence per revision; do not inherit old PASS/DONE counts.
- Preserve pre-existing staged document moves and unrelated edits. All `.txt` files remain untouched. Detailed retirement prerequisites: [cleanup.md](cleanup.md).
