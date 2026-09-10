"""
Sthira Canonical Domain Enums (PKG-0C / ARC-C01-13).
Normative Reference: rules.md (RUL-001 - RUL-083) and trd.md.
"""

from enum import Enum


class GateState(str, Enum):
    """
    Mandatory Hard Gate evaluation state (RUL-029).
    A required hard gate MUST return only PASS, FAIL, UNKNOWN, or BLOCKED.
    Missing evidence MUST return UNKNOWN/BLOCKED, never PASS (RUL-013).
    """
    PASS = "PASS"
    FAIL = "FAIL"
    UNKNOWN = "UNKNOWN"
    BLOCKED = "BLOCKED"


class AuthorityState(str, Enum):
    """
    Separate explicit administrative and legal lifecycle states (RUL-003, RUL-004).
    Algorithmic output is advisory until officially approved (RUL-001).
    """
    DRAFT = "DRAFT"
    ANALYTICAL = "ANALYTICAL"
    FIELD_VERIFIED = "FIELD_VERIFIED"
    TECHNICALLY_ENDORSED = "TECHNICALLY_ENDORSED"
    OFFICIALLY_APPROVED = "OFFICIALLY_APPROVED"
    OFFICIALLY_NOTIFIED = "OFFICIALLY_NOTIFIED"
    SUPERSEDED = "SUPERSEDED"
    WITHDRAWN = "WITHDRAWN"


class RelocationPathway(str, Enum):
    """
    Relocation and rehabilitation pathway choices (DEC-018, R0-01).
    """
    TOWNSHIP = "TOWNSHIP"  # Integrated model township (e.g. Elstone Estate model)
    SELF_RELOCATION_ASSISTANCE = "SELF_RELOCATION_ASSISTANCE"  # Financial assistance (e.g. VLRS ₹10L model)
    IN_SITU_MITIGATION = "IN_SITU_MITIGATION"  # Engineered retention/drainage in place (RUL-067)
    EXCLUDED = "EXCLUDED"  # Assessed as not requiring permanent relocation


class FundingState(str, Enum):
    """
    Explicit funding lifecycle states (RUL-069).
    Announced budgets must not be treated as household funding.
    """
    IDENTIFIED = "IDENTIFIED"
    APPLIED = "APPLIED"
    SANCTIONED = "SANCTIONED"
    COMMITTED = "COMMITTED"
    RELEASED = "RELEASED"
    RECEIVED = "RECEIVED"
    SPENT = "SPENT"
    RECONCILED = "RECONCILED"
    WITHDRAWN = "WITHDRAWN"


class DiscrepancyType(str, Enum):
    """
    Land truth and parcel observation discrepancy categories (RUL-021-026, ARC-C04).
    """
    PAPER_VACANT_GROUND_OCCUPIED = "PAPER_VACANT_GROUND_OCCUPIED"
    CADASTRAL_GRID_SHIFT = "CADASTRAL_GRID_SHIFT"  # Historical chain-survey vs DGPS offset
    UNRESOLVED_FRA_CLAIM = "UNRESOLVED_FRA_CLAIM"  # Forest Rights Act individual or community claim
    WATER_SEASONAL_DEFICIT = "WATER_SEASONAL_DEFICIT"  # Lean-season failure despite monsoon water
    SLOPE_RUNOUT_HAZARD = "SLOPE_RUNOUT_HAZARD"  # Low slope but within debris flow channel/runout (RUL-018)
    BOUNDARY_DISPUTE = "BOUNDARY_DISPUTE"


class SourceCapability(str, Enum):
    """
    Distinct data-source capabilities across S01-S54 inventory (RUL-076).
    """
    PRODUCT = "PRODUCT"
    CATALOG = "CATALOG"
    PROCESSING_SERVICE = "PROCESSING_SERVICE"
    DISPLAY_BASEMAP = "DISPLAY_BASEMAP"
    AGENCY_ROUTE = "AGENCY_ROUTE"
    FIELD_ROUTE = "FIELD_ROUTE"


class SourceActivationState(str, Enum):
    """
    Source version activation states for the AOI acquisition gate (RUL-077, source-register §6).
    """
    REGISTERED = "REGISTERED"
    ACQUIRING = "ACQUIRING"
    VALIDATED = "VALIDATED"
    QUARANTINED = "QUARANTINED"
    USABLE = "USABLE"
    STALE = "STALE"
    BLOCKED = "BLOCKED"


class DecisionState(str, Enum):
    """
    Workflow, objection, and remedy progression states (RUL-046-049, ARC-C09).
    """
    PENDING = "PENDING"
    UNDER_REVIEW = "UNDER_REVIEW"
    APPROVED = "APPROVED"
    REJECTED = "REJECTED"
    OBJECTION_FILED = "OBJECTION_FILED"
    HEARING_SCHEDULED = "HEARING_SCHEDULED"
    REMEDY_GRANTED = "REMEDY_GRANTED"
    OBJECTION_DISMISSED = "OBJECTION_DISMISSED"


class ConsentPurpose(str, Enum):
    """
    Distinct lawful-basis consent purposes (RUL-044, DEC-028, DPDP Rules 2025).
    Silence or missing contact is never consent.
    """
    PROGRAMME_PARTICIPATION = "PROGRAMME_PARTICIPATION"
    PATHWAY_CHOICE = "PATHWAY_CHOICE"
    SITE_PREFERENCE = "SITE_PREFERENCE"
    OFFER_ACCEPTANCE = "OFFER_ACCEPTANCE"
    ASSISTED_SERVICE_DELEGATION = "ASSISTED_SERVICE_DELEGATION"
    RESEARCH_DE_IDENTIFIED = "RESEARCH_DE_IDENTIFIED"


class SolverStatus(str, Enum):
    """
    Advisory allocation solver status taxonomy (RUL-073).
    """
    OPTIMAL = "OPTIMAL"
    FEASIBLE_NOT_PROVEN_OPTIMAL = "FEASIBLE_NOT_PROVEN_OPTIMAL"
    INFEASIBLE = "INFEASIBLE"
    TIME_LIMIT_WITHOUT_INCUMBENT = "TIME_LIMIT_WITHOUT_INCUMBENT"
    ERROR = "ERROR"


class RoleType(str, Enum):
    """
    Fine-grained system authorization roles (ARC-C01, RUL-054).
    """
    GOVERNMENT_APPROVER = "GOVERNMENT_APPROVER"  # District Collector / DDMA Chairperson
    DISASTER_MANAGEMENT_OFFICER = "DISASTER_MANAGEMENT_OFFICER"
    LEGAL_OFFICER = "LEGAL_OFFICER"
    REVENUE_OFFICER = "REVENUE_OFFICER"
    GIS_ANALYST = "GIS_ANALYST"
    FIELD_VERIFIER = "FIELD_VERIFIER"
    COMMUNITY_OFFICER = "COMMUNITY_OFFICER"
    AUDITOR = "AUDITOR"
    STATE_PROGRAMME_ADMIN = "STATE_PROGRAMME_ADMIN"
    PUBLIC_VIEWER = "PUBLIC_VIEWER"


class ClassificationLevel(str, Enum):
    """
    Data sensitivity and privacy protection classification (RUL-050-052).
    """
    PUBLIC = "PUBLIC"  # Aggregates and officially gazetted notices
    INTERNAL = "INTERNAL"  # Desktop screening and preliminary candidate sites
    RESTRICTED = "RESTRICTED"  # Personal data, household claims, exact parcel coordinates prior to publication
    CONFIDENTIAL = "CONFIDENTIAL"  # Sensitive personal identifiers, dispute records, security keys
