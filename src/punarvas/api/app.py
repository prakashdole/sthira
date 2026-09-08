"""
PUNARVAS-AI FastAPI Modular Application (ARC-C01 to ARC-C13).
Normative Reference: architecture.md §5, rules.md (RUL-001 advisory envelope).
"""

from typing import Any, Dict, List, Optional
from fastapi import FastAPI, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

from punarvas.core.contracts import (
    APIResponseEnvelope,
    AdvisoryEnvelope,
    GeoPoint,
    UserContext,
)
from punarvas.core.enums import (
    AuthorityState,
    ClassificationLevel,
    ConsentPurpose,
    RelocationPathway,
    RoleType,
)
from punarvas.core.audit import global_audit_ledger
from punarvas.core.outbox import global_outbox

from punarvas.modules.programme import ProgrammeRecord, programme_service
from punarvas.modules.catalog import catalog_service
from punarvas.modules.hazard import HazardLayer, hazard_service
from punarvas.modules.household import HouseholdCase, household_service
from punarvas.modules.policy import (
    SiteCriteriaInput,
    policy_engine,
    sensitivity_analysis_engine,
)
from punarvas.modules.land_truth import (
    ParcelRecord,
    land_truth_service,
    agency_import_adapter,
)
from punarvas.modules.allocation import (
    allocation_service,
    capacity_reservation_ledger,
)
from punarvas.modules.field import (
    field_sync_service,
    FieldSurveySubmission,
    DeviceStatus,
)
from punarvas.modules.governance import governance_service
from punarvas.modules.reporting import reporting_service
from punarvas.modules.evaluation import (
    evaluation_harness_service,
    CaseShadowEvaluation,
)
from punarvas.core.errors import ReservationConflictError
from punarvas.spikes import load_wayanad_fixture

from contextlib import asynccontextmanager

def bootstrap_seed_data():
    """Seed initial reference fixtures for Wayanad sandbox."""
    try:
        data = load_wayanad_fixture()
        # Seed programme
        prg = ProgrammeRecord(**data["programme"])
        user = UserContext(
            user_id="init_seed",
            username="system_seed",
            roles=[RoleType.GOVERNMENT_APPROVER],
            classification_level=ClassificationLevel.RESTRICTED,
        )
        if not programme_service.get_programme(prg.programme_id):
            programme_service.register_programme(prg, user, reason="Startup bootstrap")

        # Seed hazard layers
        for hz in data["hazard_layers"]:
            hazard_service.register_hazard_layer(
                HazardLayer(
                    layer_id=hz["layer_id"],
                    name=hz["name"],
                    source_id=hz["source_id"],
                    hazard_type="LANDSLIDE_SUSCEPTIBILITY" if "LSM" in hz["layer_id"] else "FLOOD_PROBABILITY",
                    hazard_level=hz["hazard_level"],
                    is_debris_flow_channel=hz.get("debris_flow_channel", False),
                    polygon_coords=hz["geometry"]["coordinates"][0],
                    documented_coverage="Wayanad",
                )
            )

        # Seed affected parcels
        for p in data["affected_parcels"]:
            land_truth_service.register_parcel(
                ParcelRecord(
                    parcel_id=p["parcel_id"],
                    ulpin=p.get("ulpin"),
                    survey_number=p["survey_number"],
                    village=p["village"],
                    lsg_name=p["lsg_name"],
                    centroid=GeoPoint(coordinates=p["centroid"]),
                    area_cents=p["area_cents"],
                    paper_title_holder=p["title_holder"],
                    paper_classification="PORAMBOKE" if ("PORAMBOKE" in p.get("title_holder", "").upper() or "PAPER_VACANT" in p.get("ground_state", "").upper()) else "REVENUE",
                    observed_structures_count=3 if "OCCUPIED" in p["ground_state"] else 0,
                    ground_occupied=True,
                    has_fra_claim="FRA" in str(p.get("discrepancies", [])),
                    cadastral_offset_meters=30.0 if "CADASTRAL_GRID_SHIFT" in str(p.get("discrepancies", [])) else 5.0,
                )
            )

        # Seed households
        for hh in data["synthetic_households"]:
            household_service.register_case(
                HouseholdCase(
                    household_id=hh["household_id"],
                    head_of_household=hh["head_of_household"],
                    member_count=hh["member_count"],
                    elderly_count=hh["vulnerable_members"]["elderly"],
                    disabled_count=hh["vulnerable_members"]["disabled"],
                    children_count=hh["vulnerable_members"]["children"],
                    requires_ground_floor=hh["requires_ground_floor"],
                    source_parcel_id=hh["source_parcel_id"],
                    verified_eligibility=hh["verified_eligibility"],
                    chosen_pathway=RelocationPathway(hh["chosen_pathway"]),
                    preferred_site_ids=hh["preferred_sites"],
                    has_pending_objection=hh["objection_filed"],
                ),
                actor_id="system_seed",
                reason="Startup bootstrap",
            )
            household_service.record_consent(
                household_id=hh["household_id"],
                purpose=ConsentPurpose.PROGRAMME_PARTICIPATION,
                consented=True,
                actor_id="system_seed",
                reason="Signed participation consent",
            )
    except Exception as e:
        print(f"Startup fixture seed notice: {e}")


@asynccontextmanager
async def lifespan(app: FastAPI):
    bootstrap_seed_data()
    yield


app = FastAPI(
    title="PUNARVAS-AI Advisory Decision Support Platform",
    description="Proactive permanent relocation planning and programme governance. Reference Pilot: Wayanad, Kerala.",
    version="1.0.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

from pathlib import Path
from fastapi.responses import RedirectResponse
from fastapi.staticfiles import StaticFiles

# Also run bootstrap immediately on module import so synchronous test runners see seeded data
bootstrap_seed_data()

frontend_dir = Path(__file__).resolve().parent.parent.parent.parent / "frontend"
if frontend_dir.is_dir():
    app.mount("/ui", StaticFiles(directory=str(frontend_dir), html=True), name="frontend")

@app.get("/", include_in_schema=False)
def root_redirect():
    return RedirectResponse(url="/ui/")



@app.get("/health", response_model=APIResponseEnvelope)
def get_health():
    """System liveness and audit hash head verification."""
    is_audit_valid = global_audit_ledger.verify_integrity()
    return APIResponseEnvelope(
        data={
            "status": "HEALTHY",
            "audit_chain_valid": is_audit_valid,
            "audit_head_hash": global_audit_ledger.head_hash,
            "audit_entries_count": len(global_audit_ledger.entries),
            "pending_outbox_messages": len(global_outbox.get_pending()),
        },
        meta={"service": "punarvas-core", "version": "1.0.0"},
    )


@app.get("/api/v1/programme/{programme_id}", response_model=APIResponseEnvelope)
def get_programme(programme_id: str):
    prg = programme_service.get_programme(programme_id)
    if not prg:
        raise HTTPException(status_code=404, detail="Programme not found")
    return APIResponseEnvelope(data=prg.model_dump())


@app.get("/api/v1/catalog/sources", response_model=APIResponseEnvelope)
def list_sources():
    sources = [s.model_dump() for s in catalog_service._registry.values()]
    return APIResponseEnvelope(data=sources)


class ExposureRequest(BaseModel):
    parcel_id: str
    coordinates: List[float]  # [lon, lat]
    local_slope_deg: float


@app.post("/api/v1/hazard/evaluate-exposure", response_model=APIResponseEnvelope)
def evaluate_exposure(req: ExposureRequest):
    pt = GeoPoint(coordinates=req.coordinates)
    res = hazard_service.evaluate_point_exposure(req.parcel_id, pt, req.local_slope_deg)
    return APIResponseEnvelope(data=res.model_dump())


@app.post("/api/v1/site/evaluate-gates", response_model=APIResponseEnvelope)
def evaluate_site_gates(inp: SiteCriteriaInput):
    report = policy_engine.evaluate_site(inp)
    return APIResponseEnvelope(data=report.model_dump())


@app.post("/api/v1/land/detect-discrepancies/{parcel_id}", response_model=APIResponseEnvelope)
def detect_discrepancies(parcel_id: str):
    tasks = land_truth_service.detect_discrepancies(parcel_id, actor_id="api_analyst")
    return APIResponseEnvelope(data=[t.model_dump() for t in tasks])


@app.post("/api/v1/allocation/simulate-scenario", response_model=APIResponseEnvelope)
def simulate_allocation():
    households = list(household_service._cases.values())
    site_capacities = {
        "SITE-ELSTONE-01": 250,
        "SITE-NEDUMBALA-02": 140,
    }
    site_plot_cents = {
        "SITE-ELSTONE-01": 7.0,
        "SITE-NEDUMBALA-02": 6.0,
    }
    scenario = allocation_service.generate_scenario(
        scenario_id="SCEN-WYD-CURRENT",
        households=households,
        site_capacities=site_capacities,
        site_plot_cents=site_plot_cents,
        actor_id="api_analyst",
    )
    return APIResponseEnvelope(data=scenario.model_dump())


class ObjectionRequest(BaseModel):
    household_id: str
    target_decision_id: str
    reason_category: str
    statement: str
    assigned_officer_id: str


@app.post("/api/v1/governance/objections", response_model=APIResponseEnvelope)
def file_objection(req: ObjectionRequest):
    rec = governance_service.file_objection(
        household_id=req.household_id,
        target_decision_id=req.target_decision_id,
        reason_category=req.reason_category,
        statement=req.statement,
        assigned_officer_id=req.assigned_officer_id,
        actor_id="citizen_rep",
    )
    return APIResponseEnvelope(data=rec.model_dump())


@app.get("/api/v1/reporting/dossier/{household_id}")
def get_dossier(household_id: str):
    case = household_service.get_case(household_id)
    if not case:
        raise HTTPException(status_code=404, detail="Household not found")

    rep = reporting_service.generate_bilingual_dossier_html(
        programme_title="Wayanad Landslide Rehabilitation & Resettlement Programme",
        household_id=case.household_id,
        head_name=case.head_of_household,
        pathway=case.chosen_pathway.value,
        assigned_site="SITE-ELSTONE-01",
        gate_status="PASS",
        generating_user_id="officer_web_1",
    )
    return {
        "household_id": household_id,
        "html": rep["html"],
        "checksum_sha256": rep["checksum"],
        "manifest": rep["manifest"].model_dump(),
    }


@app.get("/api/v1/reporting/public-projection", response_model=APIResponseEnvelope)
def get_public_projection():
    summary = reporting_service.generate_public_deidentified_summary(
        district="Wayanad",
        total_eligible=430,
        township_count=350,
        self_relocation_count=60,
        unassigned_count=20,
    )
    return APIResponseEnvelope(data=summary)


@app.get("/api/v1/audit/verify", response_model=APIResponseEnvelope)
def verify_audit():
    is_valid = global_audit_ledger.verify_integrity()
    return APIResponseEnvelope(
        data={
            "chain_valid": is_valid,
            "head_hash": global_audit_ledger.head_hash,
            "total_records": len(global_audit_ledger.entries),
        }
    )


# --- Phase 2: Agency Import & Reconciliation ---
class ERekhaImportRequest(BaseModel):
    batch_id: str
    records: List[Dict[str, Any]]


@app.post("/api/v1/agency-import/e-rekha", response_model=APIResponseEnvelope)
def import_e_rekha(req: ERekhaImportRequest):
    results = agency_import_adapter.import_and_reconcile_e_rekha(
        batch_id=req.batch_id,
        records=req.records,
        actor_id="api_revenue_officer",
    )
    return APIResponseEnvelope(data=[r.model_dump() for r in results])


class FRAImportRequest(BaseModel):
    batch_id: str
    records: List[Dict[str, Any]]


@app.post("/api/v1/agency-import/fra", response_model=APIResponseEnvelope)
def import_fra(req: FRAImportRequest):
    results = agency_import_adapter.import_and_reconcile_fra(
        batch_id=req.batch_id,
        records=req.records,
        actor_id="api_forest_officer",
    )
    return APIResponseEnvelope(data=[r.model_dump() for r in results])


# --- Phase 2: Field Workspace & Lost-Device Revocation ---
class LostDeviceReportRequest(BaseModel):
    device_id: str
    reason: str


@app.post("/api/v1/field/report-lost-device", response_model=APIResponseEnvelope)
def report_lost_device(req: LostDeviceReportRequest):
    try:
        dev = field_sync_service.report_lost_device(
            device_id=req.device_id,
            reason=req.reason,
            actor_id="security_admin",
        )
        return APIResponseEnvelope(data=dev.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/field/sync", response_model=APIResponseEnvelope)
def sync_offline_survey(submission: FieldSurveySubmission):
    # Dummy server records for demonstration
    current_records: Dict[str, Dict[str, Any]] = {}
    res = field_sync_service.ingest_offline_sync(
        submission=submission,
        current_server_records=current_records,
        actor_id="field_sync_daemon",
    )
    return APIResponseEnvelope(data=res.model_dump())


# --- Phase 2: Capacity Reservation Ledger ---
class CapacityReserveRequest(BaseModel):
    scenario_id: str
    site_id: str
    dwellings: int
    land_cents: float
    budget_inr: float
    water_m3_day: float


@app.post("/api/v1/capacity/reserve", response_model=APIResponseEnvelope)
def reserve_capacity(req: CapacityReserveRequest):
    try:
        res = capacity_reservation_ledger.reserve(
            scenario_id=req.scenario_id,
            site_id=req.site_id,
            dwellings=req.dwellings,
            land_cents=req.land_cents,
            budget_inr=req.budget_inr,
            water_m3_day=req.water_m3_day,
            actor_id="api_approver",
        )
        return APIResponseEnvelope(data=res.model_dump())
    except ReservationConflictError as e:
        raise HTTPException(status_code=409, detail=e.message)


@app.get("/api/v1/capacity/status/{site_id}", response_model=APIResponseEnvelope)
def get_capacity_status(site_id: str):
    rem = capacity_reservation_ledger.get_remaining_capacity(site_id)
    return APIResponseEnvelope(data=rem)


# --- Phase 2: Sensitivity Analysis ---
class SensitivityAnalysisRequest(BaseModel):
    sites: List[SiteCriteriaInput]
    perturbation_factor: float = 0.20


@app.post("/api/v1/policy/sensitivity", response_model=APIResponseEnvelope)
def run_sensitivity_analysis(req: SensitivityAnalysisRequest):
    results = sensitivity_analysis_engine.analyze_site_rank_sensitivity(
        sites=req.sites,
        engine=policy_engine,
        perturbation_factor=req.perturbation_factor,
    )
    return APIResponseEnvelope(data=[r.model_dump() for r in results])


# --- Phase 2: Kerala LSGD Disaster Management Plan Annex ---
@app.get("/api/v1/reporting/lsgd-plan-annex", response_model=APIResponseEnvelope)
def get_lsgd_dm_plan_annex(lsg_name: str = "Meppadi Grama Panchayat"):
    annex = reporting_service.generate_lsgd_dm_plan_annex(
        lsg_name=lsg_name,
        district="Wayanad",
        vulnerable_wards=[10, 11, 12],
        settlement_names=["Chooralmala", "Mundakkai", "Punchirimattam"],
        verified_beneficiary_count=430,
        host_sites=[{"site_id": "SITE-ELSTONE-01", "capacity": 200}],
        generating_user_id="api_planner",
    )
    return APIResponseEnvelope(data=annex.model_dump())


# --- Phase 2: Evaluation & Benchmark Metrics ---
@app.get("/api/v1/evaluation/metrics", response_model=APIResponseEnvelope)
def get_evaluation_metrics():
    metrics = evaluation_harness_service.calculate_metrics(actor_id="eval_api_caller")
    return APIResponseEnvelope(data=metrics.model_dump())

