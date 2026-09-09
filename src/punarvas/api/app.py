"""
PUNARVAS-AI FastAPI Modular Application (ARC-C01 to ARC-C13).
Normative Reference: architecture.md §5, rules.md (RUL-001 advisory envelope).
"""

from datetime import datetime, timezone, timedelta
from typing import Any, Dict, List, Optional
from fastapi import FastAPI, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

from punarvas.core.contracts import (
    APIResponseEnvelope,
    AdvisoryEnvelope,
    GeoPoint,
    GeographyScope,
    UserContext,
)
from punarvas.core.enums import (
    AuthorityState,
    ClassificationLevel,
    ConsentPurpose,
    FundingState,
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
    capacity_ledger,
    ReservationStatus,
    SiteCapacityConfig,
    CapacityReservation,
)
from punarvas.modules.field import (
    field_sync_service,
    FieldSurveySubmission,
    DeviceStatus,
)
from punarvas.modules.governance import (
    governance_service,
    approval_service,
    objections_service,
    ApprovalCondition,
    ApprovalConditionType,
    OfficialApprovalRecord,
    StatutoryNotificationRecord,
    ObjectionCategory,
    ObjectionAdmissibility,
    ObjectionFilingChannel,
    HearingNotice,
    ObjectionDecisionOrder,
    ObjectionCase,
    SLAEscalationRecommendation,
)
from punarvas.modules.reporting import reporting_service
from punarvas.modules.evaluation import (
    evaluation_harness_service,
    CaseShadowEvaluation,
)
from punarvas.modules.scaling import (
    DistrictProfile,
    district_onboarding_service,
    statewide_oversight_service,
)
from punarvas.modules.adaptation import (
    StateTenantPackage,
    multi_state_adapter_service,
)
from punarvas.modules.governance.national_clearinghouse import (
    InterStateRelocationRequest,
    NationalRegistryManifest,
    national_clearinghouse_service,
)
from punarvas.core.localization import (
    LOCALIZATION_REGISTRY,
    get_supported_languages,
    translate_text,
)
from punarvas.modules.live_ops import (
    StepUpToken,
    BreakGlassSession,
    RestoreVerificationResult,
    ManualDecisionRecord,
    RollbackRecord,
    step_up_auth_manager,
    break_glass_manager,
    disaster_recovery_harness,
    degraded_mode_controller,
    manual_continuity_reconciler,
    rollback_controller,
)
from punarvas.modules.reconstruction import (
    CompletionMilestoneType,
    MilestoneRecord,
    CaseCompletionSummary,
    DecisionReconstructionReport,
    DisclosureReviewResult,
    delivery_completion_tracker,
    decision_reconstruction_engine,
    disclosure_review_engine,
    DefectSeverity,
    DefectCategory,
    DefectRecord,
    RelocationNecessityReview,
    HouseholdSchemeAssessment,
    FundingRecord,
    FundingGapReport,
    BasicServicesReadiness,
    ExternalHandoffRecord,
    PostRelocationFollowUp,
    CaseDeliveryTracker,
    delivery_tracker,
)
from punarvas.modules.district_scale import (
    DistrictPolicyOverride,
    district_onboarding_engine,
    policy_inheritance_engine,
    multi_district_isolation_manager,
    scale_quota_limiter,
)
from punarvas.core.errors import (
    ReservationConflictError,
    UnauthorizedGeographyAccessError,
    EntityFrozenByObjectionError,
    ApprovalConditionUnmetError,
    UnauthorizedActionError,
    DefectsBlockCompletionError,
    UnservicedUnitHandoverError,
)
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


# --- Phase 7 (PH-4): Kerala Multi-District Scaling Endpoints ---
@app.get("/api/v1/scaling/districts", response_model=APIResponseEnvelope)
def list_districts():
    districts = district_onboarding_service.list_districts()
    return APIResponseEnvelope(data=[d.model_dump() for d in districts])


@app.get("/api/v1/scaling/districts/{district_id}", response_model=APIResponseEnvelope)
def get_district(district_id: str):
    p = district_onboarding_service.get_district(district_id)
    if not p:
        raise HTTPException(status_code=404, detail=f"District {district_id} not found")
    return APIResponseEnvelope(data=p.model_dump())


@app.post("/api/v1/scaling/districts", response_model=APIResponseEnvelope)
def onboard_district(profile: DistrictProfile):
    res = district_onboarding_service.onboard_district(profile, actor_id="api_admin")
    return APIResponseEnvelope(data=res.model_dump())


@app.get("/api/v1/scaling/statewide-dashboard", response_model=APIResponseEnvelope)
def get_statewide_dashboard():
    dash = statewide_oversight_service.get_statewide_dashboard()
    return APIResponseEnvelope(data=dash.model_dump())


@app.get("/api/v1/scaling/districts/{district_id}/fixtures", response_model=APIResponseEnvelope)
def get_district_fixtures(district_id: str):
    try:
        fixtures = statewide_oversight_service.get_district_fixture_pack(district_id)
        return APIResponseEnvelope(data=fixtures)
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


# --- Phase 8 (PH-5): Multi-State Adaptation & Tenant Isolation Endpoints ---
@app.get("/api/v1/adaptation/tenants", response_model=APIResponseEnvelope)
def list_state_tenants():
    tenants = multi_state_adapter_service.list_tenants()
    return APIResponseEnvelope(data=[t.model_dump() for t in tenants])


@app.get("/api/v1/adaptation/tenants/{state_code}", response_model=APIResponseEnvelope)
def get_state_tenant(state_code: str):
    t = multi_state_adapter_service.get_tenant(state_code)
    if not t:
        raise HTTPException(status_code=404, detail=f"Tenant for state {state_code} not found")
    return APIResponseEnvelope(data=t.model_dump())


@app.post("/api/v1/adaptation/tenants", response_model=APIResponseEnvelope)
def onboard_state_tenant(pkg: StateTenantPackage):
    res = multi_state_adapter_service.onboard_tenant(pkg, actor_id="api_admin")
    return APIResponseEnvelope(data=res.model_dump())


@app.get("/api/v1/adaptation/dossier-header/{state_code}", response_model=APIResponseEnvelope)
def get_state_dossier_header(state_code: str, case_id: str = "CASE-DEMO-001", language: Optional[str] = None):
    try:
        hdr = multi_state_adapter_service.generate_state_dossier_header(
            state_code=state_code,
            case_id=case_id,
            language=language,
            actor_id="api_dossier_generator",
        )
        return APIResponseEnvelope(data=hdr)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/adaptation/tenants/{state_code}/fixtures", response_model=APIResponseEnvelope)
def get_state_fixtures(state_code: str):
    fixtures = multi_state_adapter_service.get_state_fixture_pack(state_code)
    if not fixtures:
        raise HTTPException(status_code=404, detail=f"No fixtures found for state {state_code}")
    return APIResponseEnvelope(data=fixtures)


# --- Phase 3 (PH-3): Controlled Live Wayanad Operations & Recovery Endpoints ---

class StepUpRequest(BaseModel):
    user_id: str
    action: str
    valid_seconds: int = 300


@app.post("/api/v1/auth/step-up", response_model=APIResponseEnvelope)
def issue_step_up_token(req: StepUpRequest):
    user = UserContext(
        user_id=req.user_id,
        username=f"user_{req.user_id}",
        roles=[RoleType.GOVERNMENT_APPROVER],
    )
    try:
        tok = step_up_auth_manager.issue_step_up_token(user, req.action, req.valid_seconds)
        return APIResponseEnvelope(data=tok.model_dump())
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


class BreakGlassRequest(BaseModel):
    user_id: str
    reason: str
    justification_category: str
    approving_authority: str
    duration_minutes: int = 60


@app.post("/api/v1/auth/break-glass", response_model=APIResponseEnvelope)
def request_break_glass(req: BreakGlassRequest):
    try:
        session = break_glass_manager.request_break_glass(
            user_id=req.user_id,
            reason=req.reason,
            justification_category=req.justification_category,
            approving_authority=req.approving_authority,
            duration_minutes=req.duration_minutes,
        )
        return APIResponseEnvelope(data=session.model_dump())
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


class RevokeBreakGlassRequest(BaseModel):
    actor_id: str
    reason: str


@app.post("/api/v1/auth/break-glass/{session_id}/revoke", response_model=APIResponseEnvelope)
def revoke_break_glass(session_id: str, req: RevokeBreakGlassRequest):
    try:
        session = break_glass_manager.revoke_break_glass(session_id, req.actor_id, req.reason)
        return APIResponseEnvelope(data=session.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


class RestoreVerificationRequest(BaseModel):
    db_snapshot_hash: str
    actual_db_hash: str
    object_inventory: Dict[str, str]
    actual_objects: Dict[str, str]
    audit_checkpoint_valid: bool


@app.post("/api/v1/recovery/verify-restore", response_model=APIResponseEnvelope)
def verify_restore_consistency(req: RestoreVerificationRequest):
    result = disaster_recovery_harness.verify_restore_consistency(
        db_snapshot_hash=req.db_snapshot_hash,
        actual_db_hash=req.actual_db_hash,
        object_inventory=req.object_inventory,
        actual_objects=req.actual_objects,
        audit_checkpoint_valid=req.audit_checkpoint_valid,
    )
    return APIResponseEnvelope(data=result.model_dump())


class DegradedModeToggleRequest(BaseModel):
    engage: bool
    reason: Optional[str] = None
    actor_id: str = "system_operator"


@app.get("/api/v1/recovery/degraded-mode", response_model=APIResponseEnvelope)
def get_degraded_mode_status():
    return APIResponseEnvelope(
        data={
            "is_degraded": degraded_mode_controller.is_degraded,
            "reason": degraded_mode_controller._degraded_reason,
        }
    )


@app.post("/api/v1/recovery/degraded-mode", response_model=APIResponseEnvelope)
def set_degraded_mode(req: DegradedModeToggleRequest):
    if req.engage:
        degraded_mode_controller.engage_degraded_mode(
            reason=req.reason or "Administrative circuit breaker engaged",
            actor_id=req.actor_id,
        )
    else:
        degraded_mode_controller.disengage_degraded_mode(actor_id=req.actor_id)
    return APIResponseEnvelope(
        data={
            "is_degraded": degraded_mode_controller.is_degraded,
            "reason": degraded_mode_controller._degraded_reason,
        }
    )


class ManualContinuityRequest(BaseModel):
    manual_decision_id: str
    case_id: str
    jurisdiction_id: str
    approving_authority: str
    statutory_basis: str
    decision_summary: str
    paper_notice_reference: str
    signed_offline_time: str
    actor_id: str


@app.post("/api/v1/live/manual-continuity", response_model=APIResponseEnvelope)
def reconcile_manual_decision(req: ManualContinuityRequest):
    from datetime import datetime
    try:
        dt = datetime.fromisoformat(req.signed_offline_time)
    except Exception:
        from datetime import timezone
        dt = datetime.now(timezone.utc)
    rec = manual_continuity_reconciler.reconcile_manual_decision(
        manual_decision_id=req.manual_decision_id,
        case_id=req.case_id,
        jurisdiction_id=req.jurisdiction_id,
        approving_authority=req.approving_authority,
        statutory_basis=req.statutory_basis,
        decision_summary=req.decision_summary,
        paper_notice_reference=req.paper_notice_reference,
        signed_offline_time=dt,
        actor_id=req.actor_id,
    )
    return APIResponseEnvelope(data=rec.model_dump())


class RollbackRequest(BaseModel):
    programme_id: str
    authority_order_ref: str
    reason: str
    actor_id: str
    active_projections_count: int = 0


@app.post("/api/v1/live/rollback", response_model=APIResponseEnvelope)
def initiate_live_rollback(req: RollbackRequest):
    rec = rollback_controller.initiate_rollback(
        programme_id=req.programme_id,
        authority_order_ref=req.authority_order_ref,
        reason=req.reason,
        actor_id=req.actor_id,
        active_projections_count=req.active_projections_count,
    )
    return APIResponseEnvelope(data=rec.model_dump())


class DecisionReconstructionRequest(BaseModel):
    decision_id: str
    case_id: str
    authority_state: str
    approving_authority: str
    statutory_basis: str
    effective_valid_time: str
    inputs: List[Dict[str, Any]]


@app.post("/api/v1/live/reconstruct-decision", response_model=APIResponseEnvelope)
def reconstruct_decision_dag(req: DecisionReconstructionRequest):
    report = decision_reconstruction_engine.reconstruct_decision(
        decision_id=req.decision_id,
        case_id=req.case_id,
        authority_state=req.authority_state,
        approving_authority=req.approving_authority,
        statutory_basis=req.statutory_basis,
        effective_valid_time=req.effective_valid_time,
        inputs=req.inputs,
    )
    return APIResponseEnvelope(data=report.model_dump())


class DisclosureReviewRequest(BaseModel):
    records: List[Dict[str, Any]]
    prior_published_records: Optional[List[Dict[str, Any]]] = None
    min_k_threshold: int = 5


@app.post("/api/v1/live/disclosure-review", response_model=APIResponseEnvelope)
def review_public_disclosure(req: DisclosureReviewRequest):
    res = disclosure_review_engine.review_and_generalize_public_projection(
        records=req.records,
        prior_published_records=req.prior_published_records,
        min_k_threshold=req.min_k_threshold,
    )
    return APIResponseEnvelope(data=res.model_dump())


class MilestoneRecordRequest(BaseModel):
    milestone_type: CompletionMilestoneType
    verified_by: str
    evidence_doc_ref: str
    notes: Optional[str] = None
    unresolved_defects: int = 0
    external_system_id: Optional[str] = None
    actor_id: str = "delivery_officer"


@app.post("/api/v1/live/completion-milestones/{case_id}", response_model=APIResponseEnvelope)
def record_completion_milestone(case_id: str, req: MilestoneRecordRequest):
    try:
        rec = delivery_completion_tracker.record_milestone(
            case_id=case_id,
            milestone_type=req.milestone_type,
            verified_by=req.verified_by,
            evidence_doc_ref=req.evidence_doc_ref,
            notes=req.notes,
            unresolved_defects=req.unresolved_defects,
            external_system_id=req.external_system_id,
            actor_id=req.actor_id,
        )
        return APIResponseEnvelope(data=rec.model_dump())
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/live/completion-milestones/{case_id}", response_model=APIResponseEnvelope)
def get_completion_summary(case_id: str):
    summary = delivery_completion_tracker.get_completion_summary(case_id)
    return APIResponseEnvelope(data=summary.model_dump())


# --- Extended Phase 4: Policy Inheritance, Isolation & Scale Limiting ---

@app.get("/api/v1/districts/{district_id}/policy", response_model=APIResponseEnvelope)
def get_district_policy(district_id: str):
    overrides = policy_inheritance_engine.get_district_overrides(district_id)
    return APIResponseEnvelope(
        data={
            "district_id": district_id,
            "state_baselines": policy_inheritance_engine.STATE_BASELINES,
            "overrides": [o.model_dump() for o in overrides],
        }
    )


@app.post("/api/v1/districts/{district_id}/policy/override", response_model=APIResponseEnvelope)
def register_policy_override(district_id: str, override: DistrictPolicyOverride):
    res = policy_inheritance_engine.register_override(override, actor_id="policy_admin")
    return APIResponseEnvelope(data=res.model_dump())


class IsolationCheckRequest(BaseModel):
    user_id: str
    username: str
    roles: List[RoleType]
    district_scope: Optional[str]
    target_district_id: str
    action: str = "READ"


@app.post("/api/v1/scaling/district-isolation-check", response_model=APIResponseEnvelope)
def check_district_isolation(req: IsolationCheckRequest):
    from punarvas.core.contracts import GeographyScope
    user = UserContext(
        user_id=req.user_id,
        username=req.username,
        roles=req.roles,
        geography_scope=GeographyScope(state="Kerala", district=req.district_scope),
    )
    try:
        multi_district_isolation_manager.assert_user_can_access_district(
            user=user,
            target_district_id=req.target_district_id,
            action=req.action,
        )
        return APIResponseEnvelope(
            data={"allowed": True, "user_id": req.user_id, "target_district": req.target_district_id}
        )
    except PermissionError as e:
        raise HTTPException(status_code=403, detail=str(e))


@app.post("/api/v1/districts/{district_id}/acquire-slot", response_model=APIResponseEnvelope)
def acquire_job_slot(district_id: str):
    ok = scale_quota_limiter.acquire_job_slot(district_id)
    return APIResponseEnvelope(
        data={
            "district_id": district_id,
            "acquired": ok,
            "active_jobs": scale_quota_limiter.get_active_job_count(district_id),
        }
    )


@app.post("/api/v1/districts/{district_id}/release-slot", response_model=APIResponseEnvelope)
def release_job_slot(district_id: str):
    scale_quota_limiter.release_job_slot(district_id)
    return APIResponseEnvelope(
        data={
            "district_id": district_id,
            "released": True,
            "active_jobs": scale_quota_limiter.get_active_job_count(district_id),
        }
    )


# --- Phase 6: Trilingual Localization & National NDMA Federation Endpoints ---

@app.get("/api/v1/localization/languages", response_model=APIResponseEnvelope)
def list_supported_languages():
    langs = get_supported_languages()
    return APIResponseEnvelope(data={"supported_languages": langs})


@app.get("/api/v1/localization/{lang}", response_model=APIResponseEnvelope)
def get_localized_dictionary(lang: str):
    if lang not in LOCALIZATION_REGISTRY:
        raise HTTPException(status_code=404, detail=f"Language '{lang}' not supported. Supported: {get_supported_languages()}")
    return APIResponseEnvelope(data=LOCALIZATION_REGISTRY[lang])


@app.get("/api/v1/national/clearinghouse/corridors", response_model=APIResponseEnvelope)
def list_interstate_hazard_corridors():
    corridors = national_clearinghouse_service.list_corridors()
    return APIResponseEnvelope(data=[c.model_dump() for c in corridors])


@app.post("/api/v1/national/clearinghouse/requests", response_model=APIResponseEnvelope)
def submit_interstate_request(req: InterStateRelocationRequest):
    user = UserContext(
        user_id="sdma_state_approver",
        username="sdma_state_secretary",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state=req.origin_state, district="*"),
    )
    res = national_clearinghouse_service.submit_interstate_request(
        req=req,
        user=user,
        reason=f"Inter-state relocation assistance requested for {req.disaster_event}",
    )
    return APIResponseEnvelope(data=res.model_dump())


@app.get("/api/v1/national/clearinghouse/requests", response_model=APIResponseEnvelope)
def list_interstate_requests(state: Optional[str] = None):
    requests = national_clearinghouse_service.list_requests(state=state)
    return APIResponseEnvelope(data=[r.model_dump() for r in requests])


@app.post("/api/v1/national/clearinghouse/manifests", response_model=APIResponseEnvelope)
def federate_state_manifest(manifest: NationalRegistryManifest):
    user = UserContext(
        user_id="state_registry_daemon",
        username="state_federation_service",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state=manifest.state, district=manifest.district),
    )
    res = national_clearinghouse_service.federate_state_manifest(
        manifest=manifest,
        user=user,
        reason=f"Federated state relocation registry for {manifest.state}/{manifest.district}",
    )
    return APIResponseEnvelope(data=res.model_dump())


@app.get("/api/v1/national/clearinghouse/manifests", response_model=APIResponseEnvelope)
def list_federated_manifests(state: Optional[str] = None):
    manifests = national_clearinghouse_service.list_manifests(state=state)
    return APIResponseEnvelope(data=[m.model_dump() for m in manifests])


# --- Phase 9: Approvals, Statutory Notifications, Objections & Multi-Resource Capacity Ledger ---

# 1. Approval Request Models
class IssueApprovalRequest(BaseModel):
    entity_type: str
    entity_id: str
    entity_version: str
    approving_officer_name: str
    approving_officer_designation: str
    statutory_authority_basis: str
    approval_order_number: str
    conditions: Optional[List[ApprovalCondition]] = None
    supersedes_approval_id: Optional[str] = None
    step_up_token: str
    user_id: Optional[str] = "gov_approver"
    user_role: Optional[RoleType] = RoleType.GOVERNMENT_APPROVER


class SatisfyConditionRequest(BaseModel):
    verification_doc_hash: str
    officer_name: str
    user_id: Optional[str] = "deputy_collector"


class PublishStatutoryNotificationRequest(BaseModel):
    approval_id: str
    gazette_notification_number: str
    gazette_volume_number: str
    effective_date: datetime
    notification_title_en: str
    notification_title_ml: str
    notification_text_en: str
    notification_text_ml: str
    issuing_authority: str
    signing_officer_name: str
    digital_signature_hash: str
    user_id: Optional[str] = "gov_approver"


class WithdrawApprovalRequest(BaseModel):
    reason: str
    revocation_order_ref: str
    user_id: Optional[str] = "admin_authority"


# 2. Objections & Appeals Request Models
class FileObjectionRequest(BaseModel):
    household_id: str
    filer_name: str
    target_entity_type: str
    target_entity_id: str
    target_version_id: str
    category: ObjectionCategory
    statement: str
    assigned_officer_id: str
    assigned_officer_name: str
    evidence_hashes: Optional[List[str]] = None
    is_representative: bool = False
    representative_doc: Optional[str] = None
    filing_channel: ObjectionFilingChannel = ObjectionFilingChannel.ASSISTED_SERVICE_DESK
    sla_days: int = 21
    actor_id: Optional[str] = "citizen_desk_clerk"


class ReviewAdmissibilityRequest(BaseModel):
    is_admissible: bool
    rejection_reason: Optional[str] = None
    officer_id: Optional[str] = "reviewing_officer"


class ScheduleHearingRequest(BaseModel):
    hearing_date: datetime
    venue: str
    presiding_officer: str
    notified_parties: List[str]
    officer_id: Optional[str] = "hearing_officer"


class IssueDecisionOrderRequest(BaseModel):
    relief_granted: bool
    summary_of_grounds: str
    remedy_notes: str
    deciding_authority: str
    statutory_authority_basis: Optional[str] = "Disaster Management Act 2005 §30"
    appeal_window_days: int = 30
    officer_id: Optional[str] = "collector_chairperson"


class FileAppealRequest(BaseModel):
    appellate_statement: str
    officer_id: Optional[str] = "appellate_officer"


# 3. Capacity Ledger Request Models
class ConfigureSiteCapacityRequest(BaseModel):
    site_id: str
    district: str
    dwellings_max: int
    land_cents_max: float
    water_m3_day_max: float
    overlapping_parcel_ids: Optional[List[str]] = None


class SetProgrammeBudgetRequest(BaseModel):
    budget_inr: float


class SimulateCapacityRequest(BaseModel):
    scenario_id: str
    site_id: str
    dwellings: int
    land_cents: float
    budget_inr: float
    water_m3_day: float
    actor_id: Optional[str] = "scenario_planner"


class HoldCapacityRequest(BaseModel):
    scenario_id: str
    site_id: str
    dwellings: int
    land_cents: float
    budget_inr: float
    water_m3_day: float
    actor_id: Optional[str] = "allocation_officer"
    hold_duration_days: int = 14


class CommitCapacityRequest(BaseModel):
    reservation_id: str
    approval_id: str
    actor_id: Optional[str] = "approving_authority"


class ReleaseCapacityRequest(BaseModel):
    reservation_id: str
    reason: str
    actor_id: Optional[str] = "reallocation_officer"


# --- Governance Approvals & Notifications Endpoints ---

@app.post("/api/v1/governance/approvals", response_model=APIResponseEnvelope)
def issue_official_approval(req: IssueApprovalRequest):
    ctx = UserContext(
        user_id=req.user_id or "gov_approver",
        username=req.approving_officer_name,
        roles=[req.user_role or RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        record = approval_service.issue_official_approval(
            entity_type=req.entity_type,
            entity_id=req.entity_id,
            entity_version=req.entity_version,
            approving_officer_name=req.approving_officer_name,
            approving_officer_designation=req.approving_officer_designation,
            statutory_authority_basis=req.statutory_authority_basis,
            approval_order_number=req.approval_order_number,
            conditions=req.conditions,
            supersedes_approval_id=req.supersedes_approval_id,
            step_up_token=req.step_up_token,
            context=ctx,
        )
        return APIResponseEnvelope(data=record.model_dump())
    except EntityFrozenByObjectionError as e:
        raise HTTPException(status_code=409, detail=str(e))
    except UnauthorizedActionError as e:
        raise HTTPException(status_code=403, detail=str(e))
    except (KeyError, ValueError) as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/governance/approvals", response_model=APIResponseEnvelope)
def list_approvals(entity_id: Optional[str] = None):
    records = approval_service.list_approvals(entity_id=entity_id)
    return APIResponseEnvelope(data=[r.model_dump() for r in records])


@app.get("/api/v1/governance/approvals/{approval_id}", response_model=APIResponseEnvelope)
def get_approval(approval_id: str):
    rec = approval_service.get_approval(approval_id)
    if not rec:
        raise HTTPException(status_code=404, detail=f"Approval '{approval_id}' not found")
    return APIResponseEnvelope(data=rec.model_dump())


@app.post("/api/v1/governance/approvals/{approval_id}/conditions/{condition_id}/satisfy", response_model=APIResponseEnvelope)
def satisfy_approval_condition(approval_id: str, condition_id: str, req: SatisfyConditionRequest):
    ctx = UserContext(
        user_id=req.user_id or "deputy_collector",
        username=req.officer_name,
        roles=[RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        cond = approval_service.satisfy_condition(
            approval_id=approval_id,
            condition_id=condition_id,
            verification_doc_hash=req.verification_doc_hash,
            officer_name=req.officer_name,
            context=ctx,
        )
        return APIResponseEnvelope(data=cond.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/v1/governance/approvals/{approval_id}/can-allocate", response_model=APIResponseEnvelope)
def check_can_allocate(approval_id: str):
    try:
        can_proceed, failures = approval_service.check_can_allocate(approval_id)
        return APIResponseEnvelope(data={"approval_id": approval_id, "can_allocate": can_proceed, "blocking_failures": failures})
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/governance/approvals/{approval_id}/withdraw", response_model=APIResponseEnvelope)
def withdraw_approval(approval_id: str, req: WithdrawApprovalRequest):
    ctx = UserContext(
        user_id=req.user_id or "admin_authority",
        username="admin",
        roles=[RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        rec = approval_service.withdraw_approval(
            approval_id=approval_id,
            reason=req.reason,
            revocation_order_ref=req.revocation_order_ref,
            context=ctx,
        )
        return APIResponseEnvelope(data=rec.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/governance/notifications", response_model=APIResponseEnvelope)
def publish_statutory_notification(req: PublishStatutoryNotificationRequest):
    ctx = UserContext(
        user_id=req.user_id or "gov_approver",
        username=req.signing_officer_name,
        roles=[RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        rec = approval_service.publish_statutory_notification(
            approval_id=req.approval_id,
            gazette_notification_number=req.gazette_notification_number,
            gazette_volume_number=req.gazette_volume_number,
            effective_date=req.effective_date,
            notification_title_en=req.notification_title_en,
            notification_title_ml=req.notification_title_ml,
            notification_text_en=req.notification_text_en,
            notification_text_ml=req.notification_text_ml,
            issuing_authority=req.issuing_authority,
            signing_officer_name=req.signing_officer_name,
            digital_signature_hash=req.digital_signature_hash,
            context=ctx,
        )
        return APIResponseEnvelope(data=rec.model_dump())
    except EntityFrozenByObjectionError as e:
        raise HTTPException(status_code=409, detail=str(e))
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/v1/governance/notifications/{notification_id}", response_model=APIResponseEnvelope)
def get_statutory_notification(notification_id: str):
    rec = approval_service.get_notification(notification_id)
    if not rec:
        raise HTTPException(status_code=404, detail=f"Statutory notification '{notification_id}' not found")
    return APIResponseEnvelope(data=rec.model_dump())


# --- Objections, Appeals & Grievance Remedies Endpoints ---

@app.post("/api/v1/governance/objections/file", response_model=APIResponseEnvelope)
def file_citizen_objection(req: FileObjectionRequest):
    ctx = UserContext(
        user_id=req.actor_id or "citizen_desk_clerk",
        username="Desk Officer",
        roles=[RoleType.COMMUNITY_OFFICER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    case = objections_service.file_objection(
        household_id=req.household_id,
        filer_name=req.filer_name,
        target_entity_type=req.target_entity_type,
        target_entity_id=req.target_entity_id,
        target_version_id=req.target_version_id,
        category=req.category,
        statement=req.statement,
        assigned_officer_id=req.assigned_officer_id,
        assigned_officer_name=req.assigned_officer_name,
        actor=ctx,
        evidence_hashes=req.evidence_hashes,
        is_representative=req.is_representative,
        representative_doc=req.representative_doc,
        filing_channel=req.filing_channel,
        sla_days=req.sla_days,
    )
    return APIResponseEnvelope(data=case.model_dump())


@app.get("/api/v1/governance/objections", response_model=APIResponseEnvelope)
def list_citizen_objections(household_id: Optional[str] = None):
    cases = objections_service.list_cases(household_id=household_id)
    return APIResponseEnvelope(data=[c.model_dump() for c in cases])


@app.get("/api/v1/governance/objections/{objection_id}", response_model=APIResponseEnvelope)
def get_citizen_objection(objection_id: str):
    case = objections_service.get_case(objection_id)
    if not case:
        raise HTTPException(status_code=404, detail=f"Objection '{objection_id}' not found")
    return APIResponseEnvelope(data=case.model_dump())


@app.post("/api/v1/governance/objections/{objection_id}/admissibility", response_model=APIResponseEnvelope)
def review_objection_admissibility(objection_id: str, req: ReviewAdmissibilityRequest):
    ctx = UserContext(
        user_id=req.officer_id or "reviewing_officer",
        username="Reviewing Officer",
        roles=[RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        case = objections_service.review_admissibility(
            objection_id=objection_id,
            is_admissible=req.is_admissible,
            officer=ctx,
            rejection_reason=req.rejection_reason,
        )
        return APIResponseEnvelope(data=case.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/governance/objections/{objection_id}/hearings", response_model=APIResponseEnvelope)
def schedule_objection_hearing(objection_id: str, req: ScheduleHearingRequest):
    ctx = UserContext(
        user_id=req.officer_id or "hearing_officer",
        username="Deputy Collector",
        roles=[RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        hearing = objections_service.schedule_hearing(
            objection_id=objection_id,
            hearing_date=req.hearing_date,
            venue=req.venue,
            presiding_officer=req.presiding_officer,
            notified_parties=req.notified_parties,
            officer=ctx,
        )
        return APIResponseEnvelope(data=hearing.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/governance/objections/{objection_id}/decision", response_model=APIResponseEnvelope)
def issue_objection_decision(objection_id: str, req: IssueDecisionOrderRequest):
    ctx = UserContext(
        user_id=req.officer_id or "collector_chairperson",
        username=req.deciding_authority,
        roles=[RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        order = objections_service.issue_decision_order(
            objection_id=objection_id,
            relief_granted=req.relief_granted,
            summary_of_grounds=req.summary_of_grounds,
            remedy_notes=req.remedy_notes,
            deciding_authority=req.deciding_authority,
            statutory_authority_basis=req.statutory_authority_basis,
            appeal_window_days=req.appeal_window_days,
            officer=ctx,
        )
        return APIResponseEnvelope(data=order.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/governance/objections/{objection_id}/appeal", response_model=APIResponseEnvelope)
def file_objection_appeal(objection_id: str, req: FileAppealRequest):
    ctx = UserContext(
        user_id=req.officer_id or "appellate_officer",
        username="Appellate Officer",
        roles=[RoleType.GOVERNMENT_APPROVER],
        classification_level=ClassificationLevel.RESTRICTED,
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    try:
        case = objections_service.file_appeal(
            objection_id=objection_id,
            appellate_statement=req.appellate_statement,
            officer=ctx,
        )
        return APIResponseEnvelope(data=case.model_dump())
    except (KeyError, ValueError) as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/governance/objections/entities/{entity_id}/frozen", response_model=APIResponseEnvelope)
def check_entity_frozen_status(entity_id: str):
    is_frozen, reason = objections_service.check_is_entity_frozen(entity_id)
    pending_objs = objections_service.get_pending_objections_for_entity(entity_id)
    return APIResponseEnvelope(data={"entity_id": entity_id, "is_frozen": is_frozen, "reason": reason, "pending_objection_ids": pending_objs})


@app.get("/api/v1/governance/objections/escalations/overdue", response_model=APIResponseEnvelope)
def scan_overdue_objection_escalations(supervisory_authority: Optional[str] = "District Collector & DDMA Chairperson, Wayanad"):
    recs = objections_service.check_sla_escalations(supervisory_authority=supervisory_authority or "District Collector & DDMA Chairperson, Wayanad")
    return APIResponseEnvelope(data=[r.model_dump() for r in recs])


# --- Multi-Resource Capacity Reservation Ledger Endpoints ---

@app.post("/api/v1/capacity/sites/configure", response_model=APIResponseEnvelope)
def configure_site_capacity(req: ConfigureSiteCapacityRequest):
    capacity_ledger.configure_site(
        site_id=req.site_id,
        district=req.district,
        dwellings_max=req.dwellings_max,
        land_cents_max=req.land_cents_max,
        water_m3_day_max=req.water_m3_day_max,
        overlapping_parcel_ids=req.overlapping_parcel_ids,
    )
    return APIResponseEnvelope(data={"site_id": req.site_id, "configured": True})


@app.post("/api/v1/capacity/programme-budget", response_model=APIResponseEnvelope)
def set_programme_budget(req: SetProgrammeBudgetRequest):
    capacity_ledger.set_programme_budget(req.budget_inr)
    return APIResponseEnvelope(data={"programme_budget_inr": req.budget_inr})


@app.get("/api/v1/capacity/sites/{site_id}/remaining", response_model=APIResponseEnvelope)
def get_site_remaining_capacity(site_id: str):
    try:
        rem = capacity_ledger.get_remaining_capacity(site_id)
        return APIResponseEnvelope(data=rem)
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/capacity/simulate", response_model=APIResponseEnvelope)
def simulate_scenario_capacity(req: SimulateCapacityRequest):
    res = capacity_ledger.simulate_draft_scenario(
        scenario_id=req.scenario_id,
        site_id=req.site_id,
        dwellings=req.dwellings,
        land_cents=req.land_cents,
        budget_inr=req.budget_inr,
        water_m3_day=req.water_m3_day,
        actor_id=req.actor_id or "scenario_planner",
    )
    return APIResponseEnvelope(data=res.model_dump())


@app.post("/api/v1/capacity/hold", response_model=APIResponseEnvelope)
def hold_capacity_reservation(req: HoldCapacityRequest):
    try:
        res = capacity_ledger.hold_reservation(
            scenario_id=req.scenario_id,
            site_id=req.site_id,
            dwellings=req.dwellings,
            land_cents=req.land_cents,
            budget_inr=req.budget_inr,
            water_m3_day=req.water_m3_day,
            actor_id=req.actor_id or "allocation_officer",
            hold_duration_days=req.hold_duration_days,
        )
        return APIResponseEnvelope(data=res.model_dump())
    except EntityFrozenByObjectionError as e:
        raise HTTPException(status_code=409, detail=str(e))
    except ReservationConflictError as e:
        raise HTTPException(status_code=409, detail=str(e))
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/capacity/commit", response_model=APIResponseEnvelope)
def commit_capacity_reservation(req: CommitCapacityRequest):
    try:
        res = capacity_ledger.commit_reservation(
            reservation_id=req.reservation_id,
            approval_id=req.approval_id,
            actor_id=req.actor_id or "approving_authority",
        )
        return APIResponseEnvelope(data=res.model_dump())
    except ApprovalConditionUnmetError as e:
        raise HTTPException(status_code=412, detail=str(e))
    except (KeyError, ValueError) as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.post("/api/v1/capacity/release", response_model=APIResponseEnvelope)
def release_capacity_reservation(req: ReleaseCapacityRequest):
    try:
        res = capacity_ledger.release_reservation(
            reservation_id=req.reservation_id,
            reason=req.reason,
            actor_id=req.actor_id or "reallocation_officer",
        )
        return APIResponseEnvelope(data=res.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/v1/capacity/reservations", response_model=APIResponseEnvelope)
def list_capacity_reservations(site_id: Optional[str] = None):
    res_list = capacity_ledger.list_reservations(site_id=site_id)
    return APIResponseEnvelope(data=[r.model_dump() for r in res_list])


@app.get("/api/v1/capacity/reservations/{reservation_id}", response_model=APIResponseEnvelope)
def get_capacity_reservation(reservation_id: str):
    res = capacity_ledger.get_reservation(reservation_id)
    if not res:
        raise HTTPException(status_code=404, detail=f"Reservation '{reservation_id}' not found")
    return APIResponseEnvelope(data=res.model_dump())


# --- Phase 10: Delivery Execution, Defect Clearance, Funding Gap & Post-Relocation Follow-up ---

class NecessityReviewRequest(BaseModel):
    case_id: str
    household_id: str
    in_situ_mitigation_feasible: bool
    permanent_relocation_necessary: bool
    reviewer_name: str
    reviewer_credentials: str
    reasons: str
    uncertainty_level: str = "LOW"
    settlement_community_effects: str = "Preserves hamlet integrity"
    in_situ_description: Optional[str] = None
    estimated_in_situ_cost_inr: Optional[float] = None
    actor_id: Optional[str] = None


class SchemeAssessmentRequest(BaseModel):
    household_id: str
    tenure_category: str
    pathway: RelocationPathway = RelocationPathway.TOWNSHIP


class RecordFundingRequest(BaseModel):
    case_id: str
    source_agency: str
    cost_head: str
    state: FundingState
    amount_inr: float
    actor_id: Optional[str] = "treasury_officer"
    sanction_order_ref: Optional[str] = None
    is_announced_budget_only: bool = False


class SetRequiredCostRequest(BaseModel):
    case_id: str
    required_cost_inr: float


class VerifyServicesRequest(BaseModel):
    case_id: str
    water_supply_lpcd: float
    electricity_energised: bool
    all_weather_road_functional: bool
    sanitation_drainage_functional: bool
    officer_name: str


class LogDefectRequest(BaseModel):
    case_id: str
    site_id: str
    unit_id: str
    category: DefectCategory
    severity: DefectSeverity
    description: str
    officer_name: str


class ResolveDefectRequest(BaseModel):
    defect_id: str
    evidence_ref: str
    officer_name: str
    case_id: Optional[str] = None


class HandoverPossessionRequest(BaseModel):
    case_id: str
    officer_name: str


class PhysicalOccupationRequest(BaseModel):
    case_id: str
    field_officer_name: str


class ExternalHandoffRequest(BaseModel):
    case_id: str
    external_system_name: str
    external_reference_id: str
    accountable_agency: str
    accountable_officer: str
    delegated_scope: str
    reconciliation_method: str = "PERIODIC_API_SYNC_AND_SITE_AUDIT"


class LivelihoodFollowupRequest(BaseModel):
    case_id: str
    milestone_stage: str
    livelihood_restored: bool
    income_restoration_pct: float
    schooling_continuity: bool
    healthcare_accessible: bool
    infrastructure_rating: str
    community_satisfaction: float
    officer_name: str
    grievance_notes: Optional[str] = None


# Endpoints

@app.post("/api/v1/delivery/necessity-review", response_model=APIResponseEnvelope)
def record_necessity_review(req: NecessityReviewRequest):
    rev = delivery_tracker.record_necessity_review(
        case_id=req.case_id,
        household_id=req.household_id,
        in_situ_mitigation_feasible=req.in_situ_mitigation_feasible,
        permanent_relocation_necessary=req.permanent_relocation_necessary,
        reviewer_name=req.reviewer_name,
        reviewer_credentials=req.reviewer_credentials,
        reasons=req.reasons,
        uncertainty_level=req.uncertainty_level,
        settlement_community_effects=req.settlement_community_effects,
        in_situ_description=req.in_situ_description,
        estimated_in_situ_cost_inr=req.estimated_in_situ_cost_inr,
        actor_id=req.actor_id,
    )
    return APIResponseEnvelope(data=rev.model_dump())


@app.get("/api/v1/delivery/necessity-review/{case_id}", response_model=APIResponseEnvelope)
def get_necessity_review(case_id: str):
    rev = delivery_tracker.get_necessity_review(case_id)
    if not rev:
        raise HTTPException(status_code=404, detail=f"Necessity review for case '{case_id}' not found")
    return APIResponseEnvelope(data=rev.model_dump())


@app.post("/api/v1/delivery/scheme-assessment", response_model=APIResponseEnvelope)
def assess_household_scheme(req: SchemeAssessmentRequest):
    assessment = delivery_tracker.assess_household_scheme(
        household_id=req.household_id,
        tenure_category=req.tenure_category,
        pathway=req.pathway,
    )
    return APIResponseEnvelope(data=assessment.model_dump())


@app.get("/api/v1/delivery/scheme-assessment/{household_id}", response_model=APIResponseEnvelope)
def get_scheme_assessment(household_id: str):
    assessment = delivery_tracker.get_scheme_assessment(household_id)
    if not assessment:
        raise HTTPException(status_code=404, detail=f"Scheme assessment for household '{household_id}' not found")
    return APIResponseEnvelope(data=assessment.model_dump())


@app.post("/api/v1/delivery/funding/required-cost", response_model=APIResponseEnvelope)
def set_case_required_cost(req: SetRequiredCostRequest):
    delivery_tracker.set_required_cost(req.case_id, req.required_cost_inr)
    return APIResponseEnvelope(data={"case_id": req.case_id, "required_cost_inr": req.required_cost_inr})


@app.post("/api/v1/delivery/funding", response_model=APIResponseEnvelope)
def record_case_funding(req: RecordFundingRequest):
    rec = delivery_tracker.record_funding(
        case_id=req.case_id,
        source_agency=req.source_agency,
        cost_head=req.cost_head,
        state=req.state,
        amount_inr=req.amount_inr,
        actor_id=req.actor_id or "treasury_officer",
        sanction_order_ref=req.sanction_order_ref,
        is_announced_budget_only=req.is_announced_budget_only,
    )
    return APIResponseEnvelope(data=rec.model_dump())


@app.get("/api/v1/delivery/funding/{case_id}", response_model=APIResponseEnvelope)
def list_case_funding(case_id: str):
    records = delivery_tracker.list_funding(case_id)
    return APIResponseEnvelope(data=[r.model_dump() for r in records])


@app.get("/api/v1/delivery/funding-gap/{case_id}", response_model=APIResponseEnvelope)
def calculate_funding_gap(case_id: str, required_cost_inr: Optional[float] = None):
    gap_report = delivery_tracker.calculate_funding_gap(case_id, required_cost_inr)
    return APIResponseEnvelope(data=gap_report.model_dump())


@app.post("/api/v1/delivery/services-readiness", response_model=APIResponseEnvelope)
def verify_services_readiness(req: VerifyServicesRequest):
    services = delivery_tracker.verify_services_readiness(
        case_id=req.case_id,
        water_supply_lpcd=req.water_supply_lpcd,
        electricity_energised=req.electricity_energised,
        all_weather_road_functional=req.all_weather_road_functional,
        sanitation_drainage_functional=req.sanitation_drainage_functional,
        officer_name=req.officer_name,
    )
    return APIResponseEnvelope(data=services.model_dump())


@app.post("/api/v1/delivery/defects", response_model=APIResponseEnvelope)
def log_unit_defect(req: LogDefectRequest):
    defect = delivery_tracker.log_defect(
        case_id=req.case_id,
        site_id=req.site_id,
        unit_id=req.unit_id,
        category=req.category,
        severity=req.severity,
        description=req.description,
        officer_name=req.officer_name,
    )
    return APIResponseEnvelope(data=defect.model_dump())


@app.get("/api/v1/delivery/defects/{case_id}", response_model=APIResponseEnvelope)
def list_case_defects(case_id: str):
    defects = delivery_tracker.list_defects(case_id)
    return APIResponseEnvelope(data=[d.model_dump() for d in defects])


@app.post("/api/v1/delivery/defects/resolve", response_model=APIResponseEnvelope)
def resolve_unit_defect(req: ResolveDefectRequest):
    try:
        resolved = delivery_tracker.resolve_defect(
            defect_id=req.defect_id,
            evidence_ref=req.evidence_ref,
            officer_name=req.officer_name,
            case_id=req.case_id,
        )
        return APIResponseEnvelope(data=resolved.model_dump())
    except KeyError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/delivery/handover", response_model=APIResponseEnvelope)
def record_possession_handover(req: HandoverPossessionRequest):
    try:
        delivery_tracker.record_possession_handover(req.case_id, req.officer_name)
        return APIResponseEnvelope(data={"case_id": req.case_id, "status": "HANDED_OVER", "officer": req.officer_name})
    except UnservicedUnitHandoverError as e:
        raise HTTPException(status_code=412, detail=str(e))
    except DefectsBlockCompletionError as e:
        raise HTTPException(status_code=409, detail=str(e))
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.post("/api/v1/delivery/unit-constructed", response_model=APIResponseEnvelope)
def mark_unit_constructed(req: HandoverPossessionRequest):
    delivery_tracker.mark_unit_constructed(req.case_id, req.officer_name)
    return APIResponseEnvelope(data={"case_id": req.case_id, "status": "CONSTRUCTED", "officer": req.officer_name})


@app.post("/api/v1/delivery/offer-acceptance", response_model=APIResponseEnvelope)
def record_offer_acceptance(req: HandoverPossessionRequest):
    delivery_tracker.record_offer_acceptance(req.case_id, req.officer_name)
    return APIResponseEnvelope(data={"case_id": req.case_id, "status": "OFFER_ACCEPTED", "officer": req.officer_name})


@app.post("/api/v1/delivery/occupation", response_model=APIResponseEnvelope)
def record_physical_occupation(req: PhysicalOccupationRequest):
    try:
        delivery_tracker.record_physical_occupation(req.case_id, req.field_officer_name)
        return APIResponseEnvelope(data={"case_id": req.case_id, "status": "OCCUPIED", "field_officer": req.field_officer_name})
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/delivery/completion-status/{case_id}", response_model=APIResponseEnvelope)
def evaluate_relocation_completion(case_id: str):
    is_complete, blockers = delivery_tracker.evaluate_relocation_completion(case_id)
    return APIResponseEnvelope(data={"case_id": case_id, "is_relocation_complete": is_complete, "blocking_reasons": blockers})


@app.post("/api/v1/delivery/external-handoff", response_model=APIResponseEnvelope)
def register_external_handoff(req: ExternalHandoffRequest):
    rec = delivery_tracker.register_external_handoff(
        case_id=req.case_id,
        external_system_name=req.external_system_name,
        external_reference_id=req.external_reference_id,
        accountable_agency=req.accountable_agency,
        accountable_officer=req.accountable_officer,
        delegated_scope=req.delegated_scope,
        reconciliation_method=req.reconciliation_method,
    )
    return APIResponseEnvelope(data=rec.model_dump())


@app.get("/api/v1/delivery/external-handoff/{case_id}", response_model=APIResponseEnvelope)
def get_external_handoff(case_id: str):
    rec = delivery_tracker.get_external_handoff(case_id)
    if not rec:
        raise HTTPException(status_code=404, detail=f"External handoff for case '{case_id}' not found")
    return APIResponseEnvelope(data=rec.model_dump())


@app.post("/api/v1/delivery/followup", response_model=APIResponseEnvelope)
def record_livelihood_followup(req: LivelihoodFollowupRequest):
    fol = delivery_tracker.record_livelihood_followup(
        case_id=req.case_id,
        milestone_stage=req.milestone_stage,
        livelihood_restored=req.livelihood_restored,
        income_restoration_pct=req.income_restoration_pct,
        schooling_continuity=req.schooling_continuity,
        healthcare_accessible=req.healthcare_accessible,
        infrastructure_rating=req.infrastructure_rating,
        community_satisfaction=req.community_satisfaction,
        officer_name=req.officer_name,
        grievance_notes=req.grievance_notes,
    )
    return APIResponseEnvelope(data=fol.model_dump())


@app.get("/api/v1/delivery/followup/{case_id}", response_model=APIResponseEnvelope)
def list_livelihood_followups(case_id: str):
    fols = delivery_tracker.list_followups(case_id)
    return APIResponseEnvelope(data=[f.model_dump() for f in fols])






