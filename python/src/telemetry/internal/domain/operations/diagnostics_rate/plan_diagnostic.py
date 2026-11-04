"""The rule that limits how many diagnostics are published in a window of time (REQ-DGN-003)."""

from dataclasses import dataclass

NO_WINDOW = -1


@dataclass(frozen=True, slots=True)
class DiagnosticPlan:
    """DiagnosticPlan is the answer of ``plan_diagnostic``: whether to publish and the new state of the limiter.

    ``attach_suppressed`` is how many diagnostics were suppressed before this one (only an allowed call has it above
    zero); ``suppressed`` is the number still waiting to be reported.
    """

    allow: bool
    window_start_ms: int
    count: int
    suppressed: int
    attach_suppressed: int


def plan_diagnostic(
    now_ms: int, window_start_ms: int, count: int, suppressed: int, limit: int, window_ms: int
) -> DiagnosticPlan:
    """Return the answer for one diagnostic at ``now_ms`` and the state that follows (a fixed window).

    ``window_start_ms`` is the start of the current window (``NO_WINDOW``, -1, when there is none), ``count`` the
    diagnostics allowed in it and ``suppressed`` the ones suppressed since the last allowed one. With no window, or
    when ``now_ms - window_start_ms >= window_ms``, a new window starts at ``now_ms`` with count 0 (a clock that goes
    back stays in the window). Then a call with ``count < limit`` is allowed: the count grows by one,
    ``attach_suppressed`` is the number suppressed so far and that number goes back to 0. Otherwise the call is
    suppressed (``allow`` false, ``attach_suppressed`` 0) and the number suppressed grows by one. It is pure: the time
    is an argument, so a fake clock can drive it.
    """
    if window_start_ms == NO_WINDOW or now_ms - window_start_ms >= window_ms:
        window_start_ms = now_ms
        count = 0
    if count < limit:
        return DiagnosticPlan(True, window_start_ms, count + 1, 0, suppressed)
    return DiagnosticPlan(False, window_start_ms, count, suppressed + 1, 0)
