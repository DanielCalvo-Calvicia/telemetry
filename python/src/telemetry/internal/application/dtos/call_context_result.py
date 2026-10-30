"""The result of building a call context."""

from dataclasses import dataclass
from typing import Optional

from telemetry.internal.application.call_context.call_context import CallContext


@dataclass(frozen=True, slots=True)
class CallContextResult:
    """CallContextResult holds the call context or the error, exactly one of them: it never raises (ADR-005)."""

    call_context: Optional[CallContext] = None
    error: Optional[Exception] = None
