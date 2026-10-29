"""Black-box tests of the fakes: they record calls, can be told to fail or to delay, and have sane defaults."""

import dataclasses
import datetime
import time
import unittest

from internal.tests.fakes import (
    FakeAiTelemetrySink,
    FakeClock,
    FakeConfiguration,
    FakeDeviceTelemetrySink,
    FakeEventSink,
    FakeIdGenerator,
    FakeLocalSpool,
    FakeLogSink,
    FakeMetricsSink,
    FakeTraceSink,
)
from telemetry.internal.application.dtos.clock_info import ClockInfo
from telemetry.internal.application.dtos.configuration import Configuration
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.dtos.spool_cleanup import SpoolCleanup
from telemetry.internal.application.dtos.spool_record import SpoolRecord
from telemetry.internal.application.dtos.spool_usage import SpoolUsage
from telemetry.internal.domain.base_value_objects.identifier import Identifier
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.envelope_generated import Envelope, Resource
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.domain.value_objects.resource_identity import ResourceIdentity

SINK_FAKES = (
    FakeLogSink,
    FakeEventSink,
    FakeMetricsSink,
    FakeTraceSink,
    FakeDeviceTelemetrySink,
    FakeAiTelemetrySink,
)


def build_event(number: int = 1) -> Envelope:
    """A small envelope; the fakes never look inside."""
    return Envelope(
        data={"n": number},
        event_id=f"01900000-0000-7000-8000-{number:012x}",
        event_type="log.entry.recorded",
        occurred_at="2026-10-28T10:00:00.000000Z",
        recorded_at="2026-10-28T10:00:00.000000Z",
        resource=Resource(instance_id="instance", service_name="service"),
        schema_version="1.0.0",
        severity=Severity.INFO,
    )


class FakeSinkTest(unittest.TestCase):
    """The six sink fakes behave the same."""

    def test_default_accepts_every_event_and_records_the_batch(self) -> None:
        for fake_class in SINK_FAKES:
            with self.subTest(fake=fake_class.__name__):
                sink = fake_class()
                result = sink.send([build_event(1), build_event(2)])
                self.assertEqual(SendResult(accepted=2, dropped=0, failed=0), result)
                self.assertEqual("", result.reason)
                self.assertEqual(1, sink.calls)
                self.assertEqual([[build_event(1), build_event(2)]], sink.batches)

    def test_an_empty_batch_is_safe(self) -> None:
        for fake_class in SINK_FAKES:
            with self.subTest(fake=fake_class.__name__):
                sink = fake_class()
                self.assertEqual(SendResult(0, 0, 0), sink.send([]))
                self.assertEqual([[]], sink.batches)

    def test_the_failure_is_in_the_result_not_raised(self) -> None:
        failed = SendResult(accepted=0, dropped=0, failed=1, reason="down")
        for fake_class in SINK_FAKES:
            with self.subTest(fake=fake_class.__name__):
                sink = fake_class()
                sink.set_result(failed)
                self.assertEqual(failed, sink.send([build_event()]))
                self.assertEqual(1, sink.calls)

    def test_the_delay_is_honoured(self) -> None:
        for fake_class in SINK_FAKES:
            with self.subTest(fake=fake_class.__name__):
                sink = fake_class()
                sink.set_delay(0.005)
                started = time.perf_counter()
                sink.send([build_event()])
                self.assertGreaterEqual(time.perf_counter() - started, 0.004)

    def test_recorded_batches_are_copies(self) -> None:
        sink = FakeLogSink()
        batch = [build_event(1)]
        sink.send(batch)
        batch.append(build_event(2))
        self.assertEqual([[build_event(1)]], sink.batches)
        sink.batches[0].append(build_event(3))
        self.assertEqual([[build_event(1)]], sink.batches)


class FakeClockTest(unittest.TestCase):
    """FakeClock: settable time, a step per read, settable info."""

    def test_defaults_and_counts(self) -> None:
        clock = FakeClock()
        self.assertEqual(datetime.timezone.utc, clock.now().value.tzinfo)
        self.assertEqual(ClockInfo(), clock.clock_info())
        self.assertIsNone(clock.clock_info().source)
        self.assertIsNone(clock.clock_info().synced)
        self.assertEqual((1, 3, 0), (clock.now_calls, clock.clock_info_calls, clock.monotonic_calls))

    def test_wall_time_is_settable(self) -> None:
        moment = datetime.datetime(2030, 1, 2, 3, 4, 5, tzinfo=datetime.timezone.utc)
        clock = FakeClock()
        clock.set_wall_time(moment)
        self.assertEqual(moment, clock.now().value)
        self.assertEqual(moment, FakeClock(wall_time=moment).now().value)

    def test_the_monotonic_reading_never_goes_back_and_advances_by_the_step(self) -> None:
        clock = FakeClock(step_nanoseconds=10)
        readings = [clock.monotonic_nanoseconds() for _ in range(5)]
        self.assertEqual(sorted(readings), readings)
        self.assertEqual(40, readings[-1] - readings[0])
        clock.set_step(7)
        self.assertEqual(7, clock.monotonic_nanoseconds() - readings[-1])
        self.assertEqual(6, clock.monotonic_calls)

    def test_clock_info_is_settable(self) -> None:
        clock = FakeClock()
        clock.set_clock_info(ClockInfo(source="ntp", synced=True))
        self.assertEqual(ClockInfo(source="ntp", synced=True), clock.clock_info())


class FakeIdGeneratorTest(unittest.TestCase):
    """FakeIdGenerator: queue first, then a deterministic counter."""

    def test_generated_ids_are_valid_and_deterministic(self) -> None:
        generator = FakeIdGenerator()
        first, second = generator.new_event_id(), generator.new_event_id()
        self.assertEqual("01900000-0000-7000-8000-000000000001", first.value)
        self.assertEqual("01900000-0000-7000-8000-000000000002", second.value)
        self.assertEqual(2, generator.event_id_calls)
        self.assertEqual(first, FakeIdGenerator().new_event_id())

    def test_the_queue_comes_first(self) -> None:
        queued = EventId("01900000-0000-7000-8000-0000000000ff")
        extra = EventId("01900000-0000-7000-8000-0000000000fe")
        generator = FakeIdGenerator([queued])
        generator.queue_event_id(extra)
        self.assertEqual([queued, extra], [generator.new_event_id(), generator.new_event_id()])
        self.assertEqual("01900000-0000-7000-8000-000000000001", generator.new_event_id().value)

    def test_the_instance_id_is_deterministic(self) -> None:
        generator = FakeIdGenerator(instance_id="instance-7")
        self.assertEqual(Identifier("instance-7"), generator.new_instance_id())
        self.assertEqual(1, generator.instance_id_calls)


class FakeConfigurationTest(unittest.TestCase):
    """FakeConfiguration: a valid default, settable, counted."""

    def test_default_set_and_count(self) -> None:
        fake = FakeConfiguration()
        self.assertEqual("fake-service", fake.configuration().resource.service_name)
        custom = Configuration(resource=ResourceIdentity(service_name="other", instance_id="i"))
        fake.set_configuration(custom)
        self.assertEqual(custom, fake.configuration())
        self.assertEqual(2, fake.calls)
        self.assertEqual(custom, FakeConfiguration(custom).configuration())


class FakeLocalSpoolTest(unittest.TestCase):
    """FakeLocalSpool: append, read, acknowledge, usage, clean-up and failure."""

    def test_append_then_read_in_order_with_ids(self) -> None:
        spool = FakeLocalSpool()
        result = spool.append([build_event(1), build_event(2), build_event(3)])
        self.assertEqual(SendResult(3, 0, 0), result)
        records = spool.read_batch(2)
        self.assertEqual(["record-1", "record-2"], [record.id for record in records])
        self.assertEqual(build_event(1), records[0].event)
        self.assertIsInstance(records[0], SpoolRecord)
        self.assertEqual(3, len(spool.read_batch(10)))
        self.assertEqual([], spool.read_batch(0))

    def test_acknowledge_removes_from_the_reads_and_cleanup_frees_them(self) -> None:
        spool = FakeLocalSpool(quota_bytes=500)
        spool.append([build_event(1), build_event(2)])
        usage_before = spool.usage()
        self.assertEqual(SpoolUsage, type(usage_before))
        self.assertEqual((2, 500), (usage_before.events, usage_before.quota_bytes))
        self.assertGreater(usage_before.bytes, 0)
        spool.acknowledge(["record-1"])
        self.assertEqual(["record-2"], [record.id for record in spool.read_batch(10)])
        self.assertEqual(1, spool.usage().events)
        cleanup = spool.cleanup()
        self.assertEqual(SpoolCleanup, type(cleanup))
        self.assertEqual(1, cleanup.removed_events)
        self.assertGreater(cleanup.freed_bytes, 0)
        self.assertEqual(SpoolCleanup(0, 0), spool.cleanup())
        spool.append([build_event(3)])
        self.assertEqual(["record-2", "record-3"], [record.id for record in spool.read_batch(10)])

    def test_empty_sequences_are_safe(self) -> None:
        spool = FakeLocalSpool()
        self.assertEqual(SendResult(0, 0, 0), spool.append([]))
        spool.acknowledge([])
        self.assertEqual([], spool.read_batch(5))
        self.assertEqual(SpoolUsage(events=0, bytes=0, quota_bytes=1_000_000), spool.usage())
        self.assertEqual((1, 1, 1), (spool.append_calls, spool.read_calls, spool.acknowledge_calls))

    def test_it_can_be_told_to_fail(self) -> None:
        spool = FakeLocalSpool()
        spool.fail_with(OSError("disk full"))
        with self.assertRaises(OSError):
            spool.append([build_event()])
        with self.assertRaises(OSError):
            spool.read_batch(1)
        with self.assertRaises(OSError):
            spool.acknowledge(["record-1"])
        with self.assertRaises(OSError):
            spool.cleanup()
        spool.usage()
        spool.fail_with(None)
        self.assertEqual(1, spool.append([build_event()]).accepted)
        self.assertEqual(1, spool.append_calls - 1)

    def test_append_result_can_be_overridden_and_restored(self) -> None:
        spool = FakeLocalSpool()
        dropped = SendResult(accepted=0, dropped=1, failed=0, reason="quota")
        spool.set_append_result(dropped)
        self.assertEqual(dropped, spool.append([build_event()]))
        self.assertEqual([], spool.read_batch(10))
        spool.set_append_result(None)
        self.assertEqual(1, spool.append([build_event()]).accepted)


class DtoTest(unittest.TestCase):
    """The DTOs are frozen plain data."""

    def test_the_dtos_are_frozen(self) -> None:
        for dto in (SendResult(1, 0, 0), ClockInfo(), SpoolUsage(1, 2, 3), SpoolCleanup(1, 2)):
            with self.subTest(dto=type(dto).__name__):
                first_field = dataclasses.fields(dto)[0].name
                with self.assertRaises(dataclasses.FrozenInstanceError):
                    setattr(dto, first_field, 9)


if __name__ == "__main__":
    unittest.main()
