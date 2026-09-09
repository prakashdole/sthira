"""
Unit tests for Phase 13 Source Access, Provider Health, Operational Readiness & Blocker Gating (ARC-C13).
Normative Reference: source-register.md, rules.md (RUL-076-RUL-083), trd.md (FR-076-FR-084, AT-31-AT-38).
"""

import hashlib
from datetime import datetime, timedelta, timezone
import pytest

from punarvas.modules.source_access.contracts import (
    AOISampleGateInput,
    ActivationState,
    CapabilityType,
    DependencyBlockerEvaluationRequest,
    ObservationReconciliationRequest,
    PriorityClass,
    ReconciliationMethod,
)
from punarvas.modules.source_access.service import SourceAccessService


@pytest.fixture
def source_service():
    return SourceAccessService()


def test_s01_s54_catalog_completeness(source_service):
    """
    FR-076 & RUL-076: Verify all S01-S54 source capabilities are registered
    with valid types, priorities, custodians, owners, and explicit non-uses.
    """
    capabilities = source_service.list_capabilities()
    assert len(capabilities) == 54, f"Expected 54 capabilities, got {len(capabilities)}"

    # Check that S01 through S54 are all present
    registered_ids = {c.source_id for c in capabilities}
    for i in range(1, 55):
        expected_id = f"S{i:02d}"
        assert expected_id in registered_ids, f"Missing source ID {expected_id}"

    # Verify attributes for representative records
    s01 = source_service.get_capability("S01")
    assert s01.name == "KSDMA/GSI Landslide Susceptibility"
    assert s01.capability_type == CapabilityType.PRODUCT
    assert s01.priority_class == PriorityClass.CORE
    assert len(s01.explicit_non_uses) >= 1
    assert "Susceptibility is not runout" in s01.explicit_non_uses[0]

    # Verify S45-S50 are agency/field blockers
    for blocker_id in ["S45", "S46", "S47", "S50"]:
        record = source_service.get_capability(blocker_id)
        assert record.priority_class == PriorityClass.AGENCY_BLOCKER

    for field_blocker_id in ["S48", "S49"]:
        record = source_service.get_capability(field_blocker_id)
        assert record.priority_class == PriorityClass.FIELD_BLOCKER


def test_catalog_visibility_is_not_usable_access(source_service):
    """
    FR-077, RUL-076 & AT-31: CDSE STAC search returns an item,
    but catalog search alone MUST NOT mark the asset usable or downloaded.
    """
    res = source_service.record_catalog_search(
        source_id="S10",
        query_filter="datetime=2024-08-01/2024-08-05&bbox=75.8,11.5,76.3,11.9",
        actor_id="test-analyst",
    )
    assert res["source_id"] == "S10"
    assert res["current_state"] == ActivationState.CATALOG_VISIBLE.value
    assert res["is_usable"] is False
    assert res["dependent_workflow_status"] == "HOLD"

    cap = source_service.get_capability("S10")
    assert cap.state == ActivationState.CATALOG_VISIBLE
    assert cap.state != ActivationState.APPROVED_FOR_USE


def test_aoi_sample_gate_approval(source_service):
    """
    FR-078 & RUL-077: Permitted AOI sample passing all 12 gate criteria
    is activated to APPROVED_FOR_USE.
    """
    raw_bytes = b"MOCK_VALID_GEO_TIFF_WAYANAD_TERRAIN_DATA"
    checksum = hashlib.sha256(raw_bytes).hexdigest()

    sample = AOISampleGateInput(
        source_id="S18",
        sample_id="SAMPLE-COP-DEM-001",
        license_type="Copernicus Open Access / CC-BY-4.0",
        has_redistribution_and_offline_rights=True,
        min_lat=11.55,
        max_lat=11.85,
        min_lon=75.95,
        max_lon=76.25,
        observation_timestamp=datetime.now(timezone.utc) - timedelta(days=60),
        schema_format="GeoTIFF",
        crs="EPSG:32643",
        vertical_datum="EGM96",
        resolution_meters=30.0,
        units="meters",
        nodata_value="-9999",
        raw_payload_checksum=checksum,
        claimed_checksum=checksum,
        reviewer_id="REV-CHIEF-GEOMATICS",
        reproducibility_notes="Acquired via CDSE OData API, GDAL 3.8 warp pipeline.",
        cost_usd=0.0,
    )

    result = source_service.validate_aoi_sample(sample, actor_id="test-geomatician")
    assert result.passed is True
    assert result.status == "APPROVED_FOR_USE"
    assert len(result.quarantine_reasons) == 0
    assert result.checks["license_redistribution_offline"] is True
    assert result.checks["aoi_spatial_extent"] is True
    assert result.checks["crs_valid"] is True
    assert result.checks["checksum_match"] is True

    cap = source_service.get_capability("S18")
    assert cap.state == ActivationState.APPROVED_FOR_USE
    assert cap.latest_checksum == checksum


def test_aoi_sample_gate_quarantine_on_corrupt_checksum_or_missing_crs(source_service):
    """
    FR-078, RUL-077 & AT-38: Corrupted checksum or missing CRS immediately
    transitions source to QUARANTINED and halts dependent workflows.
    """
    # 1. Corrupt checksum
    corrupt_sample = AOISampleGateInput(
        source_id="S08",
        sample_id="SAMPLE-S2-CORRUPT-001",
        license_type="Copernicus Open Access",
        has_redistribution_and_offline_rights=True,
        min_lat=11.60,
        max_lat=11.75,
        min_lon=76.00,
        max_lon=76.20,
        observation_timestamp=datetime.now(timezone.utc) - timedelta(days=10),
        schema_format="COG",
        crs="EPSG:32643",
        resolution_meters=10.0,
        units="reflectance",
        nodata_value="0",
        raw_payload_checksum="0000000000000000000000000000000000000000000000000000000000000000",
        claimed_checksum="ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
        reviewer_id="REV-RS-SPECIALIST",
        reproducibility_notes="Automated STAC ingest pipeline.",
    )
    result = source_service.validate_aoi_sample(corrupt_sample, actor_id="test-actor")
    assert result.passed is False
    assert result.status == "QUARANTINED"
    assert any("Checksum mismatch" in r for r in result.quarantine_reasons)

    cap = source_service.get_capability("S08")
    assert cap.state == ActivationState.QUARANTINED
    assert "Checksum mismatch" in cap.quarantine_reason

    # 2. Missing CRS
    missing_crs_sample = AOISampleGateInput(
        source_id="S22",
        sample_id="SAMPLE-BUILDINGS-NO-CRS",
        license_type="CC-BY-4.0",
        has_redistribution_and_offline_rights=True,
        min_lat=11.60,
        max_lat=11.75,
        min_lon=76.00,
        max_lon=76.20,
        observation_timestamp=datetime.now(timezone.utc) - timedelta(days=30),
        schema_format="GeoJSON",
        crs="",  # Missing CRS!
        resolution_meters=0.5,
        units="polygon",
        nodata_value=None,
        raw_payload_checksum="abc123hash",
        claimed_checksum="abc123hash",
        reviewer_id="REV-GIS-SPECIALIST",
        reproducibility_notes="Google Open Buildings v3 extract.",
    )
    result_crs = source_service.validate_aoi_sample(missing_crs_sample, actor_id="test-actor")
    assert result_crs.passed is False
    assert result_crs.status == "QUARANTINED"
    assert any("Invalid or missing Coordinate Reference System" in r for r in result_crs.quarantine_reasons)


def test_mirror_group_deduplication_and_no_double_counting(source_service):
    """
    FR-079, RUL-078 & AT-32: Observations from mirror platforms sharing the same underlying
    data take (e.g. CDSE S10, Earth Search S11, Planetary Computer S12) are deduplicated;
    counting them as independent evidence is strictly rejected.
    """
    groups = source_service.list_mirror_groups()
    assert len(groups) == 5

    s2_group = next(g for g in groups if g.group_id == "MIRROR_SENTINEL_2")
    assert s2_group.primary_source_id == "S10"
    assert s2_group.reconciliation_strategy == ReconciliationMethod.PRIMARY_AUTHORITATIVE

    # Simulate 3 mirror catalog hits for the identical underlying Sentinel observation
    granule_id = "S2A_MSIL2A_20240801T050701_N0511_R019_T43PFR_20240801T084802"
    observations = [
        {"source_id": "S10", "observation_id": granule_id, "provider": "CDSE", "cloud_cover": 14.2},
        {"source_id": "S11", "observation_id": granule_id, "provider": "EarthSearch", "cloud_cover": 14.2},
        {"source_id": "S12", "observation_id": granule_id, "provider": "PlanetaryComputer", "cloud_cover": 14.2},
    ]

    req = ObservationReconciliationRequest(
        group_id="MIRROR_SENTINEL_2",
        observations=observations,
    )
    res = source_service.reconcile_mirror_observations(req, actor_id="test-analyst")

    assert res.total_input_count == 3
    assert res.reconciled_count == 1
    assert res.duplicate_count == 2
    assert res.is_independent_corroboration_rejected is True
    assert "strictly rejected" in res.explanation


def test_geography_and_paused_api_lockout(source_service):
    """
    FR-080, RUL-080, AT-33 & AT-34:
    - C-FLOOD (S07) is locked out of Wayanad/Kerala (Godavari, Tapi, Mahanadi only).
    - Glacial lake (S52) is locked out of Kerala.
    - SoilGrids REST (S31) is locked out because the API is paused.
    """
    # 1. C-FLOOD on Wayanad
    res_cflood = source_service.check_geography_and_governance("S07", "Wayanad, Kerala")
    assert res_cflood["can_link"] is False
    assert res_cflood["status"] == "UNSUPPORTED_GEOGRAPHY"
    assert res_cflood["hold"] is True

    # 2. Himalayan Glacial Lake on Kerala
    res_lake = source_service.check_geography_and_governance("S52", "Kerala")
    assert res_lake["can_link"] is False
    assert res_lake["status"] == "UNSUPPORTED_GEOGRAPHY"

    # 3. SoilGrids REST
    res_soil = source_service.check_geography_and_governance("S31", "Wayanad")
    assert res_soil["can_link"] is False
    assert res_soil["status"] == "PAUSED_API"
    assert res_soil["hold"] is True


def test_provider_health_telemetry_with_secret_redaction(source_service):
    """
    FR-081 & RUL-081: Provider health telemetry exposes quota, latency, and error rate
    while strictly redacting all secrets and tokens.
    """
    health_list = source_service.get_provider_health()
    assert len(health_list) >= 6

    for h in health_list:
        assert h.secrets_redacted is True
        assert h.redacted_token_preview == "***REDACTED***"
        # Verify no token or secret leaks in dump
        dump = h.model_dump_json()
        assert "password" not in dump.lower()
        assert "bearer" not in dump.lower()
        assert "secret_key" not in dump.lower()


def test_s45_s50_dependency_blocker_gate_enforcement(source_service):
    """
    FR-083, RUL-079 & AT-35: Production site approval or beneficiary allocation
    must verify S45-S50 mandatory blocker evidence. If ANY is missing, action is BLOCKED.
    """
    # Scenario A: Missing water and geotechnical evidence
    req_incomplete = DependencyBlockerEvaluationRequest(
        site_id="SITE-WAYANAD-ELSTONE-01",
        allocation_action="LIVE_SITE_APPROVAL",
        evidence_records={
            "S45": {"status": "VERIFIED", "summary": "Tahsildar clear title certificate #TR-2024-918"},
            "S47": {"status": "VERIFIED", "summary": "Forest Dept NOC & Gram Sabha FRA clearance"},
            "S48": {"status": "VERIFIED", "consent_percentage": 100},
            "S50": {"status": "VERIFIED", "administrative_sanction_number": "GO-452-2024-DMD"},
            # S46 (water) and S49 (geotechnics) missing!
        },
    )
    rep_incomplete = source_service.evaluate_production_blockers(req_incomplete, actor_id="test-official")
    assert rep_incomplete.can_proceed is False
    assert rep_incomplete.overall_status == "BLOCKED"
    assert len(rep_incomplete.blocking_reasons) >= 2
    assert "S46" in rep_incomplete.blockers
    assert rep_incomplete.blockers["S46"].status.value == "MISSING"

    # Scenario B: Water yield inadequate (< 55 LPCD)
    req_low_water = DependencyBlockerEvaluationRequest(
        site_id="SITE-WAYANAD-ELSTONE-01",
        allocation_action="BENEFICIARY_ALLOCATION",
        evidence_records={
            "S45": {"status": "VERIFIED", "summary": "Clear title verified"},
            "S46": {"status": "VERIFIED", "sustainable_yield_lpcd": 35, "potability_certified": True},  # 35 < 55 LPCD!
            "S47": {"status": "VERIFIED", "summary": "FRA cleared"},
            "S48": {"status": "VERIFIED", "consent_percentage": 100},
            "S49": {"status": "VERIFIED", "factor_of_safety": 1.4},
            "S50": {"status": "VERIFIED", "administrative_sanction_number": "GO-452-2024-DMD"},
        },
    )
    rep_low_water = source_service.evaluate_production_blockers(req_low_water, actor_id="test-official")
    assert rep_low_water.can_proceed is False
    assert rep_low_water.overall_status == "BLOCKED"
    assert any("S46" in r for r in rep_low_water.blocking_reasons)

    # Scenario C: Complete S45-S50 evidence -> PASS
    req_complete = DependencyBlockerEvaluationRequest(
        site_id="SITE-WAYANAD-ELSTONE-01",
        allocation_action="LIVE_SITE_APPROVAL",
        evidence_records={
            "S45": {"status": "VERIFIED", "summary": "Resurvey cadastral RoR verified by Tahsildar"},
            "S46": {"status": "VERIFIED", "sustainable_yield_lpcd": 65, "potability_certified": True},
            "S47": {"status": "VERIFIED", "summary": "FRA Gram Sabha resolution #14/2024 and Forest NOC"},
            "S48": {"status": "VERIFIED", "consent_percentage": 100},
            "S49": {"status": "VERIFIED", "factor_of_safety": 1.45},
            "S50": {"status": "VERIFIED", "administrative_sanction_number": "GO-452-2024-DMD"},
        },
    )
    rep_complete = source_service.evaluate_production_blockers(req_complete, actor_id="test-official")
    assert rep_complete.can_proceed is True
    assert rep_complete.overall_status == "PASS"
    assert len(rep_complete.blocking_reasons) == 0
    assert len(rep_complete.audit_hash) == 64


def test_basemap_decoupling_and_non_map_fallback(source_service):
    """
    FR-084, RUL-082 & AT-37: Basemap is display infrastructure;
    bulk OSM tile download is prohibited, and basemap failure
    keeps analytical lineage 100% intact with non-map tabular fallback.
    """
    cfg = source_service.get_basemap_config("S53")
    assert cfg.prohibit_osm_tile_bulk_download is True
    assert cfg.pixels_decoupled_from_analytical_lineage is True

    fb_res = source_service.simulate_basemap_failure_fallback("S53", actor_id="test-sysadmin")
    assert fb_res["basemap_healthy"] is False
    assert fb_res["analytical_lineage_decoupled"] is True
    assert fb_res["fallback_mode"] == "NON_MAP_TABULAR_VECTOR"
    assert "Analytical decisions remain 100% unaffected" in fb_res["message"]
