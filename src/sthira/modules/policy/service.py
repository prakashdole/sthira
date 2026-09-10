"""
Sthira Policy, Equations & Hard Gate Engine (ARC-C07 / C1-06).
Normative Reference: equations.md, parameters.md, rules.md (RUL-013, RUL-028, RUL-029, RUL-030, RUL-035).
"""

from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from sthira.core.contracts import HardGateResult, DimensionScore
from sthira.core.enums import GateState
from sthira.core.errors import MasterScoreProhibitedError, MissingEvidenceError


class SiteCriteriaInput(BaseModel):
    site_id: str
    hazard_susceptibility_level: str  # NONE, LOW, MODERATE, HIGH, VERY_HIGH
    in_debris_flow_runout: bool
    title_clearance_status: str  # VERIFIED_CLEAR, PENDING_DISPUTE, LITIGATION, UNKNOWN
    forest_clearance_required: bool
    fra_community_consent: Optional[bool] = None
    lean_season_tested_lpcd: Optional[float] = None  # JJM baseline 55 LPCD (RUL-030)
    has_dry_season_yield_test: bool = False
    road_access_width_m: float
    distance_to_hospital_km: float
    distance_to_school_km: float
    dwelling_capacity: int


class SiteEvaluationReport(BaseModel):
    site_id: str
    overall_gate_pass: bool
    gate_results: List[HardGateResult]
    dimension_scores: List[DimensionScore]
    summary_advisory: str


class PolicyEngine:
    """
    Evaluates mandatory hard gates and individual MCDA dimensions.
    Prohibits composite Omega scores (RUL-035).
    """

    JJ_WATER_BASELINE_LPCD = 55.0  # Jal Jeevan Mission standard (RUL-030)
    MIN_ROAD_ACCESS_WIDTH_M = 3.66  # Standard single-lane emergency road width

    def evaluate_omega_score(self, *args, **kwargs):
        """Strict enforcement of RUL-035."""
        raise MasterScoreProhibitedError()

    def evaluate_hard_gates(self, inp: SiteCriteriaInput) -> List[HardGateResult]:
        """
        Evaluate mandatory hard gates (RUL-029).
        Missing evidence produces UNKNOWN or BLOCKED (RUL-013).
        """
        results: List[HardGateResult] = []

        # Gate 1: Hazard Safety
        if inp.hazard_susceptibility_level in ("HIGH", "VERY_HIGH") or inp.in_debris_flow_runout:
            results.append(
                HardGateResult(
                    gate_id="GATE-HAZ-01",
                    gate_name="Hazard Safety Exclusion",
                    gate_group="Hazard Safety",
                    state=GateState.FAIL,
                    reason=f"Site in {inp.hazard_susceptibility_level} hazard zone or active debris flow channel.",
                )
            )
        else:
            results.append(
                HardGateResult(
                    gate_id="GATE-HAZ-01",
                    gate_name="Hazard Safety Exclusion",
                    gate_group="Hazard Safety",
                    state=GateState.PASS,
                    reason="Outside high-hazard and channelized runout zones.",
                )
            )

        # Gate 2: Legal Readiness
        if inp.title_clearance_status == "VERIFIED_CLEAR":
            if inp.forest_clearance_required and inp.fra_community_consent is None:
                results.append(
                    HardGateResult(
                        gate_id="GATE-LEG-01",
                        gate_name="Land Title & Forest Rights",
                        gate_group="Land & Legal",
                        state=GateState.BLOCKED,
                        reason="FRA Grama Sabha consultation pending on forest-adjacent parcel (RUL-025).",
                    )
                )
            else:
                results.append(
                    HardGateResult(
                        gate_id="GATE-LEG-01",
                        gate_name="Land Title & Forest Rights",
                        gate_group="Land & Legal",
                        state=GateState.PASS,
                        reason="Ownership or approved acquisition route verified.",
                    )
                )
        elif inp.title_clearance_status in ("PENDING_DISPUTE", "LITIGATION"):
            results.append(
                HardGateResult(
                    gate_id="GATE-LEG-01",
                    gate_name="Land Title & Forest Rights",
                    gate_group="Land & Legal",
                    state=GateState.BLOCKED,
                    reason=f"Title unverified or under litigation: {inp.title_clearance_status}.",
                )
            )
        else:
            results.append(
                HardGateResult(
                    gate_id="GATE-LEG-01",
                    gate_name="Land Title & Forest Rights",
                    gate_group="Land & Legal",
                    state=GateState.UNKNOWN,
                    reason="Cadastral ownership evidence missing (RUL-013).",
                )
            )

        # Gate 3: Lean-Season Water Feasibility (RUL-030, RUL-031)
        if not inp.has_dry_season_yield_test or inp.lean_season_tested_lpcd is None:
            results.append(
                HardGateResult(
                    gate_id="GATE-WAT-01",
                    gate_name="Lean-Season Water Feasibility",
                    gate_group="Water & Environment",
                    state=GateState.UNKNOWN,
                    reason="No verified lean-season hydrogeological yield test provided (RUL-031).",
                )
            )
        elif inp.lean_season_tested_lpcd < self.JJ_WATER_BASELINE_LPCD:
            results.append(
                HardGateResult(
                    gate_id="GATE-WAT-01",
                    gate_name="Lean-Season Water Feasibility",
                    gate_group="Water & Environment",
                    state=GateState.FAIL,
                    reason=f"Yield ({inp.lean_season_tested_lpcd:.1f} LPCD) below JJM baseline of {self.JJ_WATER_BASELINE_LPCD} LPCD.",
                )
            )
        else:
            results.append(
                HardGateResult(
                    gate_id="GATE-WAT-01",
                    gate_name="Lean-Season Water Feasibility",
                    gate_group="Water & Environment",
                    state=GateState.PASS,
                    reason=f"Verified lean-season yield {inp.lean_season_tested_lpcd:.1f} LPCD exceeds JJM baseline.",
                )
            )

        # Gate 4: Basic Infrastructure Access
        if inp.road_access_width_m < self.MIN_ROAD_ACCESS_WIDTH_M:
            results.append(
                HardGateResult(
                    gate_id="GATE-INF-01",
                    gate_name="Emergency Road Access",
                    gate_group="Developability & Services",
                    state=GateState.FAIL,
                    reason=f"Road width ({inp.road_access_width_m}m) below minimum emergency access ({self.MIN_ROAD_ACCESS_WIDTH_M}m).",
                )
            )
        else:
            results.append(
                HardGateResult(
                    gate_id="GATE-INF-01",
                    gate_name="Emergency Road Access",
                    gate_group="Developability & Services",
                    state=GateState.PASS,
                    reason=f"Road width {inp.road_access_width_m}m meets access requirements.",
                )
            )

        return results

    def evaluate_site(self, inp: SiteCriteriaInput) -> SiteEvaluationReport:
        """Run complete site assessment: hard gates + separate inspectable dimensions (RUL-028)."""
        gate_results = self.evaluate_hard_gates(inp)
        all_passed = all(g.state == GateState.PASS for g in gate_results)

        # Separate Dimensions (RUL-028)
        dimensions: List[DimensionScore] = [
            DimensionScore(
                dimension_id="DIM-ACC-01",
                name="Healthcare Accessibility",
                raw_value=inp.distance_to_hospital_km,
                unit="km",
                normalized_score=max(0.0, min(1.0, (20.0 - inp.distance_to_hospital_km) / 20.0)),
                weight=0.3,
                confidence=0.9,
                source_id="S41",
                contribution=max(0.0, min(1.0, (20.0 - inp.distance_to_hospital_km) / 20.0)) * 0.3,
            ),
            DimensionScore(
                dimension_id="DIM-ACC-02",
                name="Education Accessibility",
                raw_value=inp.distance_to_school_km,
                unit="km",
                normalized_score=max(0.0, min(1.0, (10.0 - inp.distance_to_school_km) / 10.0)),
                weight=0.3,
                confidence=0.9,
                source_id="S41",
                contribution=max(0.0, min(1.0, (10.0 - inp.distance_to_school_km) / 10.0)) * 0.3,
            ),
            DimensionScore(
                dimension_id="DIM-CAP-01",
                name="Dwelling Capacity",
                raw_value=float(inp.dwelling_capacity),
                unit="units",
                normalized_score=min(1.0, inp.dwelling_capacity / 200.0),
                weight=0.4,
                confidence=1.0,
                source_id="S45",
                contribution=min(1.0, inp.dwelling_capacity / 200.0) * 0.4,
            ),
        ]

        if all_passed:
            summary = "Site passed all mandatory hard gates and is COMPARABLE for advisory allocation."
        else:
            failed = [f"{g.gate_name}: {g.state.value} ({g.reason})" for g in gate_results if g.state != GateState.PASS]
            summary = f"Site is ON_HOLD / INELIGIBLE. Blocking gates: {'; '.join(failed)}"

        return SiteEvaluationReport(
            site_id=inp.site_id,
            overall_gate_pass=all_passed,
            gate_results=gate_results,
            dimension_scores=dimensions,
            summary_advisory=summary,
        )


class SensitivityResult(BaseModel):
    dimension_id: str
    dimension_name: str
    baseline_weight: float
    perturbed_weight: float
    perturbation_pct: float
    rank_reversal_detected: bool
    reversals: List[Dict[str, Any]] = Field(default_factory=list)
    max_score_delta: float = 0.0


class SensitivityAnalysisEngine:
    """
    C2-03: Sensitivity analysis and rank-instability detection (ODN-005, DEC-035).
    Tests sensitivity by perturbing dimensional weights (w +/- 20%) and checking if site relative
    comparability order reverses.
    """

    @classmethod
    def analyze_site_rank_sensitivity(
        cls,
        sites: List[SiteCriteriaInput],
        engine: PolicyEngine,
        perturbation_factor: float = 0.20,
    ) -> List[SensitivityResult]:
        if len(sites) < 2:
            return []

        # Evaluate baseline reports
        baseline_reports = [engine.evaluate_site(s) for s in sites]

        # Dimension weights
        dim_defs = [
            ("DIM-ACC-01", "Healthcare Accessibility", 0.3),
            ("DIM-ACC-02", "Education Accessibility", 0.3),
            ("DIM-CAP-01", "Dwelling Capacity", 0.4),
        ]

        def compute_aggregate_profile(report: SiteEvaluationReport, weights: Dict[str, float]) -> float:
            total = 0.0
            for d in report.dimension_scores:
                w = weights.get(d.dimension_id, d.weight)
                total += d.normalized_score * w
            return total

        baseline_weights = {d[0]: d[2] for d in dim_defs}
        baseline_scores = {r.site_id: compute_aggregate_profile(r, baseline_weights) for r in baseline_reports}
        baseline_sorted = sorted(baseline_scores.keys(), key=lambda s: baseline_scores[s], reverse=True)

        results: List[SensitivityResult] = []

        for dim_id, dim_name, base_w in dim_defs:
            for factor in (1.0 + perturbation_factor, 1.0 - perturbation_factor):
                perturbed_w = round(base_w * factor, 4)
                test_weights = dict(baseline_weights)
                test_weights[dim_id] = perturbed_w

                # Normalize remaining weights so sum equals 1.0
                other_sum = sum(v for k, v in test_weights.items() if k != dim_id)
                if other_sum > 0:
                    scale = (1.0 - perturbed_w) / other_sum
                    for k in test_weights:
                        if k != dim_id:
                            test_weights[k] = round(test_weights[k] * scale, 4)

                perturbed_scores = {r.site_id: compute_aggregate_profile(r, test_weights) for r in baseline_reports}
                perturbed_sorted = sorted(perturbed_scores.keys(), key=lambda s: perturbed_scores[s], reverse=True)

                # Check pairwise rank reversals
                reversals = []
                for i in range(len(baseline_sorted)):
                    for j in range(i + 1, len(baseline_sorted)):
                        s_a = baseline_sorted[i]
                        s_b = baseline_sorted[j]
                        # In baseline, s_a >= s_b
                        pos_a = perturbed_sorted.index(s_a)
                        pos_b = perturbed_sorted.index(s_b)
                        if pos_a > pos_b:  # S_b overtook S_a
                            reversals.append({
                                "overtaken_site": s_a,
                                "overtaking_site": s_b,
                                "baseline_scores": {s_a: baseline_scores[s_a], s_b: baseline_scores[s_b]},
                                "perturbed_scores": {s_a: perturbed_scores[s_a], s_b: perturbed_scores[s_b]},
                            })

                max_delta = max(abs(perturbed_scores[s] - baseline_scores[s]) for s in baseline_scores)

                results.append(
                    SensitivityResult(
                        dimension_id=dim_id,
                        dimension_name=dim_name,
                        baseline_weight=base_w,
                        perturbed_weight=perturbed_w,
                        perturbation_pct=round((factor - 1.0) * 100, 1),
                        rank_reversal_detected=len(reversals) > 0,
                        reversals=reversals,
                        max_score_delta=round(max_delta, 4),
                    )
                )

        return results


# Global singleton instance
policy_engine = PolicyEngine()
sensitivity_analysis_engine = SensitivityAnalysisEngine()

