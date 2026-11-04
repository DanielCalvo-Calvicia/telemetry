"""The line the writer of the diagnostics channel receives."""

from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent


def format_line(event: DiagnosticEvent) -> str:
    """Return the line the writer receives: the code, the destination, the reason and the counts, ending in ``\n``."""
    return (
        f"telemetry diagnostics: code={event.code.value} destination={event.kind}/{event.index} "
        f"reason={event.reason} count={event.count} suppressed={event.suppressed}\n"
    )
