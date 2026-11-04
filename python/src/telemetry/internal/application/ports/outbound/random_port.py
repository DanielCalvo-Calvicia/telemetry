"""The random port."""

from typing import Protocol, runtime_checkable


@runtime_checkable
class RandomPort(Protocol):
    """RandomPort is the random number the core needs: the jitter of the wait between two attempts of a retry.

    It is a port so a test can give a fixed number and the delays are the same in every run.
    """

    def random(self) -> float:
        """Return a number in [0, 1]; it is safe to call from many threads and it never raises."""
