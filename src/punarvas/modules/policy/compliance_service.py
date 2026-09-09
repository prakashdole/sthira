"""
PUNARVAS-AI Formula, Parameter & Statutory Compliance Control Engine (Phase 12 / ARC-C07).
Normative Reference: trd.md (§3.12, FR-071–FR-075), rules.md (RUL-061–RUL-066),
equations.md, parameters.md, DEC-043.
"""

from enum import Enum
import hashlib
import json
import math
from typing import Any, Dict, List, Optional, Tuple
from pydantic import BaseModel, Field

from punarvas.core.contracts import utc_now
from punarvas.core.audit import global_audit_ledger
from punarvas.core.errors import PunarvasError


# --- Enums ---

class FormulaClassification(str, Enum):
    CORE = "CORE"
    OPTIONAL = "OPTIONAL"
    SPECIALIST_EXTERNAL = "SPECIALIST_EXTERNAL"
    REJECTED = "REJECTED"


class RegistryState(str, Enum):
    DRAFT = "DRAFT"
    VALIDATED = "VALIDATED"
    APPROVED = "APPROVED"
    SUSPENDED = "SUSPENDED"
    SUPERSEDED = "SUPERSEDED"
    REJECTED = "REJECTED"


class StatuteApplicabilityState(str, Enum):
    MANDATORY_IN_FORCE = "MANDATORY_IN_FORCE"
    CONDITIONAL = "CONDITIONAL"
    REPEALED = "REPEALED"
    SUPERSEDED = "SUPERSEDED"


class JurisdictionLevel(str, Enum):
    UNION_OF_INDIA = "UNION_OF_INDIA"
    STATE_OF_KERALA = "STATE_OF_KERALA"
    DISTRICT_WAYANAD = "DISTRICT_WAYANAD"


# --- Custom Domain Errors ---

class FormulaNotActivatedError(PunarvasError):
    """Raised when an unactivated formula is executed in a decision workflow (FR-072)."""
    def __init__(self, formula_id: str, policy_id: str):
        super().__init__(f"Formula '{formula_id}' is not activated under policy '{policy_id}'. Only CORE activated formulas may execute.")


class UnvalidatedSpecialistFormulaError(PunarvasError):
    """Raised when a SPECIALIST_EXTERNAL formula is invoked natively without local validation (RUL-066)."""
    def __init__(self, formula_id: str):
        super().__init__(f"Specialist formula '{formula_id}' cannot execute natively. It requires a separately commissioned and independently validated local package.")


class RejectedFormulaExecutionError(PunarvasError):
    """Raised when a REJECTED formula (e.g. Master Score Omega E60/E57) is invoked (RUL-035, RUL-061)."""
    def __init__(self, formula_id: str, reason: str):
        super().__init__(f"Formula '{formula_id}' is REJECTED and strictly non-executable: {reason}")


class NumericalDomainError(PunarvasError):
    """Raised on invalid domain, divide-by-zero, NaN, or negative quantities where forbidden (RUL-063)."""
    def __init__(self, message: str):
        super().__init__(f"Numerical domain validation failed: {message}")


class DimensionalIncompatibilityError(PunarvasError):
    """Raised when incompatible physical dimensions are added or compared without conversion (RUL-064)."""
    def __init__(self, dim_a: str, dim_b: str):
        super().__init__(f"Dimensional incompatibility: cannot combine '{dim_a}' with '{dim_b}' without approved conversion factor.")


class MissingFormulaInputError(PunarvasError):
    """Raised when mandatory inputs are missing or None (RUL-063)."""
    def __init__(self, formula_id: str, missing_param: str):
        super().__init__(f"Formula '{formula_id}' missing mandatory input '{missing_param}'. Missing inputs yield UNKNOWN/FAIL, never 0 or PASS.")


# --- Data Models ---

class FormulaDefinition(BaseModel):
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
    state: RegistryState = RegistryState.APPROVED
    effective_from: str = Field(default_factory=lambda: utc_now().isoformat())
    effective_to: Optional[str] = None
    sha256_hash: str = ""

    def model_post_init(self, __context: Any) -> None:
        if not self.sha256_hash:
            canonical = f"{self.formula_id}:{self.classification.value}:{self.version}:{self.mathematical_expression}"
            self.sha256_hash = hashlib.sha256(canonical.encode("utf-8")).hexdigest()


class ParameterDefinition(BaseModel):
    parameter_id: str
    version: str = "1.0.0"
    formula_ids: List[str] = Field(default_factory=list)
    name: str
    definition: str
    value: Optional[Any] = None  # None if UNSET
    unit: str  # UCUM compatible or "dimensionless"
    spatial_support: str
    temporal_support: str
    source: str
    source_capability_ids: List[str] = Field(default_factory=list)
    acquisition_method: str
    authority: str
    missing_behavior: str = "FAIL_CLOSED_UNKNOWN"
    state: RegistryState = RegistryState.APPROVED
    effective_from: str = Field(default_factory=lambda: utc_now().isoformat())
    effective_to: Optional[str] = None
    sha256_hash: str = ""

    def model_post_init(self, __context: Any) -> None:
        if not self.sha256_hash:
            canonical = f"{self.parameter_id}:{self.version}:{self.value}:{self.unit}:{self.state.value}"
            self.sha256_hash = hashlib.sha256(canonical.encode("utf-8")).hexdigest()


class PolicyFormulaActivation(BaseModel):
    activation_id: str
    policy_id: str
    programme_id: str
    activated_formula_ids: List[str]
    activated_parameter_versions: Dict[str, str] = Field(default_factory=dict)
    authorized_by: str
    effective_date: str = Field(default_factory=lambda: utc_now().isoformat())
    status: str = "ACTIVE"  # ACTIVE, SUPERSEDED, REVOKED


class FormulaExecutionRequest(BaseModel):
    formula_id: str
    inputs: Dict[str, Any]
    policy_activation_id: Optional[str] = None
    reviewer_id: str = "SYSTEM_POLICY_ENGINE"


class FormulaExecutionResult(BaseModel):
    execution_id: str
    formula_id: str
    formula_version: str
    classification: FormulaClassification
    computed_value: Optional[Any] = None
    status: str = "SUCCESS"  # SUCCESS, UNKNOWN, FAILED, BLOCKED
    warnings: List[str] = Field(default_factory=list)
    uncertainty_bounds: Optional[Dict[str, float]] = None
    inputs_snapshot: Dict[str, Any] = Field(default_factory=dict)
    parameter_versions: Dict[str, str] = Field(default_factory=dict)
    software_version: str = "PUNARVAS-1.2.0"
    executed_at: str = Field(default_factory=lambda: utc_now().isoformat())
    execution_sha256: str = ""


class StatutoryComplianceRecord(BaseModel):
    statute_id: str
    title: str
    jurisdiction: JurisdictionLevel
    statutory_authority: str
    effective_date: str
    key_sections: List[Dict[str, str]] = Field(default_factory=list)
    mandatory_controls: List[str] = Field(default_factory=list)
    applicability_state: StatuteApplicabilityState = StatuteApplicabilityState.MANDATORY_IN_FORCE
    last_reviewed_at: str = Field(default_factory=lambda: utc_now().isoformat())
    reviewed_by: str = "STATE_LEGAL_ADVISOR"
    audit_hash: str = ""

    def model_post_init(self, __context: Any) -> None:
        if not self.audit_hash:
            canonical = f"{self.statute_id}:{self.jurisdiction.value}:{self.effective_date}:{len(self.mandatory_controls)}"
            self.audit_hash = hashlib.sha256(canonical.encode("utf-8")).hexdigest()


class ComplianceEvaluationResult(BaseModel):
    evaluation_id: str
    programme_id: str
    statutes_evaluated: int
    mandatory_controls_checked: int
    compliant_controls_count: int
    open_blockers: List[str]
    is_fully_compliant: bool
    evaluated_at: str = Field(default_factory=lambda: utc_now().isoformat())


# --- Engine Service ---

class ComplianceService:
    """
    Core implementation of Phase 12 / ARC-C07.
    Enforces immutable formula & parameter registry, strict execution dispatch,
    numerical domain checks, lineage replay, and legal applicability registry.
    """

    def __init__(self):
        self._formulas: Dict[str, FormulaDefinition] = {}
        self._parameters: Dict[str, ParameterDefinition] = {}
        self._activations: Dict[str, PolicyFormulaActivation] = {}
        self._execution_history: Dict[str, FormulaExecutionResult] = {}
        self._statutes: Dict[str, StatutoryComplianceRecord] = {}
        self._seed_baseline_registry()

    def _seed_baseline_registry(self) -> None:
        """Seed baseline formulas (equations.md), parameters (parameters.md), and statutes (FR-075)."""
        # 1. Formulas
        formulas_seed = [
            FormulaDefinition(
                formula_id="E01",
                name="Unit and Time Conversions",
                classification=FormulaClassification.CORE,
                owner_role="Quantitative Assurance",
                description="Physical conversions for discharge, area, mass, angle and operational duration.",
                mathematical_expression="Q_day = 86400 * Q_s; A_sqm = 10000 * A_ha; m_ton = m_kg / 1000",
                requirement_links=["FR-071", "FR-073"],
                test_links=["AT-17"],
            ),
            FormulaDefinition(
                formula_id="E02",
                name="Mandatory-Gate Eligibility",
                classification=FormulaClassification.CORE,
                owner_role="Policy & Domain Owners",
                description="Gate logic evaluating PASS, FAIL, UNKNOWN, BLOCKED. G=1 iff all gates PASS.",
                mathematical_expression="G_j = 1 if all(g_jk == PASS) else 0",
                applicable_parameters=["PAR-009"],
                requirement_links=["FR-030", "FR-071"],
                test_links=["AT-20"],
            ),
            FormulaDefinition(
                formula_id="E06",
                name="Normalized Linear Composite Indicator",
                classification=FormulaClassification.CORE,
                owner_role="Policy & Quantitative Assurance",
                description="Normalized weighted sum for MCDA criteria with strict weight normalization sum(w_i)==1.",
                mathematical_expression="S_score = sum(w_i * x_norm_i)",
                applicable_parameters=["PAR-007", "PAR-008"],
                requirement_links=["FR-039", "FR-040", "FR-073"],
                test_links=["AT-17"],
            ),
            FormulaDefinition(
                formula_id="E11",
                name="Domestic Water Demand Baseline",
                classification=FormulaClassification.CORE,
                owner_role="Water Domain Lead",
                description="JJM standard domestic water supply demand: 55 LPCD net after distribution loss.",
                mathematical_expression="D_dom = P * 55 LPCD; Cap_people = floor(Q_daily_net / 55)",
                applicable_parameters=["PAR-006"],
                requirement_links=["FR-028", "FR-029"],
                test_links=["AT-05"],
            ),
            FormulaDefinition(
                formula_id="E13",
                name="Developable Land Area Calculation",
                classification=FormulaClassification.CORE,
                owner_role="Planning & Site Lead",
                description="Net developable area excluding geotechnical hazard buffers, watercourses, and forest zones.",
                mathematical_expression="A_dev = A_total - sum(A_excluded)",
                applicable_parameters=["PAR-014"],
                requirement_links=["FR-029", "FR-073"],
                test_links=["AT-17"],
            ),
            FormulaDefinition(
                formula_id="E17",
                name="Multi-Tier Relocation Funding Gap",
                classification=FormulaClassification.CORE,
                owner_role="Finance & Programme Owner",
                description="G = max(0, sum(Cost_i) - sum(Funding_j)), excluding announced budgets until received.",
                mathematical_expression="G = max(0, sum(Cost) - sum(Funding_verified))",
                requirement_links=["FR-064", "FR-067"],
                test_links=["AT-18"],
            ),
            FormulaDefinition(
                formula_id="E26",
                name="MILP Independent Integrality & Feasibility Check",
                classification=FormulaClassification.CORE,
                owner_role="Quantitative Assurance",
                description="Validates that household allocations respect integrality, capacity, and zero-double-assignment.",
                mathematical_expression="sum_s(x_hs) <= 1, sum_h(P_h * x_hs) <= C_s",
                applicable_parameters=["PAR-012"],
                requirement_links=["FR-043", "FR-046"],
                test_links=["AT-16"],
            ),
            FormulaDefinition(
                formula_id="E37",
                name="Infinite Slope Factor of Safety",
                classification=FormulaClassification.SPECIALIST_EXTERNAL,
                owner_role="GIS & Geotechnical Reviewer",
                description="Geotechnical slope stability requiring site-specific cohesion, pore pressure and friction angle.",
                mathematical_expression="Fs = (c_prime + (gamma*z - u)*cos(beta)^2 * tan(phi_prime)) / (gamma*z * sin(beta)*cos(beta))",
                requirement_links=["FR-018", "FR-026"],
                test_links=["AT-01"],
            ),
            FormulaDefinition(
                formula_id="E55",
                name="Unverified Peak Flood Empirical Rule",
                classification=FormulaClassification.REJECTED,
                owner_role="Hydrology / Unverified",
                description="Transcript-suggested unverified empirical formula without local river basin calibration.",
                mathematical_expression="Q_peak = 0.00077 * V^1.017",
                requirement_links=["RUL-061", "RUL-066"],
                test_links=["AT-17"],
            ),
            FormulaDefinition(
                formula_id="E60",
                name="Opaque Master Composite Score (Omega)",
                classification=FormulaClassification.REJECTED,
                owner_role="Quantitative Assurance",
                description="Arbitrary multi-criteria composite index combining incompatible dimensions. Strictly prohibited by RUL-035.",
                mathematical_expression="Omega = H * E * V / C",
                requirement_links=["RUL-035", "RUL-061"],
                test_links=["AT-17"],
            ),
        ]
        for f in formulas_seed:
            self._formulas[f.formula_id] = f

        # 2. Parameters
        params_seed = [
            ParameterDefinition(
                parameter_id="PAR-001",
                name="Platform Monthly Availability",
                definition="High availability uptime target for core decision platform services.",
                value=99.5,
                unit="percent",
                spatial_support="National / Kerala State Infrastructure",
                temporal_support="Monthly rolling window",
                source="PUNARVAS Architectural Blueprint NFR-006",
                acquisition_method="Automated Uptime Monitor",
                authority="MeitY Cloud Certification Guidelines",
                state=RegistryState.APPROVED,
            ),
            ParameterDefinition(
                parameter_id="PAR-006",
                name="JJM Rural Domestic Demand Baseline",
                definition="Minimum daily potable water required per capita in rural rehabilitation settlements.",
                value=55.0,
                unit="LPCD",
                spatial_support="Wayanad District, Kerala",
                temporal_support="Year-round operational minimum",
                source="Jal Jeevan Mission (JJM) Operational Guidelines, Ministry of Jal Shakti",
                source_capability_ids=["S46", "S49"],
                acquisition_method="Government Standard / Field Borewell Verification",
                authority="Jal Jeevan Mission Director / Kerala Water Authority",
                state=RegistryState.APPROVED,
            ),
            ParameterDefinition(
                parameter_id="PAR-007",
                name="Household Priority Vulnerability Weights",
                definition="Weights for elderly, PwD, single-woman, and destitution priority calculation.",
                value=None,  # UNSET by default as required by parameters.md
                unit="dimensionless",
                spatial_support="Wayanad Revenue Division",
                temporal_support="Programme Duration",
                source="District Level Social Impact Assessment S48",
                acquisition_method="Gram Sabha / Social Justice Directorate Approval",
                authority="District Collector / DDMA Chairperson",
                state=RegistryState.DRAFT,
            ),
            ParameterDefinition(
                parameter_id="PAR-009",
                name="Mandatory Gate Expiry Window",
                definition="Validity duration of field geotechnical and water test reports before requiring re-survey.",
                value=180,
                unit="days",
                spatial_support="Kerala State Disaster Zones",
                temporal_support="Continuous",
                source="KSDMA Standard Operating Procedures",
                acquisition_method="Order No. KSDMA/2024/GATES",
                authority="Member Secretary, KSDMA",
                state=RegistryState.APPROVED,
            ),
            ParameterDefinition(
                parameter_id="PAR-014",
                name="Developable Area Layout Exclusion Buffer",
                definition="Minimum setback distance from high-hazard scarps and natural drainage paths.",
                value=30.0,
                unit="m",
                spatial_support="Wayanad Hill Tracts",
                temporal_support="Year-round",
                source="Kerala Municipality Building Rules (KMBR) & GSI Landslide Buffer",
                acquisition_method="Statutory Order",
                authority="Chief Town Planner / GSI Western Ghats Directorate",
                state=RegistryState.APPROVED,
            ),
        ]
        for p in params_seed:
            self._parameters[p.parameter_id] = p

        # 3. Statutes (FR-075)
        statutes_seed = [
            StatutoryComplianceRecord(
                statute_id="DMA-2005",
                title="Disaster Management Act, 2005 (Amended 2025)",
                jurisdiction=JurisdictionLevel.UNION_OF_INDIA,
                statutory_authority="Ministry of Home Affairs / NDMA",
                effective_date="2025-04-09",
                key_sections=[
                    {"section": "§30", "summary": "Powers and functions of District Authority (DDMA) in disaster mitigation and rehabilitation."},
                    {"section": "§31(4)", "summary": "District Disaster Management Plan shall be reviewed and updated at least once every two years."},
                    {"section": "§65", "summary": "Requisitioning of resources, land and premises for rescue or rehabilitation."},
                ],
                mandatory_controls=[
                    "CONTROL_DDMA_APPROVAL_MANDATORY",
                    "CONTROL_BIENNIAL_PLAN_UPDATE_CADENCE",
                    "CONTROL_PROHIBIT_AUTONOMOUS_GAZETTE",
                ],
                applicability_state=StatuteApplicabilityState.MANDATORY_IN_FORCE,
            ),
            StatutoryComplianceRecord(
                statute_id="RFCTLARR-2013",
                title="Right to Fair Compensation and Transparency in Land Acquisition, Rehabilitation and Resettlement Act, 2013",
                jurisdiction=JurisdictionLevel.UNION_OF_INDIA,
                statutory_authority="Ministry of Rural Development (DoLR) / Land Revenue Dept",
                effective_date="2014-01-01",
                key_sections=[
                    {"section": "§16-§19", "summary": "Rehabilitation and Resettlement scheme preparation, publication and declaration."},
                    {"section": "§31-§42", "summary": "Rehabilitation entitlements, infrastructural amenities and settlement allotment."},
                ],
                mandatory_controls=[
                    "CONTROL_REHABILITATION_SCHEME_PUBLICATION",
                    "CONTROL_INFRASTRUCTURE_AMENITIES_VERIFIED",
                    "CONTROL_PROHIBIT_DISPLACEMENT_WITHOUT_REMEDY",
                ],
                applicability_state=StatuteApplicabilityState.MANDATORY_IN_FORCE,
            ),
            StatutoryComplianceRecord(
                statute_id="FRA-2006",
                title="Scheduled Tribes and Other Traditional Forest Dwellers (Recognition of Forest Rights) Act, 2006",
                jurisdiction=JurisdictionLevel.UNION_OF_INDIA,
                statutory_authority="Ministry of Tribal Affairs / State Tribal Development Dept",
                effective_date="2008-01-01",
                key_sections=[
                    {"section": "§3(1)", "summary": "Community Forest Rights and individual forest tenure recognition."},
                    {"section": "§4(5)", "summary": "No member of a forest dwelling ST or OTFD shall be evicted until verification completion."},
                    {"section": "§6", "summary": "Grama Sabha authority to initiate process for determining forest rights."},
                ],
                mandatory_controls=[
                    "CONTROL_GRAMA_SABHA_CONSENT_MANDATORY",
                    "CONTROL_PROHIBIT_EVICTION_PENDING_FRA",
                    "CONTROL_COMMUNITY_RIGHTS_PRESERVED",
                ],
                applicability_state=StatuteApplicabilityState.MANDATORY_IN_FORCE,
            ),
            StatutoryComplianceRecord(
                statute_id="DPDP-2023",
                title="Digital Personal Data Protection Act, 2023 & DPDP Rules, 2025",
                jurisdiction=JurisdictionLevel.UNION_OF_INDIA,
                statutory_authority="Data Protection Board of India / MeitY",
                effective_date="2025-11-14",
                key_sections=[
                    {"section": "§6", "summary": "Requirement of notice and purpose-specific consent."},
                    {"section": "§8", "summary": "Obligations of Data Fiduciary (security safeguards, data minimization, accuracy)."},
                    {"section": "§9", "summary": "Processing of personal data of children and persons with disability."},
                ],
                mandatory_controls=[
                    "CONTROL_INDIA_RESIDENT_HOSTING_ONLY",
                    "CONTROL_PURPOSE_SPECIFIC_CONSENT_SEPARATION",
                    "CONTROL_RESTRICTED_FIELD_TOKENIZATION",
                    "CONTROL_K_ANONYMITY_PUBLIC_PROJECTIONS",
                ],
                applicability_state=StatuteApplicabilityState.MANDATORY_IN_FORCE,
            ),
            StatutoryComplianceRecord(
                statute_id="CERT-IN-2022",
                title="CERT-In Cybersecurity Directions under IT Act §70B",
                jurisdiction=JurisdictionLevel.UNION_OF_INDIA,
                statutory_authority="Indian Computer Emergency Response Team (CERT-In)",
                effective_date="2022-04-28",
                key_sections=[
                    {"section": "Direction 1", "summary": "Mandatory sync of system clocks with NTP of NIC or NPL."},
                    {"section": "Direction 4", "summary": "Mandatory maintenance of logs of all ICT systems for a rolling 180 days."},
                ],
                mandatory_controls=[
                    "CONTROL_NTP_CLOCK_SYNC",
                    "CONTROL_180_DAY_AUDIT_LOG_RETENTION",
                    "CONTROL_TAMPER_EVIDENT_HASH_CHAINING",
                ],
                applicability_state=StatuteApplicabilityState.MANDATORY_IN_FORCE,
            ),
        ]
        for s in statutes_seed:
            self._statutes[s.statute_id] = s

    # --- Formula Registry Methods ---

    def register_formula(self, formula: FormulaDefinition, actor_id: str) -> FormulaDefinition:
        """Register or update an immutable formula version (FR-071)."""
        self._formulas[formula.formula_id] = formula
        global_audit_ledger.append_event(
            action="FORMULA_REGISTERED",
            actor_id=actor_id,
            resource_type="FORMULA",
            resource_id=formula.formula_id,
            payload={
                "name": formula.name,
                "classification": formula.classification.value,
                "version": formula.version,
                "sha256": formula.sha256_hash,
            },
        )
        return formula

    def get_formula(self, formula_id: str) -> Optional[FormulaDefinition]:
        return self._formulas.get(formula_id)

    def list_formulas(self, classification: Optional[FormulaClassification] = None) -> List[FormulaDefinition]:
        if classification:
            return [f for f in self._formulas.values() if f.classification == classification]
        return list(self._formulas.values())

    # --- Parameter Registry Methods ---

    def register_parameter(self, param: ParameterDefinition, actor_id: str) -> ParameterDefinition:
        """Register or update a parameter definition (FR-071)."""
        self._parameters[param.parameter_id] = param
        global_audit_ledger.append_event(
            action="PARAMETER_REGISTERED",
            actor_id=actor_id,
            resource_type="PARAMETER",
            resource_id=param.parameter_id,
            payload={
                "name": param.name,
                "value": param.value,
                "unit": param.unit,
                "state": param.state.value,
            },
        )
        return param

    def get_parameter(self, parameter_id: str) -> Optional[ParameterDefinition]:
        return self._parameters.get(parameter_id)

    def list_parameters(self) -> List[ParameterDefinition]:
        return list(self._parameters.values())

    # --- Policy Formula Activation ---

    def create_policy_activation(
        self,
        policy_id: str,
        programme_id: str,
        activated_formula_ids: List[str],
        authorized_by: str,
    ) -> PolicyFormulaActivation:
        """Explicitly activates formulas for a given policy and programme (FR-072)."""
        # Validate that no REJECTED formula is activated
        for fid in activated_formula_ids:
            f = self._formulas.get(fid)
            if f and f.classification == FormulaClassification.REJECTED:
                raise RejectedFormulaExecutionError(fid, f"Cannot activate rejected formula '{f.name}'.")

        activation_id = f"ACT-{policy_id}-{utc_now().strftime('%Y%m%d%H%M%S')}"
        activation = PolicyFormulaActivation(
            activation_id=activation_id,
            policy_id=policy_id,
            programme_id=programme_id,
            activated_formula_ids=activated_formula_ids,
            authorized_by=authorized_by,
        )
        self._activations[activation_id] = activation
        global_audit_ledger.append_event(
            action="POLICY_FORMULA_ACTIVATION_CREATED",
            actor_id=authorized_by,
            resource_type="POLICY_ACTIVATION",
            resource_id=activation_id,
            payload={
                "policy_id": policy_id,
                "activated_formula_ids": activated_formula_ids,
            },
        )
        return activation

    def get_policy_activation(self, activation_id: str) -> Optional[PolicyFormulaActivation]:
        return self._activations.get(activation_id)

    # --- Formula Execution Dispatch & Numerical Guard (FR-072, FR-073, FR-074) ---

    def execute_formula(self, req: FormulaExecutionRequest) -> FormulaExecutionResult:
        """
        Executes a formula under strict validation, allow-listing, and lineage tracking.
        """
        formula = self._formulas.get(req.formula_id)
        if not formula:
            raise PunarvasError(f"Formula '{req.formula_id}' not found in registry.")

        # 1. Deny-List Checks
        if formula.classification == FormulaClassification.REJECTED:
            raise RejectedFormulaExecutionError(
                formula.formula_id,
                "RUL-035 / RUL-061 explicitly prohibits executing rejected master scores or unverified formulas."
            )
        if formula.classification == FormulaClassification.SPECIALIST_EXTERNAL:
            raise UnvalidatedSpecialistFormulaError(formula.formula_id)

        # 2. Allow-List Policy Activation Check (FR-072)
        if req.policy_activation_id:
            activation = self._activations.get(req.policy_activation_id)
            if not activation or req.formula_id not in activation.activated_formula_ids:
                raise FormulaNotActivatedError(req.formula_id, req.policy_activation_id)

        # 3. Numerical Validation Guard & Dispatch (FR-073, RUL-063, RUL-064)
        computed_val, status, warnings, uncertainty = self._dispatch_computation(req.formula_id, req.inputs)

        # 4. Create Lineage Record (FR-074)
        exec_id = f"EXEC-{req.formula_id}-{hashlib.sha256(str(utc_now().timestamp()).encode()).hexdigest()[:8].upper()}"
        
        # Build canonical execution hash for bit-for-bit replay
        canonical_str = f"{req.formula_id}:{formula.version}:{json.dumps(req.inputs, sort_keys=True)}:{computed_val}"
        exec_hash = hashlib.sha256(canonical_str.encode("utf-8")).hexdigest()

        param_versions = {
            pid: self._parameters[pid].version
            for pid in formula.applicable_parameters
            if pid in self._parameters
        }

        result = FormulaExecutionResult(
            execution_id=exec_id,
            formula_id=req.formula_id,
            formula_version=formula.version,
            classification=formula.classification,
            computed_value=computed_val,
            status=status,
            warnings=warnings,
            uncertainty_bounds=uncertainty,
            inputs_snapshot=req.inputs,
            parameter_versions=param_versions,
            execution_sha256=exec_hash,
        )

        self._execution_history[exec_id] = result

        global_audit_ledger.append_event(
            action="FORMULA_EXECUTED",
            actor_id=req.reviewer_id,
            resource_type="FORMULA_EXECUTION",
            resource_id=exec_id,
            payload={
                "formula_id": req.formula_id,
                "status": status,
                "computed_value": str(computed_val),
                "execution_sha256": exec_hash,
            },
        )

        return result

    def _dispatch_computation(
        self,
        formula_id: str,
        inputs: Dict[str, Any]
    ) -> Tuple[Any, str, List[str], Optional[Dict[str, float]]]:
        """Dispatches calculations with numerical safeguards."""
        warnings: List[str] = []

        if formula_id == "E01":
            # Unit and Time Conversions
            # Expects: mode: ("L_S_TO_L_DAY", "M3_DAY_TO_L_DAY", "HA_TO_SQM", "KG_TO_TONNES", "DEG_TO_RAD")
            # val: float, optional operating_hours: float
            mode = inputs.get("mode")
            val = inputs.get("val")
            if val is None:
                raise MissingFormulaInputError("E01", "val")
            if not isinstance(val, (int, float)) or math.isnan(val) or math.isinf(val):
                raise NumericalDomainError(f"Input value '{val}' is invalid or NaN/Inf.")
            if val < 0:
                raise NumericalDomainError("Physical quantity cannot be negative.")

            if mode == "L_S_TO_L_DAY":
                hours = inputs.get("operating_hours", 24.0)
                if hours <= 0 or hours > 24:
                    raise NumericalDomainError("Operating hours must be in (0, 24].")
                sec_day = hours * 3600.0
                res = val * sec_day
                return res, "SUCCESS", warnings, None
            elif mode == "M3_DAY_TO_L_DAY":
                res = val * 1000.0
                return res, "SUCCESS", warnings, None
            elif mode == "HA_TO_SQM":
                res = val * 10000.0
                return res, "SUCCESS", warnings, None
            elif mode == "KG_TO_TONNES":
                res = val / 1000.0
                return res, "SUCCESS", warnings, None
            elif mode == "DEG_TO_RAD":
                res = math.radians(val)
                return res, "SUCCESS", warnings, None
            else:
                raise PunarvasError(f"Unsupported conversion mode '{mode}'.")

        elif formula_id == "E02":
            # Mandatory Gate Eligibility
            # Expects: gates: List[str] with states: "PASS", "FAIL", "UNKNOWN", "BLOCKED"
            gates = inputs.get("gates")
            if gates is None:
                raise MissingFormulaInputError("E02", "gates")
            if not isinstance(gates, list) or len(gates) == 0:
                return 0, "UNKNOWN", ["Empty gate list produces UNKNOWN."], None
            
            if any(g == "FAIL" for g in gates):
                return 0, "FAILED", ["One or more mandatory gates failed."], None
            if any(g in ("UNKNOWN", "BLOCKED") for g in gates):
                return 0, "BLOCKED", ["One or more mandatory gates are UNKNOWN or BLOCKED."], None
            if all(g == "PASS" for g in gates):
                return 1, "SUCCESS", [], None
            return 0, "UNKNOWN", ["Unrecognized gate state."], None

        elif formula_id == "E06":
            # Linear Composite Score (Normalized weighted sum: sum(w_i * x_norm_i))
            # Expects: weights: Dict[str, float], values: Dict[str, float]
            weights = inputs.get("weights")
            values = inputs.get("values")
            if weights is None or values is None:
                raise MissingFormulaInputError("E06", "weights or values")
            
            # Check weight sum == 1.0 within tolerance
            weight_sum = sum(weights.values())
            if not math.isclose(weight_sum, 1.0, rel_tol=1e-3, abs_tol=1e-3):
                raise NumericalDomainError(f"Weights sum must equal 1.0; got {weight_sum:.4f}")

            score = 0.0
            for k, w in weights.items():
                if k not in values or values[k] is None:
                    # Missing input yields UNKNOWN status (RUL-063), never coerce to 0
                    return None, "UNKNOWN", [f"Missing value for criterion '{k}'. Missing values cannot be coerced to zero."], None
                v = values[k]
                if math.isnan(v) or math.isinf(v):
                    raise NumericalDomainError(f"Criterion '{k}' has NaN or Inf value.")
                if v < 0.0 or v > 1.0:
                    raise NumericalDomainError(f"Criterion '{k}' normalized value {v} is outside [0, 1].")
                score += w * v

            return round(score, 4), "SUCCESS", warnings, {"low": round(max(0.0, score - 0.05), 4), "high": round(min(1.0, score + 0.05), 4)}

        elif formula_id == "E11":
            # Domestic Water Demand & Carrying Capacity
            # Expects: tested_yield_lpcd: float, delivery_loss_pct: float (optional, default 20.0), population: Optional[int]
            yield_lpcd = inputs.get("tested_yield_lpcd")
            if yield_lpcd is None:
                raise MissingFormulaInputError("E11", "tested_yield_lpcd")
            if yield_lpcd <= 0:
                raise NumericalDomainError("Water yield must be strictly positive.")
            
            loss_pct = inputs.get("delivery_loss_pct", 20.0)
            if loss_pct < 0 or loss_pct >= 100:
                raise NumericalDomainError("Delivery loss percentage must be in [0, 100).")
            
            net_yield = yield_lpcd * (1.0 - (loss_pct / 100.0))
            jjm_baseline = 55.0
            
            pop = inputs.get("population")
            if pop is not None:
                if pop < 0:
                    raise NumericalDomainError("Population cannot be negative.")
                required_lpcd = pop * jjm_baseline
                is_sufficient = net_yield >= required_lpcd
                res_data = {
                    "net_yield_lpcd": round(net_yield, 2),
                    "required_lpcd": round(required_lpcd, 2),
                    "is_sufficient": is_sufficient,
                    "surplus_lpcd": round(net_yield - required_lpcd, 2),
                    "max_supported_people": math.floor(net_yield / jjm_baseline),
                }
                return res_data, "SUCCESS" if is_sufficient else "FAILED", warnings, None
            else:
                max_people = math.floor(net_yield / jjm_baseline)
                return {
                    "net_yield_lpcd": round(net_yield, 2),
                    "max_supported_people": max_people,
                }, "SUCCESS", warnings, None

        elif formula_id == "E13":
            # Developable Land Area Calculation
            # Expects: total_area_sqm: float, excluded_areas_sqm: List[float]
            total_area = inputs.get("total_area_sqm")
            if total_area is None:
                raise MissingFormulaInputError("E13", "total_area_sqm")
            if total_area <= 0:
                raise NumericalDomainError("Total site area must be positive.")
            
            excluded_list = inputs.get("excluded_areas_sqm", [])
            sum_excluded = sum(excluded_list)
            if sum_excluded > total_area:
                raise NumericalDomainError(f"Excluded areas ({sum_excluded} m²) exceed total site area ({total_area} m²).")
            
            developable = total_area - sum_excluded
            return {
                "total_area_sqm": total_area,
                "excluded_area_sqm": sum_excluded,
                "developable_area_sqm": developable,
                "developable_percentage": round((developable / total_area) * 100.0, 2),
            }, "SUCCESS", warnings, None

        elif formula_id == "E17":
            # Relocation Funding Gap (E17)
            # Expects: required_costs: List[float], verified_funds: List[float], announced_budgets: List[float]
            req_costs = inputs.get("required_costs", [])
            ver_funds = inputs.get("verified_funds", [])
            ann_budgets = inputs.get("announced_budgets", [])

            total_cost = sum(req_costs)
            total_verified = sum(ver_funds)
            total_announced = sum(ann_budgets)

            # RUL-069: Announced budgets NEVER reduce the funding gap
            gap = max(0.0, total_cost - total_verified)
            if total_announced > 0:
                warnings.append(f"Announced budget of ₹{total_announced:,.2f} is excluded from gap reduction until received/spent.")

            return {
                "total_required_cost": total_cost,
                "total_verified_funding": total_verified,
                "funding_gap": gap,
                "unspent_announced_budget": total_announced,
            }, "SUCCESS", warnings, None

        elif formula_id == "E26":
            # MILP Integrality and Capacity Feasibility Check
            # Expects: site_capacity: int, assignments: List[Dict[str, Any]] (each with household_size: int)
            cap = inputs.get("site_capacity")
            assignments = inputs.get("assignments")
            if cap is None or assignments is None:
                raise MissingFormulaInputError("E26", "site_capacity or assignments")
            
            total_people = sum(a.get("household_size", 0) for a in assignments)
            dwellings_assigned = len(assignments)
            
            if dwellings_assigned > cap:
                return {
                    "is_feasible": False,
                    "dwellings_assigned": dwellings_assigned,
                    "site_capacity": cap,
                    "reason": f"Dwellings assigned ({dwellings_assigned}) exceeds site dwelling capacity ({cap}).",
                }, "FAILED", ["Capacity constraint violated."], None

            return {
                "is_feasible": True,
                "dwellings_assigned": dwellings_assigned,
                "site_capacity": cap,
                "total_people": total_people,
            }, "SUCCESS", warnings, None

        else:
            raise PunarvasError(f"No executable binding for formula '{formula_id}'.")

    # --- Lineage Replay (FR-074) ---

    def reproduce_formula_execution(self, execution_id: str) -> Dict[str, Any]:
        """
        Bit-for-bit historical re-execution verification.
        Re-computes the formula with historical inputs and asserts exact SHA-256 match.
        """
        record = self._execution_history.get(execution_id)
        if not record:
            raise PunarvasError(f"Historical execution record '{execution_id}' not found.")

        # Re-dispatch
        computed_val, status, _, _ = self._dispatch_computation(record.formula_id, record.inputs_snapshot)

        canonical_str = f"{record.formula_id}:{record.formula_version}:{json.dumps(record.inputs_snapshot, sort_keys=True)}:{computed_val}"
        reproduced_hash = hashlib.sha256(canonical_str.encode("utf-8")).hexdigest()

        is_exact_match = (reproduced_hash == record.execution_sha256)
        return {
            "execution_id": execution_id,
            "formula_id": record.formula_id,
            "original_hash": record.execution_sha256,
            "reproduced_hash": reproduced_hash,
            "is_bit_for_bit_identical": is_exact_match,
            "reproduced_value": computed_val,
        }

    # --- Statutory Compliance Register (FR-075) ---

    def register_statute(self, statute: StatutoryComplianceRecord, actor_id: str) -> StatutoryComplianceRecord:
        """Register or update a statutory compliance requirement (FR-075)."""
        self._statutes[statute.statute_id] = statute
        global_audit_ledger.append_event(
            action="STATUTE_REGISTERED",
            actor_id=actor_id,
            resource_type="STATUTE",
            resource_id=statute.statute_id,
            payload={
                "title": statute.title,
                "jurisdiction": statute.jurisdiction.value,
                "mandatory_controls": statute.mandatory_controls,
            },
        )
        return statute

    def get_statute(self, statute_id: str) -> Optional[StatutoryComplianceRecord]:
        return self._statutes.get(statute_id)

    def list_statutes(self) -> List[StatutoryComplianceRecord]:
        return list(self._statutes.values())

    def verify_compliance_posture(
        self,
        programme_id: str,
        active_control_ids: List[str]
    ) -> ComplianceEvaluationResult:
        """
        Audits active controls against all mandatory statutory controls across
        the Disaster Management Act, RFCTLARR, FRA, DPDP, and CERT-In.
        """
        eval_id = f"COMPL-EVAL-{utc_now().strftime('%Y%m%d%H%M%S')}"
        all_statutes = self.list_statutes()

        total_mandatory: List[str] = []
        for s in all_statutes:
            if s.applicability_state == StatuteApplicabilityState.MANDATORY_IN_FORCE:
                total_mandatory.extend(s.mandatory_controls)

        active_set = set(active_control_ids)
        open_blockers = [ctrl for ctrl in total_mandatory if ctrl not in active_set]
        compliant_count = len(total_mandatory) - len(open_blockers)
        is_compliant = (len(open_blockers) == 0)

        res = ComplianceEvaluationResult(
            evaluation_id=eval_id,
            programme_id=programme_id,
            statutes_evaluated=len(all_statutes),
            mandatory_controls_checked=len(total_mandatory),
            compliant_controls_count=compliant_count,
            open_blockers=open_blockers,
            is_fully_compliant=is_compliant,
        )

        global_audit_ledger.append_event(
            action="COMPLIANCE_POSTURE_EVALUATED",
            actor_id="STATUTORY_AUDITOR",
            resource_type="COMPLIANCE_EVALUATION",
            resource_id=eval_id,
            payload={
                "programme_id": programme_id,
                "is_fully_compliant": is_compliant,
                "open_blockers": open_blockers,
            },
        )

        return res


# Global Singleton
compliance_service = ComplianceService()
