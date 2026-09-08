---
title: PUNARVAS-AI Parameter and Policy Registry
document_id: PUN-PARAMETERS
version: 1.2
status: Controlled baseline; programme values require approval
as_of: 2026-09-08
audience: Programme owners, policy, domain specialists, data science, engineering, QA, and audit
owner: Policy owner with relevant domain steward
normative_scope: Parameter provenance, units, validity, approval, and change control
---

# PUNARVAS-AI Parameter and Policy Registry

## 1. Purpose

This register controls values used by formulas and policy rules. It complements [equations.md](./equations.md); an equation being documented does not approve its parameters or activate it. No transcript constant may enter an active policy merely because it appeared in `idea.txt`, `claude.txt`, or the source extract.

## 2. Required parameter contract

Every active parameter version must contain:

| Field | Requirement |
| --- | --- |
| `parameter_id`, `version` | Stable identifier and immutable version |
| `formula_ids` | Applicable `E*` equations |
| `name`, `definition` | Unambiguous meaning |
| `value` or `range` | Decimal/integer/category with precision |
| `unit` | UCUM-compatible unit or explicit dimensionless marker |
| `spatial_support` | Jurisdiction, site, source footprint, CRS/scale where relevant |
| `temporal_support` | Observation period, effective dates, season, timestep |
| `source` | Publisher, document/order/dataset version, URL/reference, checksum where obtained |
| `source_capability_ids` | One or more S01–S54 entries from [source-register.md](./source-register.md), including mirror/shared-input relationship |
| `acquisition_method` | Exact API endpoint, governed file/record request, or qualified field method; owner, entitlement, quota/cost, cadence and fallback |
| `authority` | Approver, role, programme/legal basis, approval date |
| `assumptions` | Applicability and exclusions |
| `missing_behavior` | Usually `UNKNOWN`/blocked; never silent zero/pass |
| `uncertainty` | Range/distribution/quality class and basis |
| `validation` | Fixtures, field comparison, tolerance, reviewer |
| `state` | `DRAFT`, `VALIDATED`, `APPROVED`, `SUSPENDED`, `SUPERSEDED`, or `REJECTED` |

Activation requires a compatible equation version, policy version, effective date, source coverage, named reviewer, and completed tests. Changing a value creates a new version and impact analysis; it never mutates historical results.

## 3. Approved baseline product parameters

These values were explicitly included in the user-approved production documentation plan. They are product targets, not scientific or statutory constants.

| ID | Value | Scope | State | Review trigger |
| --- | --- | --- | --- | --- |
| PAR-001 | 99.5% monthly availability | Controlled production service | APPROVED BASELINE | Criticality, measured demand, or hosting change |
| PAR-002 | RPO ≤15 minutes | Core production records | APPROVED BASELINE | Coordinated restore evidence or programme mandate |
| PAR-003 | RTO ≤4 hours | Core production service | APPROVED BASELINE | Recovery exercise or programme mandate |
| PAR-004 | p95 ≤1 second | Standard read API under agreed reference load | APPROVED BASELINE | Reference-load or user-experience change |
| PAR-005 | English and Malayalam | Wayanad household-facing pilot | APPROVED BASELINE | Pilot geography/language assessment |
| PAR-006 | 55 L/person/day | Applicable JJM rural domestic service-demand baseline only | CONDITIONAL | Programme/service classification or current JJM rule changes |

PAR-006 does not establish sustainable source yield, water quality, treatment, storage, environmental availability, other demand, or delivery feasibility. Those remain independently evidenced gates.

## 4. Parameters requiring programme or specialist approval

| ID | Parameter family | Formula links | Current value | Owner | Required evidence before activation |
| --- | --- | --- | --- | --- | --- |
| PAR-007 | Household-priority criteria and weights | E06–E07 | UNSET | Programme/social-policy authority | Lawful criteria, participation, overlap analysis, sensitivity and subgroup tests |
| PAR-008 | Site-suitability criteria, anchors and weights | E06, E08 | UNSET | Multi-domain review panel | Stable bounds, units, evidence quality, correlation/rank-reversal tests |
| PAR-009 | Mandatory gate applicability and expiry | E02 | UNSET by programme | Legal/domain owners | Applicable authority, evidence age and action-stage rules |
| PAR-010 | Allocation objective order and coverage unit | E19–E26 | OPEN | Programme authority | Decision on urgency versus coverage, households versus people, preference, cost and fairness |
| PAR-011 | Allocation tie-break | E25 | OPEN | Programme authority | Fairness review and reproducible audit design |
| PAR-012 | Solver scaling, feasibility/integrality tolerances, gap and timeout | E26 | UNSET | Quantitative assurance | Solver/version benchmark and independent validator fixtures |
| PAR-013 | Water demand beyond domestic baseline | E11–E12 | UNSET by site/programme | Water engineer | Existing users, institutions, livelihoods, fire, losses and seasonal data |
| PAR-014 | Developable-area exclusions and layout density | E13 | UNSET by jurisdiction/site | Planner/geotechnical authority | Applicable planning rules and approved layout |
| PAR-015 | Livelihood similarity/resource coefficients | E10, E15, E21 | UNSET | Livelihood/domain owner | Local occupational/resource evidence and household participation |
| PAR-016 | Service-access bounds/decay | E09 | UNSET | Programme/service owner | Network evidence and approved service standard |
| PAR-017 | Uncertainty ranges/distributions | E30 | UNSET per model | Model owner | Defensible empirical or scenario basis |
| PAR-018 | Cadastral residual/IoU review thresholds | E31 | UNSET | Survey/revenue authority | Survey specification and independent checkpoints |
| PAR-019 | Retention and legal-hold periods | E34 audit context | UNSET by record class | Records/privacy/legal owner | Effective law, record schedule and litigation requirements |
| PAR-020 | Pilot accuracy/error tolerances | E33 | OPEN | Evaluation owner/domain board | Preregistered sample and consequence-based thresholds |

## 5. Parameter-to-source acquisition map

This map identifies acceptable acquisition families, not pre-approval. Each active version must cite the exact artifact and pass the source gate. A missing mandatory input remains `UNSET`/`UNKNOWN`; an alternate is never substituted merely because it is easier to obtain.

| Parameter IDs | Required evidence/source route | Acquisition and activation condition |
| --- | --- | --- |
| PAR-001–005 | User-approved product baseline plus measured service/load/localization evidence | Configuration is approved; production claims require monitoring, recovery exercises, reference load, and reviewed Malayalam content rather than an external dataset. |
| PAR-006 | S46 JJM/KWA/operator records plus S49 field tests; JJM publication supplies the 55 LPCD demand baseline | Use 55 LPCD only where the service category applies; supply/yield/quality remain separate verified parameters. |
| PAR-007 | S48 verified household/participation records, S25 aggregate context, effective scheme/legal sources | Household criteria/weights require lawful records, participation, policy approval, and bias/double-count tests; remote-sensing population cannot substitute. |
| PAR-008 | S01–S05 hazard/geology, S18–S44 public context, S45–S47 agency records, S49 qualified field evidence | Each criterion pins its own source/version/unit; mandatory safety/legal/water/developability inputs cannot be replaced by a composite score. |
| PAR-009 | S01–S07 hazard authority, S45–S50 legal/agency/field evidence, effective law/orders | Gate applicability and expiry are approved by the competent owner for the action stage; unverified source availability yields hold. |
| PAR-010–012 | S48 preferences/participation, S49–S50 capacity/cost/readiness, approved policy, solver benchmark fixtures | Allocation objectives/ties/tolerances need signed policy and independent validator evidence; synthetic fixtures may test software only. |
| PAR-013 | S38–S39 regional water context, S46 operator commitments, S49 lean-season yield/quality tests, verified population | Demand and supply parameters stay distinct; nearby water level or monsoon imagery is not source yield. |
| PAR-014 | S44 boundaries, S45 parcel/legal data, S47 environmental restrictions, S49 survey/site design, applicable planning rules | Developable area/density activates only from approved parcel, exclusions, survey, and layout evidence. |
| PAR-015 | S25 aggregate occupation context plus S48 household/community participation and local resource verification | Coefficients remain unset until local definitions, consent, resource evidence, and validation exist. |
| PAR-016 | S41 OSM, S42 PMGSY, relevant agency/service records, and route/facility field verification | Network bounds/decay use a reviewed routable graph and functioning-service evidence; mapped POIs or straight lines are insufficient. |
| PAR-017 | Source-specific QA/uncertainty in S01–S44, shared-input groups, and S49/reference validation | Distributions/ranges require empirical or approved scenario basis and must disclose correlated sources. |
| PAR-018 | S45 original cadastral geometry plus S49 survey-grade ground controls/checkpoints | Review thresholds require the survey authority's specification and independent points; road-intersection warping alone is not authoritative. |
| PAR-019 | Effective privacy, records, court/hold, programme, and legal sources plus current legal review | Retention activates per record class/date; no source-data convenience default or indefinite audit retention. |
| PAR-020 | Preregistered pilot protocol, independent reference assessors, and S01–S50 evidence appropriate to each outcome | Accuracy/error thresholds remain open until consequence-based tolerances and a valid reference sample are approved. |
| E37–E54 specialist inputs | S18–S21 terrain, S31 soil context, S32–S40 climate/water context, optional S09/S16/S17/S37/S51/S52, and mandatory S49 site measurements/model calibration | Public products support screening only. Operational geotechnical, hydraulic, runout, InSAR, or seismic parameters require a separately commissioned and independently validated model package. |

## 6. Specialist/reference parameters

Parameters for E37–E54—soil strength, soil depth, pore pressure, root reinforcement, rainfall thresholds, hydraulic boundary conditions, curve numbers, roughness, runout coefficients, InSAR conventions, ground motion, and statistical-model coefficients—remain `UNSET` and `SPECIALIST/EXTERNAL`. They may only be activated through a separately commissioned, independently validated model with documented geographic and temporal applicability.

The transcript-attributed GLOF relation `Q_peak = 0.00077 V^1.017` is `UNVERIFIED/REJECTED AS DEFAULT`; even its unit convention and source applicability require verification.

## 7. Explicitly prohibited defaults

The following are not approved parameters: 12 m²/person as total settlement or grazing capacity; 0.05 ha/person grazing; universal 15°/20°/30° slope safety cutoffs; 5/4/8/10/12/15 km service or livelihood cutoffs; 10 km runout; 10% seasonal occupancy; 15 mm/year or 1.6 m/year relocation triggers; 115.5/200 mm rainfall, API 150, 85% saturation, or `k=.84–.88` as local instability rules; six DDMA meetings/three years; universal 24-month acquisition delay; universal 4× compensation; or any `Ω`, `Λ`, `Θ`, PPM 75/50 threshold.

## 8. Validation and promotion

Before `APPROVED`, each parameter must pass unit/domain/boundary/missing-value tests, formula-specific fixtures from [equations.md](./equations.md), and a two-person review involving the policy/domain owner and quantitative assurance. Policy scoring also requires double-counting, subgroup, rank-reversal, and sensitivity review. Promotion and rollback create audit events and identify affected scenarios; historical outputs retain their original parameter version.

## 9. References

- [Equation Registry](./equations.md)
- [Open Decisions](./open-decisions.md)
- [Operational Source Register](./source-register.md#5-operational-data-source-and-acquisition-register)
- [Jal Jeevan Mission](https://jaljeevanmission.gov.in/about_jjm)
- [SciPy MILP](https://docs.scipy.org/doc/scipy/reference/generated/scipy.optimize.milp.html)
- [OECD/JRC Handbook on Composite Indicators](https://www.oecd.org/en/publications/handbook-on-constructing-composite-indicators-methodology-and-user-guide_9789264043466-en.html)
