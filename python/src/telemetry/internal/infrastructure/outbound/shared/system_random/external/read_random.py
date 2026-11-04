"""The reading of the random generator of the standard library."""

import random


def read_random() -> float:
    """Return a number from the generator of the standard library; it is in [0.0, 1.0), and safe across threads."""
    return random.random()
