"""Black-box tests of EventPipelineService and destination_kind_of: routing, delivery, failures, masking first."""

import datetime
import time
import unittest
from typing import Any, Optional, Sequence

from internal.tests.eventually import SHORT_FLUSH_INTERVAL_MS, eventually
from internal.tests.fakes import (
    FakeAiTelemetrySink,
    FakeClock,
    FakeConfiguration,
    FakeDeviceTelemetrySink,
    FakeEventSink,
    FakeIdGenerator,
    FakeLogSink,
    FakeMetricsSink,
    FakeTraceSink,
)
from telemetry.internal.application.call_context.call_context import CallContext, use_call_context
from telemetry.internal.application.dtos.configuration import Configuration
from telemetry.internal.application.dtos.delivery_settings import DeliverySettings
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.services.delivery_service import (
    REASON_NO_DESTINATION,
    REASON_QUEUE_FULL,
    REASON_SINK_PANICKED,
    DeliveryService,
    Destinations,
)
from telemetry.internal.application.services.destination_kind import (
    DESTINATION_AI_TELEMETRY,
    DESTINATION_DEVICE_TELEMETRY,
    DESTINATION_EVENT,
    DESTINATION_LOG,
    DESTINATION_METRICS,
    DESTINATION_TRACE,
    destination_kind_of,
)
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.application.services.event_pipeline_service import EventPipelineService
from telemetry.internal.application.services.limits_service import LimitsService
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.composition_root.dependencies.memory_queue_dependencies import new_memory_queue
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.event_validation.event_validation import validate_event
from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions, SensitiveDataMasker
from telemetry.internal.domain.rules import MASKED_TEXT
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.error_codes_generated import (
    ERROR_CODE_EVENT_BUILD_FAILED,
    ERROR_CODE_EVENT_SCHEMA_INVALID,
    ERROR_CODE_EVENT_TYPE_UNKNOWN,
    ERROR_CODE_LIMIT_EXCEEDED,
)
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity
from telemetry.internal.infrastructure.outbound.shared.system_clock.system_clock import SystemClock

WALL_TIME = datetime.datetime(2026, 10, 28, 10, 0, 0, 123456, tzinfo=datetime.timezone.utc)
LOG_DATA = {"category": "http", "message": "started"}

SECRETS = {
    "aws key": ("AKIAIOSFODNN7EXAMPLE", "AKIAIOSFODNN7EXAMPLE"),
    "jwt": ("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.c2lnbmF0dXJl", "c2lnbmF0dXJl"),
    "pem block": (
        "-----BEGIN RSA PRIVATE KEY-----\nFAKEFAKEFAKEFAKE\nFAKEFAKEFAKEFAKE\n-----END RSA PRIVATE KEY-----",
        "FAKEFAKEFAKEFAKE",
    ),
    "connection string": ("postgres://admin:hunter2secret@db.local:5432/app", "hunter2secret"),
    "bearer": ("Authorization: Bearer abcdef123456secret", "abcdef123456secret"),
    "password pair": ("password=hunter2secret", "hunter2secret"),
}
ALL_LEAKS = tuple(leak for _, leak in SECRETS.values())


def accept_anything(event_type: str, data: Any) -> Optional[Exception]:
    """A validator that accepts any data: the registry holds only log events today."""
    return None


def make_pipeline(
    test: unittest.TestCase,
    destinations: Destinations,
    validator: Any = validate_event,
    limits: Optional[Limits] = None,
) -> EventPipelineService:
    """A real chain (builder, masking, limits) over fakes, with a started delivery that stops when the test ends."""
    configuration = FakeConfiguration(
        Configuration(resource=ResourceIdentity(service_name="sensor-hub", instance_id="hub-01"))
    )
    builder = EventBuilderService(FakeClock(wall_time=WALL_TIME), FakeIdGenerator(), configuration, validator)
    masking = MaskingService(SensitiveDataMasker(MaskerOptions()))
    return EventPipelineService(builder, masking, LimitsService(limits or Limits()), make_delivery(test, destinations))


def make_delivery(test: unittest.TestCase, destinations: Destinations) -> DeliveryService:
    """A started delivery service over the destinations, with a short flush interval; it stops with the test."""
    delivery = DeliveryService(
        destinations,
        DeliverySettings(flush_interval_milliseconds=SHORT_FLUSH_INTERVAL_MS),
        SystemClock(),
        new_memory_queue,
    )
    delivery.start()
    test.addCleanup(delivery.stop)
    return delivery


def delivered(sink: Any, count: int) -> None:
    """Wait until the fake sink has received ``count`` events: delivery is asynchronous."""
    eventually(5.0, lambda: sum(len(batch) for batch in sink.batches) >= count, f"{count} events at the sink")


def settle() -> None:
    """Give the workers time to send what they would send, to prove that nothing arrives."""
    time.sleep(0.03)


def log_event(**fields: Any) -> NewEventInput:
    values: dict[str, Any] = {"event_type": "log.entry.recorded", "severity": Severity.INFO, "data": dict(LOG_DATA)}
    values.update(fields)
    return NewEventInput(**values)


class RaisingSink:
    """A sink that breaks the rule of ADR-005 and raises; the pipeline must contain it."""

    def __init__(self) -> None:
        self.calls = 0

    def send(self, events: Sequence[Any]) -> SendResult:
        self.calls += 1
        raise RuntimeError("the sink broke with secret hunter2secret")


class WrongResultSink:
    """A sink that returns something that is not a SendResult."""

    def send(self, events: Sequence[Any]) -> Any:
        return "not a result"


class DestinationKindTest(unittest.TestCase):
    def test_the_routing_table(self):
        table = {
            "log.entry.recorded": DESTINATION_LOG,
            "log": DESTINATION_LOG,
            "metric.counter.incremented": DESTINATION_METRICS,
            "trace.span.ended": DESTINATION_TRACE,
            "device.battery.reported": DESTINATION_DEVICE_TELEMETRY,
            "ai.model.invoked": DESTINATION_AI_TELEMETRY,
            "execution.job.finished": DESTINATION_EVENT,
            "report.created": DESTINATION_EVENT,
            "": DESTINATION_EVENT,
            ".": DESTINATION_EVENT,
            ".log": DESTINATION_EVENT,
            "LOG.entry": DESTINATION_EVENT,
            "logs.entry": DESTINATION_EVENT,
            "log-entry": DESTINATION_EVENT,
            "ai_model": DESTINATION_EVENT,
            "metrics.x": DESTINATION_EVENT,
            " log.x": DESTINATION_EVENT,
            "unknown": DESTINATION_EVENT,
        }
        for event_type, kind in table.items():
            with self.subTest(event_type=event_type):
                self.assertEqual(destination_kind_of(event_type), kind)

    def test_a_value_that_is_not_text_goes_to_the_events(self):
        for value in (None, 5, b"log.x", ["log"]):
            with self.subTest(value=value):
                self.assertEqual(destination_kind_of(value), DESTINATION_EVENT)  # type: ignore[arg-type]

    def test_every_kind_is_a_field_of_destinations(self):
        kinds = (
            DESTINATION_LOG,
            DESTINATION_EVENT,
            DESTINATION_METRICS,
            DESTINATION_TRACE,
            DESTINATION_DEVICE_TELEMETRY,
            DESTINATION_AI_TELEMETRY,
        )
        for kind in kinds:
            self.assertEqual(getattr(Destinations(), kind), ())


class DeliveryTest(unittest.TestCase):
    def test_one_input_reaches_the_log_sink_once_as_a_batch_of_one(self):
        sink = FakeLogSink()
        result = make_pipeline(self, Destinations(log=(sink,))).emit(log_event())
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (1, 0, 0, ""))
        delivered(sink, 1)
        settle()
        self.assertEqual(len(sink.batches), 1)
        self.assertEqual(len(sink.batches[0]), 1)
        self.assertEqual(sink.batches[0][0].event_type, "log.entry.recorded")

    def test_every_destination_of_the_kind_receives_the_same_event_in_order(self):
        sinks = [FakeLogSink(), FakeLogSink(), FakeLogSink()]
        other = FakeEventSink()
        result = make_pipeline(self, Destinations(log=tuple(sinks), event=(other,))).emit(log_event())
        self.assertEqual(result.accepted, 3)
        for sink in sinks:
            delivered(sink, 1)
        settle()
        texts = [canonical_json(sink.batches[0][0]) for sink in sinks]
        self.assertEqual(len(set(texts)), 1)
        self.assertEqual(other.calls, 0)

    def test_what_a_sink_answers_is_not_part_of_the_result_of_the_call(self):
        first, second, third = FakeLogSink(), FakeLogSink(), FakeLogSink()
        first.set_result(SendResult(accepted=0, dropped=1, failed=0, reason="queue_full"))
        second.set_result(SendResult(accepted=0, dropped=0, failed=1, reason="send_failed"))
        result = make_pipeline(self, Destinations(log=(first, second, third))).emit(log_event())
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (3, 0, 0, ""))
        for sink in (first, second, third):
            delivered(sink, 1)

    def test_a_sink_that_raises_does_not_stop_the_others_and_does_not_show_in_the_result(self):
        raising, after = RaisingSink(), FakeLogSink()
        before = FakeLogSink()
        result = make_pipeline(self, Destinations(log=(before, raising, after))).emit(log_event())
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (3, 0, 0, ""))
        delivered(before, 1)
        delivered(after, 1)
        eventually(5.0, lambda: raising.calls == 1, "the raising sink being called")
        self.assertEqual(REASON_SINK_PANICKED, "sink_panicked")
        self.assertEqual(REASON_QUEUE_FULL, "queue_full")
        self.assertNotIn("hunter2secret", result.reason)

    def test_a_sink_that_returns_garbage_does_not_stop_the_others(self):
        good = FakeLogSink()
        result = make_pipeline(self, Destinations(log=(WrongResultSink(), good))).emit(log_event())
        self.assertEqual((result.accepted, result.failed), (2, 0))
        delivered(good, 1)

    def test_no_destination_of_the_kind_drops_the_event_with_a_reason(self):
        other = FakeEventSink()
        result = make_pipeline(self, Destinations(event=(other,))).emit(log_event())
        self.assertEqual((result.accepted, result.dropped, result.failed), (0, 1, 0))
        self.assertEqual(result.reason, REASON_NO_DESTINATION)
        self.assertEqual(REASON_NO_DESTINATION, "no_destination")
        settle()
        self.assertEqual(other.calls, 0)

    def test_the_reason_stays_empty_when_everything_is_queued(self):
        result = make_pipeline(self, Destinations(log=(FakeLogSink(), FakeLogSink()))).emit(log_event())
        self.assertEqual(result.reason, "")

    def test_a_full_queue_drops_the_new_event_with_queue_full(self):
        sink = FakeLogSink()
        delivery = DeliveryService(
            Destinations(log=(sink,)),
            DeliverySettings(queue_size=1, batch_size=1, flush_interval_milliseconds=60_000),
            SystemClock(),
            new_memory_queue,
        )  # never started: the queue only fills
        configuration = FakeConfiguration(Configuration(resource=ResourceIdentity(service_name="s", instance_id="i")))
        builder = EventBuilderService(FakeClock(wall_time=WALL_TIME), FakeIdGenerator(), configuration, validate_event)
        pipeline = EventPipelineService(
            builder, MaskingService(SensitiveDataMasker(MaskerOptions())), LimitsService(Limits()), delivery
        )
        first = pipeline.emit(log_event())
        second = pipeline.emit(log_event())
        self.assertEqual((first.accepted, first.dropped, first.reason), (1, 0, ""))
        self.assertEqual((second.accepted, second.dropped, second.reason), (0, 1, REASON_QUEUE_FULL))


class DropTest(unittest.TestCase):
    def test_an_unknown_event_type_is_dropped_and_reaches_no_sink(self):
        sink = FakeEventSink()
        result = make_pipeline(self, Destinations(event=(sink,))).emit(log_event(event_type="report.created"))
        self.assertEqual((result.accepted, result.dropped, result.reason), (0, 1, ERROR_CODE_EVENT_TYPE_UNKNOWN))
        settle()
        self.assertEqual(sink.calls, 0)

    def test_invalid_data_is_dropped_with_its_code(self):
        sink = FakeLogSink()
        result = make_pipeline(self, Destinations(log=(sink,))).emit(log_event(data={"category": "http"}))
        self.assertEqual((result.dropped, result.reason), (1, ERROR_CODE_EVENT_SCHEMA_INVALID))
        settle()
        self.assertEqual(sink.calls, 0)

    def test_an_event_over_the_limit_is_dropped_with_limit_exceeded(self):
        sink = FakeLogSink()
        pipeline = make_pipeline(
            self, Destinations(log=(sink,)), limits=Limits(max_event_bytes=100, max_batch_bytes=1000)
        )
        result = pipeline.emit(log_event())
        self.assertEqual((result.dropped, result.reason), (1, ERROR_CODE_LIMIT_EXCEEDED))
        settle()
        self.assertEqual(sink.calls, 0)

    def test_a_validator_that_raises_gives_dropped_event_build_failed(self):
        def raising_validator(event_type: str, data: Any) -> Optional[Exception]:
            raise RuntimeError("boom")

        sink = FakeLogSink()
        result = make_pipeline(self, Destinations(log=(sink,)), validator=raising_validator).emit(log_event())
        self.assertEqual((result.accepted, result.dropped, result.failed), (0, 1, 0))
        self.assertEqual(result.reason, ERROR_CODE_EVENT_BUILD_FAILED)
        settle()
        self.assertEqual(sink.calls, 0)

    def test_an_error_without_a_code_still_gives_a_reason(self):
        def plain_error_validator(event_type: str, data: Any) -> Optional[Exception]:
            return ValueError("no code")

        result = make_pipeline(self, Destinations(log=(FakeLogSink(),)), validator=plain_error_validator).emit(
            log_event()
        )
        self.assertEqual((result.dropped, result.reason), (1, ERROR_CODE_EVENT_BUILD_FAILED))

    def test_anything_unexpected_never_escapes(self):
        pipeline = make_pipeline(self, Destinations(log=(FakeLogSink(),)))
        for garbage in (None, 5, "log.entry.recorded", object()):
            with self.subTest(garbage=garbage):
                result = pipeline.emit(garbage)  # type: ignore[arg-type]
                self.assertEqual((result.dropped, result.reason), (1, ERROR_CODE_EVENT_BUILD_FAILED))

    def test_a_masking_failure_drops_the_event_and_nothing_is_sent_unmasked(self):
        class RaisingMasker(SensitiveDataMasker):
            def mask_map(self, mapping: Any) -> Any:
                raise RuntimeError("boom")

        configuration = FakeConfiguration(Configuration(resource=ResourceIdentity(service_name="s", instance_id="i")))
        builder = EventBuilderService(FakeClock(wall_time=WALL_TIME), FakeIdGenerator(), configuration, validate_event)
        sink = FakeLogSink()
        pipeline = EventPipelineService(
            builder,
            MaskingService(RaisingMasker(MaskerOptions())),
            LimitsService(Limits()),
            make_delivery(self, Destinations(log=(sink,))),
        )
        result = pipeline.emit(log_event(data={"category": "x", "message": "password=hunter2secret"}))
        self.assertEqual((result.dropped, result.reason), (1, "masking_failed"))
        settle()
        self.assertEqual(sink.calls, 0)

    def test_a_limits_failure_drops_the_event(self):
        class FailingLimits:
            def apply(self, envelope: Any) -> Any:
                raise RuntimeError("boom")

        configuration = FakeConfiguration(Configuration(resource=ResourceIdentity(service_name="s", instance_id="i")))
        builder = EventBuilderService(FakeClock(wall_time=WALL_TIME), FakeIdGenerator(), configuration, validate_event)
        sink = FakeLogSink()
        pipeline = EventPipelineService(
            builder,
            MaskingService(SensitiveDataMasker(MaskerOptions())),
            FailingLimits(),  # type: ignore[arg-type]
            make_delivery(self, Destinations(log=(sink,))),
        )
        result = pipeline.emit(log_event())
        self.assertEqual((result.dropped, result.reason), (1, ERROR_CODE_EVENT_BUILD_FAILED))
        settle()
        self.assertEqual(sink.calls, 0)


class CallContextTest(unittest.TestCase):
    def sent_context(self, new_event: NewEventInput, call_context: Optional[CallContext]) -> Any:
        sink = FakeLogSink()
        pipeline = make_pipeline(self, Destinations(log=(sink,)))
        with use_call_context(call_context):
            pipeline.emit(new_event)
        delivered(sink, 1)
        return sink.batches[0][0].context

    def test_the_call_context_of_the_caller_becomes_the_context(self):
        context = self.sent_context(log_event(), CallContext(execution_id="run-7", correlation_id="order-9"))
        self.assertEqual((context.execution_id, context.correlation_id), ("run-7", "order-9"))
        self.assertIsNone(context.trace_id)

    def test_a_context_given_in_the_input_wins(self):
        given = CorrelationContext(execution_id="mine")
        context = self.sent_context(log_event(context=given), CallContext(execution_id="run-7"))
        self.assertEqual(context.execution_id, "mine")

    def test_no_call_context_or_an_empty_one_or_an_invalid_one_gives_no_context(self):
        for call_context in (None, CallContext(), CallContext(application="a"), CallContext(trace_id="not-hex")):
            with self.subTest(call_context=call_context):
                self.assertIsNone(self.sent_context(log_event(), call_context))

    def test_the_input_is_not_modified(self):
        new_event = log_event()
        self.sent_context(new_event, CallContext(execution_id="run-7"))
        self.assertIsNone(new_event.context)


class MaskingBeforeEverySinkTest(unittest.TestCase):
    """THE SPYING TEST (ADR-006): no sink of any kind ever sees a secret, in data, attributes or context."""

    def assert_clean(self, spy: Any, secret_kind: str) -> None:
        text = canonical_json(spy.batches[0][0]).decode("utf-8")
        for leak in ALL_LEAKS:
            self.assertNotIn(leak, text, f"{secret_kind} leaked {leak!r}")
        self.assertIn(MASKED_TEXT, text)

    def test_every_secret_in_every_place_for_every_kind_of_sink_through_the_pipeline(self):
        kinds = (
            ("log", FakeLogSink, "log.entry.recorded"),
            ("event", FakeEventSink, "execution.job.finished"),
            ("metrics", FakeMetricsSink, "metric.counter.incremented"),
            ("trace", FakeTraceSink, "trace.span.ended"),
            ("device_telemetry", FakeDeviceTelemetrySink, "device.battery.reported"),
            ("ai_telemetry", FakeAiTelemetrySink, "ai.model.invoked"),
        )
        for destination, sink_class, event_type in kinds:
            for secret_kind, (secret, _) in SECRETS.items():
                for place in ("data", "attributes", "context"):
                    if place == "context" and chr(10) in secret:
                        continue  # an identifier of the context never holds a line break: the block cannot get there
                    with self.subTest(destination=destination, secret=secret_kind, place=place):
                        spy = sink_class()
                        pipeline = make_pipeline(self, Destinations(**{destination: (spy,)}), validator=accept_anything)
                        data: dict[str, Any] = {"category": "x", "message": "plain"}
                        attributes = None
                        context = None
                        if place == "data":
                            data["message"] = "before " + secret + " after"
                        elif place == "attributes":
                            attributes = EventAttributes({"note": "attr " + secret, "tags": [secret, "plain"]})
                        else:
                            context = CorrelationContext(correlation_id=secret)
                        result = pipeline.emit(
                            NewEventInput(
                                event_type=event_type,
                                severity=Severity.INFO,
                                data=data,
                                attributes=attributes,
                                context=context,
                            )
                        )
                        self.assertEqual((result.accepted, result.dropped), (1, 0), result)
                        delivered(spy, 1)
                        self.assertEqual(spy.calls, 1)
                        self.assert_clean(spy, secret_kind)

    def test_the_context_of_the_call_context_is_masked_too(self):
        spy = FakeLogSink()
        pipeline = make_pipeline(self, Destinations(log=(spy,)))
        with use_call_context(CallContext(correlation_id="password=hunter2secret")):
            pipeline.emit(log_event())
        delivered(spy, 1)
        self.assertNotIn("hunter2secret", canonical_json(spy.batches[0][0]).decode("utf-8"))

    def test_a_spy_that_records_every_envelope_sees_only_masked_text_whatever_the_number_of_sinks(self):
        class Spy:
            def __init__(self) -> None:
                self.leaks: list = []
                self.seen = 0

            def send(self, events: Sequence[Any]) -> SendResult:
                for envelope in events:
                    text = canonical_json(envelope).decode("utf-8")
                    self.leaks.extend(leak for leak in ALL_LEAKS if leak in text)
                    self.seen += 1
                return SendResult(accepted=len(events), dropped=0, failed=0)

        spies = [Spy(), Spy()]
        pipeline = make_pipeline(self, Destinations(log=tuple(spies)))
        for secret, _ in SECRETS.values():
            pipeline.emit(
                log_event(
                    data={"category": "x", "message": secret},
                    attributes=EventAttributes({"note": secret}),
                    context=None if chr(10) in secret else CorrelationContext(correlation_id=secret),
                )
            )
        for spy in spies:
            eventually(5.0, lambda spy=spy: spy.seen >= len(SECRETS), "the spy seeing every event")
            self.assertEqual(spy.seen, len(SECRETS))
            self.assertEqual(spy.leaks, [])


if __name__ == "__main__":
    unittest.main()
