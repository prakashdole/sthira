---
title: PUNARVAS-AI Decision Record
document_id: PUN-DECISIONS
version: 1.2
status: Controlled decision record with accepted and proposed entries
as_of: 2026-09-08
audience: Programme owners, product, legal, domain, architecture, engineering, security, assurance, and future maintainers
owner: Product and architecture decision forum
normative_scope: Material decisions, rationale, rejected alternatives, consequences, evidence, and review triggers
---

# PUNARVAS-AI Decision Record

## 1. How to use this log

This file explains **why** the baseline is designed as documented. Requirements remain normative in [prd.md](./prd.md), [rules.md](./rules.md), and [trd.md](./trd.md); architecture is in [architecture.md](./architecture.md).

A material change must add or supersede a decision rather than rewrite the old rationale. Decision status values are `PROPOSED`, `ACCEPTED`, `SUPERSEDED`, or `REJECTED`.

## 2. Decision index

| ID | Decision | Status |
| --- | --- | --- |
| DEC-001 | Permanent-relocation decision support, not tactical response | ACCEPTED |
| DEC-002 | Wayanad, Kerala as the reference pilot | ACCEPTED |
| DEC-003 | Production blueprint for a mixed delivery audience | ACCEPTED |
| DEC-004 | Evidence precedence, provenance, and bitemporal versioning | ACCEPTED |
| DEC-005 | Human authority and explicit analytical-to-official states | ACCEPTED |
| DEC-006 | Hard gates plus separate explainable dimensions; no master score | ACCEPTED |
| DEC-007 | Capacity-constrained MILP for advisory household allocation | ACCEPTED |
| DEC-008 | Next.js/FastAPI modular monolith with asynchronous workers | ACCEPTED |
| DEC-009 | PostGIS system of record and open geospatial interfaces | ACCEPTED |
| DEC-010 | Private household casework and de-identified public transparency | ACCEPTED |
| DEC-011 | Offline field work and governed file exchange are first-class | ACCEPTED |
| DEC-012 | Append-only hash-linked audit and manifests, not blockchain | ACCEPTED |
| DEC-013 | Export into Kerala LSG plan process; do not replace it | ACCEPTED |
| DEC-014 | WCAG 2.2 AA/GIGW 3.0 and English/Malayalam pilot | ACCEPTED |
| DEC-015 | Consume authoritative hazard products; no baseline custom model | ACCEPTED |
| DEC-016 | Configurable SLA escalation without authority bypass | ACCEPTED |
| DEC-017 | India-resident restricted data and defense-in-depth access | ACCEPTED |
| DEC-018 | Township and approved self-relocation pathways | ACCEPTED |
| DEC-019 | Version changing Wayanad facts; do not hard-code them | ACCEPTED |
| DEC-020 | No LLM or voice functionality in authoritative paths | ACCEPTED |
| DEC-021 | Conservative production SLO and recovery baseline | ACCEPTED |
| DEC-022 | Accessible HTML plus evidence-bound multi-format exports | ACCEPTED |
| DEC-023 | Retain every mathematical family in a governed equation registry | ACCEPTED |
| DEC-024 | Allocation objective and tie policy require separate programme approval | PROPOSED |
| DEC-025 | Draft scenarios do not reserve; approval atomically reserves capacity | ACCEPTED |
| DEC-026 | Add necessity, scheme/funding, and completion/handoff domains | ACCEPTED |
| DEC-027 | Use transactional outbox and independent feasibility validation | ACCEPTED |
| DEC-028 | Separate consent purposes, state axes, retention, and compliance applicability | ACCEPTED |
| DEC-029 | Use prototype, authorized-pilot, and controlled-production deployment profiles | ACCEPTED |
| DEC-030 | Pin standards, basemap and specialist-model activation separately | PROPOSED |
| DEC-031 | Separate source products, access services, processing, display, and operational readiness | ACCEPTED |
| DEC-032 | Build with public/synthetic data first; block real decisions on agency and field evidence | ACCEPTED |
| DEC-033 | Phase 0 research findings and closure of baseline operational decisions | ACCEPTED |
| DEC-034 | PKG-0C shared domain contracts and Ponytail modular architecture | ACCEPTED |
| DEC-035 | Phase 2 Wayanad Shadow Pilot, Evaluation Protocols, and Rehearsal Architecture | ACCEPTED |
| DEC-036 | Kerala Multi-District Scaling, Policy Inheritance, and Row-Level Isolation | ACCEPTED |
| DEC-037 | Multi-State Tenant Adaptation and Cross-State Leakage Prevention | ACCEPTED |
| DEC-038 | National NDMA Sovereign Relocation Clearinghouse and Inter-State Federation | ACCEPTED |
| DEC-039 | Controlled Live Wayanad Operations, Recovery Harness, and Delivery Completion Verification | ACCEPTED |


### 2.1 Approval provenance

The user explicitly instructed **“PLEASE IMPLEMENT THIS PLAN”** on 8 September 2026. That instruction approves DEC-001–023 and DEC-025–029 as the documentation baseline, including PUNARVAS-AI, Wayanad as reference pilot, production-oriented documentation, and the named technology direction. The user's later request to add the supplied data-source package approves DEC-031–032 as documentation and delivery policy. DEC-033 and DEC-034 record research closures and implementation contracts executed per user directive. DEC-024 and DEC-030 remain proposed until specifically configured.

| Decisions | Proposed by | Approved by/date | Approval evidence |
| --- | --- | --- | --- |
| DEC-001–022 | Documentation analysis based on `idea.txt`, `claude.txt` and cited sources | User / 2026-09-08 | Current-thread directive “PLEASE IMPLEMENT THIS PLAN” |
| DEC-023, DEC-025–029 | Astra corrective review reconciled by this revision | User / 2026-09-08 | Current request: read all handoff material and make the changes in the specified files |
| DEC-031–032 | Supplied data-source feasibility handoff reconciled by revision 1.2 | User / 2026-09-08 | Current request: add all supplied data sources according to their respective files |
| DEC-033–034 | Agent research and implementation execution | User / 2026-09-08 | Current request: start coding and execute Phase 0 research and Phase 1 implementation |
| DEC-024, DEC-030 | Revision authors | Not yet approved | [Open Decisions](./open-decisions.md) ODN-006–008 and ODN-013–019 |

## 3. Detailed decisions

### DEC-001 — Permanent-relocation decision support, not tactical response

- **Context:** Early transcript sections combined long-term relocation planning with real-time emergency command, logistics, evacuation, voice, and live telemetry. Later discussion recognized these as different missions, users, data latencies, liabilities, and operating conditions.
- **Decision:** PUNARVAS-AI is scoped to proactive/permanent relocation planning and programme governance. Tactical command, civilian evacuation routing, convoy/airdrop logistics, live sensor command, and ration calculations are excluded.
- **Why:** A focused system can be validated against land, rights, water, livelihood, participation, and approval outcomes. Combining emergency command would expand safety-critical liability and prevent either product from being credible.
- **Rejected:** A dual-horizon platform; a “future-ready” tactical module hidden behind a toggle.
- **Consequences:** No sub-second live incident feeds are required. A separate future product/decision would be needed for emergency operations.
- **Evidence:** Later conclusions in `claude.txt`; the Disaster Management Act distinguishes planning, coordination, response, rehabilitation, and local-authority responsibilities.
- **Review trigger:** A separately funded emergency-command mandate with its own safety case, users, operating procedure, and acceptance requirements.

### DEC-002 — Wayanad, Kerala as the reference pilot

- **Context:** Earlier chats considered Wayanad and Uttarakhand. The user's 8 September 2026 instruction explicitly approved the supplied plan naming Wayanad as the reference pilot. Wayanad also has relevant KSDMA/GSI hazard material, a 2024 PDNA, government orders, a township programme, a vulnerability-linked relocation precedent, and documented runout, livelihood, and beneficiary-list issues.
- **Decision:** Build and validate the baseline around Wayanad while keeping all legal, policy, hazard, and language behavior configurable.
- **Why:** A real bounded district exposes the hard interdisciplinary problems and provides enough primary evidence for validation without pretending nationwide uniformity.
- **Rejected:** All-India first release; a synthetic-only pilot; Uttarakhand as the initial reference.
- **Consequences:** English/Malayalam and Kerala LSG workflows receive first implementation. Wayanad facts cannot become national defaults.
- **Evidence:** User approval dated 2026-09-08; [KSDMA 2024 reports](https://sdma.kerala.gov.in/reports-landslides-2024/), [government orders](https://sdma.kerala.gov.in/government-orders-5/), [township site](https://wayanadtownship.kerala.gov.in/).
- **Review trigger:** Completion of controlled Wayanad use or loss of lawful/operational access to essential pilot data.

### DEC-003 — Production blueprint for a mixed delivery audience

- **Context:** The transcripts sometimes optimize for a 36-hour hackathon. The user explicitly approved the production documentation plan and mixed product/engineering/GIS/legal/field/government readership on 8 September 2026.
- **Decision:** Specify production controls, operations, privacy, accessibility, audit, remedy, and phased rollout; keep documents readable across disciplines.
- **Why:** Relocation decisions affect rights, safety, land, and public money. A demo-only design would omit the controls needed for responsible use.
- **Rejected:** Hackathon-only Flask/SQLite prototype specification; procurement-only document; engineering-only design.
- **Consequences:** Delivery takes phases and institutional agreements; demo shortcuts cannot be promoted directly to production.
- **Evidence:** Current-thread directive “PLEASE IMPLEMENT THIS PLAN,” dated 2026-09-08.
- **Review trigger:** User changes the intended deliverable or audience.

### DEC-004 — Evidence precedence, provenance, and bitemporal versioning

- **Context:** Transcripts contain changing claims and code. Official counts and programme status also change over time. Land/hazard sources can conflict or be superseded.
- **Decision:** Current user intent controls product scope; later chat intent supersedes earlier chat intent; primary authoritative evidence controls factual/legal assertions. Preserve source and derived versions with valid and system time, lineage, checksum, limitations, and supersession.
- **Why:** Overwriting destroys the ability to explain what an authority knew at a decision date. A single universal source rank would also be misleading across domains.
- **Rejected:** Latest-value-only database; transcript-as-spec; one global “truth score”; copying code/formulas as requirements.
- **Consequences:** Storage and UI must handle conflicts and historical reconstruction; analysts must select a version explicitly.
- **Evidence:** DILRMP 3.0 quality/georeferencing guidance, changing Wayanad reports/orders, STAC provenance model.
- **Review trigger:** Adoption of an authoritative cross-government evidence/versioning standard that safely replaces these contracts.

### DEC-005 — Human authority and explicit analytical-to-official states

- **Context:** Publishing an unverified hazard zone or beneficiary result can affect rights, property values, and public trust. Software cannot confer legal authority.
- **Decision:** Use explicit `ANALYTICAL → FIELD_VERIFIED → TECHNICALLY_ENDORSED → OFFICIALLY_APPROVED/NOTIFIED` states, with superseded/withdrawn history. Only configured authorized acts create official state.
- **Why:** State labels prevent advisory output from being mistaken for a gazette/order and preserve accountability for each transition.
- **Rejected:** Binary draft/final; automatic approval after score threshold; automatic gazetting or allocation.
- **Consequences:** Workflows require named domain reviewers and signatories. Some cases remain on hold rather than receiving an automated answer.
- **Evidence:** Disaster Management Act roles; RFCTLARR review/publication process; KSDMA/LSG approval practice.
- **Review trigger:** A specific statutory digital-decision mechanism with defined automated authority.

### DEC-006 — Hard gates plus separate explainable dimensions; no master score

- **Context:** The transcripts propose additive and multiplicative master formulas with invented weights/constants. Such scores can hide a legal block or weak evidence and falsely borrow authority from UNDRR terminology.
- **Decision:** Evaluate mandatory gates as `PASS/FAIL/UNKNOWN/BLOCKED`; then compare remaining candidates across separately visible criteria. MCDA is versioned, approved, sensitivity-tested, and advisory. Household priority, site suitability, legal readiness, and institutional readiness never share one score.
- **Why:** A parcel cannot compensate for unresolved title or unsafe water by scoring well on roads. Separate dimensions make evidence, trade-offs, and uncertainty contestable.
- **Rejected:** `Ω`; fixed carrying-capacity score; `H×E×V/C` as a prescribed operational equation; NDBI or land-category multipliers.
- **Consequences:** UI and reports are more detailed; policy owners must define thresholds and missing-data behavior. Rankings may legitimately remain unstable/undetermined.
- **Evidence:** [UNDRR disaster risk](https://www.undrr.org/terminology/disaster-risk), peer-reviewed GIS/MCDA practice with sensitivity analysis, [JJM](https://jaljeevanmission.gov.in/about_jjm).
- **Review trigger:** Independently validated domain model for a narrowly defined criterion; it remains subject to gate and explanation rules.

### DEC-007 — Capacity-constrained MILP for advisory household allocation

- **Context:** The transcript suggests Hungarian one-to-one assignment and hospital–residents/stable matching. One site serves many households, households are indivisible, and the baseline needs heterogeneous dwelling/person/accessibility/shared-resource/budget/timing constraints. Stable matching can legitimately use programme priority orders, but does not natively express this complete constraint set.
- **Decision:** Model advisory allocation with binary household-to-option variables and a capacity-constrained mixed-integer linear formulation. Enforce consent, unacceptable options, household indivisibility, dwelling/person/accessibility capacities, and an explicit unassigned outcome.
- **Why:** MILP represents the combined constraints and supports auditable feasibility/diagnostics without splitting households. This is a stronger justification than claiming sites cannot have policy priorities.
- **Rejected:** Hungarian one-to-one assignment as the general model; unmodified hospital-residents matching for the full multi-resource problem; greedy rank assignment; forced assignment for every household. These remain documented alternatives for narrower cases in E27.
- **Consequences:** Solver/model/version, scaling, tolerances and status diagnostics must be pinned; every output needs independent feasibility validation. Objective order, coverage unit, tie policy and fairness constraints require separate policy approval. Results remain proposals and drafts reserve nothing.
- **Evidence:** Problem structure derived from PRD workflow and real capacity/preference needs; research literature supports constrained location/allocation as decision support, not automatic entitlement.
- **Review trigger:** Field evidence shows another transparent algorithm meets constraints/fairness better, or scale makes MILP operationally unsuitable.

### DEC-008 — Next.js/FastAPI modular monolith with asynchronous workers

- **Context:** There is no existing application. The product has interdependent transactional workflows plus heavy geospatial/report jobs. Early microservices would add failure modes and operational burden.
- **Decision:** Use Next.js/TypeScript PWA, a Python FastAPI modular monolith, and Celery/RabbitMQ workers. Enforce domain module interfaces and keep heavy processing as isolated processes.
- **Why:** Python has mature geospatial/optimization support; TypeScript supports a robust accessible web client; a modular monolith preserves transactions and delivery speed while remaining split-ready.
- **Rejected:** Full microservices from day one; Flask/SQLite production stack; Node-only geospatial backend; serverless-only design.
- **Consequences:** Module ownership and dependency rules are mandatory. Scaling begins with workers, tiles, indexes, and replicas before service extraction.
- **Evidence:** Product workload and operational constraints; [architecture.md](./architecture.md) split triggers.
- **Review trigger:** Independent scaling/security/ownership evidence justifies extracting a module.

### DEC-009 — PostGIS system of record and open geospatial interfaces

- **Context:** The system must relate legal records and workflow to spatial versions, serve large layers, and exchange government/open formats.
- **Decision:** Use PostgreSQL/PostGIS, STAC for raster/imagery catalogs, OGC API Features/GeoJSON for bounded vectors, MVT for large interactive layers, COG-compatible raster delivery, and explicit projected CRS for metric operations.
- **Why:** This keeps spatial and transactional integrity together while using mature, open standards and avoiding degree-based distance/area mistakes.
- **Rejected:** Shapefiles as the system of record; proprietary map APIs as canonical storage; browser-only spatial calculation; separate NoSQL geostore by default.
- **Consequences:** Spatial indexing, transformation lineage, geometry QA, and RLS-safe tile queries require specialist testing.
- **Evidence:** [OGC API Features](https://www.ogc.org/standards/ogcapi-features/), [STAC](https://docs.ogc.org/cs/25-005/25-005.html), [PostGIS ST_Transform](https://postgis.net/docs/ST_Transform.html).
- **Review trigger:** A mandated government interoperability standard supersedes a selected interface.

### DEC-010 — Private household casework and de-identified public transparency

- **Context:** Publicly explainable criteria can reduce distrust, but household vulnerability, disability, identity, bank, contact, land-claim, and precise-location information can cause harm and is subject to data protection.
- **Decision:** Keep case/evidence details restricted. Publish approved criteria, methods, aggregates, progress, notices, and only records specifically lawful and approved for disclosure. Provide each authenticated household its own reasons and remedy path.
- **Why:** Transparency does not require exposing personal data or a public household score. Purpose limitation and data minimization are compatible with explainable decisions.
- **Rejected:** Public household ranking with full reasons; entirely closed programme; anonymization assumed safe without re-identification review.
- **Consequences:** Public projections and disclosure review are separate; public and private views require distinct tests and incident response.
- **Evidence:** [DPDP Rules 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa), GIGW, Wayanad beneficiary-list disputes.
- **Review trigger:** A binding publication order changes which named fields must be public; disclosure still uses the minimum required.

### DEC-011 — Offline field work and governed file exchange are first-class

- **Context:** Field connectivity and government APIs can be unavailable. A cloud-only/API-only system would exclude evidence and users or encourage unmanaged side channels.
- **Decision:** Provide encrypted offline PWA packages, idempotent conflict-aware sync, and governed file import/export alongside APIs.
- **Why:** This reflects actual field and institutional conditions and preserves provenance better than ad hoc spreadsheets/messages.
- **Rejected:** Always-online native app; web-only forms; API-only integration; last-write-wins sync.
- **Consequences:** Device management, package expiry, conflict review, malware validation, and lost-device procedures become mandatory.
- **Evidence:** Wayanad/field workflow needs and government data-access reality.
- **Review trigger:** Managed connectivity and authoritative APIs achieve proven universal coverage for the target users.

### DEC-012 — Append-only hash-linked audit and manifests, not blockchain

- **Context:** Beneficiary, site, and decision changes need tamper evidence. The transcript proposed HSM/blockchain-like claims without a justified trust model.
- **Decision:** Use atomic append-only audit events, canonical hashes linked to prior events, periodic signed checkpoints, object/version checksums, and export manifests. Describe it as tamper-evident, not immutable.
- **Why:** It supports practical reconstruction and detection with conventional operations. Blockchain does not prevent authorized systems from receiving false input and adds unjustified complexity.
- **Rejected:** Editable audit table; blockchain; claim that a hash prevents administrators deleting storage; HSM as a baseline prerequisite.
- **Consequences:** Key/checkpoint custody, verification tooling, backup, retention, and break-glass monitoring require ownership.
- **Evidence:** Security engineering principles and PostgreSQL/object-version capabilities; no law requires blockchain for this workflow.
- **Review trigger:** A mandated records/signature standard or threat model requires stronger independently anchored evidence.

### DEC-013 — Export into Kerala LSG plan process; do not replace it

- **Context:** Kerala has an approved participatory LSG disaster-management planning template and approval flow. An auto-filled replacement would omit Gram/Ward Sabha input, working groups, technical scrutiny, District Planning Committee, and DDMA approval.
- **Decision:** Map approved relocation evidence into relevant LSG template sections/annexes and leave participatory/approval fields incomplete until provided through the lawful process.
- **Why:** The product strengthens an existing legitimate process without pretending a database-generated document is the plan itself.
- **Rejected:** New proprietary “VDMP”; fully auto-authored official plan; only a generic DPR.
- **Consequences:** Template versions and mappings must be maintained; LSG users and reviewers participate in validation.
- **Evidence:** [Kerala LSG DM plan framework](https://sdma.kerala.gov.in/local-self-government-dm-plans/).
- **Review trigger:** Kerala publishes a new mandatory digital schema/API or changes the approval process.

### DEC-014 — WCAG 2.2 AA/GIGW 3.0 and English/Malayalam pilot

- **Context:** A map-only English application would exclude affected households and some government/field users. Government web guidance and modern accessibility standards apply.
- **Decision:** Target GIGW 3.0 and WCAG 2.2 AA; provide equivalent non-map workflows; pilot household-facing content in English and Malayalam.
- **Why:** Accessibility and comprehension are prerequisites for informed participation and review, not post-launch enhancements.
- **Rejected:** Map-only UI; English-only pilot; automated translation without review; accessibility deferred to state scale.
- **Consequences:** Design, QA, translation governance, reports, and procurement must include accessibility/localization expertise.
- **Evidence:** [GIGW 3.0](https://guidelines.india.gov.in/gigw3/), [WCAG 2.2](https://www.w3.org/TR/WCAG22/).
- **Review trigger:** New government accessibility/localization mandate; the higher applicable requirement wins.

### DEC-015 — Consume authoritative hazard products; no baseline custom model

- **Context:** The transcripts proposed custom Random Forest/XGBoost, raw InSAR, hydrological and seismic formulas. Proper calibration requires validated inventories, expertise, uncertainty analysis, and governance. Existing government products are available but coverage differs.
- **Decision:** Ingest versioned authoritative products and record coverage/limits. Derived overlays estimate exposure only. No baseline custom authoritative hazard model, raw InSAR processing, or invented seismic radius.
- **Why:** The platform's differentiator is turning evidence into lawful, participatory relocation decisions, not duplicating hazard agencies with an unvalidated model.
- **Rejected:** Train overnight; treat C-FLOOD as nationwide; use local slope/Fs alone; social-media signals as hazard evidence.
- **Consequences:** Some geographies will show data gaps. New hazard methods require separate scientific assurance and authority-state integration.
- **Evidence:** [KSDMA hazard maps](https://sdma.kerala.gov.in/hazard-maps/), [C-FLOOD release](https://pib.gov.in/PressReleasePage.aspx?PRID=2149301), GSI Wayanad findings.
- **Review trigger:** An authority supplies a validated model/product and approves its operational use.

### DEC-016 — Configurable SLA escalation without authority bypass

- **Context:** CAG evidence shows institutional capacity and meeting gaps, but the Disaster Management Act does not define the transcript's universal six-meetings-in-three-years threshold.
- **Decision:** Treat readiness as operational context. Configure task SLAs and escalation recipients from programme authority; recommend escalation when overdue without changing risk, approval, or jurisdiction automatically.
- **Why:** This surfaces inaction without inventing law or allowing software to route around accountable institutions.
- **Rejected:** Governance multiplier in the risk score; automatic DDMA-to-SDMA/NDMA bypass; hard-coded meeting count.
- **Consequences:** Programme owners must maintain SLA/authority configuration; escalations are auditable recommendations.
- **Evidence:** [Disaster Management Act, 2005](https://www.mha.gov.in/sites/default/files/2022-09/The%20Disaster%20Management%20Act,%202005%5B1%5D.pdf); CAG Report 8 of 2025 on Karnataka disaster management.
- **Review trigger:** A binding rule creates a specific cadence or digital escalation mechanism.

### DEC-017 — India-resident restricted data and defense-in-depth access

- **Context:** The platform handles personal, land/legal, and potentially fine geospatial data. India has DPDP and geospatial obligations, and PostgreSQL owners/superusers can bypass row security.
- **Decision:** Keep applicable personal/fine geospatial data and backups in approved India-resident infrastructure; combine OIDC/SAML, MFA, role/programme/geography/classification authorization, database RLS, encryption, key separation, and audited break-glass.
- **Why:** No single control safely contains insider, configuration, export, and application defects. Residency and official-boundary constraints must be enforced at architecture and operations levels.
- **Rejected:** Application-only RBAC; public object URLs; shared accounts; superuser application connection; overseas support copies.
- **Consequences:** Hosting/support/vendor selection and observability are constrained; security testing must include RLS and export paths.
- **Evidence:** [DPDP Rules 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa), [Indian geospatial guidelines](https://dst.gov.in/sites/default/files/Final%20Approved%20Guidelines%20on%20Geospatial%20Data.pdf), [PostgreSQL RLS](https://www.postgresql.org/docs/current/ddl-rowsecurity.html).
- **Review trigger:** Applicable legal commencement/guidance or government hosting policy changes.

### DEC-018 — Township and approved self-relocation pathways

- **Context:** State-built townships may offer coordinated services but can delay construction or separate households from livelihood. Kerala's vulnerability-linked approach preserves more household choice in applicable programmes.
- **Decision:** Model township/site allocation and approved self-relocation/cash-assistance as distinct programme pathways with their own eligibility, evidence, milestones, and outcomes. Households can express pathway/site preferences where the programme permits.
- **Why:** A single colony-allocation model cannot represent citizen choice or the operational differences between construction and assisted self-relocation.
- **Rejected:** Township-only; software automatically switches a household to cash assistance; one shared status model that hides pathway differences.
- **Consequences:** Reports, policy, budget, monitoring, consent, and remedy must distinguish pathways.
- **Evidence:** [Kerala Vulnerability Linked Relocation Scheme](https://sdma.kerala.gov.in/vulnerability-linked-relocation-scheme/), Kerala PDNA.
- **Review trigger:** A programme authorizes a new pathway or removes household choice.

### DEC-019 — Version changing Wayanad facts; do not hard-code them

- **Context:** PDNA, beneficiary lists, site plans, house counts, completion targets, and scheme amounts changed across dated sources. Search results and news can lag official orders.
- **Decision:** Treat each figure as a source-bound dated fact, prefer current primary authority for operational use, preserve superseded versions, and label secondary reports.
- **Why:** Hard-coded counts become false and make historical decisions irreproducible. Versioning supports correction without pretending the earlier report never existed.
- **Rejected:** One “current truth” row; transcript numbers as constants; silently replacing reports with later news.
- **Consequences:** Dashboards display “as of” dates and source links; integrations need update review.
- **Evidence:** Kerala PDNA, KSDMA orders, Wayanad township publications, dated 2025–2026 implementation reporting.
- **Review trigger:** Never retired; this remains a standing data-governance decision.

### DEC-020 — No LLM or voice functionality in authoritative paths

- **Context:** The transcript proposed an Indic voice assistant and local LLM/RAG. Relocation records are sensitive and authoritative answers require deterministic evidence and current policy.
- **Decision:** Baseline decisions, explanations, and reports use structured rules, calculations, and templates. No LLM or voice agent is required or permitted in an authoritative path.
- **Why:** Determinism, evidence binding, accessibility, privacy, and contestability matter more than conversational novelty.
- **Rejected:** Sarvam voice agent; Qwen-based dispatcher; fine-tuned news classifier; LLM-written official dossier.
- **Consequences:** Natural-language convenience is deferred. Any future assistant requires a new risk/decision record and cannot approve or invent facts.
- **Evidence:** Product safety boundary and DPDP/data-minimization obligations.
- **Review trigger:** A bounded non-authoritative use case passes privacy, accuracy, sourcing, accessibility, and human-review assurance.

### DEC-021 — Conservative production SLO and recovery baseline

- **Context:** This is important government planning software but not the tactical life-safety command system removed by DEC-001. It still requires dependable case access and defensible recovery.
- **Decision:** Start with 99.5% monthly availability, RPO ≤15 minutes, RTO ≤4 hours, p95 standard reads ≤1 second, and asynchronous heavy work, verified against the TRD reference load.
- **Why:** These targets are meaningful and testable without paying for emergency-command availability. They can be tightened after measured usage and criticality.
- **Rejected:** Undefined “high availability”; 99.99% emergency SLO; best-effort backups; synchronous heavy GIS work.
- **Consequences:** Multi-failure-domain deployment, point-in-time recovery, quarterly restore tests, job queues, and observability are required.
- **Evidence:** Product operating model and phased controlled deployment.
- **Review trigger:** Measured user need, statutory service level, recovery exercise, or criticality changes.

### DEC-022 — Accessible HTML plus evidence-bound multi-format exports

- **Context:** Government review uses PDFs/documents, GIS teams need machine-readable data, and households need accessible explanations. PDF alone is difficult to search, reuse, and make fully accessible.
- **Decision:** Use accessible HTML as the canonical review presentation, controlled PDF/document derivatives for official workflows, CSV/JSON/GeoJSON/GeoPackage as applicable, and an evidence/version/checksum manifest.
- **Why:** Multiple audiences receive usable formats while one pinned structured record remains the source. Manifests support reproducibility and tamper detection.
- **Rejected:** PDF-only; dashboard-only; free-form generated prose; exports without source/policy versions.
- **Consequences:** Template/version governance and cross-format consistency/accessibility tests are required.
- **Evidence:** GIGW/WCAG, Kerala LSG templates, OGC standards.
- **Review trigger:** A binding government records/interchange standard prescribes another canonical format.

### DEC-023 — Retain every mathematical family in a governed equation registry

- **Context:** The first documentation baseline kept MCDA, water, exposure and MILP concepts in prose but omitted the explicit mathematics. Some source equations are useful, some optional, some specialist-only and some unsafe.
- **Decision:** Preserve every relevant mathematical family as E01–E60 in [equations.md](./equations.md), classified `CORE`, `OPTIONAL`, `SPECIALIST/EXTERNAL`, or `REJECTED`. Activation requires the complete formula/parameter contract and tests; inclusion alone is never approval.
- **Why:** Silent deletion loses useful implementation detail and provenance; blind restoration would reintroduce invalid scores, arbitrary constants and unvalidated hazard models.
- **Rejected:** No equations; all transcript equations in production; hiding rejected formulas.
- **Consequences:** Formula and parameter versions are first-class records. Specialist equations normally enter as qualified external products. Rejected originals remain non-executable history with replacements.
- **Evidence:** `EQUATIONS.md`, `ORIGINAL_MATH_EXTRACT.txt`, and the numerical defects recorded in E57–E59.
- **Review trigger:** A formula gains/loses validation, scope or authority.

### DEC-024 — Allocation objective and tie policy require separate programme approval

- **Status:** PROPOSED.
- **Context:** MILP defines feasibility but not whose needs take precedence. Urgency, household/person coverage, preference, cost and fairness can conflict.
- **Decision:** Prefer a lexicographic, explicitly approved objective sequence over one giant weighted objective; leave the exact order, coverage unit, fairness constraints and tie-break open in ODN-006/007.
- **Why:** Choosing these values is distributive policy, not an engineering default.
- **Rejected:** Hidden giant weights; database row order; claiming the current equation example is approved policy.
- **Consequences:** Only labelled research scenarios may run until approval; production cannot activate an objective implicitly.
- **Evidence:** E25 and [open-decisions.md](./open-decisions.md).
- **Review trigger:** Programme authority records a decision following participation and fairness review.

### DEC-025 — Draft scenarios do not reserve; approval atomically reserves capacity

- **Context:** Multiple scenarios, programmes, overlapping sites and shared services can consume the same apparent capacity.
- **Decision:** Drafts are simulations. At approval, atomically revalidate all pinned inputs and reserve compatible resource quantities in a transactional ledger; expiry, release and commitment are explicit states.
- **Why:** Per-scenario subtraction permits double allocation and can make two individually feasible proposals jointly impossible.
- **Rejected:** Reserve while drafting; eventual best-effort reconciliation; per-site checks that ignore shared resources/overlapping parcels.
- **Consequences:** Approval can fail because evidence or capacity changed and must create a replan task. Manual proposals use the same validator.
- **Evidence:** E14, E19–E26 and AT-15.
- **Review trigger:** A mandated external allocation ledger provides equivalent atomic guarantees.

### DEC-026 — Add necessity, scheme/funding, and completion/handoff domains

- **Context:** Exposure does not establish that relocation is the right intervention; scheme eligibility is not relocation need; approval is not completed rehabilitation.
- **Decision:** Add competent relocation-versus-mitigation review, effective-dated scheme/entitlement/funding records, and post-approval readiness/handover/occupation/follow-up tracking or an explicit accountable-system handoff.
- **Why:** Without these, the product can optimize an unactionable or unnecessary proposal and report administrative progress as human recovery.
- **Rejected:** Treat exposure as automatic relocation; assume one universal cash scheme; stop at allocation approval without naming downstream ownership.
- **Consequences:** Programme/finance/delivery owners and data integrations are required; additional states and privacy controls are introduced.
- **Evidence:** Kerala vulnerability-linked scheme and MHA funding material in [source-register.md](./source-register.md); Astra audit §§2.3–2.5.
- **Review trigger:** Product scope is formally narrowed to pre-implementation planning with an approved external handoff contract.

### DEC-027 — Use transactional outbox and independent feasibility validation

- **Context:** A database commit followed by queue publish can lose work; a solver can return feasible-not-optimal output or floating-point-tolerant violations.
- **Decision:** Use a transactional outbox/equivalent durable DB-to-queue pattern with idempotent consumers and reconciliation. Validate every solver or manual proposal independently against integer/domain constraints before review or reservation.
- **Why:** Queue retries alone do not close the dual-write gap, and binary declarations do not make all computations decimal-exact.
- **Rejected:** Direct commit-then-publish; trusting solver status without independent checks; rounding fractional solutions.
- **Consequences:** Outbox/consumer state, solver diagnostics, tolerances and recovery become testable architecture elements.
- **Evidence:** Celery retry/idempotence guidance, SciPy MILP floating-point contract, E26, AT-16/25.
- **Review trigger:** A different platform supplies demonstrably equivalent atomic delivery and validation guarantees.

### DEC-028 — Separate consent purposes, state axes, retention, and compliance applicability

- **Context:** One consent or status field cannot represent lawful basis, participation, site acceptance, legal acts, evidence maturity or delivery. Audit preservation can conflict with lawful deletion.
- **Decision:** Use purpose-specific participation records; independent evidence/administrative/legal/notice/reservation/delivery states; record-class retention/legal holds; and an effective-dated compliance register.
- **Why:** Collapsing these concepts creates false consent, false official state and indefinite-retention risk.
- **Rejected:** One consent checkbox; `APPROVED/NOTIFIED` as a universal state; “audit means keep everything”; applying every DPDP rule before its commencement.
- **Consequences:** Workflows and deletion/restore tests are more explicit; a minimal lawful deletion event may remain without preserving deleted personal content.
- **Evidence:** Amended disaster law, DPDP rules, CERT-In directions and RUL-044/RUL-050.
- **Review trigger:** Effective law, records schedule or programme process changes.

### DEC-029 — Use prototype, authorized-pilot, and controlled-production deployment profiles

- **Context:** The approved end state is production-oriented, while early validation benefits from a small demonstrator. Requiring full orchestration before learning would add cost without creating authority.
- **Decision:** Keep one architecture with three deployment profiles. A local/container prototype uses synthetic/public data; authorized pilot and production progressively require hardened identity, operations, residency, resilience and assurance. Kubernetes/OpenShift is not mandatory for the prototype.
- **Why:** This preserves production constraints without presenting a hackathon build as production or making production infrastructure a prerequisite to test the core flow.
- **Rejected:** Prototype promoted unchanged; mandatory Kubernetes for every environment; no prototype lane.
- **Consequences:** Environment labels and data controls are strict. Staffing, hosting, procurement and support are explicit phase gates.
- **Evidence:** Approved plan, delivery-risk review and [phases.md](./phases.md).
- **Review trigger:** Hosting/procurement decision or changed delivery objective.

### DEC-030 — Pin standards, basemap and specialist-model activation separately

- **Status:** PROPOSED.
- **Context:** “STAC 1.0/1.1-compatible,” MapLibre alone, and generic specialist-model names are insufficient conformance or licensing decisions.
- **Decision:** Select one STAC/OGC conformance profile per release, a licensed/attributed/offline-compatible basemap, and separately approved specialist model packages/calibrations only when needed.
- **Why:** A renderer is not map data; interoperability requires testable versions; scientific equations are not validated implementations.
- **Rejected:** Ambiguous multi-version claims; default public tile service; activating reference equations through ordinary configuration.
- **Consequences:** ODN-013/014/019 block affected capabilities until resolved.
- **Evidence:** OGC/STAC sources and E37–E54.
- **Review trigger:** Architecture/procurement/domain owners approve concrete profiles.

### DEC-031 — Separate source products, access services, processing, display, and operational readiness

- **Context:** The supplied 54-row inventory mixes data products, catalogs, download endpoints, processing platforms, display basemaps, agency-record routes, and field acquisition. Several platforms expose the same Sentinel/Landsat observation, and provider documentation was checked without proving authenticated download or entitlement.
- **Decision:** Preserve S01–S54 as the operational source index; model product, catalog, download/receipt, processing, and display capabilities separately; group mirrors/shared inputs; and require a permitted AOI acquisition/validation gate before any source version is decision-usable.
- **Why:** A catalog hit is not a licensed file, a processing service is not a new observation, and a basemap is not evidence. Separating these states prevents false integration/readiness claims, double-counting, and silent use outside license or coverage.
- **Rejected:** Treating all 54 rows as connected APIs; calling documentation review an end-to-end test; counting the same satellite scene from several catalogs as corroboration; letting display tiles enter analytical lineage.
- **Consequences:** Source readiness has its own states, owner, health, entitlement/quota/cost metadata, fallback, and release report. Connector implementation is larger but auditable and replaceable. CDSE is the new Copernicus baseline; SciHub is not a new dependency. Earth Engine remains optional and terms-dependent; SoilGrids REST is not assumed available; Bhoonidhi and NWDP use documented resource-specific routes; CHIRPS v3 is preferred for new integration.
- **Evidence:** Supplied `DATA_SOURCE_GUIDE.md`, identical CSV/JSON/XLSX 54-row matrix, provider references indexed in [source-register.md](./source-register.md), RUL-076–083, and FR-076–084.
- **Review trigger:** A provider materially changes product identity, API, access terms, coverage, pricing/quota, license, or deprecates a route.

### DEC-032 — Build with public/synthetic data first; block real decisions on agency and field evidence

- **Context:** The inventory shows adequate public evidence for mapping, exposure, terrain/access context, and a synthetic-household workflow, but it does not contain authoritative parcel/right records, verified household participation, site geotechnics, lean-season water proof, or programme funding/readiness records.
- **Decision:** Implement the bounded Wayanad public-data sandbox first using the minimum stack in source-register §7. Keep real site approval, household allocation, handover, and completion disabled until the applicable S45–S50 records and qualified reviews pass the acquisition and domain gates.
- **Why:** More imagery cannot establish title, consent, soil strength, sustainable water yield, or sanctioned funding. This sequencing permits useful engineering progress while ensuring unavailable mandatory evidence remains `UNKNOWN`/`HOLD`.
- **Rejected:** Waiting for every government integration before building; using open-data proxies as official records; mixing synthetic and real households; enabling production actions with warnings around missing mandatory evidence.
- **Consequences:** PH-0/PH-1 can build and validate ingestion, exposure, discrepancy, gates, scenarios, and reports. Production transition depends on formal agency/field acquisition, owner review, privacy controls, and the source-readiness report.
- **Evidence:** Source inventory S01–S54, especially S45–S50; [source-register.md](./source-register.md#6-source-activation-and-acquisition-gate); RUL-079–080; FR-082–083.
- **Review trigger:** All relevant agency and field dependencies are acquired and independently accepted for a bounded live workflow, or the pilot scope changes.

### DEC-033 — Phase 0 research findings and closure of baseline operational decisions

- **Context:** `open-decisions.md` and `phases.md` left several empirical questions open regarding statutory cadence, Wayanad township acquisition routes, Kerala VLRS scheme caps, C-FLOOD basin boundaries, GSI hazard coverage, DPDP timeline, and empirical equation derivations.
- **Decision:** Formally close baseline analytical ambiguities using primary verified facts:
  1. Under the Disaster Management (Amendment) Act 2025 (commenced 9 April 2025), §31(4) mandates District Disaster Management Plan updates every 2 years or earlier as necessary. Algorithmic outputs are advisory; statutory power remains with DDMA/SDMA.
  2. The Wayanad 2024 rehabilitation township model is anchored at Elstone Estate (Kalpetta Municipality) for ~430 disaster-affected families on ~7 cents/unit, acquired under Section 65 of the Disaster Management Act 2005.
  3. The Kerala Vulnerability Linked Relocation Scheme (G.O. Ms 6/2018/DMD et seq.) establishes an individual relocation precedent of ₹10 Lakh (₹6L for ≥3 cents safe land + ₹4L for house construction), preserving agricultural ownership while barring residential construction.
  4. C-FLOOD operational coverage is confirmed to cover Godavari, Tapi, and Mahanadi basins only; it has zero coverage in Wayanad and is locked out of Wayanad gates (RUL-017).
  5. The empirical GLOF formula $Q_{peak}=0.00077 V^{1.017}$ originates from Huggel et al. (2002) for alpine glacial outbursts; it is verified as non-applicable to Wayanad and rejected from executable code (E52).
  6. DPDP Rules 2025 were notified on 14 November 2025 with phased enforcement, mandating strict data minimization and India-resident infrastructure.
- **Why:** Replaces speculative software defaults with primary statutory and empirical reality.
- **Rejected:** Waiting indefinitely for administrative data agreements; using unverified formulas or assuming C-FLOOD coverage.
- **Consequences:** Closes ODN-001–003, ODN-010, ODN-017, ODN-018, and ODN-021 for baseline implementation.
- **Evidence:** PIB PRID 2146781; Gazette Notification 8 April 2025; KSDMA G.O. (Ms) 6/2018/DMD; CWC C-FLOOD documentation; GSI NLSM 2022 dataset; MeitY DPDP Rules notification.
- **Review trigger:** State-level amendment to Kerala VLRS caps or notification of new Wayanad land orders.

### DEC-034 — PKG-0C shared domain contracts and Ponytail modular architecture

- **Context:** `phases.md` mandates completing PKG-0C before parallel feature coding to freeze domain boundaries ARC-C01–13, state enums, API errors, bitemporal envelopes, and outbox event schemas.
- **Decision:** Implement PKG-0C and Phase 1 platform using Python FastAPI modular monolith with Pydantic v2 domain schemas, enforcing the Ponytail ladder (`YAGNI -> stdlib -> native -> one line -> minimum`). Dual-target storage architecture uses PostgreSQL/PostGIS for production and an in-memory/SQLite GeoJSON spatial engine for zero-dependency local testing. Audit events use append-only SHA-256 hash chaining.
- **Why:** Guarantees absolute contract consistency across domain modules, eliminates bloat, and allows fully deterministic automated verification without external database infrastructure.
- **Rejected:** Premature microservices; raw untyped dictionaries; third-party blockchain audit systems; mutable state rewrites.
- **Consequences:** All domain packages import from `punarvas.core`. Database changes must preserve bitemporal valid/system times and hash audit continuity.
- **Evidence:** `architecture.md` §5 & §6; `trd.md` §3; `rules.md` RUL-001–083.
- **Review trigger:** Introduction of external distributed event brokers or multi-region database replication.

### DEC-035 — Phase 2 Wayanad Shadow Pilot, Evaluation Protocols, and Rehearsal Architecture

- **Status:** ACCEPTED.
- **Context:** `phases.md` §5 & §12.5 define Phase 2 (`PH-2`) as a 12–16 week shadow pilot and field validation exercise operating strictly without official decision reliance. Operational decisions ODN-004 through ODN-009, ODN-016, and ODN-022 require explicit boundaries for household criteria, site multi-criteria calibration, allocation solver objective hierarchy, tie-breaking, reservation locking, offline survey conflict resolution, and Kerala LSGD DM Plan annex integration.
- **Decision:** Establish the Phase 2 shadow pilot and rehearsal architecture:
  1. *Operating Mode & Advisory Envelope:* All Phase 2 workflows, reports, and exports are tagged with `AuthorityState.ANALYTICAL` or `SHADOW` markers. Official decisions remain with DDMA/SDMA.
  2. *Preregistered Evaluation & Metric Tracking (R2-03):* Implement evaluation protocol comparing shadow dossiers to official human baselines across cycle time (targeting 30% reduction), false positive/negative rates, unknown rates, subgroup fairness (disability, female-headed, elderly), and rank sensitivity.
  3. *Agency Import & Reconciliation Adapters (C2-01):* Build strict adapters for Land Revenue (Bhulekh/ULPIN), Forest Department (FRA 2006 claims), and Disaster Management orders. Surface cadastral offset errors and version conflicts without silent auto-reconciliation.
  4. *Offline Field Survey & Device Revocation (C2-02 / FEAT-008):* Support mobile package export, localized sync with conflict detection (last-write-wins rejected in favor of explicit manual conflict review), and cryptographic token revocation for lost/stolen field devices.
  5. *Sensitivity Analysis & Capacity Reservation Ledger (C2-03 / FEAT-014/024):* Implement AHP weight perturbation ($w \pm 20\%$) and rank reversal detection. Provide atomic capacity reservation ledger with concurrent race condition detection and fail-closed locking for dwelling, land, budget, and water resources across competing scenarios.
  6. *Kerala LSGD Annex Generation & Delivery Tracking (C2-04 / FEAT-018/023/025):* Generate Annexures for Kerala Local Self Government Disaster Management Plans preserving unverified participatory fields as incomplete. Model delivery milestones (sanction, construction, service readiness, handover, occupation).
  7. *11 Mandatory PH-2 Scenarios Rehearsal (C2-05 / I2-01):* Build an end-to-end rehearsal test harness validating all 11 required scenarios in `phases.md` §5.
- **Why:** Prevents unvalidated automated decision making, safeguards affected citizens' rights, ensures algorithmic accountability, and satisfies all 83 normative rules.
- **Rejected:** Silent automated cadastral adjustments; live capacity booking from draft scenarios; single composite ranking score; replacing participatory LSG meetings with database-generated text.
- **Consequences:** Prepares system for controlled live deployment (Phase 3) while keeping production boundary protected.
- **Evidence:** `phases.md` §5 & §12.5, `rules.md` RUL-001–083, `trd.md` FR-042–075, Disaster Management Act 2005 §31/§65, Kerala G.O. (Ms) 6/2018/DMD.
- **Review trigger:** Formal evaluation board review at Phase 2 exit gate before Phase 3 authorization.

### DEC-036 — Kerala Multi-District Scaling, Policy Inheritance, and Row-Level Isolation (PH-4)

- **Status:** ACCEPTED.
- **Context:** `phases.md` §7 & §12.7 mandate scaling across Kerala districts (e.g., Idukki, Alappuzha, Malappuram) through repeatable configuration rather than shared unrestricted access. Cross-district data or policy assumptions (e.g. Wayanad debris flow parameters applied to Alappuzha coastal backwaters) must never leak.
- **Decision:**
  1. Implement `DistrictProfile` and `DistrictOnboardingService` providing dynamic configuration for each district's unique hazard profile, authority names, local road/water standards, and active schemes.
  2. Implement policy inheritance with strict local override rules: State base policy defines mandatory governance gates, while district-specific hazard thresholds (e.g. steep mountain tea slopes vs coastal inundation) override baseline without polluting sibling districts.
  3. Enforce strict row-level and geography isolation (RUL-054): DDMA officers in District A cannot query or modify casework in District B.
  4. Provide state-level oversight (`KSDMA`) through de-identified, privacy-safe aggregates only (RUL-052).
- **Why:** Prevents dangerous geographic policy misapplication and maintains constitutional and statutory district autonomy under the Disaster Management Act 2005.
- **Rejected:** Hardcoding Wayanad parameters as statewide defaults; unrestricted multi-district user access; raw database sharing between DDMAs.
- **Consequences:** Each district requires an onboarding manifest and independent validation pack.
- **Evidence:** `phases.md` §7, `rules.md` RUL-017, RUL-018, RUL-054, Disaster Management Act 2005 §30/§31.
- **Review trigger:** Onboarding of new Kerala districts or changes in state disaster management policy.

### DEC-037 — Multi-State Tenant Adaptation and Cross-State Leakage Prevention (PH-5)

- **Status:** ACCEPTED.
- **Context:** `phases.md` §8 & §12.8 mandate platform reusability across Indian states (pilot reference: Uttarakhand) while keeping state-specific legal frameworks, land tenure terminology, languages, and authority explicit.
- **Decision:**
  1. Implement `StateTenantPackage` and `MultiStateAdapter` architecture decoupling the core decision engine from state-specific implementations.
  2. For Uttarakhand reference adaptation: bind to USDMA / DDMA Chamoli, Devbhoomi Bhulekh land tenure (Khasra/Khatauni vs Kerala Thandaper), GLOF/cloudburst hazard classifications, and Hindi language localization.
  3. Enforce cryptographic tenant isolation: Zero leakage of Kerala G.O.s (such as VLRS ₹10L or Meppadi township terms) or Malayalam language strings into Uttarakhand dossiers, and vice-versa.
  4. Ensure coordinate reference systems (CRS) decouple cleanly per state (e.g., UTM Zone 44N EPSG:32644 for Uttarakhand vs UTM Zone 43N EPSG:32643 for Kerala).
- **Why:** Preserves the sovereign federal structure of Indian disaster management law and prevents jurisdictional invalidity.
- **Rejected:** Treating Kerala land revenue rules as national standards; mono-lingual English/Malayalam lock-in; shared multi-tenant database without strict tenant boundaries.
- **Consequences:** New state onboarding requires dedicated localization and land tenure adapter packages.
- **Evidence:** `phases.md` §8 & §12.8, Disaster Management Act 2005 §14/§22, National Disaster Management Plan (NDMP), MeitY GIGW 3.0.
- **Review trigger:** Notification of new State Disaster Management Rules or expansion to a third state.

### DEC-038 — National NDMA Sovereign Relocation Clearinghouse and Inter-State Federation (Phase 6)

- **Status:** ACCEPTED.
- **Context:** While Kerala (Wayanad/Idukki) and Uttarakhand (Joshimath/Chamoli) operate under their respective SDMAs, severe disaster events frequently traverse state boundaries (e.g., Western Ghats debris corridors across Kerala/Tamil Nadu/Karnataka, or Upper Ganga/Himalayan glacial lake outburst corridors). National Disaster Management Authority (NDMA) requires nationwide situational awareness, inter-state mutual-aid coordination, and NDRF allocation tracking without usurping state constitutional data sovereignty.
- **Decision:**
  1. Implement `NationalClearinghouseService` establishing a federated clearinghouse between SDMAs and NDMA under Disaster Management Act 2005 §3 & §6.
  2. Model cross-border hazard corridors (e.g. `CORR-WG-01` for Western Ghats Nilgiri-Wayanad, `CORR-HIM-02` for Upper Ganga Glacial & Subsidence Corridor).
  3. Support inter-state relocation and mutual-aid requests (`InterStateRelocationRequest`) with explicit role verification (`RoleType.GOVERNMENT_APPROVER`), preventing unauthorized claims.
  4. Enforce State Data Sovereignty: SDMAs retain exclusive custody of restricted personal, beneficiary, and parcel records. Only de-identified macro totals and cryptographically signed audit manifests (`NationalRegistryManifest`) are federated to NDMA.
  5. Provide trilingual localization parity across English, Malayalam, and Hindi (`RUL-055`, GIGW 3.0).
- **Why:** Delivers nationwide disaster relocation clearinghouse capability while upholding federalism, statutory state rights, and data minimization under DPDP Rules 2025.
- **Rejected:** Centralized national database holding individual household PII; bypassing SDMA statutory approval; unverified verbal inter-state mutual aid.
- **Consequences:** Inter-state coordination requires formal digital requests and state-level cryptographic manifest federation.
- **Evidence:** Disaster Management Act 2005 §3, §6, §14; National Disaster Management Plan (NDMP); `rules.md` RUL-001, RUL-002, RUL-050–058.
### DEC-039 — Controlled Live Wayanad Operations, Recovery Harness, and Delivery Completion Verification (PH-3)

- **Status:** ACCEPTED.
- **Context:** `phases.md` §6 & §12.6 define Phase 3 (`PH-3`) as a bounded live Wayanad deployment maintaining human statutory authority, multi-layer operational controls, tamper-evident auditability, automated disaster recovery verification, and post-approval delivery tracking.
- **Decision:**
  1. *Step-Up MFA Authentication (AT-28 / RUL-054):* Enforce cryptographically signed step-up tokens with time-bounded validity (default 300s) for privileged actions: `APPROVE_DECISION`, `PUBLISH_PROJECTION`, `EXPORT_RESTRICTED_DATA`, `ACTIVATE_POLICY`, and `BREAK_GLASS`.
  2. *Audited Break-Glass Emergency Bypass (NFR-013 / RUL-047):* Support time-bounded emergency sessions (max 60 minutes) requiring explicit substantive justification, category tag, and approving authority reference, automatically broadcasting high-priority tamper-evident audit alerts.
  3. *Coordinated Disaster Recovery Verification (NFR-032 / AT-27):* Verify relational database snapshot hashes, object store inventories, and append-only SHA-256 audit ledger atomically. Any missing referenced evidence object or checkpoint mismatch immediately fails restore and blocks authoritative writes.
  4. *Read-Only Degraded Mode Controller (NFR-006):* Implement circuit breaker allowing read-only access to approved historical records while blocking mutations and new allocations during broker or connectivity outages.
  5. *Manual Continuity Reconciler:* Support parallel paper-based/offline statutory decisions during communication disruptions, reconciling them into the system with bitemporal valid time and offline notice references.
  6. *Post-Approval Physical Delivery Tracker (RUL-072 / AT-22):* Enforce 8-stage physical milestone progression (`FUNDING_SANCTIONED` &rarr; `UNIT_CONSTRUCTED` &rarr; `SERVICES_FUNCTIONAL` &rarr; `DEFECTS_CLEARED` &rarr; `BENEFICIARY_ACCEPTED` &rarr; `POSSESSION_HANDED_OVER` &rarr; `OCCUPIED` &rarr; `FOLLOW_UP_COMPLETED`). Approval is NEVER counted as completed relocation! Unresolved defects strictly block handover.
  7. *Decision Provenance DAG Reconstruction (R3-01 / FEAT-020):* Enable independent auditors to cryptographically reconstruct sampled decisions from raw source checksums, policy versions, and solver seeds.
  8. *Pre-Publication Disclosure Review (FEAT-019 / RUL-075 / AT-23):* Automate threat-model verification before public dossier release: enforce $k$-anonymity ($k \ge 5$), suppress small cells, detect differencing attacks between publication rounds, and generalize coordinates.
  9. *Controlled Live Rollback:* Halt live operations and revoke active projections while strictly preserving read-only audit records and lawful statutory decisions.
- **Why:** Protects vulnerable disaster-affected citizens, prevents unevidenced administrative claims, and ensures business continuity during extreme natural emergencies.
- **Rejected:** Treating administrative approval as completed relocation; unrestricted emergency access without audit alerts; unverified disaster recovery restarts.
- **Evidence:** `phases.md` §6 & §12.6, `rules.md` RUL-001–083, `trd.md` NFR-006, NFR-013, NFR-032, AT-22, AT-23, AT-27, AT-28.
- **Review trigger:** Completion of six-month controlled live deployment window before Kerala scaling expansion.

## 4. Explicit research corrections adopted


| Transcript claim/implication | Baseline correction | Decisions/rules |
| --- | --- | --- |
| Six DDMA meetings in three years is a universal statutory threshold | The Act says meetings occur as necessary; use authorized configurable SLAs only | DEC-016; RUL-006 |
| `Risk = H×E×V/C` is a prescribed computational model | It is conceptual framing; operational models require validation and policy approval | DEC-006; RUL-034–037 |
| C-FLOOD can supply Wayanad flood forecasts | Initial documented C-FLOOD coverage is Godavari, Tapi, and Mahanadi; enforce source coverage | DEC-015; RUL-017 |
| ULPIN/digitized maps establish clear title | They identify/represent parcels; title, geometry, claims, and possession need reconciliation | DEC-004/009; RUL-021–027 |
| Positive NDBI proves occupation and requires rejection | It is only a discrepancy signal and can confuse built-up/bare surfaces | DEC-006/015; RUL-022 |
| Every forest/FRA-related site has one blanket Gram Sabha rule | Applicable FRA/forest/PESA consultation and clearance is location/action-specific | DEC-005; RUL-025 |
| 55 LPCD proves water capacity | It is a baseline service level; sustainable yield, quality, reliability, and delivery need evidence | DEC-006; RUL-030–031 |
| Private land always means 24 months and 4× compensation | Route and entitlements depend on applicable law, location, process, and order | DEC-005; RUL-027 |
| Building footprints can enumerate beneficiaries | They can support exposure estimates only; household verification is separate | DEC-004/010; RUL-019 |
| Public household scoring is required for transparency | Publish methods/aggregates; protect household evidence and provide private explanations/remedy | DEC-010; RUL-052 |
| An optimized match is an allocation | It is an advisory scenario requiring consent, process, review, and approval | DEC-007; RUL-040–043 |
| Hash logging makes a list immutable | It makes unauthorized modification detectable if custody/checkpoints are sound | DEC-012; RUL-056–058 |
| District Plans are reviewed annually | Since 9 April 2025, amended §31(4) requires review/update at least once every two years or earlier as necessary | DEC-028; FR-075 |
| 10–12 m²/person is total settlement or grazing capacity | It is cited as recommended urban open space; land capacity needs an approved layout and grazing needs livestock/feed evidence | DEC-023; E13/E15/E57 |
| Original PPM is already a 0–100 score | Its normalized weighted sum is 0–1 without ×100 and still mixes urgency with destination shortage | DEC-006/023; E57 |
| AHP consistency or PCA variance validates policy fairness | These are diagnostics/methods; policy validity needs participation, lawful criteria, subgroup and sensitivity evidence | DEC-006/023; E28–E30 |
| SCS runoff depth is an inundation map | Event runoff depth still requires routing/hydraulics and terrain to estimate inundation | DEC-015/023; E47–E49 |
| Approval means relocation is complete | Funding, functioning services, acceptance, possession, occupation, defects and follow-up are separate | DEC-026; RUL-072 |
| Every source row is a connected API or independent measurement | S01–S54 mix products, catalogs, processors, agency routes and field routes; keep capability/readiness and mirror groups explicit | DEC-031; RUL-076–078 |
| Copernicus SciHub remains the integration route | SciHub ceased operations; use CDSE with separate catalog, download and processing adapters | DEC-031; RUL-081 |
| Earth Engine is unrestricted/free for government operations | Use requires approved category, project authentication, quotas/cost and privacy review | DEC-031; RUL-081 |
| SoilGrids REST, CARTO no-key tiles, or public OSM tiles are dependable defaults | SoilGrids REST is paused; current basemap terms/keys and offline rights must be approved; do not bulk-download public OSM tiles | DEC-030/031; RUL-081–082 |
| Public imagery can complete site approval | Open sources support sandbox screening; S45–S50 agency and field evidence blocks corresponding real decisions | DEC-032; RUL-079 |
| New NISAR data restores raw InSAR to the baseline | NISAR remains optional specialist evidence with product and local validation limits | DEC-015/031; RUL-083 |

## 5. Evidence register

The controlled dated register is [source-register.md](./source-register.md); the links below are a convenience index and must not be read as blanket verification or formula approval.

- [Disaster Management Act, 2005](https://www.mha.gov.in/sites/default/files/2022-09/The%20Disaster%20Management%20Act,%202005%5B1%5D.pdf)
- [Disaster Management (Amendment) Act, 2025](https://prsindia.org/files/bills_acts/acts_parliament/2025/The_Disaster_Management_Act%2C_2025.pdf) and [commencement notice](https://www.pib.gov.in/PressReleasePage.aspx?PRID=2146781)
- [KSDMA Wayanad reports/PDNA](https://sdma.kerala.gov.in/reports-landslides-2024/)
- [KSDMA Wayanad government orders](https://sdma.kerala.gov.in/government-orders-5/)
- [Kerala Vulnerability Linked Relocation Scheme](https://sdma.kerala.gov.in/vulnerability-linked-relocation-scheme/)
- [Kerala Local Self Government DM Plans](https://sdma.kerala.gov.in/local-self-government-dm-plans/)
- [KSDMA hazard maps](https://sdma.kerala.gov.in/hazard-maps/)
- [RFCTLARR resources](https://dolr.gov.in/en/document-category/acts-rules/)
- [DILRMP 3.0 Operational Guidelines](https://dolr.gov.in/en/document/digital-india-land-records-modernization-programmedilrmp-3-0-operational-guidelines-2026-2031/)
- [ULPIN](https://dolr.gov.in/en/ulpin/)
- [Forest Rights Act and Rules](https://tribal.nic.in/FRA/data/FRARulesBook.pdf)
- [UNDRR disaster risk](https://www.undrr.org/terminology/disaster-risk)
- [Jal Jeevan Mission](https://jaljeevanmission.gov.in/about_jjm)
- [Census PCA](https://censusindia.gov.in/nada/index.php/catalog/6191)
- [Google Open Buildings](https://sites.research.google/gr/open-buildings/)
- [DPDP Rules, 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa)
- [Indian geospatial guidelines](https://dst.gov.in/sites/default/files/Final%20Approved%20Guidelines%20on%20Geospatial%20Data.pdf)
- [OGC API Features](https://www.ogc.org/standards/ogcapi-features/)
- [STAC API](https://docs.ogc.org/cs/25-005/25-005.html)
- [PostgreSQL row security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [PostGIS `ST_Transform`](https://postgis.net/docs/ST_Transform.html)
- [MapLibre GL JS](https://maplibre.org/maplibre-gl-js/docs/)
- [GIGW 3.0](https://guidelines.india.gov.in/gigw3/)
- [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
- [CERT-In Directions, 2022](https://www.cert-in.org.in/PDF/CERT-In_Directions_70B_28.04.2022.pdf)
- [Controlled source register](./source-register.md)
