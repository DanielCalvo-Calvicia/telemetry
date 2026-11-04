"""The options of the core: the adapters a project chooses, passed in code (REQ-ARC-013, ADR-013).

The configuration carries only adapter settings, never an adapter name, and an adapter never registers itself on
import: the project imports the adapters it needs and hands them to ``new``. Several destinations of one kind are
allowed (REQ-OUT-003): each ``with_*_sink`` APPENDS. A ``None`` argument is ignored.
"""

from dataclasses import dataclass, field
from typing import Optional, Protocol

from telemetry.internal.application.ports.outbound.ai_telemetry_sink_port import AITelemetrySinkPort
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.device_telemetry_sink_port import DeviceTelemetrySinkPort
from telemetry.internal.application.ports.outbound.diagnostics_channel_port import (
    DiagnosticsCallback,
    DiagnosticsWriter,
)
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort
from telemetry.internal.application.ports.outbound.local_spool_port import LocalSpoolPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.application.ports.outbound.metrics_sink_port import MetricsSinkPort
from telemetry.internal.application.ports.outbound.trace_sink_port import TraceSinkPort


@dataclass
class Options:
    """Options holds what the project passed to ``new``: the sinks by kind, the spool, the clock, the ids and the
    receivers of the diagnostics."""

    log_sinks: list[LogSinkPort] = field(default_factory=list)
    event_sinks: list[EventSinkPort] = field(default_factory=list)
    metrics_sinks: list[MetricsSinkPort] = field(default_factory=list)
    trace_sinks: list[TraceSinkPort] = field(default_factory=list)
    device_telemetry_sinks: list[DeviceTelemetrySinkPort] = field(default_factory=list)
    ai_telemetry_sinks: list[AITelemetrySinkPort] = field(default_factory=list)
    spool: Optional[LocalSpoolPort] = None
    clock: Optional[ClockPort] = None
    id_generator: Optional[IDGeneratorPort] = None
    diagnostics_callbacks: list[DiagnosticsCallback] = field(default_factory=list)
    diagnostics_writer: Optional[DiagnosticsWriter] = None


class Option(Protocol):
    """Option changes the Options while the core is being built; build one with a ``with_*`` function."""

    def __call__(self, options: Options) -> None:
        """Apply the change to the options."""


def with_log_sink(sink: Optional[LogSinkPort]) -> Option:
    """Add a destination for the log events; call it again to add more. None is ignored."""

    def apply(options: Options) -> None:
        if sink is not None:
            options.log_sinks.append(sink)

    return apply


def with_event_sink(sink: Optional[EventSinkPort]) -> Option:
    """Add a destination for the events that are not logs, metrics, traces, device or AI telemetry. None is ignored."""

    def apply(options: Options) -> None:
        if sink is not None:
            options.event_sinks.append(sink)

    return apply


def with_metrics_sink(sink: Optional[MetricsSinkPort]) -> Option:
    """Add a destination for the metric events; call it again to add more. None is ignored."""

    def apply(options: Options) -> None:
        if sink is not None:
            options.metrics_sinks.append(sink)

    return apply


def with_trace_sink(sink: Optional[TraceSinkPort]) -> Option:
    """Add a destination for the trace events; call it again to add more. None is ignored."""

    def apply(options: Options) -> None:
        if sink is not None:
            options.trace_sinks.append(sink)

    return apply


def with_device_telemetry_sink(sink: Optional[DeviceTelemetrySinkPort]) -> Option:
    """Add a destination for the device telemetry events; call it again to add more. None is ignored."""

    def apply(options: Options) -> None:
        if sink is not None:
            options.device_telemetry_sinks.append(sink)

    return apply


def with_ai_telemetry_sink(sink: Optional[AITelemetrySinkPort]) -> Option:
    """Add a destination for the AI telemetry events; call it again to add more. None is ignored."""

    def apply(options: Options) -> None:
        if sink is not None:
            options.ai_telemetry_sinks.append(sink)

    return apply


def with_local_spool(spool: Optional[LocalSpoolPort]) -> Option:
    """Set the local spool the durable delivery mode needs; the last one given wins. None is ignored."""

    def apply(options: Options) -> None:
        if spool is not None:
            options.spool = spool

    return apply


def with_clock(clock: Optional[ClockPort]) -> Option:
    """Replace the system clock, for example with a fake in a test; the last one given wins. None is ignored."""

    def apply(options: Options) -> None:
        if clock is not None:
            options.clock = clock

    return apply


def with_id_generator(id_generator: Optional[IDGeneratorPort]) -> Option:
    """Replace the UUIDv7 generator, for example with a fake in a test; the last one given wins. None is ignored."""

    def apply(options: Options) -> None:
        if id_generator is not None:
            options.id_generator = id_generator

    return apply


def with_diagnostics_callback(callback: Optional[DiagnosticsCallback]) -> Option:
    """Add a function that receives every ``DiagnosticEvent``; call it again to add more. None is ignored.

    The callbacks run in the order they were given, on a thread of the library and never in the send path; one that
    raises is contained and counted, and one that never returns only stops the delivery of diagnostics.
    """

    def apply(options: Options) -> None:
        if callback is not None:
            options.diagnostics_callbacks.append(callback)

    return apply


def with_diagnostics_writer(writer: Optional[DiagnosticsWriter]) -> Option:
    """Set an object with ``write(text)`` that receives one line per diagnostic; the last one wins. None is ignored.

    The line is ``telemetry diagnostics: code=<code> destination=<kind>/<index> reason=<reason> count=<n>
    suppressed=<n>`` and a line feed.
    """

    def apply(options: Options) -> None:
        if writer is not None:
            options.diagnostics_writer = writer

    return apply
