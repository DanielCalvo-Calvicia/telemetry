"""The system clock: the ClockPort over the clock of the machine."""

import datetime
from typing import Callable, Optional

from telemetry.internal.application.dtos.clock_info import ClockInfo
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant
from telemetry.internal.infrastructure.outbound.shared.system_clock.external import read_time as external


class SystemClock:
    """SystemClock implements ClockPort with the clock of the machine; reading it lives in ``external/``.

    ``now`` is the wall time in UTC (microsecond precision). ``monotonic_nanoseconds`` counts from the creation of the
    clock and never goes back. ``clock_info`` is EMPTY on purpose: the system clock knows neither its source nor whether
    it is synchronised, so no ``clock_source`` or ``clock_synced`` attribute is written to an event.
    """

    def __init__(
        self,
        read_time: Optional[Callable[[], datetime.datetime]] = None,
        read_monotonic: Optional[Callable[[], int]] = None,
    ) -> None:
        """Create the clock; the functions that read the machine can be replaced, which tests do."""
        self._read_time = read_time if read_time is not None else external.read_time
        self._read_monotonic = read_monotonic if read_monotonic is not None else external.read_monotonic
        self._start_nanoseconds = self._read_monotonic()

    def now(self) -> UtcInstant:
        """Return the wall time of the machine as an instant in UTC."""
        return UtcInstant(self._read_time())

    def monotonic_nanoseconds(self) -> int:
        """Return the nanoseconds since the clock was created, from the monotonic clock of the machine."""
        return self._read_monotonic() - self._start_nanoseconds

    def clock_info(self) -> ClockInfo:
        """Return an empty ClockInfo: the system clock does not know its source or whether it is synchronised."""
        return ClockInfo()
