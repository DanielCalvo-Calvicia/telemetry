"""The call context: who is running and where, carried through the program with ``contextvars``.

The identity and the correlation of a call (application, instance, execution, correlation, trace and span ids) and
the indentation of the call depth, migrated from the old ``call_depth`` package. It carries only identifiers and the
depth: never a secret, a request body or a payload (REQ-LOG-006).

The only storage is one module-level ``contextvars.ContextVar``. It is not shared mutable state (REQ-ARC-006): each
thread and each asyncio task sees its own value, a task inherits the value of the code that created it, and a change
made inside a task never reaches the others.
"""

import contextlib
import contextvars
import dataclasses
from typing import Iterator, Optional

STARTING_INDENTATION = 1

_CURRENT_CALL_CONTEXT: contextvars.ContextVar[Optional["CallContext"]] = contextvars.ContextVar(
    "telemetry_call_context", default=None
)


@dataclasses.dataclass(frozen=True, slots=True)
class CallContext:
    """The identity, the correlation and the depth of a call; an empty text means the value is not set.

    ``application`` and ``instance_id`` identify the program (kept for the old consumers and for later per-request
    use; the envelope resource comes from the configuration). ``execution_id``, ``correlation_id``, ``trace_id`` and
    ``span_id`` are the correlation of the call. ``indentation`` starts at 1 at the inbound adapter and grows by one
    for each deeper layer. The object is immutable; build it with ``new_call_context`` to have the fields checked.
    """

    application: str = ""
    instance_id: str = ""
    execution_id: str = ""
    correlation_id: str = ""
    trace_id: str = ""
    span_id: str = ""
    indentation: int = STARTING_INDENTATION


def current_call_context() -> Optional[CallContext]:
    """Return the call context of the running thread or task, or None when there is none."""
    return _CURRENT_CALL_CONTEXT.get()


@contextlib.contextmanager
def use_call_context(call_context: Optional[CallContext]) -> Iterator[Optional[CallContext]]:
    """Set ``call_context`` for the block and restore the previous one after it, also when the block raises.

    None is allowed and hides the context of an outer block.
    """
    token = _CURRENT_CALL_CONTEXT.set(call_context)
    try:
        yield call_context
    finally:
        _CURRENT_CALL_CONTEXT.reset(token)


@contextlib.contextmanager
def deeper_indentation() -> Iterator[bool]:
    """Go one layer deeper for the block and restore the previous context after it.

    When there is a call context, a copy with the indentation plus one is set for the block and the block gets True;
    the original object is never modified. When there is none, nothing is done and the block gets False.
    """
    current = _CURRENT_CALL_CONTEXT.get()
    if current is None:
        yield False
        return
    token = _CURRENT_CALL_CONTEXT.set(dataclasses.replace(current, indentation=current.indentation + 1))
    try:
        yield True
    finally:
        _CURRENT_CALL_CONTEXT.reset(token)
