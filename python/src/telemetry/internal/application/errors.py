"""The errors of the application layer."""


class EventBuildFailedError(Exception):
    """The event could not be built: a port failed or something unexpected happened. The cause is chained."""


class EnvelopeMaskingFailedError(Exception):
    """The event could not be masked. The cause is chained; the event is dropped, never sent unmasked (ADR-006)."""
