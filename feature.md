---
title: Sthira Feature Catalog
document_id: PUN-FEATURES
version: 1.2
status: Baseline backlog
as_of: 2026-09-08
audience: Product, programme, design, engineering, GIS/data, legal, field, QA, security, and delivery teams
owner: Product owner
normative_scope: User-visible capabilities, dependencies, phase allocation, and feature acceptance
---

# Sthira Feature Catalog

## 1. Document boundary

This catalog packages the product scope in [prd.md](./prd.md) into implementable, testable capabilities. It does not redefine technical contracts from [trd.md](./trd.md), domain restrictions from [rules.md](./rules.md), or component design from [architecture.md](./architecture.md).

Phase IDs refer to [phases.md](./phases.md). Decision IDs refer to [decisions.md](./decisions.md). Equation/parameter IDs refer to [equations.md](./equations.md) and [parameters.md](./parameters.md).

## 2. Phase legend

| ID | Phase |
| --- | --- |
| PH-0 | Governance, evidence, and data agreements |
| PH-1 | Platform foundation and Wayanad sandbox |
| PH-2 | Wayanad shadow pilot |
| PH-3 | Controlled live Wayanad deployment |
| PH-4 | Kerala scaling |
| PH-5 | Multi-state adaptation |

## 3. Feature catalog

### FEAT-001 — Programme and authority configuration

- **Users:** State programme administrator, district administrator, security administrator.
- **Behavior:** Create jurisdiction-scoped programmes; configure pathways, languages, stages, authorities, policy versions, SLAs, publication profiles, effective-dated legal/compliance obligations, and effective dates. Display environment and authority context on every workspace.
- **Dependencies:** Approved operating/legal workflow; identity federation; jurisdiction hierarchy.
- **Requirements:** FR-001–005, FR-075, NFR-012–013, NFR-033.
- **Outcomes/components:** OUT-07, OUT-09; ARC-C01.
- **Rules/decisions:** RUL-001–007; DEC-003, DEC-005, DEC-016.
- **Acceptance:** A user outside the configured programme/geography cannot read or mutate its records; a configuration change creates a version and does not silently change an in-flight case.
- **Phase:** PH-1; production hardening PH-3.

### FEAT-002 — Source catalog and governed ingestion

- **Users:** Data steward, GIS analyst, programme administrator.
- **Behavior:** Register source/license/coverage, upload or fetch approved data, validate file/schema/CRS/geometry/checksum, quarantine defects, compare versions, and approve a version for use.
- **Dependencies:** Object storage, malware scanning, dataset schemas, source agreements.
- **Requirements:** FR-006–011, NFR-010, NFR-024.
- **Outcomes/components:** OUT-01, OUT-03; ARC-C02.
- **Rules/decisions:** RUL-008–014; DEC-004, DEC-009, DEC-019.
- **Acceptance:** An invalid or unlicensed source cannot enter an approved processing run; every derived record reconstructs its full input lineage.
- **Phase:** PH-1; new adapters PH-2–PH-5.

### FEAT-003 — Hazard catalog, map, and authority staging

- **Users:** GIS/hazard analyst, domain reviewer, planner, approver, public user for approved outputs.
- **Behavior:** Browse approved hazard products, coverage and limitations; visualize analytical/verified/official versions distinctly; stage, supersede, or withdraw zones through authorized review.
- **Dependencies:** FEAT-001, FEAT-002, approved hazard sources, map/non-map UI.
- **Requirements:** FR-012–013, FR-017, FR-047, FR-052.
- **Outcomes/components:** OUT-01–03, OUT-07; ARC-C03, ARC-C09–C10.
- **Rules/decisions:** RUL-001–007, RUL-015–020; DEC-005, DEC-015.
- **Acceptance:** An out-of-coverage source is blocked from every decision path; if shown in an authorized isolated research view it is unmistakably `UNSUPPORTED` and cannot enter gates/scenarios/exports. An analytical zone cannot appear official through any channel.
- **Phase:** PH-1 analytical; PH-3 official workflow.

### FEAT-004 — Settlement exposure estimation

- **Users:** GIS analyst, planner, social team.
- **Behavior:** Overlay hazard/runout with settlement, building, infrastructure, and population sources; produce observed versus estimated exposure with uncertainty and reproducible processing metadata.
- **Dependencies:** FEAT-002, FEAT-003; settlement and population baselines.
- **Requirements:** FR-014–018.
- **Outcomes/components:** OUT-02–03; ARC-C03; E03–E05.
- **Rules/decisions:** RUL-015–020; DEC-004, DEC-015.
- **Acceptance:** Wayanad flat-terrain/runout scenario is identified; building-derived estimates cannot create household or beneficiary records.
- **Phase:** PH-1, validated PH-2.

### FEAT-005 — Parcel and record workspace

- **Users:** Revenue/land officer, GIS analyst, legal reviewer.
- **Behavior:** Inspect source parcel geometry, corrected geometry, ULPIN/RoR references, interests, record dates, transformations, and linked evidence without treating any single identifier as title proof.
- **Dependencies:** FEAT-002; authorized Kerala land-record/file access.
- **Requirements:** FR-019–021.
- **Outcomes/components:** OUT-01–02; ARC-C04; E31.
- **Rules/decisions:** RUL-021–027; DEC-004, DEC-009.
- **Acceptance:** Cadastral correction preserves original geometry, GCPs/error metrics, reviewer, and every dependent result's selected version.
- **Phase:** PH-1 sandbox; PH-2 authorized data.

### FEAT-006 — Land-truth discrepancy and legal-readiness workflow

- **Users:** Revenue, legal, forest-rights, social, field, and programme officers.
- **Behavior:** Compare paper records with multi-date imagery, footprints, field occupation/community-use evidence, disputes, FRA/forest status, tenancy, and litigation; route discrepancies and record the applicable legal pathway.
- **Dependencies:** FEAT-002, FEAT-005 and the base observation/evidence contract. Offline synchronization is a runtime integration, not a build prerequisite.
- **Requirements:** FR-021–024, FR-047–050.
- **Outcomes/components:** OUT-01–02, OUT-06; ARC-C04, ARC-C09; E31–E32.
- **Rules/decisions:** RUL-021–027; DEC-005, DEC-015.
- **Acceptance:** “Vacant” plus detected structures creates a discrepancy and hold—not automatic clearance or rejection; a pending FRA issue blocks advancement until authorized review.
- **Phase:** PH-1 workflow; validation PH-2; controlled use PH-3.

### FEAT-007 — Candidate-site nomination and gate board

- **Users:** Planner, GIS analyst, domain reviewers, approver.
- **Behavior:** Nominate a site, connect parcel versions, apply required gate sets, show `PASS/FAIL/UNKNOWN/BLOCKED`, list missing evidence/tasks, and prevent premature advancement.
- **Dependencies:** FEAT-003, FEAT-004, FEAT-005, FEAT-006; configured gate policy.
- **Requirements:** FR-025–027, FR-030–031.
- **Outcomes/components:** OUT-02–03, OUT-07; ARC-C05, ARC-C07, ARC-C09; E02.
- **Rules/decisions:** RUL-028–033; DEC-005, DEC-006.
- **Acceptance:** A site with any mandatory unresolved gate cannot be marked comparable/allocatable/approved; every gate has evidence and reviewer context.
- **Phase:** PH-1, validated PH-2.

### FEAT-008 — Offline field verification

- **Users:** Field verifier, domain specialist, supervisor.
- **Behavior:** Download minimal assigned packages; capture structured observations, photos/documents, GPS accuracy, timestamps, signatures/attestations; sync idempotently; route conflicts.
- **Dependencies:** FEAT-001, FEAT-002, versioned form schemas, approved devices and operating procedure. Domain forms attach through runtime interfaces after their record contracts exist.
- **Requirements:** FR-059–062, NFR-011, NFR-014, NFR-031.
- **Outcomes/components:** OUT-01, OUT-09; ARC-C02, ARC-C04–C06.
- **Rules/decisions:** RUL-008–014, RUL-050–055; DEC-011, DEC-017.
- **Acceptance:** Two offline edits preserve both versions and create a conflict task; expired/revoked packages cannot authorize new work.
- **Phase:** PH-1 prototype; PH-2 field trial; PH-3 managed devices.

### FEAT-009 — Water, infrastructure, livelihood, and capacity assessment

- **Users:** Water/geotechnical/infrastructure specialists, social/livelihood officers, planner.
- **Behavior:** Capture field and documentary evidence by domain; calculate baseline demand; record seasonal water proof, services, accessibility, livelihood continuity, capacity bases, costs, and all active/violated constraints.
- **Dependencies:** FEAT-007, FEAT-008; approved criterion definitions and qualified reviewers.
- **Requirements:** FR-026–031.
- **Outcomes/components:** OUT-02–03; ARC-C05; E09–E18.
- **Rules/decisions:** RUL-028–033; DEC-006.
- **Acceptance:** A site meeting 55 LPCD demand numerically but lacking sustainable lean-season yield/quality fails or remains unknown; capacity preserves all limiting bases without mixing units.
- **Phase:** PH-1 forms; calibration PH-2; approval use PH-3.

### FEAT-010 — Household register, verification, and deduplication

- **Users:** Authorized social team, verifier, grievance officer.
- **Behavior:** Import or enumerate households; restrict member details; link source/evidence; identify potential duplicates; verify/correct records; preserve history.
- **Dependencies:** FEAT-001, FEAT-002, FEAT-008; authorized register/collection basis.
- **Requirements:** FR-032–033, FR-038.
- **Outcomes/components:** OUT-03–04, OUT-09; ARC-C06; E04–E05.
- **Rules/decisions:** RUL-019, RUL-050–054; DEC-010, DEC-017.
- **Acceptance:** Building or exposure records cannot create beneficiaries; suspected duplicates remain reviewable and cannot be silently merged.
- **Phase:** PH-1 synthetic; PH-2 de-identified/authorized; PH-3 production.

### FEAT-011 — Eligibility and transparent priority policy

- **Users:** Programme administrator, social team, reviewer, affected household.
- **Behavior:** Evaluate effective-dated eligibility/priority rules, show rule results and reason codes, compare policy versions, and provide accessible household explanations.
- **Dependencies:** FEAT-010; approved policy and lawful data fields.
- **Requirements:** FR-034–035, FR-040–041.
- **Outcomes/components:** OUT-04–06; ARC-C06–C07; E06–E07, E30.
- **Rules/decisions:** RUL-034–039; DEC-006, DEC-010.
- **Acceptance:** Every inclusion/exclusion/tier is reproducible and explainable; no site score or institutional-readiness value changes household vulnerability.
- **Phase:** PH-1 engine; policy calibration PH-2; controlled use PH-3.

### FEAT-012 — Consent, pathway, preference, and assisted service

- **Users:** Affected household/representative, assisted-service officer, social team.
- **Behavior:** Present approved township and self-relocation options; separately capture processing notice/lawful basis, programme participation, pathway choice, ranked/unacceptable sites, offer acceptance/refusal, any community process, accommodation/livelihood needs, and revisions in English/Malayalam.
- **Dependencies:** FEAT-009, FEAT-010, FEAT-011; accessible content and notice templates.
- **Requirements:** FR-035–038.
- **Outcomes/components:** OUT-04, OUT-06, OUT-09; ARC-C06, ARC-C09.
- **Rules/decisions:** RUL-040, RUL-044–045, RUL-050–052; DEC-014, DEC-018.
- **Acceptance:** No response is never stored as consent; an offline/assisted household can complete the same substantive workflow as a self-service household.
- **Phase:** PH-2 shadow; PH-3 controlled live.

### FEAT-013 — Explainable site comparison and sensitivity

- **Users:** Domain panel, planner, approver, auditor.
- **Behavior:** Compare gate-eligible sites by separate criterion groups; display raw/normalized values, evidence, weights, confidence, contribution, and sensitivity/rank instability.
- **Dependencies:** FEAT-007, FEAT-009; approved MCDA policy.
- **Requirements:** FR-039–041.
- **Outcomes/components:** OUT-03, OUT-05; ARC-C05, ARC-C07; E06, E08–E10, E28–E30.
- **Rules/decisions:** RUL-034–037; DEC-006.
- **Acceptance:** Users can reconstruct every value and see whether plausible weight variation changes ordering; no failed gate is hidden by a high total.
- **Phase:** PH-2 calibration; PH-3 advisory use.

### FEAT-014 — Capacity-constrained allocation scenario builder

- **Users:** Planner, social team, programme reviewers, approver.
- **Behavior:** Freeze approved input versions; generate version-pinned MILP scenarios under an explicitly approved objective; enforce household indivisibility, admissibility, preferences, dwelling/person/accessibility/shared-resource/budget/readiness constraints; retain solver status/bound/gap; independently validate; explain assignments/unassigned cases and compare alternatives. Drafts reserve no capacity.
- **Dependencies:** FEAT-009, FEAT-010, FEAT-011, FEAT-012, FEAT-013; approved solver policy.
- **Requirements:** FR-042–046, FR-071–074, NFR-009–010, NFR-028.
- **Outcomes/components:** OUT-04–06, OUT-11; ARC-C07–C08; E14, E19–E27.
- **Rules/decisions:** RUL-040–043; DEC-007, DEC-018.
- **Acceptance:** A capacity-shortfall scenario returns only independently validated assignments plus explicit unassigned/contributing reasons; status distinguishes optimal, feasible-not-proven-optimal, infeasible, timeout and error; rerunning pinned inputs reproduces objective values and equivalent tie-resolved output.
- **Phase:** PH-2 shadow; PH-3 proposal support.

### FEAT-015 — Workflow, review, and controlled approval

- **Users:** Reviewers, Collector/authorized approver, programme administrator.
- **Behavior:** Route evidence and proposals through configured stages; require domain endorsements; return/reject/approve with reasons; apply step-up authentication; preserve state and versions.
- **Dependencies:** FEAT-001 and the records submitted by the applicable upstream feature; the base workflow engine does not depend on reservation, reporting, or completion features.
- **Requirements:** FR-004, FR-017, FR-030–031, FR-045–050.
- **Outcomes/components:** OUT-06–07; ARC-C09, ARC-C11.
- **Rules/decisions:** RUL-001–006, RUL-043; DEC-005.
- **Acceptance:** Only the configured authority can perform an official transition; missing/expired mandatory endorsement prevents it.
- **Phase:** PH-2 shadow; PH-3 live.

### FEAT-016 — Objection, hearing, appeal, and correction casework

- **Users:** Household/representative, assisted officer, grievance/appeal authority, auditor.
- **Behavior:** File against an exact record/version; issue receipt; manage evidence, admissibility, hearings, decisions, remedies, notices, appeal/reopening, SLA, and dependent holds.
- **Dependencies:** FEAT-010, FEAT-011, FEAT-012, FEAT-013, FEAT-014, FEAT-015; configured order/programme process.
- **Requirements:** FR-047–050.
- **Outcomes/components:** OUT-04, OUT-06; ARC-C09.
- **Rules/decisions:** RUL-044–049; DEC-005, DEC-010, DEC-019.
- **Acceptance:** A list change after an objection creates a new version and traceable notice; no fixed objection period is assumed without configured authority.
- **Phase:** PH-2 rehearsal; PH-3 live.

### FEAT-017 — SLA monitoring and authorized escalation

- **Users:** Case owner, programme administrator, oversight authority.
- **Behavior:** Track overdue tasks/reviews, send reminders, present institutional readiness as operational context, and recommend escalation according to configured authority.
- **Dependencies:** FEAT-001, FEAT-015, FEAT-016.
- **Requirements:** FR-049–051, NFR-026.
- **Outcomes/components:** OUT-06–07; ARC-C09; E32.
- **Rules/decisions:** RUL-006, RUL-060; DEC-016.
- **Acceptance:** Overdue work never changes risk, eligibility, allocation, approval, or authority state; escalation goes only to a configured authorized recipient.
- **Phase:** PH-2 shadow; PH-3 live.

### FEAT-018 — Dossiers, LSG annexes, and evidence-bound exports

- **Users:** Planner, reviewers, approver, LSG staff, auditor.
- **Behavior:** Produce field checklist, site dossier, beneficiary review pack, objection register, decision summary, LSG DM-plan annex, accessible report, machine-readable data, manifest, and checksum from pinned versions.
- **Dependencies:** FEAT-002, FEAT-003, FEAT-004, FEAT-005, FEAT-006, FEAT-007, FEAT-008, FEAT-009, FEAT-010, FEAT-011, FEAT-012, FEAT-013, FEAT-014, FEAT-015, FEAT-016, FEAT-017; approved templates and publication profiles.
- **Requirements:** FR-053–058, NFR-019–022, NFR-029.
- **Outcomes/components:** OUT-01, OUT-07, OUT-10; ARC-C10–C11; E34.
- **Rules/decisions:** RUL-056–059; DEC-012, DEC-013.
- **Acceptance:** Every factual statement is evidence-bound or marked for review; historical exports use original versions; participatory LSG content is never fabricated.
- **Phase:** PH-1 technical exports; PH-2 template validation; PH-3 official use.

### FEAT-019 — Public transparency and household status views

- **Users:** Public visitor; authenticated household/representative.
- **Behavior:** Publish approved criteria, methods, aggregates, progress, notices, supersession, and lawful lists; provide a private household view of its own status, reasons, preferences, notices, and remedy options.
- **Dependencies:** FEAT-001, FEAT-011, FEAT-012, FEAT-015, FEAT-016, FEAT-018; disclosure assessment.
- **Requirements:** FR-035, FR-047–052, NFR-019–022.
- **Rules/decisions:** RUL-007, RUL-038, RUL-044–055; DEC-010, DEC-014.
- **Outcomes/components:** OUT-06, OUT-09; ARC-C06, ARC-C09–C10.
- **Acceptance:** Disclosure threat-model tests cover inference, differencing, linkage and re-identification; authenticated households cannot inspect another household; public state matches the approved artifact.
- **Phase:** Private status in PH-3; public projection after PH-3 disclosure gate; scale PH-4.

### FEAT-020 — Audit, integrity, and assurance console

- **Users:** Auditor, security officer, programme oversight, authorized investigator.
- **Behavior:** Search events by case/object/user/time; verify hash continuity and manifests; inspect overrides, break-glass access, public releases, policy changes, and source supersession; export authorized audit packages.
- **Dependencies:** All state-changing features; security monitoring.
- **Requirements:** FR-056–058, NFR-013, NFR-025–030, NFR-032–033.
- **Outcomes/components:** OUT-01, OUT-07, OUT-09; ARC-C11; E34.
- **Rules/decisions:** RUL-056–060; DEC-012.
- **Acceptance:** An auditor reconstructs a selected official decision and detects a missing/altered audit event or mismatched export hash without receiving unnecessary personal data.
- **Phase:** PH-1 baseline; independent assurance PH-3.

### FEAT-021 — Operations and data-quality console

- **Users:** Platform operator, data steward, security/operations lead.
- **Behavior:** Monitor integrations, datasets, jobs, queues, backups, storage integrity, stale sources, failed validations, expiring evidence, and service health with separate operational/data/case alerts.
- **Dependencies:** Observability and all processing features.
- **Requirements:** FR-009–010, FR-031, NFR-006–008, NFR-025–033.
- **Outcomes/components:** OUT-01, OUT-03, OUT-07; ARC-C02, ARC-C11.
- **Rules/decisions:** RUL-008–014; DEC-008, DEC-011.
- **Acceptance:** Operators can distinguish unavailable service from overdue human review and stale evidence; recovery exercise demonstrates the defined RPO/RTO.
- **Phase:** PH-1; production hardening PH-3.

### FEAT-022 — Relocation-necessity and alternatives review

- **Users:** Hazard/geotechnical reviewers, planners, social/community team, authorized programme decision-maker.
- **Behavior:** Record settlement/community scope, hazard evidence, feasible in-situ mitigation, residual risk, relocation alternatives, uncertainty, distributional/community effects, qualified reviews, reasons and authority state.
- **Dependencies:** FEAT-002, FEAT-003, FEAT-004 and applicable field/domain evidence; no dependency on household allocation.
- **Requirements:** FR-063, FR-070, FR-075.
- **Outcomes/components:** OUT-02–03, OUT-12; ARC-C03, ARC-C05, ARC-C09.
- **Rules/decisions:** RUL-001–005, RUL-067; DEC-005, DEC-015, DEC-026.
- **Acceptance:** Hazard exposure alone cannot mark relocation necessary; a reviewed in-situ alternative and an `UNKNOWN` outcome remain representable.
- **Phase:** PH-1 synthetic workflow; PH-2 domain validation; PH-3 controlled use.

### FEAT-023 — Scheme, entitlement, and funding assessment

- **Users:** Programme/finance officer, social officer, legal reviewer, affected household.
- **Behavior:** Version schemes and cost-head rules; assess owner/tenant/landless/other categories; preserve relocation need separately; track application, sanction, release, receipt and shortfall; explain and support appeal.
- **Dependencies:** FEAT-001, FEAT-010, FEAT-012, FEAT-022; effective scheme/order and finance controls.
- **Requirements:** FR-064–067, FR-075.
- **Outcomes/components:** OUT-04, OUT-06, OUT-11; ARC-C06, ARC-C09, ARC-C12; E17–E18.
- **Rules/decisions:** RUL-068–069; DEC-018, DEC-026.
- **Acceptance:** A tenant failing an owner-only scheme remains relocation-needy where assessed, receives reasons/remedy, and cannot be assigned invented assistance.
- **Phase:** PH-1 contract/synthetic; PH-2 shadow; PH-3 controlled use after finance approval.

### FEAT-024 — Capacity reservation and commitment ledger

- **Users:** Planner, approver, finance/programme administrator, auditor.
- **Behavior:** Revalidate an approved scenario and atomically reserve or commit dwellings, people, accessibility resources, budgets, overlapping land and shared services; expire/release/replan with full history.
- **Dependencies:** FEAT-007, FEAT-009, FEAT-014, FEAT-015, FEAT-023 and the independent feasibility validator.
- **Requirements:** FR-042–046, NFR-009–010, NFR-028.
- **Outcomes/components:** OUT-04, OUT-07, OUT-11; ARC-C08–C09, ARC-C12; E14, E19–E26.
- **Rules/decisions:** RUL-070–074; DEC-007, DEC-025, DEC-027.
- **Acceptance:** Concurrent approvals cannot reserve the same capacity; drafts reserve nothing; every solver/manual result passes identical validation.
- **Phase:** PH-2 concurrency rehearsal; PH-3 controlled live.

### FEAT-025 — Delivery, handover, occupation, and follow-up

- **Users:** Programme/delivery officer, infrastructure reviewer, social/livelihood officer, household, auditor.
- **Behavior:** Track funding confirmation, unit and service readiness, defects, offer/acceptance, possession/handover, occupation, transition support, livelihood follow-up, or an explicit reconciled external-system handoff.
- **Dependencies:** FEAT-015, FEAT-018, FEAT-023, FEAT-024; approved completion criteria and accountable delivery owner.
- **Requirements:** FR-068–070.
- **Outcomes/components:** OUT-11–12; ARC-C09–C10, ARC-C12; E33 completion metrics.
- **Rules/decisions:** RUL-069, RUL-071–072; DEC-026.
- **Acceptance:** Approval or ceremony cannot close a case when required services, possession, occupation or defects remain unresolved.
- **Phase:** PH-2 workflow rehearsal; PH-3 live or explicit external handoff.

### FEAT-026 — Source acquisition and readiness management

- **Users:** Data steward, GIS analyst, integration operator, domain reviewer, security/procurement reviewer, auditor.
- **Behavior:** Maintain the S01–S54 source catalog; distinguish product, catalog, download, processing, display, agency, and field routes; configure endpoints and governed-file fallbacks; record entitlement/quota/cost/license/coverage; acquire and validate a permitted AOI sample; group mirrors/shared inputs; map every mandatory gate/criterion/parameter to an acquisition method; and publish a release-readiness report.
- **Dependencies:** FEAT-001, FEAT-002, FEAT-020, FEAT-021; provider/agency terms, approved credentials, object storage, and domain reviewers.
- **Requirements:** FR-006–010, FR-076–084, NFR-025–027, NFR-034–035.
- **Outcomes/components:** OUT-01, OUT-03, OUT-07; ARC-C02, ARC-C11, ARC-C13.
- **Rules/decisions:** RUL-008–017, RUL-076–083; DEC-004, DEC-011, DEC-015, DEC-030–032.
- **Acceptance:** CSV/JSON/workbook S01–S54 coverage reconciles exactly; a catalog-only result remains unusable; bad license/CRS/unit/NoData/checksum/coverage quarantines the sample; shared Sentinel or footprint observations are not double-counted; every missing mandatory dependency yields `UNKNOWN`/`HOLD`; and no credential is exposed in UI, logs, or exports.
- **Phase:** Register and prototype samples in PH-0/PH-1; restricted agency/field activation in PH-2/PH-3; district-specific onboarding in PH-4/PH-5.

## 4. Cross-feature release slices

| Slice | Minimum features | Demonstrated outcome |
| --- | --- | --- |
| Wayanad evidence sandbox | 001–011, 018, 020–023 and 026 foundations | The minimum public-data stack, synthetic households/parcels, versioned KSDMA/GSI/PDNA evidence, land discrepancy, formulas and scheme fixtures can be inspected and reproduced safely |
| Land/site shadow decision | 006–009, 013, 015, 018 | Paper-versus-ground discrepancy and site gates produce a reviewable, evidence-bound dossier |
| Household participation shadow | 010–012, 014, 016 | De-identified/authorized households can be prioritized, express choices, receive feasible scenarios, and object |
| Controlled official workflow | 015–025 plus production controls | Only authorized, feasible, funded/readiness-aware and privacy-reviewed outputs progress; completion is separately evidenced |

## 5. Feature-wide acceptance rules

- No feature may introduce an excluded PRD capability through an “optional” toggle.
- Every state-changing feature must emit an audit event and use optimistic concurrency/idempotency as applicable.
- Every map capability must have a non-map equivalent for critical tasks.
- Every algorithmic feature must expose inputs, versions, limitations, explanation, and human decision boundary.
- Every executable calculation must identify its `E*` formula and `PAR-*` versions and pass dimensional, domain, missing-input, uncertainty, and formula-specific fixtures.
- Every feature handling personal or sensitive geospatial data must pass role/geography/classification and export-leakage tests.
- Every phase activation follows the gates in [phases.md](./phases.md); completion by engineering alone is insufficient.

## 6. Primary references

See the research basis in [prd.md](./prd.md#13-primary-research-basis), technical standards in [trd.md](./trd.md#10-standards-and-primary-references), and rationale/evidence per decision in [decisions.md](./decisions.md).
