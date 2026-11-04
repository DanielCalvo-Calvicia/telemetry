"""A fake of the random port."""

import threading
from typing import Optional, Sequence


class FakeRandom:
    """FakeRandom gives the numbers it was told to, in order; the last one repeats, so the delays never change."""

    def __init__(self, values: Sequence[float] = (0.5,)) -> None:
        self._lock = threading.Lock()
        self._values = tuple(values)
        self._failure: Optional[Exception] = None
        self.calls = 0

    def fail_with(self, error: Optional[Exception]) -> None:
        """Make random raise this error; None clears it."""
        with self._lock:
            self._failure = error

    def random(self) -> float:
        """Return the next number, or the last one when they are used up."""
        with self._lock:
            self.calls += 1
            if self._failure is not None:
                raise self._failure
            return self._values[min(self.calls, len(self._values)) - 1]
