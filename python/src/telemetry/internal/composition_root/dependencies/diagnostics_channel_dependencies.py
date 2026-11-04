"""The dependencies of the diagnostics channel adapter."""

from typing import Iterable, Optional

from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.diagnostics_channel_port import (
    DiagnosticsCallback,
    DiagnosticsChannelPort,
    DiagnosticsWriter,
)
from telemetry.internal.infrastructure.outbound.shared.diagnostics_channel.diagnostics_channel import (
    DiagnosticsChannel,
)

_NANOSECONDS_PER_MILLISECOND = 1_000_000


def new_diagnostics_channel(
    callbacks: Iterable[DiagnosticsCallback], writer: Optional[DiagnosticsWriter], clock: ClockPort
) -> DiagnosticsChannelPort:
    """Create the diagnostics channel with the default rate limit, timed by the monotonic reading of the clock."""
    return DiagnosticsChannel(callbacks, writer, lambda: clock.monotonic_nanoseconds() // _NANOSECONDS_PER_MILLISECOND)
