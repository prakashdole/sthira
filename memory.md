---
title: PUNARVAS-AI Durable Project Memory
document_id: PUN-MEMORY
version: 1.2
status: Current context summary
as_of: 2026-09-08
audience: Future maintainers, agents, product owners, architects, reviewers, and delivery leads
owner: Product owner
normative_scope: Compact orientation and update protocol; links to normative source documents
---

# PUNARVAS-AI Durable Project Memory

## 1. Read this first

PUNARVAS-AI is a **production-oriented, government decision-support system for proactive permanent relocation from disaster-prone areas**. The reference pilot is Wayanad, Kerala.

It is not a live emergency-command platform and cannot itself establish title, declare an official hazard zone, select final beneficiaries, acquire land, force relocation, approve public funds, or bypass government authority.

This file is a compact memory, not a duplicate specification. Use:

- [prd.md](./prd.md) for product purpose, users, scope, workflows, outcomes, and exclusions.
- [rules.md](./rules.md) for binding safety, legal, evidence, scoring, privacy, publication, and authority rules.
- [trd.md](./trd.md) for `FR-*`/`NFR-*`, state machines, records, interfaces, and acceptance scenarios.
- [architecture.md](./architecture.md) for components, data/runtime flows, trust boundaries, deployment, and failure design.
- [feature.md](./feature.md) for `FEAT-*` behavior, traceability, acceptance, and phase.
- [phases.md](./phases.md) for `PH-*` entry/exit gates and rollout.
- [decisions.md](./decisions.md) for `DEC-*` rationale, alternatives, consequences, evidence, and review triggers.
- [equations.md](./equations.md) for retained core, optional, specialist, and rejected mathematical families.
- [parameters.md](./parameters.md) for values, units, provenance, approval, and activation state.
- [source-register.md](./source-register.md) for dated evidence and verification limits.
- [open-decisions.md](./open-decisions.md) for unresolved choices that block activation or rollout.

## 2. Source precedence

1. Current explicit user instructions and confirmed choices.
2. Later product-intent statements in `claude.txt` and `idea.txt` over earlier ones.
3. Current primary law, official government data/publications, and authoritative scientific sources over chatbot factual assertions.
4. Transcript code, formulas, constants, and proposed stacks are reference material only.

Never read `idea.txt` from the top as a current specification. It evolves from a dual-horizon command system into a focused relocation platform. Source extracts and chatbot reviews are evidence for reconciliation, not independent authority.

## 3. Current product truth

### In scope

- Versioned source/evidence ingestion and lineage.
- Competent relocation-necessity assessment against feasible risk reduction in place, preserving settlement/community scope.
- Authoritative hazard-layer catalog and settlement exposure estimates with uncertainty.
- Land-truth reconciliation across records, geometry, observed occupation/use, claims, forest/FRA status, and legal review.
- Candidate-site hazard, legal, water, infrastructure, accessibility, livelihood/social, capacity, cost, and field assessment.
- Verified household and scheme eligibility, transparent priority, separately recorded participation/pathway/preference/offer acceptance, accessibility needs, and livelihood.
- Explainable site comparison and sensitivity analysis after hard gates.
- Advisory capacity-constrained allocation scenarios, with unassigned reasons.
- Draft/publication, correction, objection, hearing, appeal/review, approval, supersession, and withdrawal workflows.
- Township and authorized self-relocation/assistance pathways.
- Scheme/entitlement and funding-gap tracking; atomic capacity reservation; delivery/completion or explicit external handoff.
- Accessible government dossiers, Kerala LSG DM-plan annexes, machine-readable exports, manifests, and audit.
- De-identified public transparency and private household status/remedy access.
- Offline field collection and governed file exchange.

### Out of scope

- Live emergency command, evacuation, convoy, airdrop, supply, shelter, or helicopter logistics.
- Voice assistants, general chatbots, LLM decision/report claims, regional-news or social-media scraping.
- Cesium/3D requirement, raw InSAR processing, custom hazard-model training, or invented seismic radii.
- Automatic title/FRA/forest clearance, beneficiary identity from buildings, forced relocation, gazetting, fund release, or authority bypass.
- A single `Ω`, risk, carrying-capacity, or governance score that hides separate decisions.
- Treating C-FLOOD as available outside its documented coverage.
- Treating approval, notification, funding, handover, occupation, or completion as interchangeable.

## 4. Core invariant

Every high-impact output must answer:

1. **What is it?** Hazard interpretation, exposure estimate, parcel discrepancy, site assessment, household eligibility/priority, allocation proposal, objection decision, or official artifact.
2. **Which state is it in?** Draft, analytical, field-verified, technically endorsed, officially approved/notified, superseded, or withdrawn.
3. **What evidence and versions support it?** Source, license/use, dates, checksum, CRS/resolution, method, confidence, lineage, reviewer.
4. **Which policy produced it?** Rules, thresholds, weights, missing-data behavior, sensitivity, solver/template version.
5. **Who had authority to act?** Named user/role, jurisdiction, legal/programme basis, approval and notice.
6. **How can it be corrected or challenged?** Correction, objection, hearing, appeal/review, supersession.

If any answer is missing, the item cannot be represented as official or complete.

## 5. Decision model

- Mandatory gates return `PASS`, `FAIL`, `UNKNOWN`, or `BLOCKED`.
- Unknown is not pass; lack of evidence is not proof of safety, vacancy, title, water, eligibility, or consent.
- Site dimensions remain separate: hazard, geotechnical/developability, legal, water/environment, infrastructure/services, accessibility, livelihood/social, capacity, cost, readiness.
- Household eligibility/priority, site suitability, legal readiness, programme readiness, and allocation preference are separate.
- MCDA is comparative and advisory after gate eligibility. It exposes raw values, units, normalization, evidence, confidence, weights, contribution, policy version, and sensitivity.
- Allocation uses an advisory binary MILP because households are indivisible and sites have dwelling/person/accessibility constraints. Consent and unacceptable options are hard constraints; unassigned is valid.
- Institutional delay creates reminders/escalation recommendations only. It does not change risk or bypass authority.
- Formula families E01–E60 remain documented. Only approved `CORE` versions execute; optional work is labelled, specialist models are external by default, and rejected formulas remain non-executable history.
- Allocation objective order, coverage unit, tie policy, model parameters and solver tolerances remain open until approved; drafts reserve no capacity.

## 6. Architecture baseline

- Next.js/TypeScript bilingual PWA; MapLibre 2D plus accessible non-map flows.
- Python FastAPI modular monolith with explicit domain modules.
- Celery/RabbitMQ asynchronous workers for ingestion, GIS, comparison, allocation, and reports.
- PostgreSQL/PostGIS canonical spatial/transactional record; restricted personal schema and row-level security.
- India-resident S3-compatible versioned object storage for sources/evidence/exports.
- STAC raster/imagery catalog; OGC API Features/GeoJSON, MVT, COG-compatible exchange.
- Source connector plane keeps product, catalog, authenticated download/receipt, processing service, and display basemap separate; S01–S54 readiness is governed in [source-register.md](./source-register.md).
- OIDC/SAML identity broker, MFA, geography/programme/classification authorization.
- Append-only hash-linked audit events and signed/checksummed manifests; not blockchain.
- OCI containers with separate prototype, authorized-pilot and controlled-production profiles; Kubernetes/OpenShift-compatible infrastructure is a production option, not a demonstration prerequisite.
- Transactional outbox, idempotent workers, independent allocation feasibility validator, and atomic multi-resource reservation ledger.
- OpenTelemetry-based logs/metrics/traces and separate security monitoring.
- Baseline production targets: 99.5% monthly availability, RPO ≤15 minutes, RTO ≤4 hours, standard read p95 ≤1 second.

Do not split business modules into microservices without meeting a review trigger in `DEC-008`.

## 7. Wayanad research truths

- The July 2024 failure affected Mundakkai/Chooralmala through stream-channel debris flow; flat local terrain is not sufficient evidence of safety.
- Kerala's PDNA and later programme reports contain dated, changing casualty, housing, beneficiary, site, and construction figures. Preserve them as source-bound versions.
- KSDMA publishes GSI 2022 landslide susceptibility shapefiles, including Wayanad, and says these supersede older NCESS maps.
- C-FLOOD's initially published operational coverage was Godavari, Tapi, and Mahanadi, not Wayanad.
- The supplied source matrix contains 54 datasets, platforms, agency routes, and field routes—not 54 independent observations or connected APIs. CSV, JSON, and XLSX representations agree; documentation-level verification is not operational access.
- Use CDSE rather than retired SciHub for new Copernicus integration. Earth Search and Planetary Computer are alternate delivery routes; Earth Engine is optional and terms/quota dependent; Bhoonidhi and NWDP have documented resource-specific routes.
- Prefer CHIRPS v3 for a new integration. SoilGrids REST is not a dependency while paused; use reviewed WCS/files only for optional soil context. CARTO/MapTiler/public OSM tiles require explicit key/license/offline review.
- NISAR is a specialist optional input; its availability does not restore raw InSAR to baseline scope. GRD/RTC, DEM/DSM, SoilGrids, footprints, and indices cannot supply the field/legal facts they do not measure.
- Kerala's Vulnerability Linked Relocation Scheme provides an important voluntary/self-relocation pathway precedent; current scheme amounts and applicability must come from the effective order/page, not code.
- Kerala has an approved participatory Local Self Government Disaster Management Plan template. PUNARVAS-AI populates evidence/annexes but does not replace Gram/Ward Sabha, working-group, technical, DPC, or DDMA steps.
- The Wayanad pilot begins in shadow mode and may use official outputs only after PH-3 authority gates.

## 8. Legal and evidence corrections

- The Disaster Management Act does not create the transcript's universal “six DDMA meetings in three years” rule.
- Under Disaster Management Act §31(4), as amended in 2025 and commenced 9 April 2025, the District Plan is reviewed and updated at least once every two years or earlier as necessary. This interval is not generalized to every plan or internal review; DDMA meeting/escalation rules come from current applicable authority.
- RFCTLARR applies through a defined legal route; private land does not universally mean 24 months or a fixed 4× compensation.
- FRA recognizes individual/community rights and prevents eviction until recognition/verification is complete. Consultation/consent depends on place and action; require legal review.
- ULPIN identifies a parcel but does not prove uncontested title.
- Digitized cadastral geometry may be ungeoreferenced/inaccurate; preserve source and corrected versions with GCP/error evidence.
- NDBI can confuse built surfaces and bare soil. Footprints have false detections/omissions and generally lack use/occupancy. Use as discrepancy evidence only.
- Census PCA is principally a 2011 baseline; building/population disaggregation is an estimate, not a beneficiary register.
- UNDRR's hazard/exposure/vulnerability/capacity language is conceptual, not approval of one exact formula.
- JJM's 55 LPCD is a service baseline, not sustainable-source proof.
- Scheme eligibility/funding and relocation need are independent; an owner-only or otherwise inapplicable scheme cannot erase assessed need.
- GIGW 3.0 references WCAG 2.1 AA; PUNARVAS-AI deliberately targets the higher WCAG 2.2 AA baseline.

## 9. Data and privacy posture

- Collect only data needed for an authorized programme purpose.
- Do not collect Aadhaar, bank, health, disability, contact, or other high-risk fields by default; document necessity and protection per field.
- Personal/member records and legal claims are restricted and excluded from ordinary maps, logs, exports, and public views.
- Public transparency means approved methods, criteria, aggregates, progress, notices, and lawfully publishable records—not a public sensitive household ranking.
- Personal and fine geospatial data, backups, and support paths remain in approved India-resident infrastructure as the project/government policy baseline and where legally required; do not describe DPDP as a universal localization mandate.
- Record-class retention, deletion, legal holds, historical exports and backup expiry require an approved schedule; hash linking does not authorize indefinite personal-data retention.
- Preliminary candidate coordinates may be generalized/restricted to reduce speculation and harm.

## 10. Current delivery state

- Revised documentation baseline exists; no application code, physical schema, infrastructure, scientific model, data agreement, or production integration has been implemented or validated yet.
- Target phase is `PH-0`: governance, evidence, workflow mapping, data agreements, baselines, privacy/security/ethics, and reference fixtures.
- Feature work must not begin with production personal data before PH-0 exits.
- The user approved Wayanad, the production-oriented documentation baseline, and named technology direction on 8 September 2026. This is not approval of operational formula parameters, legal workflows, data access, funding, or deployment.
- Open policy/evidence choices are tracked in [open-decisions.md](./open-decisions.md); research status is recorded by source and date in [source-register.md](./source-register.md), not inferred from tool availability.
- Current source state is inventory-only: no S01–S54 entry becomes operational until a permitted AOI sample passes license, access, coverage/time, schema/CRS/unit/NoData, checksum, quality, reproducibility, owner, and reviewer checks. Public sources can support the sandbox; S45–S50 block corresponding real approvals and allocations.

## 11. Known risks requiring continuing ownership

| Risk | Required response |
| --- | --- |
| Data/API agreements unavailable | Keep governed file import and shadow/synthetic path; do not scrape restricted systems |
| Catalog visible but download/entitlement unproven | Record discovery separately; hold the dependency until a permitted AOI sample passes |
| Mirrors/correlated sources overstate confidence | Group shared observations/inputs and deduplicate or use an approved comparison method |
| Land-record geometry/claim conflict | Preserve conflict and hold decision; require field/legal resolution |
| Hazard product incomplete or out of coverage | Show data gap; use approved alternate source; do not interpolate authority |
| Public disclosure causes household/site harm | Use restricted/default-private state, aggregation/generalization, disclosure review, withdrawal process |
| Policy weights or thresholds produce unstable outcomes | Sensitivity analysis, stakeholder/domain review, field calibration, versioning |
| Solver optimizes the wrong objective | Treat as scenarios, inspect unassigned/fairness, require participation and approval |
| Two approvals consume the same site/shared capacity | Drafts reserve nothing; atomically revalidate/reserve at approval; fail closed on conflict |
| Scheme mismatch or unfunded cost | Keep need separate, show eligibility reasons and funding gap, route alternate pathway/review |
| Approval mistaken for completed relocation | Track unit/services, acceptance, possession, occupation, defects and follow-up or explicit handoff |
| Government workflow changes | Effective-date new policy/template/authority and migrate cases explicitly |
| Field connectivity/device loss | Minimal encrypted packages, expiry, sync conflict, revoke and incident procedure |
| Accessibility or Malayalam explanation fails | Block household-facing release until manual testing and translation review pass |
| Institutional delay | Reminders and authorized escalation recommendation; never fabricate/bypass authority |

## 12. Update protocol for future maintainers/agents

1. Read this file, then the document owning the requested concern.
2. Re-check current law/order/source when the change depends on a date-sensitive fact; cite access/effective date.
3. Do not infer current requirements from old transcript code or early dual-horizon sections.
4. Identify affected `OUT-*`, `RUL-*`, `FR/NFR-*`, `FEAT-*`, `PH-*`, `ARC-*`, and `DEC-*` IDs.
5. Add or supersede a decision for any material scope, authority, policy, algorithm, architecture, privacy, or rollout change.
6. Update the owner document; other documents should link rather than duplicate normative detail.
7. Add/adjust acceptance scenarios and migration/rollback consequences.
8. Update `as_of` and version only after cross-document validation.
9. Never activate a formula from transcript text alone; update the equation, parameter, source, decision and test records together.

## 13. Essential references

- [KSDMA Wayanad reports/PDNA](https://sdma.kerala.gov.in/reports-landslides-2024/)
- [KSDMA hazard maps](https://sdma.kerala.gov.in/hazard-maps/)
- [Kerala LSG DM Plans](https://sdma.kerala.gov.in/local-self-government-dm-plans/)
- [Kerala Vulnerability Linked Relocation Scheme](https://sdma.kerala.gov.in/vulnerability-linked-relocation-scheme/)
- [Disaster Management Act, 2005](https://www.mha.gov.in/sites/default/files/2022-09/The%20Disaster%20Management%20Act,%202005%5B1%5D.pdf)
- [RFCTLARR resources](https://dolr.gov.in/en/document-category/acts-rules/)
- [DILRMP 3.0](https://dolr.gov.in/en/document/digital-india-land-records-modernization-programmedilrmp-3-0-operational-guidelines-2026-2031/)
- [Forest Rights Act and Rules](https://tribal.nic.in/FRA/data/FRARulesBook.pdf)
- [DPDP Rules, 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa)
- [Indian geospatial guidelines](https://dst.gov.in/sites/default/files/Final%20Approved%20Guidelines%20on%20Geospatial%20Data.pdf)
- [Disaster Management (Amendment) Act, 2025](https://prsindia.org/files/bills_acts/acts_parliament/2025/The_Disaster_Management_Act%2C_2025.pdf)
- [CERT-In Directions, 2022](https://www.cert-in.org.in/PDF/CERT-In_Directions_70B_28.04.2022.pdf)
