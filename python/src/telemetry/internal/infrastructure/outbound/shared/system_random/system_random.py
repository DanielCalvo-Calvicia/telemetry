"""The system random source: the RandomPort over the random generator of the standard library."""

from typing import Callable, Optional

from telemetry.internal.infrastructure.outbound.shared.system_random.external import read_random as external


class SystemRandom:
    """SystemRandom implements RandomPort with the generator of the standard library; reading it lives in ``external/``.

    The number is the jitter of a retry delay, never a secret, so a generator that is not cryptographic is the right
    one. The function that reads the generator can be replaced, which tests do. It is safe for concurrent use.
    """

    def __init__(self, read_random: Optional[Callable[[], float]] = None) -> None:
        """Create the source; the function that reads the generator can be replaced."""
        self._read_random = read_random if read_random is not None else external.read_random

    def random(self) -> float:
        """Return a number in [0, 1]; a value outside that range is cut to it, and it never raises."""
        try:
            value = float(self._read_random())
        except Exception:  # a jitter is never worth a failure (ADR-005)
            return 0.5
        if not value > 0:
            return 0.0
        return min(value, 1.0)
