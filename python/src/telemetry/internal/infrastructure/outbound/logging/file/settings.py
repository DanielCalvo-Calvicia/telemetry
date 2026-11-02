"""The settings of the file destination: the Python form of contracts/config/file.schema.json.

The consumer loads these values with its own tool, using the names of ``FILE_ENVIRONMENT_VARIABLES``, and passes them
in. THIS PACKAGE NEVER READS THE ENVIRONMENT OR A FILE FOR ITS CONFIGURATION (ADR-004). The attribute names are exactly
the property names of the schema, so ``dataclasses.asdict(settings)`` has the shape of the schema.
"""

from dataclasses import dataclass

from telemetry.internal.domain.rules import (
    DEFAULT_CONSOLE_MINIMUM_SEVERITY,
    DEFAULT_FILE_MAX_FILE_BYTES,
    DEFAULT_FILE_MAX_FILES,
    DEFAULT_FILE_NAME,
    DEFAULT_FILE_QUOTA_BYTES,
)
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.errors import ConfigProblem


@dataclass(frozen=True)
class Settings:
    """The settings of the file destination.

    ``directory`` has no default: the library picks no path. The files are the current one (``file_name``) and, once it
    rotates, ``file_name.1`` (the newest) to ``file_name.(max_files - 1)`` (the oldest).
    """

    directory: str = ""
    file_name: str = DEFAULT_FILE_NAME
    minimum_severity: str = DEFAULT_CONSOLE_MINIMUM_SEVERITY
    max_file_bytes: int = DEFAULT_FILE_MAX_FILE_BYTES
    max_files: int = DEFAULT_FILE_MAX_FILES
    quota_bytes: int = DEFAULT_FILE_QUOTA_BYTES


def _has_control_character(text: str) -> bool:
    """Say whether the text holds a code point below U+0020 or U+007F."""
    return any(character < " " or character == "\x7f" for character in text)


def _is_positive_integer(value: object) -> bool:
    """Say whether the value is an integer (a boolean is not) above zero."""
    return type(value) is int and value > 0


def _directory_is_valid(value: object) -> bool:
    return type(value) is str and value != "" and not _has_control_character(value)


def _file_name_is_valid(value: object) -> bool:
    return (
        type(value) is str
        and value not in ("", ".", "..")
        and "/" not in value
        and "\\" not in value
        and not _has_control_character(value)
    )


def validate_settings(settings: Settings) -> tuple[ConfigProblem, ...]:
    """Return every problem of the settings, in the order of the schema; an empty tuple means they are valid.

    A reason is a fixed text: it never holds a configured value. A value of another type is a problem too. Whether the
    directory can be written is checked when the destination is built, against the real file system.
    """
    problems: list[ConfigProblem] = []

    def add(field: str, reason: str) -> None:
        problems.append(ConfigProblem(field, ERROR_CODE_CONFIG_FIELD_INVALID, reason))

    if not _directory_is_valid(settings.directory):
        add("directory", "the directory must be a non-empty text without control characters")
    if not _file_name_is_valid(settings.file_name):
        add("file_name", "the file name must be a non-empty text without a path separator or control character")
    severities = tuple(member.value for member in Severity)
    if not (type(settings.minimum_severity) is str and settings.minimum_severity in severities):
        add("minimum_severity", "the minimum severity must be TRACE, DEBUG, INFO, WARN, ERROR or CRITICAL")
    max_file_bytes_is_valid = _is_positive_integer(settings.max_file_bytes)
    if not max_file_bytes_is_valid:
        add("max_file_bytes", "the maximum file size must be an integer above zero")
    if not _is_positive_integer(settings.max_files):
        add("max_files", "the number of files must be an integer above zero")
    if not _is_positive_integer(settings.quota_bytes):
        add("quota_bytes", "the quota must be an integer above zero")
    elif max_file_bytes_is_valid and settings.quota_bytes < settings.max_file_bytes:
        add("quota_bytes", "the quota must not be below the maximum file size")
    return tuple(problems)
