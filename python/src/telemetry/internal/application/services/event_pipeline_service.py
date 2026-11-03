"""The service that runs one input through the whole chain and queues the event for the destinations."""

import dataclasses
from typing import Optional

from telemetry.internal.application.call_context.call_context import current_call_context
from telemetry.internal.application.call_context.correlation import to_correlation_context
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.services.delivery_service import DeliveryService
from telemetry.internal.application.services.destination_kind import destination_kind_of
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.domain.errors import error_code_of
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_EVENT_BUILD_FAILED


class EventPipelineService:
    """EventPipelineService turns one NewEventInput into one masked, limited, valid event delivered to the sinks.

    Delivery is ASYNCHRONOUS (REQ-DLV-002): the thread of the caller only ENQUEUES the event in the DeliveryService and
    returns, and the workers of the destinations send it later in batches, so a slow sink never slows the caller. It
    holds no mutable state of its own, so one instance serves many threads.
    """

    def __init__(
        self,
        builder: EventBuilderService,
        masking: MaskingService,
        limits: LimitsService,
        delivery: DeliveryService,
    ) -> None:
        """Keep the three services of the chain and the delivery service; the composition root builds them."""
        self._builder = builder
        self._masking = masking
        self._limits = limits
        self._delivery = delivery

    def emit(self, new_event: NewEventInput) -> EmitResult:
        """Run the input through the chain and return what became of it; it NEVER raises (ADR-005, REQ-DLV-002).

        The steps, in this order:

        1. When the input has no context and the calling thread has a call context, the correlation of that call
           context is used as the context of the event (a copy of the input is made; a call context that does not map
           is ignored).
        2. Build the envelope (EventBuilderService).
        3. Mask it (MaskingService), BEFORE any sink: no code path sends an envelope that was not masked (ADR-006).
        4. Apply the size limits (LimitsService).
        5. Route it by the first segment of the event type (see destination_kind_of) and hand it to
           ``DeliveryService.enqueue``, which puts it in the queue of EVERY destination of that kind. No destination of
           that kind gives ``dropped=1`` with the reason ``no_destination``; a full queue gives ``dropped=1`` with the
           reason of its policy (``queue_full_newest`` by default, see ``DeliveryService.enqueue``). The sinks are
           called later, by their workers, never by this call. With the policy ``block_with_timeout`` a full queue
           makes this call WAIT for room, at most the configured timeout: the one case in which a call can wait.

        After ``shutdown`` none of these steps runs: the result is ``dropped=1`` with the reason ``shutdown``, and the
        drop is counted in the stats of the destinations of the kind.

        A failure of step 2, 3 or 4 gives ``dropped=1`` and the code of the error as the reason. Any unexpected
        exception gives ``dropped=1`` and the reason ``event_build_failed``. The result adds what became of the event in
        every destination: ``accepted`` means QUEUED, not delivered, and the reason is the first that is not empty. What
        a sink does with the event afterwards (a failure, a drop) is a counter of the delivery (T-05.5), not part of
        the result of this call.
        """
        try:
            if self._delivery.closed:  # after shutdown nothing is built or queued: the event is dropped and counted
                return self._delivery.reject_closed(destination_kind_of(new_event.event_type))
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
        return self._delivery.enqueue(destination_kind_of(new_event.event_type), limited.envelope)

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
