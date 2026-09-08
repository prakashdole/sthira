---
title: PUNARVAS-AI Documentation and Assurance Plan
document_id: PUN-PLAN
version: 1.2
status: Approved documentation baseline; implementation validation pending
as_of: 2026-09-08
audience: Product, programme, legal, GIS, field, engineering, security, delivery, and assurance teams
owner: Product owner
normative_scope: Documentation ownership, approved baseline, revision controls, and acceptance plan
---

# PUNARVAS-AI Documentation and Assurance Plan

## Summary

Maintain the nine project documents and five controlled companion registers at the repository root for a mixed government, product, GIS, legal, field, and engineering audience.

Authoritative precedence:

1. Current user instructions and confirmed choices.
2. Later transcript statements over earlier transcript statements.
3. Primary law, government publications, and scientific evidence over factual claims made by chatbots.
4. Embedded transcript code is non-normative reference only.

The user approved this production-oriented blueprint, Wayanad reference pilot, product name, and technology baseline in the instruction **“PLEASE IMPLEMENT THIS PLAN”** on 8 September 2026. This approves documentation direction, not statutory authority, funding, data access, scientific validation, or production readiness. The product is a permanent-relocation decision-support platform—not a live emergency-command system.

## Document Set

- `prd.md`: Product problem, users, outcomes, end-to-end workflows, success metrics, Wayanad pilot, scope, and exclusions.
- `rules.md`: Safety, legal, privacy, provenance, scoring, approval, publication, and human-review rules. It will explicitly prohibit automatic title clearance, forced relocation, auto-gazetting, and unverified public hazard claims.
- `trd.md`: Numbered functional and non-functional requirements, data contracts, state machines, interfaces, security, accessibility, localization, SLOs, disaster recovery, and acceptance criteria. It will not contain deployment architecture.
- `architecture.md`: Technology choices, components, trust boundaries, deployment topology, data flows, failure handling, scaling, and Mermaid diagrams. It will reference TRD requirement IDs instead of repeating requirements.
- `feature.md`: Feature catalog organized by user role, behavior, dependencies, acceptance criteria, and rollout phase.
- `phases.md`: Governance/data-agreement phase, platform foundation, Wayanad shadow pilot, controlled live deployment, Kerala scaling, and multi-state adaptation, each with entry and exit gates.
- `memory.md`: Compact durable context for future agents: current truth, glossary, precedence rules, non-goals, architecture baseline, research corrections, risks, and update protocol.
- `decisions.md`: ADR-style log for every material decision, including context, rationale, rejected alternatives, consequences, evidence, status, and review trigger.
- `equations.md`: Complete mathematical retention registry classifying every relevant equation as core, optional, specialist/external, or rejected.
- `parameters.md`: Effective-dated parameter, units, provenance, approval, and validation register.
- `source-register.md`: Dated evidence register plus the complete S01–S54 operational acquisition inventory, coverage, access route, caveats, and activation gate.
- `open-decisions.md`: Decision and validation gaps that prevent silent defaults or overclaiming.
- `changes.md`: Revision record, ID migration notes, formula-disposition matrix, and validation evidence.

Each file will have consistent metadata, stable requirement/decision IDs, explicit ownership boundaries, cross-links, an “as of 8 September 2026” date, and its own relevant references.

## Product and Technical Baseline

### Core workflow

1. Register a relocation programme and jurisdiction, then record whether permanent relocation is necessary compared with feasible risk reduction in place.
2. Ingest and version authoritative hazard, cadastral, demographic, infrastructure, and evidence sources.
   Start with the reviewed public Wayanad minimum stack and synthetic household/parcel fixtures; keep mandatory unavailable inputs `UNKNOWN`/`HOLD`.
3. Estimate settlement exposure with uncertainty; never treat estimates as beneficiary counts.
4. Reconcile parcel records, observed occupation, disputes, FRA status, and field evidence.
5. Assess candidate sites across safety, legal readiness, lean-season water, infrastructure, livelihood, accessibility, capacity, and cost.
6. Enumerate and verify households, eligibility, vulnerability, consent, and preferences.
7. Assess scheme-specific eligibility, entitlement, funding status, cost coverage, milestones, and shortfalls without equating funding eligibility with relocation need.
8. Produce reviewable allocation scenarios for township or self-relocation pathways.
9. Run objections, appeals, approvals, notification, reservation, and supersession workflows.
10. Track or explicitly hand off funding, construction/unit readiness, functioning services, acceptance, possession, occupation, defects, and livelihood follow-up.
11. Generate government dossiers, accessible reports, LSG disaster-plan annexes, machine-readable exports, and tamper-evident manifests.

### Decision policy

- Use hard gates with `PASS`, `FAIL`, `UNKNOWN`, or `BLOCKED`, followed by separate evidence-backed criterion scores.
- Do not create the transcript’s opaque `Ω` master score or use institutional readiness to inflate household risk.
- Keep household priority, site suitability, legal readiness, and allocation preference separate.
- MCDA weights must be approved, versioned, sensitivity-tested, explainable, and recalibrated against field results.
- Allocation will use an auditable, capacity-constrained MILP scenario generator with indivisible households, consent, preferences, accessibility, and site capacity constraints. Results remain proposals requiring approval.
- Allocation objective order, coverage unit, tie policy, solver tolerances, and fairness constraints remain explicit open policy decisions; no software default may settle them.
- Evidence, administrative, legal, notice, reservation, and delivery states are separate axes. Official approval and notification are not assumed to be the same act.
- Every mathematical family is retained in [equations.md](./equations.md); only approved, validated, in-scope versions may execute.

### Architecture

- Next.js/TypeScript bilingual PWA with MapLibre 2D maps and equivalent accessible table/form views.
- Python FastAPI modular monolith with domain modules and background workers; no premature microservices.
- PostgreSQL/PostGIS as the spatial system of record.
- India-resident S3-compatible object storage for immutable source files, COGs, evidence, and exports.
- STAC catalog for raster/imagery provenance, OGC API Features/GeoJSON for vector exchange, and MVT for large map layers.
- Separate source registry, catalog discovery, authenticated download/governed receipt, optional processing, evidence validation, and display basemap layers; alternate catalogs for one observation are not independent evidence.
- OIDC/SAML identity, MFA, role plus geography-scoped authorization, and PostgreSQL row-level security.
- Append-only, hash-linked audit events and signed export manifests; no blockchain claim.
- Offline field-verification PWA with controlled synchronization and conflict review.
- Transactional outbox/equivalent delivery, idempotent workers, independent allocation feasibility validation, and atomic capacity reservations.
- Authorization enforced across APIs, tiles, STAC, COG/object delivery, reports, search, signed URLs, and caches.
- Default targets: 99.5% monthly availability, RPO ≤15 minutes, RTO ≤4 hours, p95 standard reads ≤1 second, and asynchronous handling of heavy spatial/report jobs.
- GIGW 3.0 and WCAG 2.2 AA, with English/Malayalam pilot localization and map-independent access.
- Separate prototype, authorized-pilot, and production deployment profiles; Kubernetes/OpenShift is a production option, not a demonstration prerequisite.

### Core records

Define versioned contracts for programmes, jurisdictions, relocation-necessity assessments, settlements/communities, households and restricted members, datasets, hazard layers/zones, exposure estimates, parcels and parcel versions, legal interests/claims, observations, evidence, candidate sites, assessments, criterion results, schemes, eligibility/entitlements, funding, preferences, allocation scenarios, capacity reservations/commitments, objections, decisions, approvals, notices, delivery/completion milestones, workflow tasks, exports, formula/parameter versions, and audit events.

Every dataset version will record publisher, source URL, S01–S54 capability, acquisition method, entitlement/quota/cost, license plus redistribution/offline rights, checksum, acquisition and valid time, coverage, schema, CRS/vertical datum, resolution, units/NoData, method, confidence, shared-input/mirror links, supersession, fallback, owner, and responsible reviewer.

## Research Corrections and Evidence

The documents and decision log will cite primary sources including the Disaster Management Act, RFCTLARR Act and Kerala rules, FRA, DILRMP 3.0, KSDMA hazard maps and LSG plan template, Kerala PDNA and relocation scheme, GSI Wayanad findings, DPDP law/rules, Indian geospatial guidelines, JJM, Census, Open Buildings, UNDRR, OGC, PostgreSQL/PostGIS, MapLibre, GIGW, and WCAG.

The dated status, narrow supported claim, and unresolved verification for each source belong in [source-register.md](./source-register.md). Linked material is not treated as fully verified merely because a page exists or a research tool returned it.

Corrections to preserve explicitly:

- No universal statutory DDMA meeting-count formula or autonomous authority bypass.
- Under Disaster Management Act §31(4), as amended in 2025 and commenced 9 April 2025, a District Plan is reviewed/updated at least once every two years or earlier as necessary; this is not a universal cadence for every plan or workflow.
- C-FLOOD does not currently provide Wayanad coverage.
- ULPIN and digitized cadastral maps do not prove uncontested title.
- NDBI and building footprints cannot prove occupation or beneficiary identity.
- FRA/Gram Sabha requirements are context-specific and require legal review.
- JJM’s 55 LPCD is a service baseline, not proof of sustainable water yield.
- Private land does not always imply a fixed two-year acquisition workflow or fixed compensation multiplier.
- Wayanad beneficiary and construction figures are time-versioned facts, not constants.
- Public transparency will be de-identified unless publication of an individual record is specifically lawful and approved.
- No custom hazard model, raw InSAR pipeline, voice/LLM decision path, social scraping, Cesium globe, rescue routing, convoy logistics, or automatic relocation order.
- CDSE replaces obsolete SciHub; Earth Engine remains terms/quota dependent; Bhoonidhi and NWDP use documented resource-specific routes; CHIRPS v3 is preferred for a new integration; paused SoilGrids REST and public OSM tiles are not dependencies.

## Validation and Acceptance

- Trace every feature to a PRD outcome, TRD requirement, rule, architecture component, phase, decision, equation/parameter where applicable, and test.
- Run consistency checks ensuring TRD and architecture content are not mixed or duplicated.
- Validate scenarios covering debris-flow runout on flat terrain, paper-vacant but occupied land, cadastral offsets, pending FRA claims, inadequate lean-season water, changing beneficiary lists, objections, self-relocation choice, capacity shortfalls, offline sync conflicts, escalation without authority bypass, superseded hazard zones, accessible non-map use, catalog-without-download, expired entitlement, paused endpoint, correlated mirrors, out-of-coverage data, and rejected AOI samples.
- Verify every legal/numeric claim has a primary citation, date, jurisdiction, and qualification; remove invented constants and unsupported claims.
- Confirm all controlled files exist, link correctly, preserve transcript mathematics only through classified references, and disclose rather than conceal unresolved policy, legal, scientific, data, staffing, procurement, and validation work.

## Assumptions

- Product name remains **PUNARVAS-AI**.
- Wayanad is the reference pilot; architecture remains configurable for other states.
- Documentation is in English; the planned product is bilingual for the pilot.
- The blueprint supports official decision-making but is not itself legal, cadastral, hydrological, or geotechnical certification.
- Research is represented by the dated [source register](./source-register.md); operational users must revalidate time-sensitive law, orders, coverage, licences, and figures. Tool availability is not evidence quality.
