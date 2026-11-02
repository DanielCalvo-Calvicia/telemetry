"""The UUID generator: the IDGeneratorPort with UUIDv7."""

from typing import Callable, Optional

from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.infrastructure.outbound.shared.uuid_generator.error.map_generate_error import map_generate_error
from telemetry.internal.infrastructure.outbound.shared.uuid_generator.external import read_sources as external
from telemetry.internal.infrastructure.outbound.shared.uuid_generator.helpers.uuid_v7_layout import (
    RANDOM_BYTE_COUNT,
    uuid_v7_text,
)


class UuidGenerator:
    """UuidGenerator implements IDGeneratorPort with UUIDv7, written with the standard library only (REQ-ARC-005).

    The clock and the random source are read in ``external/``; the layout is the pure helper. A failure of the random
    source raises an IdGenerationFailedError chained from the cause: there is NO fallback to a weaker source.
    """

    def __init__(
        self,
        read_milliseconds: Optional[Callable[[], int]] = None,
        read_random: Optional[Callable[[int], bytes]] = None,
    ) -> None:
        """Create the generator; the functions that read the machine can be replaced, which tests do."""
        self._read_milliseconds = read_milliseconds if read_milliseconds is not None else external.read_milliseconds
        self._read_random = read_random if read_random is not None else external.read_random

    def _new_text(self) -> str:
        try:
            return uuid_v7_text(self._read_milliseconds(), self._read_random(RANDOM_BYTE_COUNT))
        except Exception as cause:
            raise map_generate_error(cause) from cause

    def new_event_id(self) -> EventId:
        """Return a new event id, a UUIDv7; raise IdGenerationFailedError when the sources fail."""
        return EventId(self._new_text())

    def new_instance_id(self) -> Identifier:
        """Return the text of a new UUIDv7 as the instance id; raise IdGenerationFailedError when the sources fail."""
        return Identifier(self._new_text())
