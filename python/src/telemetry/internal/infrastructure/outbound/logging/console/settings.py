"""The settings of the console destination: the Python form of contracts/config/console.schema.json.

The consumer loads these values with its own tool, using the names of ``CONSOLE_ENVIRONMENT_VARIABLES``, and passes
them in. THIS PACKAGE NEVER READS THE ENVIRONMENT OR A FILE (ADR-004). The attribute names are exactly the property
names of the schema, so ``dataclasses.asdict(settings)`` has the shape of the schema.
"""

from dataclasses import dataclass

from telemetry.internal.domain.rules import CONSOLE_FORMATS, DEFAULT_CONSOLE_FORMAT, DEFAULT_CONSOLE_MINIMUM_SEVERITY
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.errors import ConfigProblem


@dataclass(frozen=True)
class Settings:
    """The settings of the console destination: the ``format`` (json or text) and the ``minimum_severity``."""

    format: str = DEFAULT_CONSOLE_FORMAT
    minimum_severity: str = DEFAULT_CONSOLE_MINIMUM_SEVERITY


def _is_one_of(value: object, allowed: tuple[str, ...]) -> bool:
    """Say whether the value is a text equal to one of the allowed ones (no case folding, no other type)."""
    return type(value) is str and value in allowed


def validate_settings(settings: Settings) -> tuple[ConfigProblem, ...]:
    """Return every problem of the settings, in the order of the schema; an empty tuple means they are valid.

    A reason is a fixed text: it never holds the configured value. A value that is not a text is a problem too.
    """
    problems: list[ConfigProblem] = []
    if not _is_one_of(settings.format, CONSOLE_FORMATS):
        problems.append(ConfigProblem("format", ERROR_CODE_CONFIG_FIELD_INVALID, "the format must be json or text"))
    severities = tuple(member.value for member in Severity)
    if not _is_one_of(settings.minimum_severity, severities):
        problems.append(
            ConfigProblem(
                "minimum_severity",
                ERROR_CODE_CONFIG_FIELD_INVALID,
                "the minimum severity must be TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL",
            )
        )
    return tuple(problems)
