"""Hardened CAP 1.2 XML parser with XXE protection and raw XML preservation.

All text in parsed output is inert synthetic data.  Consumers must escape it
for their output context and must never evaluate imported source text.
"""

from __future__ import annotations

import re
import json
from datetime import datetime, timezone
from defusedxml.ElementTree import fromstring as _safe_xml

from sthira_v2.contracts import (
    CAPCategory,
    CAPCertainty,
    CAPMessageType,
    CAPScope,
    CAPSeverity,
    CAPStatus,
    CAPUrgency,
    CAPUrgency,
    EvidenceClass,
    FactState,
    FreshnessState,
    LineString,
    LocalizedText,
    OfficialAlert,
    Point,
    Polygon,
    SourceProvenance,
    ValidationState,
    ZoneType,
)


# ── CAP field → model mapper ──────────────────────────────────────────────

# CAP category mapping
_CAP_CATEGORY_MAP: dict[str, CAPCategory] = {
    "Geo": CAPCategory.GEO,
    "Met": CAPCategory.MET,
    "Safety": CAPCategory.SAFETY,
    "Security": CAPCategory.SECURITY,
    "Rescue": CAPCategory.RESCUE,
    "Fire": CAPCategory.FIRE,
    "Health": CAPCategory.HEALTH,
    "Env": CAPCategory.ENV,
    "Transport": CAPCategory.TRANSPORT,
    "Infra": CAPCategory.INFRA,
    "CBRNE": CAPCategory.CBRNE,
    "Other": CAPCategory.OTHER,
}

# CAP status mapping
_CAP_STATUS_MAP: dict[str, CAPStatus] = {
    "Actual": CAPStatus.ACTUAL,
    "Exercise": CAPStatus.EXERCISE,
    "System": CAPStatus.SYSTEM,
    "Test": CAPStatus.TEST,
    "Draft": CAPStatus.DRAFT,
}

# CAP message type mapping
_CAP_MTYPE_MAP: dict[str, CAPMessageType] = {
    "Alert": CAPMessageType.ALERT,
    "Update": CAPMessageType.UPDATE,
    "Cancel": CAPMessageType.CANCEL,
    "Ack": CAPMessageType.ACK,
    "Error": CAPMessageType.ERROR,
}

# CAP scope mapping
_CAP_SCOPE_MAP: dict[str, CAPScope] = {
    "Public": CAPScope.PUBLIC,
    "Restricted": CAPScope.RESTRICTED,
    "Private": CAPScope.PRIVATE,
}

# CAP urgency mapping
_CAP_URGENCY_MAP: dict[str, CAPUrgency] = {
    "Immediate": CAPUrgency.IMMEDIATE,
    "Expected": CAPUrgency.EXPECTED,
    "Future": CAPUrgency.FUTURE,
    "Past": CAPUrgency.PAST,
    "Unknown": CAPUrgency.UNKNOWN,
}

# CAP severity mapping
_CAP_SEVERITY_MAP: dict[str, CAPSeverity] = {
    "Extreme": CAPSeverity.EXTREME,
    "Severe": CAPSeverity.SEVERE,
    "Moderate": CAPSeverity.MODERATE,
    "Minor": CAPSeverity.MINOR,
    "Unknown": CAPSeverity.UNKNOWN,
}

# CAP certainty mapping
_CERTAINTY_MAP: dict[str, CAPCertainty] = {
    "Observed": CAPCertainty.OBSERVED,
    "Likely": CAPCertainty.LIKELY,
    "Possible": CAPCertainty.POSSIBLE,
    "Unlikely": CAPCertainty.UNLIKELY,
    "Unknown": CAPCertainty.UNKNOWN,
}
