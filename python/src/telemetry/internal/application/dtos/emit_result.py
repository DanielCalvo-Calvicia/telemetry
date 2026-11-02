"""The result of one emit: what became of the event."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class EmitResult:
    """EmitResult says what became of the event of one emit, summed over the destinations that received it.

    ``accepted``, ``dropped`` and ``failed`` add the SendResult of every destination. An event that never reached a
    destination (it was not built, masked or limited, or no destination serves its kind) is ``dropped=1`` with the code
    of the catalog, or ``no_destination``, in ``reason``. ``reason`` is empty when nothing was dropped or failed.
    """

    accepted: int = 0
    dropped: int = 0
    failed: int = 0
    reason: str = ""
