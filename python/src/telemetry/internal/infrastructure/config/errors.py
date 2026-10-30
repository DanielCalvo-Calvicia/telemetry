"""The errors of the configuration: every problem names its field and a code, never a configured value."""

from dataclasses import dataclass

from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID


@dataclass(frozen=True)
class ConfigProblem:
    """One problem: the dotted path of the field, the code of the catalog and a fixed reason."""

    field: str
    code: str
    reason: str


class ConfigurationError(ClassifiedError):
    """The configuration is not valid: ``problems`` holds all of them, and ``code`` is the one of the first."""

    def __init__(self, problems: tuple[ConfigProblem, ...]) -> None:
        """Keep the problems; the message has one line per problem, ``field: reason (code)``."""
        message = "\n".join(f"{problem.field}: {problem.reason} ({problem.code})" for problem in problems)
        code = problems[0].code if problems else ERROR_CODE_CONFIG_FIELD_INVALID
        super().__init__(message, code)
        self.problems = tuple(problems)
