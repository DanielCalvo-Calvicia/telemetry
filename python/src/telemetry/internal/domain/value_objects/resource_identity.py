"""The resource of an event: the service that produced it."""

import re
from dataclasses import dataclass
from typing import Optional

from telemetry.internal.domain.errors import ResourceFieldInvalidError, ResourceFieldMissingError, TimeZoneInvalidError
from telemetry.internal.domain.value_objects.field_problems import FieldProblems

TIME_ZONE_PATTERN = re.compile(r"[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*")


def validate_time_zone(text: str) -> str:
    """Return ``text`` when it has the shape of an IANA name; raise TimeZoneInvalidError otherwise.

    The name is for display only (ADR-012); it is never used to compute a time, so it is not looked up.
    """
    if not isinstance(text, str) or TIME_ZONE_PATTERN.fullmatch(text) is None:
        raise TimeZoneInvalidError("a time zone must look like an IANA name, for example Europe/Madrid")
    return text


@dataclass(frozen=True, slots=True)
class ResourceIdentity:
    """service_name and instance_id are required identifiers; the other fields are optional (``None`` is absent).

    An optional field that is present but empty is invalid. Every failed field is reported in one error:
    ResourceFieldMissingError when a required field is missing, ResourceFieldInvalidError otherwise.
    """

    service_name: str
    instance_id: str
    environment: Optional[str] = None
    service_version: Optional[str] = None
    host: Optional[str] = None
    time_zone: Optional[str] = None

    def __post_init__(self) -> None:
        """Validate every field and report all the failures together."""
        problems = FieldProblems()
        problems.identifier("service_name", self.service_name, required=True)
        problems.identifier("instance_id", self.instance_id, required=True)
        problems.identifier("environment", self.environment, required=False)
        problems.identifier("service_version", self.service_version, required=False)
        problems.identifier("host", self.host, required=False)
        if self.time_zone is not None:
            try:
                validate_time_zone(self.time_zone)
            except TimeZoneInvalidError as error:
                problems.add_invalid("time_zone", error)
        problems.raise_if_any("the resource", ResourceFieldMissingError, ResourceFieldInvalidError)
