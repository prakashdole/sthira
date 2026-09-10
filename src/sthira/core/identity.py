"""
Prototype identity directory and HMAC session tokens (REM-001, DEC-046).
Local signed sessions stand in for the Keycloak/OIDC broker. Identity is
taken from a verified token, never from request body fields.
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import secrets
import time
from typing import Dict, List, Optional

from sthira.core.contracts import GeographyScope, UserContext
from sthira.core.enums import ClassificationLevel, RoleType

TOKEN_TTL_SECONDS = 8 * 3600
PBKDF2_ROUNDS = 120_000
_PASSWORD_SALT = b"sthira-prototype-directory-v1"
PROTOTYPE_PASSWORD = os.environ.get("STHIRA_PROTOTYPE_PASSWORD", "local-prototype-only")


def _auth_secret() -> bytes:
    env = os.environ.get("STHIRA_AUTH_SECRET")
    if env:
        return env.encode("utf-8")
    return b"sthira-prototype-hmac-not-for-production"


def hash_password(password: str) -> str:
    return hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), _PASSWORD_SALT, PBKDF2_ROUNDS).hex()


def _b64url(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii")


def _b64url_decode(text: str) -> bytes:
    pad = "=" * (-len(text) % 4)
    return base64.urlsafe_b64decode(text + pad)


class DirectoryUser:
    def __init__(
        self,
        username: str,
        user_id: str,
        roles: List[RoleType],
        password_hash: str,
        state: str = "Kerala",
        district: Optional[str] = "Wayanad",
        classification: ClassificationLevel = ClassificationLevel.RESTRICTED,
    ):
        self.username = username
        self.user_id = user_id
        self.roles = roles
        self.password_hash = password_hash
        self.state = state
        self.district = district
        self.classification = classification

    def to_context(self) -> UserContext:
        return UserContext(
            user_id=self.user_id,
            username=self.username,
            roles=list(self.roles),
            geography_scope=GeographyScope(state=self.state, district=self.district),
            classification_level=self.classification,
        )


def _seed_directory() -> Dict[str, DirectoryUser]:
    hashed = hash_password(PROTOTYPE_PASSWORD)
    users = [
        DirectoryUser(
            username="collector.wayanad",
            user_id="usr_collector_01",
            roles=[RoleType.GOVERNMENT_APPROVER],
            password_hash=hashed,
            classification=ClassificationLevel.RESTRICTED,
        ),
        DirectoryUser(
            username="analyst.wayanad",
            user_id="usr_analyst_01",
            roles=[RoleType.GIS_ANALYST],
            password_hash=hashed,
            classification=ClassificationLevel.INTERNAL,
        ),
        DirectoryUser(
            username="auditor.kerala",
            user_id="usr_auditor_01",
            roles=[RoleType.AUDITOR],
            password_hash=hashed,
            district=None,
            classification=ClassificationLevel.INTERNAL,
        ),
        DirectoryUser(
            username="public.viewer",
            user_id="usr_public_01",
            roles=[RoleType.PUBLIC_VIEWER],
            password_hash=hashed,
            district=None,
            classification=ClassificationLevel.PUBLIC,
        ),
    ]
    return {u.username: u for u in users}


USER_DIRECTORY: Dict[str, DirectoryUser] = _seed_directory()


def get_prototype_user(username: str) -> UserContext:
    rec = USER_DIRECTORY.get(username)
    if rec is None:
        raise KeyError(f"Unknown prototype user '{username}'")
    return rec.to_context()


def authenticate_password(username: str, password: str) -> Optional[UserContext]:
    rec = USER_DIRECTORY.get(username)
    if rec is None:
        return None
    offered = hash_password(password)
    if not hmac.compare_digest(offered, rec.password_hash):
        return None
    return rec.to_context()


def issue_access_token(user: UserContext, ttl_seconds: int = TOKEN_TTL_SECONDS) -> str:
    now = int(time.time())
    payload = {
        "sub": user.user_id,
        "username": user.username,
        "roles": [r.value for r in user.roles],
        "state": user.geography_scope.state,
        "district": user.geography_scope.district,
        "classification": user.classification_level.value,
        "iat": now,
        "exp": now + ttl_seconds,
        "nonce": secrets.token_hex(8),
        "iss": "sthira-prototype-directory",
    }
    body = _b64url(json.dumps(payload, separators=(",", ":"), sort_keys=True).encode("utf-8"))
    sig = hmac.new(_auth_secret(), body.encode("ascii"), hashlib.sha256).digest()
    return f"{body}.{_b64url(sig)}"


def verify_access_token(token: str) -> Optional[UserContext]:
    try:
        body, sig = token.split(".", 1)
    except ValueError:
        return None
    expected = hmac.new(_auth_secret(), body.encode("ascii"), hashlib.sha256).digest()
    try:
        offered = _b64url_decode(sig)
    except Exception:
        return None
    if not hmac.compare_digest(expected, offered):
        return None
    try:
        payload = json.loads(_b64url_decode(body))
    except Exception:
        return None
    if int(payload.get("exp", 0)) < int(time.time()):
        return None
    if payload.get("iss") != "sthira-prototype-directory":
        return None
    try:
        roles = [RoleType(r) for r in payload.get("roles", [])]
        classification = ClassificationLevel(payload.get("classification", ClassificationLevel.INTERNAL.value))
    except ValueError:
        return None
    if not payload.get("sub") or not payload.get("username"):
        return None
    return UserContext(
        user_id=payload["sub"],
        username=payload["username"],
        roles=roles or [RoleType.PUBLIC_VIEWER],
        geography_scope=GeographyScope(state=payload.get("state") or "Kerala", district=payload.get("district")),
        classification_level=classification,
    )


def parse_bearer(authorization: Optional[str]) -> Optional[str]:
    if not authorization:
        return None
    scheme, _, remainder = authorization.partition(" ")
    if scheme.lower() != "bearer" or not remainder:
        return None
    return remainder.strip()
