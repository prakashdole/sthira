"""
Unit and Contract Test Suite for Phase 12: Formula, Parameter & Statutory Compliance Control
Normative Reference: trd.md (§3.12, FR-071–FR-075), rules.md (RUL-061–RUL-066),
equations.md, parameters.md, DEC-043.
"""

import pytest
from punarvas.modules.policy import (
    FormulaClassification,
    RegistryState,
    StatuteApplicabilityState,
    FormulaNotActivatedError,
    UnvalidatedSpecialistFormulaError,
    RejectedFormulaExecutionError,
    NumericalDomainError,
    MissingFormulaInputError,
    FormulaDefinition,
    ParameterDefinition,
    FormulaExecutionRequest,
    StatutoryComplianceRecord,
    ComplianceService,
)


@pytest.fixture
def compliance():
    """Create an isolated instance of ComplianceService for tests."""
    return ComplianceService()


def test_registry_seeded_correctly(compliance: ComplianceService):
    """Test FR-071 / RUL-061: Seeded registry contains expected formulas, parameters, and statutes."""
    # 1. Formulas
    e01 = compliance.get_formula("E01")
    assert e01 is not None
    assert e01.classification == FormulaClassification.CORE
    assert e01.sha256_hash != ""

    e37 = compliance.get_formula("E37")
    assert e37 is not None
    assert e37.classification == FormulaClassification.SPECIALIST_EXTERNAL

    e60 = compliance.get_formula("E60")
    assert e60 is not None
    assert e60.classification == FormulaClassification.REJECTED

    # 2. Parameters
    p006 = compliance.get_parameter("PAR-006")
    assert p006 is not None
    assert p006.value == 55.0
    assert p006.unit == "LPCD"
    assert "S46" in p006.source_capability_ids

    p007 = compliance.get_parameter("PAR-007")
    assert p007 is not None
    assert p007.value is None  # Must remain UNSET per parameters.md

    # 3. Statutes
    dma = compliance.get_statute("DMA-2005")
    assert dma is not None
    assert dma.applicability_state == StatuteApplicabilityState.MANDATORY_IN_FORCE
    assert "CONTROL_BIENNIAL_PLAN_UPDATE_CADENCE" in dma.mandatory_controls


def test_allow_list_policy_activation(compliance: ComplianceService):
    """Test FR-072: Only CORE formulas explicitly activated by approved policy may execute."""
    # Register policy activation with only E01 and E11
    act = compliance.create_policy_activation(
        policy_id="POL-WAYANAD-01",
        programme_id="PROG-WYD-REHAB",
        activated_formula_ids=["E01", "E11"],
        authorized_by="DDMA_CHAIRPERSON",
    )
    assert act.activation_id.startswith("ACT-POL-WAYANAD-01")

    # E01 is activated -> executes successfully
    req = FormulaExecutionRequest(
        formula_id="E01",
        inputs={"mode": "L_S_TO_L_DAY", "val": 1.0, "operating_hours": 24.0},
        policy_activation_id=act.activation_id,
        reviewer_id="ENG_OFFICER_01",
    )
    res = compliance.execute_formula(req)
    assert res.status == "SUCCESS"
    assert res.computed_value == 86400.0

    # E13 is not activated under this policy -> raises FormulaNotActivatedError
    req_unactivated = FormulaExecutionRequest(
        formula_id="E13",
        inputs={"total_area_sqm": 50000.0, "excluded_areas_sqm": [10000.0]},
        policy_activation_id=act.activation_id,
        reviewer_id="ENG_OFFICER_01",
    )
    with pytest.raises(FormulaNotActivatedError) as exc:
        compliance.execute_formula(req_unactivated)
    assert "Formula 'E13' is not activated" in str(exc.value)

    # Attempting to activate a REJECTED formula fails
    with pytest.raises(RejectedFormulaExecutionError) as exc_rej:
        compliance.create_policy_activation(
            policy_id="POL-ILLEGAL-01",
            programme_id="PROG-WYD-REHAB",
            activated_formula_ids=["E60"],
            authorized_by="BAD_ACTOR",
        )
    assert "Cannot activate rejected formula" in str(exc_rej.value)


def test_deny_list_rejected_and_specialist_formulas(compliance: ComplianceService):
    """Test RUL-035, RUL-061, RUL-066: Prohibit execution of rejected & specialist formulas."""
    # E60 (Opaque Omega score) strictly prohibited
    req_e60 = FormulaExecutionRequest(
        formula_id="E60",
        inputs={"H": 0.5, "E": 0.4, "V": 0.8, "C": 0.5},
    )
    with pytest.raises(RejectedFormulaExecutionError) as exc_e60:
        compliance.execute_formula(req_e60)
    assert "RUL-035 / RUL-061 explicitly prohibits" in str(exc_e60.value)

    # E55 (Unverified peak flood empirical power law) prohibited
    req_e55 = FormulaExecutionRequest(
        formula_id="E55",
        inputs={"V": 1000.0},
    )
    with pytest.raises(RejectedFormulaExecutionError):
        compliance.execute_formula(req_e55)

    # E37 (Infinite slope stability specialist formula) fails without local package
    req_e37 = FormulaExecutionRequest(
        formula_id="E37",
        inputs={"beta": 25.0, "c_prime": 12.0},
    )
    with pytest.raises(UnvalidatedSpecialistFormulaError) as exc_e37:
        compliance.execute_formula(req_e37)
    assert "cannot execute natively" in str(exc_e37.value)


def test_numerical_validation_and_domain_guards(compliance: ComplianceService):
    """Test FR-073 / RUL-063: Boundary checks, domain guards, and non-coercion of missing inputs."""
    # 1. E01: Negative quantity or invalid hours
    with pytest.raises(NumericalDomainError):
        compliance.execute_formula(
            FormulaExecutionRequest(
                formula_id="E01",
                inputs={"mode": "L_S_TO_L_DAY", "val": -5.0},
            )
        )
    with pytest.raises(NumericalDomainError):
        compliance.execute_formula(
            FormulaExecutionRequest(
                formula_id="E01",
                inputs={"mode": "L_S_TO_L_DAY", "val": 1.0, "operating_hours": 30.0},
            )
        )

    # 2. E06: Weights sum must equal 1.0
    with pytest.raises(NumericalDomainError):
        compliance.execute_formula(
            FormulaExecutionRequest(
                formula_id="E06",
                inputs={
                    "weights": {"hazard": 0.5, "water": 0.3},  # Sum = 0.8 != 1.0
                    "values": {"hazard": 0.9, "water": 0.8},
                },
            )
        )

    # 3. E06: Missing input value returns UNKNOWN (RUL-063: Never coerced to 0 or PASS)
    res_missing = compliance.execute_formula(
        FormulaExecutionRequest(
            formula_id="E06",
            inputs={
                "weights": {"hazard": 0.5, "water": 0.5},
                "values": {"hazard": 0.8, "water": None},  # Missing water
            },
        )
    )
    assert res_missing.status == "UNKNOWN"
    assert res_missing.computed_value is None
    assert any("Missing value for criterion 'water'" in w for w in res_missing.warnings)

    # 4. E13: Excluded area exceeding total area
    with pytest.raises(NumericalDomainError):
        compliance.execute_formula(
            FormulaExecutionRequest(
                formula_id="E13",
                inputs={"total_area_sqm": 10000.0, "excluded_areas_sqm": [6000.0, 5000.0]},
            )
        )


def test_water_carrying_capacity_calculation(compliance: ComplianceService):
    """Test E11 JJM 55 LPCD benchmark fixture arithmetic (equations.md Section I Fixture 4)."""
    # 1 L/s continuous discharge = 86,400 L/day.
    # 20% delivery loss = 69,120 L/day net.
    # Supported people = floor(69120 / 55) = 1,256 people.
    # For population 1,000 -> required = 55,000 -> sufficient!
    res = compliance.execute_formula(
        FormulaExecutionRequest(
            formula_id="E11",
            inputs={
                "tested_yield_lpcd": 86400.0,
                "delivery_loss_pct": 20.0,
                "population": 1000,
            },
        )
    )
    assert res.status == "SUCCESS"
    data = res.computed_value
    assert data["net_yield_lpcd"] == 69120.0
    assert data["required_lpcd"] == 55000.0
    assert data["is_sufficient"] is True
    assert data["max_supported_people"] == 1256


def test_funding_gap_excludes_announced_budgets(compliance: ComplianceService):
    """Test E17 / RUL-069: Announced budget is excluded from funding gap calculation."""
    res = compliance.execute_formula(
        FormulaExecutionRequest(
            formula_id="E17",
            inputs={
                "required_costs": [5000000.0, 3000000.0],  # Total = ₹80,00,000
                "verified_funds": [2000000.0],              # Verified = ₹20,00,000
                "announced_budgets": [6000000.0],          # Announced = ₹60,00,000
            },
        )
    )
    assert res.status == "SUCCESS"
    data = res.computed_value
    # Gap must be ₹80L - ₹20L = ₹60L, NOT ₹0!
    assert data["funding_gap"] == 6000000.0
    assert any("Announced budget of ₹6,000,000.00 is excluded" in w for w in res.warnings)


def test_lineage_replay_and_bit_for_bit_identity(compliance: ComplianceService):
    """Test FR-074: Historical execution audit record and exact replay verification."""
    req = FormulaExecutionRequest(
        formula_id="E06",
        inputs={
            "weights": {"accessibility": 0.4, "infrastructure": 0.6},
            "values": {"accessibility": 0.75, "infrastructure": 0.90},
        },
        reviewer_id="OFFICER_REPLAY_TEST",
    )
    initial_res = compliance.execute_formula(req)
    assert initial_res.status == "SUCCESS"
    assert initial_res.computed_value == 0.84
    assert initial_res.execution_sha256 != ""

    # Replay historical execution
    replay = compliance.reproduce_formula_execution(initial_res.execution_id)
    assert replay["is_bit_for_bit_identical"] is True
    assert replay["original_hash"] == replay["reproduced_hash"]
    assert replay["reproduced_value"] == 0.84


def test_statutory_compliance_posture_evaluation(compliance: ComplianceService):
    """Test FR-075: Statutory compliance register verification and blocker detection."""
    # 1. Fully compliant case
    full_controls = [
        "CONTROL_DDMA_APPROVAL_MANDATORY",
        "CONTROL_BIENNIAL_PLAN_UPDATE_CADENCE",
        "CONTROL_PROHIBIT_AUTONOMOUS_GAZETTE",
        "CONTROL_REHABILITATION_SCHEME_PUBLICATION",
        "CONTROL_INFRASTRUCTURE_AMENITIES_VERIFIED",
        "CONTROL_PROHIBIT_DISPLACEMENT_WITHOUT_REMEDY",
        "CONTROL_GRAMA_SABHA_CONSENT_MANDATORY",
        "CONTROL_PROHIBIT_EVICTION_PENDING_FRA",
        "CONTROL_COMMUNITY_RIGHTS_PRESERVED",
        "CONTROL_INDIA_RESIDENT_HOSTING_ONLY",
        "CONTROL_PURPOSE_SPECIFIC_CONSENT_SEPARATION",
        "CONTROL_RESTRICTED_FIELD_TOKENIZATION",
        "CONTROL_K_ANONYMITY_PUBLIC_PROJECTIONS",
        "CONTROL_NTP_CLOCK_SYNC",
        "CONTROL_180_DAY_AUDIT_LOG_RETENTION",
        "CONTROL_TAMPER_EVIDENT_HASH_CHAINING",
    ]
    eval_full = compliance.verify_compliance_posture("PROG-WYD-FULL", full_controls)
    assert eval_full.is_fully_compliant is True
    assert len(eval_full.open_blockers) == 0

    # 2. Incomplete case missing DPDP and FRA controls
    partial_controls = [
        "CONTROL_DDMA_APPROVAL_MANDATORY",
        "CONTROL_BIENNIAL_PLAN_UPDATE_CADENCE",
        "CONTROL_NTP_CLOCK_SYNC",
    ]
    eval_partial = compliance.verify_compliance_posture("PROG-WYD-PARTIAL", partial_controls)
    assert eval_partial.is_fully_compliant is False
    assert len(eval_partial.open_blockers) > 0
    assert "CONTROL_GRAMA_SABHA_CONSENT_MANDATORY" in eval_partial.open_blockers
    assert "CONTROL_INDIA_RESIDENT_HOSTING_ONLY" in eval_partial.open_blockers
