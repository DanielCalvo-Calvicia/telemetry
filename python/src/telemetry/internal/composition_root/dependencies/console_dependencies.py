"""The dependencies of the console destination."""

from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.infrastructure.outbound.logging.console.console_destination import ConsoleDestination, Writer
from telemetry.internal.infrastructure.outbound.logging.console.external.standard_output import Utf8StandardOutput
from telemetry.internal.infrastructure.outbound.logging.console.settings import Settings


def new_console_destination(settings: Settings) -> LogSinkPort:
    """Create the console destination that writes to the standard output; invalid settings raise ConfigurationError."""
    return ConsoleDestination(settings, Utf8StandardOutput())


def new_console_destination_with_writer(settings: Settings, writer: Writer) -> LogSinkPort:
    """Create the console destination over any object with ``write(text)``; invalid settings raise an error."""
    return ConsoleDestination(settings, writer)
