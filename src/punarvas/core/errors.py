"""
PUNARVAS-AI Typed Domain Exceptions (PKG-0C).
Normative Reference: rules.md (RUL-001, RUL-013, RUL-017, RUL-029, RUL-035).
"""


class PunarvasError(Exception):
    """Base exception for all PUNARVAS-AI domain failures."""
    def __init__(self, message: str, rule_id: str = "", resolution: str = ""):
        super().__init__(message)
        self.message = message
        self.rule_id = rule_id
        self.resolution = resolution


class AuthorityBypassError(PunarvasError):
    """Raised when software permissions attempt to act as statutory authority (RUL-002, RUL-004)."""
    def __init__(self, message: str = "Automated system cannot approve, gazette, or notify without human authority."):
        super().__init__(message, rule_id="RUL-002", resolution="Submit proposal to authorized DDMA/SDMA role for review.")


class MissingEvidenceError(PunarvasError):
    """Raised when required evidence is absent; absence of evidence cannot pass (RUL-013)."""
    def __init__(self, missing_items: str):
        super().__init__(
            f"Missing required evidence: {missing_items}. Status must remain UNKNOWN or BLOCKED.",
            rule_id="RUL-013",
            resolution="Acquire verified evidence before advancing gate."
        )


class HardGateBlockedError(PunarvasError):
    """Raised when an operation is attempted on an entity failing a mandatory hard gate (RUL-029)."""
    def __init__(self, gate_name: str, state: str, reason: str):
        super().__init__(
            f"Mandatory gate '{gate_name}' returned '{state}': {reason}. Advancement blocked.",
            rule_id="RUL-029",
            resolution="Resolve blocking condition or select alternative site/pathway."
        )


class MasterScoreProhibitedError(PunarvasError):
    """Raised if an implementation attempts an opaque Omega or single composite master score (RUL-035)."""
    def __init__(self):
        super().__init__(
            "Computing a single composite Omega master score is strictly prohibited by RUL-035.",
            rule_id="RUL-035",
            resolution="Evaluate distinct inspectable dimensions separately after hard gates."
        )


class OutOfCoverageError(PunarvasError):
    """Raised when a data layer (such as C-FLOOD) is used outside its validated geographic coverage (RUL-017)."""
    def __init__(self, source_name: str, requested_aoi: str, valid_coverage: str):
        super().__init__(
            f"Source '{source_name}' cannot be used for '{requested_aoi}'. Documented coverage is restricted to: {valid_coverage}.",
            rule_id="RUL-017",
            resolution="Switch to authorized local district product (e.g. KSDMA/GSI)."
        )


class UnauthorizedGeographyAccessError(PunarvasError):
    """Raised when a user attempts actions outside their authorized jurisdiction scope (RUL-054)."""
    def __init__(self, user_scope: str, requested_scope: str):
        super().__init__(
            f"User jurisdiction scope '{user_scope}' does not permit access to '{requested_scope}'.",
            rule_id="RUL-054",
            resolution="Request delegated authority or multi-jurisdiction role."
        )


class AuditIntegrityError(PunarvasError):
    """Raised when append-only audit hash chain verification detects tampering (RUL-056)."""
    def __init__(self, event_id: str, expected_hash: str, actual_hash: str):
        super().__init__(
            f"Audit log integrity check failed at event {event_id}. Expected {expected_hash}, found {actual_hash}.",
            rule_id="RUL-056",
            resolution="Trigger security incident protocol and restore from validated backup."
        )


class CapacityExceededError(PunarvasError):
    """Raised when allocation or reservation exceeds site dwelling or water capacity (RUL-041, RUL-070)."""
    def __init__(self, site_id: str, requested: int, available: int, metric: str = "dwellings"):
        super().__init__(
            f"Site '{site_id}' capacity exceeded for {metric}. Requested: {requested}, Available: {available}.",
            rule_id="RUL-041",
            resolution="Scale back allocation or reserve additional site parcels."
        )
