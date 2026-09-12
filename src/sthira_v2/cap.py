"""Small, fail-closed CAP 1.2 ingestion boundary for local/demo use.

The parser deliberately accepts no DTDs.  CAP content is untrusted input and
is retained verbatim only as an audit/artifact value, never as executable text.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
import hashlib
import re
from typing import Callable, Protocol
from xml.etree import ElementTree as ET

from .contracts import (
    CAPArea, CAPCategory, CAPCertainty, CAPMessageType, CAPResource,
    CAPScope, CAPSeverity, CAPStatus, CAPUrgency, FactState, FreshnessState,
    LocalizedText, OfficialAlert, SourceProvenance, AlertState, Polygon,
    EvidenceClass, ValidationState,
)

MAX_XML_BYTES = 1_000_000
_TAG = re.compile(r"(?P<name>[^}]+)$")


class CAPError(ValueError):
    """Invalid or unauthorized CAP artifact."""


def _children(node: ET.Element, name: str) -> list[ET.Element]:
    return [child for child in node if _TAG.search(child.tag).group("name") == name]


def _child(node: ET.Element, name: str, required: bool = True) -> str | None:
    values = _children(node, name)
    if not values:
        if required:
            raise CAPError(f"missing CAP field: {name}")
        return None
    value = "".join(values[0].itertext()).strip()
    if required and not value:
        raise CAPError(f"empty CAP field: {name}")
    return value or None


def _time(value: str) -> datetime:
    try:
        parsed = datetime.fromisoformat(value.strip().replace("Z", "+00:00"))
    except ValueError as exc:
        raise CAPError(f"invalid CAP timestamp: {value}") from exc
    if parsed.tzinfo is None:
        raise CAPError("CAP timestamp must be timezone-aware")
    return parsed.astimezone(timezone.utc)


def _enum(enum_type, value: str):
    try:
        return enum_type(value)
    except ValueError as exc:
        raise CAPError(f"invalid CAP {enum_type.__name__}: {value}") from exc


def _polygon(value: str) -> Polygon:
    points = []
    for pair in value.split():
        try:
            lat, lon = (float(part) for part in pair.split(","))
        except (ValueError, TypeError) as exc:
            raise CAPError("invalid CAP polygon") from exc
        points.append((lon, lat))
    if len(points) < 4 or points[0] != points[-1]:
        raise CAPError("CAP polygon must be closed and have four positions")
    return Polygon(coordinates=(tuple(points),))


@dataclass(frozen=True)
class ParsedCAP:
    alert: OfficialAlert
    raw_xml: bytes


class CAPHTTPResponse(Protocol):
    status_code: int
    headers: dict[str, str]
    content: bytes


@dataclass(frozen=True)
class CAPFetchResult:
    state: str
    parsed: ParsedCAP | None
    etag: str | None
    error: str | None = None


class CAPHTTPAdapter:
    """ETag-aware CAP refresh seam with bounded retries and cache preservation."""

    def __init__(
        self,
        fetch: Callable[[dict[str, str], float], CAPHTTPResponse],
        *,
        source_uri: str,
        sender_allow_list: set[str] | frozenset[str],
        timeout_seconds: float = 10.0,
        max_attempts: int = 3,
        sleep: Callable[[float], None] | None = None,
        jitter: Callable[[], float] | None = None,
    ) -> None:
        if timeout_seconds <= 0 or max_attempts < 1:
            raise ValueError("CAP HTTP bounds are invalid")
        self.fetch = fetch
        self.source_uri = source_uri
        self.sender_allow_list = frozenset(sender_allow_list)
        self.timeout_seconds = timeout_seconds
        self.max_attempts = max_attempts
        self.sleep = sleep or (lambda _seconds: None)
        self.jitter = jitter or (lambda: 0.0)
        self.etag: str | None = None
        self.cached: ParsedCAP | None = None
        self.consecutive_failures = 0

    def refresh(self, *, retrieved_at: datetime | None = None) -> CAPFetchResult:
        headers = {"Accept": "application/xml", "Cache-Control": "no-cache"}
        if self.etag:
            headers["If-None-Match"] = self.etag
        last_error: str | None = None
        for attempt in range(self.max_attempts):
            try:
                response = self.fetch(headers, self.timeout_seconds)
                if response.status_code == 304:
                    if self.cached is None:
                        raise CAPError("304 received without a valid cached CAP")
                    self.consecutive_failures = 0
                    return CAPFetchResult("CACHED_NOT_MODIFIED", self.cached, self.etag)
                if response.status_code != 200:
                    raise CAPError(f"CAP source returned HTTP {response.status_code}")
                parsed = parse_cap(
                    response.content,
                    source_uri=self.source_uri,
                    sender_allow_list=self.sender_allow_list,
                    retrieved_at=retrieved_at,
                )
                self.cached = parsed
                self.etag = response.headers.get("ETag") or response.headers.get("etag")
                self.consecutive_failures = 0
                return CAPFetchResult("UPDATED", parsed, self.etag)
            except (CAPError, TimeoutError, OSError) as exc:
                last_error = str(exc)
                if attempt + 1 < self.max_attempts:
                    self.sleep(min(30.0, 2**attempt + max(0.0, self.jitter())))
        self.consecutive_failures += 1
        if self.cached is not None:
            return CAPFetchResult("STALE_CACHE", self.cached, self.etag, last_error)
        return CAPFetchResult("UNAVAILABLE", None, self.etag, last_error)


def parse_cap(
    raw_xml: bytes | str,
    *,
    source_uri: str = "fixture://synthetic-cap",
    sender_allow_list: set[str] | frozenset[str] = frozenset(),
    retrieved_at: datetime | None = None,
) -> ParsedCAP:
    raw = raw_xml.encode() if isinstance(raw_xml, str) else bytes(raw_xml)
    if len(raw) > MAX_XML_BYTES:
        raise CAPError("CAP artifact exceeds maximum size")
    if re.search(br"<!DOCTYPE|<!ENTITY", raw, flags=re.IGNORECASE):
        raise CAPError("DTD and entity declarations are forbidden")
    try:
        root = ET.fromstring(raw)
    except ET.ParseError as exc:
        raise CAPError("malformed CAP XML") from exc
    if _TAG.search(root.tag).group("name") != "alert":
        raise CAPError("CAP root must be alert")

    identifier = _child(root, "identifier")
    sender = _child(root, "sender")
    if sender_allow_list and sender not in sender_allow_list:
        raise CAPError("CAP sender is not allow-listed")
    sent = _time(_child(root, "sent"))
    status = _enum(CAPStatus, _child(root, "status"))
    msg_type = _enum(CAPMessageType, _child(root, "msgType"))
    scope = _enum(CAPScope, _child(root, "scope"))
    references = tuple(filter(None, (_child(root, "references", False) or "").split()))
    infos = _children(root, "info")
    if not infos:
        raise CAPError("CAP requires an info block")
    info = infos[0]
    language = _child(info, "language")
    category = _enum(CAPCategory, _child(info, "category"))
    area_nodes = _children(info, "area")
    areas: list[CAPArea] = []
    for area in area_nodes:
        polygons = tuple(_polygon(p) for p in [_child(area, "polygon", False)] if p)
        areas.append(CAPArea(area_description=_child(area, "areaDesc"), polygons=polygons))
    if not areas:
        raise CAPError("CAP requires an area")
    effective = _time(_child(info, "effective", False) or _child(root, "sent"))
    expires_text = _child(info, "expires")
    expires = _time(expires_text)
    if expires <= effective:
        raise CAPError("CAP expires must be after effective")
    now = retrieved_at or datetime.now(timezone.utc)
    issued = sent
    digest = hashlib.sha256(raw).hexdigest()
    text = _child(info, "instruction", False) or _child(info, "description")
    resources = tuple(
        CAPResource(
            description=_child(resource, "resourceDesc"),
            mime_type=_child(resource, "mimeType", False),
            uri=_child(resource, "uri"),
            digest=_child(resource, "digest", False),
        )
        for resource in _children(info, "resource")
    )
    alert = OfficialAlert(
        identifier=identifier, sender=sender, sent=sent, issued=issued,
        status=status, message_type=msg_type, scope=scope, references=references,
        language=language, categories=(category,), event=_child(info, "event"),
        response_types=tuple(x.strip() for x in (_child(info, "responseType", False) or "").split(",")),
        urgency=_enum(CAPUrgency, _child(info, "urgency")),
        severity=_enum(CAPSeverity, _child(info, "severity")),
        certainty=_enum(CAPCertainty, _child(info, "certainty")),
        effective=effective, onset=_time(_child(info, "onset", False)) if _child(info, "onset", False) else None,
        expires=expires, sender_name=sender, headline=_child(info, "headline"),
        description=_child(info, "description"), instructions=(LocalizedText(
            language=language, text=text, human_reviewed=False
        ),), areas=tuple(areas), resources=resources, lifecycle_state=AlertState.RECEIVED,
        version=1, provenance=SourceProvenance(
            source_id="CAP", authority_name=sender, source_uri=source_uri,
            artifact_id=identifier, artifact_sha256=digest, retrieved_at=now,
            issued_at=sent, version=1, evidence_class=EvidenceClass.SYNTHETIC_DEMO
        ), fact_state=FactState(
            validation=ValidationState.VALID,
            freshness=FreshnessState.EXPIRED if expires <= now else FreshnessState.CURRENT,
            last_refreshed_at=now
        )
    ).transition_to(AlertState.VALIDATED).transition_to(AlertState.ACTIVE)
    if msg_type is CAPMessageType.CANCEL:
        alert = alert.transition_to(AlertState.CANCELLED)
    return ParsedCAP(alert=alert, raw_xml=raw)


class AlertLifecycleService:
    """In-memory, fixture-backed CAP lifecycle store."""

    def __init__(self, *, sender_allow_list: set[str] | frozenset[str], clock: Callable[[], datetime] | None = None):
        self.sender_allow_list = frozenset(sender_allow_list)
        self.clock = clock or (lambda: datetime.now(timezone.utc))
        self.alerts: dict[str, ParsedCAP] = {}
        self.quarantine: list[dict[str, str]] = []

    def ingest(self, raw_xml: bytes | str, **kwargs) -> ParsedCAP | None:
        try:
            parsed = parse_cap(raw_xml, sender_allow_list=self.sender_allow_list,
                               retrieved_at=self.clock(), **kwargs)
        except CAPError as exc:
            self.quarantine.append({"reason": str(exc)})
            return None
        alert = parsed.alert
        if alert.message_type is CAPMessageType.CANCEL:
            for reference in alert.references:
                prior = self.alerts.get(reference.split(",")[0])
                if prior:
                    self.alerts[reference.split(",")[0]] = ParsedCAP(
                        prior.alert.model_copy(update={"lifecycle_state": AlertState.CANCELLED,
                                                       "fact_state": prior.alert.fact_state.model_copy(update={"freshness": FreshnessState.EXPIRED})}),
                        prior.raw_xml)
            self.alerts[alert.identifier] = parsed
        elif alert.identifier in self.alerts:
            return self.alerts[alert.identifier]
        else:
            for reference in alert.references:
                prior = self.alerts.get(reference.split(",")[0])
                if prior:
                    self.alerts[reference.split(",")[0]] = ParsedCAP(
                        prior.alert.model_copy(update={"lifecycle_state": AlertState.SUPERSEDED}), prior.raw_xml)
            self.alerts[alert.identifier] = parsed
        return parsed

    def expire(self) -> None:
        now = self.clock()
        for key, parsed in list(self.alerts.items()):
            if parsed.alert.lifecycle_state is AlertState.ACTIVE and parsed.alert.expires <= now:
                self.alerts[key] = ParsedCAP(parsed.alert.model_copy(update={
                    "lifecycle_state": AlertState.EXPIRED,
                    "fact_state": parsed.alert.fact_state.model_copy(update={"freshness": FreshnessState.EXPIRED}),
                }), parsed.raw_xml)

    def active(self) -> list[ParsedCAP]:
        self.expire()
        return [p for p in self.alerts.values() if p.alert.lifecycle_state is AlertState.ACTIVE]

    def get(self, identifier: str) -> ParsedCAP | None:
        self.expire()
        return self.alerts.get(identifier)
