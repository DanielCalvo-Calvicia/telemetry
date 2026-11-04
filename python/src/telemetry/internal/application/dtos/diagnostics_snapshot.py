"""The snapshot of the diagnostics of the core."""

from dataclasses import dataclass, field

from telemetry.internal.application.dtos.channel_diagnostics import ChannelDiagnostics
from telemetry.internal.application.dtos.destination_diagnostics import DestinationDiagnostics


@dataclass(frozen=True, slots=True)
class DiagnosticsSnapshot:
    """DiagnosticsSnapshot is a copy of the diagnostics of every destination and of the channel, at one moment.

    ``destinations`` are in the order of the kinds and of the sinks. Changing the snapshot or what it holds does not
    change the library.
    """

    destinations: tuple[DestinationDiagnostics, ...] = ()
    channel: ChannelDiagnostics = field(default_factory=ChannelDiagnostics)
