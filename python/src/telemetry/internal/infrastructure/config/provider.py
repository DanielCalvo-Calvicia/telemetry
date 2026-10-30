"""The configuration provider: the ConfigurationPort of a consumer that passes its values in."""

from telemetry.internal.application.dtos.configuration import Configuration
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.spool_settings import SpoolSettings
from telemetry.internal.application.errors import ConfigurationInstanceIdFailedError
from telemetry.internal.application.ports.outbound.id_generator_port import IDGeneratorPort
from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions
from telemetry.internal.domain.value_objects.delivery_mode import parse_delivery_mode
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity
from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.config.validate import validate_config


def _none_when_empty(text: str) -> str | None:
    return text if text != "" else None


class ConfigurationProvider:
    """ConfigurationProvider is the ConfigurationPort of a consumer that passes its values in code, or loaded from its
    own source by its own tool. It reads no environment and no file (ADR-004).

    Creating it validates the configuration (ConfigurationError with every problem) and, when the instance id is
    empty, asks the id generator for one (ConfigurationInstanceIdFailedError, with the cause chained). What it hands
    out is a copy: later changes of the Config do not reach it.
    """

    def __init__(self, config: Config, id_generator: IDGeneratorPort) -> None:
        """Validate the configuration and build the Configuration of the core."""
        problems = validate_config(config)
        if problems:
            raise ConfigurationError(problems)
        resource = config.resource
        instance_id = resource.instance_id
        if instance_id == "":
            try:
                instance_id = id_generator.new_instance_id().value
            except Exception as failure:
                raise ConfigurationInstanceIdFailedError("the instance id could not be generated") from failure
        delivery = config.delivery
        limits = config.limits
        masking = config.masking
        self._configuration = Configuration(
            resource=ResourceIdentity(
                service_name=resource.service_name,
                instance_id=instance_id,
                environment=_none_when_empty(resource.environment),
                service_version=_none_when_empty(resource.service_version),
                host=_none_when_empty(resource.host),
                time_zone=_none_when_empty(resource.time_zone),
            ),
            delivery=DeliverySettings(
                mode=parse_delivery_mode(delivery.mode),
                queue_size=delivery.queue_size,
                batch_size=delivery.batch_size,
                flush_interval_milliseconds=delivery.flush_interval_ms,
                timeout_milliseconds=delivery.timeout_ms,
            ),
            limits=Limits(
                max_attribute_keys=limits.max_attribute_keys,
                max_attribute_key_chars=limits.max_attribute_key_chars,
                max_attribute_value_chars=limits.max_attribute_value_chars,
                max_event_bytes=limits.max_event_bytes,
                max_batch_bytes=limits.max_batch_bytes,
            ),
            spool=SpoolSettings(directory=config.spool.directory, quota_bytes=config.spool.quota_bytes),
            masking=MaskerOptions(
                extra_patterns=tuple(masking.extra_patterns),
                field_names=None if masking.field_names is None else tuple(masking.field_names),
                extra_field_names=tuple(masking.extra_field_names),
                field_classes=dict(masking.field_classes),
                class_rules=dict(masking.class_rules),
            ),
        )

    def configuration(self) -> Configuration:
        """Return the validated configuration."""
        return self._configuration
