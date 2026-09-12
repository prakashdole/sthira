"""Deterministic, non-consequential Voice Map Control command parser."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
import re


class MapIntent(StrEnum):
    SHOW_MY_LOCATION = "SHOW_MY_LOCATION"
    SHOW_ALERT_AREA = "SHOW_ALERT_AREA"
    SHOW_SAFE_ZONE = "SHOW_SAFE_ZONE"
    SHOW_ROUTE = "SHOW_ROUTE"
    SHOW_ALL_SAFE_ZONES = "SHOW_ALL_SAFE_ZONES"
    ZOOM_IN = "ZOOM_IN"
    ZOOM_OUT = "ZOOM_OUT"
    PAN_NORTH = "PAN_NORTH"
    PAN_SOUTH = "PAN_SOUTH"
    PAN_EAST = "PAN_EAST"
    PAN_WEST = "PAN_WEST"
    RECENTER = "RECENTER"
    REPEAT_INSTRUCTION = "REPEAT_INSTRUCTION"
    CHANGE_LANGUAGE = "CHANGE_LANGUAGE"
    OPEN_EMERGENCY_CALL_CONFIRMATION = "OPEN_EMERGENCY_CALL_CONFIRMATION"
    FOCUS_PLACE = "FOCUS_PLACE"


@dataclass(frozen=True)
class ParsedCommand:
    intent: MapIntent
    place: str | None = None
    needs_confirmation: bool = False
    target_id: str | None = None
    language: str | None = None


_PATTERNS: tuple[tuple[MapIntent, re.Pattern[str]], ...] = (
    (MapIntent.SHOW_MY_LOCATION, re.compile(r"\b(show|find|locate)\b.*\b(my location|where i am|current position)\b", re.I)),
    (MapIntent.SHOW_ALERT_AREA, re.compile(r"\b(show|display)\b.*\b(alert|danger|red)\b", re.I)),
    (MapIntent.SHOW_ALL_SAFE_ZONES, re.compile(r"\b(show|display)\b.*\b(all|every)\b.*\b(safe zone|shelter)\b", re.I)),
    (MapIntent.SHOW_ROUTE, re.compile(r"\b(show|repeat|display)\b.*\b(route|way|directions)\b|\bhow do i reach\b", re.I)),
    (MapIntent.SHOW_SAFE_ZONE, re.compile(r"\b(show|display)\b.*\b(assigned|my|safe zone|destination|shelter)\b", re.I)),
    (MapIntent.ZOOM_IN, re.compile(r"\b(zoom in|closer)\b", re.I)),
    (MapIntent.ZOOM_OUT, re.compile(r"\b(zoom out|wider|farther)\b", re.I)),
    (MapIntent.PAN_NORTH, re.compile(r"\b(pan|move)\b.*\b(north|up)\b", re.I)),
    (MapIntent.PAN_SOUTH, re.compile(r"\b(pan|move)\b.*\b(south|down)\b", re.I)),
    (MapIntent.PAN_EAST, re.compile(r"\b(pan|move)\b.*\b(east|right)\b", re.I)),
    (MapIntent.PAN_WEST, re.compile(r"\b(pan|move)\b.*\b(west|left)\b", re.I)),
    (MapIntent.RECENTER, re.compile(r"\b(recenter|centre|center)\b", re.I)),
    (MapIntent.REPEAT_INSTRUCTION, re.compile(r"\b(repeat|again)\b.*\b(route|instruction|directions)\b", re.I)),
    (MapIntent.CHANGE_LANGUAGE, re.compile(r"\b(change|switch|use)\b.*\b(language|english|malayalam|മലയാളം)\b", re.I)),
    (MapIntent.OPEN_EMERGENCY_CALL_CONFIRMATION, re.compile(r"\b(open|call)\b.*\b(emergency|112)\b", re.I)),
)


_PLACE_ALIASES = {
    "community hall": ("Community Hall", "SZ-DEMO-01"),
    "demo community hall": ("Demo Community Hall", "SZ-DEMO-01"),
    "school shelter": ("School Shelter", "SZ-DEMO-02"),
    "demo school shelter": ("Demo School Shelter", "SZ-DEMO-02"),
    "sports centre": ("Sports Centre", "SZ-DEMO-03"),
    "sports center": ("Sports Centre", "SZ-DEMO-03"),
}


def parse_map_command(transcript: str, *, confidence: float, threshold: float = 0.85) -> ParsedCommand | None:
    """Parse only allow-listed map commands; all consequential requests fail closed."""
    if not transcript.strip() or confidence < threshold:
        return None
    lowered = transcript.lower()
    if re.search(r"\b(arrive|confirm|capacity|assign|cancel|override|predict|safest|nearest|best|ignore|system prompt)\b", lowered):
        return None
    for intent, pattern in _PATTERNS:
        if pattern.search(transcript):
            if intent is MapIntent.OPEN_EMERGENCY_CALL_CONFIRMATION:
                return ParsedCommand(intent, needs_confirmation=True)
            if intent is MapIntent.CHANGE_LANGUAGE:
                language = "ml-IN" if re.search(r"malayalam|മലയാളം", lowered) else "en-IN"
                return ParsedCommand(intent, language=language)
            return ParsedCommand(intent)
    match = re.fullmatch(r"\s*(?:focus|show)\s+(?:on|at)?\s*(?P<place>[a-z0-9][a-z0-9 .'-]{1,100})\s*", transcript, re.I)
    if match:
        place = match.group("place").strip()
        known = _PLACE_ALIASES.get(place.lower())
        if known is None:
            return None
        return ParsedCommand(MapIntent.FOCUS_PLACE, known[0], target_id=known[1])
    return None
