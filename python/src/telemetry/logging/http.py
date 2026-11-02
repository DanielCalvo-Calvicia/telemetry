"""The HTTP destination: batches of events sent to the URL the consumer configures (REQ-OUT-002, REQ-OUT-006).

    import telemetry
    from telemetry.logging import http

    sink = http.new_destination(http.Settings(url="https://logs.example.invalid/ingest", bearer_token=token))
    core = telemetry.new(telemetry.microservice(), telemetry.with_log_sink(sink))

The URL is required: without it ``telemetry.ConfigurationError`` is raised when the destination is built, and it names
the field. The library reads no environment (ADR-004): the consumer loads the values and passes them in. The
destination sends each call as ONE batch request, does not queue and does not retry on its own; a failure is in the
``SendResult`` and never an exception. An event below the minimum severity is not sent and counts as accepted.
"""

from telemetry.internal.composition_root.dependencies.http_destination_dependencies import new_http_destination
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.outbound.logging.http.http_destination import Delivery, HttpDestination
from telemetry.internal.infrastructure.outbound.logging.http.settings import Settings

Destination = HttpDestination
"""The HTTP destination: it serves the log capability and the event capability; ``deliver`` also gives the outcome."""

__all__ = ["Delivery", "Destination", "Settings", "default_settings", "new_destination", "new_destination_with_limits"]


def default_settings() -> Settings:
    """Return the default settings: no URL (the consumer must set it), the other fields at their defaults."""
    return Settings()


def new_destination(settings: Settings) -> Destination:
    """Create the HTTP destination with the default size limits; invalid settings raise ConfigurationError."""
    return new_http_destination(settings)


def new_destination_with_limits(settings: Settings, limits: Limits) -> Destination:
    """Create the HTTP destination with the size limits of the configuration; invalid settings raise an error."""
    return new_http_destination(settings, limits)
