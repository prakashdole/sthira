"""
HTTP authentication and degraded-mode mutation guard (REM-001, REM-007, DEC-046).
"""

from starlette.requests import Request
from starlette.responses import JSONResponse

from sthira.core.identity import parse_bearer, verify_access_token
from sthira.modules.live_ops.service import degraded_mode_controller

PUBLIC_EXACT = {
    "/",
    "/health",
    "/openapi.json",
    "/docs",
    "/docs/oauth2-redirect",
    "/redoc",
    "/favicon.ico",
}
PUBLIC_PREFIXES = ("/ui",)
PUBLIC_POST = {"/api/v1/auth/login"}
PUBLIC_GET_PREFIXES = (
    "/api/v1/demo",
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
    if method == "GET" and any(path == p or path.startswith(p + "/") for p in PUBLIC_GET_PREFIXES):
        return True
    return False


async def security_middleware(request: Request, call_next):
    path = request.url.path
    method = request.method.upper()

    if not _is_public(method, path):
        token = parse_bearer(request.headers.get("authorization"))
        user = verify_access_token(token) if token else None
        if user is None:
            return JSONResponse(status_code=401, content={"detail": "Not authenticated"})
        request.state.user = user
    else:
        token = parse_bearer(request.headers.get("authorization"))
        request.state.user = verify_access_token(token) if token else None

    if method in MUTATING and path not in DEGRADED_ALLOWED:
        try:
            degraded_mode_controller.assert_writes_allowed()
        except RuntimeError as exc:
            return JSONResponse(status_code=503, content={"detail": str(exc)})

    return await call_next(request)
