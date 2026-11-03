"""The level of the add_log contract for a severity (pure)."""

from telemetry.internal.domain.value_objects.enums_generated import Severity


def map_level(severity: Severity) -> str:
    """Return the severity as the old adapter wrote it: the same name, with CRITICAL written FATAL."""
    return "FATAL" if severity == Severity.CRITICAL else severity.value
