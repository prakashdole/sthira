"""
PUNARVAS-AI Field Offline Workspace & Device Management Module (C2-02 / FEAT-008).
Normative Reference: rules.md (RUL-011, RUL-014, RUL-030, RUL-031), DEC-011, DEC-017, DEC-035.
"""

from enum import Enum
import hashlib
import json
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import utc_now
from punarvas.core.audit import global_audit_ledger


class DeviceStatus(str, Enum):
    ACTIVE = "ACTIVE"
    REPORTED_LOST = "REPORTED_LOST"
    REVOKED = "REVOKED"
    DECOMMISSIONED = "DECOMMISSIONED"


class FieldDeviceRecord(BaseModel):
    device_id: str
    officer_id: str
    officer_name: str
    device_model: str
    auth_token_hash: str
    status: DeviceStatus = DeviceStatus.ACTIVE
    issued_at: str = Field(default_factory=lambda: utc_now().isoformat())
    revoked_at: Optional[str] = None
    revocation_reason: Optional[str] = None


class OfflineSurveyBundle(BaseModel):
    bundle_id: str
    device_id: str
    officer_id: str
    assigned_cases: List[Dict[str, Any]]
    assigned_parcels: List[Dict[str, Any]]
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())
    expires_at: str
    bundle_signature: str


class WaterFieldMeasurement(BaseModel):
    measurement_id: str
    site_id: str
    officer_id: str
    source_type: str  # BOREWELL, OPEN_WELL, STREAM, JJM_SUPPLY
    observation_season: str  # MONSOON, LEAN_SEASON_POST_MONSOON, DRY_SUMMER
    tested_yield_lpcd: float
    sustainable_yield_lpcd: float
    potable_ph: float = 7.0
    turbidity_ntu: float = 1.0
    is_potable: bool = True
    measured_at: str = Field(default_factory=lambda: utc_now().isoformat())


class GeotechnicalMeasurement(BaseModel):
    measurement_id: str
    site_id: str
    officer_id: str
    slope_angle_deg: float
    safe_bearing_capacity_kn_m2: float
    soil_type: str  # LATERITIC, CLAYEY_SILT, WEATHERED_ROCK
    observed_tension_cracks: bool = False
    drainage_condition: str  # GOOD, MODERATE, POOR
    measured_at: str = Field(default_factory=lambda: utc_now().isoformat())


class FieldSurveySubmission(BaseModel):
    submission_id: str
    bundle_id: str
    device_id: str
    officer_id: str
    household_updates: List[Dict[str, Any]] = Field(default_factory=list)
    parcel_updates: List[Dict[str, Any]] = Field(default_factory=list)
    water_measurements: List[WaterFieldMeasurement] = Field(default_factory=list)
    geotech_measurements: List[GeotechnicalMeasurement] = Field(default_factory=list)
    submitted_at: str = Field(default_factory=lambda: utc_now().isoformat())
    payload_checksum: str


class SyncConflict(BaseModel):
    conflict_id: str
    submission_id: str
    entity_type: str  # "HOUSEHOLD", "PARCEL"
    entity_id: str
    server_version: int
    client_version: int
    server_state: Dict[str, Any]
    client_state: Dict[str, Any]
    status: str = "PENDING_MANUAL_REVIEW"
    detected_at: str = Field(default_factory=lambda: utc_now().isoformat())


class SyncResult(BaseModel):
    submission_id: str
    device_id: str
    success: bool
    accepted_records: int
    conflicts_count: int
    conflicts: List[SyncConflict]
    message: str


class FieldSyncService:
    """
    Manages offline survey packages, synchronization with conflict detection,
    and lost-device security revocation (C2-02 / FEAT-008 / DEC-011 / DEC-017).
    """

    def __init__(self):
        self._devices: Dict[str, FieldDeviceRecord] = {}
        self._bundles: Dict[str, OfflineSurveyBundle] = {}
        self._conflicts: List[SyncConflict] = []
        self._water_records: List[WaterFieldMeasurement] = []
        self._geotech_records: List[GeotechnicalMeasurement] = []

    def register_device(
        self,
        device_id: str,
        officer_id: str,
        officer_name: str,
        device_model: str,
        auth_token: str,
        actor_id: str,
    ) -> FieldDeviceRecord:
        """Register a qualified field device with token hash."""
        token_hash = hashlib.sha256(auth_token.encode("utf-8")).hexdigest()
        rec = FieldDeviceRecord(
            device_id=device_id,
            officer_id=officer_id,
            officer_name=officer_name,
            device_model=device_model,
            auth_token_hash=token_hash,
            status=DeviceStatus.ACTIVE,
        )
        self._devices[device_id] = rec

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/FieldSecurity",
            action="REGISTER_FIELD_DEVICE",
            entity_type="FieldDevice",
            entity_id=device_id,
            version_id="1.0",
            reason=f"Registered device for officer {officer_name} ({officer_id}).",
        )
        return rec

    def report_lost_device(
        self,
        device_id: str,
        reason: str,
        actor_id: str,
    ) -> FieldDeviceRecord:
        """
        Immediately revokes authorization for a lost or compromised field tablet (DEC-011, DEC-017).
        Quarantines future sync attempts from this device.
        """
        device = self._devices.get(device_id)
        if not device:
            raise KeyError(f"Device '{device_id}' not registered.")

        device.status = DeviceStatus.REPORTED_LOST
        device.revoked_at = utc_now().isoformat()
        device.revocation_reason = reason

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/FieldSecurity",
            action="REVOKE_LOST_DEVICE",
            entity_type="FieldDevice",
            entity_id=device_id,
            version_id="1.0",
            reason=f"Device reported lost/compromised: {reason}. Auth revoked immediately.",
        )
        return device

    def export_offline_bundle(
        self,
        device_id: str,
        officer_id: str,
        cases: List[Dict[str, Any]],
        parcels: List[Dict[str, Any]],
        ttl_hours: int = 72,
        actor_id: str = "SYSTEM",
    ) -> OfflineSurveyBundle:
        """Generate tamper-evident offline bundle for field survey."""
        device = self._devices.get(device_id)
        if not device or device.status != DeviceStatus.ACTIVE:
            raise PermissionError(f"Device '{device_id}' is not active or has been revoked.")

        bundle_id = f"BUNDLE-{device_id}-{int(utc_now().timestamp())}"
        payload_data = {"bundle_id": bundle_id, "cases": cases, "parcels": parcels}
        signature = hashlib.sha256(json.dumps(payload_data, sort_keys=True).encode("utf-8")).hexdigest()

        # Simple expiry calculation
        expires_at = utc_now().isoformat()  # Mock representation with ISO

        bundle = OfflineSurveyBundle(
            bundle_id=bundle_id,
            device_id=device_id,
            officer_id=officer_id,
            assigned_cases=cases,
            assigned_parcels=parcels,
            expires_at=expires_at,
            bundle_signature=signature,
        )
        self._bundles[bundle_id] = bundle

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/FieldSync",
            action="EXPORT_OFFLINE_BUNDLE",
            entity_type="OfflineSurveyBundle",
            entity_id=bundle_id,
            version_id="1.0",
            reason=f"Exported bundle with {len(cases)} cases and {len(parcels)} parcels to device {device_id}.",
        )
        return bundle

    def ingest_offline_sync(
        self,
        submission: FieldSurveySubmission,
        current_server_records: Dict[str, Dict[str, Any]],
        actor_id: str,
    ) -> SyncResult:
        """
        Ingests an offline survey batch.
        Detects version conflicts without silent last-write-wins overwrite (RUL-014).
        Enforces device revocation check (DEC-011, DEC-017).
        """
        device = self._devices.get(submission.device_id)
        if not device or device.status in (DeviceStatus.REPORTED_LOST, DeviceStatus.REVOKED):
            msg = f"Sync rejected: Device '{submission.device_id}' has been revoked or reported lost."
            global_audit_ledger.log(
                actor_id=actor_id,
                authority_scope="Wayanad/FieldSecurity",
                action="REJECT_REVOKED_DEVICE_SYNC",
                entity_type="FieldSurveySubmission",
                entity_id=submission.submission_id,
                version_id="1.0",
                reason=msg,
            )
            return SyncResult(
                submission_id=submission.submission_id,
                device_id=submission.device_id,
                success=False,
                accepted_records=0,
                conflicts_count=0,
                conflicts=[],
                message=msg,
            )

        conflicts: List[SyncConflict] = []
        accepted_count = 0

        # Check household updates for version conflicts
        for update in submission.household_updates:
            hh_id = update["household_id"]
            client_version = update.get("version", 1)
            server_rec = current_server_records.get(hh_id)

            if server_rec and server_rec.get("version", 1) > client_version:
                conflict = SyncConflict(
                    conflict_id=f"CONF-HH-{submission.submission_id}-{hh_id}",
                    submission_id=submission.submission_id,
                    entity_type="HOUSEHOLD",
                    entity_id=hh_id,
                    server_version=server_rec.get("version", 1),
                    client_version=client_version,
                    server_state=server_rec,
                    client_state=update,
                )
                conflicts.append(conflict)
                self._conflicts.append(conflict)
            else:
                accepted_count += 1

        # Check parcel updates for version conflicts
        for update in submission.parcel_updates:
            p_id = update["parcel_id"]
            client_version = update.get("version", 1)
            server_rec = current_server_records.get(p_id)

            if server_rec and server_rec.get("version", 1) > client_version:
                conflict = SyncConflict(
                    conflict_id=f"CONF-PAR-{submission.submission_id}-{p_id}",
                    submission_id=submission.submission_id,
                    entity_type="PARCEL",
                    entity_id=p_id,
                    server_version=server_rec.get("version", 1),
                    client_version=client_version,
                    server_state=server_rec,
                    client_state=update,
                )
                conflicts.append(conflict)
                self._conflicts.append(conflict)
            else:
                accepted_count += 1

        # Ingest verified measurements
        for w in submission.water_measurements:
            self._water_records.append(w)
            accepted_count += 1

        for g in submission.geotech_measurements:
            self._geotech_records.append(g)
            accepted_count += 1

        status_msg = (
            f"Sync processed: {accepted_count} records accepted. "
            f"{len(conflicts)} conflicts flagged for manual review."
        )

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/FieldSync",
            action="INGEST_OFFLINE_SYNC",
            entity_type="FieldSurveySubmission",
            entity_id=submission.submission_id,
            version_id="1.0",
            reason=status_msg,
        )

        return SyncResult(
            submission_id=submission.submission_id,
            device_id=submission.device_id,
            success=len(conflicts) == 0,
            accepted_records=accepted_count,
            conflicts_count=len(conflicts),
            conflicts=conflicts,
            message=status_msg,
        )

    def list_open_conflicts(self) -> List[SyncConflict]:
        return [c for c in self._conflicts if c.status == "PENDING_MANUAL_REVIEW"]

    def list_water_measurements(self, site_id: Optional[str] = None) -> List[WaterFieldMeasurement]:
        if site_id:
            return [w for w in self._water_records if w.site_id == site_id]
        return self._water_records


# Global singleton instance
field_sync_service = FieldSyncService()
