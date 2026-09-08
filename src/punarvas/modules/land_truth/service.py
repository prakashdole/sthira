"""
PUNARVAS-AI Land Truth & Discrepancy Detection Module (ARC-C04 / C1-04).
Normative Reference: rules.md (RUL-021-027, RUL-021 ULPIN limits, RUL-022 footprint discrepancy).
"""

import hashlib
import json
from typing import Any, Dict, List, Optional
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


class AgencyRecord(BaseModel):
    agency_name: str  # e.g., "KERALA_REVENUE_E_REKHA", "FOREST_DEPT_FRA", "DDMA_WAYANAD"
    record_id: str
    survey_number: str
    village: str
    lsg_name: str
    source_freshness_iso: str
    payload: Dict[str, Any]
    checksum: str


class AgencyReconciliationResult(BaseModel):
    record_id: str
    survey_number: str
    status: str  # "RECONCILED", "DISCREPANCY_DETECTED", "PENDING_MANUAL_REVIEW"
    discrepancies: List[str] = Field(default_factory=list)
    tasks_created: List[str] = Field(default_factory=list)
    timestamp: str = Field(default_factory=lambda: utc_now().isoformat())


class AgencyImportAdapter:
    """
    C2-01: Restricted agency import adapters for land revenue, FRA/forest, and DDMA orders.
    Detects cadastral shifts, FRA claim pendency, and paper vs ground discrepancies.
    Enforces RUL-021-027 and DEC-035.
    """

    def __init__(self, land_truth_svc: "LandTruthService"):
        self._land_truth = land_truth_svc
        self._imported_records: Dict[str, AgencyRecord] = {}

    def import_and_reconcile_e_rekha(
        self,
        batch_id: str,
        records: List[Dict[str, Any]],
        actor_id: str,
    ) -> List[AgencyReconciliationResult]:
        """Ingest Kerala e-Rekha / Bhulekh cadastral records and reconcile with ground truth."""
        results: List[AgencyReconciliationResult] = []

        for r in records:
            rec_id = r["record_id"]
            survey_no = r["survey_number"]
            checksum = hashlib.sha256(json.dumps(r, sort_keys=True).encode("utf-8")).hexdigest()

            agency_rec = AgencyRecord(
                agency_name="KERALA_REVENUE_E_REKHA",
                record_id=rec_id,
                survey_number=survey_no,
                village=r.get("village", "Meppadi"),
                lsg_name=r.get("lsg_name", "Meppadi Grama Panchayat"),
                source_freshness_iso=r.get("source_freshness_iso", utc_now().isoformat()),
                payload=r,
                checksum=checksum,
            )
            self._imported_records[rec_id] = agency_rec

            # Check if parcel exists in land truth
            parcel = self._land_truth._parcels.get(r.get("parcel_id"))
            discrepancies = []
            tasks_created = []

            if parcel:
                # 1. Area discrepancy check
                reported_cents = float(r.get("area_cents", parcel.area_cents))
                if abs(reported_cents - parcel.area_cents) > 0.5:
                    msg = f"Area mismatch: e-Rekha states {reported_cents} cents vs field survey {parcel.area_cents} cents."
                    discrepancies.append(msg)

                # 2. Cadastral offset (Bhulekh shift)
                offset = float(r.get("measured_offset_m", parcel.cadastral_offset_meters))
                if offset > 25.0:
                    parcel.cadastral_offset_meters = offset
                    tasks = self._land_truth.detect_discrepancies(parcel.parcel_id, actor_id=actor_id)
                    tasks_created.extend([t.task_id for t in tasks])
                    discrepancies.append(f"Cadastral offset of {offset:.1f}m exceeds 25m threshold (Bhulekh grid shift).")

                # 3. Paper-vacant vs physical occupation
                is_paper_vacant = r.get("classification", parcel.paper_classification) in ("PORAMBOKE", "GOVERNMENT_VACANT")
                if is_paper_vacant and (parcel.observed_structures_count > 0 or parcel.ground_occupied):
                    tasks = self._land_truth.detect_discrepancies(parcel.parcel_id, actor_id=actor_id)
                    tasks_created.extend([t.task_id for t in tasks])
                    discrepancies.append("Paper record indicates vacant Poramboke, but structures/occupancy observed.")

            status = "DISCREPANCY_DETECTED" if discrepancies else "RECONCILED"
            res = AgencyReconciliationResult(
                record_id=rec_id,
                survey_number=survey_no,
                status=status,
                discrepancies=discrepancies,
                tasks_created=tasks_created,
            )
            results.append(res)

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/AgencyImport",
            action="IMPORT_E_REKHA_BATCH",
            entity_type="AgencyBatch",
            entity_id=batch_id,
            version_id="1.0",
            reason=f"Processed {len(records)} e-Rekha records. Detected discrepancies in {[r.record_id for r in results if r.discrepancies]}.",
        )

        return results

    def import_and_reconcile_fra(
        self,
        batch_id: str,
        records: List[Dict[str, Any]],
        actor_id: str,
    ) -> List[AgencyReconciliationResult]:
        """Ingest Forest Department & Forest Rights Act (FRA 2006) claims."""
        results: List[AgencyReconciliationResult] = []

        for r in records:
            rec_id = r["record_id"]
            survey_no = r["survey_number"]
            checksum = hashlib.sha256(json.dumps(r, sort_keys=True).encode("utf-8")).hexdigest()

            agency_rec = AgencyRecord(
                agency_name="FOREST_DEPT_FRA",
                record_id=rec_id,
                survey_number=survey_no,
                village=r.get("village", "Meppadi"),
                lsg_name=r.get("lsg_name", "Meppadi Grama Panchayat"),
                source_freshness_iso=r.get("source_freshness_iso", utc_now().isoformat()),
                payload=r,
                checksum=checksum,
            )
            self._imported_records[rec_id] = agency_rec

            discrepancies = []
            tasks_created = []
            has_pending_fra = r.get("has_pending_claim", True)
            claim_type = r.get("claim_type", "COMMUNITY_FOREST_RIGHTS")  # e.g., CFR or IFR

            parcel = self._land_truth._parcels.get(r.get("parcel_id"))
            if parcel and has_pending_fra:
                parcel.has_fra_claim = True
                tasks = self._land_truth.detect_discrepancies(parcel.parcel_id, actor_id=actor_id)
                tasks_created.extend([t.task_id for t in tasks])
                discrepancies.append(f"Pending FRA 2006 claim ({claim_type}) requires Grama Sabha & FRC resolution.")

            status = "DISCREPANCY_DETECTED" if discrepancies else "RECONCILED"
            results.append(
                AgencyReconciliationResult(
                    record_id=rec_id,
                    survey_number=survey_no,
                    status=status,
                    discrepancies=discrepancies,
                    tasks_created=tasks_created,
                )
            )

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/AgencyImport",
            action="IMPORT_FRA_BATCH",
            entity_type="AgencyBatch",
            entity_id=batch_id,
            version_id="1.0",
            reason=f"Processed {len(records)} FRA claim records. Pending claims flagged for Grama Sabha review.",
        )

        return results


# Global singleton instance
land_truth_service = LandTruthService()
agency_import_adapter = AgencyImportAdapter(land_truth_service)

