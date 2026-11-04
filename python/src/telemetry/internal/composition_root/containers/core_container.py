"""The core container: it wires the services of the chain and hands out the two operations of the core."""

from telemetry.internal.application.dtos.diagnostics_snapshot import DiagnosticsSnapshot
from telemetry.internal.application.dtos.emit_result import EmitResult
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.dtos.shutdown_result import ShutdownResult
from telemetry.internal.application.ports.inbound.logger_port import LoggerPort
from telemetry.internal.application.services.delivery_service import DeliveryService, Destinations
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.application.services.event_pipeline_service import EventPipelineService
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.application.services.logger_service import LoggerService
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.composition_root.config.options import Option, Options
from telemetry.internal.composition_root.dependencies.diagnostics_channel_dependencies import new_diagnostics_channel
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.composition_root.dependencies.system_clock_dependencies import new_system_clock
from telemetry.internal.composition_root.dependencies.system_random_dependencies import new_system_random
from telemetry.internal.composition_root.dependencies.uuid_generator_dependencies import new_uuid_generator
from telemetry.internal.domain.errors import DeliveryModeUnknownError
from telemetry.internal.domain.operations.event_validation.event_validation import validate_event
from telemetry.internal.domain.operations.masking.sensitive_data_masker import SensitiveDataMasker
from telemetry.internal.domain.rules import DEFAULT_SHUTDOWN_TIMEOUT_MILLISECONDS
from telemetry.internal.domain.value_objects.delivery_mode import parse_delivery_mode
from telemetry.internal.domain.value_objects.enums_generated import DeliveryMode
from telemetry.internal.domain.value_objects.error_codes_generated import ERROR_CODE_CONFIG_FIELD_INVALID
from telemetry.internal.infrastructure.config.config import Config
from telemetry.internal.infrastructure.config.errors import ConfigProblem, ConfigurationError
from telemetry.internal.infrastructure.config.provider import ConfigurationProvider
from telemetry.internal.infrastructure.config.validate import validate_config

DURABLE_NOT_AVAILABLE_PROBLEM = ConfigProblem(
    "delivery.mode",
    ERROR_CODE_CONFIG_FIELD_INVALID,
    "the durable mode is not available yet: it needs the spool of T-06.5",
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

    def __init__(self, pipeline: EventPipelineService, logger: LoggerPort, delivery: DeliveryService) -> None:
        """Keep the pipeline, the logger over it and the delivery service; build a core with ``new_core``."""
        self._pipeline = pipeline
        self._logger = logger
        self._delivery = delivery

    @property
    def logger(self) -> LoggerPort:
        """The logger of this core: one method per severity, each call one ``log.entry.recorded`` event; no raise."""
        return self._logger

    def emit(self, new_event: NewEventInput) -> EmitResult:
        """Turn one input into one masked, limited, valid event QUEUED for the sinks; it never raises (ADR-005).

        The call returns when the event is in the queues: the sinks are called later by their workers, so ``accepted``
        in the result means queued, not delivered.
        """
        return self._pipeline.emit(new_event)

    def diagnostics(self) -> DiagnosticsSnapshot:
        """Return a COPY of the diagnostics of every destination and of the diagnostics channel (REQ-DGN-001).

        Per destination: queue depth and capacity, events accepted, delivered and dropped (and the drops by reason),
        export failures, attempts, retries, batches and their size, the latency of the calls, the last success, the
        configuration and serialization failures and the result of the last flush. The spool figures are 0 until
        T-06.5. Changing the snapshot does not change the library. It never raises and it still answers after
        ``shutdown``. Diagnostics are not a health signal: a failing destination does not make the application
        unhealthy.
        """
        try:
            return self._delivery.diagnostics()
        except Exception:  # never fails the caller (ADR-005)
            return DiagnosticsSnapshot()

    def flush(self, timeout_ms: int = DEFAULT_SHUTDOWN_TIMEOUT_MILLISECONDS) -> ShutdownResult:
        """Send what is queued now and keep running; return what became of it. It never raises.

        BECAUSE THERE IS NO SYNCHRONOUS SEND, a program MUST call ``shutdown()`` before it exits, or the events still
        queued are lost: the library is passive and registers no hook at interpreter exit. ``flush`` is for a
        checkpoint in a program that goes on.

        Per destination it counts the events queued plus the ones being sent when it is called, makes the workers send
        them at once (the flush interval does not apply) and waits at most ``timeout_ms`` milliseconds. ``sent`` are the
        events the destinations accepted, ``dropped`` all the others (dropped or failed by the destination, displaced
        from a full queue, or still waiting when the time ends; a failure counts after the retries of the retry mode)
        and ``spooled`` is 0 until the spool (T-06). A timeout of 0 or less returns at once with everything pending
        counted as dropped; after ``shutdown`` it returns the zero result. It blocks the calling thread for its
        duration, also in asyncio.
        """
        try:
            return self._delivery.flush(timeout_ms)
        except Exception:  # never fails the caller (ADR-005)
            return ShutdownResult()

    def shutdown(self, timeout_ms: int = DEFAULT_SHUTDOWN_TIMEOUT_MILLISECONDS) -> ShutdownResult:
        """Flush, then stop the workers and close the core; return what became of the events. It never raises.

        BECAUSE THERE IS NO SYNCHRONOUS SEND, a program MUST call ``shutdown()`` before it exits, or the events still
        queued are lost: the library is passive and registers no hook at interpreter exit.

        The flush and the stop share the budget of ``timeout_ms`` milliseconds (``flush`` says how the events are
        counted). The workers are joined only for the time that is left: a destination that never answers does not hold
        the call past the timeout, because its worker is a DAEMON thread that is abandoned and ends with the process.
        After it the core is closed: ``emit`` and every log method return ``dropped=1`` with the reason ``shutdown``
        and the drop is counted in the stats of the destinations. A second call returns the FIRST result and ``flush``
        returns the zero result. It blocks the calling thread for its duration, and so the event loop when it is
        called from asyncio; it may be called there, but it is better to run it in a thread (``asyncio.to_thread``).
        """
        try:
            return self._delivery.shutdown(timeout_ms)
        except Exception:  # never fails the caller (ADR-005)
            return ShutdownResult()


def new_core(config: Config, *options: Option) -> Core:
    """Build the core from a configuration and the options; the adapters are chosen in code (REQ-ARC-013, ADR-013).

    The options are applied in order; the clock and the id generator default to the system clock and the UUIDv7
    generator. The configuration is validated here, at start, and ``ConfigurationError`` is raised with EVERY problem
    (REQ-CFG-002); the durable delivery mode adds ONE problem, ``delivery.mode``, to the same error, with or without a
    spool adapter: the durable mode needs the spool of T-06.5 and is refused until it exists (the validation of the
    configuration itself still accepts it, so the profiles ``robotics`` and ``offline_device`` stay valid values).
    Nothing is read from the environment and nothing is kept in a global.

    This is the one place that starts the delivery: it builds the DeliveryService (a bounded queue and a daemon worker
    thread per destination, sized by the delivery section of the configuration, with the mode ``best_effort`` or
    ``retry``, the system random source for the jitter and the diagnostics channel with the callbacks and the writer
    of the options) and calls ``start()``; nothing runs on
    import or in a constructor. The delivery measures time with the system clock, not with the clock of the options:
    that clock stamps the events, and a fake one would never let a batch age.
    """
    applied = Options()
    for option in options:
        option(applied)
    clock = applied.clock if applied.clock is not None else new_system_clock()
    id_generator = applied.id_generator if applied.id_generator is not None else new_uuid_generator()

    problems = list(validate_config(config))
    if _is_durable(config):
        problems.append(DURABLE_NOT_AVAILABLE_PROBLEM)
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
    delivery_clock = new_system_clock()
    delivery = DeliveryService(
        destinations,
        configuration.delivery,
        delivery_clock,
        new_memory_queue,
        new_system_random(),
        new_diagnostics_channel(applied.diagnostics_callbacks, applied.diagnostics_writer, delivery_clock),
    )
    pipeline = EventPipelineService(builder, masking, limits, delivery)
    delivery.start()
    return Core(pipeline, LoggerService(pipeline), delivery)
