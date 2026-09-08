"""
PUNARVAS-AI Multi-State Adaptation & Tenant Isolation Module (PH-5 / C5-01, C5-02).
Normative Reference: phases.md §8 & §12.8, rules.md (RUL-017, RUL-018, RUL-054), DEC-037.
"""

from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import UserContext, utc_now
from punarvas.core.errors import UnauthorizedGeographyAccessError
from punarvas.core.audit import global_audit_ledger


class StateTenantPackage(BaseModel):
    tenant_id: str  # e.g., "TENANT-KL", "TENANT-UK"
    state_code: str  # "KL", "UK"
    state_name: str  # "Kerala", "Uttarakhand"
    statutory_authority: str  # e.g., "Uttarakhand State Disaster Management Authority (USDMA)"
    official_languages: List[str]  # e.g., ["hi", "en"] or ["ml", "en"]
    disaster_relief_manual: str
    land_record_system: str  # e.g., "Devbhoomi / Bhulekh Uttarakhand (Khasra/Khatauni)"
    default_crs: str  # e.g., "EPSG:32644" (UTM 44N) vs "EPSG:32643" (UTM 43N)
    primary_hazard_mechanics: str
    active_schemes: List[str] = Field(default_factory=list)
    onboarded_at: str = Field(default_factory=lambda: utc_now().isoformat())


class MultiStateAdapterService:
    """
    C5-01: Multi-state adaptation and tenant isolation (DEC-037).
    Guarantees that state-specific legal frameworks, land tenure terminology,
    coordinate reference systems, and languages are decoupled without cross-state data leakage.
    """

    GLOSSARIES: Dict[str, Dict[str, str]] = {
        "UK_hi": {
            "title": "पुनर्वास-एआई स्थायी पुनर्वास निर्णय सहायता प्रणाली",
            "advisory_notice": "केवल परामर्श निर्णय सहायता। कोई आधिकारिक सरकारी आदेश या अधिसूचना नहीं।",
            "authority": "उत्तराखंड राज्य आपदा प्रबंधन प्राधिकरण (USDMA)",
            "pathway_township": "मॉडल सुरक्षित टाउनशिप पुनर्वास",
            "pathway_self_relocation": "एसडीआरएफ स्व-पुनर्वास वित्तीय सहायता",
            "land_tenure_term": "खसरा / खतौनी संख्या",
        },
        "KL_ml": {
            "title": "പുനർവാസ്-എഐ ശാശ്വത പുനരധിവാസ ഉപദേശക രേഖ",
            "advisory_notice": "ഉപദേശക സ്വഭാവമുള്ളത് മാത്രം. ഔദ്യോഗിക ഗസറ്റ് വിജ്ഞാപനമല്ല.",
            "authority": "കേരള സംസ്ഥാന ദുരന്ത നിവാരണ അതോറിറ്റി (KSDMA)",
            "pathway_township": "മാതൃകാ ടൗൺഷിപ്പ്",
            "pathway_self_relocation": "കേരള വി.എൽ.ആർ.എസ് സ്വയം പുനരധിവാസ ധനസഹായം",
            "land_tenure_term": "തണ്ടപ്പേര് / സർവേ നമ്പർ",
        },
    }

    def __init__(self):
        self._tenants: Dict[str, StateTenantPackage] = {}
        self._seed_default_tenants()

    def _seed_default_tenants(self):
        # 1. Kerala Tenant
        self._tenants["KL"] = StateTenantPackage(
            tenant_id="TENANT-KL",
            state_code="KL",
            state_name="Kerala",
            statutory_authority="Kerala State Disaster Management Authority (KSDMA)",
            official_languages=["ml", "en"],
            disaster_relief_manual="Kerala State Disaster Management Plan & LSGD Framework",
            land_record_system="Kerala e-Rekha / Thandaper Cadastral Register",
            default_crs="EPSG:32643",  # UTM Zone 43N
            primary_hazard_mechanics="MONSOON_DEBRIS_FLOW_AND_SLOPE_INSTABILITY",
            active_schemes=[
                "Kerala Vulnerability Linked Relocation Scheme (G.O. Ms 6/2018/DMD)",
                "Wayanad Model Township Package (DM Act §65)",
            ],
        )

        # 2. Uttarakhand Tenant (Second Reference State for Multi-State Adaptation)
        self._tenants["UK"] = StateTenantPackage(
            tenant_id="TENANT-UK",
            state_code="UK",
            state_name="Uttarakhand",
            statutory_authority="Uttarakhand State Disaster Management Authority (USDMA)",
            official_languages=["hi", "en"],
            disaster_relief_manual="Uttarakhand Disaster Relief Manual & SDRF Guidelines",
            land_record_system="Devbhoomi / Bhulekh Uttarakhand (Khasra/Khatauni)",
            default_crs="EPSG:32644",  # UTM Zone 44N
            primary_hazard_mechanics="GLOF_CLOUDBURST_AND_SUBSIDENCE",
            active_schemes=[
                "Uttarakhand SDRF Permanent Resettlement Norms",
                "Joshimath Subsidence Rehabilitation Package",
            ],
        )

    def register_tenant(self, pkg: StateTenantPackage, actor_id: str) -> StateTenantPackage:
        self._tenants[pkg.state_code] = pkg
        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope=f"National/{pkg.state_name}",
            action="REGISTER_STATE_TENANT",
            entity_type="StateTenantPackage",
            entity_id=pkg.tenant_id,
            version_id="1.0",
            reason=f"Registered tenant for {pkg.state_name} ({pkg.state_code}) with {pkg.default_crs}.",
        )
        return pkg

    def get_tenant(self, state_code: str) -> Optional[StateTenantPackage]:
        return self._tenants.get(state_code.upper())

    def list_tenants(self) -> List[StateTenantPackage]:
        return list(self._tenants.values())

    def verify_tenant_isolation(self, user: UserContext, target_state: str):
        """
        Enforce strict state tenant isolation (RUL-054, DEC-037).
        Users authorized in Kerala cannot inspect or modify Uttarakhand state records.
        """
        user_state = user.geography_scope.state
        if user_state in ("National", "ALL", "*"):
            return  # National auditor or central coordinator

        # Normalize state names / codes
        state_map = {"kerala": "KL", "kl": "KL", "uttarakhand": "UK", "uk": "UK"}
        u_code = state_map.get(user_state.lower(), user_state.upper())
        t_code = state_map.get(target_state.lower(), target_state.upper())

        if u_code != t_code:
            raise UnauthorizedGeographyAccessError(
                user_scope=user.geography_scope.state,
                requested_scope=target_state,
            )

    def get_localized_glossary(self, state_code: str, lang: str) -> Dict[str, str]:
        key = f"{state_code.upper()}_{lang.lower()}"
        return self.GLOSSARIES.get(key, {})

    def generate_state_dossier_header(
        self,
        state_code: str,
        case_id: str = "CASE-DEMO-001",
        language: Optional[str] = None,
        actor_id: str = "system",
    ) -> Dict[str, Any]:
        """
        Generate localized dossier header for a given state tenant.
        Zero cross-state leakage guaranteed (DEC-037).
        """
        tenant = self.get_tenant(state_code)
        if not tenant:
            raise KeyError(f"State tenant '{state_code}' not found.")

        target_lang = language or ("hi" if "hi" in tenant.official_languages else "ml")
        glossary = self.get_localized_glossary(state_code, target_lang)

        return {
            "case_id": case_id,
            "state_code": tenant.state_code,
            "state_name": tenant.state_name,
            "statutory_authority": tenant.statutory_authority,
            "legal_framework": tenant.disaster_relief_manual,
            "land_tenure_system": tenant.land_record_system,
            "land_record_system": tenant.land_record_system,
            "crs": tenant.default_crs,
            "crs_projection": tenant.default_crs,
            "title_en": f"PUNARVAS-AI Advisory Relocation Dossier — {tenant.state_name}",
            "title_local": glossary.get("title", ""),
            "advisory_notice_local": glossary.get("advisory_notice", ""),
            "land_tenure_label_local": glossary.get("land_tenure_term", ""),
            "glossary": glossary,
            "is_advisory": True,
        }

    def get_state_fixture_pack(self, state_code: str) -> Dict[str, Any]:
        """
        C5-02: Reference fixture pack for multi-state adaptation (Uttarakhand).
        Demonstrates GLOF/subsidence hazard in Chamoli/Joshimath.
        """
        if state_code.upper() != "UK":
            raise NotImplementedError(f"Fixture pack for state '{state_code}' not implemented.")

        return {
            "state_code": "UK",
            "state_name": "Uttarakhand",
            "district": "Chamoli",
            "primary_hazard_profile": "LAND_SUBSIDENCE_AND_GLOF",
            "hazard_context": "Himalayan tectonic active zone with high land subsidence and GLOF risk.",
            "sample_parcels": [
                {
                    "parcel_id": "PRC-UK-001",
                    "district": "Chamoli (Joshimath)",
                    "village": "Sunil",
                    "khasra_no": "112/1",
                    "area_nali": 5.0,
                }
            ],
            "sample_site": {
                "site_id": "SITE-UK-JOSHIMATH-SAFE-01",
                "village": "Dharkot (Safe Ridge)",
                "district": "Chamoli",
                "authority": "DDMA Chamoli",
                "hazard_susceptibility_level": "LOW",
                "in_debris_flow_runout": False,
                "title_clearance_status": "VERIFIED_CLEAR",
                "land_record_identifier": "Khatauni No. 45, Khasra 142/2",
                "dwelling_capacity": 120,
                "road_access_width_m": 4.5,
                "lean_season_tested_lpcd": 70.0,
                "projected_crs": "EPSG:32644",
            },
            "sample_household": {
                "household_id": "HH-UK-CHAMOLI-001",
                "head_of_household": "Ram Prasad Semwal",
                "member_count": 5,
                "source_khasra_no": "112/1",
                "chosen_pathway": "TOWNSHIP",
            },
        }


# Global singleton instance
multi_state_adapter_service = MultiStateAdapterService()
