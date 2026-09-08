"""
PUNARVAS-AI Multi-State Adaptation & Legal Isolation Framework (PH-5 / C5-01).
Normative Reference: phases.md §8 & §12.8, rules.md (RUL-002, RUL-027, RUL-068).

Guarantees:
1. State-specific law/workflow isolation: Kerala rules (VLRS ₹10L cap, LSGD DM Plan annex) are NEVER treated as national defaults.
2. Shared national statutory core: All states inherit DM Act 2005 (amended 2025 §31(4)), RFCTLARR 2013, FRA 2006, and DPDP Rules 2025.
3. State-specific data source adapters: KSDMA for Kerala, USDMA/CBRI for Uttarakhand.
4. Multilingual template & terminology isolation (ML in Kerala, HI in Uttarakhand).
"""

from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import UserContext, utc_now
from punarvas.core.enums import AuthorityState, RoleType
from punarvas.core.errors import AuthorityBypassError
from punarvas.core.audit import global_audit_ledger


class ResettlementSchemeNorm(BaseModel):
    scheme_id: str
    name: str
    legal_order_reference: str
    max_assistance_inr: Optional[float] = None
    land_component_inr: Optional[float] = None
    construction_component_inr: Optional[float] = None
    min_safe_land_cents: Optional[float] = None
    agricultural_retention_permitted: bool = True
    residential_rebuilding_prohibited: bool = True


class StateLegalWorkflowMap(BaseModel):
    state_id: str
    land_revenue_act: str
    panchayat_act: str
    forest_rights_consultation_body: str  # e.g., Grama Sabha (Kerala) vs Van Panchayat / Gram Sabha (Uttarakhand)
    statutory_appeals_authority: str
    local_disaster_plan_format: str  # e.g., Kerala LSGD DM Plan vs District DM Plan Annexure


class StateSourceAdapterConfig(BaseModel):
    state_id: str
    sdma_portal_name: str
    cadastral_system_name: str  # e.g. Kerala "Bhulekh / E-Rekha" vs Uttarakhand "Bhulekh Uttarakhand"
    default_hazard_source_id: str
    groundwater_authority: str  # e.g., Kerala Ground Water Dept vs Uttarakhand Peyjal Nigam / CGWB


class StatePackage(BaseModel):
    state_id: str
    state_name: str
    state_code: str
    primary_language: str
    supported_languages: List[str]
    sdma_name: str
    statutory_mandate: str
    shared_national_laws: List[str] = Field(default_factory=lambda: [
        "Disaster Management Act 2005 (as amended 2025 §31(4))",
        "RFCTLARR Act 2013",
        "Scheduled Tribes and Other Traditional Forest Dwellers (FRA) Act 2006",
        "Digital Personal Data Protection Act 2023 / Rules 2025",
    ])
    schemes: List[ResettlementSchemeNorm] = Field(default_factory=list)
    workflow_map: StateLegalWorkflowMap
    source_adapter: StateSourceAdapterConfig
    version: str = "1.0"
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())


class StatePackageLoader:
    """
    Loads, manages, and isolates state-specific adaptation packages (C5-01).
    Ensures state legal assumptions do not cross tenant borders.
    """

    def __init__(self):
        self._packages: Dict[str, StatePackage] = {}
        self._seed_reference_state_packages()

    def _seed_reference_state_packages(self):
        # 1. Kerala State Package (Reference Pilot)
        kerala_pkg = StatePackage(
            state_id="Kerala",
            state_name="Kerala",
            state_code="KL",
            primary_language="ml",
            supported_languages=["ml", "en"],
            sdma_name="Kerala State Disaster Management Authority (KSDMA)",
            statutory_mandate="Kerala State Disaster Management Rules & DMA 2005",
            schemes=[
                ResettlementSchemeNorm(
                    scheme_id="SCHEME-KL-VLRS-2018",
                    name="Kerala Vulnerability Linked Relocation Scheme (VLRS)",
                    legal_order_reference="G.O. (Ms) No. 6/2018/DMD & G.O. (Ms) No. 2/2020/DMD",
                    max_assistance_inr=1000000.0,  # ₹10 Lakh
                    land_component_inr=600000.0,    # ₹6 Lakh
                    construction_component_inr=400000.0,  # ₹4 Lakh
                    min_safe_land_cents=3.0,
                    agricultural_retention_permitted=True,
                    residential_rebuilding_prohibited=True,
                ),
                ResettlementSchemeNorm(
                    scheme_id="SCHEME-KL-WAYANAD-TOWNSHIP-2024",
                    name="Wayanad Rehabilitation Township Package (Elstone Estate Model)",
                    legal_order_reference="G.O. (Ms) No. 44/2024/DMD",
                    max_assistance_inr=1500000.0,
                    land_component_inr=0.0,  # Direct 7-cent plot allocation
                    construction_component_inr=1500000.0,
                    min_safe_land_cents=7.0,
                    agricultural_retention_permitted=False,
                    residential_rebuilding_prohibited=True,
                )
            ],
            workflow_map=StateLegalWorkflowMap(
                state_id="Kerala",
                land_revenue_act="Kerala Land Reforms Act 1963 & Kerala Land Relinquishment Act 1958",
                panchayat_act="Kerala Panchayat Raj Act 1994",
                forest_rights_consultation_body="Grama Sabha (Ooru Vikasana Samithi for ST settlements)",
                statutory_appeals_authority="District Collector / Land Revenue Commissioner",
                local_disaster_plan_format="Kerala LSGD Disaster Management Plan Template",
            ),
            source_adapter=StateSourceAdapterConfig(
                state_id="Kerala",
                sdma_portal_name="KSDMA Portal",
                cadastral_system_name="E-Rekha / Bhoo-Aadhaar Kerala",
                default_hazard_source_id="S01-GSI-NLSM-KL",
                groundwater_authority="Kerala State Ground Water Department (KSGWD) / KWA",
            ),
        )
        self._packages["Kerala"] = kerala_pkg

        # 2. Uttarakhand State Package (Himalayan Second State Reference)
        uk_pkg = StatePackage(
            state_id="Uttarakhand",
            state_name="Uttarakhand",
            state_code="UK",
            primary_language="hi",
            supported_languages=["hi", "en"],
            sdma_name="Uttarakhand State Disaster Management Authority (USDMA)",
            statutory_mandate="Uttarakhand State Disaster Management Rules 2007 & DMA 2005",
            schemes=[
                ResettlementSchemeNorm(
                    scheme_id="SCHEME-UK-PUNARVAS-2021",
                    name="Uttarakhand Mukhya Mantri Punarvas Yojana (Hill Resettlement Policy)",
                    legal_order_reference="Uttarakhand Disaster Management Department Order 412/USDMA/2021",
                    max_assistance_inr=1200000.0,  # ₹12 Lakh
                    land_component_inr=500000.0,
                    construction_component_inr=700000.0,
                    min_safe_land_cents=4.0,
                    agricultural_retention_permitted=True,
                    residential_rebuilding_prohibited=True,
                ),
                ResettlementSchemeNorm(
                    scheme_id="SCHEME-UK-JOSHIMATH-SPECIAL-2023",
                    name="Joshimath Subsidence Special Rehabilitation Package",
                    legal_order_reference="Cabinet Decision / GO No. 18/USDMA/Joshimath/2023",
                    max_assistance_inr=2000000.0,
                    land_component_inr=800000.0,
                    construction_component_inr=1200000.0,
                    min_safe_land_cents=5.0,
                    agricultural_retention_permitted=True,
                    residential_rebuilding_prohibited=True,
                )
            ],
            workflow_map=StateLegalWorkflowMap(
                state_id="Uttarakhand",
                land_revenue_act="Uttar Pradesh Zamindari Abolition and Land Reforms Act 1950 (as adapted in Uttarakhand)",
                panchayat_act="Uttarakhand Panchayati Raj Act 2016",
                forest_rights_consultation_body="Van Panchayat & Gram Sabha (Uttarakhand Panchayati Forest Rules 2005)",
                statutory_appeals_authority="District Magistrate / Commissioner Garhwal/Kumaon",
                local_disaster_plan_format="Uttarakhand DDMA District Disaster Management Plan",
            ),
            source_adapter=StateSourceAdapterConfig(
                state_id="Uttarakhand",
                sdma_portal_name="USDMA Web Portal",
                cadastral_system_name="Bhulekh Uttarakhand (Devbhoomi)",
                default_hazard_source_id="S03-USDMA-CBRI-UK",
                groundwater_authority="Uttarakhand Peyjal Nigam / Jal Sansthan",
            ),
        )
        self._packages["Uttarakhand"] = uk_pkg

    def register_state_package(self, package: StatePackage, user: UserContext, reason: str) -> StatePackage:
        """Register or update a state adaptation package (C5-01)."""
        if not user.has_role(RoleType.GOVERNMENT_APPROVER):
            raise AuthorityBypassError("Only National / State Authority can register state adaptation packages.")

        self._packages[package.state_id] = package

        global_audit_ledger.log(
            actor_id=user.user_id,
            authority_scope=f"STATE_PACKAGE/{package.state_id}",
            action="REGISTER_STATE_PACKAGE",
            entity_type="StatePackage",
            entity_id=package.state_id,
            version_id=package.version,
            reason=reason,
            correlation_id=user.correlation_id,
        )
        return package

    def get_state_package(self, state_id: str) -> Optional[StatePackage]:
        return self._packages.get(state_id)

    def list_state_packages(self) -> List[StatePackage]:
        return list(self._packages.values())

    def get_state_schemes(self, state_id: str) -> List[ResettlementSchemeNorm]:
        """Strict isolation: never return another state's schemes."""
        pkg = self.get_state_package(state_id)
        if not pkg:
            raise KeyError(f"State package '{state_id}' is not installed.")
        return pkg.schemes

    def get_state_workflow_map(self, state_id: str) -> StateLegalWorkflowMap:
        """Strict isolation: retrieve state-specific legal procedures."""
        pkg = self.get_state_package(state_id)
        if not pkg:
            raise KeyError(f"State package '{state_id}' is not installed.")
        return pkg.workflow_map


state_package_loader = StatePackageLoader()
