"""Translate a failure of the sources of an identifier to the error of the application."""

from telemetry.internal.application.errors import IdGenerationFailedError


def map_generate_error(cause: BaseException) -> IdGenerationFailedError:
    """Return an IdGenerationFailedError chained from the cause; its message never holds a value of the cause."""
    failure = IdGenerationFailedError("an identifier could not be generated")
    failure.__cause__ = cause
    return failure
