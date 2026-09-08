---
title: PUNARVAS-AI Open Decisions and Validation Gaps
document_id: PUN-OPEN
version: 1.2
status: Active decision backlog
as_of: 2026-09-08
audience: Programme sponsors, product, policy, legal, domain, architecture, delivery, and assurance teams
owner: Product and architecture decision forum
normative_scope: Unresolved choices that block activation, pilot progression, or production claims
---

# PUNARVAS-AI Open Decisions and Validation Gaps

## 1. Confirmed baseline versus open implementation choices

The user approved the 8 September 2026 plan to use PUNARVAS-AI, Wayanad as the reference pilot, a production-oriented blueprint, the documented technology baseline, and the separation between permanent-relocation planning and emergency command. Those are not open merely because earlier transcripts discussed alternatives.

The items below remain open because the approved plan deliberately requires programme, legal, scientific, community, procurement, or operational evidence that has not yet been supplied. `OPEN` blocks the affected activation; `PH-0` means it must be closed before controlled pilot use; `BEFORE-PRODUCTION` allows sandbox work but not production reliance.

## 2. Decision register

| ID | Decision required | Owner(s) | Due gate | Default while open | Closure evidence |
| --- | --- | --- | --- | --- | --- |
| ODN-001 | Name the statutory/programme authority and precise Wayanad workflow/case type | Government sponsor, legal lead | PH-0 | Analytical/sandbox only | Charter, order, RACI and signed workflow |
| ODN-002 | Approve relocation-necessity test and how in-situ mitigation alternatives are documented | Hazard, engineering, social and programme panel | PH-0 | No case labelled “relocation necessary” | Policy, reasons, reviewer qualifications and fixtures |
| ODN-003 | Approve scheme-specific household categories, eligibility, entitlements, cost heads and funding milestones | Programme, finance and legal owners | PH-0 | Eligibility/funding `UNKNOWN`; does not erase need | Effective order/scheme version and test cases |
| ODN-004 | Approve household-priority criteria, weights or ordered rules | Programme/social-policy authority | PH-2 | No numeric priority ranking | Participation record, legal review, subgroup/double-count tests |
| ODN-005 | Approve site-comparison criteria, anchors, weights and rank-instability response | Multi-domain panel | PH-2 | Show dimensions without aggregate rank | Policy version, sensitivity/rank-reversal results |
| ODN-006 | Choose allocation objective order: urgency versus coverage; household versus person coverage; preference, cost and fairness | Programme authority | PH-2 | Solver may run only against labelled test objectives | Signed policy and adversarial fixtures |
| ODN-007 | Choose allocation tie-break and whether community/caregiving links are hard or soft | Programme/social authority | PH-2 | Do not resolve material ties automatically | Participation/fairness assessment and versioned rule |
| ODN-008 | Select solver/version, scaling, tolerances, timeout, gap policy and independent validator | Quantitative assurance and engineering | PH-2 | No production scenario | Benchmark, deterministic fixtures and validator evidence |
| ODN-009 | Define reservation expiry/release, cross-scenario commitments, overlapping sites and shared-resource locking | Programme, finance and engineering | PH-2 | Drafts reserve nothing; approval fails closed on conflict | Transaction model and race-condition tests |
| ODN-010 | Define conditional-approval classes and which conditions block comparison, reservation, allocation, handover or occupation | Legal and domain owners | PH-0 | Unknown/blocking conditions prohibit advancement | Gate matrix with authority and expiry |
| ODN-011 | Define distinct lawful-basis, participation, pathway, preference/acceptance and community-process records | Privacy, legal and social owners | PH-0 | Silence/missing contact is no consent | Forms, notices, policy and assisted-flow tests |
| ODN-012 | Set record-class retention, deletion, legal hold, backup expiry and historical-export policy | Records, privacy and legal owners | PH-0 | Minimum collection; no ad-hoc deletion or indefinite retention promise | Effective schedule and deletion/restore tests |
| ODN-013 | Pin STAC/OGC conformance profiles and supported CRS/format versions | GIS architect/data steward | PH-1 | Non-conformant adapters remain sandboxed | Conformance report and fixtures |
| ODN-014 | Select basemap provider, attribution, license, availability and offline rights | Procurement/GIS/product | PH-1 | No production basemap dependency | Contract/license and offline package test |
| ODN-015 | Select prototype, authorized-pilot and production hosting profiles and budget | Sponsor, procurement, security, operations | Before each profile | Local/sandbox only | Costed bill of materials, staffing and approval |
| ODN-016 | Pre-register pilot sample, reference evidence, seasonal windows, tolerances and subgroup analysis | Evaluation lead and domain board | PH-2 entry | No “validated” or predictive claim | Signed protocol and independent review |
| ODN-017 | Define completion/handoff ownership after approval, including funding, unit/services, possession, occupation and follow-up | Programme/delivery authority | PH-0 | Case remains incomplete or explicitly handed off | State model, responsible system/owner and reconciliation method |
| ODN-018 | Confirm exact DPDP commencement/applicability, CERT-In applicability, geospatial restrictions and records law | Legal/privacy/security owners | PH-0 and release | More protective baseline; no claim of universal legal mandate | Effective-dated compliance register |
| ODN-019 | Verify specialist model sources, calibration and independent validation for any proposed activation | Relevant scientific authority | Before model activation | Consume approved external output only | Model card, source, local validation and authority approval |
| ODN-020 | Decide whether the separately supplied SIH problem statement remains an acceptance target | Product sponsor | Before competition claim | Do not claim exact SIH compliance | Authoritative problem statement and mapping |
| ODN-021 | Select exact product releases, primary/alternate connectors, licenses, quotas/cost ceilings, refresh cadence, and fallbacks for the Wayanad minimum public stack | GIS/data steward, procurement, security, domain owners | PH-1 | Registered/documented only; no operational-access claim | Permitted AOI samples and source-readiness report passing FR-078/NFR-035 |
| ODN-022 | Secure the S45–S50 land/right, water/service, household, field/geotechnical, and programme/funding agreements and collection plans for the bounded live workflow | Government sponsor, legal/data owners, field/domain leads | PH-2 exit / PH-3 entry | Sandbox/shadow only; affected actions stay `UNKNOWN`/`HOLD` | Signed agreements, qualified field plans/results, privacy controls, owner reviews, and acceptance scenarios |

## 3. Known unresolved evidence

- Current documented coverage and permitted use for each operational hazard source.
- The exact Wayanad data-sharing route for cadastral, household, scheme, water and legal records.
- Current Kerala scheme rules, caps, hazard/category coverage and stage-payment evidence for each pathway.
- Measured baseline dossier cycle time; the 30% reduction remains a target, not a result.
- Local pilot staffing, procurement lead time, hosting cost and support model.
- Seasonal water observation period and independent reference assessments.
- The transcript-attributed GLOF coefficient and any local specialist hazard parameters.

## 4. Closure procedure

Closing an item requires a new or amended `DEC-*` record with approver, role, date, evidence, affected IDs, consequences and review trigger. Update [parameters.md](./parameters.md), [source-register.md](./source-register.md), tests and all affected documents. Never close an item solely because software needs a default value.
