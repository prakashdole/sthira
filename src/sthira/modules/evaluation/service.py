"""
Sthira Preregistered Evaluation & Benchmark Harness (R2-03 / phases.md §5).
Normative Reference: rules.md (RUL-001, RUL-013, RUL-028, RUL-035), DEC-035, ODN-016.
"""

from enum import Enum
import hashlib
import json
import statistics
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from sthira.core.contracts import utc_now
from sthira.core.audit import global_audit_ledger


class EvaluationTargetStatus(str, Enum):
    MET = "MET"
    UNMET = "UNMET"
    INCONCLUSIVE = "INCONCLUSIVE"


class DossierTimeComparison(BaseModel):
    case_id: str
    baseline_human_hours: float
    shadow_system_hours: float
    hours_saved: float
    pct_reduction: float


class SubgroupFairnessMetrics(BaseModel):
    subgroup_name: str  # "PWD_OR_ELDERLY", "FEMALE_HEADED", "MARGINALIZED_COMMUNITY"
    total_cases: int
    eligible_count: int
    assigned_count: int
    unassigned_count: int
    assignment_rate: float
    disparity_vs_overall: float


class CaseShadowEvaluation(BaseModel):
    case_id: str
    official_ground_truth_necessity: str  # "RELOCATION_NECESSARY", "IN_SITU_SAFE", "DISPUTED"
    system_advisory_necessity: str  # "RELOCATION_NECESSARY", "IN_SITU_SAFE", "UNKNOWN"
    has_unknown_or_held: bool = False
    baseline_hours: float
    shadow_hours: float
    household_tags: List[str] = Field(default_factory=list)  # e.g., ["PWD", "FEMALE_HEADED"]
    assigned_site_id: Optional[str] = None


class ShadowComparisonMetrics(BaseModel):
    total_cases_evaluated: int
    true_positives: int
    true_negatives: int
    false_positives: int
    false_negatives: int
    unknown_or_held_cases: int
    median_baseline_hours: float
    median_shadow_hours: float
    median_time_reduction_pct: float
    target_30_pct_status: EvaluationTargetStatus
    subgroup_fairness: List[SubgroupFairnessMetrics]
    evaluated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    sha256_checksum: str


class EvaluationHarnessService:
    """
    R2-03: Preregistered evaluation protocol and benchmark reporter.
    Validates shadow pilot results against official human baselines without tuning after inspection.
    """

    def __init__(self):
        self._case_evaluations: List[CaseShadowEvaluation] = []

    def record_case(self, case: CaseShadowEvaluation):
        self._case_evaluations.append(case)

    def calculate_metrics(self, actor_id: str = "EVALUATION_BOARD") -> ShadowComparisonMetrics:
        cases = self._case_evaluations
        if not cases:
            return ShadowComparisonMetrics(
                total_cases_evaluated=0,
                true_positives=0,
                true_negatives=0,
                false_positives=0,
                false_negatives=0,
                unknown_or_held_cases=0,
                median_baseline_hours=0.0,
                median_shadow_hours=0.0,
                median_time_reduction_pct=0.0,
                target_30_pct_status=EvaluationTargetStatus.INCONCLUSIVE,
                subgroup_fairness=[],
                sha256_checksum="empty",
            )

        tp = 0
        tn = 0
        fp = 0
        fn = 0
        unknown_held = 0

        baseline_times = []
        shadow_times = []

        for c in cases:
            baseline_times.append(c.baseline_hours)
            shadow_times.append(c.shadow_hours)

            if c.has_unknown_or_held or c.system_advisory_necessity == "UNKNOWN":
                unknown_held += 1
                continue

            sys_reloc = c.system_advisory_necessity == "RELOCATION_NECESSARY"
            gt_reloc = c.official_ground_truth_necessity == "RELOCATION_NECESSARY"

            if sys_reloc and gt_reloc:
                tp += 1
            elif not sys_reloc and not gt_reloc:
                tn += 1
            elif sys_reloc and not gt_reloc:
                fp += 1
            elif not sys_reloc and gt_reloc:
                fn += 1

        med_base = statistics.median(baseline_times) if baseline_times else 0.0
        med_shad = statistics.median(shadow_times) if shadow_times else 0.0
        if med_base > 0:
            reduction_pct = round(((med_base - med_shad) / med_base) * 100.0, 1)
        else:
            reduction_pct = 0.0

        if len(cases) < 5:
            target_status = EvaluationTargetStatus.INCONCLUSIVE
        elif reduction_pct >= 30.0:
            target_status = EvaluationTargetStatus.MET
        else:
            target_status = EvaluationTargetStatus.UNMET

        # Subgroup fairness
        subgroups = ["PWD_OR_ELDERLY", "FEMALE_HEADED", "MARGINALIZED_COMMUNITY"]
        overall_assigned_rate = (
            sum(1 for c in cases if c.assigned_site_id is not None) / len(cases)
            if cases else 0.0
        )
        fairness_list: List[SubgroupFairnessMetrics] = []

        for sg in subgroups:
            sg_cases = [c for c in cases if sg in c.household_tags]
            if not sg_cases:
                continue
            assigned = sum(1 for c in sg_cases if c.assigned_site_id is not None)
            unassigned = len(sg_cases) - assigned
            rate = round(assigned / len(sg_cases), 3)
            disparity = round(rate - overall_assigned_rate, 3)

            fairness_list.append(
                SubgroupFairnessMetrics(
                    subgroup_name=sg,
                    total_cases=len(sg_cases),
                    eligible_count=len(sg_cases),
                    assigned_count=assigned,
                    unassigned_count=unassigned,
                    assignment_rate=rate,
                    disparity_vs_overall=disparity,
                )
            )

        payload = {
            "total_cases": len(cases),
            "tp": tp,
            "tn": tn,
            "fp": fp,
            "fn": fn,
            "unknown_held": unknown_held,
            "median_baseline": med_base,
            "median_shadow": med_shad,
            "reduction_pct": reduction_pct,
            "target_status": target_status.value,
        }
        checksum = hashlib.sha256(json.dumps(payload, sort_keys=True).encode("utf-8")).hexdigest()

        metrics = ShadowComparisonMetrics(
            total_cases_evaluated=len(cases),
            true_positives=tp,
            true_negatives=tn,
            false_positives=fp,
            false_negatives=fn,
            unknown_or_held_cases=unknown_held,
            median_baseline_hours=round(med_base, 1),
            median_shadow_hours=round(med_shad, 1),
            median_time_reduction_pct=reduction_pct,
            target_30_pct_status=target_status,
            subgroup_fairness=fairness_list,
            sha256_checksum=checksum,
        )

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/Evaluation",
            action="CALCULATE_EVALUATION_METRICS",
            entity_type="EvaluationMetrics",
            entity_id=f"EVAL-{int(utc_now().timestamp())}",
            version_id="1.0",
            reason=(
                f"Evaluated {len(cases)} shadow cases. Median time reduction: {reduction_pct}% "
                f"({target_status.value}). TP={tp}, TN={tn}, FP={fp}, FN={fn}, Unknown={unknown_held}."
            ),
        )
        return metrics


# Global singleton instance
evaluation_harness_service = EvaluationHarnessService()
