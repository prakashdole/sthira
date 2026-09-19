# Repository inventory and cleanup plan

Inspected 2026-09-19. This is a migration/deletion manifest, **not permission to delete these paths now**. User's final instruction limits this task to planning Markdown. All 206 tracked paths are inventoried below; source modules, imports, endpoints, test structure, fixtures, build/deploy setup and existing plans were inspected for scope/dependencies. This is not a line-by-line correctness/security audit or a claim that all tests ran.

## What must survive

Preserve official-source provenance, version/time semantics, CAP lifecycle and malformed-input fixtures, package cross-reference checks, capacity conservation/idempotency examples, explicit confirmations, source activation, offline invalidation, privacy boundaries and accessible fallback behavior. Port semantics into Go tests; do not port every Python class or every old test. Model weights and original evidence/user files must not be discarded with old application code.

## Dependency traps observed

- `src/sthira/api/app.py` is the deployed entry point and mounts `src/sthira_v2/app.py`; legacy middleware/core identity are still used.
- `tests/conftest.py` imports legacy live-operations state. V2 API tests also use the mixed entry point. A test filename beginning `test_v2` does not prove independence.
- `src/sthira_v2/app.py` and `voice_map.py` read `frontend/v2/src/scenario.json`; API startup also reads bundled CAP/package fixtures.
- `Makefile`, Dockerfile, Compose, pyproject, CI and migrations refer to Python/current web paths. Packaging must be updated before removing them.
- Two CAP parsing modules, local/in-memory versus SQL repositories, old/new UI and cloud-model adapters need consumer/evidence mapping; similar names alone do not prove safe duplication.
- Existing `.txt` files are protected by project instructions. Do not rewrite/delete `requirements.txt` or `requirements-voice.txt` as a shortcut; new Go/inference manifests can supersede usage while original files remain preserved.
- Downloaded weights, ignored `.env`/local data and other untracked user assets are not covered by Git's rollback. Do not scan, delete or move them as generic cleanup; handle only explicitly identified task assets if later authorized.

## Retirement order and proof

1. P0 freezes paths, hashes, imports, external consumers and useful invariants. Reconcile root instructions and task tracker in that future implementation stage; this task leaves them untouched.
2. P1 adds the isolated Go boundary and contract fixtures without changing the legacy runtime. New code does not import or invoke legacy permanent-relocation modules.
3. P2–P7 migrate required behavior and prove Go/DB/model contracts. Keep the Python version as an explicit reference fixture/test target; never two authoritative writers to the same database.
4. P8–P9 replace the web demo with verified Android+iPhone flows and an adequate operator surface. Migrate the scenario catalogue out of frontend source ownership before retiring it.
5. P10 removes retired runtime modules, obsolete tests and build references in coherent stages after import/static/package checks and replacement acceptance pass. Git history retains code; no redundant `legacy-copy/` tree is needed.
6. Re-run clean checkout build, integration tests, endpoint inventory, image/dependency scan and representative mobile flow. Check no active reference to deleted paths and no unrelated/.txt changes. Commit each verified stage; never claim deletion alone improved runtime efficiency.

## Disposition codes

- **KEEP**: durable evidence/config/document to retain or update as required.
- **PORT**: preserve relevant behavior/evidence; retire old implementation only after replacement passes.
- **RETIRE**: obsolete product behavior; remove only after dependency/consumer checks and relevant replacement gate.
- **REPLACE**: build/UI/framework-specific path replaced during migration; preserve until replacement works.
- **PROTECT**: no edit/delete in cleanup without explicit change to the governing instruction.

The table classifies scope, not code quality. `tests/test_v2*` are PORT evidence; named legacy tests may contain reusable invariants and must be reviewed before deletion. P0 refines this manifest against the then-current checkout.

## Full tracked-path inventory at planning start

| Path | Lines | Disposition | Gate / reason |
| --- | ---: | --- | --- |
| `.dockerignore` | 8 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `.github/workflows/ci.yml` | 26 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `.gitignore` | 67 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `.reticle/.gitignore` | 22 | RETIRE | P10: old web-only instrumentation if no remaining consumer |
| `CLAUDE.md` | 11 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `Dockerfile` | 19 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `GEMINI.md` | 47 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `Makefile` | 27 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `README.md` | 1 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `acceptance-matrix.json` | 27 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `alembic.ini` | 30 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `claude.txt` | 333 | PROTECT | User/project preservation rule |
| `deploy/.env.example` | 17 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `deploy/RUNBOOK.md` | 61 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `deploy/THREAT-MODEL.md` | 15 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `deploy/docker-compose.demo.yml` | 15 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `fixtures/idukki_fixture.json` | 142 | KEEP | P2/P10: preserve source evidence; classify synthetic/legacy use |
| `fixtures/synthetic_alappuzha.json` | 159 | KEEP | P2/P10: preserve source evidence; classify synthetic/legacy use |
| `fixtures/synthetic_cap_alert.xml` | 26 | KEEP | P2/P10: preserve source evidence; classify synthetic/legacy use |
| `fixtures/synthetic_idukki.json` | 158 | KEEP | P2/P10: preserve source evidence; classify synthetic/legacy use |
| `fixtures/synthetic_wayanad.json` | 266 | KEEP | P2/P10: preserve source evidence; classify synthetic/legacy use |
| `fixtures/uttarakhand_fixture.json` | 153 | KEEP | P2/P10: preserve source evidence; classify synthetic/legacy use |
| `fixtures/v2_wayanad_demo.json` | 107 | KEEP | P2/P10: preserve source evidence; classify synthetic/legacy use |
| `frontend/app.js` | 4360 | RETIRE | P0/P10: old UI; remove serving/build references first |
| `frontend/index.html` | 91 | RETIRE | P0/P10: old UI; remove serving/build references first |
| `frontend/styles.css` | 1813 | RETIRE | P0/P10: old UI; remove serving/build references first |
| `frontend/tokens.css` | 34 | RETIRE | P0/P10: old UI; remove serving/build references first |
| `frontend/v2/index.html` | 15 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/package-lock.json` | 1439 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/package.json` | 21 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/public/manifest.webmanifest` | 9 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/public/sw.js` | 25 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/src/main.ts` | 413 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/src/mapActions.ts` | 98 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/src/scenario.json` | 17 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/src/styles.css` | 160 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/src/vite-env.d.ts` | 4 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/tsconfig.json` | 13 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `frontend/v2/vite.config.ts` | 3 | REPLACE | P5/P8/P10: retain reference until mobile + catalogue replacement |
| `idea.txt` | 2422 | PROTECT | User/project preservation rule |
| `migrations/env.py` | 32 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `migrations/script.py.mako` | 15 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `migrations/versions/20260911_01_v2_persistence.py` | 20 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `plan/architecture.md` | 146 | KEEP | Current planning source of truth |
| `plan/changes.md` | 54 | KEEP | Current planning source of truth |
| `plan/decisions.md` | 150 | KEEP | Current planning source of truth |
| `plan/equations.md` | 81 | KEEP | Current planning source of truth |
| `plan/feature.md` | 54 | KEEP | Current planning source of truth |
| `plan/memory.md` | 33 | KEEP | Current planning source of truth |
| `plan/open-decisions.md` | 34 | KEEP | Current planning source of truth |
| `plan/parameters.md` | 65 | KEEP | Current planning source of truth |
| `plan/phases.md` | 99 | KEEP | Current planning source of truth |
| `plan/plan.md` | 50 | KEEP | Current planning source of truth |
| `plan/prd.md` | 95 | KEEP | Current planning source of truth |
| `plan/prompt.md` | 671 | KEEP | Current planning source of truth |
| `plan/rules.md` | 82 | KEEP | Current planning source of truth |
| `plan/source-register.md` | 94 | KEEP | Current planning source of truth |
| `plan/trd.md` | 124 | KEEP | Current planning source of truth |
| `plan/voice-map-system-prompt.md` | 425 | KEEP | Current planning source of truth |
| `pyproject.toml` | 36 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `requirements-voice.txt` | 9 | PROTECT | User/project preservation rule |
| `requirements.txt` | 7 | PROTECT | User/project preservation rule |
| `scripts/__init__.py` | 1 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `scripts/check_v2_database.py` | 98 | REPLACE | P1/P3/P7/P10: migration/deployment/reference checks |
| `src/sthira/__init__.py` | 6 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/api/__init__.py` | 6 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/api/app.py` | 2917 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/api/middleware.py` | 108 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/__init__.py` | 87 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/audit.py` | 179 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/contracts.py` | 212 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/enums.py` | 160 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/errors.py` | 154 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/identity.py` | 194 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/localization.py` | 100 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/core/outbox.py` | 122 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/adaptation/__init__.py` | 12 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/adaptation/service.py` | 220 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/allocation/__init__.py` | 36 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/allocation/reservation_ledger.py` | 373 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/allocation/service.py` | 362 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/catalog/__init__.py` | 6 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/catalog/service.py` | 132 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/district_scale/__init__.py` | 37 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/district_scale/service.py` | 343 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/evaluation/__init__.py` | 20 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/evaluation/service.py` | 215 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/field/__init__.py` | 24 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/field/service.py` | 349 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/governance/__init__.py` | 56 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/governance/approval_service.py` | 408 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/governance/national_clearinghouse.py` | 140 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/governance/objections_service.py` | 440 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/governance/service.py` | 309 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/hazard/__init__.py` | 6 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/hazard/service.py` | 96 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/household/__init__.py` | 6 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/household/service.py` | 94 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/land_truth/__init__.py` | 25 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/land_truth/service.py` | 298 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/live_ops/__init__.py` | 45 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/live_ops/service.py` | 470 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/policy/__init__.py` | 64 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/policy/compliance_service.py` | 960 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/policy/service.py` | 381 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/programme/__init__.py` | 6 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/programme/district_service.py` | 180 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/programme/service.py` | 76 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/programme/state_package.py` | 224 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/reconstruction/__init__.py` | 59 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/reconstruction/delivery_tracker.py` | 922 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/reconstruction/service.py` | 337 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/reporting/__init__.py` | 35 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/reporting/service.py` | 970 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/reporting/statewide_dashboard.py` | 145 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/resilience/__init__.py` | 35 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/resilience/contracts.py` | 136 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/resilience/service.py` | 555 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/scaling/__init__.py` | 16 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/scaling/service.py` | 236 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/modules/security/__init__.py` | 16 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/security/service.py` | 76 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/source_access/__init__.py` | 53 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/source_access/contracts.py` | 243 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/modules/source_access/service.py` | 1239 | PORT | P0/P7/P10: extract applicable invariants; remove legacy coupling |
| `src/sthira/spikes/__init__.py` | 20 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira/spikes/fixture_loader.py` | 94 | RETIRE | P0/P10: superseded product; inspect shared consumers first |
| `src/sthira_v2/__init__.py` | 4 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/allocation.py` | 112 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/app.py` | 285 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/audit.py` | 178 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/azure_openai.py` | 54 | RETIRE | P6/P10: optional demo cloud providers replaced |
| `src/sthira_v2/cap.py` | 298 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/config.py` | 104 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/contracts.py` | 553 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/fixtures.py` | 87 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/local_voice.py` | 102 | PORT | P6: preserve model/runtime evidence; isolated worker |
| `src/sthira_v2/multimodal.py` | 102 | PORT | P6: preserve model/runtime evidence; isolated worker |
| `src/sthira_v2/nemotron.py` | 74 | RETIRE | P6/P10: optional demo cloud providers replaced |
| `src/sthira_v2/offline.py` | 63 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/operational_package.py` | 166 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/package_service.py` | 103 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/persistence/__init__.py` | 5 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/persistence/models.py` | 149 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/persistence/repositories.py` | 84 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/readiness.py` | 78 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/security.py` | 73 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/source_activation.py` | 160 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/source_health.py` | 130 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/speech_stt.py` | 70 | PORT | P6: preserve model/runtime evidence; isolated worker |
| `src/sthira_v2/voice_commands.py` | 88 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/voice_map.py` | 147 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `src/sthira_v2/xml_parser.py` | 105 | PORT | P1–P7/P10: required emergency semantics; no blind translation |
| `tasks/lessons.md` | 12 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `tasks/phase0_inventory.json` | 77 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `tasks/todo.md` | 120 | KEEP | Preserve/update deliberately; no blanket cleanup |
| `tests/__init__.py` | 0 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/authutil.py` | 17 | PORT | P0/P10: test/auth/invariant dependencies |
| `tests/conftest.py` | 12 | PORT | P0/P10: test/auth/invariant dependencies |
| `tests/test_api.py` | 358 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_api_phase10.py` | 257 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_api_phase11.py` | 235 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_api_phase12.py` | 152 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_api_phase13.py` | 209 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_api_phase14.py` | 219 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_api_phase9.py` | 259 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_core_contracts.py` | 149 | PORT | P0/P10: test/auth/invariant dependencies |
| `tests/test_fixtures.py` | 64 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_p0_security.py` | 143 | PORT | P0/P10: test/auth/invariant dependencies |
| `tests/test_phase10_delivery_execution.py` | 200 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase11_reporting_manifests.py` | 299 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase12_formula_compliance.py` | 292 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase13_source_readiness.py` | 339 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase14_platform_resilience.py` | 319 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase2_shadow_pilot.py` | 547 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase3_controlled_live.py` | 411 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase4_kerala_scaling.py` | 231 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase6_national_clearinghouse.py` | 209 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_phase9_governance_approvals.py` | 583 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_scaling_and_adaptation.py` | 167 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_v2_allocation.py` | 52 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_assignment_api.py` | 26 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_audit_source_activation.py` | 110 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_azure_openai.py` | 11 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_cap.py` | 106 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_contracts.py` | 192 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_database_readiness.py` | 34 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_demo_scenario_api.py` | 24 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_fixtures.py` | 42 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_multimodal.py` | 40 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_nemotron.py` | 43 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_offline.py` | 31 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_openapi.py` | 19 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_operational_package.py` | 67 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_operational_package_api.py` | 13 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_package_service.py` | 39 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_persistence.py` | 57 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_phase0.py` | 48 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_readiness.py` | 51 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_request_observability.py` | 13 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_security.py` | 21 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_source_health.py` | 51 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_speech_stt.py` | 25 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_v2_voice_commands.py` | 22 | PORT | P1–P9/P10: behavior reference, not automatic Go acceptance |
| `tests/test_wave_a.py` | 219 | RETIRE | P0/P10: review for shared safety evidence before removal |
| `tests/test_wave_b.py` | 257 | RETIRE | P0/P10: review for shared safety evidence before removal |
