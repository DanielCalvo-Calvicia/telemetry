"""The severity of an event: the generated enum, and the parsing of a text into it."""

from telemetry.internal.domain.errors import SeverityInvalidError
from telemetry.internal.domain.value_objects.enums_generated import Severity


def parse_severity(text: str) -> Severity:
    """Return the Severity whose value is exactly ``text``; raise SeverityInvalidError otherwise (no case folding)."""
    for member in Severity:
        if member.value == text:
            return member
    raise SeverityInvalidError(f"{text!r} is not a severity of the contract")
