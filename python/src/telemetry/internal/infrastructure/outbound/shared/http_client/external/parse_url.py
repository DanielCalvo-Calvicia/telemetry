"""The parsing of a URL (urllib is an I/O module for the architecture rules, so it lives here)."""

import urllib.parse
from typing import Optional


def parse_url(url: str) -> Optional[tuple[str, str]]:
    """Return ``(scheme, host)`` in lower case, or None when the text is not a URL.

    When a port is present it must be an integer from 1 to 65535.

    The host is empty when the URL has none. A space or a control character anywhere makes the URL invalid.
    """
    if any(ord(character) <= 0x20 or ord(character) == 0x7F for character in url):
        return None
    try:
        parts = urllib.parse.urlsplit(url)
        port = parts.port  # raises ValueError for a port above 65535
    except ValueError:
        return None
    if port is not None and not 1 <= port <= 65535:
        return None
    return parts.scheme.lower(), (parts.hostname or "").lower()
