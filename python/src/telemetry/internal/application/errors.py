"""The errors of the application layer."""


class EventBuildFailedError(Exception):
    """The event could not be built: a port failed or something unexpected happened. The cause is chained."""
