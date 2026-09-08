---
title: PUNARVAS-AI Technical Requirements Document
document_id: PUN-TRD
version: 1.2
status: Baseline for implementation and verification
as_of: 2026-09-08
audience: Engineering, architecture, QA, security, GIS, data, product, programme, legal, field, and operations teams
owner: Technical product owner and system assurance lead
normative_scope: Functional requirements, interfaces, data contracts, non-functional requirements, and acceptance
---

# PUNARVAS-AI Technical Requirements Document

## 1. Document boundary

This document specifies **what the system must do and how compliance is measured**. It intentionally does not prescribe runtime components or deployment topology; those belong in [architecture.md](./architecture.md). Product intent is in [prd.md](./prd.md), domain constraints in [rules.md](./rules.md), feature packaging in [feature.md](./feature.md), formula definitions in [equations.md](./equations.md), and parameter governance in [parameters.md](./parameters.md).

All requirements are subject to the rules cited from [rules.md](./rules.md). “Approved” means approved by the role and process configured for the applicable programme; it does not mean that software created legal authority.

## 2. Technical scope and environments

The system must support four isolated environment classes:

- **Development:** synthetic data only unless a specifically approved restricted test dataset is provided.
- **Test/assurance:** synthetic, de-identified, or explicitly approved datasets; automated and manual assurance.
- **Pilot/shadow:** authorized Wayanad data; no output becomes an official decision solely from shadow use.
- **Production:** approved jurisdictions, data-sharing agreements, operating procedure, named authorities, monitoring, backup, and incident response.

No production personal or restricted geospatial data may be copied to a lower environment without approved de-identification and transfer controls.

## 3. Functional requirements

### 3.1 Programme, jurisdiction, and access

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-001 | Authorized administrators shall create a relocation programme with jurisdiction, purpose, legal/programme basis, pathways, languages, authorities, policy versions, review stages, SLA rules, publication rules, and effective dates. | API/UI test; audit record inspection |
| FR-002 | Every operational record shall belong to a programme and geography scope. Cross-scope access shall be denied unless a separately authorized oversight role permits it. | Authorization and row-isolation tests |
| FR-003 | The system shall support the roles defined in PRD §6 and custom role-to-permission mappings without granting statutory authority through configuration. | Permission-matrix test; legal-owner review |
| FR-004 | A policy or workflow change shall create a new effective-dated version; existing cases shall retain their version until an authorized migration is recorded. | Version/migration test |
| FR-005 | Users shall see their current role, programme, geography, data classification, and whether the workspace is analytical, shadow, or production. | UI/accessibility test |

### 3.2 Dataset and evidence lifecycle

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-006 | The system shall ingest approved vector, raster, tabular, document, image, and API-sourced evidence, including GeoPackage, GeoJSON, zipped Shapefile, COG/GeoTIFF, CSV, JSON, PDF, and common image formats. | Format fixture suite |
| FR-007 | Before use, ingestion shall validate file integrity, declared schema, CRS/spatial bounds, geometry validity, required metadata, license/use basis, dates, checksum, and malware status. | Invalid/valid fixture tests |
| FR-008 | Every dataset version shall store the metadata contract in §6.2 and an immutable-while-retained source-object reference. Derived data shall link to every input version and processing run; lawful deletion/restriction shall leave only the permitted tombstone/lineage. | Lineage and retention test |
| FR-009 | Records with invalid geometry, missing required metadata, disallowed use, or implausible geographic bounds shall be quarantined and excluded from analysis until reviewed. | Quarantine test |
| FR-010 | The system shall compare incoming and prior versions, report changed coverage/schema/features, and require a supersession decision. | Differential-ingestion test |
| FR-011 | Evidence items shall support source, subject, observation time, location/accuracy, author/collector, reviewer, classification, attachment hash, and chain of custody. | Evidence contract test |

### 3.3 Hazard and exposure

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-012 | Hazard adapters shall declare hazard type, source authority, geographic/time coverage, resolution, method, limits, and state before a layer can be used. | Adapter contract test |
| FR-013 | The system shall prevent a hazard product from feeding gates, eligibility, allocation, official output, or operational conclusions outside its declared coverage. An authorized out-of-coverage display is permitted only in an isolated research view marked `UNSUPPORTED`, with no decision linkage. | Coverage-negative and isolation tests |
| FR-014 | Exposure jobs shall intersect approved hazard/runout geometry with settlements, buildings, infrastructure, and population baselines while recording source versions, processing parameters, CRS, and uncertainty method. | Reproducible spatial-job test |
| FR-015 | Exposure output shall distinguish observed counts from estimated counts and shall display a range/confidence or explicit “not quantified” state. | UI/export assertion |
| FR-016 | Building-derived estimates shall not create household/person records or beneficiary eligibility. | Data-flow and authorization test |
| FR-017 | Hazard outputs shall implement the authority-state machine in §5.1 and expose the same state in maps, tables, APIs, and exports. | Cross-channel state test |
| FR-018 | A site screen for applicable landslide hazards shall support upstream channel/runout evidence independently of local slope. | Wayanad runout scenario |

### 3.4 Parcel and land-truth reconciliation

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-019 | The system shall preserve source parcel identifiers, ULPIN where supplied, Record of Rights reference, source geometry, record dates, ownership/interest evidence, and source jurisdiction. | Import and record inspection |
| FR-020 | Corrected/georeferenced parcel geometry shall be a new parcel version with method, control points, source/target CRS, residual/error metrics, and reviewer; the source geometry shall remain retrievable. | Cadastral-offset scenario |
| FR-021 | Users shall record possession, occupation observations, buildings, community/common use, tenancy, charges, litigation, forest classification, FRA claims/rights, and contradictory evidence without overwriting the underlying record. | Conflict-preservation test |
| FR-022 | Remote-sensing or footprint analysis may create a discrepancy with method/confidence but shall not set legal clearance or vacancy. | Paper-vacant scenario |
| FR-023 | Legal review shall select the applicable transfer/acquisition/consultation pathway, identify blocking and non-blocking conditions, supporting authority, reviewer, review date, and expiry/review trigger. | Legal workflow test |
| FR-024 | Parcel legal readiness shall remain `UNKNOWN` or `BLOCKED` while mandatory claims/occupation/clearance evidence is unresolved. | Pending-FRA scenario |

### 3.5 Candidate-site assessment

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-025 | Users shall nominate candidate sites without implying approval, attach one or more parcel versions, proposed use/capacity, and nomination source. | State and wording test |
| FR-026 | Site assessment shall support separate criterion groups for hazard, geotechnical/developability, legal, water/environment, infrastructure/services, accessibility, livelihood/social, capacity, cost, and readiness. | Assessment-schema test |
| FR-027 | Every criterion result shall include raw value, unit, method, source/evidence versions, observation date, confidence/uncertainty, reviewer qualification/role, policy threshold, gate outcome, and explanation. | Required-field validation |
| FR-028 | Water assessment shall calculate baseline demand and separately record lean-season yield, duration/sample basis, quality, reliability, competing demand, treatment, storage, and distribution assumptions. | Inadequate-water scenario |
| FR-029 | Capacity shall expose separate dwelling, household, person, water, sanitation, developable-area, access/service, shared-resource, budget, readiness-date, and phase constraints. It shall report all active/violated constraints and shall not invent one uniquely binding cause where constraints interact. | Capacity and counterfactual fixtures |
| FR-030 | A required `UNKNOWN`, `BLOCKED`, or `FAIL` gate shall prevent a site from being represented as approved or allocatable. Non-blocking conditions and conditions due before reservation, handover, or occupation shall be separately typed and enforced at their action stage. | Gate-stage enforcement test |
| FR-031 | Qualified reviewers shall sign/endorse only their configured domain, and the system shall detect expired evidence or a changed source version that requires reassessment. | Expiry/invalidation test |

### 3.6 Household, eligibility, priority, and preference

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-032 | Authorized users shall import or enumerate households and restricted members with source, verification state, duplicate-candidate handling, and minimal identifiers. | Import/deduplication test |
| FR-033 | Household identity/eligibility shall not be derived from building footprints or estimated population. | Negative lineage test |
| FR-034 | Eligibility and priority evaluations shall use an effective-dated policy and produce rule-level pass/fail/unknown results, reason codes, input evidence, reviewer status, and prior-version comparison. | Policy evaluation test |
| FR-035 | Household-facing explanations shall be available in English and Malayalam for the Wayanad pilot and in an accessible printable form. | Translation/accessibility review |
| FR-036 | The system shall separately capture processing notice/lawful basis, programme participation, pathway choice, ranked/unacceptable site preferences, offer acceptance/refusal, any required community process, accessibility/accommodation needs, livelihood dependencies, representative authority, and timestamped revisions. | Purpose-separation and assisted-intake tests |
| FR-037 | Silence, missing contact, or an incomplete form shall be distinguishable from consent or refusal. | State test |
| FR-038 | Restricted member attributes shall be separately access-controlled and omitted from ordinary list, map, export, and public views. | Field-level authorization test |

### 3.7 Comparative assessment and allocation scenarios

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-039 | Comparative MCDA shall operate only on candidates meeting the configured gate prerequisites and shall preserve each criterion contribution. | Gated-comparison test |
| FR-040 | A policy set shall record criteria, direction, normalization, thresholds, weights, missing-data behavior, approval, effective dates, consistency checks, and sensitivity configuration. | Policy contract test |
| FR-041 | The system shall show rank/order changes across approved sensitivity ranges and warn when an alternative's position is unstable. | Sensitivity fixture test |
| FR-042 | Allocation scenarios shall implement E19–E24: binary household-to-option/phase variables, explicit unassigned outcome, household indivisibility, admissibility, dwelling/person/accommodation/resource capacities, budgets, site readiness, and approved community/caregiving constraints. | Solver property and dimensional tests |
| FR-043 | The approved objective ordering, coverage unit, fairness constraints, tie-break, model/solver version, options, coefficient scaling, tolerances and seed shall be versioned. Same pinned inputs shall reproduce the objective values and an equivalent approved solution; unexplained row-order tie-breaking is prohibited. | Re-run and tie fixtures |
| FR-044 | Each proposed assignment shall explain preferences, livelihood/accessibility fit, constraints, objective level and alternatives. Unassigned explanations shall distinguish no feasible option, capacity competition, preference/admissibility restriction, policy priority, and solver termination; counterfactuals shall be provided where useful without claiming unique causation. | Explanation/counterfactual test |
| FR-045 | Users may create multiple draft scenarios and compare trade-offs; drafts shall reserve no capacity. Approval shall atomically revalidate input versions and reserve all affected site/shared resources across programmes, preventing double allocation and overlapping-site double counting. | Concurrent-approval race test |
| FR-046 | Manual overrides shall require reason, authority, supporting evidence, affected household notice status, and audit event, and shall pass the same independent feasibility validator as solver output. | Override and feasibility test |

### 3.8 Objections, approvals, publication, and escalation

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-047 | Configurable workflows shall support draft, internal review, authorized publication, objections/corrections, hearings, decisions, appeal/review, finalization, supersession, and withdrawal. | Workflow matrix test |
| FR-048 | Each objection shall receive a receipt ID and store filer/representative, subject/version challenged, grounds, evidence, filing channel/time, assigned authority, SLA, hearings, decision/reasons, remedy, and notice. | Objection lifecycle test |
| FR-049 | Workflow deadlines and authorities shall be effective-dated and cite the governing programme/order. The system shall not supply a universal beneficiary-appeal period. | Configuration validation |
| FR-050 | A pending objection or expired mandatory review shall visibly block or qualify dependent publication/approval according to policy. | Dependency test |
| FR-051 | Case SLA monitoring shall send reminders and recommend escalation to configured authorized roles; it shall not change a substantive decision or bypass a role. | Escalation scenario |
| FR-052 | Public release shall require an approved publication profile that specifies fields, aggregation/de-identification, geography precision, state, language, authority, release date, and withdrawal/supersession behavior. | Privacy/publication test |

### 3.9 Reports, LSG integration, and audit

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-053 | The system shall generate review dossiers and field checklists from structured data and fixed templates; every factual claim shall link to evidence or be marked as an assertion requiring review. | Report traceability test |
| FR-054 | Kerala LSG DM Plan output shall map approved fields into the government template/annex and preserve fields requiring participatory input or external approval as incomplete rather than inventing content. | Template mapping review |
| FR-055 | Human-readable outputs shall have accessible HTML and, wherever PDF is a required delivery format, a tagged accessible PDF that passes the approved checks. Machine-readable output shall include CSV and appropriate GeoJSON/GeoPackage/JSON. A toolchain limitation cannot waive a required access need. | Export fixture and manual/automated accessibility tests |
| FR-056 | Each export shall include state, confidentiality, programme/jurisdiction, generating user/time, policy and source versions, unresolved conditions, manifest, SHA-256 checksums, and signature/approval status. | Manifest verification |
| FR-057 | Audit events shall meet `RUL-056`, be queryable by authorized auditors, and support verification of hash-link continuity without claiming blockchain immutability. | Tamper-evidence test |
| FR-058 | An approved historical export shall be reproducible bit-for-bit where deterministic rendering permits, or content-equivalent with a documented renderer change. | Reproduction test |

### 3.10 Offline field work

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-059 | Authorized users shall download the minimum assigned field package for offline use, with expiry, device binding, classification warning, and encrypted local storage. | Offline security test |
| FR-060 | Offline observations shall capture device time, server receipt time, GPS accuracy, collector, form/schema version, evidence hashes, and sync state. | Offline capture test |
| FR-061 | Synchronization shall be idempotent. Conflicting edits shall create a review task and preserve both versions; last-write-wins is prohibited for substantive evidence. | Offline conflict scenario |
| FR-062 | Remote revoke shall prevent future use/sync where technically possible; the operating procedure shall address lost devices and offline data expiry. | Device incident exercise |

### 3.11 Relocation necessity, schemes, funding, and completion

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-063 | Each programme case shall record a competent assessment of permanent-relocation necessity, considered in-situ risk-reduction alternatives, evidence, uncertainty, settlement/community effects, reviewer qualifications, reasons, and authority state. | Alternatives-review fixture |
| FR-064 | Scheme configuration shall record hazard/programme/jurisdiction applicability, household/tenure categories, qualifying assessments, eligible cost heads, caps or calculation method, restrictions, milestones, sanction authority, appeals, effective dates, and supersession. | Scheme-version contract test |
| FR-065 | A household scheme assessment shall preserve relocation-need state independently from scheme eligibility and shall produce rule-level eligible/ineligible/unknown results with reasons and alternatives. | Owner/tenant/landless fixtures |
| FR-066 | Funding records shall distinguish identified, applied, sanctioned, committed, released, received, spent, reconciled, and withdrawn amounts; prevent duplicate funding; and link sanctions, receipts, cost heads and milestone evidence. | Funding-state and duplicate test |
| FR-067 | The system shall compute a versioned funding gap from eligible required cost and confirmed non-duplicate funding using E17, without treating announced budgets or applications as received funds. | Funding-gap fixture |
| FR-068 | Delivery tracking shall separately record site/unit readiness, required functioning services, defects, household offer and acceptance, possession/handover, occupation, transition support and livelihood follow-up. | Incomplete-unit scenario |
| FR-069 | If another system owns delivery or payment, PUNARVAS-AI shall record the accountable owner, external reference, last verified state/time, reconciliation method and unresolved discrepancy; handoff shall not be reported as completion. | External-handoff test |
| FR-070 | Allocation approval, notification, funding sanction, handover and verified completion shall remain distinct states and dates unless a configured legal workflow explicitly combines specified acts. | Independent-state test |

### 3.12 Formula, parameter, and compliance control

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-071 | The system shall maintain immutable effective-dated formula and parameter versions implementing the contracts in [equations.md](./equations.md) and [parameters.md](./parameters.md), with owner approval and affected requirement/test links. | Registry contract test |
| FR-072 | Only `CORE` formulas explicitly activated by an approved policy may execute in decision workflows. `OPTIONAL` results shall be separately labelled; `SPECIALIST/EXTERNAL` and `REJECTED` formulas shall be non-executable by default. | Activation-negative tests |
| FR-073 | Formula execution shall validate units, CRS/time support, domains, missing/zero/negative inputs, geometry validity, source coverage, parameter validity, uncertainty, tolerance and rounding before emitting a result. | Boundary/property suite |
| FR-074 | Every numerical result shall retain formula/parameter/input/software versions, warnings, uncertainty, execution time and reviewer state, and remain reproducible or explainable after supersession. | Lineage/replay test |
| FR-075 | The system shall maintain an effective-dated legal/compliance applicability register covering current amended disaster law, programme orders, data protection commencement, CERT-In applicability, geospatial rules, records retention and publication controls. | Legal-register review |

### 3.13 Source acquisition and integration control

| ID | Requirement | Verification |
| --- | --- | --- |
| FR-076 | The system shall register each `S01`–`S54` source capability from [source-register.md](./source-register.md#5-operational-data-source-and-acquisition-register) with type (`PRODUCT`, `CATALOG`, `DOWNLOAD`, `PROCESSING`, `DISPLAY`, `AGENCY_RECORD`, or `FIELD_ACQUISITION`), priority/class, custodian, owner, intended use, and explicit non-uses. | Source-catalog completeness test |
| FR-077 | A source integration shall separately model discovery, authentication, entitlement, download/receipt, processing, and display endpoints; successful catalog search shall not mark an asset downloaded, licensed, validated, or usable. | State-transition and negative-path tests |
| FR-078 | Before `APPROVED_FOR_USE`, one permitted AOI sample shall pass license/redistribution/offline-rights, extent/time, schema/format, CRS/vertical datum, resolution/scale, unit/NoData, quality/uncertainty, checksum, cadence/latency, authentication/quota/cost, reviewer, and reproducibility checks. | AOI acquisition-gate suite |
| FR-079 | The system shall record shared lineage and mirror relationships and shall prevent the same underlying observation, building, population surface, rainfall input, DEM, or land-cover input from being counted as independent evidence or summed without an approved reconciliation method. | Duplicate/mirror fixture |
| FR-080 | Every criterion, gate, equation parameter, and processing input shall identify an approved source/acquisition method or an explicit `MISSING` acquisition task. Mandatory missing, stale, inaccessible, failed, or out-of-coverage inputs shall yield `UNKNOWN`/`HOLD`. | Parameter-source coverage report |
| FR-081 | API adapters shall expose entitlement, token/credential state, quota/rate/cost limits, last successful catalog request, last successful download, source observation age, next retry, and governed-file fallback without logging secrets. | Adapter health and secret-leak tests |
| FR-082 | The Wayanad sandbox shall support the minimum public stack in source-register §7 and synthetic household/sample parcel fixtures, with synthetic and real records visibly separated and impossible to merge accidentally. | Sandbox provenance and isolation test |
| FR-083 | Production site approval/allocation controls shall verify applicable S45–S50 land/right, household/participation, qualified survey, water/service, and programme/funding evidence before enabling the relevant action. | Blocker-gate scenarios |
| FR-084 | Basemap configuration shall record provider/style/source version, key/credential method, attribution, permitted display/export/cache/offline use, privacy implications, quotas, availability, and fallback, and shall keep basemap pixels out of analytical lineage. | Basemap-license and lineage test |

## 4. Interface requirements

### 4.1 General API contract

- Versioned HTTPS APIs under `/api/v1`; UTF-8 JSON unless a geospatial/download media type is negotiated.
- UUID identifiers; ISO 8601 timestamps with offsets; stored canonical time in UTC; explicit units.
- Idempotency keys for retriable creation/command requests.
- Optimistic concurrency using record version and `ETag`/`If-Match` for mutable resources.
- Cursor pagination and server-enforced page limits; filters must honor authorization scope.
- Standard error object: `code`, `message`, `correlation_id`, field errors, retryability, and safe remediation text.
- Long processing requests return an asynchronous job resource with state, progress, input/output versions, cancellation eligibility, and structured failure.
- All state-changing requests produce audit and correlation records.

### 4.2 Geospatial interfaces

- OGC API Features-compatible read interfaces for approved vector collections where disclosure permits.
- GeoJSON for bounded feature exchange; vector tiles for large interactive layers.
- STAC records for raster/imagery assets and derived products using one release-pinned version and declared conformance classes. A new STAC version requires compatibility tests and a controlled migration.
- Cloud-optimized raster delivery where source/use policy permits range requests.
- API responses must declare CRS or follow the relevant standard default; metric calculations must use an appropriate projected CRS, not unqualified degree measurements.
- Official Survey of India boundary requirements and restricted-layer policy apply to publication.

### 4.3 Identity and integration

- OIDC is the preferred interactive/API identity protocol; SAML federation shall be supported through an approved identity broker when required.
- MFA is mandatory for privileged, approval, publication, export of restricted data, and security-administration actions.
- Service integrations use separately managed service identities and least-privilege scopes; user credentials are prohibited.
- Import/export remains a supported first-class integration because government APIs and connectivity cannot be assumed.

## 5. State machines

State dimensions are independent. Evidence maturity never substitutes for an administrative/legal act, and an administrative approval never proves field completion.

### 5.1 Evidence maturity

```text
DRAFT -> ANALYTICAL -> FIELD_VERIFIED -> TECHNICALLY_ENDORSED
      -> EXPIRED | SUPERSEDED | WITHDRAWN
```

- Evidence may be put `ON_HOLD` and later `RESUMED`; expiry requires revalidation before dependent progression.
- Imported official records retain their external authority and provenance; they are not forced through fabricated internal steps.

### 5.2 Administrative/legal state

```text
DRAFT -> UNDER_REVIEW -> RETURNED | REJECTED | APPROVED
APPROVED -> NOTIFIED -> EFFECTIVE
any applicable state -> ON_HOLD | STAYED -> RESUMED
APPROVED/NOTIFIED/EFFECTIVE -> SUPERSEDED | WITHDRAWN
REJECTED/WITHDRAWN -> APPEALED | REOPENED
```

Approval, notification and effective date are separate. Each transition records authority, basis, reason, evidence package, time and notice obligations. A rejection or return is never deletion.

### 5.3 Dataset state

```text
RECEIVED -> VALIDATING -> VALIDATED -> APPROVED_FOR_USE -> SUPERSEDED
                     \-> QUARANTINED -> CORRECTED/REPLACED
```

### 5.4 Candidate-site state

```text
NOMINATED -> DESKTOP_SCREENING -> FIELD_REVIEW -> LEGAL/DOMAIN_REVIEW
          -> COMPARABLE -> PROPOSED -> APPROVED | REJECTED | ON_HOLD
          -> SUPERSEDED | WITHDRAWN
```

`COMPARABLE` and `PROPOSED` require configured gates; `APPROVED` requires authorized workflow completion.

### 5.5 Household/list state

```text
IMPORTED/ENUMERATED -> DUPLICATE_REVIEW -> VERIFIED
 -> ELIGIBILITY_DRAFT -> DRAFT_LIST -> OBJECTIONS_OPEN
 -> REVIEWED -> FINAL_APPROVED -> SUPERSEDED
```

An appeal/review is a linked case that may suspend, stay, qualify or reopen a dependent record without erasing its history. Preference withdrawal and lost contact are separate from refusal.

### 5.6 Objection state

```text
RECEIVED -> ACKNOWLEDGED -> ADMISSIBILITY_REVIEW -> UNDER_REVIEW
 -> HEARING_SCHEDULED/HELD -> DECIDED -> NOTIFIED -> CLOSED
                                      \-> APPEALED/REOPENED
```

### 5.7 Reservation and delivery state

```text
SIMULATED -> RESERVED -> COMMITTED -> RELEASED | CANCELLED | EXPIRED

APPROVED_FOR_DELIVERY -> FUNDING_CONFIRMED -> UNIT_AND_SERVICES_READY
 -> OFFERED -> ACCEPTED -> HANDED_OVER -> OCCUPIED -> FOLLOW_UP_COMPLETE
```

A draft/simulated allocation reserves nothing. Delivery steps may branch, return for defects, pause, or be owned by an external system; every transition preserves evidence and accountable owner.

### 5.8 Source acquisition state

```text
REGISTERED -> DOCUMENTED -> ENTITLED -> SAMPLE_ACQUIRED -> VALIDATING
 -> VALIDATED -> APPROVED_FOR_USE -> STALE | SUSPENDED | SUPERSEDED
                                  \-> QUARANTINED
```

`CATALOG_VISIBLE` is recorded as a capability observation and is never equivalent to `SAMPLE_ACQUIRED`. A governed manual/file route may move through the same evidence states without an API. `STALE`, `SUSPENDED`, `QUARANTINED`, missing, or out-of-coverage mandatory inputs produce `UNKNOWN`/`HOLD` until a reviewed replacement version is approved.

## 6. Data contracts

### 6.1 Common fields

All versioned business records shall include:

- `id`, `version_id`, `programme_id`, `jurisdiction_id`;
- lifecycle/authority `state`, `effective_from`, optional `effective_to`;
- `valid_time` (when the fact is true) and `system_time` (when the system learned/stored it);
- `created_at`, `created_by`, `updated_at`, `updated_by` where applicable;
- source/evidence links, confidence/uncertainty, classification, and policy/schema version;
- geometry plus source CRS where spatial; canonical storage geometry must not erase the source representation;
- supersedes/superseded-by links and row version for concurrency.

### 6.2 Dataset version

Required fields: title, publisher, source URL/reference, source-register ID, source-capability type, responsible custodian and owner, license/use basis including redistribution/cache/offline rights, access classification, acquisition method and endpoint, authentication/entitlement state, quota/rate/cost conditions, acquired time, observation/valid period, expected cadence/latency, geographic coverage, schema, CRS and vertical datum, scale/resolution, units and NoData, method, uncertainty/quality, limitations/non-uses, mirror/shared-input links, fallback route, original-object URI, media type, byte size, SHA-256 checksum, ingestion run, quality report, reviewer, approval state, and supersession.

### 6.3 Core record groups

| Group | Records and essential relationships |
| --- | --- |
| Programme | `programme`, `jurisdiction`, `authority`, `role_assignment`, `workflow_definition`, `sla_policy`, `publication_profile` |
| Evidence | `dataset`, `dataset_version`, `processing_run`, `evidence_item`, `observation`, `review`, `source_conflict` |
| Source acquisition | `source_capability`, `source_endpoint`, `source_entitlement`, `acquisition_attempt`, `source_sample`, `source_health`, `source_dependency`, `source_mirror_group` |
| Hazard/exposure | `hazard_layer`, `hazard_zone_version`, `settlement`, `building_observation`, `population_baseline`, `exposure_estimate` |
| Land | `parcel`, `parcel_version`, `record_interest`, `possession_observation`, `legal_claim`, `encumbrance`, `forest_fra_status`, `legal_review` |
| Site | `candidate_site`, `site_parcel`, `assessment`, `criterion_result`, `gate_result`, `capacity_version`, `cost_estimate` |
| Household | `household`, restricted `household_member`, `household_member_period`, `household_merge_split`, `identity_evidence`, `duplicate_candidate`, `necessity_assessment`, `eligibility_result`, `priority_result`, purpose-specific `participation_record`, `preference`, `offer_response` |
| Programme finance/delivery | `scheme_version`, `entitlement_assessment`, `cost_head`, `funding_source`, `sanction`, `funding_transaction`, `delivery_milestone`, `external_handoff` |
| Allocation | `policy_set`, `formula_version`, `parameter_version`, `policy_formula_activation`, `scenario`, `scenario_input`, `allocation_proposal`, `constraint_result`, `reservation`, `commitment`, `override` |
| Governance | `objection`, `hearing`, `decision`, `approval`, `notice`, `workflow_task`, `escalation_recommendation` |
| Output/audit | `report_template`, `export_artifact`, `export_manifest`, `audit_event`, `signature_record` |

Personal/member data and public/analytical spatial data must be separable for access, encryption, export, retention, and deletion/restriction workflows.

### 6.4 Minimum executable relational contract

The physical schema may rename fields, but it shall preserve these types, cardinalities and constraints. `uuid` identifiers are non-null; timestamps are `timestamptz`; money stores integer minor units plus ISO 4217 currency; measured values store decimal value plus unit; geometries declare subtype/SRID and retain source CRS metadata.

| Record | Required keys and cardinality | Mandatory constraints |
| --- | --- | --- |
| `programme` | one `jurisdiction`; many policy/workflow/scheme versions | unique `(jurisdiction_id, code, version)`; non-overlapping effective periods per active code |
| `dataset_version` | one dataset; one immutable object; zero/many derived runs | unique `(dataset_id, version)` and checksum/object-version pair; valid period well formed; approved use requires coverage/license/reviewer |
| `processing_run` | one formula/software version; many pinned inputs and outputs | immutable input set; terminal status in success/failed/cancelled; no success without complete lineage |
| `settlement`/`community` | one programme scope; versioned geometry; many households/shared assets | source identity unique within dataset version; geometry valid or quarantined |
| `household` | one current programme case; many historical memberships/preferences/assessments | stable household UUID; no building-derived creation; merge/split only through explicit event |
| `household_member_period` | one person-token and household; `[valid_from, valid_to)` | no overlapping membership for same programme/person unless reviewed multi-household rule permits it; member attributes remain restricted |
| `household_merge_split` | source set to target set, reason and effective time | at least one source/target; cycle prohibited; prior IDs retained as aliases/history |
| `parcel_version` | one parcel; one dataset/source geometry; many interests/observations | unique `(parcel_id, version)`; source geometry never overwritten; corrected version cites transformation/reviewer |
| `candidate_site` | one programme; many `site_parcel` rows and resource capacities | geometry derived from pinned parcel versions; overlapping candidates carry a shared-capacity/overlap group |
| `site_parcel` | many-to-many site↔parcel-version with contribution fraction | unique `(site_id, parcel_version_id)`; fraction in `(0,1]`; duplicated/overlapping area not counted twice |
| `resource_capacity_version` | one site or shared-resource group; resource type/unit/time/phase | nonnegative approved/committed/reserved amounts in compatible units; committed+reserved ≤ approved |
| `necessity_assessment` | one case/settlement version; many alternatives/evidence/reviews | conclusion enum includes `RELOCATION_REQUIRED`, `MITIGATION_IN_PLACE`, `MIXED`, `UNKNOWN`; no official conclusion without competent review |
| `scheme_version` | one programme/jurisdiction; many eligibility/cost/milestone rules | unique code/version/effective period; no silent use outside hazard/category/date scope |
| `entitlement_assessment` | one household/pathway/scheme version; many rule results | eligibility enum `ELIGIBLE/INELIGIBLE/UNKNOWN`; relocation need stored separately |
| `funding_transaction` | one household/site/programme cost head and source reference | amount >0; state transition constrained; external transaction/sanction reference unique per funding source where supplied |
| `preference`/`acceptance` | one household, purpose, option/pathway and validity period | purpose enum prevents reuse across data processing, participation, pathway, ranking and offer acceptance; withdrawal supersedes prior version |
| `scenario` | pinned policy/formula/parameter/solver/input versions; many proposals | draft scenarios create no reservation; terminal solver status and diagnostics required |
| `allocation_proposal` | unique household outcome per scenario | exactly one assigned option or explicit unassigned outcome; no household split; all resource uses recorded |
| `reservation` | proposal/approval plus site/shared resource and quantity | unique idempotency key; transactional state; active quantities cannot exceed remaining capacity; expiry/release auditable |
| `objection`/`appeal` | one challenged object/version; many events/evidence/notices | immutable filed time/receipt; decision and notice separate; stays/holds linked to dependencies |
| `delivery_milestone` | one household/pathway/site-unit or external handoff | type/state/evidence/reviewer/time required; completion prohibited while required predecessor, service or defect remains unresolved |
| `audit_event` | one stream sequence and prior hash; optional actor, mandatory service/authority context | append-only; canonical encoding/version; personal payload minimized; deletion/legal-hold events do not break verification |

Foreign keys use restrict or explicit supersession/closure behavior for decision-used records; cascading deletion is prohibited for evidence, decisions, reservations, approvals, notices and audit. Nullable fields must state why absence is lawful; missing mandatory domain evidence is represented by an explicit state, not a nullable value interpreted as pass.

### 6.5 Formula, parameter, and compliance records

`formula_version`, `parameter_version`, `policy_formula_activation`, `legal_instrument`, and `compliance_obligation` shall implement the contracts in [equations.md](./equations.md), [parameters.md](./parameters.md), and [source-register.md](./source-register.md). An activation uniquely pins formula, parameters, policy, programme/jurisdiction, effective period, approver and test-evidence version. Specialist or rejected formulas cannot be activated by ordinary configuration.

### 6.6 Source capability and parameter dependency

`source_capability` preserves the S01–S54 identity, class, source/product versus service role, owner, intended use, non-uses, and review trigger. Each `source_endpoint` has its own authentication, entitlement, quota, cost, terms, and observed health. `source_dependency` links a requirement, gate, criterion, equation input, or parameter to acceptable source versions and defines whether alternates are equivalent, complementary, or research-only. A mandatory dependency has an explicit acquisition task and cannot be satisfied by a display basemap, catalog listing, correlated mirror, or synthetic fixture in production.

## 7. Non-functional requirements

### 7.1 Performance and scale

| ID | Requirement |
| --- | --- |
| NFR-001 | Standard authorized read APIs shall meet p95 ≤1 second and p99 ≤2.5 seconds under the agreed Wayanad reference load, excluding file downloads and declared asynchronous jobs. |
| NFR-002 | After initial layer load, ordinary pan/zoom/filter interactions shall respond within 2 seconds at p95 on the pilot's supported device/network profile. |
| NFR-003 | The initial district workspace shall become usable within 5 seconds at p75 on a 10 Mbps connection using the reference dataset and cold browser cache. |
| NFR-004 | Spatial analysis, ingestion, allocation, and report work expected to exceed 3 seconds shall be asynchronous, cancellable where safe, observable, and retryable without duplicate business effects. |
| NFR-005 | Load tests shall cover at least 250,000 households, 2 million vector features in a district workspace, 100 concurrent interactive users, and 20 concurrent heavy jobs before controlled production. These are engineering targets, not Wayanad population claims. |

### 7.2 Availability, continuity, and correctness

| ID | Requirement |
| --- | --- |
| NFR-006 | Production service target is 99.5% monthly availability excluding notified maintenance; degraded read-only access should remain available when processing/integration services fail. |
| NFR-007 | Production recovery targets are RPO ≤15 minutes and RTO ≤4 hours for core records; immutable source objects and approved exports shall have independently tested restore procedures. |
| NFR-008 | Encrypted backups shall be automated, geographically separated within the approved India-residency boundary, restore-tested quarterly, and retained according to an approved schedule. |
| NFR-009 | Money and count-based authoritative records shall use exact integer/decimal storage and explicit units. Numerical libraries/solvers may use floating point only with recorded conversion, scaling, tolerances and error handling; every proposal shall pass the independent domain/integer feasibility validator. |
| NFR-010 | Reprocessing with unchanged code/config/input versions shall produce equivalent results and a recorded processing environment/version. |

### 7.3 Security and privacy

| ID | Requirement |
| --- | --- |
| NFR-011 | All network traffic shall use current approved TLS; stored databases, objects, backups, and offline field data shall be encrypted with managed key rotation and separation of duties. |
| NFR-012 | Authorization shall combine role, programme, geography, record state, action, and data classification, with database-enforced row isolation for sensitive business records. |
| NFR-013 | Database owners/superusers and break-glass access shall not be used by ordinary application flows; all break-glass use requires approval, time limit, reason, alert, and review. |
| NFR-014 | High-risk fields shall use field/column-level protection or tokenization where needed and shall be redacted from logs, traces, support bundles, and ordinary exports. |
| NFR-015 | Secrets shall be centrally managed and never stored in source, client bundles, exported configuration, or logs. |
| NFR-016 | Security assurance shall include threat modeling, SAST, dependency/container scanning, secret scanning, DAST, authorization testing, API abuse/rate tests, and independent penetration testing before controlled production. |
| NFR-017 | The system shall support data-subject/citizen correction, access, erasure/restriction, notice, breach, and grievance workflows to the extent applicable, while preserving records required by law and audit schedules. |
| NFR-018 | Fine geospatial and personal data shall remain in approved India-resident infrastructure and approved support paths under the project/government hosting baseline and any applicable legal restriction. Documentation shall not misstate this deployment policy as a universal DPDP localization mandate. |

### 7.4 Accessibility, localization, and usability

| ID | Requirement |
| --- | --- |
| NFR-019 | All critical web workflows shall meet WCAG 2.2 AA and applicable GIGW 3.0 requirements, verified by automated checks and manual keyboard/screen-reader testing. |
| NFR-020 | Every map task required for a decision shall have an equivalent accessible table/search/form flow. Status shall use text/icon/pattern as well as color. |
| NFR-021 | Pilot UI, notices, explanations, and household-facing exports shall support English and Malayalam with controlled translation review and fallback that never hides a warning. |
| NFR-022 | Dates, numbers, units, names, addresses, and search shall support Indian formats and Unicode without changing canonical stored values. |

### 7.5 Maintainability and observability

| ID | Requirement |
| --- | --- |
| NFR-023 | Domain modules shall expose documented contracts and prohibit direct cross-domain data mutation outside approved services/workflows. |
| NFR-024 | Schema, rule, policy, report-template, and API changes shall be versioned and backward compatible for an announced support window or provide a tested migration/rollback path. |
| NFR-025 | Logs, metrics, traces, job status, data-quality alerts, audit-integrity alerts, and integration-health indicators shall use correlation IDs and exclude unnecessary personal data. |
| NFR-026 | Operational alerts shall distinguish service failure, data-quality failure, overdue human work, and substantive case status; the system shall not label ordinary workflow delay as a disaster alert. |
| NFR-027 | Source code, configuration, infrastructure definitions, and templates shall pass review, automated tests, and reproducible build controls before release. |

### 7.6 Reliability, delivery-path security, and compliance operations

| ID | Requirement |
| --- | --- |
| NFR-028 | A committed business change that requires asynchronous work shall use a transactional outbox or equivalent durable commit-to-delivery mechanism, with idempotent consumers, deduplication, retry limits, dead-letter handling and reconciliation. |
| NFR-029 | Authorization and leakage tests shall cover APIs, HTML, search, OGC/STAC metadata, vector tiles, COG/object range delivery, signed URLs, reports, worker credentials and every cache key/path. |
| NFR-030 | RLS tests shall cover table owners, superusers, `BYPASSRLS`, views/security definer behavior and pooled-connection context reset. No ordinary runtime identity may bypass the intended policy. |
| NFR-031 | Offline design shall define key custody/unlock, device/browser binding, shared-device logout, XSS controls, quota/eviction handling, attachment limits, expiry/revocation limits, and recovery/export of unsynced evidence without promising guaranteed remote wipe. |
| NFR-032 | Restore tests shall recover and reconcile database records, object versions/inventories, audit checkpoints, signing/encryption keys and manifests as one consistency set; missing referenced evidence shall fail integrity checks. |
| NFR-033 | Where CERT-In directions apply, operations shall maintain the designated point of contact, synchronized clocks, applicable incident reporting within six hours, and securely retained ICT logs for a rolling 180 days within India, with exercised procedures and effective-date review. |
| NFR-034 | Source adapters shall use bounded retries, backoff, circuit breaking, timeouts, rate/quota controls, credential rotation, and version-addressed caches; external failure shall not exhaust worker capacity or silently serve an unlabelled stale result. |
| NFR-035 | A machine-readable source-readiness report shall be reproducible for each release and shall show every mandatory dependency, acquisition state, sample/checksum, coverage, age, license/entitlement, reviewer, fallback, and blocking reason. |

## 8. Acceptance scenarios

| ID | Scenario | Required result |
| --- | --- | --- |
| AT-01 | A flat Wayanad parcel intersects an authoritative debris-flow channel/runout zone | Local slope does not cause a pass; hazard gate fails/holds with source and explanation |
| AT-02 | Revenue record says vacant government land; imagery/footprints show structures | A discrepancy task is created; legal/vacancy state stays unknown/blocked; no automatic rejection or clearance |
| AT-03 | Source cadastral layer is offset from verified control points | Both geometries remain; correction metrics/reviewer are recorded; dependent analysis uses the selected version visibly |
| AT-04 | A parcel has a pending FRA/community-rights issue | Site cannot become allocatable; applicable legal/consultation workflow is assigned without a blanket legal conclusion |
| AT-05 | Water demand meets 55 LPCD on paper but lean-season yield/quality is inadequate | Water gate fails/holds and capacity identifies water as binding |
| AT-06 | A later beneficiary order changes a previously published list | New version supersedes the old; affected reasons/notices/objections and historical export remain traceable |
| AT-07 | A household chooses self-relocation rather than township allocation | The two pathways remain distinct and the solver does not assign the household to a township |
| AT-08 | Participating households exceed accessible/site capacity | Solver returns feasible proposals plus explicitly unassigned households and supported contributing reasons; no household is split |
| AT-09 | District workflow exceeds a configured SLA | Reminders/escalation recommendation go only to configured authorities; no substantive state changes automatically |
| AT-10 | An analytical zone is later officially notified with changed geometry | Both versions and states remain; public view shows only the approved geometry/state and correct effective date |
| AT-11 | Two offline officers edit the same field assessment | Sync preserves both, creates conflict review, and does not silently choose last write |
| AT-12 | Public user requests household-level score or preliminary site coordinates | Access is denied/redacted; an approved aggregate or generalized view is returned where configured |
| AT-13 | Keyboard/screen-reader user completes a site/household review without the map | Equivalent data, state, actions, warnings, and evidence links are available and operable |
| AT-14 | Historical dossier is regenerated after a source update | Regeneration pins original source/policy versions; it does not silently use the latest source |
| AT-15 | Two approvals race for the same dwelling, overlapping land, budget, or shared water | One transaction reserves the capacity; the other fails closed and requires replan; no double subtraction/allocation occurs |
| AT-16 | Solver reaches time limit with a feasible incumbent | Result is labelled feasible-not-proven-optimal with bound/gap; independent validator passes before review; no optimal claim is made |
| AT-17 | Formula receives missing input, zero denominator, wrong CRS, incompatible units, NaN or infinity | Execution fails with typed reason or returns `UNKNOWN`; no silent zero/pass/coercion occurs |
| AT-18 | A tenant fails an owner-only scheme rule but has verified relocation need | Scheme result is ineligible with reason while relocation need remains unchanged and alternate funding/pathway task is created |
| AT-19 | Household was not contacted or withdraws a site preference | Record remains no-contact/withdrawn; it is not treated as refusal, consent or acceptance and no unacceptable option is allocated |
| AT-20 | Site is conditionally approved but water is an allocation-stage blocking condition | Site may remain conditionally approved administratively but cannot be reserved or allocated until the water gate passes |
| AT-21 | Official approval precedes a separate statutory notice | Approval and notification dates/artifacts remain distinct; publication uses only the state permitted by the workflow |
| AT-22 | Approved household is offered an unfinished or unserviced unit | Relocation remains incomplete; handover/occupation is blocked and defects/tasks remain visible |
| AT-23 | Public aggregates can be differenced to isolate a small household group | Disclosure control suppresses/generalizes the output and records the threat-model rule; it does not promise inference is impossible |
| AT-24 | An API request is denied but the same restricted layer is requested by tile, STAC, COG URL, search or cache | Every path denies or returns only an approved projection; no stale cache or signed URL crosses programme/geography scope |
| AT-25 | Database commit succeeds while message publication fails | Durable outbox/reconciliation later delivers exactly the intended idempotent effect; no business state is lost or duplicated |
| AT-26 | Offline storage is evicted or a device is lost before sync | User sees unsynced risk/recovery state; server revokes future access where reachable; no claim of guaranteed remote wipe is made |
| AT-27 | Coordinated restore contains database rows but lacks referenced objects/checkpoints/keys | Integrity check fails the restore; service does not resume authoritative writes until the consistency set is complete |
| AT-28 | Retention period expires while a legal hold covers only part of a case | Eligible personal data is deleted/restricted, held records remain, backup expiry is tracked, and a minimal lawful deletion event preserves audit continuity |
| AT-29 | Two vulnerability categories include the same person | Priority/count calculation follows the approved non-double-count rule and exposes the denominator/source version |
| AT-30 | A source is displayed outside coverage in research mode | It is marked `UNSUPPORTED`, isolated from gates/scenarios/official exports, and the use is audited |
| AT-31 | CDSE STAC returns a Sentinel item but authenticated asset download has not succeeded | Source remains `DOCUMENTED`/`CATALOG_VISIBLE`; no dataset version becomes usable and the dependency stays `HOLD` |
| AT-32 | Earth Search and Planetary Computer expose the same Sentinel observation | Both routes link to one mirror/shared-observation group; exposure does not count them as independent corroboration |
| AT-33 | SoilGrids REST is unavailable while WCS/files remain documented | Optional soil context stays unavailable or uses a separately validated WCS/file adapter; no mandatory workflow fails over to an invented endpoint |
| AT-34 | C-FLOOD is requested for Wayanad or a Himalayan glacial-lake layer is configured for Kerala | Coverage/geography check blocks decision linkage and records `UNSUPPORTED`; approved local alternatives remain explicit |
| AT-35 | Real households are selected while parcel/right, lean-season water, or programme funding records are missing | Scenario/approval remains `HOLD`; synthetic fixtures cannot satisfy the production dependency |
| AT-36 | A positive NDBI, building footprint, DSM, SoilGrids class, or SAR backscatter change is presented as an operational fact it cannot measure | Validation rejects the semantic mapping and creates a review task without setting occupation, title, soil strength, displacement, or safety |
| AT-37 | Basemap key expires or public OSM tile access is unavailable | Analytical records and maps remain source-correct; approved fallback/non-map views work and no public tile bulk-download workaround occurs |
| AT-38 | A permitted AOI source sample has missing CRS, implausible units, unlicensed redistribution, stale time coverage, or checksum mismatch | It is quarantined; source readiness reports the exact failure and no dependent gate can pass |

## 9. Release evidence

Each release candidate shall provide:

- requirements-to-tests and features-to-requirements traceability;
- rule/policy version and migration report;
- schema/API compatibility report;
- data-quality and spatial-regression results;
- unit, integration, end-to-end, solver-property, accessibility, localization, security, performance, backup/restore, and offline-sync results;
- formula/parameter activation report with dimensional, boundary, missing-input, monotonicity, double-counting, rank-reversal, uncertainty and independent-feasibility results;
- pilot evaluation report against the preregistered sample, independent references, seasonal observations, subgroup checks and false-positive/false-negative tolerances;
- unresolved defects and residual-risk acceptance by the named owner;
- deployment, rollback, data migration, support, and incident runbooks.

## 10. Standards and primary references

1. [OGC API Features](https://www.ogc.org/standards/ogcapi-features/)
2. [STAC API Community Standard 1.0](https://docs.ogc.org/cs/25-005/25-005.html)
3. [PostgreSQL row security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
4. [PostGIS `ST_Transform`](https://postgis.net/docs/ST_Transform.html)
5. [MapLibre GL JS](https://maplibre.org/maplibre-gl-js/docs/)
6. [Indian Guidelines on Geospatial Data, 2021](https://dst.gov.in/sites/default/files/Final%20Approved%20Guidelines%20on%20Geospatial%20Data.pdf)
7. [GIGW 3.0](https://guidelines.india.gov.in/gigw3/)
8. [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
9. [Digital Personal Data Protection Rules, 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa)
10. [Google Open Buildings](https://sites.research.google/gr/open-buildings/)
11. [Census of India Primary Census Abstract](https://censusindia.gov.in/nada/index.php/catalog/6191)
12. [Disaster Management (Amendment) Act, 2025](https://prsindia.org/files/bills_acts/acts_parliament/2025/The_Disaster_Management_Act%2C_2025.pdf)
13. [CERT-In Directions, 2022](https://www.cert-in.org.in/PDF/CERT-In_Directions_70B_28.04.2022.pdf)
14. [Equation Registry](./equations.md)
15. [Parameter Registry](./parameters.md)
16. [Operational Source Register](./source-register.md#5-operational-data-source-and-acquisition-register)
