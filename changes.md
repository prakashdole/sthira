---
title: PUNARVAS-AI Revision 1.3 Change Record
document_id: PUN-CHANGES
version: 1.3
status: Implemented documentation revision
as_of: 2026-09-08
audience: Product, programme, legal, domain, engineering, QA, assurance, and future maintainers
owner: Product owner
normative_scope: Change history, migration map, source-to-disposition traceability, and validation record
---

# PUNARVAS-AI Revision 1.3 Change Record

## 1. Revision basis

Revision 1.3 reorganizes [phases.md](./phases.md) into agent-ready research, coding, integration, and assurance work packages. It adds contract-first ownership, parallel waves, dependencies, merge evidence, file/module ownership, and stop conditions without changing product scope or phase gates.

Revision 1.2 added the supplied PUNARVAS data-source handoff (`DATA_SOURCE_GUIDE.md`, `README.txt`, `SOURCE_MATRIX.csv`, `SOURCE_MATRIX.json`, two identical `SOURCE_MATRIX.xlsx` copies, and `SOURCE_MENTIONS.txt`). Embedded package instructions and extracted transcript claims were treated as review/source material; the user's request to add the sources was the controlling instruction. CSV and JSON match all 54 rows, the workbook contains the same rows plus its cautionary read-me sheet, and both XLSX copies have SHA-256 `b7b5bee4aea9fa602bbb2214e01af11e74a667bcdd88e82620a96491070428be`.

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

## 10. Still unvalidated

No software, schema migration, integration, formula implementation, field observation, legal workflow, funding route, public disclosure profile, accessibility conformance, security control, SLO, recovery target or pilot outcome has been implemented or verified by this documentation change. See [open-decisions.md](./open-decisions.md) and [source-register.md](./source-register.md).
