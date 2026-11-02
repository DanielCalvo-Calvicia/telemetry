"""The service that runs one input through the whole chain and delivers the event to the destinations."""

import dataclasses
from typing import Optional

from telemetry.internal.application.call_context.call_context import current_call_context
from telemetry.internal.application.call_context.correlation import to_correlation_context
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.ports.outbound.ai_telemetry_sink_port import AITelemetrySinkPort
from telemetry.internal.application.ports.outbound.device_telemetry_sink_port import DeviceTelemetrySinkPort
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.application.ports.outbound.metrics_sink_port import MetricsSinkPort
from telemetry.internal.application.ports.outbound.trace_sink_port import TraceSinkPort
from telemetry.internal.application.services.destination_kind import destination_kind_of
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.domain.errors import error_code_of
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_EVENT_BUILD_FAILED

REASON_NO_DESTINATION = "no_destination"
REASON_SINK_RAISED = "sink_panicked"


@dataclasses.dataclass(frozen=True)
class Destinations:
    """Destinations holds the sinks of each kind, in the order they were given; any kind may have several."""

    log: tuple[LogSinkPort, ...] = ()
    event: tuple[EventSinkPort, ...] = ()
    metrics: tuple[MetricsSinkPort, ...] = ()
    trace: tuple[TraceSinkPort, ...] = ()
    device_telemetry: tuple[DeviceTelemetrySinkPort, ...] = ()
    ai_telemetry: tuple[AITelemetrySinkPort, ...] = ()


class EventPipelineService:
    """EventPipelineService turns one NewEventInput into one masked, limited, valid event delivered to the sinks.

    Delivery is SYNCHRONOUS for now: the thread of the caller calls every sink. The bounded queue and the delivery
    modes arrive with T-05; until then a slow sink slows the caller. It holds no mutable state, so one instance serves
    many threads.
    """

    def __init__(
        self,
        builder: EventBuilderService,
        masking: MaskingService,
        limits: LimitsService,
        destinations: Destinations,
    ) -> None:
        """Keep the three services of the chain and the destinations; the composition root builds them."""
        self._builder = builder
        self._masking = masking
        self._limits = limits
        self._destinations = destinations

    def emit(self, new_event: NewEventInput) -> EmitResult:
        """Run the input through the chain and return what became of it; it NEVER raises (ADR-005, REQ-DLV-002).

        The steps, in this order:

        1. When the input has no context and the calling thread has a call context, the correlation of that call
           context is used as the context of the event (a copy of the input is made; a call context that does not map
           is ignored).
        2. Build the envelope (EventBuilderService).
        3. Mask it (MaskingService), BEFORE any sink: no code path sends an envelope that was not masked (ADR-006).
        4. Apply the size limits (LimitsService).
        5. Route it by the first segment of the event type (see destination_kind_of). No destination of that kind
           gives ``dropped=1`` with the reason ``no_destination``.
        6. Call ``send([envelope])`` on EVERY destination of the kind, in the order given, each call guarded on its
           own: a sink that raises counts ``failed=1`` with the reason ``sink_panicked`` and never stops the others.

        A failure of step 2, 3 or 4 gives ``dropped=1`` and the code of the error as the reason. Any unexpected
        exception gives ``dropped=1`` and the reason ``event_build_failed``. The result adds the SendResult of every
        destination; its reason is the first that is not empty.
        """
        try:
            return self._emit(new_event)
        except Exception:  # the last resort of ADR-005: nothing escapes to the caller of a log call
            return EmitResult(dropped=1, reason=ERROR_CODE_EVENT_BUILD_FAILED)

    def _emit(self, new_event: NewEventInput) -> EmitResult:
        new_event = self._with_call_context(new_event)
        built = self._builder.build(new_event)
        if built.error is not None or built.envelope is None:
            return self._dropped(built.error)
        masked = self._masking.mask(built.envelope)
        if masked.error is not None or masked.envelope is None:
            return self._dropped(masked.error)
        limited = self._limits.apply(masked.envelope)
        if limited.error is not None or limited.envelope is None:
            return self._dropped(limited.error)
        envelope = limited.envelope
        sinks = getattr(self._destinations, destination_kind_of(new_event.event_type))
        if not sinks:
            return EmitResult(dropped=1, reason=REASON_NO_DESTINATION)
        accepted = dropped = failed = 0
        reason = ""
        for sink in sinks:
            try:
                sent = sink.send([envelope])
                sent_counts = (sent.accepted, sent.dropped, sent.failed)
                sent_reason = sent.reason
            except Exception:  # a sink never fails the caller and never stops the others (ADR-005)
                sent_counts = (0, 0, 1)
                sent_reason = REASON_SINK_RAISED
            accepted += sent_counts[0]
            dropped += sent_counts[1]
            failed += sent_counts[2]
            if reason == "" and sent_reason:
                reason = sent_reason
        return EmitResult(accepted=accepted, dropped=dropped, failed=failed, reason=reason)

    @staticmethod
    def _with_call_context(new_event: NewEventInput) -> NewEventInput:
        if new_event.context is not None:
            return new_event
        call_context = current_call_context()
        if call_context is None:
            return new_event
        context, _ = to_correlation_context(call_context)
        if context is None:
            return new_event
        return dataclasses.replace(new_event, context=context)

    @staticmethod
    def _dropped(error: Optional[Exception]) -> EmitResult:
        code = error_code_of(error) if error is not None else ""
        return EmitResult(dropped=1, reason=code or ERROR_CODE_EVENT_BUILD_FAILED)
