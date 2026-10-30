"""The configuration the consumer passed to the core."""

from dataclasses import dataclass, field

from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.spool_settings import SpoolSettings
from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity


@dataclass(frozen=True, slots=True)
class Configuration:
    """Configuration is what the consumer passed in, validated: the whole of it since T-03.6.

    The resource is required; the other parts have the defaults of the standard, so that ``Configuration(resource)``
    is a valid start.
    """

    resource: ResourceIdentity
    delivery: DeliverySettings = field(default_factory=DeliverySettings)
    limits: Limits = field(default_factory=Limits)
    spool: SpoolSettings = field(default_factory=SpoolSettings)
    masking: MaskerOptions = field(default_factory=MaskerOptions)
