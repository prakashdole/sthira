# Phase P10 Evidence — Obsolete File Retirement and Final Artifact Verification

**Phase**: P10 — Retire obsolete files and verify the final artifact  
**Owner**: Coordinator (Repository Optimization & Asset Retirement)  
**Base Commit**: `3884141` (Q04 ledger completion) on `CLEAN`  
**Host Environment**: macOS 15.x (Darwin arm64, Apple M2), Go 1.27.1, Swift 6.4, Node v26.8.1, Python 3.11.

---

## 1. Summary of Actions & Asset Retirement

In accordance with Phase P10 specifications (`plan/prompt.md` lines 1469–1493 and `plan/cleanup.md`), all superseded legacy assets from prior v1 hackathon milestones (permanent relocation, land truth, candidate-site screening, beneficiary casework, retired v1 UI, and unused cloud LLM adapters) have been formally retired from the codebase:

### A. Retired Superseded Modules & Files (102 files removed)
1. **Retired Legacy v1 Web Client**:
   - `frontend/app.js` (4,360 lines of legacy DOM manipulation)
   - `frontend/index.html` (legacy HTML)
   - `frontend/styles.css` (legacy styles)
   - `frontend/tokens.css` (legacy tokens)
2. **Retired Superseded v1 Python Application (`src/sthira/`)**:
   - `src/sthira/api/` (`app.py`, `middleware.py`, `__init__.py`)
   - `src/sthira/core/` (`audit.py`, `contracts.py`, `enums.py`, `errors.py`, `identity.py`, `localization.py`, `outbox.py`)
   - `src/sthira/modules/` (19 modules: `adaptation`, `allocation`, `catalog`, `district_scale`, `evaluation`, `field`, `governance`, `hazard`, `household`, `land_truth`, `live_ops`, `policy`, `programme`, `reconstruction`, `reporting`, `resilience`, `scaling`, `security`, `source_access`)
   - `src/sthira/spikes/` (`fixture_loader.py`)
   - `src/sthira/__init__.py`
3. **Retired Superseded Cloud AI Spikes**:
   - `src/sthira_v2/azure_openai.py` (superseded by self-hosted Sarvam-30B MoE, D59)
   - `src/sthira_v2/nemotron.py` (superseded by self-hosted Sarvam-30B MoE, D59)
4. **Retired Obsolete v1 Test Suites & Helpers**:
   - `tests/authutil.py`, `tests/conftest.py`
   - `tests/test_api.py`, `tests/test_api_phase*.py` (phases 9–14)
   - `tests/test_phase*.py` (phases 2, 3, 4, 6, 9, 10, 11, 12, 13, 14)
   - `tests/test_scaling_and_adaptation.py`, `tests/test_wave_a.py`, `tests/test_wave_b.py`
   - `tests/test_core_contracts.py`, `tests/test_fixtures.py`, `tests/test_p0_security.py`
   - `tests/test_v2_azure_openai.py`, `tests/test_v2_nemotron.py`
   - Legacy FastAPI mounting tests (`test_v2_assignment_api.py`, `test_v2_cap.py`, `test_v2_chat.py`, `test_v2_demo_scenario_api.py`, `test_v2_openapi.py`, `test_v2_operational_package_api.py`, `test_v2_phase0.py`, `test_v2_request_observability.py`, `test_v2_speech_stt.py`)

### B. Strictly Protected & Preserved Assets
- **Protected `.txt` Files**: `claude.txt`, `idea.txt`, `requirements.txt`, `requirements-voice.txt` remain 100% intact and unedited per standing instructions.
- **Production Go Backend (`backend/`)**: All 16 packages, HTTP v3 server, contracts, migrations, storage, drills, and worker lifecycle controllers preserved.
- **Modern Citizen Web UI (`frontend/v2/`)**: Complete TypeScript, Vite, MapLibre GL UI with multilingual support and operator surface.
- **Mobile Citizen Apps (`mobile/`)**: Kotlin Multiplatform shared core (`mobile/shared`), Jetpack Compose (`mobile/android`), and SwiftUI (`mobile/ios`).
- **Retained Python Speech Adapters (`src/sthira_v2/`)**:
  - `src/sthira_v2/speech_asr_adapter.py` (IndicConformer-600M)
  - `src/sthira_v2/speech_tts_adapter.py` (Indic Parler-TTS)
  - Tested by `tests/test_b2_adapters.py` and `tests/test_v2_real_adapters.py` (20/20 PASS).
- **Retained v2 Reference Logic**: All 75 unit tests in `tests/` pass with zero failures.

---

## 2. Clean Build & Verification Sweep

Following the removal of legacy files and update of `Makefile`, the entire project was built and verified:

```bash
# 1. Full Makefile Check Suite
$ make check
# Target check-go:
cd backend && test -z "$(/opt/homebrew/opt/go/bin/gofmt -l .)"
cd backend && /opt/homebrew/opt/go/bin/go vet ./...
cd backend && /opt/homebrew/opt/go/bin/go build ./...
# [PASS]

# Target test-go:
cd backend && /opt/homebrew/opt/go/bin/go test ./...
# [PASS - 16 packages green]

# Target check-frontend-v2:
cd frontend/v2 && npm ci --ignore-scripts && npm test && npm run build
# [PASS - 21 vitest tests pass, Vite production build exit 0]

# Target check-adapters:
./.venv/bin/python -m pytest -q tests/test_b2_adapters.py tests/test_v2_real_adapters.py
# [PASS - 20 passed in 1.97s]

# Target check-python:
./.venv/bin/python -m compileall -q src tests
# [PASS - exit 0]

# 2. Remaining Python Unit Suite
$ ./.venv/bin/python -m pytest -q
# [PASS - 75 passed in 2.10s]

# 3. iOS Swift Source Validation
$ find mobile/ios -name "*.swift" -exec swiftc -parse {} +
# [PASS - exit 0]

# 4. System Assurance & Failure Drills
$ cd backend && go test -v ./internal/drills/...
# [PASS - 21/21 tests pass]
```

---

## 3. Phase Ledger Update

With Phase P10 complete, the software-engineering lifecycle (Phases P0–P10) is fully closed:
- Gate S: **PROVISIONAL PASS** (Software evidence complete, clean build proven).
- Remaining Phases (P11 & P12) are blocked strictly on external government authorizations, live NDMA feeds, and operational sign-offs.
