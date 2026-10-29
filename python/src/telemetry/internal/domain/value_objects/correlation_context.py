"""The correlation context of an event: the trace and the execution it belongs to."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.errors import ContextFieldInvalidError
from telemetry.internal.domain.value_objects.field_problems import FieldProblems

TRACE_ID_WIDTH = 32
SPAN_ID_WIDTH = 16


@dataclass(frozen=True, slots=True)
class CorrelationContext:
    """Every field is optional (``None`` is absent) and the empty context is valid.

    trace_id is 32 and span_id is 16 lowercase hex digits (W3C); execution_id and correlation_id are identifiers. A
    field that is present but empty is invalid; every failed field is reported in one ContextFieldInvalidError.
    """

    trace_id: Optional[str] = None
    span_id: Optional[str] = None
    execution_id: Optional[str] = None
    correlation_id: Optional[str] = None

    def __post_init__(self) -> None:
        """Validate every field and report all the failures together."""
        problems = FieldProblems()
        problems.hex_digits("trace_id", self.trace_id, TRACE_ID_WIDTH)
        problems.hex_digits("span_id", self.span_id, SPAN_ID_WIDTH)
        problems.identifier("execution_id", self.execution_id, required=False)
        problems.identifier("correlation_id", self.correlation_id, required=False)
        problems.raise_if_any("the context", ContextFieldInvalidError, ContextFieldInvalidError)
