"""The class of an HTTP status (the matrix of section 10.6)."""

CLASS_SUCCESS = "success"
CLASS_TRANSIENT = "transient"
CLASS_PERMANENT = "permanent"


def classify_status(status: int) -> str:
    """Return the class of a status: 2xx success; 408, 429 and 5xx transient; any other 3xx or 4xx permanent.

    A redirect is never followed, so it is permanent. A status that is not a valid final HTTP status (below 200,
    600 or more, zero, negative) is transient: it is retried with a limit.
    """
    if 200 <= status <= 299:
        return CLASS_SUCCESS
    if status in (408, 429) or 500 <= status <= 599:
        return CLASS_TRANSIENT
    if 300 <= status <= 499:
        return CLASS_PERMANENT
    return CLASS_TRANSIENT
