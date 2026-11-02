"""The core container: it wires the services of the chain and hands out the two operations of the core."""

from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.dtos.shutdown_result import ShutdownResult
from telemetry.internal.application.ports.inbound.logger_port import LoggerPort
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.application.services.event_pipeline_service import Destinations, EventPipelineService
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.application.services.logger_service import LoggerService
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.composition_root.config.options import Option, Options
from telemetry.internal.composition_root.dependencies.system_clock_dependencies import new_system_clock
from telemetry.internal.composition_root.dependencies.uuid_generator_dependencies import new_uuid_generator
from telemetry.internal.domain.errors import DeliveryModeUnknownError
from telemetry.internal.domain.operations.event_validation.event_validation import validate_event
from telemetry.internal.domain.operations.masking.sensitive_data_masker import SensitiveDataMasker
from telemetry.internal.domain.value_objects.delivery_mode import parse_delivery_mode
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.errors import ConfigProblem, ConfigurationError
from telemetry.internal.infrastructure.config.provider import ConfigurationProvider
from telemetry.internal.infrastructure.config.validate import validate_config

SPOOL_REQUIRED_PROBLEM = ConfigProblem(
    "delivery.mode", ERROR_CODE_CONFIG_FIELD_INVALID, "the durable mode needs a spool adapter"
)


def _is_durable(config: Config) -> bool:
    try:
        return parse_delivery_mode(config.delivery.mode) is DeliveryMode.DURABLE
    except DeliveryModeUnknownError:
        return False


class Core:
    """Core is the telemetry core a project builds with ``new_core`` and keeps for the life of its process.

    It holds the wiring of one build and nothing global: two cores in one process share nothing. It is safe to call
    from many threads.
    """

    def __init__(self, pipeline: EventPipelineService, logger: LoggerPort) -> None:
        """Keep the pipeline and the logger over it; build a core with ``new_core``."""
        self._pipeline = pipeline
        self._logger = logger

    @property
    def logger(self) -> LoggerPort:
        """The logger of this core: one method per severity, each call one ``log.entry.recorded`` event; no raise."""
        return self._logger

    def emit(self, new_event: NewEventInput) -> EmitResult:
        """Turn one input into one masked, limited, valid event delivered to the sinks; it never raises (ADR-005)."""
        return self._pipeline.emit(new_event)

    def shutdown(self) -> ShutdownResult:
        """Shut the core down and say what became of the events still waiting.

        This is a STUB that returns the zero result: T-05.4 gives it the flush and the shutdown (REQ-DLV-010). Until
        then every emit has already been delivered, synchronously, when it returns.
        """
        return ShutdownResult()


def new_core(config: Config, *options: Option) -> Core:
    """Build the core from a configuration and the options; the adapters are chosen in code (REQ-ARC-013, ADR-013).

    The options are applied in order; the clock and the id generator default to the system clock and the UUIDv7
    generator. The configuration is validated here, at start, and ``ConfigurationError`` is raised with EVERY problem
    (REQ-CFG-002); a durable delivery mode without a spool adds the problem ``delivery.mode`` to the same error.
    Nothing is read from the environment and nothing is kept in a global.
    """
    applied = Options()
    for option in options:
        option(applied)
    clock = applied.clock if applied.clock is not None else new_system_clock()
    id_generator = applied.id_generator if applied.id_generator is not None else new_uuid_generator()

    problems = list(validate_config(config))
    if applied.spool is None and _is_durable(config):
        problems.append(SPOOL_REQUIRED_PROBLEM)
    if problems:
        raise ConfigurationError(tuple(problems))

    provider = ConfigurationProvider(config, id_generator)
    configuration = provider.configuration()
    builder = EventBuilderService(clock, id_generator, provider, validate_event)
    masking = MaskingService(SensitiveDataMasker(configuration.masking))
    limits = LimitsService(configuration.limits)
    destinations = Destinations(
        log=tuple(applied.log_sinks),
        event=tuple(applied.event_sinks),
        metrics=tuple(applied.metrics_sinks),
        trace=tuple(applied.trace_sinks),
        device_telemetry=tuple(applied.device_telemetry_sinks),
        ai_telemetry=tuple(applied.ai_telemetry_sinks),
    )
    pipeline = EventPipelineService(builder, masking, limits, destinations)
    return Core(pipeline, LoggerService(pipeline))
