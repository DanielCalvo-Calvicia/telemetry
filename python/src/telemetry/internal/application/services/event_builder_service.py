"""The service that turns what a capability gives the core into a complete, valid envelope."""

from typing import Any, Callable, Mapping, Optional

from telemetry.internal.application.dtos.build_result import BuildResult
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.errors import EventBuildFailedError
from telemetry.internal.application.mappers import envelope_mapper
from telemetry.internal.application.ports.outbound.clock_port import ClockPort
from telemetry.internal.application.ports.outbound.configuration_port import ConfigurationPort
from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort
from telemetry.internal.domain.errors import EventSchemaInvalidError, InstantFormatInvalidError
from telemetry.internal.domain.rules import SCHEMA_VERSION
from telemetry.internal.domain.value_objects.utc_instant import UtcInstant

EventDataValidator = Callable[[str, Any], Optional[Exception]]

OBSERVED_AT_KEY = "observed_at"
CLOCK_SOURCE_KEY = "clock_source"
CLOCK_SYNCED_KEY = "clock_synced"
OBSERVED_AT_MESSAGE = "data.observed_at: must be a UTC time in the format YYYY-MM-DDTHH:MM:SS.ffffffZ"


class EventBuilderService:
    """EventBuilderService builds the envelope of an event from a NewEventInput and the ports.

    The envelope it returns is NOT masked and NOT limited yet: masking (T-03.4) and limits (T-03.5) are plugged in
    later and T-03.9 wires the chain. Nothing may send this envelope to a sink before masking (ADR-006).
    """

    def __init__(
        self,
        clock: ClockPort,
        id_generator: IDGeneratorPort,
        configuration: ConfigurationPort,
        validate_data: EventDataValidator,
    ) -> None:
        """Keep the ports and the validator of the data; the composition root passes validate_event."""
        self._clock = clock
        self._id_generator = id_generator
        self._configuration = configuration
        self._validate_data = validate_data

    def build(self, new_event: NewEventInput) -> BuildResult:
        """Build the envelope, or return the error; it never raises (ADR-005, REQ-DLV-002).

        The steps, in this order:

        1. Read the clock once. ``recorded_at`` is always that reading (REQ-CTR-008, ADR-012). ``occurred_at`` is the
           one given, otherwise that reading. When ``data`` has ``observed_at`` it must be text in the contract time
           format; ``occurred_at`` then equals it, over a given one. Any other form is an event_schema_invalid result
           that names ``data.observed_at`` and never its value.
        2. Take a new event id from the id generator.
        3. Take the resource from the configuration. A failure of step 2 or 3 is an EventBuildFailedError.
        4. Ask the clock for its info. ``clock_source`` and ``clock_synced`` are added to a copy of the attributes of
           the caller only when known (None is absent, never false); on a clash the clock wins.
        5. Assemble the envelope (see the mapper for the null-versus-empty rules).
        6. Validate the built data against the schema of its event type; a classified error is returned as it is.

        Any other failure, of a port or unexpected, is an EventBuildFailedError chained from its cause.
        """
        try:
            return self._build(new_event)
        except Exception as error:  # the last resort of ADR-005: nothing escapes to the caller of a log call
            failure = EventBuildFailedError("the event could not be built")
            failure.__cause__ = error
            return BuildResult(error=failure)

    def _build(self, new_event: NewEventInput) -> BuildResult:
        now = self._clock.now()
        raw_data = new_event.data
        if raw_data is not None and not isinstance(raw_data, Mapping):
            return BuildResult(error=EventSchemaInvalidError("data: must be an object"))
        data = dict(raw_data) if raw_data is not None else {}
        occurred_at = new_event.occurred_at if new_event.occurred_at is not None else now
        if OBSERVED_AT_KEY in data:
            observed_at = data[OBSERVED_AT_KEY]
            try:
                if not isinstance(observed_at, str):
                    raise InstantFormatInvalidError("not text")
                occurred_at = UtcInstant.parse(observed_at)
            except InstantFormatInvalidError:
                return BuildResult(error=EventSchemaInvalidError(OBSERVED_AT_MESSAGE))

        event_id = self._id_generator.new_event_id()
        resource = self._configuration.configuration().resource

        clock_info = self._clock.clock_info()
        attributes: dict[str, Any] = new_event.attributes.as_dict() if new_event.attributes is not None else {}
        if clock_info.source is not None:
            attributes[CLOCK_SOURCE_KEY] = clock_info.source
        if clock_info.synced is not None:
            attributes[CLOCK_SYNCED_KEY] = clock_info.synced

        envelope = envelope_mapper.to_envelope(
            schema_version=SCHEMA_VERSION,
            event_id=str(event_id),
            event_type=new_event.event_type,
            occurred_at=str(occurred_at),
            recorded_at=str(now),
            severity=new_event.severity,
            resource=envelope_mapper.to_resource(resource),
            actor=envelope_mapper.to_actor(new_event.actor),
            context=envelope_mapper.to_context(new_event.context),
            attributes=attributes,
            data=data,
        )

        error = self._validate_data(new_event.event_type, envelope.data)
        if error is not None:
            return BuildResult(error=error)
        return BuildResult(envelope=envelope)
