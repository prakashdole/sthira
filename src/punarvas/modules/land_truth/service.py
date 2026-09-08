"""
PUNARVAS-AI Land Truth & Discrepancy Detection Module (ARC-C04 / C1-04).
Normative Reference: rules.md (RUL-021-027, RUL-021 ULPIN limits, RUL-022 footprint discrepancy).
"""

from typing import Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import GeoPoint, utc_now
from punarvas.core.enums import DiscrepancyType
from punarvas.core.audit import global_audit_ledger


class ParcelRecord(BaseModel):
    parcel_id: str
    ulpin: Optional[str] = None  # 14-digit Bhu-Aadhaar
    survey_number: str
    village: str
    lsg_name: str
    centroid: GeoPoint
    area_cents: float
    paper_title_holder: str
    paper_classification: str  # REVENUE, PRIVATE, PORAMBOKE, FOREST
    observed_structures_count: int = 0  # From Open Buildings or field observation
    ground_occupied: bool = False
    has_fra_claim: bool = False
    cadastral_offset_meters: float = 0.0  # Measured offset between digitized map and ground DGPS


class DiscrepancyTask(BaseModel):
    task_id: str
    parcel_id: str
    discrepancy_type: DiscrepancyType
    severity: str  # HIGH, MEDIUM, LOW
    description: str
    status: str = "OPEN"  # OPEN, UNDER_FIELD_INSPECTION, RESOLVED
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())


class LandTruthService:
    """
    Detects paper vs ground discrepancies without making conclusive legal title rulings (RUL-021, RUL-022).
    """

    def __init__(self):
        self._parcels: Dict[str, ParcelRecord] = {}
        self._tasks: List[DiscrepancyTask] = []

    def register_parcel(self, parcel: ParcelRecord) -> ParcelRecord:
        self._parcels[parcel.parcel_id] = parcel
        return parcel

    def detect_discrepancies(self, parcel_id: str, actor_id: str) -> List[DiscrepancyTask]:
        """Analyze parcel for spatial and legal discrepancies."""
        parcel = self._parcels.get(parcel_id)
        if not parcel:
            raise KeyError(f"Parcel '{parcel_id}' not found.")

        detected: List[DiscrepancyTask] = []

        # Discrepancy 1: Paper says vacant, ground has structures (RUL-022)
        is_paper_vacant = parcel.paper_classification in ("PORAMBOKE", "GOVERNMENT_VACANT")
        if is_paper_vacant and (parcel.observed_structures_count > 0 or parcel.ground_occupied):
            task = DiscrepancyTask(
                task_id=f"TASK-DISC-{parcel_id}-01",
                parcel_id=parcel_id,
                discrepancy_type=DiscrepancyType.PAPER_VACANT_GROUND_OCCUPIED,
                severity="HIGH",
                description=(
                    f"Paper records show '{parcel.paper_classification}', but {parcel.observed_structures_count} "
                    "structures or active occupation observed on ground. Requires human field inquiry (RUL-022)."
                ),
            )
            detected.append(task)

        # Discrepancy 2: Cadastral grid shift exceeds tolerance (Bhulekh shift) (RUL-023)
        if parcel.cadastral_offset_meters > 25.0:
            task = DiscrepancyTask(
                task_id=f"TASK-DISC-{parcel_id}-02",
                parcel_id=parcel_id,
                discrepancy_type=DiscrepancyType.CADASTRAL_GRID_SHIFT,
                severity="MEDIUM",
                description=(
                    f"Digitized cadastral map has a {parcel.cadastral_offset_meters:.1f}m residual offset "
                    "against ground DGPS control points. Georeferenced adjustment required (RUL-023)."
                ),
            )
            detected.append(task)

        # Discrepancy 3: Unresolved Forest Rights Act (FRA) claim (RUL-024, RUL-025)
        if parcel.has_fra_claim or parcel.paper_classification == "FOREST":
            task = DiscrepancyTask(
                task_id=f"TASK-DISC-{parcel_id}-03",
                parcel_id=parcel_id,
                discrepancy_type=DiscrepancyType.UNRESOLVED_FRA_CLAIM,
                severity="HIGH",
                description=(
                    "Parcel has pending FRA Section 4(5) rights recognition or forest classification. "
                    "Requires Grama Sabha and Forest Rights Committee resolution before action."
                ),
            )
            detected.append(task)

        self._tasks.extend(detected)

        if detected:
            global_audit_ledger.log(
                actor_id=actor_id,
                authority_scope="Wayanad/Revenue",
                action="DETECT_DISCREPANCIES",
                entity_type="Parcel",
                entity_id=parcel_id,
                version_id="1.0",
                reason=f"Detected {len(detected)} land truth discrepancy tasks.",
            )

        return detected

    def list_open_tasks(self, parcel_id: Optional[str] = None) -> List[DiscrepancyTask]:
        if parcel_id:
            return [t for t in self._tasks if t.parcel_id == parcel_id and t.status == "OPEN"]
        return [t for t in self._tasks if t.status == "OPEN"]


# Global singleton instance
land_truth_service = LandTruthService()
