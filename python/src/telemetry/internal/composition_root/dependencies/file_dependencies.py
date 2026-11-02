"""The dependencies of the file destination."""

from telemetry.internal.infrastructure.outbound.logging.file.file_destination import FileDestination
from telemetry.internal.infrastructure.outbound.logging.file.settings import Settings


def new_file_destination(settings: Settings) -> FileDestination:
    """Create the file destination; invalid settings or a directory that cannot be written raise ConfigurationError."""
    return FileDestination(settings)
