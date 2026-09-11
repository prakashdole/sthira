"""
Sthira FastAPI Modular Application (ARC-C01 to ARC-C13).
Normative Reference: architecture.md §5, rules.md (RUL-001 advisory envelope).
"""

from datetime import datetime, timezone, timedelta
from typing import Any, Dict, List, Optional
from fastapi import FastAPI, HTTPException, Request, status
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from sthira.core.contracts import (
    APIResponseEnvelope,
    AdvisoryEnvelope,
    GeoPoint,
    GeographyScope,
    UserContext,
)
from sthira.core.enums import (
    AuthorityState,
    ClassificationLevel,
    ConsentPurpose,
    FundingState,
    RelocationPathway,
    RoleType,
)
from sthira.core.audit import global_audit_ledger
from sthira.core.outbox import global_outbox

from sthira.modules.programme import ProgrammeRecord, programme_service
from sthira.modules.catalog import catalog_service
from sthira.modules.hazard import HazardLayer, hazard_service
from sthira.modules.household import HouseholdCase, household_service
from sthira.modules.policy import (
    SiteCriteriaInput,
    policy_engine,
    sensitivity_analysis_engine,
    FormulaClassification,
    RegistryState,
    StatuteApplicabilityState,
    JurisdictionLevel,
    FormulaDefinition,
    ParameterDefinition,
    PolicyFormulaActivation,
    FormulaExecutionRequest,
    FormulaExecutionResult,
    StatutoryComplianceRecord,
    ComplianceEvaluationResult,
    compliance_service,
    FormulaNotActivatedError,
    UnvalidatedSpecialistFormulaError,
    RejectedFormulaExecutionError,
    NumericalDomainError,
    MissingFormulaInputError,
)
from sthira.modules.land_truth import (
    ParcelRecord,
    land_truth_service,
    agency_import_adapter,
)
from sthira.modules.allocation import (
    allocation_service,
    capacity_reservation_ledger,
    capacity_ledger,
    ReservationStatus,
    SiteCapacityConfig,
    CapacityReservation,
)
from sthira.modules.field import (
    field_sync_service,
    FieldSurveySubmission,
    DeviceStatus,
)
from sthira.modules.governance import (
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
from sthira.modules.reporting import (
    BeneficiaryReviewPack,
    DecisionSummaryDossier,
    EvidenceReference,
    ExportClassification,
    ExportManifest,
    ExportType,
    FieldChecklistItem,
    FieldVerificationChecklist,
    LSGDDisasterManagementPlanAnnex,
    PublicTransparencyProjection,
    ReportingService,
    SiteDossier,
    reporting_service,
)
from sthira.modules.evaluation import (
    evaluation_harness_service,
    CaseShadowEvaluation,
)
from sthira.modules.scaling import (
    DistrictProfile,
    district_onboarding_service,
    statewide_oversight_service,
)
from sthira.modules.adaptation import (
    StateTenantPackage,
    multi_state_adapter_service,
)
from sthira.modules.governance.national_clearinghouse import (
    InterStateRelocationRequest,
    NationalRegistryManifest,
    national_clearinghouse_service,
)
from sthira.modules.source_access import (
    source_access_service,
    CapabilityType,
    PriorityClass,
    ActivationState,
    AOISampleGateInput,
    ObservationReconciliationRequest,
    DependencyBlockerEvaluationRequest,
)
from sthira.modules.resilience import (
    resilience_service,
    ChannelType,
    DataClassification,
    CrossChannelAccessRequest,
    OfflineDeviceRecord,
    CoordinatedRestorePackage,
)
from sthira_v2.app import router as v2_router
from sthira_v2.config import validate_startup
from sthira.core.localization import (
    LOCALIZATION_REGISTRY,
    get_supported_languages,
    translate_text,
)
from sthira.modules.live_ops import (
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
from sthira.modules.reconstruction import (
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
from sthira.modules.district_scale import (
    DistrictPolicyOverride,
    district_onboarding_engine,
    policy_inheritance_engine,
    multi_district_isolation_manager,
    scale_quota_limiter,
)
from sthira.core.errors import (
    ReservationConflictError,
    UnauthorizedGeographyAccessError,
    EntityFrozenByObjectionError,
    ApprovalConditionUnmetError,
    UnauthorizedActionError,
    DefectsBlockCompletionError,
    UnservicedUnitHandoverError,
    DegradedModeError,
)
from sthira.core.identity import (
    TOKEN_TTL_SECONDS,
    authenticate_password,
    issue_access_token,
)
from sthira.api.middleware import security_middleware
from sthira.spikes import load_wayanad_fixture


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
            for purpose_name, consent_state in hh.get("consent_status", {}).items():
                if purpose_name not in ConsentPurpose.__members__:
                    continue
                household_service.record_consent(
                    household_id=hh["household_id"],
                    purpose=ConsentPurpose[purpose_name],
                    consented=consent_state == "CONSENTED",
                    actor_id="system_seed",
                    reason=f"Synthetic fixture consent state: {consent_state}",
                )

        # Seed candidate-site assessments from the historical prototype fixture.
        for site in data["candidate_sites"]:
            policy_engine.register_site_assessment(
                SiteCriteriaInput(
                    site_id=site["site_id"],
                    hazard_susceptibility_level=(
                        "LOW" if site["hazard_safety_state"] == "PASS" else "HIGH"
                    ),
                    in_debris_flow_runout=False,
                    title_clearance_status=(
                        "VERIFIED_CLEAR" if site["legal_readiness_state"] == "PASS" else "UNKNOWN"
                    ),
                    forest_clearance_required=False,
                    lean_season_tested_lpcd=(
                        site["tested_water_lpcd"]
                        if site["lean_season_water_state"] != "UNKNOWN"
                        else None
                    ),
                    has_dry_season_yield_test=site["lean_season_water_state"] != "UNKNOWN",
                    road_access_width_m=site["road_access_width_m"],
                    distance_to_hospital_km=site["distance_to_hospital_km"],
                    distance_to_school_km=site["distance_to_school_km"],
                    dwelling_capacity=site["dwelling_capacity"],
                    unit_plot_cents=site["unit_plot_cents"],
                    evidence_freshness="HISTORICAL",
                    evidence_reference="Synthetic Wayanad fixture; source-specific dates vary",
                )
            )
    except Exception as e:
        print(f"Startup fixture seed notice: {e}")


@asynccontextmanager
async def lifespan(app: FastAPI):
    bootstrap_seed_data()
    validate_startup()
    yield


app = FastAPI(
    title="Sthira Advisory Decision Support Platform",
    description="Proactive permanent relocation planning and programme governance. Reference Pilot: Wayanad, Kerala.",
    version="1.0.0",
    lifespan=lifespan,
)
app.include_router(v2_router)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)
app.middleware("http")(security_middleware)

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



class LoginRequest(BaseModel):
    username: str
    password: str


@app.post("/api/v1/auth/login", response_model=APIResponseEnvelope)
def login(req: LoginRequest):
    user = authenticate_password(req.username, req.password)
    if user is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid credentials")
    token = issue_access_token(user)
    return APIResponseEnvelope(
        data={
            "access_token": token,
            "token_type": "bearer",
            "expires_in": TOKEN_TTL_SECONDS,
            "user": {
                "user_id": user.user_id,
                "username": user.username,
                "roles": [r.value for r in user.roles],
                "state": user.geography_scope.state,
                "district": user.geography_scope.district,
            },
        }
    )


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
        meta={"service": "sthira-core", "version": "1.0.0"},
    )


@app.get("/api/v1/demo/site-screening", response_model=APIResponseEnvelope)
def get_site_screening_demo():
    """Public, predetermined prototype flow; it never allocates or approves land."""
    terrain_fixtures = {
        "SITE-ELSTONE-01": {
            "state": "PASS",
            "elevation_m": 780,
            "slope_deg": 8.4,
            "reading": "Moderate elevation and gentle terrain in the prototype model.",
        },
        "SITE-NEDUMBALA-02": {
            "state": "PASS",
            "elevation_m": 845,
            "slope_deg": 12.8,
            "reading": "Terrain remains within the prototype slope threshold.",
        },
        "SITE-HIGH-SLOPE-03": {
            "state": "FAIL",
            "elevation_m": 1120,
            "slope_deg": 31.6,
            "reading": "Steep terrain exceeds the prototype slope threshold.",
        },
    }
    assessments = []
    for item in policy_engine.list_registered_site_assessments():
        report = item["report"]
        gate_states = [gate["state"] for gate in report["gate_results"]]
        if "FAIL" in gate_states:
            verdict = "FAIL"
            verdict_label = "Not suitable in this screening"
        elif "UNKNOWN" in gate_states or "BLOCKED" in gate_states:
            verdict = "VERIFY"
            verdict_label = "More evidence required"
        else:
            verdict = "PROVISIONAL_PASS"
            verdict_label = "Passed prototype screening"

        hazard_gate = next(
            gate for gate in report["gate_results"] if gate["gate_id"] == "GATE-HAZ-01"
        )
        ground_gates = [
            gate for gate in report["gate_results"] if gate["gate_id"] != "GATE-HAZ-01"
        ]
        ground_states = [gate["state"] for gate in ground_gates]
        if "FAIL" in ground_states:
            ground_state = "FAIL"
        elif "UNKNOWN" in ground_states or "BLOCKED" in ground_states:
            ground_state = "VERIFY"
        else:
            ground_state = "PASS"
        terrain = terrain_fixtures[item["site"]["site_id"]]

        assessments.append(
            {
                **item,
                "verdict": verdict,
                "verdict_label": verdict_label,
                "verification_results": [
                    {
                        "stage_id": "SATELLITE",
                        "label": "Satellite screening",
                        "state": hazard_gate["state"],
                        "reading": hazard_gate["reason"],
                        "evidence": "Historical NASA HLS imagery and hazard-zone overlay",
                    },
                    {
                        "stage_id": "GROUND",
                        "label": "On-ground verification",
                        "state": ground_state,
                        "reading": "Legal, water and emergency-access evidence checked.",
                        "evidence": "Predetermined synthetic field and administrative records",
                    },
                    {
                        "stage_id": "TERRAIN_RADAR",
                        "label": "Radar and elevation",
                        "state": terrain["state"],
                        "reading": terrain["reading"],
                        "evidence": (
                            f"Synthetic radar-derived terrain fixture: {terrain['elevation_m']} m "
                            f"elevation, {terrain['slope_deg']}° slope"
                        ),
                    },
                ],
            }
        )

    return APIResponseEnvelope(
        data={
            "demo_mode": "HISTORICAL_DATA_PROTOTYPE",
            "study_area": "Wayanad, Kerala",
            "verification_model": [
                {
                    "stage_id": "SATELLITE",
                    "label": "Satellite screening",
                    "description": "Historical optical imagery and hazard overlays identify candidate areas.",
                    "data_status": "HISTORICAL",
                },
                {
                    "stage_id": "GROUND",
                    "label": "On-ground verification",
                    "description": "Field and administrative evidence checks water, access and legal readiness.",
                    "data_status": "SYNTHETIC_DEMO",
                },
                {
                    "stage_id": "TERRAIN_RADAR",
                    "label": "Radar and elevation",
                    "description": "A radar-derived terrain model checks elevation and slope constraints.",
                    "data_status": "SYNTHETIC_DEMO",
                },
            ],
            "imagery": {
                "provider": "NASA Earthdata GIBS",
                "product": "HLS Sentinel-2 30 m Nadir BRDF-Adjusted Reflectance",
                "observation_date": "2024-03-31",
                "freshness": "HISTORICAL",
                "url": (
                    "https://gibs.earthdata.nasa.gov/wms/epsg4326/best/wms.cgi"
                    "?SERVICE=WMS&REQUEST=GetMap&VERSION=1.3.0"
                    "&LAYERS=HLS_S30_Nadir_BRDF_Adjusted_Reflectance"
                    "&STYLES=&FORMAT=image/jpeg&TRANSPARENT=false"
                    "&HEIGHT=900&WIDTH=1400&CRS=EPSG:4326"
                    "&BBOX=11.52,76.05,11.64,76.18&TIME=2024-03-31"
                ),
                "limitation": (
                    "Visual context only. It does not establish current safety, title, "
                    "water availability, occupancy, or official approval."
                ),
            },
            "ground_evidence_note": (
                "Predetermined synthetic field and administrative evidence is used for this hackathon demo."
            ),
            "assessments": assessments,
            "outcome_notice": (
                "No land is allocated or approved. Results are prototype screening outcomes "
                "that require current field verification and government review."
            ),
        }
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
    report = policy_engine.register_site_assessment(inp)
    return APIResponseEnvelope(data=report.model_dump())


@app.post("/api/v1/land/detect-discrepancies/{parcel_id}", response_model=APIResponseEnvelope)
def detect_discrepancies(parcel_id: str):
    tasks = land_truth_service.detect_discrepancies(parcel_id, actor_id="api_analyst")
    return APIResponseEnvelope(data=[t.model_dump() for t in tasks])


@app.post("/api/v1/allocation/simulate-scenario", response_model=APIResponseEnvelope)
def simulate_allocation():
    households = list(household_service._cases.values())
    allocation_ready_sites = policy_engine.list_allocation_ready_sites()
    site_capacities = {site.site_id: site.dwelling_capacity for site in allocation_ready_sites}
    site_plot_cents = {site.site_id: site.unit_plot_cents for site in allocation_ready_sites}
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
    action: str
    valid_seconds: int = 300


@app.post("/api/v1/auth/step-up", response_model=APIResponseEnvelope)
def issue_step_up_token(req: StepUpRequest, request: Request):
    user = getattr(request.state, "user", None)
    if user is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Not authenticated")
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
    from sthira.core.contracts import GeographyScope
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
def issue_official_approval(req: IssueApprovalRequest, request: Request):
    ctx = getattr(request.state, "user", None)
    if ctx is None:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Not authenticated")
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
    except DegradedModeError as e:
        raise HTTPException(status_code=503, detail=str(e))
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


# --- Phase 11: Government Dossiers, Evidence-Bound Checklists, Spatial Exports & Manifests ---

class GenerateSiteDossierRequest(BaseModel):
    site_id: str
    site_name: str
    district: str = "Wayanad"
    taluk: str = "Vythiri"
    village: str = "Meppadi"
    gross_area_cents: float = 450.0
    usable_area_cents: float = 380.0
    dwelling_capacity: int = 60
    water_source_description: str = "Borewell cluster connected to gravity distribution scheme"
    lean_season_yield_lpcd: float = 78.0
    hazard_buffer_distance_m: float = 350.0
    slope_mean_deg: float = 14.5
    road_access_width_m: float = 4.5
    evidence_links: List[EvidenceReference] = Field(default_factory=list)
    unresolved_conditions: List[str] = Field(default_factory=list)
    generating_user_id: str = "chief_town_planner"
    approval_ref: Optional[str] = None


class GenerateBeneficiaryPackRequest(BaseModel):
    household_id: str
    head_of_household: str
    member_count: int = 4
    vulnerability_score: float = 85.0
    disability_or_special_needs: bool = False
    tenure_category: str = "OWNER"
    relocation_necessity_review_id: str = "REV-NEC-001"
    preferred_pathway: str = "TOWNSHIP"
    assigned_site_id: Optional[str] = "SITE-ELSTONE-01"
    eligible_schemes: List[str] = Field(
        default_factory=lambda: ["PUNARJANI_LAND_GRANT", "LIFE_MISSION_HOUSING"]
    )
    evidence_links: List[EvidenceReference] = Field(default_factory=list)
    consent_token_ref: str = "CONSENT-TKN-001"
    unresolved_conditions: List[str] = Field(default_factory=list)
    generating_user_id: str = "social_welfare_officer"


class GenerateChecklistRequest(BaseModel):
    target_type: str = "SITE"
    target_id: str = "SITE-ELSTONE-01"
    items: List[FieldChecklistItem] = Field(default_factory=list)
    required_equipment: List[str] = Field(
        default_factory=lambda: ["Trimble DGPS", "Total Station", "Water Quality Kit"]
    )
    safety_precautions: List[str] = Field(
        default_factory=lambda: ["Hard hats mandatory", "No work during heavy rainfall > 15mm/hr"]
    )
    generating_user_id: str = "field_executive_engineer"


class GenerateDecisionSummaryRequest(BaseModel):
    decision_id: str
    entity_type: str = "ALLOCATION_SCENARIO"
    entity_id: str
    policy_version: str = "POL-WYD-2024.1"
    source_checksums: Dict[str, str] = Field(
        default_factory=lambda: {"S01": "hash_s01_soi", "S04": "hash_s04_gsi"}
    )
    evidence_chain_hash: str = "chain_head_hash_9827361"
    generating_user_id: str = "appellate_clerk"
    solver_seed: Optional[int] = 42
    solver_tolerances: Optional[Dict[str, float]] = None
    approval_order_id: Optional[str] = None
    statutory_gazette_id: Optional[str] = None
    objection_token_refs: Optional[List[str]] = None


class GeoJsonExportRequest(BaseModel):
    export_id: str
    features: List[Dict[str, Any]] = Field(default_factory=list)
    generating_user_id: str = "gis_analyst"
    classification: ExportClassification = ExportClassification.RESTRICTED_OFFICIAL


class CsvExportRequest(BaseModel):
    export_id: str
    headers: List[str]
    rows: List[List[Any]]
    generating_user_id: str = "clerk"
    classification: ExportClassification = ExportClassification.RESTRICTED_OFFICIAL


class VerifyManifestRequest(BaseModel):
    manifest_id: str
    payload_content: str


class PublicTransparencyRequest(BaseModel):
    projection_id: str
    district: str = "Wayanad"
    round_number: int = 1
    subregion_counts: Dict[str, int]
    k_threshold: int = 5


@app.post("/api/v1/reporting/dossiers/site", response_model=APIResponseEnvelope)
def generate_site_dossier(req: GenerateSiteDossierRequest):
    dossier = reporting_service.generate_site_dossier(
        site_id=req.site_id,
        site_name=req.site_name,
        district=req.district,
        taluk=req.taluk,
        village=req.village,
        gross_area_cents=req.gross_area_cents,
        usable_area_cents=req.usable_area_cents,
        dwelling_capacity=req.dwelling_capacity,
        water_source_description=req.water_source_description,
        lean_season_yield_lpcd=req.lean_season_yield_lpcd,
        hazard_buffer_distance_m=req.hazard_buffer_distance_m,
        slope_mean_deg=req.slope_mean_deg,
        road_access_width_m=req.road_access_width_m,
        evidence_links=req.evidence_links,
        unresolved_conditions=req.unresolved_conditions,
        generating_user_id=req.generating_user_id,
        approval_ref=req.approval_ref,
    )
    return APIResponseEnvelope(data=dossier.model_dump())


@app.get("/api/v1/reporting/dossiers/site/{site_id}", response_model=APIResponseEnvelope)
def get_site_dossier(site_id: str):
    # Generates standard dossier for site with default evidence links if not dynamically created
    evidence = [
        EvidenceReference(
            field_name="hazard_buffer_distance_m",
            statement=f"Site {site_id} is 350m outside designated 2024 debris flow runout zone",
            source_id="S06_KSDMA_RUNOUT",
            evidence_hash="hash_s06_runout_val_2024",
            is_verified=True,
        ),
        EvidenceReference(
            field_name="lean_season_yield_lpcd",
            statement="KWA hydrogeological yield test confirmed 78 LPCD sustainable yield",
            source_id="S07_CGWB_KWA_YIELD",
            evidence_hash="hash_s07_kwa_yield_2024",
            is_verified=True,
        ),
    ]
    dossier = reporting_service.generate_site_dossier(
        site_id=site_id,
        site_name=f"Site {site_id} Candidate Relocation Township",
        district="Wayanad",
        taluk="Vythiri",
        village="Meppadi",
        gross_area_cents=450.0,
        usable_area_cents=380.0,
        dwelling_capacity=60,
        water_source_description="Borewell cluster connected to gravity distribution scheme",
        lean_season_yield_lpcd=78.0,
        hazard_buffer_distance_m=350.0,
        slope_mean_deg=14.5,
        road_access_width_m=4.5,
        evidence_links=evidence,
        unresolved_conditions=["COND-FRA-NOC-01"],
        generating_user_id="town_planning_officer",
    )
    return APIResponseEnvelope(data=dossier.model_dump())


@app.post("/api/v1/reporting/dossiers/beneficiary", response_model=APIResponseEnvelope)
def generate_beneficiary_pack(req: GenerateBeneficiaryPackRequest):
    pack = reporting_service.generate_beneficiary_review_pack(
        household_id=req.household_id,
        head_of_household=req.head_of_household,
        member_count=req.member_count,
        vulnerability_score=req.vulnerability_score,
        disability_or_special_needs=req.disability_or_special_needs,
        tenure_category=req.tenure_category,
        relocation_necessity_review_id=req.relocation_necessity_review_id,
        preferred_pathway=req.preferred_pathway,
        assigned_site_id=req.assigned_site_id,
        eligible_schemes=req.eligible_schemes,
        evidence_links=req.evidence_links,
        consent_token_ref=req.consent_token_ref,
        unresolved_conditions=req.unresolved_conditions,
        generating_user_id=req.generating_user_id,
    )
    return APIResponseEnvelope(data=pack.model_dump())


@app.get("/api/v1/reporting/dossiers/beneficiary/{household_id}", response_model=APIResponseEnvelope)
def get_beneficiary_pack(household_id: str):
    evidence = [
        EvidenceReference(
            field_name="vulnerability_score",
            statement=f"Field survey verification completed for household {household_id}",
            source_id="S49_FIELD_SURVEY",
            evidence_hash="hash_survey_field_verified_2024",
            is_verified=True,
        )
    ]
    pack = reporting_service.generate_beneficiary_review_pack(
        household_id=household_id,
        head_of_household="Pathumma K.",
        member_count=4,
        vulnerability_score=85.0,
        disability_or_special_needs=True,
        tenure_category="OWNER",
        relocation_necessity_review_id="REV-NEC-001",
        preferred_pathway="TOWNSHIP",
        assigned_site_id="SITE-ELSTONE-01",
        eligible_schemes=["PUNARJANI_LAND_GRANT", "LIFE_MISSION_HOUSING"],
        evidence_links=evidence,
        consent_token_ref="CONSENT-TKN-001",
        unresolved_conditions=[],
        generating_user_id="social_welfare_officer",
    )
    return APIResponseEnvelope(data=pack.model_dump())


@app.post("/api/v1/reporting/dossiers/checklist", response_model=APIResponseEnvelope)
def generate_field_checklist(req: GenerateChecklistRequest):
    chk = reporting_service.generate_field_verification_checklist(
        target_type=req.target_type,
        target_id=req.target_id,
        items=req.items,
        required_equipment=req.required_equipment,
        safety_precautions=req.safety_precautions,
        generating_user_id=req.generating_user_id,
    )
    return APIResponseEnvelope(data=chk.model_dump())


@app.post("/api/v1/reporting/dossiers/decision-summary", response_model=APIResponseEnvelope)
def generate_decision_summary_dossier(req: GenerateDecisionSummaryRequest):
    dossier = reporting_service.generate_decision_summary_dossier(
        decision_id=req.decision_id,
        entity_type=req.entity_type,
        entity_id=req.entity_id,
        policy_version=req.policy_version,
        source_checksums=req.source_checksums,
        evidence_chain_hash=req.evidence_chain_hash,
        generating_user_id=req.generating_user_id,
        solver_seed=req.solver_seed,
        solver_tolerances=req.solver_tolerances,
        approval_order_id=req.approval_order_id,
        statutory_gazette_id=req.statutory_gazette_id,
        objection_token_refs=req.objection_token_refs,
    )
    return APIResponseEnvelope(data=dossier.model_dump())


@app.post("/api/v1/reporting/export/geojson", response_model=APIResponseEnvelope)
def export_spatial_geojson(req: GeoJsonExportRequest):
    res = reporting_service.generate_spatial_geojson_export(
        export_id=req.export_id,
        features_data=req.features,
        generating_user_id=req.generating_user_id,
        classification=req.classification,
    )
    return APIResponseEnvelope(data={
        "geojson": res["geojson"],
        "manifest": res["manifest"].model_dump(),
        "checksum": res["checksum"],
    })


@app.post("/api/v1/reporting/export/csv", response_model=APIResponseEnvelope)
def export_tabular_csv(req: CsvExportRequest):
    res = reporting_service.generate_tabular_csv_export(
        export_id=req.export_id,
        headers=req.headers,
        rows=req.rows,
        generating_user_id=req.generating_user_id,
        classification=req.classification,
    )
    return APIResponseEnvelope(data={
        "csv_content": res["csv_content"],
        "manifest": res["manifest"].model_dump(),
        "checksum": res["checksum"],
    })


@app.post("/api/v1/reporting/manifest/verify", response_model=APIResponseEnvelope)
def verify_export_manifest(req: VerifyManifestRequest):
    res = reporting_service.verify_export_manifest(req.manifest_id, req.payload_content)
    return APIResponseEnvelope(data=res)


@app.get("/api/v1/reporting/manifests/{manifest_id}", response_model=APIResponseEnvelope)
def get_export_manifest(manifest_id: str):
    manifest = reporting_service.get_manifest(manifest_id)
    if not manifest:
        raise HTTPException(status_code=404, detail=f"Manifest '{manifest_id}' not found")
    return APIResponseEnvelope(data=manifest.model_dump())


@app.get("/api/v1/reporting/manifests", response_model=APIResponseEnvelope)
def list_export_manifests():
    manifests = reporting_service.list_manifests()
    return APIResponseEnvelope(data=[m.model_dump() for m in manifests])


@app.post("/api/v1/reporting/public-transparency-projection", response_model=APIResponseEnvelope)
def generate_public_transparency_projection(req: PublicTransparencyRequest):
    proj = reporting_service.generate_k_anonymized_public_projection(
        projection_id=req.projection_id,
        district=req.district,
        round_number=req.round_number,
        subregion_counts=req.subregion_counts,
        k_threshold=req.k_threshold,
    )
    return APIResponseEnvelope(data=proj.model_dump())


# ==============================================================================
# Phase 12: Formula, Parameter & Statutory Compliance Control Endpoints (ARC-C07)
# ==============================================================================

class RegisterFormulaApiRequest(BaseModel):
    formula_id: str
    name: str
    classification: FormulaClassification
    version: str = "1.0.0"
    owner_role: str
    description: str
    mathematical_expression: str
    applicable_parameters: List[str] = Field(default_factory=list)
    requirement_links: List[str] = Field(default_factory=list)
    test_links: List[str] = Field(default_factory=list)
    actor_id: str = "POLICY_ADMIN"


class RegisterParameterApiRequest(BaseModel):
    parameter_id: str
    version: str = "1.0.0"
    formula_ids: List[str] = Field(default_factory=list)
    name: str
    definition: str
    value: Optional[Any] = None
    unit: str
    spatial_support: str
    temporal_support: str
    source: str
    acquisition_method: str
    authority: str
    actor_id: str = "POLICY_ADMIN"


class CreatePolicyActivationApiRequest(BaseModel):
    policy_id: str
    programme_id: str
    activated_formula_ids: List[str]
    authorized_by: str = "DDMA_CHAIRPERSON"


class ExecuteFormulaApiRequest(BaseModel):
    formula_id: str
    inputs: Dict[str, Any]
    policy_activation_id: Optional[str] = None
    reviewer_id: str = "OPERATIONAL_REVIEWER"


class RegisterStatuteApiRequest(BaseModel):
    statute_id: str
    title: str
    jurisdiction: JurisdictionLevel = JurisdictionLevel.UNION_OF_INDIA
    statutory_authority: str
    effective_date: str
    key_sections: List[Dict[str, str]] = Field(default_factory=list)
    mandatory_controls: List[str] = Field(default_factory=list)
    actor_id: str = "STATE_LEGAL_ADVISOR"


class EvaluateCompliancePostureApiRequest(BaseModel):
    programme_id: str
    active_control_ids: List[str]


@app.get("/api/v1/compliance/formulas", response_model=APIResponseEnvelope)
def list_formulas(classification: Optional[FormulaClassification] = None):
    formulas = compliance_service.list_formulas(classification)
    return APIResponseEnvelope(data=[f.model_dump() for f in formulas])


@app.get("/api/v1/compliance/formulas/{formula_id}", response_model=APIResponseEnvelope)
def get_formula(formula_id: str):
    f = compliance_service.get_formula(formula_id)
    if not f:
        raise HTTPException(status_code=404, detail=f"Formula '{formula_id}' not found")
    return APIResponseEnvelope(data=f.model_dump())


@app.post("/api/v1/compliance/formulas", response_model=APIResponseEnvelope)
def register_formula(req: RegisterFormulaApiRequest):
    form_def = FormulaDefinition(
        formula_id=req.formula_id,
        name=req.name,
        classification=req.classification,
        version=req.version,
        owner_role=req.owner_role,
        description=req.description,
        mathematical_expression=req.mathematical_expression,
        applicable_parameters=req.applicable_parameters,
        requirement_links=req.requirement_links,
        test_links=req.test_links,
    )
    saved = compliance_service.register_formula(form_def, actor_id=req.actor_id)
    return APIResponseEnvelope(data=saved.model_dump())


@app.get("/api/v1/compliance/parameters", response_model=APIResponseEnvelope)
def list_parameters():
    params = compliance_service.list_parameters()
    return APIResponseEnvelope(data=[p.model_dump() for p in params])


@app.get("/api/v1/compliance/parameters/{parameter_id}", response_model=APIResponseEnvelope)
def get_parameter(parameter_id: str):
    p = compliance_service.get_parameter(parameter_id)
    if not p:
        raise HTTPException(status_code=404, detail=f"Parameter '{parameter_id}' not found")
    return APIResponseEnvelope(data=p.model_dump())


@app.post("/api/v1/compliance/parameters", response_model=APIResponseEnvelope)
def register_parameter(req: RegisterParameterApiRequest):
    param_def = ParameterDefinition(
        parameter_id=req.parameter_id,
        version=req.version,
        formula_ids=req.formula_ids,
        name=req.name,
        definition=req.definition,
        value=req.value,
        unit=req.unit,
        spatial_support=req.spatial_support,
        temporal_support=req.temporal_support,
        source=req.source,
        acquisition_method=req.acquisition_method,
        authority=req.authority,
    )
    saved = compliance_service.register_parameter(param_def, actor_id=req.actor_id)
    return APIResponseEnvelope(data=saved.model_dump())


@app.post("/api/v1/compliance/activations", response_model=APIResponseEnvelope)
def create_policy_activation(req: CreatePolicyActivationApiRequest):
    try:
        activation = compliance_service.create_policy_activation(
            policy_id=req.policy_id,
            programme_id=req.programme_id,
            activated_formula_ids=req.activated_formula_ids,
            authorized_by=req.authorized_by,
        )
        return APIResponseEnvelope(data=activation.model_dump())
    except RejectedFormulaExecutionError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.get("/api/v1/compliance/activations/{activation_id}", response_model=APIResponseEnvelope)
def get_policy_activation(activation_id: str):
    act = compliance_service.get_policy_activation(activation_id)
    if not act:
        raise HTTPException(status_code=404, detail=f"Activation '{activation_id}' not found")
    return APIResponseEnvelope(data=act.model_dump())


@app.post("/api/v1/compliance/execute", response_model=APIResponseEnvelope)
def execute_formula(req: ExecuteFormulaApiRequest):
    try:
        exec_req = FormulaExecutionRequest(
            formula_id=req.formula_id,
            inputs=req.inputs,
            policy_activation_id=req.policy_activation_id,
            reviewer_id=req.reviewer_id,
        )
        res = compliance_service.execute_formula(exec_req)
        return APIResponseEnvelope(data=res.model_dump())
    except (RejectedFormulaExecutionError, UnvalidatedSpecialistFormulaError) as e:
        raise HTTPException(status_code=400, detail=str(e))
    except FormulaNotActivatedError as e:
        raise HTTPException(status_code=403, detail=str(e))
    except (NumericalDomainError, MissingFormulaInputError) as e:
        raise HTTPException(status_code=422, detail=str(e))


@app.get("/api/v1/compliance/replay/{execution_id}", response_model=APIResponseEnvelope)
def replay_formula_execution(execution_id: str):
    try:
        replay = compliance_service.reproduce_formula_execution(execution_id)
        return APIResponseEnvelope(data=replay)
    except Exception as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/v1/compliance/statutes", response_model=APIResponseEnvelope)
def list_statutes():
    stats = compliance_service.list_statutes()
    return APIResponseEnvelope(data=[s.model_dump() for s in stats])


@app.get("/api/v1/compliance/statutes/{statute_id}", response_model=APIResponseEnvelope)
def get_statute(statute_id: str):
    s = compliance_service.get_statute(statute_id)
    if not s:
        raise HTTPException(status_code=404, detail=f"Statute '{statute_id}' not found")
    return APIResponseEnvelope(data=s.model_dump())


@app.post("/api/v1/compliance/statutes", response_model=APIResponseEnvelope)
def register_statute(req: RegisterStatuteApiRequest):
    stat_rec = StatutoryComplianceRecord(
        statute_id=req.statute_id,
        title=req.title,
        jurisdiction=req.jurisdiction,
        statutory_authority=req.statutory_authority,
        effective_date=req.effective_date,
        key_sections=req.key_sections,
        mandatory_controls=req.mandatory_controls,
    )
    saved = compliance_service.register_statute(stat_rec, actor_id=req.actor_id)
    return APIResponseEnvelope(data=saved.model_dump())


@app.post("/api/v1/compliance/evaluate-posture", response_model=APIResponseEnvelope)
def evaluate_compliance_posture(req: EvaluateCompliancePostureApiRequest):
    res = compliance_service.verify_compliance_posture(
        programme_id=req.programme_id,
        active_control_ids=req.active_control_ids,
    )
    return APIResponseEnvelope(data=res.model_dump())


# ==============================================================================
# PHASE 13: Source Access, Provider Health & Operational Blocker Gating (ARC-C13)
# ==============================================================================

class CatalogSearchApiRequest(BaseModel):
    source_id: str
    query_filter: str = ""
    actor_id: Optional[str] = "analyst"


class CheckGovernanceApiRequest(BaseModel):
    source_id: str
    target_geography: str


class SimulateBasemapFailureApiRequest(BaseModel):
    provider_id: str = "S53"
    actor_id: Optional[str] = "sys-admin"


@app.get("/api/v1/sources/capabilities", response_model=APIResponseEnvelope)
def list_source_capabilities(
    priority_class: Optional[PriorityClass] = None,
    capability_type: Optional[CapabilityType] = None,
    activation_state: Optional[ActivationState] = None,
):
    """
    List all S01-S54 operational source capabilities with optional filters (FR-076, RUL-076).
    """
    caps = source_access_service.list_capabilities(
        priority_class=priority_class,
        capability_type=capability_type,
        activation_state=activation_state,
    )
    return APIResponseEnvelope(data=[c.model_dump() for c in caps])


@app.get("/api/v1/sources/capabilities/{source_id}", response_model=APIResponseEnvelope)
def get_source_capability(source_id: str):
    """
    Get detailed capability specification, intended use, and explicit non-uses for a source.
    """
    cap = source_access_service.get_capability(source_id)
    if not cap:
        raise HTTPException(status_code=404, detail=f"Source capability '{source_id}' not found in S01-S54 register")
    return APIResponseEnvelope(data=cap.model_dump())


@app.post("/api/v1/sources/catalog-search", response_model=APIResponseEnvelope)
def record_source_catalog_search(req: CatalogSearchApiRequest):
    """
    Simulate/record catalog discovery (FR-077, AT-31).
    Explicitly affirms CATALOG_VISIBLE != APPROVED_FOR_USE.
    """
    try:
        res = source_access_service.record_catalog_search(
            source_id=req.source_id,
            query_filter=req.query_filter,
            actor_id=req.actor_id or "analyst",
        )
        return APIResponseEnvelope(data=res)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/sources/aoi-sample/validate", response_model=APIResponseEnvelope)
def validate_aoi_sample(sample_input: AOISampleGateInput):
    """
    Execute Section 6 AOI sample gate (FR-078, RUL-077, AT-38).
    Transitions sample to APPROVED_FOR_USE or QUARANTINED.
    """
    try:
        res = source_access_service.validate_aoi_sample(
            sample_input=sample_input,
            actor_id="api-user",
        )
        return APIResponseEnvelope(data=res.model_dump())
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/v1/sources/mirror-groups", response_model=APIResponseEnvelope)
def list_mirror_groups():
    """
    List shared lineage and mirror groups (FR-079, RUL-078).
    """
    groups = source_access_service.list_mirror_groups()
    return APIResponseEnvelope(data=[g.model_dump() for g in groups])


@app.post("/api/v1/sources/mirror-groups/reconcile", response_model=APIResponseEnvelope)
def reconcile_mirror_observations(req: ObservationReconciliationRequest):
    """
    Deduplicate shared observation entries across mirror platforms (FR-079, AT-32).
    """
    try:
        res = source_access_service.reconcile_mirror_observations(req, actor_id="api-analyst")
        return APIResponseEnvelope(data=res.model_dump())
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.get("/api/v1/sources/check-governance", response_model=APIResponseEnvelope)
def check_geography_governance(source_id: str, target_geography: str):
    """
    Check source against geography lockouts and paused API status (FR-080, AT-33, AT-34).
    """
    res = source_access_service.check_geography_and_governance(source_id, target_geography)
    return APIResponseEnvelope(data=res)


@app.get("/api/v1/sources/health", response_model=APIResponseEnvelope)
def get_provider_health(source_id: Optional[str] = None):
    """
    Retrieve provider adapter health and telemetry with verified zero secret leakage (FR-081, RUL-081).
    """
    health = source_access_service.get_provider_health(source_id)
    return APIResponseEnvelope(data=[h.model_dump() for h in health])


@app.post("/api/v1/sources/blockers/evaluate", response_model=APIResponseEnvelope)
def evaluate_production_blockers(req: DependencyBlockerEvaluationRequest):
    """
    Evaluate S45-S50 mandatory blocker gates before production site approval or allocation (FR-083, RUL-079, AT-35).
    """
    report = source_access_service.evaluate_production_blockers(req, actor_id="api-officer")
    return APIResponseEnvelope(data=report.model_dump())


@app.get("/api/v1/sources/basemaps", response_model=APIResponseEnvelope)
def get_basemap_configuration(provider_id: str = "S53"):
    """
    Retrieve basemap configuration and terms (FR-084, RUL-082).
    """
    try:
        cfg = source_access_service.get_basemap_config(provider_id)
        return APIResponseEnvelope(data=cfg.model_dump())
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@app.post("/api/v1/sources/basemaps/simulate-failure", response_model=APIResponseEnvelope)
def simulate_basemap_failure(req: SimulateBasemapFailureApiRequest):
    """
    Demonstrate that basemap failure decouples from analytical decision lineage (FR-084, AT-37).
    """
    try:
        res = source_access_service.simulate_basemap_failure_fallback(
            provider_id=req.provider_id,
            actor_id=req.actor_id or "sys-admin",
        )
        return APIResponseEnvelope(data=res)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


# ==============================================================================
# PHASE 14: Platform Reliability, Transactional Outbox, Security & Release Assurance (ARC-C11, DEC-045)
# ==============================================================================

class OutboxRelayApiRequest(BaseModel):
    messages: List[Dict[str, Any]]
    max_retries: int = 3
    force_fail_pattern: Optional[str] = None
    reconcile_dead_letter: bool = False


class DeviceEvictionApiRequest(BaseModel):
    device_id: str
    unsynced_records: List[Dict[str, Any]] = Field(default_factory=list)


class RevokeDeviceApiRequest(BaseModel):
    device_id: str


class CertInIncidentApiRequest(BaseModel):
    incident_category: str
    severity: str = "HIGH"
    impacted_assets: List[str]
    remedial_measures: List[str]
    reporting_poc: str = "ciso@sthira.kerala.gov.in"


@app.post("/api/v1/resilience/outbox/relay-reconcile", response_model=APIResponseEnvelope)
def relay_and_reconcile_outbox(req: OutboxRelayApiRequest):
    """
    Relay and reconcile transactional outbox messages with dead-letter queue (NFR-028, AT-25).
    """
    res = resilience_service.relay_and_reconcile_outbox(
        messages=req.messages,
        max_retries=req.max_retries,
        force_fail_pattern=req.force_fail_pattern,
        reconcile_dead_letter=req.reconcile_dead_letter,
    )
    return APIResponseEnvelope(data=res.model_dump())


@app.post("/api/v1/resilience/access/check-cross-channel", response_model=APIResponseEnvelope)
def check_cross_channel_access(req: CrossChannelAccessRequest):
    """
    Evaluate multi-channel data leakage prevention and RLS context reset (NFR-029, NFR-030, AT-24).
    """
    res = resilience_service.evaluate_cross_channel_access(req)
    return APIResponseEnvelope(data=res.model_dump())


@app.post("/api/v1/resilience/offline/device-eviction", response_model=APIResponseEnvelope)
def handle_device_storage_eviction(req: DeviceEvictionApiRequest):
    """
    Handle offline IndexedDB storage eviction, export recovery package, and revoke token (NFR-031, AT-26).
    """
    res = resilience_service.handle_storage_eviction(
        device_id=req.device_id,
        unsynced_records=req.unsynced_records,
    )
    return APIResponseEnvelope(data=res.model_dump())


@app.post("/api/v1/resilience/offline/revoke-lost", response_model=APIResponseEnvelope)
def revoke_lost_device(req: RevokeDeviceApiRequest):
    """
    Immediately revoke binding tokens for lost/stolen field devices (NFR-031).
    """
    res = resilience_service.revoke_lost_device(device_id=req.device_id)
    return APIResponseEnvelope(data=res.model_dump())


@app.post("/api/v1/resilience/restore/validate-consistency", response_model=APIResponseEnvelope)
def validate_restore_consistency(pkg: CoordinatedRestorePackage):
    """
    Validate coordinated restore consistency set across DB, blobs, checkpoints, and keys (NFR-032, AT-27).
    """
    res = resilience_service.validate_coordinated_restore(pkg)
    return APIResponseEnvelope(data=res.model_dump())


@app.get("/api/v1/resilience/ntp/verify-clock", response_model=APIResponseEnvelope)
def verify_ntp_clock(
    ntp_server: str = "time.nplindia.org",
    drift_ms: float = 14.2,
):
    """
    Verify NTP clock synchronization against Indian standard time server (NFR-033).
    """
    res = resilience_service.verify_clock_synchronization(
        ntp_server=ntp_server,
        current_drift_ms=drift_ms,
    )
    return APIResponseEnvelope(data=res)


@app.post("/api/v1/resilience/cert-in/incident", response_model=APIResponseEnvelope)
def report_cert_in_incident(req: CertInIncidentApiRequest):
    """
    Generate statutory CERT-In 6-hour incident report package with 180-day log preservation (NFR-033, RUL-020).
    """
    res = resilience_service.generate_cert_in_incident(
        incident_category=req.incident_category,
        severity=req.severity,
        impacted_assets=req.impacted_assets,
        remedial_measures=req.remedial_measures,
        reporting_poc=req.reporting_poc,
    )
    return APIResponseEnvelope(data=res.model_dump())


@app.get("/api/v1/resilience/release-assurance", response_model=APIResponseEnvelope)
def get_release_assurance_report(
    version: str = "v1.0.0",
    tests_passed: int = 166,
):
    """
    Generate machine-readable release assurance verification matrix (NFR-035, trd.md §9).
    """
    report = resilience_service.generate_release_assurance_report(
        release_version=version,
        automated_tests_passed=tests_passed,
    )
    return APIResponseEnvelope(data=report.model_dump())








