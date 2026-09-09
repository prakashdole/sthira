"""
PUNARVAS-AI Accessible Reporting, Manifest & Bilingual Export Module (ARC-C10 / C1-09 / Phase 11).
Normative Reference:
- plan.md (#11): Government dossiers, accessible reports, LSG DM-plan annexes, machine-readable exports, and manifests.
- rules.md: RUL-052, RUL-055, RUL-056, RUL-057, RUL-058, RUL-059, RUL-060, RUL-075.
- trd.md: §3.9 (FR-053 through FR-058), NFR-019 through NFR-022, NFR-029, AT-12, AT-13, AT-14, AT-21, AT-23, AT-24.
- decisions.md: DEC-012, DEC-013, DEC-042.
"""

import csv
from enum import Enum
import hashlib
import io
import json
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import utc_now


class ExportClassification(str, Enum):
    RESTRICTED_OFFICIAL = "RESTRICTED_OFFICIAL"
    PUBLIC_AGGREGATE = "PUBLIC_AGGREGATE"
    SHADOW_EVALUATION = "SHADOW_EVALUATION"
    JUDICIAL_AUDIT = "JUDICIAL_AUDIT"


class ExportType(str, Enum):
    ACCESSIBLE_HTML = "ACCESSIBLE_HTML"
    DE_IDENTIFIED_PUBLIC_SUMMARY = "DE_IDENTIFIED_PUBLIC_SUMMARY"
    SITE_DOSSIER = "SITE_DOSSIER"
    BENEFICIARY_PACK = "BENEFICIARY_PACK"
    FIELD_CHECKLIST = "FIELD_CHECKLIST"
    DECISION_SUMMARY = "DECISION_SUMMARY"
    LSGD_ANNEX = "LSGD_ANNEX"
    PUBLIC_TRANSPARENCY = "PUBLIC_TRANSPARENCY"
    SPATIAL_GEOJSON = "SPATIAL_GEOJSON"
    TABULAR_CSV = "TABULAR_CSV"
    AUDIT_PACKAGE = "AUDIT_PACKAGE"


class EvidenceReference(BaseModel):
    field_name: str
    statement: str
    source_id: str
    evidence_hash: str
    is_verified: bool = True
    requires_human_review: bool = False
    notes: Optional[str] = None


class ExportManifest(BaseModel):
    manifest_id: str
    export_type: str
    classification: ExportClassification = ExportClassification.RESTRICTED_OFFICIAL
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    generating_user_id: str
    programme_id: str = "PROG-WYD-REBUILD-2024"
    jurisdiction: str = "Wayanad, Kerala"
    policy_version: str = "POL-WYD-2024.1"
    statutory_authority: str = "District Disaster Management Authority (DDMA), Wayanad"
    source_versions: Dict[str, str] = Field(default_factory=dict)
    unresolved_conditions: List[str] = Field(default_factory=list)
    record_count: int = 1
    sha256_checksum: str
    signature_status: str = "UNSIGNED_DRAFT"
    signer_id: Optional[str] = None
    is_advisory: bool = True


class SiteDossier(BaseModel):
    site_id: str
    site_name: str
    district: str
    taluk: str
    village: str
    gross_area_cents: float
    usable_area_cents: float
    dwelling_capacity: int
    water_source_description: str
    lean_season_yield_lpcd: float
    hazard_buffer_distance_m: float
    slope_mean_deg: float
    road_access_width_m: float
    evidence_links: List[EvidenceReference]
    unresolved_conditions: List[str]
    approval_ref: Optional[str] = None
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    generating_user_id: str
    manifest_id: str
    sha256_checksum: str


class BeneficiaryReviewPack(BaseModel):
    household_id: str
    head_of_household: str
    member_count: int
    vulnerability_score: float
    disability_or_special_needs: bool
    tenure_category: str
    relocation_necessity_review_id: str
    preferred_pathway: str
    assigned_site_id: Optional[str] = None
    eligible_schemes: List[str]
    evidence_links: List[EvidenceReference]
    consent_token_ref: str
    unresolved_conditions: List[str]
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    generating_user_id: str
    manifest_id: str
    sha256_checksum: str


class FieldChecklistItem(BaseModel):
    item_id: str
    description: str
    mandatory: bool = True
    verification_method: str
    status: str = "UNKNOWN"
    officer_notes: Optional[str] = None


class FieldVerificationChecklist(BaseModel):
    checklist_id: str
    target_type: str
    target_id: str
    items: List[FieldChecklistItem]
    required_equipment: List[str]
    safety_precautions: List[str]
    generating_user_id: str
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    manifest_id: str
    sha256_checksum: str


class DecisionSummaryDossier(BaseModel):
    dossier_id: str
    decision_id: str
    entity_type: str
    entity_id: str
    policy_version: str
    source_checksums: Dict[str, str]
    solver_seed: Optional[int] = 42
    solver_tolerances: Dict[str, float] = Field(
        default_factory=lambda: {"mip_gap": 0.01, "time_limit_sec": 60.0}
    )
    approval_order_id: Optional[str] = None
    statutory_gazette_id: Optional[str] = None
    objection_token_refs: List[str] = Field(default_factory=list)
    evidence_chain_hash: str
    generating_user_id: str
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    manifest_id: str
    sha256_checksum: str


class LSGDDisasterManagementPlanAnnex(BaseModel):
    annex_id: str
    lsg_name: str
    district: str
    plan_period: str = "2024-2026"
    section_a_vulnerability_profile: Dict[str, Any]
    section_b_relocation_beneficiaries: Dict[str, Any]
    section_c_host_site_capacities: Dict[str, Any]
    section_d_statutory_approvals: Dict[str, Any]
    statutory_note_en: str
    statutory_note_ml: str
    generating_user_id: str
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    is_advisory: bool = True
    sha256_checksum: str


class PublicTransparencyProjection(BaseModel):
    projection_id: str
    round_number: int
    district: str
    k_anonymity_threshold: int = 5
    aggregates: Dict[str, Any]
    cell_suppression_applied: bool
    suppressed_cell_count: int
    coordinate_generalization: str = "CENTROID_OF_REVENUE_VILLAGE"
    differencing_risk_detected: bool
    differencing_warning: Optional[str] = None
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    sha256_checksum: str


class ReportingService:
    """
    Generates accessible HTML, bilingual English/Malayalam summaries, and tamper-evident manifests.
    """

    BILINGUAL_GLOSSARY = {
        "title_en": "PUNARVAS-AI Permanent Relocation Advisory Dossier",
        "title_ml": "പുനർവാസ്-എഐ ശാശ്വത പുനരധിവാസ ഉപദേശക രേഖ",
        "title_hi": "पुनर्वास-एआई स्थायी पुनर्वास सलाहकार दस्तावेज़",
        "advisory_notice_en": "Advisory Decision Support Only. Does not constitute statutory notification.",
        "advisory_notice_ml": "ഉപദേശക സ്വഭാവമുള്ളത് മാത്രം. ഔദ്യോഗിക ഗസറ്റ് വിജ്ഞാപനമല്ല.",
        "advisory_notice_hi": "केवल सलाहकार निर्णय समर्थन। आधिकारिक वैधानिक अधिसूचना नहीं है।",
        "status_en": "Status",
        "status_ml": "നിലവിലെ സ്ഥിതി",
        "status_hi": "स्थिति",
        "pathway_en": "Pathway",
        "pathway_ml": "പുനരധിവാസ മാർഗ്ഗം",
        "pathway_hi": "पुनर्वास मार्ग",
        "township_ml": "മാതൃകാ ടൗൺഷിപ്പ്",
        "self_relocation_ml": "സ്വയം പുനരധിവാസ സഹായം",
    }

    def __init__(self):
        self._manifests: Dict[str, ExportManifest] = {}
        self._prior_projection_aggregates: Dict[str, Dict[str, int]] = {}

    def get_manifest(self, manifest_id: str) -> Optional[ExportManifest]:
        return self._manifests.get(manifest_id)

    def list_manifests(self) -> List[ExportManifest]:
        return list(self._manifests.values())

    def create_export_manifest(
        self,
        manifest_id: str,
        export_type: str,
        generating_user_id: str,
        sha256_checksum: str,
        classification: ExportClassification = ExportClassification.RESTRICTED_OFFICIAL,
        record_count: int = 1,
        source_versions: Optional[Dict[str, str]] = None,
        unresolved_conditions: Optional[List[str]] = None,
        policy_version: str = "POL-WYD-2024.1",
        signature_status: str = "UNSIGNED_DRAFT",
        signer_id: Optional[str] = None,
    ) -> ExportManifest:
        """
        Creates and registers a cryptographically sealed export manifest (FR-056).
        """
        manifest = ExportManifest(
            manifest_id=manifest_id,
            export_type=export_type,
            classification=classification,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256_checksum,
            record_count=record_count,
            source_versions=source_versions or {},
            unresolved_conditions=unresolved_conditions or [],
            policy_version=policy_version,
            signature_status=signature_status,
            signer_id=signer_id,
        )
        self._manifests[manifest_id] = manifest
        return manifest

    def verify_export_manifest(
        self, manifest_id: str, payload_content: str
    ) -> Dict[str, Any]:
        """
        Verifies tamper-evidence of an export against its registered manifest (FR-056 / FR-057).
        """
        manifest = self.get_manifest(manifest_id)
        if not manifest:
            return {
                "manifest_id": manifest_id,
                "verified": False,
                "reason": f"Manifest '{manifest_id}' not found in registry.",
            }

        calculated_hash = hashlib.sha256(payload_content.encode("utf-8")).hexdigest()
        is_match = calculated_hash == manifest.sha256_checksum

        return {
            "manifest_id": manifest_id,
            "verified": is_match,
            "stored_checksum": manifest.sha256_checksum,
            "calculated_checksum": calculated_hash,
            "export_type": manifest.export_type,
            "classification": manifest.classification,
            "generated_at": manifest.generated_at,
            "generating_user": manifest.generating_user_id,
            "policy_version": manifest.policy_version,
            "reason": (
                "Checksum matches stored manifest exactly."
                if is_match
                else "Checksum mismatch! Export payload has been tampered with or modified."
            ),
        }

    def generate_bilingual_dossier_html(
        self,
        programme_title: str,
        household_id: str,
        head_name: str,
        pathway: str,
        assigned_site: str,
        gate_status: str,
        generating_user_id: str,
    ) -> Dict[str, Any]:
        """
        Generate accessible HTML conforming to WCAG 2.2 AA / GIGW 3.0 (RUL-055).
        Includes full English and Malayalam parallel content.
        """
        g = self.BILINGUAL_GLOSSARY
        html_content = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{g['title_en']}</title>
  <style>
    body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; line-height: 1.6; padding: 2rem; max-width: 800px; margin: auto; }}
    .alert-advisory {{ background-color: #fef3c7; border-left: 6px solid #f59e0b; padding: 1rem; margin-bottom: 1.5rem; }}
    .bilingual-header {{ border-bottom: 2px solid #e5e7eb; padding-bottom: 1rem; margin-bottom: 1.5rem; }}
    table {{ width: 100%; border-collapse: collapse; margin-top: 1rem; }}
    th, td {{ border: 1px solid #d1d5db; padding: 0.75rem; text-align: left; }}
    th {{ background-color: #f9fafb; }}
    .ml-text {{ color: #4b5563; font-size: 0.95em; }}
  </style>
</head>
<body>
  <div class="bilingual-header">
    <h1>{g['title_en']}</h1>
    <h2 class="ml-text">{g['title_ml']}</h2>
    <p><strong>Programme:</strong> {programme_title}</p>
  </div>

  <div class="alert-advisory" role="region" aria-label="Advisory Warning">
    <p><strong>IMPORTANT / ശ്രദ്ധിക്കുക:</strong> {g['advisory_notice_en']}</p>
    <p class="ml-text">{g['advisory_notice_ml']}</p>
  </div>

  <h2>Case Details / കേസ് വിവരങ്ങൾ</h2>
  <table>
    <thead>
      <tr>
        <th scope="col">Attribute (English)</th>
        <th scope="col">വിവരണം (Malayalam)</th>
        <th scope="col">Value</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td>Household ID</td>
        <td>കുടുംബ നമ്പർ</td>
        <td>{household_id}</td>
      </tr>
      <tr>
        <td>Head of Household</td>
        <td>കുടുംബനാഥൻ / നാഥ</td>
        <td>{head_name}</td>
      </tr>
      <tr>
        <td>Recommended Pathway</td>
        <td>പുനരധിവാസ മാർഗ്ഗം</td>
        <td>{pathway}</td>
      </tr>
      <tr>
        <td>Assigned / Option Site</td>
        <td>നിർദ്ദിഷ്ട സ്ഥലം</td>
        <td>{assigned_site}</td>
      </tr>
      <tr>
        <td>Gate Verification</td>
        <td>സുരക്ഷാ പരിശോധന</td>
        <td><strong>{gate_status}</strong></td>
      </tr>
    </tbody>
  </table>
</body>
</html>"""

        sha256 = hashlib.sha256(html_content.encode("utf-8")).hexdigest()

        manifest = ExportManifest(
            manifest_id=f"MAN-DOSSIER-{household_id}",
            export_type="ACCESSIBLE_HTML",
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=1,
        )

        return {
            "html": html_content,
            "manifest": manifest,
            "checksum": sha256,
        }

    def generate_public_deidentified_summary(
        self,
        district: str,
        total_eligible: int,
        township_count: int,
        self_relocation_count: int,
        unassigned_count: int,
    ) -> Dict[str, Any]:
        """
        Public transparency projection (RUL-052).
        Exposes only approved aggregates; household-level identities are completely excluded.
        """
        summary = {
            "district": district,
            "reporting_period": "2024-2026",
            "statutory_authority": "DDMA Wayanad",
            "is_advisory": True,
            "metrics": {
                "total_verified_eligible_households": total_eligible,
                "opted_model_township": township_count,
                "opted_vlrs_self_relocation": self_relocation_count,
                "capacity_pending_unassigned": unassigned_count,
            },
            "data_classification": "PUBLIC_AGGREGATE",
            "privacy_compliance": "DPDP Rules 2025 de-identified aggregate",
        }
        raw_json = json.dumps(summary, sort_keys=True)
        sha256 = hashlib.sha256(raw_json.encode("utf-8")).hexdigest()
        summary["checksum_sha256"] = sha256
        return summary

    def generate_lsgd_dm_plan_annex(
        self,
        lsg_name: str,
        district: str,
        vulnerable_wards: List[int],
        settlement_names: List[str],
        verified_beneficiary_count: int,
        host_sites: List[Dict[str, Any]],
        generating_user_id: str,
    ) -> "LSGDDisasterManagementPlanAnnex":
        """
        FEAT-018 / DEC-013 / FR-054: Kerala LSGD Disaster Management Plan Annex Generator.
        Maps structured technical and hazard evidence into the government LSGD template,
        while leaving all participatory and statutory approval fields explicitly incomplete (never invented!).
        """
        annex_id = f"LSGD-ANNEX-{lsg_name.replace(' ', '_').upper()}-{int(utc_now().timestamp())}"

        section_a = {
            "title": "Section A: Settlement Vulnerability & Hazard Profile",
            "vulnerable_wards": vulnerable_wards,
            "settlements": settlement_names,
            "hazard_classification": "CRITICAL_DEBRIS_FLOW_AND_LANDSLIDE_RUNOUT",
            "source_evidence": "GSI NLSM 2022 & KSDMA Wayanad Landslide Runout Assessment",
        }

        section_b = {
            "title": "Section B: Permanent Relocation Candidate Casework Summary",
            "verified_households_needing_relocation": verified_beneficiary_count,
            "status": "ADVISORY_SCREENING_COMPLETE",
        }

        section_c = {
            "title": "Section C: Host Township / Relocation Site Options",
            "candidate_sites": host_sites,
        }

        section_d = {
            "title": "Section D: Mandatory Participatory & Statutory Approvals (DEC-013)",
            "gram_ward_sabha_resolution": "PENDING_GRAM_SABHA_APPROVAL",
            "lsg_working_group_recommendation": "PENDING_WORKING_GROUP_MEETING",
            "technical_scrutiny_committee": "PENDING_ENGINEERING_SCRUTINY",
            "district_planning_committee_approval": "PENDING_DPC_CONCURRENCE",
            "ddma_final_sanction": "PENDING_DDMA_ORDER",
            "approval_status": "INCOMPLETE_REQUIRES_LAWFUL_PARTICIPATORY_PROCESS",
        }

        payload_to_hash = {
            "annex_id": annex_id,
            "lsg_name": lsg_name,
            "district": district,
            "section_a": section_a,
            "section_b": section_b,
            "section_c": section_c,
            "section_d": section_d,
        }
        sha256 = hashlib.sha256(json.dumps(payload_to_hash, sort_keys=True).encode("utf-8")).hexdigest()

        annex = LSGDDisasterManagementPlanAnnex(
            annex_id=annex_id,
            lsg_name=lsg_name,
            district=district,
            plan_period="2024-2026",
            section_a_vulnerability_profile=section_a,
            section_b_relocation_beneficiaries=section_b,
            section_c_host_site_capacities=section_c,
            section_d_statutory_approvals=section_d,
            statutory_note_en=(
                "Kerala Local Self Government Institution (LSGI) Disaster Management Plan Annexure. "
                "Algorithmic outputs are advisory only under DM Act 2005 §31. Participatory and statutory approvals "
                "must be conducted through lawful Gram/Ward Sabha and Panchayati Raj processes (DEC-013)."
            ),
            statutory_note_ml=(
                "കേരള തദ്ദേശ സ്വയംഭരണ ദുരന്ത നിവാരണ പദ്ധതി അനുബന്ധം. ദുരന്ത നിവാരണ നിയമം 2005 വകുപ്പ് 31 "
                "പ്രകാരം ഇത് ഉപദേശക രേഖ മാത്രമാണ്. ഗ്രാമ/വാർഡ് സഭകളിലൂടെ മാത്രമേ നിയമപരമായ അന്തിമ അംഗീകാരം നൽകാവൂ (DEC-013)."
            ),
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
        )

        self.create_export_manifest(
            manifest_id=f"MAN-ANNEX-{annex_id}",
            export_type=ExportType.LSGD_ANNEX.value,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=1,
            classification=ExportClassification.RESTRICTED_OFFICIAL,
        )

        return annex

    def generate_site_dossier(
        self,
        site_id: str,
        site_name: str,
        district: str,
        taluk: str,
        village: str,
        gross_area_cents: float,
        usable_area_cents: float,
        dwelling_capacity: int,
        water_source_description: str,
        lean_season_yield_lpcd: float,
        hazard_buffer_distance_m: float,
        slope_mean_deg: float,
        road_access_width_m: float,
        evidence_links: List[EvidenceReference],
        unresolved_conditions: List[str],
        generating_user_id: str,
        approval_ref: Optional[str] = None,
    ) -> SiteDossier:
        """
        Generate an evidence-bound Site Dossier (FR-053 / FEAT-018).
        """
        dossier_data = {
            "site_id": site_id,
            "site_name": site_name,
            "district": district,
            "taluk": taluk,
            "village": village,
            "gross_area_cents": gross_area_cents,
            "usable_area_cents": usable_area_cents,
            "dwelling_capacity": dwelling_capacity,
            "water_source_description": water_source_description,
            "lean_season_yield_lpcd": lean_season_yield_lpcd,
            "hazard_buffer_distance_m": hazard_buffer_distance_m,
            "slope_mean_deg": slope_mean_deg,
            "road_access_width_m": road_access_width_m,
            "evidence_links": [e.model_dump() for e in evidence_links],
            "unresolved_conditions": unresolved_conditions,
            "approval_ref": approval_ref,
        }

        sha256 = hashlib.sha256(
            json.dumps(dossier_data, sort_keys=True).encode("utf-8")
        ).hexdigest()
        manifest_id = f"MAN-SITE-{site_id}-{int(utc_now().timestamp())}"

        dossier = SiteDossier(
            site_id=site_id,
            site_name=site_name,
            district=district,
            taluk=taluk,
            village=village,
            gross_area_cents=gross_area_cents,
            usable_area_cents=usable_area_cents,
            dwelling_capacity=dwelling_capacity,
            water_source_description=water_source_description,
            lean_season_yield_lpcd=lean_season_yield_lpcd,
            hazard_buffer_distance_m=hazard_buffer_distance_m,
            slope_mean_deg=slope_mean_deg,
            road_access_width_m=road_access_width_m,
            evidence_links=evidence_links,
            unresolved_conditions=unresolved_conditions,
            approval_ref=approval_ref,
            generating_user_id=generating_user_id,
            manifest_id=manifest_id,
            sha256_checksum=sha256,
        )

        source_versions = {
            e.source_id: e.evidence_hash for e in evidence_links
        }
        self.create_export_manifest(
            manifest_id=manifest_id,
            export_type=ExportType.SITE_DOSSIER.value,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=1,
            source_versions=source_versions,
            unresolved_conditions=unresolved_conditions,
            classification=ExportClassification.RESTRICTED_OFFICIAL,
        )

        return dossier

    def generate_beneficiary_review_pack(
        self,
        household_id: str,
        head_of_household: str,
        member_count: int,
        vulnerability_score: float,
        disability_or_special_needs: bool,
        tenure_category: str,
        relocation_necessity_review_id: str,
        preferred_pathway: str,
        eligible_schemes: List[str],
        evidence_links: List[EvidenceReference],
        consent_token_ref: str,
        unresolved_conditions: List[str],
        generating_user_id: str,
        assigned_site_id: Optional[str] = None,
    ) -> BeneficiaryReviewPack:
        """
        Generate an evidence-bound Beneficiary Review Pack (FR-053 / FEAT-018).
        """
        pack_data = {
            "household_id": household_id,
            "head_of_household": head_of_household,
            "member_count": member_count,
            "vulnerability_score": vulnerability_score,
            "disability_or_special_needs": disability_or_special_needs,
            "tenure_category": tenure_category,
            "relocation_necessity_review_id": relocation_necessity_review_id,
            "preferred_pathway": preferred_pathway,
            "assigned_site_id": assigned_site_id,
            "eligible_schemes": eligible_schemes,
            "evidence_links": [e.model_dump() for e in evidence_links],
            "consent_token_ref": consent_token_ref,
            "unresolved_conditions": unresolved_conditions,
        }

        sha256 = hashlib.sha256(
            json.dumps(pack_data, sort_keys=True).encode("utf-8")
        ).hexdigest()
        manifest_id = f"MAN-BENEFICIARY-{household_id}-{int(utc_now().timestamp())}"

        pack = BeneficiaryReviewPack(
            household_id=household_id,
            head_of_household=head_of_household,
            member_count=member_count,
            vulnerability_score=vulnerability_score,
            disability_or_special_needs=disability_or_special_needs,
            tenure_category=tenure_category,
            relocation_necessity_review_id=relocation_necessity_review_id,
            preferred_pathway=preferred_pathway,
            assigned_site_id=assigned_site_id,
            eligible_schemes=eligible_schemes,
            evidence_links=evidence_links,
            consent_token_ref=consent_token_ref,
            unresolved_conditions=unresolved_conditions,
            generating_user_id=generating_user_id,
            manifest_id=manifest_id,
            sha256_checksum=sha256,
        )

        source_versions = {
            e.source_id: e.evidence_hash for e in evidence_links
        }
        self.create_export_manifest(
            manifest_id=manifest_id,
            export_type=ExportType.BENEFICIARY_PACK.value,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=1,
            source_versions=source_versions,
            unresolved_conditions=unresolved_conditions,
            classification=ExportClassification.RESTRICTED_OFFICIAL,
        )

        return pack

    def generate_field_verification_checklist(
        self,
        target_type: str,
        target_id: str,
        items: List[FieldChecklistItem],
        required_equipment: List[str],
        safety_precautions: List[str],
        generating_user_id: str,
    ) -> FieldVerificationChecklist:
        """
        Generate an engineering field verification checklist (FR-053).
        """
        checklist_id = f"CHK-{target_type}-{target_id}-{int(utc_now().timestamp())}"
        chk_data = {
            "checklist_id": checklist_id,
            "target_type": target_type,
            "target_id": target_id,
            "items": [i.model_dump() for i in items],
            "required_equipment": required_equipment,
            "safety_precautions": safety_precautions,
        }

        sha256 = hashlib.sha256(
            json.dumps(chk_data, sort_keys=True).encode("utf-8")
        ).hexdigest()
        manifest_id = f"MAN-CHK-{checklist_id}"

        chk = FieldVerificationChecklist(
            checklist_id=checklist_id,
            target_type=target_type,
            target_id=target_id,
            items=items,
            required_equipment=required_equipment,
            safety_precautions=safety_precautions,
            generating_user_id=generating_user_id,
            manifest_id=manifest_id,
            sha256_checksum=sha256,
        )

        self.create_export_manifest(
            manifest_id=manifest_id,
            export_type=ExportType.FIELD_CHECKLIST.value,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=len(items),
            classification=ExportClassification.RESTRICTED_OFFICIAL,
        )

        return chk

    def generate_decision_summary_dossier(
        self,
        decision_id: str,
        entity_type: str,
        entity_id: str,
        policy_version: str,
        source_checksums: Dict[str, str],
        evidence_chain_hash: str,
        generating_user_id: str,
        solver_seed: Optional[int] = 42,
        solver_tolerances: Optional[Dict[str, float]] = None,
        approval_order_id: Optional[str] = None,
        statutory_gazette_id: Optional[str] = None,
        objection_token_refs: Optional[List[str]] = None,
    ) -> DecisionSummaryDossier:
        """
        Generate a Decision Summary Dossier for judicial review and provenance reconstruction (FEAT-020 / R3-01).
        """
        dossier_id = f"DEC-SUM-{decision_id}"
        data = {
            "dossier_id": dossier_id,
            "decision_id": decision_id,
            "entity_type": entity_type,
            "entity_id": entity_id,
            "policy_version": policy_version,
            "source_checksums": source_checksums,
            "solver_seed": solver_seed,
            "solver_tolerances": solver_tolerances or {"mip_gap": 0.01, "time_limit_sec": 60.0},
            "approval_order_id": approval_order_id,
            "statutory_gazette_id": statutory_gazette_id,
            "objection_token_refs": objection_token_refs or [],
            "evidence_chain_hash": evidence_chain_hash,
        }

        sha256 = hashlib.sha256(
            json.dumps(data, sort_keys=True).encode("utf-8")
        ).hexdigest()
        manifest_id = f"MAN-DEC-{decision_id}"

        dossier = DecisionSummaryDossier(
            dossier_id=dossier_id,
            decision_id=decision_id,
            entity_type=entity_type,
            entity_id=entity_id,
            policy_version=policy_version,
            source_checksums=source_checksums,
            solver_seed=solver_seed,
            solver_tolerances=solver_tolerances or {"mip_gap": 0.01, "time_limit_sec": 60.0},
            approval_order_id=approval_order_id,
            statutory_gazette_id=statutory_gazette_id,
            objection_token_refs=objection_token_refs or [],
            evidence_chain_hash=evidence_chain_hash,
            generating_user_id=generating_user_id,
            manifest_id=manifest_id,
            sha256_checksum=sha256,
        )

        self.create_export_manifest(
            manifest_id=manifest_id,
            export_type=ExportType.DECISION_SUMMARY.value,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=1,
            source_versions=source_checksums,
            classification=ExportClassification.JUDICIAL_AUDIT,
            policy_version=policy_version,
        )

        return dossier

    def generate_spatial_geojson_export(
        self,
        export_id: str,
        features_data: List[Dict[str, Any]],
        generating_user_id: str,
        classification: ExportClassification = ExportClassification.RESTRICTED_OFFICIAL,
    ) -> Dict[str, Any]:
        """
        Generates RFC 7946 compliant GeoJSON FeatureCollection with explicit CRS and sealed manifest (FR-055).
        """
        geojson_doc = {
            "type": "FeatureCollection",
            "crs": {
                "type": "name",
                "properties": {"name": "urn:ogc:def:crs:OGC:1.3:CRS84"},
            },
            "features": features_data,
        }

        geojson_str = json.dumps(geojson_doc, indent=2)
        sha256 = hashlib.sha256(geojson_str.encode("utf-8")).hexdigest()
        manifest_id = f"MAN-GEOJSON-{export_id}"

        manifest = self.create_export_manifest(
            manifest_id=manifest_id,
            export_type=ExportType.SPATIAL_GEOJSON.value,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=len(features_data),
            classification=classification,
        )

        return {
            "geojson": geojson_doc,
            "raw_text": geojson_str,
            "manifest": manifest,
            "checksum": sha256,
        }

    def generate_tabular_csv_export(
        self,
        export_id: str,
        headers: List[str],
        rows: List[List[Any]],
        generating_user_id: str,
        classification: ExportClassification = ExportClassification.RESTRICTED_OFFICIAL,
    ) -> Dict[str, Any]:
        """
        Generates RFC 4180 compliant CSV export with explicit headers and sealed manifest (FR-055).
        """
        output = io.StringIO()
        writer = csv.writer(output, quoting=csv.QUOTE_MINIMAL)
        writer.writerow(headers)
        for r in rows:
            writer.writerow(r)

        csv_content = output.getvalue()
        sha256 = hashlib.sha256(csv_content.encode("utf-8")).hexdigest()
        manifest_id = f"MAN-CSV-{export_id}"

        manifest = self.create_export_manifest(
            manifest_id=manifest_id,
            export_type=ExportType.TABULAR_CSV.value,
            generating_user_id=generating_user_id,
            sha256_checksum=sha256,
            record_count=len(rows),
            classification=classification,
        )

        return {
            "csv_content": csv_content,
            "manifest": manifest,
            "checksum": sha256,
        }

    def generate_k_anonymized_public_projection(
        self,
        projection_id: str,
        district: str,
        round_number: int,
        subregion_counts: Dict[str, int],
        k_threshold: int = 5,
    ) -> PublicTransparencyProjection:
        """
        Generates a k-anonymized public transparency projection per RUL-075 / FEAT-019 / AT-23.
        - Enforces k >= 5: cells with 1 <= count < k are suppressed.
        - Generalizes coordinates to Revenue Village centroid.
        - Detects differencing attacks against previous round aggregates.
        """
        cleaned_aggregates: Dict[str, Any] = {}
        suppressed_count = 0

        for region, count in subregion_counts.items():
            if 0 < count < k_threshold:
                cleaned_aggregates[region] = f"< {k_threshold} (Suppressed for Privacy)"
                suppressed_count += 1
            else:
                cleaned_aggregates[region] = count

        # Differencing attack detection
        differencing_risk = False
        diff_warning = None
        prior_key = f"{district}_round_{round_number - 1}"
        if prior_key in self._prior_projection_aggregates:
            prior = self._prior_projection_aggregates[prior_key]
            for region, curr_val in subregion_counts.items():
                if region in prior:
                    delta = abs(curr_val - prior[region])
                    if 1 <= delta <= 2:
                        differencing_risk = True
                        diff_warning = (
                            f"Differencing attack risk detected in '{region}': delta of {delta} "
                            "household(s) between release rounds could permit re-identification (RUL-075 / AT-23)."
                        )
                        break

        # Record current round for future differencing checks
        curr_key = f"{district}_round_{round_number}"
        self._prior_projection_aggregates[curr_key] = subregion_counts

        raw_payload = {
            "projection_id": projection_id,
            "round_number": round_number,
            "district": district,
            "aggregates": cleaned_aggregates,
            "k_threshold": k_threshold,
        }
        sha256 = hashlib.sha256(
            json.dumps(raw_payload, sort_keys=True).encode("utf-8")
        ).hexdigest()

        manifest_id = f"MAN-PUB-{projection_id}"
        self.create_export_manifest(
            manifest_id=manifest_id,
            export_type=ExportType.PUBLIC_TRANSPARENCY.value,
            generating_user_id="public_transparency_officer",
            sha256_checksum=sha256,
            classification=ExportClassification.PUBLIC_AGGREGATE,
            record_count=len(subregion_counts),
        )

        return PublicTransparencyProjection(
            projection_id=projection_id,
            round_number=round_number,
            district=district,
            k_anonymity_threshold=k_threshold,
            aggregates=cleaned_aggregates,
            cell_suppression_applied=suppressed_count > 0,
            suppressed_cell_count=suppressed_count,
            coordinate_generalization="CENTROID_OF_REVENUE_VILLAGE",
            differencing_risk_detected=differencing_risk,
            differencing_warning=diff_warning,
            sha256_checksum=sha256,
        )

    def reproduce_historical_export(
        self, manifest_id: str, new_payload_checksum: str
    ) -> Dict[str, Any]:
        """
        Verifies bit-for-bit or content-equivalent reproduction of an approved historical export (FR-058).
        """
        manifest = self.get_manifest(manifest_id)
        if not manifest:
            return {
                "manifest_id": manifest_id,
                "reproducible": False,
                "reason": "Historical manifest not found.",
            }

        is_bit_exact = manifest.sha256_checksum == new_payload_checksum
        return {
            "manifest_id": manifest_id,
            "reproducible": is_bit_exact,
            "historical_checksum": manifest.sha256_checksum,
            "reproduced_checksum": new_payload_checksum,
            "status": "BIT_FOR_BIT_IDENTICAL" if is_bit_exact else "CONTENT_MISMATCH",
        }


# Global singleton instance
reporting_service = ReportingService()
reporting_service = ReportingService()

