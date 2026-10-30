"""Black-box tests of MaskingService, with a spying sink: no secret reaches a sink, whatever the capability."""

import copy
import datetime
import threading
import unittest
from typing import Any, Optional

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
from telemetry.internal.application.dtos.configuration import Configuration
from telemetry.internal.application.dtos.mask_result import MaskResult
from telemetry.internal.application.dtos.new_event_input import NewEventInput
from telemetry.internal.application.errors import EnvelopeMaskingFailedError
from telemetry.internal.application.services.event_builder_service import EventBuilderService
from telemetry.internal.application.services.masking_service import MaskingService
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.domain.operations.event_validation.event_validation import validate_event
from telemetry.internal.domain.operations.masking.sensitive_data_masker import MaskerOptions, SensitiveDataMasker
from telemetry.internal.domain.rules import MASKED_TEXT
from telemetry.internal.domain.value_objects.correlation_context import CorrelationContext
from telemetry.internal.domain.value_objects.enums_generated import ActorKind, Severity
from telemetry.internal.domain.value_objects.envelope_generated import Actor, Envelope
from telemetry.internal.domain.value_objects.event_attributes import EventAttributes
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity

WALL_TIME = datetime.datetime(2026, 10, 28, 10, 0, 0, 123456, tzinfo=datetime.timezone.utc)

# The secret of each kind (fake) and the text that must survive around it.
SECRETS = {
    "aws key": "AKIAIOSFODNN7EXAMPLE",
    "jwt": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.c2lnbmF0dXJl",
    "pem block": "-----BEGIN RSA PRIVATE KEY-----\nFAKEFAKEFAKEFAKE\nFAKEFAKEFAKEFAKE\n-----END RSA PRIVATE KEY-----",
    "connection string": "postgres://admin:hunter2secret@db.local:5432/app",
    "bearer": "Authorization: Bearer abcdef123456secret",
    "password pair": "password=hunter2secret",
}
# What must never appear in a batch.
LEAKS = {
    "aws key": "AKIAIOSFODNN7EXAMPLE",
    "jwt": "c2lnbmF0dXJl",
    "pem block": "FAKEFAKEFAKEFAKE",
    "connection string": "hunter2secret",
    "bearer": "abcdef123456secret",
    "password pair": "hunter2secret",
}
SINKS = (FakeLogSink, FakeEventSink, FakeMetricsSink, FakeTraceSink, FakeDeviceTelemetrySink, FakeAiTelemetrySink)


def make_builder() -> EventBuilderService:
    """A real builder over fakes, with the real validator."""
    configuration = FakeConfiguration(
        Configuration(resource=ResourceIdentity(service_name="sensor-hub", instance_id="hub-01"))
    )
    return EventBuilderService(FakeClock(wall_time=WALL_TIME), FakeIdGenerator(), configuration, validate_event)


def make_service(**options: Any) -> MaskingService:
    return MaskingService(SensitiveDataMasker(MaskerOptions(**options)))


def build(message: str, attributes: Optional[dict] = None, context: Optional[CorrelationContext] = None) -> Envelope:
    """Build a log envelope that has NOT been masked."""
    result = make_builder().build(
        NewEventInput(
            event_type="log.entry.recorded",
            severity=Severity.INFO,
            data={"category": "test", "message": message},
            context=context,
            attributes=EventAttributes(attributes) if attributes is not None else None,
        )
    )
    assert result.error is None and result.envelope is not None, result.error
    return result.envelope


class RaisingMasker:
    """A masker that fails, to prove that a failure never lets the envelope through."""

    def mask_map(self, mapping: Any) -> Any:
        raise RuntimeError("boom with secret hunter2secret")

    def mask_value(self, field_name: str, value: Any) -> Any:
        raise RuntimeError("boom")


class SpyingSinkTest(unittest.TestCase):
    def run_chain(self, sink, envelope: Envelope, service: MaskingService) -> None:
        """The test-local chain of T-03.9: the builder's envelope goes to the sink only through mask."""
        result = service.mask(envelope)
        if result.envelope is not None:
            sink.send([result.envelope])

    def test_no_secret_reaches_any_sink_while_the_rest_of_the_text_survives(self):
        for sink_class in SINKS:
            for kind, secret in SECRETS.items():
                with self.subTest(sink=sink_class.__name__, secret=kind):
                    sink = sink_class()
                    envelope = build(
                        "before " + secret + " after",
                        attributes={"note": "attr " + secret, "tags": [secret, "plain"], "api_key": "k"},
                        context=CorrelationContext(correlation_id="password=hunter2secret"),
                    )
                    self.assertIn(LEAKS[kind], canonical_json(envelope).decode("utf-8"))
                    self.run_chain(sink, envelope, make_service())
                    self.assertEqual(sink.calls, 1)
                    sent = canonical_json(sink.batches[0][0]).decode("utf-8")
                    self.assertNotIn(LEAKS[kind], sent)
                    self.assertNotIn("hunter2secret", sent)
                    self.assertIn("before ", sent)
                    self.assertIn("plain", sent)
                    self.assertIn(MASKED_TEXT, sent)

    def test_a_masker_that_raises_gives_an_error_and_no_envelope(self):
        envelope = build("password=hunter2secret")
        result = MaskingService(RaisingMasker()).mask(envelope)
        self.assertIsNone(result.envelope)
        self.assertIsInstance(result.error, EnvelopeMaskingFailedError)
        self.assertIsInstance(result.error.__cause__, RuntimeError)
        self.assertNotIn("hunter2secret", str(result.error))
        sink = FakeLogSink()
        self.run_chain(sink, envelope, MaskingService(RaisingMasker()))
        self.assertEqual(sink.calls, 0)

    def test_a_failure_in_the_context_is_also_an_error(self):
        class ContextFailingMasker(SensitiveDataMasker):
            def mask_value(self, field_name: str, value: Any) -> Any:
                raise RuntimeError("boom")

        envelope = build("m", context=CorrelationContext(correlation_id="abc"))
        result = MaskingService(ContextFailingMasker()).mask(envelope)
        self.assertIsNone(result.envelope)
        self.assertIsInstance(result.error, EnvelopeMaskingFailedError)

    def test_the_result_is_a_pair_of_which_exactly_one_is_set(self):
        good = make_service().mask(build("m"))
        self.assertIsNotNone(good.envelope)
        self.assertIsNone(good.error)
        self.assertEqual(MaskResult(), MaskResult(None, None))


class MaskingServiceTest(unittest.TestCase):
    def test_the_input_is_never_modified_and_the_result_is_a_copy(self):
        envelope = build(
            "password=hunter2secret",
            attributes={"api_key": "k", "tags": ["Bearer abcdef123456secret"]},
            context=CorrelationContext(correlation_id="token=abc12345678"),
        )
        before = copy.deepcopy(envelope)
        result = make_service().mask(envelope)
        self.assertEqual(envelope, before)
        masked = result.envelope
        self.assertIsNot(masked, envelope)
        self.assertIsNot(masked.data, envelope.data)
        self.assertIsNot(masked.attributes, envelope.attributes)
        self.assertIsNot(masked.context, envelope.context)
        self.assertIsNot(masked.resource, envelope.resource)
        masked.data["extra"] = 1
        masked.resource.host = "changed"
        self.assertEqual(envelope, before)

    def test_only_data_attributes_and_context_change(self):
        envelope = build("password=hunter2secret", attributes={"a": "b"}, context=CorrelationContext(trace_id="a" * 32))
        masked = make_service().mask(envelope).envelope
        for name in (
            "event_id",
            "event_type",
            "occurred_at",
            "recorded_at",
            "severity",
            "schema_version",
            "resource",
            "actor",
        ):
            self.assertEqual(getattr(masked, name), getattr(envelope, name), name)
        self.assertNotEqual(masked.data, envelope.data)
        self.assertEqual(masked.context.trace_id, "a" * 32)

    def test_null_stays_null_and_empty_stays_empty(self):
        envelope = build("m")
        self.assertIsNone(envelope.attributes)
        self.assertIsNone(envelope.context)
        masked = make_service().mask(envelope).envelope
        self.assertIsNone(masked.attributes)
        self.assertIsNone(masked.context)
        envelope.attributes = {}
        self.assertEqual(make_service().mask(envelope).envelope.attributes, {})

    def test_a_context_with_some_fields_keeps_the_absent_ones_absent(self):
        envelope = build("m", context=CorrelationContext(execution_id="run-1", correlation_id="Bearer abcdef123456"))
        context = make_service().mask(envelope).envelope.context
        self.assertEqual(context.execution_id, "run-1")
        self.assertEqual(context.correlation_id, "Bearer *****MASKED*****")
        self.assertIsNone(context.trace_id)
        self.assertIsNone(context.span_id)

    def test_the_actor_is_kept(self):
        envelope = build("m")
        envelope.actor = Actor(id="device-1", kind=ActorKind.DEVICE)
        masked = make_service().mask(envelope).envelope
        self.assertEqual(masked.actor, envelope.actor)
        self.assertIsNot(masked.actor, envelope.actor)

    def test_masking_twice_changes_nothing(self):
        service = make_service(extra_patterns=["order-[0-9]+"])
        envelope = build(
            "password=hunter2secret order-123",
            attributes={"api_key": "k", "note": "Bearer abcdef123456secret"},
            context=CorrelationContext(correlation_id="token=abc12345678"),
        )
        once = service.mask(envelope).envelope
        twice = service.mask(once).envelope
        self.assertEqual(canonical_json(once), canonical_json(twice))

    def test_the_class_rules_apply_to_data_and_attributes(self):
        service = make_service(
            field_classes={"patient_name": "pii", "email": "pii2", "body": "internal"},
            class_rules={"pii": "drop", "pii2": "redact", "internal": "allow"},
        )
        envelope = build(
            "m",
            attributes={"PatientName": "Ann", "email": "a@b.c", "body": "visible"},
        )
        envelope.data.update({"patient_name": "Bob", "email": "b@c.d", "body": "visible text"})
        masked = service.mask(envelope).envelope
        self.assertEqual(masked.attributes, {"email": MASKED_TEXT, "body": "visible"})
        self.assertNotIn("patient_name", masked.data)
        self.assertEqual(masked.data["email"], MASKED_TEXT)
        self.assertEqual(masked.data["body"], "visible text")

    def test_many_threads_mask_at_the_same_time(self):
        service = make_service()
        envelope = build("password=hunter2secret", attributes={"api_key": "k"})
        results: list = []
        lock = threading.Lock()

        def work() -> None:
            for _ in range(50):
                result = service.mask(envelope)
                with lock:
                    results.append(canonical_json(result.envelope))

        threads = [threading.Thread(target=work) for _ in range(8)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.assertEqual(len(results), 400)
        self.assertEqual(len(set(results)), 1)
        self.assertNotIn(b"hunter2secret", results[0])


if __name__ == "__main__":
    unittest.main()
