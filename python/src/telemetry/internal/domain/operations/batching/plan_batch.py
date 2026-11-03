"""The rule that decides when a batch leaves the queue (REQ-DLV-006)."""

from dataclasses import dataclass

PLAN_IDLE = "idle"
PLAN_SEND = "send"
PLAN_WAIT = "wait"

WAIT_FOR_EVER = -1


@dataclass(frozen=True, slots=True)
class Plan:
    """Plan is what the worker does next: ``idle`` (nothing queued), ``send`` ``take`` events or ``wait`` ``wait_ms``.

    ``take`` is only meaningful for ``send`` and ``wait_ms`` for ``wait``; ``send`` has ``wait_ms`` 0 and ``idle`` has
    ``wait_ms`` -1, which means wait for ever, until an event arrives.
    """

    action: str
    take: int = 0
    wait_ms: int = WAIT_FOR_EVER


def plan_batch(now_ms: int, queued: int, oldest_arrival_ms: int, batch_size: int, flush_interval_ms: int) -> Plan:
    """Return what to do with the queue: a batch leaves when it holds batch_size events or the oldest is old enough.

    No event queued gives ``idle``. At least ``batch_size`` queued gives ``send`` of ``batch_size``. Otherwise, when
    the oldest event has waited ``flush_interval_ms`` or more, ``send`` of ``min(queued, batch_size)``; when it has
    not, ``wait`` for what is left of the interval. It is pure: the time is an argument, so a fake clock can drive it.
    """
    if queued <= 0:
        return Plan(PLAN_IDLE)
    if queued >= batch_size:
        return Plan(PLAN_SEND, take=batch_size, wait_ms=0)
    if now_ms - oldest_arrival_ms >= flush_interval_ms:
        return Plan(PLAN_SEND, take=min(queued, batch_size), wait_ms=0)
    return Plan(PLAN_WAIT, wait_ms=oldest_arrival_ms + flush_interval_ms - now_ms)
