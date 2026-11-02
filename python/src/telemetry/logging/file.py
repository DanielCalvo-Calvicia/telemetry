"""The file destination: events appended as JSON Lines to a local file, with rotation and a quota (section 7.2).

    import telemetry
    from telemetry.logging import file

    sink = file.new_destination(file.Settings(directory="my-log-directory"))
    core = telemetry.new(telemetry.microservice(), telemetry.with_log_sink(sink))

The library picks no path and reads no environment (ADR-004): the consumer loads the values and passes them in. The
directory is created owner only and must be writable; otherwise ``telemetry.ConfigurationError`` is raised when the
destination is built. The current file rotates to ``name.1`` (the newest) ... ``name.(max_files - 1)`` (the oldest)
when the next line would pass ``max_file_bytes``; when all the files together pass ``quota_bytes`` the oldest rotated
files are deleted first. An event below the minimum severity is not written and counts as accepted. A failed write
fails that event in the result and never the caller.
"""

from telemetry.internal.composition_root.dependencies.file_dependencies import new_file_destination
from telemetry.internal.infrastructure.outbound.logging.file.file_destination import Counters, FileDestination
from telemetry.internal.infrastructure.outbound.logging.file.settings import Settings

Destination = FileDestination
"""The file destination: it serves the log capability and the event capability; ``counters`` has its counters."""

__all__ = ["Counters", "Destination", "Settings", "default_settings", "new_destination"]


def default_settings() -> Settings:
    """Return the default settings: no directory (the consumer must set it), the other fields at their defaults."""
    return Settings()


def new_destination(settings: Settings) -> Destination:
    """Create the file destination, creating the directory and the current file.

    Invalid settings or a directory that cannot be written raise ConfigurationError.
    """
    return new_file_destination(settings)
