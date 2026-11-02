"""The console destination: events as JSON lines or readable text on the standard output (REQ-LOG-001, REQ-LOG-003).

    import telemetry
    from telemetry.logging import console

    sink = console.new_destination(console.Settings(format="text", minimum_severity="DEBUG"))
    core = telemetry.new(telemetry.microservice(), telemetry.with_log_sink(sink))

The library reads no environment (ADR-004): the consumer loads the values and passes them in. An event below the
minimum severity is not written and counts as accepted. Invalid settings raise ``telemetry.ConfigurationError``.
"""

from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.composition_root.dependencies.console_dependencies import (
    new_console_destination,
    new_console_destination_with_writer,
)
from telemetry.internal.infrastructure.outbound.logging.console.console_destination import Writer
from telemetry.internal.infrastructure.outbound.logging.console.settings import Settings

__all__ = ["Settings", "default_settings", "new_destination", "new_destination_with_writer"]


def default_settings() -> Settings:
    """Return the default settings: the json format and a minimum severity of INFO."""
    return Settings()


def new_destination(settings: Settings) -> LogSinkPort:
    """Create the console destination that writes to the standard output, always as UTF-8.

    It serves both the log capability and the event capability. Invalid settings raise ConfigurationError.
    """
    return new_console_destination(settings)


def new_destination_with_writer(settings: Settings, writer: Writer) -> LogSinkPort:
    """Create the console destination over any object with ``write(text)``, such as an ``io.StringIO``.

    Invalid settings raise ConfigurationError.
    """
    return new_console_destination_with_writer(settings, writer)
