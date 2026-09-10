"""
Sthira National NDMA Sovereign Relocation Clearinghouse (Phase 6 / National Federation).
Normative Reference: Disaster Management Act 2005 §3 & §6, rules.md (RUL-001, RUL-002, RUL-050-058).

Features:
1. National NDMA Inter-State Relocation Registry & Coordination Hub.
2. Cross-border hazard corridor tracking (inter-state river basins, Western Ghats, Himalayan GLOF corridors).
3. Inter-State Relocation Assistance & Mutual Aid compacts (e.g., NDRF central funding allocation, inter-state land transfer coordination).
4. State Data Sovereignty: SDMAs retain canonical custody of all personal/household data; only de-identified cryptographic manifests and formal inter-state requests are federated.
5. Append-only tamper-evident audit linkage for national inter-state handoffs.
"""

from typing import Any, Dict, List, Optional
from uuid import uuid4
from pydantic import BaseModel, Field

from sthira.core.contracts import AdvisoryEnvelope, utc_now, UserContext
from sthira.core.enums import AuthorityState, RoleType
from sthira.core.errors import AuthorityBypassError
from sthira.core.audit import global_audit_ledger


class InterStateHazardCorridor(BaseModel):
    corridor_id: str
    name: str
    affected_states: List[str]
    hazard_type: str  # GLOF, INTERSTATE_RIVER_BASIN, WESTERN_GHATS_DEBRIS_FLOW
    lead_coordinating_agency: str = "National Disaster Management Authority (NDMA)"
    monitoring_notes: str
    active: bool = True


class InterStateRelocationRequest(BaseModel):
    request_id: str = Field(default_factory=lambda: f"NDMA-REQ-{str(uuid4())[:8].upper()}")
    origin_state: str
    origin_district: str
    destination_state: Optional[str] = None  # None if requesting central NDRF financial aid within origin state
    disaster_event: str
    total_affected_households: int
    requested_assistance_type: str  # NDRF_SPECIAL_PACKAGE, INTERSTATE_LAND_EXCHANGE, TECHNICAL_ASSURANCE
    status: str = "SUBMITTED"  # SUBMITTED, UNDER_INTER_STATE_REVIEW, SANCTIONED, REJECTED
    ndma_remarks: Optional[str] = None
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())


class NationalRegistryManifest(BaseModel):
    manifest_id: str = Field(default_factory=lambda: f"NDMA-REG-{str(uuid4())[:8].upper()}")
    state: str
    district: str
    programme_id: str
    verified_eligible_count: int
    allocated_count: int
    state_audit_head_hash: str
    federated_at: str = Field(default_factory=lambda: utc_now().isoformat())


class NationalClearinghouseService:
    """
    Manages national-level inter-state relocation coordination,
    NDMA registry aggregation, and sovereign state federation.
    """

    def __init__(self):
        self._corridors: Dict[str, InterStateHazardCorridor] = {}
        self._requests: Dict[str, InterStateRelocationRequest] = {}
        self._manifests: List[NationalRegistryManifest] = []
        self._seed_reference_corridors()

    def _seed_reference_corridors(self):
        # 1. Western Ghats Ecological & Landslide Corridor
        self._corridors["CORR-WG-01"] = InterStateHazardCorridor(
            corridor_id="CORR-WG-01",
            name="Western Ghats Nilgiri-Wayanad High Hazard Corridor",
            affected_states=["Kerala", "Tamil Nadu", "Karnataka"],
            hazard_type="WESTERN_GHATS_DEBRIS_FLOW",
            monitoring_notes="Continuous steep slope escarpment across Wayanad, Nilgiris, and Kodagu border zones.",
        )
        # 2. Upper Ganga Himalayan Basin & Glacial Corridor
        self._corridors["CORR-HIM-02"] = InterStateHazardCorridor(
            corridor_id="CORR-HIM-02",
            name="Upper Ganga-Alaknanda Glacial & Subsidence Corridor",
            affected_states=["Uttarakhand", "Himachal Pradesh"],
            hazard_type="GLOF_AND_LAND_SUBSIDENCE",
            monitoring_notes="Covers Joshimath, Chamoli, and border watersheds with high glacial lake risk.",
        )

    def submit_interstate_request(
        self, req: InterStateRelocationRequest, user: UserContext, reason: str
    ) -> InterStateRelocationRequest:
        """Submit an inter-state coordination or NDRF package request to NDMA."""
        if not user.has_role(RoleType.GOVERNMENT_APPROVER):
            raise AuthorityBypassError("Only State Government Approvers (SDMA) can submit national clearinghouse requests.")

        self._requests[req.request_id] = req

        global_audit_ledger.log(
            actor_id=user.user_id,
            authority_scope=f"NATIONAL_CLEARINGHOUSE/{req.origin_state}",
            action="SUBMIT_INTERSTATE_REQUEST",
            entity_type="InterStateRelocationRequest",
            entity_id=req.request_id,
            version_id="1.0",
            reason=reason,
            correlation_id=user.correlation_id,
        )
        return req

    def federate_state_manifest(
        self, manifest: NationalRegistryManifest, user: UserContext, reason: str
    ) -> NationalRegistryManifest:
        """Register a state's cryptographically verified relocation progress with NDMA."""
        self._manifests.append(manifest)

        global_audit_ledger.log(
            actor_id=user.user_id,
            authority_scope=f"NDMA_REGISTRY/{manifest.state}",
            action="FEDERATE_STATE_MANIFEST",
            entity_type="NationalRegistryManifest",
            entity_id=manifest.manifest_id,
            version_id="1.0",
            reason=reason,
            correlation_id=user.correlation_id,
        )
        return manifest

    def list_corridors(self) -> List[InterStateHazardCorridor]:
        return list(self._corridors.values())

    def list_requests(self, state: Optional[str] = None) -> List[InterStateRelocationRequest]:
        if state:
            return [r for r in self._requests.values() if r.origin_state == state or r.destination_state == state]
        return list(self._requests.values())

    def list_manifests(self, state: Optional[str] = None) -> List[NationalRegistryManifest]:
        if state:
            return [m for m in self._manifests if m.state == state]
        return list(self._manifests)


national_clearinghouse_service = NationalClearinghouseService()
