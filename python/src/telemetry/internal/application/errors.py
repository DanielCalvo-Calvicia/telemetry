"""The errors of the application layer."""


class EventBuildFailedError(Exception):
    """The event could not be built: a port failed or something unexpected happened. The cause is chained."""


class EnvelopeMaskingFailedError(Exception):
    """The event could not be masked. The cause is chained; the event is dropped, never sent unmasked (ADR-006)."""


class EnvelopeLimitsFailedError(Exception):
    """The size limits could not be applied. The cause is chained; the event is dropped, never sent unchecked."""


class ConfigurationInstanceIdFailedError(Exception):
    """The instance id could not be generated at start. The cause is chained."""
