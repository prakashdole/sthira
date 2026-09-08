---
title: PUNARVAS-AI Product Requirements Document
document_id: PUN-PRD
version: 1.2
status: Baseline for implementation
as_of: 2026-09-08
audience: Government programme owners, product, GIS, legal, field, design, engineering, security, and assurance teams
owner: Product owner with State/District programme authority
normative_scope: Product purpose, users, outcomes, scope, workflows, and product acceptance
---

# PUNARVAS-AI Product Requirements Document

## 1. Document boundary

This document defines **why the product exists, who it serves, what outcomes it must enable, and what is in or out of scope**. Measurable technical requirements are in [trd.md](./trd.md), system composition is in [architecture.md](./architecture.md), binding domain constraints are in [rules.md](./rules.md), feature-level acceptance is in [feature.md](./feature.md), and mathematical methods are classified in [equations.md](./equations.md). Parameters, evidence gaps, and unresolved policy choices are controlled in [parameters.md](./parameters.md), [source-register.md](./source-register.md), and [open-decisions.md](./open-decisions.md).

PUNARVAS-AI is advisory decision-support software. It is not a legal title system, hazard-certification authority, allocation authority, or substitute for field, legal, hydrological, social, or geotechnical review.

## 2. Product summary

PUNARVAS-AI helps Indian public authorities plan **proactive and permanent relocation from disaster-prone areas**. It brings hazard evidence, settlement exposure, land records, observed occupation, legal interests, field verification, water and infrastructure feasibility, household needs, livelihood fit, preferences, approvals, objections, and audit history into one governed workflow.

The product addresses the gap between an available hazard map and an implementable, fair, reviewable relocation programme. It does not attempt to predict every disaster or run real-time response operations.

The reference pilot is Wayanad, Kerala. The product must remain configurable for other hazards, districts, states, legal workflows, languages, and programme types.

## 3. Evidence and requirement precedence

Product interpretation follows this order:

1. Current user-approved instructions and decisions.
2. Later statements in `claude.txt` and `idea.txt` over earlier statements when they concern product intent.
3. Current primary law, official government publications, and authoritative scientific sources over factual assertions made in the transcripts.
4. Transcript code and formulas as non-normative design references only.

Contradictions are resolved in [decisions.md](./decisions.md). Time-varying facts are stored with source, valid time, observation time, and supersession rather than hard-coded into the product.

## 4. Problem statement

Public relocation programmes fail or stall when evidence and authority are fragmented:

- Hazard layers identify potentially unsafe settlements but do not determine lawful relocation eligibility.
- Digitized cadastral records can be ungeoreferenced, outdated, disputed, or inconsistent with occupation on the ground.
- A parcel that appears empty can contain undocumented households, community use, forest rights, tenancy, or litigation.
- A physically safe site can fail because of lean-season water, infrastructure cost, inaccessible services, loss of livelihood, or lack of community acceptance.
- Building footprints and Census aggregates can estimate exposure but cannot identify beneficiaries.
- Beneficiary lists, site rankings, and policy weights can change without an intelligible history or a usable objection process.
- Government data and approvals are often exchanged through files and paper, while APIs and reliable connectivity may be unavailable.
- Analytical maps can cause harm if presented as official notifications before verification and authorization.

## 5. Product vision

Enable an authorized public team to move from **versioned evidence to a field-verifiable, legally reviewable, preference-aware relocation proposal**, while ensuring that no analytical result is mistaken for an official decision and every material change can be explained.

## 6. Users and responsibilities

| User | Primary need | Product responsibility |
| --- | --- | --- |
| State programme administrator / SDMA | Configure programmes, policy sets, jurisdiction, escalation, and oversight | Provide governed configuration and cross-district visibility without bypassing legal authority |
| District Collector / authorized approver | Review evidence and issue or reject administrative decisions | Present complete dossiers, unresolved conditions, recorded advice, and approval controls |
| DDMA / district planner | Coordinate relocation cases and local plans | Provide case workflow, task ownership, deadlines, dependencies, and LSG-plan exports |
| GIS/hazard analyst | Manage authoritative layers and exposure analysis | Preserve provenance, uncertainty, CRS, versions, model limits, and review status |
| Revenue/land officer | Reconcile parcel records, ownership, occupation, and encumbrances | Represent evidence and conflicts without declaring conclusive title automatically |
| Legal/forest-rights reviewer | Determine applicable acquisition, FRA, forest, consultation, or litigation workflow | Provide issue tracking and signed legal-review status, not automated legal conclusions |
| Geotechnical/water/infrastructure specialist | Assess site safety and service feasibility | Provide structured field assessments, evidence, qualifications, and expiry/review dates |
| Social/livelihood officer | Verify households, vulnerability, livelihood, accessibility, and preferences | Support assisted intake, consent, deduplication, and sensitive-data controls |
| Field verifier | Capture offline observations and evidence | Provide offline-first forms, location/accuracy metadata, safe sync, and supervisor review |
| Grievance/appeal officer | Receive, investigate, and decide objections | Preserve filing, evidence, hearings, decisions, notices, deadlines, and escalation path |
| Auditor/oversight body | Reconstruct who knew, changed, approved, or published what | Provide read-only, scoped access to provenance, decisions, and append-only audit history |
| Affected household/authorized representative | Understand status, provide preferences, correct records, and object | Provide accessible assisted or self-service participation without exposing other households |
| Public visitor | Understand programme criteria and official progress | Show approved, de-identified or lawfully published information only |

No role gains authority merely because the software exposes an action. Authorization derives from the applicable programme, law, and government order.

## 7. Product outcomes

| ID | Outcome | Pilot indicator |
| --- | --- | --- |
| OUT-01 | Traceable evidence | 100% of decision-used datasets and evidence items have source, version, checksum, dates, reviewer, and permitted use |
| OUT-02 | Safer site screening | Every recommended site has completed required hazard/runout, legal, occupation, water, infrastructure, livelihood, and field gates or is visibly on hold |
| OUT-03 | Honest uncertainty | 100% of derived exposure and suitability results disclose method, confidence/uncertainty, source version, and limitations |
| OUT-04 | Household agency | Every allocation proposal is based on recorded eligibility, consent state, preferences, accessibility needs, and a reversible review process |
| OUT-05 | Explainable prioritization | Every priority or comparative score can be decomposed into rules, inputs, weights, missing evidence, policy version, and sensitivity result |
| OUT-06 | Procedural fairness | Every affected record can be corrected or objected to through a configured, acknowledged, time-tracked workflow |
| OUT-07 | Controlled authority | Zero analytical items are presented or exported as official without the required approval state and signatory |
| OUT-08 | Faster preparation | Shadow pilot demonstrates at least 30% reduction in median time to assemble a complete review dossier compared with the measured existing process |
| OUT-09 | Privacy and accessibility | Zero unauthorized public personal-data disclosures; all critical workflows pass GIGW 3.0 and WCAG 2.2 AA acceptance checks |
| OUT-10 | Interoperable outputs | Approved dossiers and LSG-plan annexes are available in human-readable and machine-readable forms with manifest and checksum |
| OUT-11 | Actionable funding and delivery | Every proposed pathway shows scheme eligibility, confirmed funding, uncovered cost, readiness milestones, and the owner of post-approval delivery |
| OUT-12 | Defensible necessity and completion | Every case records considered in-situ mitigation alternatives and distinguishes proposal, approval, handover, occupation, and verified follow-up |

Baselines for time, error rate, reversal rate, and preference satisfaction must be measured during Phase 0/2; they must not be invented retrospectively.

## 8. Scope

### 8.1 Programme and evidence management

- Create jurisdiction-scoped relocation programmes, cases, policy versions, authorities, review stages, and SLA/escalation rules.
- Ingest data by governed API, file upload, or offline transfer when no dependable integration exists.
- Version source and derived datasets without overwriting the record used for an earlier decision.
- Record licenses, use restrictions, retention, access classification, and source quality.
- Maintain the complete S01–S54 acquisition inventory, distinguish data products from catalogs/processing/display services, and show whether access is merely documented, entitled, sampled, validated, approved, stale, or blocked.
- Map every required gate, criterion, and parameter to an acquisition method; preserve missing mandatory inputs as `UNKNOWN`/`HOLD` rather than substituting convenient open-data proxies.

### 8.2 Hazard and exposure intelligence

- Consume authoritative landslide, flood, runout, and other approved hazard layers through pluggable adapters.
- Overlay settlements, buildings, infrastructure, and administrative boundaries to estimate exposure.
- Show that flat local terrain may remain exposed to upstream channelized debris flow.
- Separate analytical interpretation, field verification, technical endorsement, and official notification.
- Treat building-derived population as an estimate with bounds, never as a household or beneficiary register.
- Record a competent, evidence-backed assessment of whether permanent relocation is necessary, whether risk reduction in place is feasible, and why each considered alternative was accepted, rejected, deferred, or remains unknown. Exposure alone does not establish relocation necessity.
- Preserve settlement/community units, social networks, shared assets, and proposed delivery phases alongside household case records.

### 8.3 Land-truth and legal-readiness assessment

- Preserve cadastral geometry and record versions, ULPIN where available, Record of Rights references, acquisition/transfer route, claims, encumbrances, tenancy, forest classification, FRA status, community use, litigation, and legal opinion.
- Compare records with multi-date imagery, building footprints, field observations, and community evidence to raise discrepancies.
- Route discrepancies to a human reviewer; remote sensing alone cannot prove vacancy, occupation, ownership, or legal availability.
- Represent unresolved issues as `UNKNOWN` or `BLOCKED`, not as silently clear or permanently impossible.

### 8.4 Candidate-site assessment

- Assess hazard avoidance, debris-flow/runout, terrain, geotechnical evidence, drainage, lean-season water yield and quality, environmental constraints, infrastructure, services, accessibility, livelihood continuity, social/cultural factors, capacity, cost, and implementation dependencies.
- Use the Jal Jeevan Mission 55 LPCD service level as a baseline demand assumption when applicable, while requiring separate evidence of sustainable yield, reliability, quality, and delivery feasibility.
- Maintain source-specific criteria and units rather than collapsing all evidence into one opaque score.

### 8.5 Household eligibility, priority, and preference

- Import or enumerate households through authorized registers and field verification.
- Deduplicate people and households without using building counts as identities.
- Apply a versioned eligibility and priority policy with reason codes and human review.
- Capture accessibility needs, livelihood dependencies, household composition, and separately versioned records for processing lawful basis/notice, programme participation, relocation pathway choice, site preference, offer acceptance/refusal, and any required community process. Silence or failed contact is neither consent nor refusal.
- Restrict sensitive member-level data and expose only the minimum needed to each role.

### 8.6 Allocation scenarios and pathways

- Support planned-township/site allocation and approved vulnerability-linked/self-relocation assistance as distinct pathways.
- Generate capacity-constrained allocation scenarios from consenting eligible households, approved sites, household indivisibility, preferences, accessibility, livelihood fit, and programme constraints.
- Show unassigned households and supported contributing constraints or counterfactuals; never silently drop, split, or force an assignment and never claim one uniquely binding reason where constraints interact.
- Require authorized review, consultation where applicable, objection opportunity, and approval before a proposal becomes a decision.

### 8.7 Scheme, entitlement, and funding

- Configure each applicable scheme by hazard, jurisdiction, programme, household/tenure category, qualifying assessment, eligible cost head, cap or calculation method, effective dates, restrictions, milestones, appeal route, and competent sanctioning authority.
- Track funding as identified, applied, sanctioned, committed, released, received, spent, reconciled, or withdrawn. Preserve sanction/receipt evidence and prevent duplicate counting.
- Calculate and display the uncovered funding gap without treating lack of eligibility under one scheme as evidence that relocation is unnecessary or that a household is ineligible under every pathway.
- Keep owner, tenant, landless, livelihood-dependent, host-community, and other applicable interests explicit. No category receives an invented universal entitlement.

### 8.8 Governance, grievances, and transparency

- Support draft lists, verification, publication approval, objection windows, hearings, amendments, appeals, finalization, and supersession.
- Configure deadlines and authorities from the applicable government order or programme; do not assume one universal appeal period.
- Monitor case SLAs and recommend escalation to an authorized higher role. The software cannot bypass an authority or invent a statutory meeting cadence.
- Publish de-identified methods, aggregates, progress, official notices, and lawfully approved lists; protect household-level evidence by default.

### 8.9 Delivery, completion, and handoff

- Track programme commitments, site/unit construction or procurement, functioning water/sanitation/access/energy and other required services, defects, household offer and acceptance, lawful possession/handover, actual occupation, transition support, and livelihood follow-up.
- A site allocation or approval is not a completed relocation. Completion criteria are pathway- and programme-specific, effective-dated, evidenced, and independently reviewable.
- Where another government system owns construction, payments, possession, or follow-up, record the accountable owner, external reference, last verified status, reconciliation cadence, and explicit handoff boundary.

### 8.10 Outputs

- Generate site-assessment dossiers, household/beneficiary review packs, decision summaries, field checklists, objection registers, approval histories, and progress reports.
- Populate relocation-relevant annexes for Kerala's approved Local Self Government Disaster Management Plan template; do not claim to replace its participatory preparation and approval process.
- Export accessible PDF/HTML, CSV, GeoJSON/GeoPackage where appropriate, evidence packages, and a JSON manifest with source versions and SHA-256 checksums.

## 9. End-to-end reference workflow

1. **Programme setup:** an authorized administrator chooses jurisdiction, programme authority, applicable policy, pathways, stages, languages, review roles, and publication rules.
2. **Evidence intake:** analysts import authoritative layers and registers; the system validates structure, CRS, license, checksum, dates, and geographic scope.
3. **Exposure assessment:** spatial processing estimates affected settlements and buildings, recording method and uncertainty.
4. **Household verification:** authorized teams reconcile estimates with local registers, field enumeration, identity evidence, and deduplication.
5. **Candidate discovery:** officers nominate parcels or import an authorized inventory; the system does not autonomously appropriate land.
6. **Desktop screening:** hazard, runout, record, occupation, environmental, water, access, and service evidence produce gates and missing-evidence tasks.
7. **Field/legal review:** qualified reviewers capture evidence, resolve or preserve conflicts, and sign their domain conclusions.
8. **Necessity/alternatives review:** competent reviewers compare permanent relocation with feasible in-situ risk reduction and record reasons, uncertainty, affected settlement/community, and authority state.
9. **Comparison:** eligible candidates are compared by separate criteria using an approved policy version and sensitivity analysis.
10. **Funding assessment:** programme staff evaluate scheme eligibility, entitlements, confirmed funding, cost coverage, milestones, and shortfalls without changing the underlying relocation-need assessment.
11. **Participation:** households receive understandable options and separately provide participation, pathway, preference, and offer-acceptance decisions through assisted or self-service channels.
12. **Scenario generation:** the system creates reviewable allocations or self-relocation pathways and explains unassigned cases.
13. **Objection and approval:** authorized bodies publish the appropriate draft, accept objections, record hearings/decisions, approve or return work, and preserve every version. Approval and notification are distinct when the applicable procedure says so.
14. **Delivery and completion:** approved artifacts are signed/exported; reservations, funding, unit/service readiness, handover, occupation, defects, and follow-up are tracked or explicitly reconciled from the responsible external system.

## 10. Wayanad pilot

The pilot must use Wayanad as a **reference and validation case**, not as a source of permanent constants.

- Use KSDMA's downloadable GSI 2022 landslide susceptibility data and relevant official layers, with source/version labels.
- Include channelized debris-flow/runout assessment because the July 2024 failures affected Mundakkai and Chooralmala through stream channels.
- Reconcile Kerala PDNA facts, government orders, and later implementation reports as separate dated records.
- Support Elston/Nedumbala or subsequent site facts only through versioned case data.
- Represent township and self-relocation/assistance choices separately.
- Support Kerala's English/Malayalam context, LSG planning workflow, and approved publication/appeal process.
- Begin in shadow mode with de-identified or authorized data; no pilot output becomes an official decision solely because the system produced it.
- Build the first sandbox from SOI/LGD geography, KSDMA/GSI hazards, one reviewed DEM, Sentinel-2 through CDSE or an approved alternate, Open Buildings plus selected OSM/Microsoft checks, Census/optional WorldPop context, WorldCover, OSM/PMGSY access, NAQUIM/NWDP water context, and reviewed IMD/IMERG/CHIRPS history. Treat mirrors and shared-input products as related, not independent evidence.
- Keep real site approval/allocation disabled until authoritative parcel/right records, verified household participation, qualified site/geotechnical evidence, lean-season water/service proof, and programme/funding/readiness records pass review.

## 11. Explicit exclusions

PUNARVAS-AI will not include:

- live emergency command, seismic/rainfall command HUDs, evacuation routing, convoy routing, airdrop, ration, shelter, or helicopter logistics;
- a civilian self-evacuation navigator;
- Sarvam or other voice assistants, general-purpose chatbots, or LLMs in authoritative decision paths;
- a Cesium/3D globe requirement;
- raw Sentinel-1 InSAR processing or locally trained hazard/susceptibility models in the baseline;
- regional-news or social-media scraping;
- the invented seismic impact-radius formula or the transcript's `Ω` score;
- automated legal title clearance, FRA clearance, beneficiary identity, land acquisition, forced relocation, gazetting, fund release, or authority bypass;
- automatic rejection based only on NDBI, NDWI, a footprint, ULPIN, land category, or a single map;
- blockchain or claims of mathematical/legal immutability;
- a replacement for field inspection, Gram/Ward Sabha participation, legal review, statutory publication, or authorized approval.

## 12. Product-level acceptance

The baseline product is acceptable for a controlled live pilot only when:

- every feature in [feature.md](./feature.md) traces to an outcome above, one or more `FR-*` requirements, a `RUL-*` rule, a phase, and a decision;
- the Wayanad validation scenarios in [trd.md](./trd.md) pass with authorized reviewers;
- required data agreements, privacy assessment, legal workflow mapping, security testing, accessibility audit, backup/restore exercise, and user training are complete;
- domain reviewers confirm that analytical, verified, endorsed, and official states cannot be confused;
- public views cannot expose restricted household, parcel-claimant, or preliminary site data;
- the programme authority signs the pilot operating procedure and residual-risk register.
- the pilot evaluation follows a preregistered sampling/reference-evidence protocol and no target is described as achieved until measured.
- the release source-readiness report shows a validated, permitted AOI sample for every mandatory dependency, with no catalog-only, out-of-coverage, stale, unlicensed, or synthetic input satisfying a live gate.

The user approved Wayanad and the production-oriented documentation baseline on 8 September 2026. This does not approve operational policy values, statutory workflows, data access, funding, or live deployment; those remain gated in [open-decisions.md](./open-decisions.md).

## 13. Primary research basis

1. [Disaster Management Act, 2005](https://www.mha.gov.in/sites/default/files/2022-09/The%20Disaster%20Management%20Act,%202005%5B1%5D.pdf), read with the [Disaster Management (Amendment) Act, 2025](https://prsindia.org/files/bills_acts/acts_parliament/2025/The_Disaster_Management_Act%2C_2025.pdf) and its [commencement](https://www.pib.gov.in/PressReleasePage.aspx?PRID=2146781)
2. [KSDMA hazard maps](https://sdma.kerala.gov.in/hazard-maps/)
3. [KSDMA Meppadi/Wayanad 2024 reports and PDNA](https://sdma.kerala.gov.in/reports-landslides-2024/)
4. [KSDMA Wayanad government orders](https://sdma.kerala.gov.in/government-orders-5/)
5. [Kerala Vulnerability Linked Relocation Scheme](https://sdma.kerala.gov.in/vulnerability-linked-relocation-scheme/)
6. [Kerala Local Self Government DM Plans and approved template](https://sdma.kerala.gov.in/local-self-government-dm-plans/)
7. [Department of Land Resources acts and RFCTLARR rules](https://dolr.gov.in/en/document-category/acts-rules/)
8. [DILRMP 3.0 Operational Guidelines 2026–2031](https://dolr.gov.in/en/document/digital-india-land-records-modernization-programmedilrmp-3-0-operational-guidelines-2026-2031/)
9. [Forest Rights Act and Rules](https://tribal.nic.in/FRA/data/FRARulesBook.pdf)
10. [UNDRR disaster-risk terminology](https://www.undrr.org/terminology/disaster-risk)
11. [Jal Jeevan Mission service level](https://jaljeevanmission.gov.in/about_jjm)
12. [Digital Personal Data Protection Rules, 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa)
13. [PUNARVAS-AI Evidence and Source Register](./source-register.md)
