---
title: PUNARVAS-AI Equation Registry
document_id: PUN-EQUATIONS
version: 1.1
status: Controlled mathematical reference
as_of: 2026-09-08
audience: Product, policy, GIS, data science, engineering, domain assurance, QA, and audit teams
owner: Policy owner and quantitative assurance lead
normative_scope: Formula classification, mathematical contracts, limitations, provenance, and validation fixtures
---

# PUNARVAS-AI Equation Registry

Prepared 8 September 2026 from the supplied mathematical handoff and source extract. This controlled project document is linked to [trd.md](./trd.md), [parameters.md](./parameters.md), [rules.md](./rules.md), and [changes.md](./changes.md).

## Read this first

The version 1.0 plans retained mathematical methods in prose but omitted most explicit formulas. This versioned registry restores the useful mathematics, supplies missing operational calculations, and records why some original equations must not be implemented unchanged.

**Including an equation is not the same as requiring its implementation.** This is a broad, project-scoped reference covering the equation families found in both chats plus the calculations missing from the plans. It is not a claim to enumerate every equation in geotechnics, hydrology or public administration.

### Status key

- **C — Core specification:** needed when the corresponding planned workflow is implemented. Product-policy formulas below are proposals, not approved law or validated science.
- **O — Optional analytical method:** retain in documentation; activate only if useful and approved.
- **R — Specialist/reference:** retain for completeness; normally ingest outputs from qualified providers rather than building the model.
- **X — Rejected/archived:** preserve the original and reason; do not execute as a decision rule.

### Registry routing and ownership

The detailed E01–E60 entries below define purpose, formula, inputs, units/conventions, assumptions, missing/zero handling, uncertainty and validation limits. This index adds implementation ownership and requirement/test routing. `C` means required only when its corresponding workflow is in scope; it does not supply an unapproved parameter.

| ID | Status | Owner | Requirement/test links |
| --- | --- | --- | --- |
| E01 | C | Quantitative assurance | FR-071–074; AT-17 |
| E02 | C | Policy/domain owners | FR-030, FR-071–074; AT-20 |
| E03 | C | GIS/hazard lead | FR-012–018; AT-01, AT-30 |
| E04 | C if estimation used | GIS/demography lead | FR-014–016; AT-17 |
| E05 | O | Demography lead | FR-014–016; AT-17 |
| E06 | C if scoring used | Policy/quantitative assurance | FR-039–041, FR-073; AT-17 |
| E07 | C if numeric priority used | Social-policy owner | FR-034–035, FR-071–074; AT-29 |
| E08 | C if ranking used | Multi-domain policy panel | FR-039–041; AT-20 |
| E09 | C/O | Accessibility/infrastructure lead | FR-026–029, FR-073 |
| E10 | O | Livelihood lead | FR-026–029, FR-042 |
| E11 | C | Water-domain lead | FR-028–029; AT-05 |
| E12 | C/O | Water-domain lead | FR-028–029; AT-05, AT-17 |
| E13 | C | Planning/site lead | FR-029, FR-073; AT-17 |
| E14 | C | Capacity/allocation owner | FR-029, FR-045; AT-15 |
| E15 | O | Livestock/ecology specialist | FR-026, FR-029; AT-17 |
| E16 | O | Programme planning | FR-026, FR-029 |
| E17 | C | Finance/programme owner | FR-064–067; AT-18 |
| E18 | O | Finance/economic appraisal | FR-064–067 |
| E19 | C | Allocation owner | FR-042–046; AT-08 |
| E20 | C | Programme/social/legal owners | FR-036–037, FR-042; AT-19 |
| E21 | C | Allocation/site owners | FR-029, FR-042; AT-08 |
| E22 | C when budget in scope | Finance/allocation owners | FR-042, FR-066–067; AT-18 |
| E23 | C when phased planning in scope | Delivery/allocation owners | FR-042, FR-068–070; AT-22 |
| E24 | O/mandatory when accommodation requires | Social/accessibility owner | FR-036, FR-042; AT-08 |
| E25 | C; policy open | Programme authority | FR-043–044; ODN-006–007 |
| E26 | C | Quantitative assurance | FR-043–046, NFR-009; AT-16 |
| E27 | O | Quantitative assurance | DEC-007; comparative test only |
| E28 | O | Policy/facilitation owner | FR-040–041; consistency/sensitivity test |
| E29 | O | Social-science/statistics owner | FR-034, FR-040; subgroup/leakage test |
| E30 | C if ranking used | Quantitative assurance | FR-041, FR-074; rank-reversal test |
| E31 | C/O | Survey/GIS authority | FR-020; AT-03 |
| E32 | C/O | Evidence/workflow owner | FR-027, FR-049–051 |
| E33 | C | Independent evaluation lead | PH-2 evaluation protocol |
| E34 | C | SRE/security/audit owners | FR-057–058, NFR-006–008; AT-27 |
| E35 | O | Remote-sensing specialist | FR-022, FR-073; AT-02, AT-17 |
| E36 | O | Remote-sensing/land reviewer | FR-022; AT-02 |
| E37 | C/O | GIS/geotechnical reviewer | FR-018, FR-026–027; AT-01 |
| E38 | O/R | Hydrology/GIS specialist | FR-026–027; specialist validation |
| E39 | R | InSAR specialist/external provider | DEC-015; specialist validation |
| E40 | R | Geotechnical specialist/external provider | DEC-015; specialist validation |
| E41 | R | Hydrogeotechnical specialist | DEC-015; specialist validation |
| E42 | R | Geotechnical/ecology specialist | FR-026–027; specialist validation |
| E43 | R/O | Meteorology/geotechnical specialist | DEC-015; specialist validation |
| E44 | R | Hazard-model authority | DEC-015; spatial/temporal holdout tests |
| E45 | R | Runout-model authority | FR-018; local calibration/validation |
| E46 | R | Runout-model authority | FR-018; local calibration/validation |
| E47 | R | Hydrology specialist | DEC-015; runoff boundary fixtures |
| E48 | R | Hydraulic specialist | DEC-015; mass/unit/conveyance tests |
| E49 | R | Hydraulic-model authority | DEC-015; conservation/benchmark tests |
| E50 | O/R | Flood-risk specialist | FR-026–027; assumption tests |
| E51 | R/O | Hydrology/telemetry specialist | Out of baseline; source-quality tests |
| E52 | R; unverified coefficient | GLOF specialist | ODN-019; no execution |
| E53 | R | Seismic/geotechnical specialist | DEC-015; specialist validation |
| E54 | O/R | Risk-analysis owner | DEC-006; scenario-probability tests |
| E55 | O/R; outside baseline | Emergency-programme owner | DEC-001; archive test |
| E56 | O; outside tactical baseline | Transport/emergency owner | DEC-001; archive test |
| E57 | X | Policy/quantitative assurance | DEC-006/023; Ω/PPM regression fixtures |
| E58 | X | Legal/policy/quantitative assurance | DEC-006/016/023; non-execution test |
| E59 | X | Scientific assurance | DEC-015/023; non-execution test |
| E60 | C boundary | Product/legal authority | RUL-001–004, RUL-072; AT-21–22 |

Sections E01–E60 are equation groups, not a count of individual equalities. ORIGINAL_MATH_EXTRACT.txt preserves original display/inline mathematics and selected calculation-code context with line numbers, including repeats and superseded formulas. Read its warning: source text contains incorrect claims.

### Required contract for every implemented equation

Record: ID/version/status; purpose; exact formula; symbol definitions; units and CRS; spatial/time support; input/source versions; permitted input domain; parameter approval/provenance/effective date; missing/zero/negative handling; output meaning; uncertainty; numerical tolerance; rounding; evidence/reviewer requirements; implementation function; requirements/features/tests. Never silently turn unknown into zero or a pass. Never select policy constants merely because an equation needs a number. Formula outputs cannot issue legal clearance or approve relocation.

# A. Core decision-support calculations

## E01 — Unit and time conversions [C]

$$Q_{L/day}=86400Q_{L/s},\qquad Q_{L/day}=1000Q_{m^3/day}$$
$$A_{m^2}=10000A_{ha},\qquad m_{tonnes}=m_{kg}/1000$$
$$\beta_{rad}=\beta_{deg}\pi/180$$

Use operating seconds/day instead of 86400 for a source available only part of the day. A measured instantaneous discharge is not automatically sustainable daily supply. Store timestamps with timezone; compute in UTC and present in local civil time. Do not use projected metres and latitude/longitude degrees interchangeably.

## E02 — Mandatory-gate eligibility [C; proposed logical contract]

$$g_{jk}\in\{PASS,FAIL,UNKNOWN,BLOCKED\}$$
$$G_j=\begin{cases}1&\text{all applicable mandatory gates have a current PASS}\\0&\text{otherwise, with the original reasons/states preserved}\end{cases}$$

A non-applicable requirement must be explicitly documented as such, not silently marked PASS. G_j is a computational permission to consider an option, not its legal status. Conditional approvals must identify which conditions block comparison, reservation, allocation or handover. Re-evaluate at the relevant action time.

## E03 — Source coverage and observed exposure [C]

For analysis area A, valid source footprint V and hazard polygon H:

$$coverage(A)=\frac{|A\cap V|}{|A|},\qquad exposedArea(A)=|A\cap H\cap V|$$

For building B_b, select and label one policy:

$$e_b^{whole}=\mathbb I(|B_b\cap H|>0)$$
$$e_b^{fraction}=\frac{|B_b\cap H|}{|B_b|}$$

The first counts an intersected building as affected; the second estimates exposed floor/footprint share, not necessarily people. Point/touch intersection versus positive area must be specified. Union same-event hazard polygons before counting. Distinct hazards remain separately reported; do not sum repeated exposure of the same person. NoData/out-of-coverage is unknown, not safe. Area zero or invalid geometry requires repair/review, not division.

## E04 — Dasymetric population estimate [C if estimation is used]

Let P_v be a reference population total for the same area/vintage. Let w_b be a nonnegative residential allocation weight, such as validated residential floor area:

$$\widehat P_b=P_v\frac{w_b}{\sum_{m\in v}w_m},\qquad \widehat P_{exposed}=\sum_{b\in v}\widehat P_b e_b$$

The chat's footprint-only equation is the special case w_b=footprint area and e_b=whole-building exposure. The weights assume how population is distributed; they do not observe people. Zero total weight, incomplete building inventory, mixed building uses and population-date mismatch invalidate unqualified estimates. Report a range under documented occupancy/weight scenarios. This must not create household identity, entitlement or beneficiary rows.

## E05 — Dwelling-based and seasonal population estimates [O]

$$\widehat P=N_{dwellings}\,\bar h\,o$$
$$\widehat P_{active,t}=\widehat P_{baseline}\,o_t$$

Mean persons per occupied dwelling is h-bar; occupancy fraction o lies in [0,1]. Buildings are not necessarily dwellings or households; multiple dwellings/households require a richer model. The original 0.10 seasonal coefficient is unsupported. Estimate o_t from suitable local evidence; darkness in nighttime imagery does not prove abandonment. If baseline already measures occupied population, do not multiply occupancy twice.

## E06 — Criterion normalization [C if MCDA/priority scoring is used]

With policy anchors L_k<U_k and known input x:

$$clip(z)=\min(1,\max(0,z))$$
$$n_k^{benefit}(x)=clip\left(\frac{x-L_k}{U_k-L_k}\right)$$
$$n_k^{cost}(x)=clip\left(\frac{U_k-x}{U_k-L_k}\right)$$

Anchors, direction and clipping are policy choices. Prefer stable, approved anchors over candidate-set minima/maxima, which can change a score merely by adding a candidate. Equal anchors or missing x produce a validation issue/unknown, not 0. No missing criterion may be dropped and its weight redistributed without an explicit approved method.

## E07 — Separate household urgency [C; proposed policy form]

$$U_i=100\sum_{k=1}^{K}\alpha_k n_{ik},\qquad \alpha_k\ge0,\quad\sum_k\alpha_k=1$$

Alternatively use approved ordered rule-based tiers instead of a numeric score. Criteria must concern the household's assessed need/urgency and lawful accommodations. Do not include candidate-site capacity, unresolved destination title or district meeting counts in U_i. Correlated needs must not be counted twice inadvertently. Document sensitive-data authority and explanations. No default weights are approved here.

If counting members with one or more vulnerability indicators:

$$p_{vulnerable}=\frac{|S_1\cup S_2\cup\cdots\cup S_K|}{P_{reference}}$$

Use the union, not an unqualified sum of overlapping groups. Aggregate denominators must refer to the same population. The chat's SC/ST+elderly+infant sum can double count and is not guaranteed to stay below one.

## E08 — Gated site suitability [C if comparative scoring is used]

For admissible site j and household/community i:

$$S_{ij}=100\sum_k w_k s_{ijk},\qquad w_k\ge0,\quad\sum_k w_k=1$$
$$contribution_{ijk}=100w_ks_{ijk}$$

All normalized s values are in [0,1]. Display criterion values and contributions; retain dimension-level results. Set S to NOT RANKED if a mandatory prerequisite fails/is unknown—do not assign zero and mix it with admissible candidates. This is a suitability comparison, not a population capacity or a combined household-risk/legal score. A dashboard of dimensions without a single final ranking is also valid.

## E09 — Route cost, distance and service accessibility [C/O]

$$d_{network}(a,b)=\min_{\pi:a\to b}\sum_{e\in\pi}l_e$$
$$t_{network}(a,b)=\min_{\pi:a\to b}\sum_{e\in\pi}(l_e/v_e+delay_e)$$
$$I_j=\sum_r w_r\exp(-t_{jr}/\tau_r),\qquad w_r\ge0,\quad\sum_r w_r=1$$

l is edge length, v is consistent speed, delay and t are time, tau>0 is an approved decay time. Distances can replace times only with consistent units and a stated interpretation. Accessibility should reflect actual network, service operation, accessibility needs and seasonality—not straight-line proximity alone. No path is UNREACHABLE; missing routing data is UNKNOWN. Neither establishes helicopter necessity. The old 10/5/12 km decay constants and 5/4/8 km cutoffs are not universal standards.

## E10 — Livelihood similarity and burden [O]

$$M_{ij}=\frac{\mathbf L_i\cdot\mathbf R_j}{\|\mathbf L_i\|\|\mathbf R_j\|}$$
$$\Delta t_{ij}=t_{new,ij}-t_{baseline,i}$$

For comparable nonnegative vectors, cosine similarity is [0,1]. A zero vector returns undefined/unknown. Occupation categories and resource categories must actually correspond; identical direction does not prove enough land, jobs or income. Use measured travel time/cost, enforce resource constraints separately, and respect stated preferences. If estimating income effects, preserve uncertainty rather than infer them from cosine similarity.

## E11 — Domestic demand and dependable water supply [C]

$$D_{domestic,j}=d_jP_{new,j}$$
$$Q_{raw,available,j}=Q_{lean,j}-Q_{existing,j}-Q_{other,j}-Q_{reserved,j}$$
$$Q_{delivered,j}=\min\{\max(0,Q_{raw,available,j})\eta_{treatment,j},Q_{treatment-cap,j},Q_{distribution-cap,j}\}(1-\ell_j)$$
$$P_{water,j}=\left\lfloor\frac{\max(0,Q_{delivered,j}-D_{new,non-domestic,j})}{d_j}\right\rfloor$$

All Q and D here are L/day on the same accounting basis. d is L/person/day; 55 LPCD may be the applicable JJM baseline, not necessarily the complete development demand. Eta and loss fraction ell require evidence. Ensure loss accounting and existing demand are not subtracted twice. An oversubscribed source is a flagged deficit, not hidden by max(0). Unknown supply, failed quality, insufficient legal rights or missing distribution evidence prevents approval even if the arithmetic passes. Livestock, fire, institutional and commercial demand need separate appropriate inputs.

Source: https://jaljeevanmission.gov.in/about_jjm

## E12 — Water adequacy, storage and reliability [C/O]

$$a_{water}=Q_{delivered}/D_{total},\qquad H_{display}=\min(1,a_{water})$$
$$V_{t+1}=V_t+(Q_{in,t}-Q_{out,t}-Q_{loss,t})\Delta t$$
$$0\le V_t\le V_{usable,max}$$
$$reliability_{observed}=\frac{N_{periods\ meeting\ service}}{N_{valid\ observed\ periods}}$$

Adequacy requires positive demand; zero demand is not an infinite score. V is volume; flows and time must match units. Model spill and unmet demand explicitly at bounds; don't silently clip them away. Include observation coverage alongside reliability. Tank size and recharge measures do not fix an unproven sustainable source. Storage autonomy V_usable/D_daily is only a simple no-inflow estimate, not a design guarantee.

## E13 — Developable land and common-unit capacity [C]

$$A_{usable,j}=\left|A_j\setminus\bigcup_k E_{jk}\right|$$
$$P_{max,j}=\min(P_{water,j},P_{sanitation,j},P_{service,j},P_{approved-layout,j},\ldots)$$

E are documented exclusion areas; use their union to avoid double-subtraction. The min formula is valid only if every term is genuinely expressed as persons under the same assumptions. Keep dwellings, households, accessible units and persons as separate constraints. Undefined mandatory capacities are unknown, not infinity.

For preliminary layout estimation only:

$$H_{land,estimate}=\left\lfloor A_{residential,net}/a_{plot}\right\rfloor$$

Here net area excludes roads/services/open space as appropriate; approved typology, setbacks, infrastructure, ownership and actual fit still control capacity. Do not replace this with A_usable/(12 m²/person). An area ratio cannot establish an approved layout.

## E14 — Capacity remaining and shared resources [C]

$$C^{remaining}_{jr}=C^{approved}_{jr}-C^{committed}_{jr}-C^{reserved}_{jr}$$
$$\sum_{i,j}a_{ijr}x_{ij}\le C^{remaining}_r$$

r indexes a resource with its own units. Commitments and reservations must be disjoint ledger states. Negative remaining capacity is an error requiring review, not a silently floored number. Use the shared-resource inequality for shared water, roads, overlapping site land or common infrastructure; per-site checks alone permit double counting. Approval must atomically recheck/reserve these amounts.

## E15 — Livestock/grazing requirement [O; expert inputs]

$$F_{annual}=\sum_a N_a f_a d_a$$
$$A_{grazing}=F_{from-grazing}/Y_{usable,annual}$$

N is animal count, f is feed dry-matter kg/animal/day, d is feeding days, Y is usable sustainable kg/ha/year after ecological/seasonal constraints. Include other feed sources and animal categories. This is a feed balance, not a universal grazing entitlement. Urban 10–12 m²/person open-space guidance cannot replace it. Source for the urban distinction: https://www.pib.gov.in/PressReleasePage.aspx?PRID=1813182

## E16 — Demand-growth and scenario envelope [O]

$$P_t=P_0(1+r)^t$$
$$D_t=d_tP_t+D_{other,t}$$

r is an approved annual scenario growth assumption and t is years. This is not a prediction of individual household growth. Run low/base/high demographic and service-demand scenarios; explain whether reserve capacity is policy or engineering design. Do not mix annual growth and monthly t.

## E17 — Cost, contingency, funding gap and entitlement [C]

$$Cost_{base}=\sum_k quantity_k\,rate_k$$
$$Cost_{total}=Cost_{base}+Contingency+Taxes+OtherCosts$$
$$Contingency=c\,Cost_{eligible-base}$$
$$FundingGap=\max(0,Cost_{required}-Funding_{confirmed,nonduplicate})$$

Rates need location, date, units, exclusions, escalation and procurement basis. Contingency/taxes must not be applied twice. Sanctioned, committed and received funding are separate—not amounts to add for the same award. Entitlement uses programme rules, not engineering cost alone. For a scheme that genuinely specifies reimbursement capped at a limit:

$$Grant_{ik}=\min(EligibleCost_{ik},Cap_{ik})$$

Use that only for such a scheme; fixed grants, stage payments and other rules differ. Keep eligibility unknown separate from zero award. No universal private-land 4× multiplier or 24-month delay. The original illustrative DPR sum was correct: ₹201,000,000 + 10% = ₹221,100,000.

## E18 — Discounted lifecycle cost [O]

$$NPV=\sum_{t=0}^{T}\frac{Cost_t-Benefit_t}{(1+r_d)^t}$$

Costs/benefits must use one price basis: nominal cash flows with nominal discounting or real with real. State whose benefits count and distributional effects. NPV cannot substitute for rights, consent or safety gates and must not impose an unapproved monetary value on lives or cultural loss.

# B. Household-to-site allocation — actual missing model

These equations are a proposed baseline, not an approved allocation policy. i=household, j=site/option, r=resource, t=phase. p_i is verified household size; x is a binary proposal. Non-site pathways need their own option/resource definitions.

## E19 — Binary decisions and unassigned outcome [C]

$$x_{ij}\in\{0,1\},\qquad u_i\in\{0,1\}$$
$$\sum_jx_{ij}+u_i=1\quad\forall i$$

A household is not split. Unassigned is a valid outcome, not automatic rejection of eligibility or need. Define the population entering the scenario and separately report households awaiting contact, assessment or programme eligibility.

## E20 — Admissibility and preference constraints [C]

$$x_{ij}\le E_i,\quad x_{ij}\le G_j,\quad x_{ij}\le A_{ij}$$

E_i means approved programme participation/eligibility for this scenario, G_j means current option gates, and A_ij means this option is acceptable under the applicable participation/preference process. These are known 0/1 derived permissions, not replacements for multi-state source records. Unknown → not allocatable yet, with reason retained. Preferences may change; freeze a version and revalidate before final action.

## E21 — Dwellings, persons and accommodation [C]

$$\sum_i x_{ij}\le H_j^{remaining}$$
$$\sum_i p_i x_{ij}\le P_j^{remaining}$$
$$\sum_i a_{ir}x_{ij}\le C_{jr}^{remaining}\quad\forall j,r$$

a_ir is household demand for a particular resource; e.g. an accessible dwelling. The first equation assumes one housing unit per household; use dwelling-type variables where this does not hold. Some households require multiple rooms/units or specific combinations, so a single accessible-unit count may not suffice. Shared resources require E14 too.

## E22 — Budget and site activation [C when in scope]

$$y_j\in\{0,1\},\quad x_{ij}\le y_j$$
$$\sum_{i,j}c_{ij}x_{ij}+\sum_j f_jy_j\le B_{available}$$

f_j is a fixed site-opening cost not already in c_ij. Avoid opening a site with no assigned households unless justified as a separate infrastructure decision. Distinguish engineering budget, eligible scheme costs and cash-flow funding; future unconfirmed grants cannot fund a current phase by assumption.

## E23 — Phasing and readiness [C when phased planning is in scope]

$$x_{ijt}\in\{0,1\},\qquad \sum_{j,t}x_{ijt}+u_i=1$$
$$x_{ijt}=0\quad\text{if site/services/funding are not ready in phase }t$$
$$\sum_i\sum_{\tau\le t}a_{ir}x_{ij\tau}\le C_{jrt}^{available}\quad\forall j,r,t$$
$$\sum_{i,j,\tau\le t}c_{ij\tau}x_{ij\tau}\le B_{\le t}^{available}$$

C and B are cumulative available amounts under a documented convention. Household occupancy is persistent unless the model explicitly supports departures; do not reset occupied capacity every phase. Count costs in their actual payment phases; add fixed-site and staged-construction costs where needed.

## E24 — Community/caregiving links [O; some are mandatory accommodations]

For a voluntarily requested, approved same-site group g:

$$z_{gj}\in\{0,1\},\quad \sum_jz_{gj}\le1,\quad x_{ij}=z_{gj}\;\forall i\in g$$

This makes the group assigned together or all unassigned; it can reduce feasibility. Use only if that is the actual request/policy. Neighbouring sites, shared phase or travel-time proximity require different constraints. Do not impose blanket community grouping or destroy individual preferences to improve a score.

For a soft pairwise separation indicator b_ik:

$$b_{ik}\ge x_{ij}-x_{kj},\quad b_{ik}\ge x_{kj}-x_{ij}\quad\forall j$$

Penalize b only if approved and define behaviour when one/both are unassigned.

## E25 — Objective ordering, ties and fairness [C; policy decision required]

A possible lexicographic structure—not an approved default—is:

$$\max F_1=\sum_i U_i(1-u_i)$$
$$\text{then maximize }F_2=\sum_i(1-u_i)$$
$$\text{then maximize }F_3=\sum_{i,j}S_{ij}x_{ij}$$
$$\text{then minimize }F_4=\sum_{i,j}c_{ij}x_{ij}+\sum_jf_jy_j$$

Alternatively use urgency tiers and maximize served households in the highest tier before lower tiers. Decide whether the first two levels should be reversed, whether to count people, and whether fairness requirements are constraints. Fix previous objective optima (within declared tolerances) before optimizing the next level. A casual weighted sum can trade away a high-priority goal; do not invent giant coefficients. Add an approved reproducible tie-break; database row order or identity spelling must not silently determine access. A lottery requires governance, seed handling and audit, not just deterministic IDs.

For evaluation, not automatic quotas:

$$serviceRate_g=\frac{\sum_{i\in g}(1-u_i)}{N_{eligible,g}}$$

Show denominators, need/capacity context and uncertainty; small groups require privacy protection. Different service rates are a diagnostic, not proof of unlawful discrimination or fairness by themselves.

## E26 — Solver diagnostics and independent validation [C]

For minimization with incumbent f_inc and lower bound b:

$$absoluteGap=f_{inc}-b\ge0$$
$$relativeGap=\frac{f_{inc}-b}{\max(1,|f_{inc}|)}$$

This is one reporting convention; preserve the solver's native gap definition too, since implementations differ. If no incumbent/bound exists, report unavailable, not zero. A feasible time-limit result is not optimal. Independently validate every proposed assignment against authoritative constraints, exact integer counts, resource units and input versions. Do not round a fractional relaxation and call it feasible. Deterministic coefficients/seed do not guarantee identical tie solutions across solver versions; pin versions and explicitly resolve ties.

Source: https://docs.scipy.org/doc/scipy/reference/generated/scipy.optimize.milp.html

## E27 — Algorithm alternatives retained for comparison [O]

Hungarian/linear assignment's basic form:

$$\min\sum_{i,j}c_{ij}x_{ij},\quad\sum_jx_{ij}=1,\quad\sum_ix_{ij}\le1$$

This is suitable for one-to-one slots under that model, not a full multi-resource settlement allocation. Slot expansion alone does not solve variable household/person/shared-resource constraints.

Hospital-residents matching requires capacities and an explicit household ranking plus site/policy priority order. Stability means **no acceptable blocking pair** in which a household prefers another site and that site has room or prefers that household to a currently assigned one. It does not guarantee all households get their favourite option or maximize urgency/fairness. Keep as an alternative for simpler constraints, not an automatically superior or impossible method. No single equation or claimed runtime makes the policy correct.

# C. Optional weighting, sensitivity and quality

## E28 — Analytic Hierarchy Process [O]

$$A\mathbf w=\lambda_{max}\mathbf w,\quad \widehat w_k=w_k/\sum_rw_r$$
$$CI=\frac{\lambda_{max}-n}{n-1},\qquad CR=CI/RI_n$$

A is a positive reciprocal comparison matrix (a_kk=1, a_kl=1/a_lk). Use the relevant random-index table; n≤2 has special consistency handling, not division by zero. CR≤0.10 is a common heuristic, not a legal or ethical validation. Weights may come from another approved method; AHP is not mandatory. Record whose judgments were used and test disagreement and sensitivity.

## E29 — PCA/SoVI-style exploratory construction [O]

$$z_{ik}=\frac{x_{ik}-\bar x_k}{s_k},\qquad C=\frac{Z^TZ}{n-1}$$
$$C\mathbf v_r=\lambda_r\mathbf v_r,\qquad PC_{ir}=\sum_kz_{ik}v_{kr}$$
$$V_i=\sum_{r\in retained}q_rPC_{ir}$$

Drop/investigate zero-variance variables; don't divide by zero. Fit on an appropriate reference sample, not evaluation data. q_r, component sign, retention and scaling require a documented methodology; this is a general SoVI-style construction, not a universally exact implementation. Variance explained is not ethical priority, causal importance or predictive validity. Small biased household samples and sensitive variables can make this inappropriate.

## E30 — Ranking and uncertainty sensitivity [C if ranking is used]

$$\Delta rank_j=rank_j(w^{scenario})-rank_j(w^{baseline})$$
$$topKFrequency_j=\frac{1}{M}\sum_{m=1}^{M}\mathbb I(rank_j^{(m)}\le K)$$
$$[L_y,U_y]=[q_{0.05}(y^{(m)}),q_{0.95}(y^{(m)})]$$

Vary justified weights, anchors, source errors and capacities while keeping gates intact. The quantiles are scenario/simulation ranges, not automatically confidence intervals or true probabilities. Dependence between inputs matters. Display unstable rank, unknown results and range overlap. Do not rerank blocked candidates as admissible through a weight perturbation.

## E31 — Georeferencing residual and overlap [C/O]

For fitted transform T of source control point s_l and independently checked reference r_l:

$$e_l=\|T(s_l)-r_l\|,\qquad RMSE_{2D}=\sqrt{\frac1n\sum_l e_l^2}$$
$$IoU(A,B)=\frac{|A\cap B|}{|A\cup B|}$$

Use an appropriate metric CRS; report x/y residuals, maxima, checkpoint locations and independent checkpoints. Fitting residuals alone can hide overfitting. A low RMSE or high IoU does not prove legal boundary agreement. Empty union is undefined. Preserve original cadastral geometry; the old exp(-shift/100m) legal multiplier is not justified.

## E32 — Evidence completeness and time [C/O]

$$completeness=\frac{N_{required\ fields\ valid}}{N_{applicable\ required\ fields}}$$
$$age=t_{decision}-t_{observation}$$
$$overdue=\max(0,t_{now}-t_{due})$$

Completeness is a workflow metric, not confidence that a claim is true; a single missing mandatory item can block even at 99%. Due dates must follow the correct programme calendar and pauses/stays, not a universal duration. Institutional overdue status triggers reminders/escalation, not inflated household hazard.

## E33 — Validation and operational metrics [C]

$$precision=\frac{TP}{TP+FP},\quad recall=\frac{TP}{TP+FN},\quad F1=\frac{2TP}{2TP+FP+FN}$$
$$MAE=\frac1n\sum_i|\hat y_i-y_i|,\qquad RMSE=\sqrt{\frac1n\sum_i(\hat y_i-y_i)^2}$$
$$Brier=\frac1n\sum_i(p_i-y_i)^2$$

For binary probability calibration, y∈{0,1}; Brier is not for arbitrary scores called probabilities. Undefined denominators require an explicit reporting convention. Use independent/spatially and temporally held-out references where appropriate; report samples and intervals. AUC alone does not establish calibration, operational usefulness or transfer to Wayanad.

$$timeReduction=100\frac{median(T_{baseline})-median(T_{assisted})}{median(T_{baseline})}$$
$$completionRate=\frac{N_{verified\ completed}}{N_{defined\ cohort}}$$

Fix comparable case mix, start/end events, observation window and missing cases. Approved allocations do not count as occupied homes. The 30% improvement figure is a target until measured.

## E34 — Availability, recovery and tamper evidence [C]

$$availability=1-\frac{unavailable\ service\ time}{eligible\ measurement\ time}$$
$$h_n=SHA256(domain\_tag\Vert h_{n-1}\Vert canonical(event_n))$$

Define outage measurement and exclusions. RPO measures tolerated data-loss interval; RTO measures restoration time, not data consistency. Hashes need canonical serialization, protected append paths and external checkpoints/signatures to resist undetected full-chain rewriting. Hash continuity is not proof facts are true and is not immutability. Minimize personal data in events and define deletion/legal-hold handling; cryptographic hashes can still be sensitive in context.

# D. Imagery and terrain — retained, with limits

## E35 — Vegetation, water and built-up indices [O; useful discrepancy tools]

$$NDVI=\frac{NIR-Red}{NIR+Red}$$
$$NDWI_{McFeeters}=\frac{Green-NIR}{Green+NIR}$$
$$NDBI=\frac{SWIR-NIR}{SWIR+NIR}$$
$$MNDWI=\frac{Green-SWIR}{Green+SWIR}$$

For Sentinel-2, common choices are Red=B4, Green=B3, NIR=B8 (10 m), SWIR=B11 (20 m). Aggregate/align to a justified common support; interpolating B11 to 10 m does not create 10 m SWIR detail. Use appropriate reflectance, QA/cloud/shadow masks and atmospheric/sensor handling. Reject tiny denominators; do not hide invalid values with an unexplained epsilon. The nominal [-1,1] range assumes suitable nonnegative reflectances; preprocessing artifacts/negative values need flags. Specify which NDWI—different indices share that name.

Positive NDBI may be bare soil, not a house. NDWI<0.1 over a few scenes cannot establish no seasonal flooding. None of these proves occupancy, title, safe drinking water or a beneficiary's existence.

Sources: https://developers.google.com/earth-engine/datasets/catalog/COPERNICUS_S2_SR_HARMONIZED and https://developers.google.com/earth-engine/datasets/catalog/GOOGLE_Research_open-buildings_v3_polygons

## E36 — Parcel discrepancy and temporal change [O]

$$builtFraction_j=\frac{|B_{detected}\cap A_j\cap V|}{|A_j\cap V|}$$
$$\Delta builtFraction_j=builtFraction_{j,t_2}-builtFraction_{j,t_1}$$

Require valid coverage, consistent resolution, classification definition, season and dates. Track error and independent validation; a fraction is not a legal-clearance probability. Distinguish any structure, built surface, residential use and actual occupation. A discrepancy opens review; it does not automatically reject a parcel or displace existing occupants.

## E37 — Slope and topographical usability [C/O]

$$\beta=\arctan\sqrt{(\partial z/\partial x)^2+(\partial z/\partial y)^2}$$
$$T_j=\frac{|A_j\cap\{\beta<\beta_{policy}\}|}{|A_j|}$$

z is elevation, not soil depth. Gradients require matched horizontal/vertical units and justified DEM resolution. The area ratio measures slope-qualified area, not contiguity or constructability; connected-component/layout analysis is separate. Local flatness does not exclude upstream runout. No universal 15° or 20° safety threshold is approved.

## E38 — Topographic Wetness Index [O/R]

$$TWI=\ln\left(\frac{a}{\tan\beta}\right)$$

a is specific upslope contributing area (area per contour length), not simply raw pixel count. This conventional index is unit/resolution/method-dependent; state units and DEM flow routing. Near-zero slope, sinks, flat areas and NoData need documented handling; arbitrary clipping changes the model. TWI is a terrain proxy, not groundwater observation or a flood-depth map; adding it does not automatically improve accuracy.

## E39 — InSAR displacement and velocity [R]

Under an explicitly chosen phase sign convention:

$$\Delta d_{LOS}=s\frac{\lambda}{4\pi}\Delta\phi,\qquad s\in\{-1,1\}$$
$$\widehat v_{LOS}=\frac{\sum_t(t-\bar t)(d_t-\bar d)}{\sum_t(t-\bar t)^2}$$

Lambda is radar wavelength, phase is radians and displacement is along line of sight. Unwrapping, atmosphere, orbit, coherence, reference motion and uncertainty require expert processing. LOS velocity is not automatically vertical subsidence or slope-parallel motion. A velocity-class boundary is not a relocation threshold. Do not normalize all landslides by 1.6 m/year or use a universal 15 mm/year red-zone trigger.

# E. Specialist hazard equations — document, normally do not build

These are scientific reference models, not parcel safety certificates. Use existing qualified hazard products where suitable. If local modelling is commissioned, a domain specialist must define the exact model, boundary/initial conditions, parameters, calibration, validation, uncertainty and approval. No empirical coefficient below becomes a universal Indian parameter.

## E40 — Shear strength and factor of safety [R]

$$\tau_{resist}=c'+(\sigma_n-u_p)\tan\phi'$$
$$F_s=\frac{c'+(\sigma_n-u_p)\tan\phi'}{\tau_{drive}}$$

For a simplified infinite slope with **vertical** soil depth z, unit weight gamma, slope beta and pore pressure u_p:

$$\sigma_n=\gamma z\cos^2\beta,\quad\tau_{drive}=\gamma z\sin\beta\cos\beta$$
$$F_s=\frac{\tan\phi'}{\tan\beta}+\frac{c'-u_p\tan\phi'}{\gamma z\sin\beta\cos\beta}$$
$$u_p=\gamma_w\psi$$

c', sigma and u are pressure (e.g. kPa), gamma is kN/m³, z/pressure head psi are metres, angles use consistent trig units. Soil depth and strength are not supplied by a DEM/lithology label alone. The original h_w formula can be compatible with a particular slope-parallel water-table convention; do not interchange h_w and pressure head without deriving the geometry. Uniform saturation, infinite-slope geometry and pore-pressure assumptions have limitations. Fs<1 means modelled resistance below driving stress—not observed active failure. Flat slopes/zero depth require a different treatment, not numerical epsilon certification.

Source: https://pubs.usgs.gov/of/2008/1159/

## E41 — Richards flow and reduced diffusion [R]

A general isotropic head form, with elevation z_e increasing upward:

$$\frac{\partial\theta}{\partial t}=\nabla\cdot[K(\psi)\nabla(\psi+z_e)]$$

A reduced, locally linearized one-dimensional diffusion representation:

$$\frac{\partial\psi}{\partial t}=D_{eff}\frac{\partial^2\psi}{\partial Z^2}$$

Theta is water content, K hydraulic conductivity (length/time), psi head, D_eff diffusivity (length²/time). This reduced equation is **not the full Richards equation**. The chats' versions with and without cos²(beta) depend on coordinates and model derivation; do not arbitrarily choose or combine them. Use the precise TRIGRS/Iverson implementation and its assumptions. Initial moisture/pressure, infiltration versus runoff, basal/lateral boundaries and rainfall forcing are required. Rainfall alone does not supply soil parameters or pore pressure.

Source: https://pubs.usgs.gov/of/2008/1159/

## E42 — Root reinforcement and post-construction stability [R]

$$c_r=k\sum_n t_{r,n}RAR_n$$
$$c'_{effective,t}=c'_{soil}+c_{r,t}$$
$$F_{s,post}=\frac{c'_{soil}+c_{r,post}+(\sigma_{n,post}-u_{post})\tan\phi'}{\tau_{drive,post}}$$

Root tensile strength t is pressure, RAR is dimensionless root-area ratio and k represents geometry/model assumptions. Do not subtract root cohesion twice if c' already excludes roots. Post-construction stresses/pore pressures need an actual engineering model; simply increasing soil depth to stand for any foundation loading is not generally valid. Wu–Waldron-type simultaneous reinforcement may overestimate strength because roots break progressively. Keep the old “surcharge minus cohesion loss” only in the rejected archive.

Sources: https://www.mdpi.com/2076-3263/11/5/212 and https://agupubs.onlinelibrary.wiley.com/doi/full/10.1029/2004WR003801

## E43 — Antecedent rainfall and intensity-duration [R/O]

$$API_t=P_t+kAPI_{t-1},\qquad0\le k<1$$
$$I_{threshold}=aD^b$$

P/API use rainfall-depth units and a fixed timestep; D and I use explicitly stated duration/intensity units. The decay factor k and intensity-duration a,b must be calibrated to appropriate local event/non-event evidence. Missing rainfall cannot mean zero. API is a proxy, not measured saturation. No universal 115.5/200 mm, API150, 85% saturation or k=.84–.88 local instability threshold follows from these equations. Meteorological rainfall categories are not legal relocation triggers.

## E44 — Frequency ratio and logistic susceptibility [R]

$$FR_c=\frac{n_{landslide,c}/n_{landslide,total}}{n_{area,c}/n_{area,total}}$$
$$LSI(s)=\sum_kFR_{class(k,s)}$$
$$p(s)=\frac{1}{1+\exp[-(b_0+\sum_kb_kx_k(s))]}$$

FR/LSI are relative associations, not automatically calibrated failure probabilities. Define inventory completeness, correlated predictors, background sampling and zero-count/smoothing rules. Logistic probabilities fitted on artificially balanced case-control data are not calibrated prevalence probabilities without appropriate adjustment/validation. Use spatial/time holdouts and external validation; susceptibility is not a forecast time or a compulsory-relocation boundary.

## E45 — Voellmy runout resistance [R]

$$\tau=\mu N+\frac{\rho g\|\mathbf v\|^2}{\xi},\qquad N=\rho gh\cos\beta$$

Tau and N are Pa, rho kg/m³, h metres, velocity m/s, mu dimensionless and xi m/s² in this convention. A friction law alone does not calculate a runout polygon; mass/momentum, terrain, release volume and calibrated rheology are also needed. Do not import another Indian event's mu/xi as locally validated.

Optional yield-stress extension documented by RAMMS:

$$\tau=\mu N+\frac{\rho g\|\mathbf v\|^2}{\xi}+(1-\mu)N_0[1-\exp(-N/N_0)]$$

For N0>0, with the model's defined yield-stress units. Optional curvature contribution in compatible terrain coordinates:

$$a_{curvature}=\mathbf v^TK\mathbf v,\qquad\Delta N=\rho h a_{curvature}$$

K is the model's curvature tensor; implementation sign/contact constraints matter. These less-important extensions belong to the selected specialist model, not improvised parcel scoring.

Source: https://ramms.ch/ramms-debrisflow/friction-parameters/

## E46 — Travel angle [R; empirical screening only]

$$\alpha_E=\arctan\left(\frac{z_{source}-z_{endpoint}}{L_{horizontal}}\right)$$

Specify the source point and horizontal-distance definition. Travel-angle envelopes need local calibration and terrain/path treatment. Do not equate tan(alpha_E) to Voellmy mu as a universal exclusion law, use a universal 10 km corridor, or mark everything outside a cone safe. Use authoritative runout products or qualified modelling.

## E47 — SCS/NRCS event runoff [R]

$$S=25400/CN-254\quad(\mathrm{mm}),\qquad I_a=\lambda_aS$$
$$Q_{runoff}=\begin{cases}0&P\le I_a\\\frac{(P-I_a)^2}{P-I_a+S}&P>I_a\end{cases}$$
$$CN_{composite}=\frac{\sum_kA_kCN_k}{\sum_kA_k}$$

P and Q are event depths in mm; 0<CN≤100. Lambda_a=.2 is the classic default, not an unchangeable physical constant. Use compatible calibrated CN/abstraction choices; the method is for event simulation. Do not double count directly connected impervious areas in both composite CN and a separate fraction. Runoff depth is **not inundation depth or river discharge**; those need routing and terrain/hydraulics.

Source: https://www.hec.usace.army.mil/confluence/hmsdocs/hmstrm/canopy-surface-infiltration-and-runoff-volume/infiltration/scs-curve-number-loss-model

## E48 — Runoff volume, discharge and Manning conveyance [R]

$$V_{runoff,m^3}=Q_{runoff,mm}A_{m^2}/1000$$
$$Q_{discharge}=A_{flow}v$$
$$Q_{Manning}=\frac1nA_{flow}R_h^{2/3}S_f^{1/2}$$

Discharge is m³/s in SI, hydraulic radius R_h=A_flow/wetted perimeter, n is the Manning coefficient and S_f is energy slope. Q_discharge is not SCS runoff depth. Manning's steady/uniform-flow approximation is not a complete rapidly varying flood/runout model. Channel geometry, roughness, blockage and boundary evidence are required. Do not infer water depth simply by spreading runoff volume uniformly over a site.

## E49 — Two-dimensional shallow-water equations [R]

A simplified clear-water depth-averaged form on fixed bed z_b, neglecting wind, Coriolis and turbulence terms:

$$\frac{\partial h}{\partial t}+\frac{\partial(hu)}{\partial x}+\frac{\partial(hv)}{\partial y}=R-I$$
$$\frac{\partial(hu)}{\partial t}+\frac{\partial(hu^2+gh^2/2)}{\partial x}+\frac{\partial(huv)}{\partial y}=-gh\frac{\partial z_b}{\partial x}-\tau_{bx}/\rho$$
$$\frac{\partial(hv)}{\partial t}+\frac{\partial(huv)}{\partial x}+\frac{\partial(hv^2+gh^2/2)}{\partial y}=-gh\frac{\partial z_b}{\partial y}-\tau_{by}/\rho$$

h is depth (m), u/v horizontal velocities (m/s), R/I rainfall/infiltration rates (m/s), tau_b bed shear. Source/sink momentum treatment must be specified; above assumes no added horizontal source momentum and omits other source terms. A production hydraulic solver needs wetting/drying, mesh, boundaries, numerical stability, structures and conservation tests. HEC-RAS can use shallow-water **or diffusion-wave** formulations; do not assert every provider run uses the same equations. Do not build this from scratch for the MVP.

Source: https://www.hec.usace.army.mil/confluence/rasdocs/r2dum/latest/running-a-model-with-2d-flow-areas/shallow-water-or-diffusion-wave-equations

## E50 — Return periods and annual exceedance [O/R]

$$p_{annual}=1/T$$
$$P(\text{at least one exceedance in }n\text{ years})=1-(1-1/T)^n$$

This standard relationship assumes the specified stationary independent annual-event model. A 100-year event is not one event exactly every 100 years. Nonstationarity, multiple mechanisms, flood-map uncertainty, blockage and freeboard matter. Being above a mapped H100 elevation alone does not certify site safety.

## E51 — River stage trend [R/O]

$$\widehat{dH/dt}=\frac{H_t-H_{t-\Delta t}}{\Delta t}$$

Use a consistent gauge/datum/time basis, quality flags and uncertainty. Warning/danger levels are station-specific; they do not uniformly mean imminent embankment overtopping. The chats' H_d−0.5 m, 0.3 m/hour and 15 km downstream thresholds are unapproved. Stage trend is not a flood footprint or relocation instruction.

## E52 — GLOF peak-discharge family [R; original coefficient unverified]

$$Q_{peak}=aV^b$$

V is outburst volume and a's units depend on b and the unit convention. The chat gives **Q_peak=0.00077 V^1.017**, attributed to Huggel/Popov. Retain that exact original in the archive, but its coefficient, attribution, dam type, unit convention and applicable sample were not adequately verified here. It is not an executable default. A peak estimate does not provide a breach hydrograph, travel time or downstream inundation. No glacial-lake module is justified for Wayanad merely to add equations; retain it for a separately scoped Himalayan extension.

## E53 — Seismic slope displacement [R]

For the simplified Newmark sliding-block model on a statically stable slope:

$$a_c=(F_s-1)g\sin\alpha\quad(F_s>1)$$
$$I_A=\frac{\pi}{2g}\int_0^T a(t)^2dt$$

Acceleration a_c is m/s²; a_c/g is dimensionless. Arias intensity I_A is m/s when a(t) is m/s². Given correctly resolved driving acceleration a_d(t), a simple one-direction no-reverse sliding model uses:

$$\dot v=\begin{cases}a_d-a_c&v>0\\\max(0,a_d-a_c)&v=0\end{cases},\qquad v\ge0,\qquad D_N=\int v(t)dt$$

Integrate with stopping events so v never becomes negative. Statically unstable slopes (Fs≤1) require different assessment. Contrary to the chat's “no new inputs needed,” displacement requires ground motion or validated predictors plus model assumptions, not Fs alone. Jibson regressions have model-specific coefficients, units, error and validity ranges; select and verify a published version, not improvised constants. Newmark displacement is not an earthquake-radius or authoritative zone formula.

Sources: https://pubs.usgs.gov/publication/70029910 and https://pubs.usgs.gov/of/1998/ofr-98-113/ofr98-113.html

## E54 — Risk as expected loss, not an all-purpose score [O/R]

For mutually exclusive hazard scenarios s over a defined time horizon:

$$\mathbb E[L]=\sum_s p_s\,\mathbb E[L\mid s]$$

Use calibrated probabilities and a stated loss measure; dependent/overlapping scenarios cannot be naively added. Keep exposure, vulnerability and coping capacity explicit in the underlying model. UNDRR defines risk probabilistically as a function of those factors; it does **not** prescribe the transcript's exact H×E×V/C or Ω. Such a ratio may be a conceptual illustration, not a validated household/site scoring formula.

Source: https://www.undrr.org/terminology/disaster-risk

# F. Low-priority/emergency calculations preserved, not reintroduced into scope

## E55 — Emergency supply arithmetic [O/R; outside permanent-relocation baseline]

$$Water_{L}=P\,d_{water}\,days$$
$$Energy_{kcal}=P\,e_{daily}\,days$$
$$MealPackets=\lceil P\,mealsPerDay\,days\rceil$$
$$ShelterUnits=\lceil Households\,(1+reserveFraction)\rceil$$
$$Payload_{kg}=\sum_kquantity_k\,unitMass_k+PackagingMass$$

Parameters require a current humanitarian/clinical/logistics assessment; no universal water/ration/medical-kit norm is endorsed here. Drinking water is not all domestic water. Tablets cannot be added to litres as if the units match. The original 72-hour code `pop*3` provides only one day's packets if 3 meals/person/day was intended; `int(...)` truncates supplies; its payload omitted some listed supplies. Air operations require trained operational assessment, not a population formula. Preserve the method but keep this module out of scope unless explicitly approved.

## E56 — Road reachability, not blocked-edge counting [O; outside tactical baseline]

$$reachable(a,b)=\mathbb I(\exists\text{ admissible path from }a\text{ to }b)$$

Remove/qualify edges using verified closures, vehicle constraints and time. `blocked_count>0` does not prove all access is severed or that an airdrop is necessary. Generic truck-width/gradient/bridge/load/helipad thresholds in the chats are not universal approvals. A binary path result also cannot certify a route safe for civilian evacuation.

# G. Rejected originals and replacement map

## E57 — Original PPM and composite carrying-capacity score [X as supplied; corrected concepts in E07/E08/E11–E14]

Original PPM:

$$PPM=0.40H+0.25(P_{affected}/Capacity_{max})+0.20(1-A)+0.15V$$

Arithmetic-only scale repair:

$$PPM_{arith}=100[0.40H+0.25clip(P_{affected}/Capacity_{max})+0.20(1-A)+0.15V]$$

The repair needs positive capacity and all other terms in [0,1], but **does not fix the conceptual mixing of household need and destination shortage**. Retain as historical, not the recommended urgency algorithm. The 75/50 phase cutoffs and 0–6 month/1–2 year/3–5 year windows lack an approved governing policy.

Original carrying-capacity comparison:

$$CC=0.25T+0.30H+0.20I+0.25L$$
$$H=\min(1,Q/(P\cdot70))$$
$$L=0.6\mathbb I(SoilMatch)+0.4\min(1,A_{commons}/(P\cdot0.05ha))$$

Later version:

$$C_{ij}=\min\{1,0.30[A_{usable}/(P_i\cdot12m^2)]+0.35[Q_{dry}/(P_i\cdot55LPCD)]+0.20I+0.15M_{ij}\}$$

A weighted suitability score is not a number of people a site can support. Uncapped component ratios can compensate for shortages even with an outer min. Water, land and other critical resource bounds must be separate. 70 is not the general JJM rural baseline; 55 still needs applicability. Neither 0.05 ha/person nor 12 m²/person is a verified universal grazing/settlement capacity. The original exponential road/power/PHC sum is retained via E09 with approved parameters; original topographical area ratio via E37.

## E58 — Ω, legal multiplier and institutional multiplier [X]

$$\Omega_{ij}=clip_{[0,100]}\left(100\sqrt{\frac{H_iE_iV_i}{\epsilon+C_{ij}}}\,\Lambda_j\Theta_i\right)$$
$$\Lambda_j=(1-\delta_{FRA})\left[1-\min\left(1,\frac{A_{NDBI,built}}{A_{RoR,vacant}}\right)\right]\exp(-\Delta_{shift}/100m)$$
$$\Theta_i=1+0.25\max(0,6-N_{DDMA,3yr})$$

Reasons to reject:

- Maximizing Ω can prefer lower-capacity sites because C is in the denominator.
- Destination legal difficulty should not make a household's need disappear; Lambda→0 forces Ω→0.
- Satellite built-up fractions/grid shift are not legal probabilities or title evidence. Zero cadastral-vacant denominator is undefined.
- FRA/forest/community rights require the actual applicable legal pathway, not a blanket permanent zero.
- The six-meetings-per-three-years baseline is not a universal statutory formula.
- Clipping/square roots/epsilon avoid some numerical overflow but do not confer conceptual validity, fairness or calibration.
- The expanded original formula and `Cost=100−Ω` preserve the same defects; changing assignment software cannot repair the score.

Replace with E02, E07, E08, E14, E19–E26 and E32. Keep the fully expanded original in ORIGINAL_MATH_EXTRACT.txt for provenance, not implementation.

## E59 — Original hazard mash-up and unsupported shortcuts [X]

$$H_i=\min\{1,\max[1-\min_tF_s,\;v_{InSAR}/v_{crit},\;\mathbb I_{runout}]\}$$
$$E_i=\min(1,P_{affected}/1000)$$
$$V_i=0.45(N_{kutcha}/N_{total})+0.35[(P_{SC/ST}+P_{elderly}+P_{infants})/P_{affected}]+0.20(1-A)$$

These combine quantities with incompatible meanings; 1000 people and velocity reference are arbitrary; vulnerability groups overlap; zero/unknown denominators, LOS sign/units and incomplete coverage can break the claimed bounds. A binary runout indicator can flatten meaningful scenario severity into H=1. Preserve individual evidence instead of calling this a calibrated multi-hazard probability.

Other archived originals:

$$R_{seismic}=10^{0.5M-1}\;km$$
$$\Delta\tau=AddedSurcharge-RootCohesionLoss$$
$$P_{active}=0.10P_{baseline}$$

Do not execute: unsupported seismic radius; conflated mechanical terms; unsourced seasonal factor. Use E53, E42 and E05 respectively if those modules are actually commissioned. Replace “GREEN otherwise” with explicit unknown/coverage/authority handling. Fixed InSAR, slope, flood depth, water-index, rainfall, runout-angle, distance and legal-cost/time thresholds all need an authority/domain source plus local applicability; being later in idea.txt does not validate them.

## E60 — What an equation cannot decide [C boundary]

$$AnalyticalOutput\ne LegalApproval\ne InformedAcceptance\ne CompletedRelocation$$

This is a conceptual distinction, not a computational formula. Court stays, rights, safe occupation, consent, compensation, programme eligibility and notification require evidence and competent process. Human review is not a magic cure for an invalid model; the math must still be valid, interpretable and testable.

# H. Traceability back to the chats

Line references use the supplied originals; repeated equations remain in the extract.

| Original family | Original location | Treatment here |
|---|---|---|
| Fs; slope; red/orange/green | idea 179–200; claude 137–142,197 | E37/E40 + rejected thresholds E59 |
| Dasymetric exposure | idea 227–234,2133–2139; claude 145 | E03/E04 |
| CC/topography/water/proximity/livelihood | idea 238–267,1362–1367,2149–2158; claude 149–153 | E08–E15/E37/E57 |
| PPM and action phases | idea 271–283; claude 155–159 | E07/E23/E57 |
| Seismic radius in code and later prose | idea 338–349,1752–1755; claude 164–166 | E53/E59 |
| Water conversion in SQL | idea 395–400 | E01/E11 |
| Supply arithmetic, mass conversion and emergency pack | idea 460–490,1073–1077,1441–1445; claude 169–172 | E55 |
| Dwelling population and seasonal coefficient | idea 1001,1161; claude 180 | E05 |
| Added-load/root-loss shortcut | idea 1383; claude 176–177 | E42/E59 |
| API, rainfall, earthquake and gauge tripwires | idea 1727–1766 | E43/E51/E59 |
| Iverson diffusion and transient Fs | idea 2105,2125–2129; claude 200–203 | E40/E41 |
| Voellmy/Heim | idea 2106,2131,2344; claude 230–234 | E45/E46 |
| Combined H/E/V | idea 2117–2147 | E04/E07/E54/E59 |
| Lambda/Theta | idea 2160–2182 | E02/E31/E32/E58 |
| Omega, expanded form, limit and Hungarian cost | idea 2186–2248 | E19–E27/E58 |
| Land-category multipliers; capacity/space/distance cutoffs | idea 2296–2415 | E02/E11–E15/E59 |
| Wu–Waldron | claude 205–209 | E42 |
| Rainfall intensity-duration | claude 211–213 | E43 |
| Frequency ratio/logistic | claude 218–225 | E44 |
| GLOF empirical peak | claude 239–243 | E52; coefficient remains unverified |
| Risk ratio | idea 2108; claude 246–250 | E54/E58 |
| AHP/weighted sum | claude 255–264 | E08/E28 |
| NDVI/NDWI/NDBI | claude 290–294 | E35/E36 |
| PCA/SoVI (described, not fully formulated) | claude 296–300 | E29 |
| TWI | claude 305–308 | E38 |
| SCS runoff | claude 310–313 | E47/E48 |
| Newmark/Jibson | claude 315–318 | E53 |
| Hospital-residents | claude 320–321 | E27 |
| Shallow water (named, not written out) | claude 323–324 | E49 |
| Missing operational mathematics in MD plans | trd FR-027–045, FR-056–061, NFRs | E01–E34 |

# I. Minimum fixtures before implementation

1. **Scale:** normalized all-one weighted inputs yield 100, not 1, on a 0–100 display. Unknown inputs do not become zeros or redistribute weights silently.
2. **Gate:** excellent transport cannot compensate for failed/unknown legal or water gate.
3. **Original Ω regression:** H=.25, E=.5, V=.4, epsilon=.01 and Lambda=Theta=1 give Ω≈43.85 at C=.25 but Ω≈22.25 at C=1. This proves the bad direction when maximizing Ω.
4. **Water:** 1 L/s operating continuously equals 86,400 L/day. After a single 20% delivery loss it is 69,120 L/day, at most floor(69120/55)=1,256 people before other demands/constraints. This arithmetic does not verify sustainability or quality.
5. **Exposure:** zero footprint denominator returns unknown; two overlapping hazard polygons do not double-count the same building/person.
6. **People:** overlapping vulnerability categories use union or another explicitly approved non-double-counting construction.
7. **Capacity:** site limit two dwellings/five people can accept a three-person and a two-person household, but not two three-person households. Never split the latter household.
8. **Approval race:** two scenarios cannot reserve the same final dwelling/shared supply; one must fail/replan transactionally.
9. **Preferences:** an unacceptable site is never assigned simply because it improves coverage. Infeasibility/unassigned remains visible.
10. **Runoff:** for CN=80, S=63.5 mm and Ia=12.7 mm; rainfall 10 mm gives runoff 0, not a positive value from squaring P−Ia.
11. **Index:** zero spectral denominator is invalid; a resampled SWIR raster does not acquire new spatial detail; no NDBI threshold sets ownership/occupation.
12. **Numerics:** domain errors, NaN, infinity, singular matrices, empty geometry, wrong CRS and kg/L/person/dwelling mixups are rejected, not coerced.
13. **Solver:** validate a feasible-but-not-optimal time-limit result distinctly from optimal; infeasible results contain no invented assignment. Manually edited proposals must pass the same validator.
14. **History:** a frozen scenario can be reproduced or explained under pinned inputs/policy/solver; changed preferences, evidence expiry or court stay invalidates progression when required.
15. **Scope:** specialist/rejected equations are never imported into active policy merely because this reference includes them.

## Governance rule

Do not remove these mathematical families silently. A change must explicitly mark the affected group `CORE`, `OPTIONAL`, `SPECIALIST/EXTERNAL`, or `REJECTED WITH REPLACEMENT` and update its requirement/test links. Only approved, validated, in-scope calculations enter an executable specification. The presence of equations does not make the platform scientifically or legally validated.
