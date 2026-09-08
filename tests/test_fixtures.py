"""
Tests for Synthetic Wayanad Fixtures & Spikes (C0-01).
"""

from punarvas.spikes import (
    load_wayanad_fixture,
    load_idukki_fixture,
    load_alappuzha_fixture,
    load_district_fixture,
)


def test_synthetic_wayanad_fixture_integrity():
    data = load_wayanad_fixture()
    assert "_metadata" in data
    assert len(data["_metadata"]["checksum_sha256"]) == 64
    assert data["programme"]["district"] == "Wayanad"
    assert len(data["affected_parcels"]) == 4
    assert len(data["candidate_sites"]) == 3
    assert len(data["synthetic_households"]) == 4

    # Verify Elstone Estate candidate site
    elstone = next(s for s in data["candidate_sites"] if s["site_id"] == "SITE-ELSTONE-01")
    assert elstone["dwelling_capacity"] == 250
    assert elstone["unit_plot_cents"] == 7.0
    assert elstone["lean_season_water_state"] == "PASS"
    assert elstone["tested_water_lpcd"] >= 55.0  # Above JJM baseline (RUL-030)


def test_synthetic_idukki_fixture_integrity():
    """Verify C4-03 / RUL-017: Independent Idukki fixture without Wayanad assumptions."""
    data = load_idukki_fixture()
    assert "_metadata" in data
    assert len(data["_metadata"]["checksum_sha256"]) == 64
    assert data["programme"]["district"] == "Idukki"
    assert len(data["affected_parcels"]) == 2
    assert len(data["candidate_sites"]) == 1
    assert len(data["synthetic_households"]) == 2

    site = data["candidate_sites"][0]
    assert site["site_id"] == "SITE-IDU-MUNNAR-01"
    assert site["road_access_width_m"] >= 4.0  # Mountain tea road specification
    assert site["tested_water_lpcd"] >= 55.0


def test_synthetic_alappuzha_fixture_integrity():
    """Verify C4-03 / RUL-017: Independent Alappuzha fixture without debris flow assumptions."""
    data = load_alappuzha_fixture()
    assert "_metadata" in data
    assert len(data["_metadata"]["checksum_sha256"]) == 64
    assert data["programme"]["district"] == "Alappuzha"
    assert len(data["affected_parcels"]) == 2
    assert len(data["candidate_sites"]) == 1
    assert len(data["synthetic_households"]) == 2

    # Verify no debris flow channel in coastal lowland polders
    for hz in data["hazard_layers"]:
        assert hz["debris_flow_channel"] is False

    site = data["candidate_sites"][0]
    assert site["site_id"] == "SITE-ALP-KUTTANAD-01"
    assert site["slope_degrees"] < 5.0  # Flat delta terrain
    assert site["tested_water_lpcd"] >= 55.0

