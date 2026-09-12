"""
HTTP authentication and degraded-mode mutation guard (REM-001, REM-007, DEC-046).
"""

from starlette.requests import Request
from starlette.responses import JSONResponse

from sthira.core.identity import parse_bearer, verify_access_token
from sthira.modules.live_ops.service import degraded_mode_controller
from sthira_v2.security import MAX_AUDIO_BYTES, MAX_REQUEST_BYTES, new_request_id, validate_public_identifier

PUBLIC_EXACT = {
    "/",
    "/health",
    "/openapi.json",
    "/docs",
    "/docs/oauth2-redirect",
    "/redoc",
    "/favicon.ico",
}
PUBLIC_PREFIXES = ("/ui", "/v2")
PUBLIC_POST = {"/api/v1/auth/login"}
PUBLIC_POST_PREFIXES = ("/api/v2/voice/",)
PUBLIC_GET_PREFIXES = (
    "/api/v1/demo",
    "/api/v2/status",
    "/api/v2/readiness",
    "/api/v2/alerts",
    "/api/v2/demo",
    "/api/v2/operational-packages",
    "/api/v1/localization",
    "/api/v1/reporting/public-projection",
    "/api/v1/reporting/public-transparency-projection",
)
DEGRADED_ALLOWED = {"/api/v1/recovery/degraded-mode", "/api/v1/auth/login"}
MUTATING = {"POST", "PUT", "PATCH", "DELETE"}


def _is_public(method: str, path: str) -> bool:
    if path in PUBLIC_EXACT:
        return True
    if any(path.startswith(p) for p in PUBLIC_PREFIXES):
        return True
    if method == "OPTIONS":
        return True
    if method == "POST" and path in PUBLIC_POST:
        return True
    if method == "POST" and any(path.startswith(prefix) for prefix in PUBLIC_POST_PREFIXES):
        return True
    if method == "POST" and (path == "/api/v2/assignments" or path.startswith("/api/v2/assignments/")):
        return True
    if method == "GET" and path.startswith("/api/v2/assignments/"):
        return True
    if method == "GET" and any(path == p or path.startswith(p + "/") for p in PUBLIC_GET_PREFIXES):
        return True
    return False


async def security_middleware(request: Request, call_next):
    path = request.url.path
    method = request.method.upper()

    incoming_request_id = request.headers.get("x-request-id")
    try:
        request_id = validate_public_identifier(incoming_request_id) if incoming_request_id else new_request_id()
    except ValueError:
        request_id = new_request_id()
    request.state.request_id = request_id

    content_length = request.headers.get("content-length")
    if content_length:
        try:
            limit = (MAX_AUDIO_BYTES * 4 // 3) + 1024 if path == "/api/v2/voice/transcriptions" else MAX_REQUEST_BYTES
            if int(content_length) > limit:
                response = JSONResponse(status_code=413, content={"detail": "request body exceeds limit"})
                response.headers["X-Request-ID"] = request_id
                return response
        except ValueError:
            response = JSONResponse(status_code=400, content={"detail": "invalid content-length"})
            response.headers["X-Request-ID"] = request_id
            return response

    if not _is_public(method, path):
        token = parse_bearer(request.headers.get("authorization"))
        user = verify_access_token(token) if token else None
        if user is None:
            response = JSONResponse(status_code=401, content={"detail": "Not authenticated"})
            response.headers["X-Request-ID"] = request_id
            return response
        request.state.user = user
    else:
        token = parse_bearer(request.headers.get("authorization"))
        request.state.user = verify_access_token(token) if token else None

    if method in MUTATING and path not in DEGRADED_ALLOWED:
        try:
            degraded_mode_controller.assert_writes_allowed()
        except RuntimeError as exc:
            response = JSONResponse(status_code=503, content={"detail": str(exc)})
            response.headers["X-Request-ID"] = request_id
            return response

    response = await call_next(request)
    response.headers["X-Request-ID"] = request_id
    response.headers.setdefault("X-Content-Type-Options", "nosniff")
    response.headers.setdefault("Referrer-Policy", "no-referrer")
    response.headers.setdefault("X-Frame-Options", "DENY")
    return response
