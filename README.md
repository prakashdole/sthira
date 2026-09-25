# Sthira

Citizen-facing emergency-guidance bridge. Reads authoritative government
sources (alerts, red zones, safe zones, routes, facilities, allocation policy)
and presents an understandable, accessible, action-ready view to the citizen.
Does not predict hazards, allocate land, or authorize routes — those are owned
by government systems and reproduced from their published data only.

See `plan/prd.md` for the product surface and `plan/round-two-demo.md` for the
September 28–29 demo plan.

## Active backend

The active product backend is the **Go service** in `backend/`, serving the
frozen `/api/v3` contract (`backend/contracts/openapi.yaml`). The Python code
under `src/sthira_v2/` and the legacy `src/sthira/` module are **retained as
reference evidence and as the runtime adapters for the three selected AI/ML
models** (ASR, constrained middle model, TTS). Python is not the active API;
do not extend it for new product behavior.

## Repo layout

- `backend/` — Go service, `/api/v3` contract, durable PostgreSQL/PostGIS
  store, voice orchestration, offline publication/queue, scenario
  preparation. Pinned Go 1.27.1; standard-library-only P1 slice; pgx/PostGIS
  for the durable slice. Run with `cd backend && STHIRA_DATABASE_DSN=... go
  run ./cmd/sthira`.
- `frontend/v2/` — TypeScript/Vite/MapLibre GL JS reference/demo UI.
  Round-two prototype (laptop + phone responsive). Not a P8 production
  client; P8 selects the Android/iOS framework at Gate B.
- `src/sthira_v2/` — Python reference + retained ML adapter implementations
  (ASR/TTS/middle workers). Used by `backend/internal/asrworker`,
  `ttsworker`, `middleworker`, and `backend/eval`. No new product behavior.
- `fixtures/` — explicitly synthetic exercise fixtures (Waynad, Idukki,
  Alappuzha, Uttarakhand, etc.). Labelled `SYNTHETIC_DEMO`. Never presented
  as real authority.
- `plan/` — governing product, architecture, rules, decisions, contracts,
  current execution playbook (`plan/prompt.md`) and round-two demo plan.
- `tools/` and `deploy/` — operational tooling (scenario preparation CLI,
  Dockerized backend prototype).

## Status

- Round-two demo priority through 2026-09-29 (see `plan/round-two-demo.md`).
- Backend Gate B (full P7 closure) is NOT_READY; the integration gates and
  remaining P0–P7 work are recorded in `plan/prompt.md` §6–7.
- P8 (Android + iPhone) and P9 (regional whole-system readiness) are gated on
  Gate B and explicit user authorization.

## Don't

- Do not modify `.txt` files unless explicitly instructed.
- Do not extend the legacy permanent-relocation Python product (`src/sthira/`).
- Do not enable live government integrations until source activation
  (`backend/internal/sourceact`) reaches OPERATIONAL with verified identity.
- Do not download model weights, rent GPUs, or spend money without explicit
  authorization. Real-model execution is BLOCKED_HARDWARE until approved
  hardware is wired and reported.
- Do not push, publish, deploy or rewrite history without user authorization.