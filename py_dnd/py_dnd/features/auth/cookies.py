"""Cookie helpers for auth responses with configurable path/domain/secure settings."""

from fastapi import Response

from py_dnd.core.config import get_settings


def _cookie_kwargs() -> dict:
    """Build shared cookie kwargs from settings."""
    settings = get_settings()
    kwargs: dict = {
        "path": settings.COOKIE_PATH,
        "secure": settings.COOKIE_SECURE,
        "samesite": settings.COOKIE_SAMESITE,
    }
    if settings.COOKIE_DOMAIN:
        kwargs["domain"] = settings.COOKIE_DOMAIN
    return kwargs


def set_auth_cookies(response: Response, access_token: str, refresh_token: str, id_token: str) -> None:
    """Set authentication cookies on the response with proper security settings."""
    base = _cookie_kwargs()
    response.set_cookie(key="access_token", value=access_token, httponly=True, **base)
    response.set_cookie(key="refresh_token", value=refresh_token, httponly=True, **base)
    response.set_cookie(key="id_token", value=id_token, httponly=True, **base)


def clear_auth_cookies(response: Response) -> None:
    """Clear authentication cookies from the response."""
    base = _cookie_kwargs()
    for name in ("access_token", "refresh_token", "id_token"):
        response.delete_cookie(key=name, **base)
