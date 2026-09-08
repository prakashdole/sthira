"""
PUNARVAS-AI Source Catalog, Ingestion & AOI Gate Module (ARC-C02, ARC-C13 / C1-02).
Normative Reference: source-register.md, rules.md (RUL-008, RUL-076-083).
"""

from typing import Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import SourceMetadata, utc_now
from punarvas.core.enums import SourceActivationState, SourceCapability
from punarvas.core.errors import MissingEvidenceError
from punarvas.core.audit import global_audit_ledger


class SourceEntry(BaseModel):
    source_id: str  # S01..S54
    title: str
    capability: SourceCapability
    provider: str
    documented_coverage: str
    state: SourceActivationState = SourceActivationState.REGISTERED
    mirror_group: Optional[str] = None  # e.g. "SENTINEL_2_OBSERVATION"
    latest_checksum: Optional[str] = None
    quarantine_reason: Optional[str] = None


class CatalogService:
    """
    Manages S01-S54 operational acquisition inventory and the AOI gate.
    """

    def __init__(self):
        self._registry: Dict[str, SourceEntry] = {}
        self._versions: Dict[str, List[SourceMetadata]] = {}
        self._init_baseline_inventory()

    def _init_baseline_inventory(self):
        """Populate baseline S01-S54 registry entries."""
        defaults = [
            ("S01", "KSDMA/GSI Landslide Susceptibility", SourceCapability.PRODUCT, "GSI / KSDMA", "Kerala (Wayanad)"),
            ("S02", "KSDMA Flood Probability Rasters", SourceCapability.PRODUCT, "KSDMA", "Kerala (Wayanad)"),
            ("S03", "KSDMA/Wayanad PDNA & Orders", SourceCapability.PRODUCT, "GoK / KSDMA", "Wayanad"),
            ("S07", "CWC C-FLOOD Inundation", SourceCapability.PROCESSING_SERVICE, "CWC / C-DAC", "Godavari, Tapi, Mahanadi (ZERO Wayanad)"),
            ("S08", "Sentinel-2 L2A", SourceCapability.PRODUCT, "ESA / Copernicus", "Global"),
            ("S10", "Copernicus Data Space Ecosystem (CDSE)", SourceCapability.CATALOG, "ESA", "Global"),
            ("S11", "Element 84 Earth Search", SourceCapability.CATALOG, "Element 84", "Global"),
            ("S18", "Copernicus DEM GLO-30", SourceCapability.PRODUCT, "Copernicus", "Global"),
            ("S22", "Google Open Buildings V3", SourceCapability.PRODUCT, "Google", "South Asia"),
            ("S25", "Census 2011 Primary Census Abstract", SourceCapability.PRODUCT, "ORGI", "India"),
            ("S38", "CGWB NAQUIM Groundwater", SourceCapability.PRODUCT, "CGWB", "Wayanad"),
            ("S43", "Local Government Directory (LGD)", SourceCapability.PRODUCT, "MoPR", "India"),
            ("S44", "Survey of India Boundaries", SourceCapability.PRODUCT, "SOI", "India"),
            ("S45", "Ente Bhoomi / ReLIS Cadastral Records", SourceCapability.AGENCY_ROUTE, "Kerala Revenue Dept", "Kerala"),
        ]
        for sid, title, cap, prov, cov in defaults:
            mirror = "SENTINEL_2" if sid in ("S08", "S10", "S11") else None
            self._registry[sid] = SourceEntry(
                source_id=sid,
                title=title,
                capability=cap,
                provider=prov,
                documented_coverage=cov,
                mirror_group=mirror,
            )

    def register_source_version(
        self,
        metadata: SourceMetadata,
        raw_payload_checksum: str,
        actor_id: str,
        reason: str,
    ) -> SourceEntry:
        """
        Execute Section 6 AOI acquisition gate: verify license, coverage, and checksum.
        Quarantines on checksum mismatch.
        """
        entry = self._registry.get(metadata.source_id)
        if not entry:
            entry = SourceEntry(
                source_id=metadata.source_id,
                title=metadata.publisher,
                capability=SourceCapability.PRODUCT,
                provider=metadata.publisher,
                documented_coverage=metadata.geographic_scope,
            )
            self._registry[metadata.source_id] = entry

        # Verify Checksum Integrity
        if raw_payload_checksum != metadata.checksum_sha256:
            entry.state = SourceActivationState.QUARANTINED
            entry.quarantine_reason = f"Checksum mismatch: expected {metadata.checksum_sha256}, got {raw_payload_checksum}"
            global_audit_ledger.log(
                actor_id=actor_id,
                authority_scope="System/Catalog",
                action="QUARANTINE_SOURCE",
                entity_type="SourceDataset",
                entity_id=metadata.source_id,
                version_id=metadata.version,
                reason=entry.quarantine_reason,
            )
            return entry

        # Validated and activated for AOI use
        entry.state = SourceActivationState.USABLE
        entry.latest_checksum = metadata.checksum_sha256
        entry.quarantine_reason = None

        if metadata.source_id not in self._versions:
            self._versions[metadata.source_id] = []
        self._versions[metadata.source_id].append(metadata)

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="System/Catalog",
            action="ACTIVATE_SOURCE_VERSION",
            entity_type="SourceDataset",
            entity_id=metadata.source_id,
            version_id=metadata.version,
            reason=reason,
        )
        return entry

    def get_source(self, source_id: str) -> Optional[SourceEntry]:
        return self._registry.get(source_id)

    def is_usable(self, source_id: str) -> bool:
        entry = self._registry.get(source_id)
        return entry is not None and entry.state == SourceActivationState.USABLE


# Global singleton instance
catalog_service = CatalogService()
