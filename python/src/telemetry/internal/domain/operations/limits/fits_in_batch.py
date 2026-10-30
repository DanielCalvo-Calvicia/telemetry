"""Whether one more event fits in an HTTP batch."""

from telemetry.internal.domain.value_objects.limits import Limits


def fits_in_batch(batch_bytes: int, event_bytes: int, limits: Limits) -> bool:
    """Return True when the bytes already in the batch plus the bytes of the event do not exceed the batch limit.

    It is pure. The framing bytes of a batch (separators, brackets) belong to the batcher (T-04.4), not to this check.
    """
    return batch_bytes + event_bytes <= limits.max_batch_bytes
