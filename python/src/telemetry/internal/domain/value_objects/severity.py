"""The severity of an event: the generated enum, and the parsing of a text into it."""

from telemetry.internal.domain.errors import SeverityInvalidError
from telemetry.internal.domain.value_objects.enums_generated import Severity


def parse_severity(text: str) -> Severity:
    """Return the Severity whose value is exactly ``text``; raise SeverityInvalidError otherwise (no case folding)."""
    for member in Severity:
        if member.value == text:
            return member
    raise SeverityInvalidError("the severity is not one of the values of the contract")


def severity_rank(severity: Severity) -> int:
    """Return the rank of a severity: TRACE is 0 and CRITICAL is 5, the order of the members of the contract.

    The rank is the position of the member in the generated enum, so it never repeats the values of the contract. An
    unknown value ranks -1.
    """
    for rank, member in enumerate(Severity):
        if member == severity:
            return rank
    return -1


def severity_at_least(severity: Severity, minimum: Severity) -> bool:
    """Say whether ``severity`` is as high as ``minimum`` or higher; false when either one is unknown."""
    rank = severity_rank(severity)
    minimum_rank = severity_rank(minimum)
    return rank >= 0 and minimum_rank >= 0 and rank >= minimum_rank
