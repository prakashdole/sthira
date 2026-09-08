"""
PUNARVAS-AI Accessible Reporting, Manifest & Bilingual Export Module (ARC-C10 / C1-09).
Normative Reference: rules.md (RUL-052, RUL-055, RUL-058) and trd.md §3.
"""

import hashlib
import json
from typing import Any, Dict, List
from pydantic import BaseModel, Field

from punarvas.core.contracts import utc_now


class ExportManifest(BaseModel):
    manifest_id: str
    export_type: str  # ACCESSIBLE_HTML, DE_IDENTIFIED_PUBLIC_SUMMARY, AUDIT_PACK
    generated_at: str = Field(default_factory=lambda: utc_now().isoformat())
    generating_user_id: str
    policy_version: str = "POL-WYD-2024.1"
    statutory_authority: str = "District Disaster Management Authority (DDMA), Wayanad"
    is_advisory: bool = True
    sha256_checksum: str
    record_count: int


class ReportingService:
    """
    Generates accessible HTML, bilingual English/Malayalam summaries, and tamper-evident manifests.
    """

    BILINGUAL_GLOSSARY = {
        "title_en": "PUNARVAS-AI Permanent Relocation Advisory Dossier",
        "title_ml": "പുനർവാസ്-എഐ ശാശ്വത പുനരധിവാസ ഉപദേശക രേഖ",
        "advisory_notice_en": "Advisory Decision Support Only. Does not constitute statutory notification.",
        "advisory_notice_ml": "ഉപദേശക സ്വഭാവമുള്ളത് മാത്രം. ഔദ്യോഗിക ഗസറ്റ് വിജ്ഞാപനമല്ല.",
        "status_en": "Status",
        "status_ml": "നിലവിലെ സ്ഥിതി",
        "pathway_en": "Pathway",
        "pathway_ml": "പുനരധിവാസ മാർഗ്ഗം",
        "township_ml": "മാതൃകാ ടൗൺഷിപ്പ്",
        "self_relocation_ml": "സ്വയം പുനരധിവാസ സഹായം",
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


# Global singleton instance
reporting_service = ReportingService()
