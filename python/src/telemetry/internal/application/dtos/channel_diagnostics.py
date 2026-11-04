"""The counters of the diagnostics channel."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class ChannelDiagnostics:
    """ChannelDiagnostics counts what the diagnostics channel did with the diagnostics it was given.

    ``published`` passed the rate limit and were queued for the callbacks, ``suppressed`` were held back by the rate
    limit, ``dropped`` found the queue of the channel full and ``callback_errors`` counts the callbacks (or the writer)
    that raised.
    """

    published: int = 0
    suppressed: int = 0
    dropped: int = 0
    callback_errors: int = 0
