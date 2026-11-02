"""The dependencies of the HTTP destination."""

from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.outbound.logging.http.http_destination import HttpDestination
from telemetry.internal.infrastructure.outbound.logging.http.settings import Settings


def new_http_destination(settings: Settings, limits: Limits = Limits()) -> HttpDestination:
    """Create the HTTP destination over the shared client; invalid settings raise ConfigurationError."""
    return HttpDestination(settings, limits)
