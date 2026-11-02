"""The only place that sends a request: http.client, ssl and sockets."""

import http.client
import ssl
import urllib.parse
from dataclasses import dataclass
from typing import Mapping, Optional

from telemetry.internal.infrastructure.outbound.shared.http_client.error.map_transport_error import (
    REASON_RESPONSE_UNREADABLE,
    map_transport_error,
)

MAX_READ_BYTES = 65536  # the body of the response is read up to 64 KiB and discarded


@dataclass(frozen=True)
class PostResult:
    """What came back: the status, the value of Retry-After and a fixed reason when the exchange failed."""

    status_code: int = 0
    retry_after: str = ""
    failure: str = ""


def _context(skip_tls_verification: bool) -> ssl.SSLContext:
    context = ssl.create_default_context()
    if skip_tls_verification:
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
    return context


def post_request(
    url: str,
    method: str,
    body: bytes,
    headers: Mapping[str, str],
    timeout_seconds: float,
    skip_tls_verification: bool,
) -> PostResult:
    """Send one request and return the result; it never follows a redirect and never raises a transport error.

    ``failure`` holds a fixed reason (see ``map_transport_error``), never the text of the exception. When the status
    arrived but the body could not be read, the status is kept and the failure is ``response_unreadable``.
    """
    connection: Optional[http.client.HTTPConnection] = None
    status_code = 0
    try:
        parts = urllib.parse.urlsplit(url)
        host = parts.hostname or ""
        path = (parts.path or "/") + ("?" + parts.query if parts.query else "")
        if parts.scheme.lower() == "https":
            connection = http.client.HTTPSConnection(
                host, parts.port or 443, timeout=timeout_seconds, context=_context(skip_tls_verification)
            )
        else:
            connection = http.client.HTTPConnection(host, parts.port or 80, timeout=timeout_seconds)
        connection.request(method, path, body=body, headers=dict(headers))
        response = connection.getresponse()
        status_code = response.status
        retry_after = response.getheader("Retry-After") or ""
        try:
            response.read(MAX_READ_BYTES)
        except Exception:  # any read error means the response is unreadable
            return PostResult(status_code, retry_after, REASON_RESPONSE_UNREADABLE)
        return PostResult(status_code, retry_after)
    except Exception as cause:  # every transport failure is translated to a fixed reason
        return PostResult(status_code, "", map_transport_error(cause))
    finally:
        if connection is not None:
            connection.close()
