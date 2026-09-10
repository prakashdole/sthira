---
title: Sthira Domain, Safety, and Governance Rules
document_id: PUN-RULES
version: 1.2
status: Normative baseline
as_of: 2026-09-08
audience: Product, legal, programme, GIS, field, engineering, security, assurance, and audit teams
owner: Programme authority with legal and domain stewards
normative_scope: Binding product and operational constraints
---

# Sthira Domain, Safety, and Governance Rules

## 1. Document boundary

This is the normative rulebook for product behavior and use. Product goals belong in [prd.md](./prd.md), measurable system requirements in [trd.md](./trd.md), and implementation topology in [architecture.md](./architecture.md). Where another document conflicts with a rule here, the conflict must be resolved through [decisions.md](./decisions.md) before release.

`MUST`, `MUST NOT`, `SHOULD`, and `MAY` have their conventional requirements meanings.

## 2. Authority and state rules

| ID | Rule |
| --- | --- |
| RUL-001 | The system MUST describe every algorithmic output as advisory until the applicable qualified reviewer and authority approve it. |
| RUL-002 | Software permissions MUST NOT be presented as statutory authority. Each approval or publication action MUST identify its legal/programme basis and authorized role. |
| RUL-003 | Hazard evidence, site assessments, administrative decisions, legal acts, notices, reservations, and delivery MUST use separate explicit state fields. `OFFICIALLY_APPROVED` and `OFFICIALLY_NOTIFIED` MUST NOT be treated as one act unless the applicable workflow establishes that equivalence. |
| RUL-004 | Only an authorized act outside the algorithm may move an item into an official/notification state. The system MUST preserve the approving identity, authority, time, evidence package, and signed artifact. |
| RUL-005 | A later version MUST supersede rather than overwrite an earlier decision-used version. Historical exports MUST remain reproducible from manifest versions to the extent lawful retention permits; lawful deletion/restriction MUST leave the minimum permitted tombstone and reason rather than silently rewriting history. |
| RUL-006 | SLA monitoring MAY recommend or route an escalation to an authorized role, but MUST NOT bypass an authority, fabricate a statutory deadline, or treat meeting frequency as a risk multiplier. |
| RUL-007 | Any preliminary site whose disclosure could enable speculation, intimidation, or harm MUST be access-restricted or spatially generalized until authorized publication. |

## 3. Evidence and provenance rules

| ID | Rule |
| --- | --- |
| RUL-008 | Every dataset and material evidence item used in a result MUST have a source, publisher, license/use basis, checksum, acquisition time, valid/observation time, geographic scope, reviewer, and version. Spatial data MUST also record CRS, scale/resolution, and processing lineage. |
| RUL-009 | Evidence MUST be preserved as submitted while lawfully retained. Corrections MUST create a new version linked to the corrected item and reason. Deletion/restriction follows RUL-050 and records the minimum lawful tombstone without retaining prohibited personal content. |
| RUL-010 | Source conflicts MUST remain visible until an authorized reviewer resolves them. A lower-priority source MUST NOT silently overwrite an official or signed record. |
| RUL-011 | Source authority is contextual. Official notification/court orders, signed domain verification, authoritative agency data, local registers, open data, and derived analysis MUST be labeled rather than collapsed into an unconditional global ranking. |
| RUL-012 | Time-varying counts, programme amounts, site capacity, construction status, and beneficiary lists MUST be stored as dated facts. They MUST NOT be embedded as permanent software constants. |
| RUL-013 | A result with missing required evidence MUST be `UNKNOWN` or `BLOCKED`; absence of evidence MUST NOT be converted into a pass. |
| RUL-014 | Data reuse MUST comply with its license, access classification, purpose, retention, and disclosure limits. |

## 4. Hazard and exposure rules

| ID | Rule |
| --- | --- |
| RUL-015 | The baseline MUST consume approved hazard products; it MUST NOT train or publish a new authoritative susceptibility model without a separate scientific validation and approval process. |
| RUL-016 | The system MUST record hazard type, source agency, product date, method, resolution, known coverage, uncertainty, and supersession. |
| RUL-017 | C-FLOOD or another source MUST NOT feed gates, eligibility, allocation, official outputs, or operational conclusions outside its documented coverage. An out-of-coverage layer MAY appear only in an isolated research view with an unmistakable unsupported warning, no decision linkage, and recorded authorization. A missing source MUST create an alternate-source or data-gap state. |
| RUL-018 | Local slope alone MUST NOT establish landslide safety. Applicable runout, drainage/channel, upstream source, and field evidence MUST be considered. |
| RUL-019 | Exposure derived from Census, footprints, remote sensing, or dasymetric allocation MUST be labeled as an estimate with bounds/method and MUST NOT create household identities, eligibility, or beneficiary status. |
| RUL-020 | Analytical risk information MUST NOT be publicly represented as a gazetted red zone. Map legends, exports, APIs, and tables MUST expose the state and authority consistently. |

## 5. Land and legal rules

| ID | Rule |
| --- | --- |
| RUL-021 | ULPIN, Record of Rights, mutation, a digitized cadastral map, or a government-land label MUST NOT by itself prove conclusive or uncontested title. |
| RUL-022 | Remote sensing, NDBI, a building footprint, or absence from a map MUST NOT prove vacancy, occupation, use, ownership, or encumbrance. Such evidence MAY create a discrepancy task only. |
| RUL-023 | Cadastral geometry MUST retain original/source geometry and any georeferenced or corrected geometry as separate versions with transformation method, control points, residual error, and reviewer. |
| RUL-024 | Candidate-site legal assessment MUST represent ownership evidence, acquisition/transfer route, tenancy, possession, litigation, charges, forest classification, FRA claims/rights, common/community use, and required consultation or clearance. |
| RUL-025 | FRA, forest, PESA, and Gram Sabha requirements MUST be determined for the actual location and proposed action by authorized legal/forest-rights reviewers. The system MUST NOT apply a blanket consent rule or blanket permanent rejection. |
| RUL-026 | Unresolved occupation, title, community use, or rights claims MUST place the relevant gate on hold. A final reject requires an authorized reason; a later resolution MUST permit reassessment. |
| RUL-027 | Private land MUST route through the applicable negotiated purchase, transfer, acquisition, RFCTLARR, or other approved workflow. The system MUST NOT assume a universal duration, compensation multiplier, or procedure. |

## 6. Site and service rules

| ID | Rule |
| --- | --- |
| RUL-028 | Site assessment MUST keep safety, legal readiness, water, infrastructure, accessibility, livelihood/social fit, capacity, cost, and implementation readiness as separately inspectable dimensions. |
| RUL-029 | A required hard gate MUST return only `PASS`, `FAIL`, `UNKNOWN`, or `BLOCKED`, with evidence, reviewer, rule/policy version, and reason. |
| RUL-030 | The 55 LPCD Jal Jeevan Mission level MAY be used as an applicable baseline demand. Passing water feasibility additionally requires authorized evidence of lean-season yield, reliability, quality, competing demand, treatment, storage, and distribution feasibility. |
| RUL-031 | Satellite water indices, monsoon observation, or a database entry MUST NOT establish sustainable water availability alone. |
| RUL-032 | Capacity MUST identify its limiting bases, including developable area, dwelling units, household/person capacity, water, sanitation, access, services, environmental limits, and phasing. |
| RUL-033 | Any fixed distance, slope, flood, livelihood, or service threshold MUST cite an approved policy or domain standard, include units and applicability, and be versioned. Unapproved transcript constants MUST NOT be used. |

## 7. Priority, scoring, and allocation rules

| ID | Rule |
| --- | --- |
| RUL-034 | Household eligibility/priority, site suitability, legal readiness, programme readiness, and allocation preference MUST be separate decisions and data products. |
| RUL-035 | The system MUST NOT implement the transcript's `Ω` formula or another opaque master score that can hide a failed or unknown gate. |
| RUL-036 | Comparative MCDA MAY run only after hard-gate evaluation. Every criterion MUST expose raw value, unit, normalization, source, confidence, weight, policy version, missing-data handling, and contribution. |
| RUL-037 | MCDA policy sets MUST be approved, versioned, tested for internal consistency where applicable, sensitivity-tested, and recalibrated against field/decision outcomes. Material rank instability MUST be shown to reviewers. |
| RUL-038 | A household priority policy MUST be published in understandable terms to affected households, subject to lawful privacy limits, and provide reason codes for inclusion, exclusion, and rank/tier. |
| RUL-039 | Protected or sensitive characteristics MAY be used only when lawfully required for vulnerability, accommodation, or affirmative priority and MUST NOT be repurposed. |
| RUL-040 | Allocation MAY be generated only for households and options permitted by the applicable lawful workflow. A `CONDITIONALLY_APPROVED` option is allocatable only when every allocation-stage blocking prerequisite is `PASS`; conditions deferred to reservation, handover, or occupation MUST be named and enforced at that action. Compulsory-acquisition workflows require separate legal configuration and MUST NOT be forced into a universal voluntary-consent model. |
| RUL-041 | Allocation optimization MUST preserve household indivisibility, site capacity, explicit accessibility constraints, preferences, and a visible unassigned outcome. It MUST NOT force an infeasible assignment to maximize coverage. |
| RUL-042 | Every proposed allocation MUST provide a plain-language explanation, policy/solver version, constraints, alternatives considered, and supported reasons or counterfactuals for an unassigned household. It MUST NOT claim one uniquely binding reason when constraints or objective trade-offs interact. |
| RUL-043 | A solver result MUST remain a proposal until participation, review, objection opportunity, and authorized approval are complete. Manual changes MUST require a reason and remain auditable. |

## 8. Participation, objection, and remedy rules

| ID | Rule |
| --- | --- |
| RUL-044 | Processing lawful basis/notice, programme participation, pathway choice, site preference, offer acceptance/refusal, and any legally required community process MUST be separately recorded through accessible procedures. Silence, missing contact, or an incomplete form MUST NOT be treated as consent or refusal. |
| RUL-045 | The system MUST support assisted service for households without devices, connectivity, literacy, or accessible self-service. |
| RUL-046 | Draft eligibility, priority, site, or allocation records MUST provide a correction/objection route before finalization whenever the governing procedure requires it. |
| RUL-047 | Objection and appeal authorities, filing windows, notice methods, hearing steps, SLA, and remedies MUST be configured from the applicable order or programme. No universal period may be assumed. |
| RUL-048 | An objection MUST receive a receipt, immutable filing time, scope, evidence history, assigned officer, decision/reason, and notice record. |
| RUL-049 | A pending objection MUST visibly affect the status of dependent decisions according to the approved workflow. It MUST NOT disappear from exports or dashboards. |

## 9. Privacy, security, and publication rules

| ID | Rule |
| --- | --- |
| RUL-050 | Personal data MUST be collected for an explicit authorized purpose, minimized, access-scoped, kept accurate, protected, and handled under the provisions applicable on the processing date. Retention, deletion, restriction, legal holds, old exports, audit events, and backup expiry MUST follow an approved record-class schedule; a hash chain never justifies indefinite personal-data retention. |
| RUL-051 | Aadhaar, bank, health, disability, contact, identity-document, and precise household-location data MUST NOT be collected by default. Each field requires a documented necessity and lawful basis; high-risk fields require field-level protection. |
| RUL-052 | Public transparency MUST use aggregates, de-identification, approved criteria, official notices, and lawfully publishable records. Household-level scores/evidence are restricted unless specific publication is lawful and authorized. |
| RUL-053 | Preliminary site coordinates and sensitive/fine geospatial data MUST obey applicable Indian geospatial restrictions, official boundary requirements, access controls, and approved hosting policy. India-resident deployment is a product/government policy baseline, not a claim that DPDP universally mandates localization of every record. |
| RUL-054 | All state-changing actions MUST use authenticated identity, least privilege, geography scope, and auditable authorization. Shared accounts are prohibited. |
| RUL-055 | The product MUST provide a non-map equivalent for every critical map workflow and MUST NOT rely on color alone. English and Malayalam pilot content MUST convey equivalent status and warnings. |

## 10. Audit, exports, and automation rules

| ID | Rule |
| --- | --- |
| RUL-056 | Audit events MUST be append-only and tamper-evident, recording actor, authority/scope, action, affected object/version, time, reason, request/correlation ID, and prior-event/hash linkage where applicable. |
| RUL-057 | The product MUST NOT claim that a database or hash chain is immutable or a blockchain. Corrections occur through compensating events and new versions. |
| RUL-058 | Every official or review export MUST name its state, generating user, source/policy versions, unresolved conditions, generated time, manifest, and checksum. A signed approval is separate from generation. |
| RUL-059 | Generated prose MUST be template-based in the baseline. Any future LLM-assisted text requires a new decision, source-bound claims, restricted data controls, full traceability, and human approval; it may not make authoritative decisions. |
| RUL-060 | Automatic reminders, validation, analysis, and scenario generation MAY assist users. Automatic gazetting, legal clearance, beneficiary finalization, forced relocation, fund release, or authority bypass are prohibited. |

## 11. Mathematics, parameters, and numerical assurance

| ID | Rule |
| --- | --- |
| RUL-061 | Every mathematical family from the source conversations MUST remain traceable in [equations.md](./equations.md) as `CORE`, `OPTIONAL`, `SPECIALIST/EXTERNAL`, or `REJECTED`; documentation alone MUST NOT activate a formula. |
| RUL-062 | Every executable formula and parameter version MUST satisfy the contract in [equations.md](./equations.md) and [parameters.md](./parameters.md), including units, CRS/time support, provenance, applicability, missing/zero handling, uncertainty, owner, tests, and approval. |
| RUL-063 | Missing, invalid, out-of-domain, out-of-coverage, zero-denominator, NaN, infinite, empty-geometry, or unit-incompatible inputs MUST produce a typed validation failure or `UNKNOWN`; they MUST NOT be coerced to zero, pass, or a favorable score. |
| RUL-064 | Only compatible quantities may be added, compared, or minimized. Population, households, dwellings, area, money, flow, storage, probability, suitability, and legal status MUST remain dimensionally distinct unless an approved conversion is explicit. |
| RUL-065 | Scoring must test correlated criteria, double-counting, stable normalization anchors, rank reversal, sensitivity, and subgroup effects. AHP consistency or PCA variance MUST NOT be described as proof of validity or fairness. |
| RUL-066 | Specialist terrain, hydrological, runout, InSAR, seismic, or susceptibility equations MAY be retained as references but MUST NOT become authoritative product calculations without independent local scientific validation and approval. |

## 12. Programme, reservation, and completion rules

| ID | Rule |
| --- | --- |
| RUL-067 | Exposure or hazard evidence MUST NOT by itself establish that permanent relocation is necessary. A competent review MUST record feasible in-situ mitigation alternatives, reasons, uncertainty, settlement/community effects, and authority state. |
| RUL-068 | Scheme eligibility and entitlement MUST be effective-dated and specific to hazard, jurisdiction, programme, tenure/household category, cost head, cap/calculation, restrictions, milestones, sanction and appeal. Ineligibility under one scheme MUST NOT erase relocation need or eligibility under another pathway. |
| RUL-069 | Identified, applied, sanctioned, committed, released, received, spent, reconciled, and withdrawn funding MUST be separate states. Announced budgets MUST NOT be treated as household funding, and duplicate funding MUST be prevented. |
| RUL-070 | Draft scenarios are simulations and MUST NOT reserve capacity. Approval MUST atomically revalidate input versions and reserve dwellings, people, accessibility resources, budgets, overlapping parcels and shared services across programmes. Conflicts MUST fail closed. |
| RUL-071 | Reservation expiry, renewal, release, conversion to commitment, cancellation and supersession MUST be explicit, authorized and auditable; capacity MUST never be subtracted twice. |
| RUL-072 | An allocation, sanction, generated dossier, ceremony, or notification MUST NOT be represented as completed relocation. Completion requires the configured evidence for unit/site readiness, functioning services, acceptance, possession/handover, occupation, defects and follow-up—or an explicit accountable-system handoff. |
| RUL-073 | Solver status MUST distinguish optimal, feasible-not-proven-optimal, infeasible, time-limit-without-incumbent, and error. Version, options, scaling, tolerances, seed/tie policy, incumbent, bound and gap MUST be retained when applicable. |
| RUL-074 | Every solver or manually edited proposal MUST pass the same independent integer, unit, eligibility, preference, resource, budget, readiness and reservation feasibility validator before review or approval. |
| RUL-075 | Public disclosure controls MUST address inference, differencing, linkage and re-identification risk. The product MUST NOT promise that de-identified outputs make inference impossible. |

## 13. Source acquisition and activation rules

| ID | Rule |
| --- | --- |
| RUL-076 | A data product, catalog, download endpoint, processing service, display basemap, agency record route, and field-acquisition route MUST be represented as distinct source capabilities. Locating provider documentation or a catalog MUST NOT be recorded as successful access to a usable dataset. |
| RUL-077 | Before a source version may affect a decision, a permitted AOI sample MUST pass license/use, authentication, coverage/time, schema/format, CRS/vertical datum, unit/NoData, quality, checksum, reproducibility, freshness, owner, and reviewer checks defined in [source-register.md](./source-register.md#6-source-activation-and-acquisition-gate). |
| RUL-078 | Mirrors or processing platforms serving the same underlying observation MUST NOT be counted as independent evidence. Footprints, population models, rainfall products, DEMs, and land-cover products MUST disclose shared inputs and be reconciled or deduplicated before comparison or aggregation. |
| RUL-079 | Public/open data MAY support the mapping and exposure sandbox. Real site approval, household allocation, handover, or completion MUST remain disabled until the applicable land/right, household/participation, qualified field/geotechnical, lean-season water/service, and programme/funding evidence is verified. |
| RUL-080 | Source access failures, missing entitlements, expired tokens, quota exhaustion, stale versions, unsupported geography/time, paused APIs, or absent required field evidence MUST produce a visible `UNKNOWN`/`HOLD` and an acquisition task; the system MUST NOT silently substitute a weaker source or cached value. |
| RUL-081 | CDSE replaces obsolete SciHub for new Copernicus integration; SoilGrids REST MUST NOT be a dependency while paused; CHIRPS v3 is the preferred new CHIRPS integration; Earth Engine use requires approved use category, quota, cost, and privacy review; and Bhoonidhi/NWDP access MUST use their documented resource-specific routes. |
| RUL-082 | A basemap is display infrastructure, not analytical evidence. Provider key, attribution, usage, caching/offline, export, and privacy terms MUST be approved, and public OpenStreetMap tile services MUST NOT be bulk-downloaded for offline use. |
| RUL-083 | Sentinel-1 GRD/RTC, NISAR availability, a DEM/DSM, SoilGrids, remote-sensing indices, and building footprints MUST NOT be promoted into measured displacement, engineering parameters, safe-foundation evidence, occupation, ownership, or consent. Specialist processing requires its own activation and review. |

## 14. Mandatory gate sets

Gate applicability is configured by programme and reviewed by domain owners. The baseline gate groups are:

| Gate group | Examples | Advancement condition |
| --- | --- | --- |
| Hazard safety | Approved hazard exclusions, runout/channel assessment, flood/drainage, field hazard review | All applicable mandatory gates pass; unknown/blocked holds advancement |
| Land/legal readiness | Parcel identity/geometry, record reconciliation, occupation/community use, claims, FRA/forest, transfer/acquisition route | Authorized reviewers record the applicable route and no unresolved blocking issue |
| Water/environment | Lean-season quantity, quality, reliability, competing demand, treatment/distribution, environmental permissions | Sustainable service plan is evidenced and endorsed |
| Developability/services | Usable land, geotechnical evidence, drainage, sanitation, roads, energy, health, education, accessibility | Capacity basis and implementation dependencies are endorsed |
| Social/livelihood | Participation, livelihood continuity/transition, cultural/community needs, host-community impacts | Required consultation is complete and material harms have an approved response |
| Necessity/alternatives | Competent hazard/engineering/social review; feasible in-situ mitigation; settlement/community consequences | Relocation conclusion and alternatives are reasoned, evidence-backed, and in the required authority state |
| Scheme/funding | Applicable scheme/category, entitlement, eligible costs, sanction, nonduplicate funds and shortfall | Required funding state for the action is evidenced; lack of one scheme does not negate need |
| Household allocation | Verified eligibility, consent, preference, accommodation, site capacity, objection state | Proposal is feasible and may enter review; it is not yet an official allocation |
| Handover/occupation | Unit/site readiness, functioning services, unresolved defects, acceptance, possession and transition support | All conditions required for the configured milestone pass; approval alone cannot close the case |

## 15. Rule-change control

1. A proposed change identifies affected rules, legal/programme basis, data and migration impact, responsible owner, test cases, and effective date.
2. Legal, domain, privacy, security, and product owners review changes relevant to them.
3. The approved change creates a new policy/rule version; in-flight cases keep or explicitly migrate from the prior version.
4. Re-evaluation is never silent. Affected cases and outputs are flagged, recomputed if authorized, and compared with their prior result.
5. Material choices and rejected alternatives are recorded in [decisions.md](./decisions.md).

## 16. Primary references

1. [Disaster Management Act, 2005](https://www.mha.gov.in/sites/default/files/2022-09/The%20Disaster%20Management%20Act,%202005%5B1%5D.pdf), read with the [2025 amendment](https://prsindia.org/files/bills_acts/acts_parliament/2025/The_Disaster_Management_Act%2C_2025.pdf) and [commencement notice](https://www.pib.gov.in/PressReleasePage.aspx?PRID=2146781)
2. [RFCTLARR resources, Department of Land Resources](https://dolr.gov.in/en/document-category/acts-rules/)
3. [Forest Rights Act and Rules](https://tribal.nic.in/FRA/data/FRARulesBook.pdf)
4. [DILRMP 3.0 Operational Guidelines 2026–2031](https://dolr.gov.in/en/document/digital-india-land-records-modernization-programmedilrmp-3-0-operational-guidelines-2026-2031/)
5. [KSDMA hazard maps](https://sdma.kerala.gov.in/hazard-maps/)
6. [Kerala Local Self Government DM Plans](https://sdma.kerala.gov.in/local-self-government-dm-plans/)
7. [Jal Jeevan Mission](https://jaljeevanmission.gov.in/about_jjm)
8. [Digital Personal Data Protection Rules, 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa)
9. [Indian Guidelines on Geospatial Data, 2021](https://dst.gov.in/sites/default/files/Final%20Approved%20Guidelines%20on%20Geospatial%20Data.pdf)
10. [GIGW 3.0](https://guidelines.india.gov.in/gigw3/)
11. [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
12. [CERT-In Directions, 2022](https://www.cert-in.org.in/PDF/CERT-In_Directions_70B_28.04.2022.pdf)
13. [Equation Registry](./equations.md)
14. [Parameter Registry](./parameters.md)
15. [Operational Source Register](./source-register.md#5-operational-data-source-and-acquisition-register)
