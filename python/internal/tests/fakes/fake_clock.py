"""A fake of the clock port."""

import datetime
import threading
from typing import Optional

from telemetry.internal.application.dtos.clock_info import ClockInfo
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant

DEFAULT_WALL_TIME = datetime.datetime(2026, 10, 28, 10, 0, 0, tzinfo=datetime.timezone.utc)


class FakeClock:
    """FakeClock has a settable wall time and a monotonic reading that advances by a settable step per read."""

    def __init__(self, wall_time: Optional[datetime.datetime] = None, step_nanoseconds: int = 1000) -> None:
        self._lock = threading.Lock()
        self._wall_time = wall_time if wall_time is not None else DEFAULT_WALL_TIME
        self._step_nanoseconds = step_nanoseconds
        self._monotonic = 0
        self._clock_info = ClockInfo()
        self.now_calls = 0
        self.monotonic_calls = 0
        self.clock_info_calls = 0

    def set_wall_time(self, wall_time: datetime.datetime) -> None:
        """Set the time that now() returns."""
        with self._lock:
            self._wall_time = wall_time

    def set_step(self, step_nanoseconds: int) -> None:
        """Set how far the monotonic reading advances on each read."""
        with self._lock:
            self._step_nanoseconds = step_nanoseconds

    def set_clock_info(self, clock_info: ClockInfo) -> None:
        """Set what clock_info() returns."""
        with self._lock:
            self._clock_info = clock_info

    def now(self) -> UtcInstant:
        """Return the settable wall time."""
        with self._lock:
            self.now_calls += 1
            return UtcInstant(self._wall_time)

    def monotonic_nanoseconds(self) -> int:
        """Advance by the step, then return the reading."""
        with self._lock:
            self.monotonic_calls += 1
            self._monotonic += self._step_nanoseconds
            return self._monotonic

    def clock_info(self) -> ClockInfo:
        """Return the settable clock info."""
        with self._lock:
            self.clock_info_calls += 1
            return self._clock_info
