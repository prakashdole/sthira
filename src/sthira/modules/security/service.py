"""
Sthira Security & Privacy Controls (ARC-C11 / C1-11).
Normative Reference: rules.md (RUL-050-055), DPDP Rules 2025, CERT-In Directions 2022.
"""

from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from sthira.core.contracts import utc_now
from sthira.core.audit import global_audit_ledger


class DataMinimizationValidator:
    """
    Enforces RUL-050 & RUL-051: Aadhaar, bank, health, or detailed biometric fields
    MUST NOT be collected by default or stored in unrestricted tables.
    """

    PROHIBITED_DEFAULT_FIELDS = {
        "aadhaar", "aadhaar_number", "uidai",
        "bank_account", "bank_account_number", "ifsc",
        "biometric", "medical_diagnosis", "health_history"
    }

    @classmethod
    def validate_payload(cls, data: Dict[str, Any]) -> List[str]:
        """Detect any prohibited personal data field collected without documented necessity."""
        violations = []
        for key in data.keys():
            normalized = key.lower().replace("-", "_").strip()
            if normalized in cls.PROHIBITED_DEFAULT_FIELDS:
                violations.append(f"Prohibited default collection of high-risk field: '{key}' (RUL-051).")
        return violations


class IncidentReport(BaseModel):
    incident_id: str
    incident_type: str  # AUDIT_TAMPERING, UNAUTHORIZED_GEOGRAPHY_ACCESS, EXPORT_LEAKAGE
    severity: str  # CRITICAL, HIGH, MEDIUM
    detected_at: datetime = Field(default_factory=utc_now)
    cert_in_deadline: datetime  # Must report within 6 hours of detection (CERT-In Directions 2022)
    details: str
    reported_to_cert_in: bool = False


class SecurityService:
    """
    Manages data protection impact boundaries, audit safeguards, and CERT-In compliance.
    """

    def create_incident_report(self, incident_type: str, details: str, severity: str = "HIGH") -> IncidentReport:
        now = utc_now()
        # 6-hour statutory reporting deadline under CERT-In Directions 2022
        cert_in_deadline = datetime.fromtimestamp(now.timestamp() + (6 * 3600), tz=timezone.utc)
        report = IncidentReport(
            incident_id=f"INC-{int(now.timestamp())}",
            incident_type=incident_type,
            severity=severity,
            detected_at=now,
            cert_in_deadline=cert_in_deadline,
            details=details,
        )
        global_audit_ledger.log(
            actor_id="SECURITY_MONITOR",
            authority_scope="System/Security",
            action="SECURITY_INCIDENT_FLAGGED",
            entity_type="IncidentReport",
            entity_id=report.incident_id,
            version_id="1.0",
            reason=f"CERT-In 6-hour trigger: {incident_type} - {details}",
        )
        return report


security_service = SecurityService()
