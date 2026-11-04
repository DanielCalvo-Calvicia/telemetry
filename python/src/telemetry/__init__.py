"""The telemetry standard of the Snow Factory projects: one contract for Go and Python.

Build the core with ``new(config, *options)``, emit with ``emit`` and close with ``shutdown``. The adapters a project
uses are passed in code with the ``with_*`` options; no environment is read and nothing is global.

    import telemetry

    core = telemetry.new(telemetry.microservice(), telemetry.with_log_sink(my_sink))
    result = core.emit(telemetry.NewEventInput(...))
    core.logger.info(telemetry.LogEntry(category="http", message="request done", fields={"order": 7}))
    core.shutdown()

``core.logger`` is a ``Logger``: one method per severity, each call one ``log.entry.recorded`` event, and it never
raises. ``LoggingHandler`` sends the records of the standard ``logging`` to it. ``core.diagnostics()`` returns a copy
of the counters of every destination, and ``with_diagnostics_callback`` / ``with_diagnostics_writer`` receive the
diagnostics (a failure of the delivery) on a channel of their own, never as an event.
"""

from telemetry.internal.application.call_context.call_context import (
    CallContext,
    current_call_context,
    deeper_indentation,
    use_call_context,
)
from telemetry.internal.application.call_context.new_call_context import new_call_context
from telemetry.internal.application.dtos.channel_diagnostics import ChannelDiagnostics
from telemetry.internal.application.dtos.destination_diagnostics import DestinationDiagnostics
from telemetry.internal.application.dtos.diagnostic_event import DiagnosticEvent
from telemetry.internal.application.dtos.diagnostics_snapshot import DiagnosticsSnapshot
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.flush_outcome import FlushOutcome
from telemetry.internal.application.dtos.log_entry import LogEntry
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.dtos.shutdown_result import ShutdownResult
from telemetry.internal.application.ports.inbound.logger_port import LoggerPort as Logger
from telemetry.internal.application.ports.outbound.ai_telemetry_sink_port import AITelemetrySinkPort
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.device_telemetry_sink_port import DeviceTelemetrySinkPort
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort
from telemetry.internal.application.ports.outbound.local_spool_port import LocalSpoolPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.application.ports.outbound.metrics_sink_port import MetricsSinkPort
from telemetry.internal.application.ports.outbound.trace_sink_port import TraceSinkPort
from telemetry.internal.composition_root.config.options import (
    Option,
    with_ai_telemetry_sink,
    with_clock,
    with_device_telemetry_sink,
    with_diagnostics_callback,
    with_diagnostics_writer,
    with_event_sink,
    with_id_generator,
    with_local_spool,
    with_log_sink,
    with_metrics_sink,
    with_trace_sink,
)
from telemetry.internal.composition_root.containers.core_container import Core as Telemetry
from telemetry.internal.composition_root.containers.core_container import new_core as new
from telemetry.internal.domain.value_objects.actor_identity import ActorIdentity
from telemetry.internal.domain.value_objects.enums_generated import DiagnosticCode, Severity
from telemetry.internal.domain.value_objects.envelope_generated import Envelope
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.config.config import (
    Config,
    DeliveryConfig,
    LimitsConfig,
    MaskingConfig,
    ResourceConfig,
    SpoolConfig,
)
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.config.profiles import cronjob, microservice, offline_device, robotics, worker
from telemetry.internal.infrastructure.inbound.sdk.logging_handler import LoggingHandler

__all__ = [
    "AITelemetrySinkPort",
    "ActorIdentity",
    "CallContext",
    "ChannelDiagnostics",
    "ClockPort",
    "Config",
    "ConfigurationError",
    "DeliveryConfig",
    "DestinationDiagnostics",
    "DeviceTelemetrySinkPort",
    "DiagnosticCode",
    "DiagnosticEvent",
    "DiagnosticsSnapshot",
    "EmitResult",
    "Envelope",
    "EventAttributes",
    "EventSinkPort",
    "FlushOutcome",
    "IDGeneratorPort",
    "Limits",
    "LimitsConfig",
    "LocalSpoolPort",
    "LogEntry",
    "LogSinkPort",
    "Logger",
    "LoggingHandler",
    "MaskingConfig",
    "MetricsSinkPort",
    "NewEventInput",
    "Option",
    "ResourceConfig",
    "SendResult",
    "Severity",
    "ShutdownResult",
    "SpoolConfig",
    "Telemetry",
    "TraceSinkPort",
    "cronjob",
    "current_call_context",
    "deeper_indentation",
    "microservice",
    "new",
    "new_call_context",
    "offline_device",
    "robotics",
    "use_call_context",
    "with_ai_telemetry_sink",
    "with_clock",
    "with_device_telemetry_sink",
    "with_diagnostics_callback",
    "with_diagnostics_writer",
    "with_event_sink",
    "with_id_generator",
    "with_local_spool",
    "with_log_sink",
    "with_metrics_sink",
    "with_trace_sink",
    "worker",
]
