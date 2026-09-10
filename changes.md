---
title: Sthira Revision 1.4 Change Record
document_id: PUN-CHANGES
version: 1.4
status: Post-implementation audit and remaining-work register
as_of: 2026-09-09
audience: Product, programme, legal, domain, engineering, QA, assurance, and future maintainers
owner: Product owner
normative_scope: Change history, migration map, source-to-disposition traceability, and validation record
---

# Sthira Revision 1.4 Change Record

## 1. Revision basis

Revision 1.4 records the 9 September 2026 repository, commit, test, architecture-conformance, security, and production-readiness audit. It does **not** declare the software complete. The current repository is a useful synthetic-data demonstrator and domain/API prototype, but it does not yet implement the production architecture or satisfy the PH-1 through PH-3 exit gates. Sections 11–15 are the controlling current implementation status and remaining-work register; earlier sections remain the historical documentation-change record.

Revision 1.3 reorganizes [phases.md](./phases.md) into agent-ready research, coding, integration, and assurance work packages. It adds contract-first ownership, parallel waves, dependencies, merge evidence, file/module ownership, and stop conditions without changing product scope or phase gates.

Revision 1.2 added the supplied Sthira data-source handoff (`DATA_SOURCE_GUIDE.md`, `README.txt`, `SOURCE_MATRIX.csv`, `SOURCE_MATRIX.json`, two identical `SOURCE_MATRIX.xlsx` copies, and `SOURCE_MENTIONS.txt`). Embedded package instructions and extracted transcript claims were treated as review/source material; the user's request to add the sources was the controlling instruction. CSV and JSON match all 54 rows, the workbook contains the same rows plus its cautionary read-me sheet, and both XLSX copies have SHA-256 `b7b5bee4aea9fa602bbb2214e01af11e74a667bcdd88e82620a96491070428be`.

Revision 1.1 had reconciled the original `idea.txt` and `claude.txt`, the eight initial product documents, `plan.md`, and the supplied Astra equation/assurance handoff. That history remains below.

The user had already approved the 8 September 2026 plan. Therefore Wayanad, the production-oriented blueprint, product name, and named architecture remain accepted, with explicit dated provenance. This resolves the handoff's concern that the earlier transcripts alone did not prove those choices.

## 2. Revision 1.3 parallel-delivery change

[phases.md](./phases.md) now defines PKG-0C, R0/C0, C1/I1, R2/C2/I2, R3/C3/I3, R4/C4/I4, and R5/C5/I5 packages. Each package separates research from implementation, names outputs and dependencies, maps coding ownership to existing features/components, identifies safe parallel waves, and defines integration/exit evidence. No application code, research result, data access, or phase completion is claimed by this planning change.

## 3. Revision 1.2 data-source changes

| File | Revision 1.2 change |
| --- | --- |
| [source-register.md](./source-register.md) | Added every S01–S54 source, access class, acquisition route, intended use, qualification, activation gate, and recommended Wayanad public stack |
| [rules.md](./rules.md) | Added RUL-076–083 for source/service separation, AOI validation, mirror handling, blocker gates, provider corrections, basemaps, and proxy limits |
| [trd.md](./trd.md) | Added FR-076–084, NFR-034–035, AT-31–38, source states, contracts, dependency records, and release-readiness evidence |
| [architecture.md](./architecture.md) | Added ARC-C13, the source connector plane, acquisition flow, provider/fallback boundaries, and failure handling without moving requirements into architecture |
| [feature.md](./feature.md) | Added FEAT-026 source acquisition/readiness and wired it into the Wayanad evidence sandbox |
| [phases.md](./phases.md) | Added public AOI sample gates, S45–S50 live blockers, connector failure scenarios, and source-readiness exit evidence |
| [parameters.md](./parameters.md) | Added S01–S54/acquisition fields and a complete parameter-family-to-source acquisition map |
| [prd.md](./prd.md) | Added product-level source readiness, minimum public stack, and agency/field blockers without turning acquisition details into architecture |
| [plan.md](./plan.md) | Updated the documentation/assurance plan for the full source register and acquisition validation |
| [memory.md](./memory.md) | Added durable source truth, provider corrections, current inventory-only state, and shared-input risks |
| [decisions.md](./decisions.md) | Added DEC-031–032 with rationale, rejected alternatives, consequences, evidence, and review triggers |
| [open-decisions.md](./open-decisions.md) | Added ODN-021–022 for exact public connectors/releases and restricted agency/field acquisition |
| [changes.md](./changes.md) | Recorded this additive revision and validation results |

No source dataset, provider account, API entitlement, agency agreement, or field measurement was created by this documentation revision. The two XLSX copies are mirrors of one artifact, not two evidence sources.

### 3.1 Additive identifiers

- RUL-076–083
- FR-076–084
- NFR-034–035
- AT-31–38
- FEAT-026
- ARC-C13
- DEC-031–032
- ODN-021–022
- S01–S54 (preserved handoff identifiers; separate from SRC-001–029 evidence citations)

## 4. Revision 1.1 files changed or added

| File | Revision 1.1 change |
| --- | --- |
| [plan.md](./plan.md) | Corrected readiness/research wording; added approval provenance, formula registers, funding/completion, profiles and evidence gaps |
| [prd.md](./prd.md) | Added relocation-necessity alternatives, community/settlement scope, schemes/funding, consent separation and completion/handoff outcomes |
| [rules.md](./rules.md) | Resolved coverage/conditional approval/consent/retention conflicts; added numerical, reservation, funding and completion invariants |
| [trd.md](./trd.md) | Added explicit allocation/reservation behavior, executable schema constraints, independent state axes, formulas, compliance requirements, security/reliability NFRs and adversarial tests |
| [architecture.md](./architecture.md) | Added formula activation, validator/reservation ledger, transactional outbox, complete authorization paths, offline key/recovery handling, coordinated restore and deployment profiles |
| [feature.md](./feature.md) | Broke FEAT-006/007/008 cycle; added outcome/component links and FEAT-022–025 |
| [phases.md](./phases.md) | Reconciled features/tests, added honest prototype lane, economics/dependencies and preregistered pilot evaluation |
| [memory.md](./memory.md) | Corrected amended District Plan cadence; removed irrelevant tool/source history; added mathematical retention, approval provenance and open gaps |
| [decisions.md](./decisions.md) | Added approval provenance, corrected MILP rationale, new decisions DEC-023–030 and expanded correction register |
| [equations.md](./equations.md) | Added E01–E60 controlled mathematical registry, full retention, owners, status, limitations and fixtures |
| [parameters.md](./parameters.md) | Added parameter schema, approved product targets, open policy/scientific values and prohibited defaults |
| [source-register.md](./source-register.md) | Added dated, qualified evidence register and unresolved verification list |
| [open-decisions.md](./open-decisions.md) | Added ODN-001–020 with owners, phase gates, safe defaults and closure evidence |
| [changes.md](./changes.md) | Added this traceable revision and migration record |

No application code or source transcript was changed. Documentation does not claim the platform, legal process, scientific models, or pilot has been implemented or validated.

## 5. Revision 1.1 identifier migration

Existing IDs are preserved: OUT-01–10, RUL-001–060, FR-001–062, NFR-001–027, AT-01–14, FEAT-001–021, PH-0–5, ARC-C01–11 and DEC-001–022.

New IDs are additive:

- OUT-11–12: actionable funding/delivery and defensible necessity/completion.
- RUL-061–075: mathematical, funding, reservation, completion and disclosure controls.
- FR-063–075: necessity, scheme/funding, completion and formula/compliance records.
- NFR-028–033: outbox, delivery-path authorization, RLS, offline and recovery details, CERT-In operations.
- AT-15–30: double-reservation, solver, numerical, funding, consent, condition, state, completion, disclosure, authorization, outbox, offline, restore, retention, double-counting and unsupported-coverage cases.
- FEAT-022–025: necessity review, scheme/funding, reservation ledger and completion tracking.
- ARC-C12: scheme/funding/completion domain.
- DEC-023–030 and ODN-001–020.
- E01–E60 and PAR-001–020.

## 6. Revision 1.1 handoff correction coverage

| # | Correction | Implemented in |
| --- | --- | --- |
| 1 | Restore/classify all mathematics and require complete contracts | equations, parameters, rules, TRD |
| 2 | Apply amended District Plan cadence and review current Act | memory, plan, PRD, rules, TRD, source register |
| 3 | Record real approval provenance; do not fabricate consent | decisions, open decisions |
| 4 | Define relocation boundary, necessity alternatives, pilot/profile and community scale | PRD, features, phases, decisions |
| 5 | Add scheme-specific eligibility, funding, entitlements and shortfalls | PRD, rules, TRD, features, equations |
| 6 | Define completion beyond allocation or explicit handoff | PRD, rules, TRD, features |
| 7 | Keep urgency, suitability, readiness, funding, consent and delay separate | rules, TRD, equations |
| 8 | Specify allocation variables, resources, budgets, phases and open objective policy | equations, TRD, parameters, open decisions |
| 9 | Prevent double allocation through atomic shared-resource reservations | rules, TRD, architecture, feature |
| 10 | Correct numerical/solver status, tolerance, tie and explanation claims | rules, TRD, equations, decisions |
| 11 | Resolve unsupported source coverage | rules, TRD, features, tests |
| 12 | Resolve conditional approval versus mandatory gates | rules, TRD, open decisions |
| 13 | Separate evidence, administrative, legal, notice, hold and delivery states | TRD, rules, decisions |
| 14 | Separate lawful basis, participation, pathway, preference, offer and community process | PRD, rules, TRD, features |
| 15 | Replace unjustified water/land/grazing constants with evidence | equations, parameters, rules |
| 16 | Specify exposure, population and imagery assumptions/NoData | equations, rules, TRD |
| 17 | Keep hazard models as qualified inputs with coverage/uncertainty | equations, source register, rules |
| 18 | Preserve and reject Ω/Λ/Θ, bad PPM/cutoffs and overclaims | equations, parameters, decisions |
| 19 | Add executable data types, cardinality, constraints and version semantics | TRD §6.4–6.5 |
| 20 | Add dimensional, boundary, missing, monotonicity and ranking tests | equations, rules, TRD |
| 21 | Protect API, tiles, STAC, objects, search, reports and caches; test RLS | architecture, TRD |
| 22 | Close DB-to-queue dual-write gap | architecture, TRD, decisions |
| 23 | Specify offline keys, devices, eviction, revocation limits and recovery | architecture, TRD |
| 24 | Resolve retention/deletion/legal holds/audit/backups and DPDP timing | rules, TRD, decisions, open decisions |
| 25 | Operationalize CERT-In and test bilingual/non-map/HTML/PDF access | TRD, source register, features |
| 26 | Add prototype/pilot/production economics, basemap and staffing gates | architecture, phases, open decisions |
| 27 | Remove feature dependency cycle and complete OUT/ARC traceability | feature, automated graph check |
| 28 | Repair phase/acceptance mappings and disclose 32–46 week pre-live range | phases |
| 29 | Pre-register independent pilot evaluation and qualify 30% target | phases, equations, open decisions |
| 30 | Replace “research complete/current” and tool history with dated evidence gaps | plan, memory, source register |

## 7. Original mathematics disposition matrix

The complete formulas, symbols, units and limitations are in [equations.md](./equations.md). This matrix provides the requested original idea → later intent → decision → destination → status → test mapping.

| Source family | Latest defensible intent | Decision | Destination | Status | Principal test |
| --- | --- | --- | --- | --- | --- |
| Units/time/CRS conversion | Explicit common units and support | DEC-023 | E01; FR-073 | CORE | AT-17 |
| Hard gates | Unknown is not pass; action-stage conditions | DEC-006/028 | E02; RUL-029/040/063 | CORE | AT-20 |
| Hazard/source overlap | Coverage and NoData explicit | DEC-004/015 | E03; FR-013–018 | CORE | AT-01/30 |
| Dasymetric exposure | Estimate only; conserve baseline; no beneficiaries | DEC-004/015 | E04; FR-014–016 | CORE when used | Zero-denominator/overlap fixture |
| Dwelling/seasonal population | Optional range; no 0.10 default/night-light vacancy | DEC-015/023 | E05 | OPTIONAL | Missing/occupancy scenario |
| Normalization | Stable approved anchors; no silent reweight | DEC-006/023 | E06 | CORE when scoring | 0–100/boundary fixture |
| Household urgency/PPM | Separate need from site scarcity and institutions | DEC-006 | E07 replaces E57 | CORE policy form; original REJECTED | Double-counting/Ω regression |
| Site suitability/CC | Gate first; criteria visible; not capacity | DEC-006 | E08 replaces E57 | CORE when ranking | Gate/non-compensation fixture |
| Network access/decay | Network costs with approved units/parameters | DEC-006/023 | E09 | CORE/OPTIONAL | Unreachable/unknown fixture |
| Livelihood cosine fit | Optional compatibility, not sufficiency | DEC-006/023 | E10 | OPTIONAL | Zero-vector/resource test |
| Water conversion/demand/supply | 55 LPCD conditional; lean supply and services separate | DEC-006/023 | E01, E11–E12 | CORE | AT-05; water fixture |
| Developable land/layout | Union exclusions, contiguity/layout; no 12 m² shortcut | DEC-006/023 | E13, E37 | CORE/OPTIONAL | Overlap/unit fixture |
| Capacity and shared resources | Compatible units and atomic remaining capacity | DEC-007/025 | E14 | CORE | AT-15 |
| Grazing/feed | Evidence-based feed balance, not urban open-space norm | DEC-023 | E15 | OPTIONAL | Unit/source review |
| Growth scenarios | Labelled scenarios, not predictions | DEC-023 | E16 | OPTIONAL | Time-unit/bounds test |
| Cost/funding/entitlement | Cost heads, confirmed nonduplicate funding, scheme rules | DEC-026 | E17–E18 | CORE/OPTIONAL | AT-18 |
| Binary allocation/unassigned | Indivisible household and explicit unassigned outcome | DEC-007 | E19 | CORE | AT-08 |
| Eligibility/participation/preferences | Purpose-specific records and admissible options | DEC-007/028 | E20 | CORE | AT-19 |
| Dwelling/person/accessibility resources | Separate units and capacities | DEC-007/025 | E21 | CORE | AT-08/15 |
| Budget/site activation | Fixed and variable cost without double count | DEC-007/026 | E22 | CORE when in scope | Budget fixture |
| Multi-period allocation | Readiness, cumulative capacity and cash flow | DEC-007/026 | E23 | CORE when phased | AT-22 |
| Community/caregiving links | Voluntary/required, hard or soft by policy | DEC-007/024 | E24 | OPTIONAL/conditional | Linked-household fixture |
| Objective/ties/fairness | Explicit policy-approved lexicographic order | DEC-024 | E25 | PROPOSED/OPEN | Objective/tie fixture |
| Solver diagnostics | Version/tolerance/status/gap plus independent validation | DEC-007/027 | E26 | CORE | AT-16 |
| Hungarian/stable matching | Retain for narrower assumptions | DEC-007 | E27 | OPTIONAL | Comparative feasibility fixture |
| AHP | Optional elicitation; consistency is not fairness | DEC-006/023 | E28 | OPTIONAL | Consistency/sensitivity test |
| PCA/SoVI | Exploratory only; variance is not policy validity | DEC-006/023 | E29 | OPTIONAL | Leakage/subgroup test |
| Ranking sensitivity | Show instability; not safety probability | DEC-006 | E30 | CORE if ranking | Rank-reversal fixture |
| Cadastral transformation/IoU | Error/discrepancy, never title proof | DEC-004/009 | E31 | CORE/OPTIONAL | AT-03 |
| Evidence completeness/overdue | Workflow metrics, not truth/risk | DEC-016 | E32 | CORE/OPTIONAL | AT-09 |
| Accuracy and completion metrics | Preregistered independent evaluation | DEC-023/026 | E33 | CORE | PH-2 protocol |
| Availability/hash chain | Service metric and tamper evidence, not immutability | DEC-012/021 | E34 | CORE | AT-27/28 |
| NDVI/NDWI/NDBI | Optional discrepancy evidence with aligned bands/masks | DEC-015/023 | E35–E36 | OPTIONAL | AT-02/17 |
| Slope/TWI | Terrain evidence, not soil depth/safety/water proof | DEC-015/023 | E37–E38 | CORE/OPTIONAL/REFERENCE | AT-01 |
| InSAR | Qualified external/specialist evidence | DEC-015 | E39 | SPECIALIST/EXTERNAL | Model assurance |
| Factor of safety/Richards/root effects | Qualified site-specific engineering only | DEC-015/023 | E40–E42 | SPECIALIST/EXTERNAL | Independent validation |
| API/rainfall thresholds | Calibrated context only; no universal triggers | DEC-015/023 | E43 | SPECIALIST/OPTIONAL | Local holdout test |
| FR/logistic susceptibility | Qualified model; not automatic probability/order | DEC-015 | E44 | SPECIALIST/EXTERNAL | Spatial/temporal validation |
| Voellmy/travel angle | Qualified runout model/screen; no universal cone | DEC-015 | E45–E46 | SPECIALIST/EXTERNAL | Local calibration |
| SCS runoff/Manning/shallow water | Correct piecewise/unit models; not automatic flood map | DEC-015/023 | E47–E49 | SPECIALIST/EXTERNAL | Runoff/conservation benchmark |
| Return period/stage trend | Qualified assumptions; no deterministic safety | DEC-015/023 | E50–E51 | OPTIONAL/SPECIALIST | Assumption/quality test |
| GLOF peak relation | Preserve attribution uncertainty; no Wayanad module | DEC-015/030 | E52 | SPECIALIST, UNVERIFIED | Source verification required |
| Newmark/Arias | Requires ground motion and applicable regression | DEC-015 | E53 | SPECIALIST/EXTERNAL | Specialist benchmark |
| H×E×V/C/expected loss | Conceptual/qualified risk only; no master score | DEC-006 | E54, E58–E59 | OPTIONAL/REJECTED original | Calibration/scenario test |
| Emergency supplies/routing | Preserve arithmetic/reference; outside product | DEC-001 | E55–E56 | OUT-OF-SCOPE REFERENCE | Non-activation test |
| PPM/CC original | Preserve defects and corrected replacements | DEC-006/023 | E57 | REJECTED | Scale/compensation regression |
| Ω/Λ/Θ | Preserve provenance; never execute | DEC-006/016/023 | E58 | REJECTED | Non-execution/Ω regression |
| Seismic radius/root-loss/seasonal/fixed cutoffs | No unsupported universal constants | DEC-015/023 | E59 | REJECTED | Parameter deny-list test |
| Mathematical authority boundary | Output ≠ legal approval ≠ acceptance ≠ completion | DEC-005/026 | E60 | CORE | AT-21/22 |

## 8. Validation performed for revision 1.1

- Existing identifiers were preserved and new identifiers were additive.
- Feature build dependencies were separated from runtime integrations to remove the FEAT-006 → FEAT-008 → FEAT-007 → FEAT-006 cycle.
- TRD and architecture boundaries remain explicit: TRD owns measurable requirements/contracts; architecture owns technology/topology/flows.
- Automated structural checks passed for 14 controlled Markdown files: local links resolve, front matter and delimiters are balanced, explicit feature dependencies are acyclic, and every feature contains dependencies, requirements, outcome/component mapping, rules/decisions, acceptance and phase.
- Definition checks found complete, duplicate-free sequences: 12 outcomes, 75 rules, 75 functional requirements, 33 non-functional requirements, 30 acceptance scenarios, 25 features, 6 phases, 12 architecture components, 30 decisions, 60 equation groups, 20 parameters, 20 open decisions and 29 sources.
- No embedded transcript code was promoted to a requirement. Original formulas remain preserved through the controlled equation/rejection registry.

## 9. Revision 1.2 validation

- Confirmed the CSV, JSON, and workbook `Sources` sheet contain the same 54 records in the same fields/order; both supplied XLSX copies are byte-identical with SHA-256 `b7b5bee4aea9fa602bbb2214e01af11e74a667bcdd88e82620a96491070428be`.
- Confirmed source-register table coverage is exactly S01–S54 with no missing or duplicate ID.
- Confirmed complete, duplicate-free controlled sequences: RUL-001–083, FR-001–084, NFR-001–035, AT-01–38, FEAT-001–026, ARC-C01–13, DEC-001–032, ODN-001–022, PAR-001–020, and existing SRC-001–029.
- Checked all 14 Markdown files for valid front matter, balanced code fences, consistent table structure, and resolvable local links; no errors found.
- Confirmed all 26 feature entries retain users, behavior, dependencies, requirements, outcome/component mapping, rules/decisions, acceptance, and phase, and their dependency graph is acyclic.
- Confirmed no reference exceeds the defined ID range and no source-package instruction/code was promoted as evidence of operational access.

## 10. Historical validation boundary through revision 1.3

Revisions 1.1–1.3 changed documentation only. At that point no software, schema migration, integration, formula implementation, field observation, legal workflow, funding route, public disclosure profile, accessibility conformance, security control, SLO, recovery target, or pilot outcome had been implemented or verified. Application code was added in later commits. Its current audited status is recorded below and supersedes any later commit message or decision entry that describes a phase as complete without the required exit evidence.

## 11. Revision 1.4 repository and test audit

Audit target: clean `main` at commit `1e4aa90` on 9 September 2026.

| Check | Result | Qualification |
| --- | --- | --- |
| Git working tree | PASS | Clean before and after the audit |
| Commit/object integrity | PASS | 41 commits; `git fsck --full` returned no error |
| Python tests | PASS | 172 tests passed with two third-party deprecation warnings using `.venv/bin/python -m pytest -q` |
| Test isolation | PASS | Every `tests/test_*.py` file also passed in its own Python process |
| Python syntax/import compilation | PASS | `python -m compileall -q src` |
| JavaScript syntax | PASS | `node --check frontend/app.js` |
| Installed dependency consistency | PASS | `.venv/bin/python -m pip check` |
| FastAPI smoke check | PASS | Application started; `/health`, `/ui/`, and `/openapi.json` returned HTTP 200 |
| API authentication audit | FAIL | All 147 FastAPI routes have zero authentication/authorization dependencies |
| Privileged approval abuse check | FAIL | An unauthenticated request with the invented token `MFA-STEPUP-anything` returned HTTP 200 and `OFFICIALLY_APPROVED` |
| Durable outbox retry check | FAIL | A failed core outbox item became `FAILED`, was not retried, and never reached `DEAD_LETTER` |
| Browser-wide UI crawl | UNAVAILABLE | No Reticle-instrumented browser session or project configuration exists; this must not be reported as a passed UI audit |
| Coverage, lint, typing, SAST and dependency vulnerability scan | UNAVAILABLE | No configured tools, thresholds, or CI gates exist |
| Production architecture conformance | FAIL | Required database, migrations, identity, workers, object storage, GIS delivery, observability and deployment assets are absent |

The 172 passing tests are useful regression evidence for the current in-memory model. They are not evidence of production authentication, database isolation, real provider integration, browser accessibility, load capacity, recovery objectives, or government/field validation.

## 12. Current implementation classification

### 12.1 Implemented as prototype behavior

- FastAPI application shell and OpenAPI schema with broad domain-route coverage.
- Pydantic contracts, enums, structured error types, advisory envelopes, and synthetic fixtures.
- In-process domain services for programmes, hazards, households, land discrepancies, policy evaluation, governance, objections, allocation, delivery, reporting, source readiness, resilience, scaling, and adaptation.
- Hash-linked in-process audit records and demonstrator outbox structures.
- Static HTML/CSS/JavaScript demonstration interface with English, Malayalam, and partial Hindi strings.
- Synthetic scenario tests for many rules and acceptance cases.

### 12.2 Not implemented as production capability

- Persistent PostgreSQL/PostGIS data model, database constraints, row-level security, migrations, bitemporal history, and transactional repositories.
- Real OIDC/SAML login, identity-provider integration, verified MFA, session management, role/geography/classification enforcement, and protected endpoints.
- India-resident versioned object storage, immutable evidence objects, signing-key custody, signed URLs, and retention/legal-hold enforcement.
- Celery/RabbitMQ or equivalent durable jobs, a transactional database outbox, working retry/backoff/dead-letter processing, and reconciliation workers.
- STAC, OGC API Features, MVT, COG, GDAL/PROJ processing, authenticated map/data delivery, and actual MapLibre rendering.
- Real S01–S54 acquisition adapters, provider telemetry, AOI downloads, checksums, licenses/entitlements, quarantines, and source-refresh/supersession jobs. Current health values and many source states are seeded demonstrations.
- The specified Next.js/TypeScript installable PWA, service worker, encrypted offline store, reliable IndexedDB synchronization, device binding, and browser eviction recovery.
- A real capacity-constrained MILP adapter and independent solver verification. Current allocation is a deterministic greedy matcher.
- Controlled PDF/GeoPackage generation, durable manifest/signature verification, official-template management, notification delivery, and external departmental reconciliation.
- Metrics, logs, traces, SIEM integration, alerting, rate limits, CSP/security headers, secrets management, backup automation, failover, RPO/RTO measurement, or load-tested scaling.
- OCI/container definitions, infrastructure-as-code, deployment environments, reproducible lock files, release pipeline, rollback automation, and operator runbooks.

## 13. Required code changes and remaining coding work

Priority meanings: **P0** blocks any live or restricted-data use; **P1** blocks a credible shadow pilot; **P2** is required before broader scaling or maintainable release.

| ID | Priority | Required change / remaining code | Minimum completion evidence |
| --- | --- | --- | --- |
| REM-001 | P0 | **Prototype done (DEC-046):** HMAC sessions from a local identity directory; identity/roles/geography from the token; non-public routes return 401. **Still open:** Keycloak/OIDC/SAML broker, issuer/audience from a real IdP, session revocation store. | Unauthorized/expired/forged token tests pass on the prototype; OIDC federation tests remain open |
| REM-002 | P0 | **Prototype done (DEC-046):** step-up tokens bound to principal, action, nonce and expiry; consume-on-use; `MFA-STEPUP-anything` returns 403; frontend no longer ships a privileged token. **Still open:** IdP-backed MFA/step-up. | Forgery, replay and cross-action tests pass |
| REM-003 | P0 | Introduce PostgreSQL/PostGIS repositories, transactional unit-of-work boundaries and Alembic migrations. Move all domain state, audit events, approvals and reservations out of process memory. | Restart-persistence, migration upgrade/rollback, concurrency, constraint and transaction tests against real PostgreSQL/PostGIS |
| REM-004 | P0 | Implement database-enforced RLS and scoped service identities; ensure application owners, superusers, `BYPASSRLS`, views, functions and pooled-connection reuse cannot leak records. | NFR-012/NFR-030 and AT-24 tests run against PostgreSQL, not an input-based simulator |
| REM-005 | P0 | **In-process done (DEC-046):** core outbox retries `FAILED` through `DEAD_LETTER` and reconciles without duplicating idempotency keys; resilience service uses that outbox. **Still open:** durable PostgreSQL outbox table, broker worker, exponential wall-clock backoff. | Injected broker failure progresses through retries to dead letter and later reconciliation |
| REM-006 | P0 | Add encryption and managed-key boundaries for databases, objects, backups, sensitive fields and offline packages. Add secret management and remove all demonstrator credentials/tokens from client code. Client MFA example token is removed; server prototype secrets remain env-overridable defaults. | Key-rotation, redaction, secret-scan, lost-device and authorized-decryption tests |
| REM-007 | P0 | **Prototype done (DEC-046):** HTTP mutations and approval/reservation/notification commands fail closed while degraded (503). **Still open:** hold/outage flags at a real transaction boundary once PostgreSQL exists. | Approval and other mutations fail closed during degraded mode |
| REM-008 | P1 | Implement immutable/versioned S3-compatible object storage, checksums, evidence links, signed export manifests, retention, legal holds and coordinated DB/object/key restore. | Real backup-and-restore exercise proves NFR-007/008/032 and AT-27 |
| REM-009 | P1 | Implement selected S01–S54 connectors and governed uploads with real entitlement, coverage, freshness, CRS, unit, license and checksum validation. Remove seeded provider-health claims from production paths. | Permitted AOI sample receipts and failure/fallback tests for each activated source; S45–S50 remain blocked until authorized |
| REM-010 | P1 | Implement PostGIS/GDAL spatial ingestion and reproducible overlays, uncertainty handling, STAC catalog, authorized OGC/vector/COG delivery, MVT tiles and MapLibre map/non-map parity. | AT-01–05/10/30/34/36 with real spatial fixtures and cross-channel authorization |
| REM-011 | P1 | Replace greedy allocation with the approved capacity-constrained MILP adapter, pinned inputs, solver status/gap/seed, explicit unassigned results, policy-approved objective, sensitivity analysis and independent feasibility validation. | AT-08 and AT-15–17 plus infeasible, tie, concurrent reservation and reproducibility cases |
| REM-012 | P1 | Replace the static 3,903-line JavaScript page with the architecture-approved componentized Next.js/TypeScript PWA, or formally revise the architecture through an ADR. Implement service worker/offline workflow and secure storage rather than UI simulations. | Production build, browser E2E tests, offline/reconnect/conflict tests and no client-side privileged token |
| REM-013 | P1 | Complete accessible HTML/PDF, CSV, GeoJSON and GeoPackage outputs, template versioning, cryptographic signing, public/private projections and withdrawal/regeneration. | Deterministic/content-equivalent reproduction, tamper, privacy and assisted non-map tests |
| REM-014 | P1 | Add notification and external-system adapter boundaries with idempotent delivery and reconciliation. Do not claim Gazette, treasury, land, water or departmental publication from locally created records. | Sandbox/agency contract tests, delivery receipts and explicit unavailable/manual states |
| REM-015 | P1 | Split the 2,715-line API module and duplicate services into bounded routers/application services/repositories. Stop reading private service fields directly and remove duplicated capacity, district-scaling, recovery and outbox implementations. | Dependency-boundary tests, smaller reviewable modules and one authoritative implementation per capability |
| REM-016 | P1 | Add CI with locked/reproducible dependencies, formatting/linting, typing, unit/integration tests, schema compatibility, secret/dependency/container scanning and build artifacts. | A clean pipeline on a fresh runner; protected merge gate and retained reports |
| REM-017 | P1 | Add browser E2E and accessibility verification for every critical workflow, including keyboard and screen-reader checks, English/Malayalam completeness, non-map parity, error/empty/loading states and responsive layouts. | WCAG 2.2 AA/GIGW evidence plus a full non-truncated UI crawl with zero unexplained failures |
| REM-018 | P1 | Add performance and reliability tests for the TRD reference load, asynchronous jobs, source failure, concurrent reservations, cache isolation and restart/failover. | Measured NFR-001–010 and NFR-028–035 results; no configuration-only claims |
| REM-019 | P2 | Add OCI containers, environment configuration, infrastructure-as-code, ingress separation, observability, alerts, backup schedules, patching, rollback and operational runbooks. | Reproducible staging deployment and exercised rollback/restore/incident drills |
| REM-020 | P2 | Add API pagination/filtering, optimistic concurrency, idempotency keys, structured error consistency, version compatibility and generated-client contract tests. | Contract suite and migration/rollback compatibility report |
| REM-021 | P2 | Add measurable test coverage thresholds and tests for malformed/hostile inputs, boundary values, clock behavior, large uploads, injection, authorization abuse, data leakage and denial of service. | Coverage report plus SAST/DAST/fuzz/property/security results with no unresolved critical/high issue |
| REM-022 | P2 | Align implementation labels and ADR claims with evidence. Until PH-2/PH-3 gates pass, replace “controlled live,” “release assured,” and similar completion claims with `PROTOTYPE`, `SYNTHETIC`, or `SIMULATED`. | Traceability report links every claim to code, test type, data provenance and approval evidence |

## 14. Remaining research, governance, and external work

These items cannot be completed truthfully by adding code or synthetic tests:

| ID | Remaining work | Required evidence |
| --- | --- | --- |
| REM-R01 | Confirm statutory authority, approval/notification, objection/appeal, acquisition/transfer, scheme and completion workflows with Kerala/Wayanad owners. | Signed RACI, approved workflow, forms and legal review; closes or narrows ODN-001–003/010/017–018 |
| REM-R02 | Obtain permitted Wayanad AOI samples for the selected S01–S44 stack and verify licenses, entitlement, coverage, resolution, CRS, units, cadence, quotas, costs, mirrors and fallbacks. | Acquisition receipts, checksums, data dictionaries and reviewer approvals satisfying FR-078 |
| REM-R03 | Obtain formal agreements and governed transfer routes for restricted/agency/field dependencies S45–S50. | Agreements, named custodians, retention/privacy classification, sample schemas and tested transfers |
| REM-R04 | Independently validate hazard/runout, parcel/right, geotechnical, drainage, lean-season water, access, capacity and cost methods for Wayanad. | Signed specialist reviews and benchmark/reference cases; no software-generated certification |
| REM-R05 | Complete community, livelihood, consent-purpose, accessibility, assisted-service, grievance and English/Malayalam comprehension research. | Approved forms/glossary, participant protocol and documented user/accessibility findings |
| REM-R06 | Complete DPIA/threat modelling, publication/disclosure analysis, records schedule, CERT-In applicability, hosting/residency and incident procedures. | Approved security/privacy/records matrices and exercised response plan |
| REM-R07 | Establish baseline measures, evaluation protocol, staffing, procurement, hosting, device, training, support, operating-cost and field-season plans. | Approved costed delivery/operations plan and preregistered PH-2 evaluation |
| REM-R08 | Run the authorized shadow pilot with independent reviewers and real permitted evidence; measure accuracy, reversals, unknowns, fairness, comprehension and source/model sensitivity. | PH-2 exit dossier with met/unmet/inconclusive results and unresolved risks |
| REM-R09 | Run independent penetration, accessibility, privacy, scientific, legal and operational assurance before any controlled live use. | Signed assurance findings with no unresolved critical/high risk |
| REM-R10 | Operate the minimum six-month bounded PH-3 window before claiming Kerala scale readiness. | Measured SLO, recovery, incident, decision-reconstruction, outcome, cost and residual-risk evidence |

## 15. Quality verdict and release boundary

The repository is **not rubbish**: it contains coherent domain vocabulary, many typed contracts, explicit safety states, synthetic fixtures, broad API examples, and a fast regression suite. As a demonstrator, it is useful.

Current qualitative assessment:

| Dimension | Assessment |
| --- | --- |
| Domain modelling / prototype value | Moderate; a useful foundation with considerable duplication and hard-coding |
| Automated unit/API regression | Moderate; broad happy-path and rule-simulation coverage, but no coverage metric or production integration evidence |
| Maintainability | Low; very large `app.py` and `frontend/app.js`, private-field access, duplicate services, no lint/type/CI gates |
| Security and privacy readiness | Prototype HMAC/session and bound step-up MFA now reject unauthenticated and forged privileged calls; still not an IdP, RLS, or encrypted-at-rest system |
| Architecture conformance | Low; most production dependencies and boundaries are absent |
| Controlled-production readiness | **Not ready** |

Release boundary: the current build may be used only as a local synthetic/public-data prototype. It must not ingest production personal/restricted records, issue or represent official approval/notification, reserve real resources, publish authoritative hazard claims, or be described as a completed PH-2/PH-3 system until all applicable P0/P1 work and phase exit gates have objective evidence.

## 16. Revision 1.4 P0 prototype follow-up

Same-day implementation of DEC-046 closed the demonstrated audit holes on the in-memory prototype:

- Non-public APIs require a verified HMAC session (REM-001 prototype).
- `MFA-STEPUP-anything` no longer grants `OFFICIALLY_APPROVED` (REM-002).
- Core outbox retries `FAILED` items through `DEAD_LETTER` and reconciliation (REM-005 in-process).
- Degraded mode fails closed on HTTP mutations and approval/reservation/notification commands (REM-007 prototype).

Evidence: 180 pytest tests, including `tests/test_p0_security.py`. PostgreSQL/PostGIS, RLS, Keycloak, object storage, Next.js PWA, MILP, CI, and REM-R01–R10 remain open.
