"""The rule that keeps the reason of a diagnostic a short code (REQ-SEC-007, ADR-005)."""

MAX_REASON_CHARACTERS = 64
UNSPECIFIED_REASON = "unspecified"
_ALLOWED_CHARACTERS = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.:-")


def sanitize_reason(reason: object) -> str:
    """Keep only ``A-Z a-z 0-9 _ . : -``, replace any other character by ``_`` and cut to 64 characters.

    An empty reason (or one that is not text) becomes ``unspecified``. A reason is a code, never a message, a field
    value, a token, a URL or the value of an exception, so this is the last guard before a diagnostic leaves.
    """
    if not isinstance(reason, str) or reason == "":
        return UNSPECIFIED_REASON
    return "".join(
        character if character in _ALLOWED_CHARACTERS else "_" for character in reason[:MAX_REASON_CHARACTERS]
    )
