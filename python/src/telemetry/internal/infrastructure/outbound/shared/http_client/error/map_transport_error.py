"""Translate a failure of the transport to a fixed reason.

The reason is one of a closed set of texts. The text of the exception is NEVER used: it can hold the URL, and a URL can
carry a secret in its query (REQ-SEC-007). This module imports neither ssl nor socket (they live in ``external/``): it
reads the class of the exception.
"""

REASON_TIMEOUT = "timeout"
REASON_TLS_VERIFICATION_FAILED = "tls_verification_failed"
REASON_NETWORK_ERROR = "network_error"
REASON_RESPONSE_UNREADABLE = "response_unreadable"


def map_transport_error(cause: BaseException) -> str:
    """Return the fixed reason of a failure while connecting, sending or waiting for the response.

    A timeout is ``timeout``; any error of the ``ssl`` module is ``tls_verification_failed``; any other one is
    ``network_error``.
    """
    if isinstance(cause, TimeoutError):
        return REASON_TIMEOUT
    if any(klass.__module__ == "ssl" for klass in type(cause).__mro__):
        return REASON_TLS_VERIFICATION_FAILED
    return REASON_NETWORK_ERROR
