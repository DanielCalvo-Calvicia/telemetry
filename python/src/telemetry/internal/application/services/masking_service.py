"""The service that masks an envelope once, before any sink."""

import dataclasses
from typing import Optional

from telemetry.internal.application.dtos.mask_result import MaskResult
from telemetry.internal.application.errors import EnvelopeMaskingFailedError
from telemetry.internal.domain.operations.masking.sensitive_data_masker import SensitiveDataMasker
from telemetry.internal.domain.value_objects.envelope_generated import Context, Envelope

# The text fields of the context, which are masked by their own (snake case) names.
CONTEXT_TEXT_FIELDS = ("trace_id", "span_id", "execution_id", "correlation_id")


class MaskingService:
    """MaskingService masks an envelope with a SensitiveDataMasker.

    No path may send an envelope to a sink without going through mask (ADR-006); T-03.9 wires the chain.
    """

    def __init__(self, masker: SensitiveDataMasker) -> None:
        """Keep the masker; the composition root builds it from the options of the configuration."""
        self._masker = masker

    def mask(self, envelope: Envelope) -> MaskResult:
        """Return a masked COPY of the envelope, or the error; it never raises (ADR-005).

        Only ``data``, ``attributes`` and ``context`` are masked. The event type, the ids, the times, the severity, the
        schema version, the resource and the actor are never changed. The input is never modified. When anything
        fails the result carries an EnvelopeMaskingFailedError chained from the cause and NO envelope: the caller drops
        the event, it never gets the unmasked one.
        """
        try:
            return MaskResult(envelope=self._mask(envelope))
        except Exception as error:  # the last resort of ADR-005: nothing escapes to the caller of a log call
            failure = EnvelopeMaskingFailedError("the event could not be masked")
            failure.__cause__ = error
            return MaskResult(error=failure)

    def _mask(self, envelope: Envelope) -> Envelope:
        return dataclasses.replace(
            envelope,
            data=self._masker.mask_map(envelope.data),
            attributes=self._masker.mask_map(envelope.attributes),
            context=self._mask_context(envelope.context),
            resource=dataclasses.replace(envelope.resource),
            actor=dataclasses.replace(envelope.actor) if envelope.actor is not None else None,
        )

    def _mask_context(self, context: Optional[Context]) -> Optional[Context]:
        if context is None:
            return None
        masked_fields = {}
        for field_name in CONTEXT_TEXT_FIELDS:
            text = getattr(context, field_name)
            masked_fields[field_name] = self._masker.mask_value(field_name, text) if text is not None else None
        return dataclasses.replace(context, **masked_fields)
