"""The header that carries the authentication."""

from typing import Optional


def auth_header(bearer_token: str, api_key_header: str, api_key_value: str) -> Optional[tuple[str, str]]:
    """Return ``(name, value)``: ``Authorization: Bearer <token>``, or the API key in its header; None for neither."""
    if bearer_token != "":
        return "Authorization", "Bearer " + bearer_token
    if api_key_header != "" and api_key_value != "":
        return api_key_header, api_key_value
    return None
