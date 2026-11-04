"""The diagnostics channel port."""

from typing import Callable, Protocol, runtime_checkable

from telemetry.internal.application.dtos.channel_diagnostics import ChannelDiagnostics
from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent

DiagnosticsCallback = Callable[[DiagnosticEvent], object]


class DiagnosticsWriter(Protocol):
    """DiagnosticsWriter is anything with ``write(text)``, such as ``sys.stderr`` or an open text file."""

    def write(self, text: str, /) -> object:
        """Write the text."""


@runtime_checkable
class DiagnosticsChannelPort(Protocol):
    """DiagnosticsChannelPort is what the application requires to tell the consumer that the delivery had a problem.

    It is NOT a destination: a diagnostic never goes through the pipeline, the queues, the masker or a sink, so a
    failure of telemetry never creates an event that can fail again (REQ-DGN-003). ``publish`` never blocks and never
    raises; the callbacks run on a thread of their own.
    """

    def publish(self, event: DiagnosticEvent) -> None:
        """Offer one diagnostic: the rate limit and the bounded queue may drop it, and the call returns at once."""

    def start(self) -> None:
        """Start the thread that calls the callbacks (nothing starts in a constructor); it does nothing twice."""

    def stop(self) -> None:
        """End the thread without waiting for a callback that hangs; after it ``publish`` does nothing."""

    def counters(self) -> ChannelDiagnostics:
        """Return a copy of the counters of the channel."""
