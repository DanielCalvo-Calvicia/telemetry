"""Check the data of an event against the schema of its event type, at the source (REQ-CTR-002, REQ-CTR-003).

The validators of the event types are generated from contracts/schemas by tools/contract_generator
(event_validators_generated.py): plain code over the rules of rules.py, with no JSON Schema library, so the core keeps a
minimal set of dependencies (REQ-ARC-005). The registry of event types is read at generation, never at run time.

Validation never raises and never returns the value it rejected: a value may be a secret. A rejection is returned as a
classified error that names the event type and the field path (ADR-005, REQ-DLV-002).
"""

from collections.abc import Callable
from typing import Any

from telemetry.internal.domain.errors import ClassifiedError, EventSchemaInvalidError, EventTypeUnknownError
from telemetry.internal.domain.operations.event_validation.event_validators_generated import EVENT_VALIDATORS
from telemetry.internal.domain.operations.event_validation.rules import display_text
from telemetry.internal.domain.value_objects.event_types_generated import EVENT_TYPES

ROOT_PATH = "data"


def validate_event(event_type: str, data: Any) -> ClassifiedError | None:
    """Check that the data of an event matches the schema of its event type.

    The data is the generic JSON value (a ``dict``). It returns ``None`` when the data is valid. Otherwise it RETURNS,
    and never raises, a classified error: ``EventTypeUnknownError`` (code ``event_type_unknown``) when the type is not
    registered and ``EventSchemaInvalidError`` (code ``event_schema_invalid``) when the data does not match, with a
    message like ``log.entry.recorded: data.message: required field is missing``. The message names the field path and
    never the value. A last-resort ``try`` turns any unexpected exception into ``event_schema_invalid``.
    """
    return run_safely(event_type, lambda: _validate_event(event_type, data))


def run_safely(event_type: Any, validate: Callable[[], ClassifiedError | None]) -> ClassifiedError | None:
    """Run a validation and turn any exception into the classified error ``event_schema_invalid`` ("validation failed").

    The caller of a log call never sees an exception. ``validate_event`` runs every validation through it.
    """
    try:
        return validate()
    except Exception:  # the last resort of ADR-005: nothing escapes to the caller of a log call
        return EventSchemaInvalidError(f"{_event_type_text(event_type)}: validation failed")


def _event_type_text(event_type: Any) -> str:
    """Return the event type as a name that is safe to show; a value that is not a string is not shown."""
    return display_text(event_type) if isinstance(event_type, str) else "(not a string)"


def _validate_event(event_type: Any, data: Any) -> ClassifiedError | None:
    """Run the validator generated for the event type."""
    if not isinstance(event_type, str) or event_type not in EVENT_TYPES or event_type not in EVENT_VALIDATORS:
        return EventTypeUnknownError(f"the event type is not registered: {_event_type_text(event_type)}")
    problem = EVENT_VALIDATORS[event_type](data, ROOT_PATH)
    if problem is not None:
        return EventSchemaInvalidError(f"{event_type}: {problem.path}: {problem.reason}")
    return None
