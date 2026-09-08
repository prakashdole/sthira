---
title: PUNARVAS-AI Delivery and Rollout Phases
document_id: PUN-PHASES
version: 1.3
status: Baseline delivery plan
as_of: 2026-09-08
audience: Programme sponsors, government owners, product, delivery, engineering, GIS/data, legal, field, security, QA, and operations
owner: Programme director and product owner
normative_scope: Delivery sequence, entry/exit gates, evidence, rollout, and stop conditions
---

# PUNARVAS-AI Delivery and Rollout Phases

## 1. Rollout principles

- Stage progression is evidence-gated, not deadline-gated. Indicative durations support planning but do not waive exit criteria.
- Start with Wayanad and approved public/synthetic/de-identified data; introduce personal and restricted data only after controls and agreements are proven.
- Shadow outputs are compared with authorized human work and cannot become official by convenience.
- Domain, legal, community, privacy, accessibility, security, and operational readiness are release criteria alongside software quality.
- Every phase can pause, narrow, or roll back without deleting evidence or decision history.
- National/multi-state scale comes from configurable policies and adapters, not Kerala assumptions embedded in code.

## 2. Stage overview

| Phase | Indicative duration | Operating mode | Primary outcome |
| --- | --- | --- | --- |
| PH-0 | 6–10 weeks | Research/governance | Authorized problem, data, legal, ethics, and validation baseline |
| PH-1 | 14–20 weeks | Sandbox | Secure end-to-end foundation on synthetic/public Wayanad data |
| PH-2 | 12–16 weeks | Shadow pilot | Field and historical validation without official decision reliance |
| PH-3 | Minimum 6 months | Controlled live | Authorized Wayanad production use with independent assurance |
| PH-4 | 6–12 months after PH-3 evidence | Kerala scale | Multiple districts and state-level governance/operations |
| PH-5 | Programme-specific | Multi-state | Reusable platform with state-specific legal/data configuration |

Durations begin only when the entry gate is met and must be replanned when data agreements, statutory procedures, or field seasons change.

PH-0 through PH-2 therefore represent **32–46 weeks before controlled live use**, assuming timely staffing and access. This is not a hackathon schedule. A separate 4–8 week, scope-capped prototype may demonstrate public/synthetic evidence → exposure → parcel discrepancy → site gates → sample household preferences → feasible scenario → dossier, but it cannot use production personal data, reserve capacity, issue official output, or claim scientific, legal, compliance, or production validation.

## 3. PH-0 — Governance, evidence, and data agreements

### Objectives

- Establish named programme authority, product owner, domain stewards, decision rights, and pilot operating boundaries.
- Map the actual Wayanad/Kerala workflow for hazard review, site review, beneficiary identification, publication, objections/appeals, acquisition/transfer, FRA/forest issues, LSG plans, approval, and signing.
- Reconcile and own all S01–S54 entries, separating products, catalogs, processing services, display basemaps, agency records, and field acquisition; record licenses, custodians, coverage, quality, cadence, authentication, quotas/costs, residency, sharing conditions, and fallbacks.
- Establish baseline process time/error/rework measures for later outcome comparison.
- Approve pilot criteria, mandatory gates, terminology, translation, privacy notice, retention, public disclosure, and community-participation approach.
- Define relocation-necessity/alternatives review, scheme/entitlement/funding, consent-purpose separation, reservation, and completion/handoff ownership.
- Approve or leave explicitly open every active equation, parameter, allocation objective, solver tolerance, and pilot evaluation threshold.

### Deliverables

- Programme charter and RACI with authorized signatories and escalation recipients.
- Legal/workflow matrix tied to current orders and review triggers.
- Data inventory and signed/recorded data-sharing approvals.
- Wayanad source register and parameter-dependency map covering S01–S54, including the public minimum stack, alternate/mirror relationships, exact acquisition method, and the S45–S50 restricted/field blockers.
- Data protection impact/risk assessment, geospatial compliance assessment, threat model, ethics/fairness plan, and records-retention schedule.
- Reference test dataset with synthetic households and approved public geospatial data.
- Baseline metrics and shadow-pilot evaluation protocol.
- Versioned equation/parameter registry, dated source register, open-decision register, compliance applicability register, scheme/funding matrix, and completion-state map.
- Costed delivery plan covering staffing, procurement, hosting, basemap/offline rights, data agreements, field devices, training, security/accessibility assurance, and ongoing support.
- Source acquisition plan that names an owner and approved AOI sample for each required public source, plus formal request/fieldwork plans for agency and field blockers; documentation/catalog visibility is not counted as access.

### Exit gate

- Programme authority, legal, privacy, security, GIS/hazard, revenue/land, social, field, accessibility, and operations owners sign their relevant baseline.
- No unresolved issue would make possession or processing of planned pilot data unlawful or unsafe.
- Required policies have owners, source authority, effective date, and test examples.
- Community/household participation design has an assisted-service route and does not presume consent.
- Every PH-1 mandatory public dependency has either a validated permitted sample or an explicit `UNKNOWN`/`HOLD`, owner, fallback, and due gate; no untested source is described as operational.

### Stop conditions

- No lawful basis/data agreement for household or restricted land data.
- No authorized owner for official states, objections, or publication.
- Proposed criteria cannot be supported or validated with available evidence.

## 4. PH-1 — Platform foundation and Wayanad sandbox

### Objectives

- Implement the modular platform, core contracts, access model, provenance/versioning, object storage, audit, asynchronous jobs, map/non-map shell, and observability.
- Demonstrate end-to-end public/synthetic flows before introducing production personal data.

### Included features

FEAT-001–011 foundations (including FEAT-006 land discrepancy), FEAT-018 technical exports, FEAT-020–023 and FEAT-026 foundations, and an FEAT-008 offline prototype.

### Required demonstrations

- Ingest a KSDMA/GSI layer, reject/quarantine a faulty fixture, version a replacement, and reproduce an exposure result.
- Acquire or governed-import permitted AOI samples for SOI/LGD, KSDMA/GSI, one DEM, Sentinel-2, buildings, Census, land cover, roads/access, water context, and rainfall context; record exact license, dates, coverage, schema/CRS/units/NoData/checksum, and reviewer.
- Demonstrate that CDSE catalog visibility is not download success; group an alternate-catalog Sentinel item as the same observation; keep Earth Engine, Bhoonidhi, and paid/specialist routes disabled until their own entitlement/terms tests pass.
- Preserve source and corrected sample cadastral geometry with transformation error.
- Detect a paper-versus-ground discrepancy without making a legal conclusion.
- Apply site hard gates, demonstrate inadequate lean-season water, and show missing evidence.
- Create synthetic household eligibility explanations without using building counts as identities.
- Generate accessible HTML, machine-readable data, manifest, and verified checksum.
- Prove programme/geography isolation, restricted-field redaction, audit continuity, backup restore, and non-map operation.

### Exit gate

- All PH-1 feature acceptance and AT-01–AT-05, AT-09 as a configured escalation foundation, AT-12–AT-14, AT-17–AT-20, and AT-23–AT-28 pass on synthetic/reference data.
- Security review finds no unresolved critical/high risk; accessibility review covers the critical paths.
- Restore test meets provisional RPO/RTO; processing is reproducible from pinned inputs.
- Operations, support, incident, data-correction, and rollback runbooks are usable by staff other than their authors.
- FR-076–084, NFR-034–035, and AT-31–38 pass; the source-readiness report has no falsely green mandatory dependency.

## 5. PH-2 — Wayanad shadow pilot

### Objectives

- Validate system conclusions alongside—but never in place of—the authorized existing process.
- Test field evidence, legal/land discrepancy handling, water/site criteria, household participation, policy sensitivity, allocation scenarios, objections, and LSG output with real users.

### Included features

FEAT-006–025 in shadow/rehearsal mode, including a mock or de-identified FEAT-019 household view. Authenticated live household access remains PH-3 unless separately authorized.

### Method

- Select a bounded set of historical and current cases approved by the programme authority.
- Freeze source and policy versions for each comparison.
- Blind or sequence reviews where feasible so the tool does not merely reproduce the official answer.
- Record false positives/negatives, unknowns, reversals after field/legal review, evidence age, processing time, workload, rank sensitivity, unassigned reasons, preference satisfaction, accessibility failures, and user comprehension.
- Pre-register sampling frame/size, inclusion/exclusion, independent reference assessors, seasonal observation windows, tolerances, missing-data handling, subgroup/privacy analysis, leakage controls, and sign-off before examining outcome results.
- Separate retrospective replay from prospective observation; neither is called prediction unless the evaluated model and temporal design support that claim.
- Conduct Gram/Ward Sabha or other participation only through the authorized government process; record product feedback without representing the shadow output as a decision.

### Required scenarios

- Wayanad channelized debris flow affecting locally flat terrain.
- Paper-vacant parcel with observed structures/community use.
- Cadastral offset and disputed source versions.
- Pending FRA/forest/legal issue.
- Seasonal water shortfall.
- Changed beneficiary list plus objection and notice.
- Township versus self-relocation choice.
- Capacity/accessibility constraint leaving households explicitly unassigned.
- Overdue workflow recommending, but not executing, escalation.
- Offline conflict and lost-device exercise.
- Competing scenarios and overlapping sites attempting to reserve the same dwelling, land, budget, or shared water.
- Owner/tenant/landless scheme cases, incomplete funding, distinct consent purposes, conditional approvals, solver timeout, accessible non-map/PDF use, and incomplete unit/services after approval.
- Missing/stale source, expired API entitlement, paused endpoint, mirror duplication, and governed-file fallback without weakening provenance or converting `UNKNOWN` to `PASS`.

### Exit gate

- Domain reviewers approve calibrated gate/criterion definitions and document error/uncertainty limits.
- The programme authority accepts shadow comparison results and residual risks.
- Median complete-dossier preparation time is measured against comparable cases with predefined start/end points and missing-case handling. The 30% target is reported as met, unmet, or inconclusive—not assumed.
- No unresolved critical privacy, security, accessibility, fairness, legal-authority, or data-quality failure remains.
- Objection, correction, notice, and policy-version changes are successfully rehearsed end-to-end.
- The controlled-live scope, users, data, publication profile, and rollback boundary are explicitly approved.
- Applicable S45–S50 land/right, household, field/geotechnical, lean-season water/service, and programme/funding evidence has passed its owner review for every live action; open blockers remain outside the live scope.

## 6. PH-3 — Controlled live Wayanad deployment

### Objectives

- Use PUNARVAS-AI for a bounded authorized workflow while retaining human decision authority and parallel contingency procedures.
- Prove operational reliability, remedy, audit, public disclosure, and organizational adoption over at least six months.

### Controls

- Limit initial live use by named programme, geography, case type, users, templates, and decisions.
- Require MFA/step-up for approval, publication, restricted export, policy activation, and break-glass access.
- Hold weekly operational/data-quality review and monthly programme/fairness/privacy review during the controlled period.
- Maintain an approved manual continuity route; reconcile any offline/manual decision back into the system with evidence and audit.
- Release public projections only after legal/privacy/disclosure review and a household harm check.

### Exit gate

- Availability, RPO/RTO, performance, accessibility, localization, security, data-quality, and incident targets are met for the agreed evaluation window.
- Independent reviewers can reconstruct sampled decisions and exports from source to approval.
- Affected households successfully use assisted correction/objection pathways; notices and remedies are evidenced.
- No material decision was presented as official before authorization; no unresolved unauthorized disclosure occurred.
- Programme owners approve quantified outcomes, limitations, staffing model, support cost, and Kerala expansion proposal.
- Required delivery/completion data are either verified in PUNARVAS-AI or reconciled from a named accountable external system; approval is not counted as completed relocation.

### Rollback

- Stop new decisions/publication while retaining read-only approved artifacts and audit access.
- Revert routing to the authorized manual process, notify affected users/owners, reconcile outstanding cases, and preserve all records.
- A rollback does not invalidate a legally issued decision; its authority determines correction/withdrawal.

## 7. PH-4 — Kerala scaling

### Objectives

- Add districts through repeatable onboarding, not shared unrestricted access.
- Operationalize state oversight, district isolation, common source adapters, policy inheritance/overrides, bilingual support, training, and support.

### Work

- Validate district hazard/source coverage; never assume Wayanad or C-FLOOD availability elsewhere.
- Configure district/LSG workflows, authorities, publication profiles, and local evidence.
- Add scale testing, queue quotas, source-health SLAs, statewide dashboards based on approved aggregates, and district support tiers.
- Establish policy-governance board, release calendar, statewide accessibility/security assurance, and periodic field calibration.

### Exit gate

- At least two materially different districts complete onboarding and controlled use without source, legal, or policy assumptions leaking between them.
- State and district roles demonstrate row/geography isolation, oversight, local remedy, and continuity.
- Scale tests meet NFR targets and operating costs/capacity are approved.
- Each district has trained owners and current data/source agreements.

## 8. PH-5 — Multi-state adaptation

### Objectives

- Reuse the platform while keeping state-specific law, records, terminology, languages, hazards, and programme authority explicit.

### Required adaptation package

- State legal/workflow map and reviewed policy set.
- Data adapters, licenses, residency/hosting approval, boundary sources, language pack, templates, authorities, publication and remedy processes.
- Local hazard/product coverage and scientific validation.
- State/district reference fixtures and regression scenarios.
- Migration/isolation plan preventing Kerala-specific codes or personal data from leaking into the new tenant/jurisdiction.

### Exit gate

- A second state completes PH-0 through PH-3 gates for its own scope.
- No Kerala-specific rule is treated as national law; shared rules identify their national authority.
- Independent security/privacy and domain assurance confirms tenant/jurisdiction isolation and local correctness.

## 9. Planning assumptions and contingencies

The dates above assume a cross-functional core team covering product/delivery, domain/GIS/data, frontend/backend, QA, security/privacy, accessibility/localization, DevOps/operations, legal/programme policy, field coordination and community participation. PH-0 must turn this into named availability/FTEs and a budget; the plan does not invent staffing counts.

| Dependency | Planning requirement | Contingency |
| --- | --- | --- |
| Data/API agreement | S01–S54 owner, permissible purpose, fields, update cadence, entitlement/quota/cost, AOI sample, and lead time | Governed file exchange or synthetic/public scope; no scraping restricted systems and no catalog-only access claim |
| Procurement/hosting | Costed profile, security approval, contract and support | Prototype remains local/container-only; live gate moves rather than weakening controls |
| Field season/water | Lean-season observation window and qualified staff | Pilot schedule shifts or water stays `UNKNOWN`; monsoon values are not substituted |
| Basemap/offline content | License, attribution, cache/offline rights and quotas | Use approved minimal reference layers or defer offline basemap |
| Translation/accessibility | Human reviewers and assistive-technology participants | Block household-facing release until equivalent access is proven |
| Solver/policy approval | Objective, parameters, fairness and validator | Run labelled research scenarios only; do not reserve or approve |
| External delivery systems | Interface owner, status semantics and reconciliation | Record explicit handoff and stale/unknown status; never infer completion |

Schedule changes must identify the affected gate, cost, scope and risk. Removing evidence, participation, privacy, security or scientific validation is not an acceptable schedule contingency.

## 10. Cross-phase metrics

| Category | Measures |
| --- | --- |
| Evidence | S01–S54 readiness, permitted AOI samples, provenance completeness, license/coverage/age, entitlement/quotas, mirror groups, quarantines, unresolved conflicts, and reproducible jobs |
| Safety/quality | Gate reversals after field review, unsupported coverage attempts, unknown/blocked aging, spatial regression failures |
| Fairness/participation | Explanation comprehension, assisted-service completion, preference satisfaction, unassigned reasons, objection outcomes and time |
| Authority | Premature official/public state attempts, overrides, expired approvals, notice completion, escalation recommendations versus actions |
| Privacy/security | Access denials, export leakage tests, incidents, high-risk field access, break-glass use, patch/scan posture |
| Accessibility | WCAG/GIGW defects, keyboard/screen-reader completion, translation defects, non-map parity |
| Delivery/operations | Dossier cycle time, availability, job failure/retry, performance percentiles, RPO/RTO exercises, support volume |

Metrics are disaggregated only where lawful and safe. A metric cannot become an eligibility or priority factor unless separately approved as policy.

## 11. Release governance

- Product owner accepts scope and outcome evidence.
- Domain stewards accept their criteria, evidence requirements, and known limits.
- Legal/programme authority accepts workflow and authority mapping.
- Privacy/security/accessibility owners accept their controls and residual risks.
- Operations accepts monitoring, support, backup, recovery, and rollback.
- Community/household-facing changes undergo comprehension and assisted-service validation.
- Any failed mandatory gate keeps the release in its current phase regardless of schedule pressure.

## 12. Agent-ready parallel work breakdown

This section converts the phase gates above into bounded assignments for separate research and coding agents. It does not authorize work from a later phase, weaken a gate, or add product scope. One agent owns one package at a time; agents working simultaneously must use separate branches/worktrees and avoid editing another package's owned module or migration.

### 12.1 Coordination rules

1. **Contract-first:** complete PKG-0C before parallel production coding. It freezes identifiers, API/error envelopes, domain-event shape, audit fields, source metadata, authorization context, state enums, units, and test-fixture conventions.
2. **Research produces executable inputs:** every research package ends with versioned sources, decisions/open decisions, data dictionaries, example cases, and acceptance fixtures—not free-form notes alone.
3. **Coding uses safe fixtures:** PH-0/PH-1 agents use public, synthetic, or explicitly approved data. No agent may obtain restricted records by scraping or substitute synthetic/open data for an S45–S50 production dependency.
4. **Single ownership of shared surfaces:** one integration agent owns shared OpenAPI schemas, database migrations, event contracts, dependency versions, global design tokens, and CI configuration. Feature agents propose changes through that owner.
5. **Vertical packages:** each coding package includes storage/domain logic, API, UI/non-map behavior, authorization, audit, tests, and documentation for its bounded capability. “Frontend done” or “backend done” alone is not a completed package.
6. **Mocks are explicit:** an unavailable integration receives a contract mock labelled `SYNTHETIC`, `UNAVAILABLE`, or `NOT_ENTITLED`; a mock response can never set an evidence or gate state to approved/pass.
7. **Merge evidence:** every package hands off changed files, migrations, API/schema diff, test command/results, fixture provenance, unresolved risks, affected IDs, and rollback notes.
8. **Integration cadence:** merge contract changes first, then independent domain packages, then cross-domain workflows. Run the phase acceptance set after each integration wave, not only at the end.

### 12.2 Shared contract package — complete before broad parallel coding

| Package | Type | Owner profile | Work and outputs | Blocks/unlocks |
| --- | --- | --- | --- | --- |
| PKG-0C | Research + coding contract | Lead architect, data architect, QA contract owner | Freeze domain boundaries ARC-C01–13; common IDs/time/version/state/audit/source contracts; role/geography/classification context; API errors/idempotency/concurrency; event/outbox envelope; test fixture manifest; module/file ownership map; migration naming and compatibility rules | Uses PH-0 decisions and RUL-001–083. Unlocks all `C1-*`; later changes require impact review and contract tests. |

### 12.3 PH-0 parallel packages — governance and build preparation

These packages may start together. `R0-*` findings feed PKG-0C and the indicated PH-1 coding package.

| Package | Type | Scope and required output | Feeds | Dependency/finish rule |
| --- | --- | --- | --- | --- |
| R0-01 | Research | Statutory/programme authority, Wayanad workflow, approvals/notices, objections/appeals, escalation, acquisition/transfer, LSG integration, necessity and completion RACI | C1-01, C1-08 | Closes or narrows ODN-001–003, 010, 017–018; signed workflow fixtures required. |
| R0-02 | Research | Acquire and validate permitted AOI samples for the S01–S44 minimum public stack; licenses, entitlement, quota/cost, coverage, schemas, mirror groups, fallbacks, checksums | C1-02, C1-03 | Must satisfy FR-078 for selected samples; catalog visibility alone is incomplete. |
| R0-03 | Research | Formal access plan for S45–S50: land/right, forest/FRA, water/operator, household, qualified field, programme/funding records | C2-01–C2-04 | Produces agreements/request letters, data dictionaries, privacy/retention classification, owners, and lead times; does not block PH-1 synthetic work. |
| R0-04 | Research | Hazard/exposure and land/site methodology: Wayanad reference cases, runout/channel evidence, population/footprint limits, terrain/CRS rules, source-specific uncertainty and field-review criteria | C1-03, C1-04 | Outputs reviewed fixtures for AT-01–05, 17, 30–36; cannot approve specialist models. |
| R0-05 | Research | Household, livelihood, community, consent-purpose, assisted-service, objection, English/Malayalam terminology, and accessibility research | C1-05, C1-08, C1-09, C1-10 | Produces minimal field dictionary, purpose-specific forms, translation glossary, comprehension/accessibility cases; never a fabricated beneficiary list. |
| R0-06 | Research | Privacy, security, records, geospatial publication, threat model, disclosure/inference controls, incident/CERT-In applicability | C1-01, C1-11, C1-12 | Produces classification/retention/access matrices and abuse cases for AT-12, 23–28. |
| R0-07 | Research | Baseline process measures, preregistered evaluation, economics, staffing, procurement, hosting, basemap/offline rights, support and field-season plan | C1-09, C1-12; PH-2 | Closes/narrows ODN-014–016, 020–022; provides measurable entry/exit evidence. |
| C0-01 | Coding spike | Reproducible developer environment, lint/test/docs checks, dependency and secret scanning, synthetic fixture loader | PKG-0C, all C1 packages | Disposable/non-production until reviewed; no restricted data or production-readiness claim. |
| C0-02 | Coding spike | Accessible bilingual navigation/form prototypes and map-independent Wayanad workflow using synthetic records | C1-05, C1-09, C1-10 | Validate information architecture and terminology with R0-05; do not create binding UI states. |
| C0-03 | Coding spike | PostGIS spatial fixture, object/checksum flow, RLS context, outbox/idempotency, offline conflict, and accessible HTML/PDF proof-of-concepts | C1-01, C1-02, C1-10–12 | Each spike ends with a keep/reject decision and test evidence; spike code is not automatically production code. |

### 12.4 PH-1 coding packages — platform foundation

After PKG-0C freezes shared contracts, Wave A packages can run concurrently. Wave B may start against contract mocks but cannot finish until its named Wave A dependencies integrate.

#### Wave A — independent foundations

| Package | Primary ownership | Features/components | Coding deliverable | Research input and completion gate |
| --- | --- | --- | --- | --- |
| C1-01 | Programme, identity, authorization and compliance | FEAT-001; ARC-C01 | Programme/jurisdiction/policy context, OIDC/SAML broker integration boundary, MFA hooks, role/geography/classification authorization, RLS context, compliance register, audit hooks | R0-01/R0-06; FR-001–005/075 and authorization portions of AT-12/24/28 pass. |
| C1-02 | Source registry, acquisition and ingestion | FEAT-002/026; ARC-C02/C13 | S01–S54 registry import, endpoint/entitlement states, governed upload, selected public connectors, immutable object/checksum, validation/quarantine, mirror groups, lineage and readiness report | R0-02; FR-006–011/076–084, NFR-034–035, AT-14/30–38 pass. |
| C1-03 | Hazard, exposure and geospatial delivery | FEAT-003/004; ARC-C03 | Approved hazard/settlement/building/population ingestion, coverage enforcement, reproducible overlay, uncertainty, authorized OGC/tiles/COG and equivalent tables | R0-02/R0-04 plus C1-02 contract; FR-012–018, AT-01/10/30/34/36 pass. |
| C1-05 | Household and participation foundation | FEAT-010–012; ARC-C06 | Synthetic household/member import, duplicate/merge history, necessity/eligibility/priority explanation, purpose-specific participation/preferences and bilingual assisted/non-map forms | R0-05 plus C1-01 authorization; FR-032–038/063–065, AT-06/07/18/19/29 pass on synthetic data. |
| C1-06 | Policy, equations and comparison engine | FEAT-005/013; ARC-C07 | Versioned rule/formula/parameter activation, hard gates, separate dimensions, explanations, sensitivity and deny-list for rejected/specialist methods | R0-04 and approved registries; FR-039–041/071–074, AT-17/20/29/36 pass. |
| C1-11 | Security and privacy controls | Cross-cutting; ARC-C01/C11/C13 | Key/secret integration, encryption/tokenization boundaries, CSP/security headers, object/tile/report authorization tests, disclosure controls, security automation | R0-06; no unresolved critical/high issue and NFR-012–018/029–031 pass for PH-1 paths. |
| C1-12 | Delivery platform and operations | FEAT-020/021; ARC-C11 | CI/CD, migrations, container profiles, observability, source/job/data-quality alerts, backup/restore, outbox relay, dead-letter/reconciliation, runbooks | R0-06/R0-07; NFR-006–008/023–028/032–035 and AT-25/27 pass. |

#### Wave B — domain features on the frozen foundations

| Package | Primary ownership | Features/components | Coding deliverable | Dependencies and completion gate |
| --- | --- | --- | --- | --- |
| C1-04 | Land truth and site assessment | FEAT-006–009/022; ARC-C04/C05 | Parcel/source/corrected geometry, interests/occupation/claims, discrepancy tasks, candidate sites, gates, criteria, water/capacity/cost, necessity alternatives and field-review contracts | C1-02/03/06 plus R0-01/03/04; FR-019–031/063 and AT-02–05 pass on sample/synthetic data. |
| C1-07 | Allocation scenario and validator | FEAT-014/024 foundation; ARC-C08 | Pinned scenario inputs, MILP adapter boundary, explicit unassigned outcome, diagnostics/explanations, independent feasibility validator, simulation-only reservation model | C1-05/06 and site-capacity contract from C1-04; FR-039–046/073–074 and AT-08/15–17 pass without live reservation. |
| C1-08 | Governance, remedy, schemes and completion | FEAT-015–017/022–025 foundations; ARC-C09/C12 | Tasks/SLA, objections/hearings/decisions/notices, scheme/entitlement/funding state, reservation/delivery/completion state contracts and synthetic workflows | C1-01/05/06 plus R0-01/05; FR-047–052/064–070 and AT-09/18–22 pass. |
| C1-09 | Reporting and publication | FEAT-018/019; ARC-C10 | Accessible HTML, controlled PDF, CSV/GeoJSON/GeoPackage, LSG annex mapping, manifests/signature state, public/private projections and disclosure review | C1-01–08 stable read contracts plus R0-05/06; FR-053–058, NFR-019–022/029 and AT-12–14/21/23/24 pass. |
| C1-10 | Offline field workflow | FEAT-008 offline path; field parts of ARC-C04–06 | Scoped encrypted packages, form/schema versioning, GPS/evidence hashes, idempotent sync, conflict review, expiry/eviction/lost-device recovery state | C1-01/04/05/11 plus R0-05/06; FR-059–062, NFR-031, AT-11/26 pass. |

#### PH-1 integration packages

| Package | Type | Work and exit evidence |
| --- | --- | --- |
| I1-01 | Integration | Merge Wave A contracts; run schema/API/event compatibility, RLS, outbox, source-ingestion, object and audit tests. No Wave B merge before this passes. |
| I1-02 | Integration | Merge Wave B in dependency order C1-04/05/06 → C1-07/08 → C1-09/10; resolve only through contract owner, never by cross-module direct writes. |
| I1-03 | End-to-end QA | Execute all PH-1 demonstrations and required AT scenarios, accessibility/security review, restore, source-readiness report, and traceability. PH-1 exits only through §4 gates. |

### 12.5 PH-2 parallel packages — shadow pilot and field validation

| Package | Type | Scope/output | Dependencies and boundary |
| --- | --- | --- | --- |
| R2-01 | Research/field | Independently review hazard/runout, parcel/rights, geotechnics, drainage, lean-season water, access/services, and reference outcomes for selected cases | Uses authorized S45–S49 and preregistered protocol; reviewers remain independent of coding agents where feasible. |
| R2-02 | Research/social/legal | Household/community participation, pathway/preferences, assisted service, objections, scheme/entitlement/funding, livelihood and host-community evidence | Authorized process only; shadow results cannot recruit, approve, or allocate households. |
| R2-03 | Research/evaluation | Measure dossier time, reversals, false positives/negatives, unknowns, subgroup/fairness, comprehension, accessibility and source/model sensitivity | Freeze protocol before results; report met/unmet/inconclusive without tuning the reference after inspection. |
| C2-01 | Coding | Restricted agency import adapters and reconciliation for land/FRA/forest/programme records; source freshness and conflict review | Requires R0-03 agreements and C1-01/02/04; no portal scraping. |
| C2-02 | Coding | Production-shaped field packages, household/member controls, survey evidence, water/geotechnical measurements and conflict workflows | Requires C1-05/10/11 and approved devices/forms; restrict live household self-service until PH-3 authority. |
| C2-03 | Coding | Calibrate approved gates/criteria, sensitivity and explanations; harden solver/validator, concurrency and reservation rehearsal | Requires R2-01/02 evidence and approved policy; no hidden threshold/default or live reservation. |
| C2-04 | Coding | Complete objections/notices, scheme/funding, LSG templates, delivery/handoff rehearsal, accessible bilingual reports and private mock status | Requires C1-08/09 and R2-02; all outputs remain `SHADOW`. |
| C2-05 | Coding/assurance | Fix only observed defects; performance, security, privacy, accessibility, offline-loss, restore and source-failure exercises | No feature expansion during evaluation; regress all AT-01–38 and preregistered cases. |
| I2-01 | Integration | Freeze source/policy/software versions per case, run retrospective/prospective shadow workflows, reconcile independent findings, and produce the PH-2 exit dossier | Controlled live scope stays blocked until §5 exit gates and S45–S50 evidence pass. |

### 12.6 PH-3 parallel packages — controlled live operation

| Package | Type | Scope/output | Completion rule |
| --- | --- | --- | --- |
| R3-01 | Assurance/operations | Independent decision reconstruction, source freshness, fairness/privacy/accessibility reviews, household remedy observation and incident review | Continues throughout the minimum six-month controlled window. |
| R3-02 | Programme research | Measure adoption, staffing/support cost, delivery/funding/readiness reconciliation, occupation/follow-up and residual risks | Does not count approval, sanction or handover as completed relocation. |
| C3-01 | Coding/platform | Harden deployment, migrations, scaling, monitoring, backup/failover, key rotation, patching and manual continuity/reconciliation | SLO/recovery/security evidence must be measured, not inferred from configuration. |
| C3-02 | Coding/product | Resolve bounded live defects in workflows, source adapters, reports, localization, accessibility and assisted service | Changes retain effective versions/migrations and cannot widen the approved live scope. |
| C3-03 | Coding/data | Operate source refresh/supersession, restricted imports, data corrections, public projection withdrawal/regeneration and manifests | Stale/failed mandatory data holds affected actions; no silent recompute of official decisions. |
| I3-01 | Release/assurance | Weekly operational and monthly programme reviews; reconcile manual decisions; compile PH-3 outcome, incident, cost and residual-risk evidence | Kerala scaling proposal begins only after §6 exit gate approval. |

### 12.7 PH-4 parallel packages — Kerala scaling

| Package | Type | Scope/output | Completion rule |
| --- | --- | --- | --- |
| R4-01 | Research/onboarding | Per-district authority, workflow, sources/coverage, language, scheme, staffing, support and data-agreement pack | Wayanad assumptions are never copied without review. |
| R4-02 | Research/assurance | Cross-district policy, fairness, accessibility, security, source quality, cost and support-tier review | At least two materially different districts supply evidence. |
| C4-01 | Coding | Configuration-driven district onboarding, policy inheritance/override, source-adapter configuration and isolation | No shared unrestricted district records or Kerala-wide hard-coded gate. |
| C4-02 | Coding | Scale queues/storage/tiles/reports, quotas, state aggregate dashboards, support tooling and release governance | Meet NFR targets and disclosure controls under statewide reference load. |
| C4-03 | Coding/quality | District fixture packs, regression suites, migration/rollback and training environment | Each district passes its own PH-0–PH-3-equivalent gates. |
| I4-01 | Integration | Onboard districts independently, test state oversight plus district isolation, then evaluate shared components | §7 exit gate controls any statewide claim. |

### 12.8 PH-5 parallel packages — multi-state adaptation

| Package | Type | Scope/output | Completion rule |
| --- | --- | --- | --- |
| R5-01 | Research | State-specific law/workflow, authority, hazards, sources, records, programmes, languages, publication/remedy and hosting/residency assessment | Produces a new adaptation package; Kerala rules are only references. |
| R5-02 | Research/assurance | Independent tenant-isolation, legal/privacy, scientific, accessibility and operational review | State-specific evidence and approvers required. |
| C5-01 | Coding | State configuration/package loader, policy/template/language isolation, new source adapters and migration tools | Shared core remains versioned; state exceptions do not become national defaults. |
| C5-02 | Coding/quality | New state fixtures, regression/performance/security suites, deployment/support automation | No state enters live use without its own earlier-phase evidence. |
| I5-01 | Integration | Run the second state through PH-0–PH-3 gates and publish reuse-versus-localization findings | §8 exit gate defines completion. |

### 12.9 Suggested simultaneous-agent allocation

Use the smallest number of agents that keeps package ownership clear. A practical PH-1 maximum is seven coding agents in Wave A (`C1-01`, `02`, `03`, `05`, `06`, `11`, `12`), with research agents continuing R0 packages. After I1-01, reassign those agents to Wave B rather than adding more agents. Packages `C1-07`, `08`, `09`, and `10` must not all edit shared scenario/workflow schemas directly; PKG-0C/I1-01 owns those contract changes.

For every agent prompt, include:

- package ID and phase;
- allowed modules/files and explicitly excluded scope;
- required `FEAT-*`, `ARC-*`, `FR/NFR-*`, `RUL-*`, `AT-*`, `S*`, `DEC-*`, and `ODN-*` links;
- authoritative input artifacts and whether each is real, public, restricted, or synthetic;
- expected API/schema/event contracts and mocks;
- exact tests and exit evidence;
- dependency versions/commit and handoff recipient;
- stop conditions, unresolved questions, and actions requiring human authority.

## 13. References

1. [PUNARVAS-AI PRD](./prd.md)
2. [PUNARVAS-AI TRD](./trd.md)
3. [PUNARVAS-AI Rules](./rules.md)
4. [PUNARVAS-AI Feature Catalog](./feature.md)
5. [Kerala Local Self Government DM Plans](https://sdma.kerala.gov.in/local-self-government-dm-plans/)
6. [KSDMA Wayanad reports](https://sdma.kerala.gov.in/reports-landslides-2024/)
7. [Open Decisions](./open-decisions.md)
8. [Source Register](./source-register.md)
