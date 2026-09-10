"""
Tests for Phase 11: Government Dossiers, Evidence-Bound Field Checklists,
Spatial Exports, Tamper-Evident Manifests, and Public Transparency Projections (ARC-C10 / FEAT-018, FEAT-019).
Normative Reference: plan.md (#11), trd.md (§3.9, FR-053-FR-058, NFR-019-NFR-022, AT-12, AT-13, AT-14, AT-21, AT-23),
rules.md (RUL-052-RUL-060, RUL-075), DEC-012, DEC-013, DEC-042.
"""

import json
import pytest
from sthira.modules.reporting import (
    ReportingService,
    EvidenceReference,
    ExportClassification,
    ExportType,
    FieldChecklistItem,
)


@pytest.fixture
def service():
    return ReportingService()


def test_site_dossier_generation_and_manifest(service):
    """Verify FR-053 / FEAT-018: Evidence-bound Site Dossier with registered manifest."""
    evidence = [
        EvidenceReference(
            field_name="hazard_buffer_distance_m",
            statement="Site boundary is 350m outside designated 2024 debris flow runout zone",
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

    dossier = service.generate_site_dossier(
        site_id="SITE-NEDUMBALA-01",
        site_name="Nedumbala Resettlement Zone",
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
        generating_user_id="chief_town_planner",
        approval_ref="ORD-DDMA-WYD-2025-012",
    )

    assert dossier.site_id == "SITE-NEDUMBALA-01"
    assert dossier.dwelling_capacity == 60
    assert len(dossier.evidence_links) == 2
    assert dossier.unresolved_conditions == ["COND-FRA-NOC-01"]
    assert len(dossier.sha256_checksum) == 64

    # Verify manifest was registered
    manifest = service.get_manifest(dossier.manifest_id)
    assert manifest is not None
    assert manifest.export_type == ExportType.SITE_DOSSIER.value
    assert manifest.sha256_checksum == dossier.sha256_checksum
    assert manifest.record_count == 1


def test_beneficiary_review_pack_generation(service):
    """Verify FR-053 / FEAT-018: Beneficiary Review Pack with vulnerability & scheme links."""
    evidence = [
        EvidenceReference(
            field_name="vulnerability_score",
            statement="Senior citizen headed household with disabled dependent",
            source_id="S49_FIELD_SURVEY",
            evidence_hash="hash_survey_hh_007",
            is_verified=True,
        )
    ]

    pack = service.generate_beneficiary_review_pack(
        household_id="HH-WYD-900",
        head_of_household="Pathumma K.",
        member_count=4,
        vulnerability_score=85.0,
        disability_or_special_needs=True,
        tenure_category="OWNER",
        relocation_necessity_review_id="REV-NEC-009",
        preferred_pathway="TOWNSHIP",
        assigned_site_id="SITE-NEDUMBALA-01",
        eligible_schemes=["PUNARJANI_LAND_GRANT", "LIFE_MISSION_HOUSING"],
        evidence_links=evidence,
        consent_token_ref="CONSENT-TKN-WYD-900",
        unresolved_conditions=[],
        generating_user_id="social_welfare_officer",
    )

    assert pack.household_id == "HH-WYD-900"
    assert pack.vulnerability_score == 85.0
    assert pack.disability_or_special_needs is True
    assert "LIFE_MISSION_HOUSING" in pack.eligible_schemes
    assert service.get_manifest(pack.manifest_id) is not None


def test_field_verification_checklist(service):
    """Verify FR-053: Engineering field verification checklist with mandatory items."""
    items = [
        FieldChecklistItem(
            item_id="CHK-01",
            description="Verify physical all-weather road access width >= 3.66m",
            mandatory=True,
            verification_method="DGPS_SURVEY_WHEEL",
            status="UNKNOWN",
        ),
        FieldChecklistItem(
            item_id="CHK-02",
            description="Inspect crown scarp distance from proposed layout perimeter",
            mandatory=True,
            verification_method="LASER_RANGEFINDER",
            status="UNKNOWN",
        ),
        FieldChecklistItem(
            item_id="CHK-03",
            description="Check borewell casing elevation above 100-yr high flood line",
            mandatory=True,
            verification_method="TOTAL_STATION_ELEVATION",
            status="UNKNOWN",
        ),
    ]

    chk = service.generate_field_verification_checklist(
        target_type="SITE",
        target_id="SITE-NEDUMBALA-01",
        items=items,
        required_equipment=["Trimble DGPS", "Leica Total Station", "Water Depth Probe"],
        safety_precautions=["Hard hats mandatory", "No inspection during rainfall > 15mm/hr"],
        generating_user_id="field_executive_engineer",
    )

    assert len(chk.items) == 3
    assert chk.target_id == "SITE-NEDUMBALA-01"
    manifest = service.get_manifest(chk.manifest_id)
    assert manifest is not None
    assert manifest.record_count == 3


def test_decision_summary_dossier_provenance(service):
    """Verify FEAT-020 / R3-01: Decision Summary Dossier with pinned solver seed & checksums."""
    dossier = service.generate_decision_summary_dossier(
        decision_id="DEC-ALLOC-BATCH-01",
        entity_type="ALLOCATION_SCENARIO",
        entity_id="SCEN-WYD-2025-A",
        policy_version="POL-WYD-2024.1",
        source_checksums={"S01": "hash_s01", "S04": "hash_s04", "S06": "hash_s06"},
        solver_seed=101,
        solver_tolerances={"mip_gap": 0.005, "time_limit_sec": 120.0},
        approval_order_id="GO(P)-REV-2025-09",
        statutory_gazette_id="GAZ-KL-WYD-2025-88",
        objection_token_refs=["RCPT-Sthira-OBJ-001"],
        evidence_chain_hash="chain_head_hash_9827361",
        generating_user_id="appellate_clerk",
    )

    assert dossier.decision_id == "DEC-ALLOC-BATCH-01"
    assert dossier.solver_seed == 101
    assert dossier.approval_order_id == "GO(P)-REV-2025-09"
    manifest = service.get_manifest(dossier.manifest_id)
    assert manifest.classification == ExportClassification.JUDICIAL_AUDIT


def test_spatial_geojson_and_tabular_csv_exports(service):
    """Verify FR-055: RFC 7946 GeoJSON and RFC 4180 CSV exports with sealed manifests."""
    # 1. GeoJSON
    features = [
        {
            "type": "Feature",
            "geometry": {
                "type": "Point",
                "coordinates": [76.1285, 11.5242],
            },
            "properties": {
                "site_id": "SITE-ELSTONE-01",
                "capacity": 80,
                "status": "PASS",
            },
        }
    ]
    geo_res = service.generate_spatial_geojson_export(
        export_id="EXPORT-SITES-01",
        features_data=features,
        generating_user_id="gis_analyst",
    )
    assert geo_res["geojson"]["type"] == "FeatureCollection"
    assert geo_res["geojson"]["crs"]["properties"]["name"] == "urn:ogc:def:crs:OGC:1.3:CRS84"
    assert service.get_manifest(geo_res["manifest"].manifest_id) is not None

    # 2. CSV
    headers = ["Household_ID", "Head_Name", "Pathway", "Eligible"]
    rows = [
        ["HH-001", "Raman K.", "TOWNSHIP", "YES"],
        ["HH-002", "Sita M.", "SELF_RELOCATION", "YES"],
    ]
    csv_res = service.generate_tabular_csv_export(
        export_id="EXPORT-TABULAR-01",
        headers=headers,
        rows=rows,
        generating_user_id="clerk",
    )
    assert "Household_ID,Head_Name,Pathway,Eligible" in csv_res["csv_content"]
    assert "HH-001,Raman K.,TOWNSHIP,YES" in csv_res["csv_content"]


def test_manifest_verification_and_tamper_detection(service):
    """Verify FR-056 / FR-057: Tamper-evidence of export manifests."""
    data = {"report": "Official Evacuation Zone Summary", "records": 42}
    raw_str = json.dumps(data, sort_keys=True)
    valid_hash = service.create_export_manifest(
        manifest_id="MAN-TEST-VALID-01",
        export_type="TEST_REPORT",
        generating_user_id="test_officer",
        sha256_checksum=__import__("hashlib").sha256(raw_str.encode("utf-8")).hexdigest(),
    )

    # Verification with exact payload: PASS
    verify_pass = service.verify_export_manifest("MAN-TEST-VALID-01", raw_str)
    assert verify_pass["verified"] is True

    # Verification with tampered payload: FAIL
    tampered_str = json.dumps({"report": "TAMPERED Evacuation Zone Summary", "records": 42}, sort_keys=True)
    verify_fail = service.verify_export_manifest("MAN-TEST-VALID-01", tampered_str)
    assert verify_fail["verified"] is False
    assert "mismatch" in verify_fail["reason"].lower()


def test_k_anonymity_and_differencing_attack_defense(service):
    """Verify RUL-075 / FEAT-019 / AT-23: k-anonymity (k >= 5) and differencing detection."""
    # Round 1: Small cells (< 5) must be suppressed
    round1_counts = {
        "Meppadi_Ward_1": 24,
        "Meppadi_Ward_2": 3,  # < 5: MUST BE SUPPRESSED!
        "Vellarimala_Ward_4": 18,
    }
    proj1 = service.generate_k_anonymized_public_projection(
        projection_id="PUB-PROJ-WYD-01",
        district="Wayanad",
        round_number=1,
        subregion_counts=round1_counts,
        k_threshold=5,
    )
    assert proj1.cell_suppression_applied is True
    assert proj1.suppressed_cell_count == 1
    assert "< 5" in str(proj1.aggregates["Meppadi_Ward_2"])
    assert proj1.differencing_risk_detected is False

    # Round 2: A delta of 1 or 2 triggers differencing attack detection
    round2_counts = {
        "Meppadi_Ward_1": 25,  # Delta of 1! Triggers differencing detection
        "Meppadi_Ward_2": 3,
        "Vellarimala_Ward_4": 18,
    }
    proj2 = service.generate_k_anonymized_public_projection(
        projection_id="PUB-PROJ-WYD-02",
        district="Wayanad",
        round_number=2,
        subregion_counts=round2_counts,
        k_threshold=5,
    )
    assert proj2.differencing_risk_detected is True
    assert "differencing attack risk" in proj2.differencing_warning.lower()


def test_reproduce_historical_export(service):
    """Verify FR-058: Bit-for-bit historical export reproduction."""
    payload = "DETERMINISTIC_REPORT_LINE_01\nDETERMINISTIC_REPORT_LINE_02\n"
    checksum = __import__("hashlib").sha256(payload.encode("utf-8")).hexdigest()
    manifest_id = "MAN-HISTORICAL-001"

    service.create_export_manifest(
        manifest_id=manifest_id,
        export_type="ACCESSIBLE_REPORT",
        generating_user_id="archivist",
        sha256_checksum=checksum,
    )

    res_exact = service.reproduce_historical_export(manifest_id, checksum)
    assert res_exact["reproducible"] is True
    assert res_exact["status"] == "BIT_FOR_BIT_IDENTICAL"

    res_diff = service.reproduce_historical_export(manifest_id, "different_checksum_hash")
    assert res_diff["reproducible"] is False
    assert res_diff["status"] == "CONTENT_MISMATCH"
