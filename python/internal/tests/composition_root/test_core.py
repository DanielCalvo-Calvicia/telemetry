"""Black-box tests of the core built by ``telemetry.new``: isolation, concurrency, start errors and the options."""

import asyncio
import dataclasses
import threading
import time
import unittest
from typing import Any

import telemetry
from internal.tests.eventually import eventually, stop_workers_on_cleanup, with_short_flush
from internal.tests.fakes import BlockingLogSink, FakeClock, FakeIdGenerator, FakeLocalSpool, FakeLogSink
from telemetry.internal.application.errors import IdGenerationFailedError
from telemetry.internal.composition_root.config.options import Options
from telemetry.internal.composition_root.containers.core_container import Core
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json


def config_for(profile: Any = telemetry.microservice) -> telemetry.Config:
    """A valid configuration of a profile with a service name."""
    base = profile()
    resource = dataclasses.replace(base.resource, service_name="sensor-hub", instance_id="hub-01")
    spool = dataclasses.replace(base.spool, directory="spool-dir")
    return with_short_flush(dataclasses.replace(base, resource=resource, spool=spool))


def build(test: unittest.TestCase, config: telemetry.Config, *options: Any) -> Any:
    """Build the core with ``telemetry.new`` and stop its workers when the test ends."""
    return stop_workers_on_cleanup(test, telemetry.new(config, *options))


def received(sink: Any, count: int) -> None:
    """Wait until the fake sink has received ``count`` events: delivery is asynchronous."""
    eventually(5.0, lambda: sum(len(batch) for batch in sink.batches) >= count, f"{count} events at the sink")


def log_event(message: str = "started") -> telemetry.NewEventInput:
    return telemetry.NewEventInput(
        event_type="log.entry.recorded",
        severity=telemetry.Severity.INFO,
        data={"category": "http", "message": message},
    )


class NewCoreTest(unittest.TestCase):
    def test_the_core_has_a_logger_that_delivers_to_the_log_sinks(self):
        sink = FakeLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink))
        self.assertIsInstance(core.logger, telemetry.Logger)
        self.assertIs(core.logger, core.logger)
        core.logger.warn(telemetry.LogEntry(category="http", message="slow"))
        received(sink, 1)
        self.assertEqual(sink.batches[0][0].severity, telemetry.Severity.WARN)

    def test_new_returns_a_telemetry_that_delivers_one_event(self):
        sink = FakeLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink))
        self.assertIsInstance(core, telemetry.Telemetry)
        self.assertIsInstance(core, Core)
        result = core.emit(log_event())
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (1, 0, 0, ""))
        received(sink, 1)
        envelope = sink.batches[0][0]
        self.assertEqual(envelope.resource.service_name, "sensor-hub")
        self.assertEqual(envelope.resource.instance_id, "hub-01")

    def test_the_defaults_are_the_system_clock_and_the_uuid_v7_generator(self):
        sink = FakeLogSink()
        config = dataclasses.replace(config_for(), resource=telemetry.ResourceConfig(service_name="svc"))
        build(self, config, telemetry.with_log_sink(sink)).emit(log_event())
        received(sink, 1)
        envelope = sink.batches[0][0]
        self.assertRegex(envelope.event_id, r"^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$")
        self.assertRegex(envelope.resource.instance_id, r"^[0-9a-f]{8}-[0-9a-f]{4}-7")
        self.assertRegex(envelope.recorded_at, r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$")

    def test_a_core_without_sinks_drops_with_no_destination(self):
        result = build(self, config_for()).emit(log_event())
        self.assertEqual((result.dropped, result.reason), (1, "no_destination"))

    def test_shutdown_returns_the_zero_result(self):
        core = build(self, config_for(), telemetry.with_log_sink(FakeLogSink()))
        self.assertEqual(core.shutdown(), telemetry.ShutdownResult(sent=0, spooled=0, dropped=0))
        self.assertEqual(core.shutdown(), telemetry.ShutdownResult())

    def test_an_invalid_configuration_raises_with_every_problem_and_names_the_field(self):
        config = dataclasses.replace(
            telemetry.microservice(),
            delivery=dataclasses.replace(telemetry.microservice().delivery, queue_size=0),
        )
        with self.assertRaises(telemetry.ConfigurationError) as raised:
            build(self, config)
        fields = [problem.field for problem in raised.exception.problems]
        self.assertEqual(fields, ["resource.service_name", "delivery.queue_size"])
        self.assertIn("delivery.queue_size", str(raised.exception))

    def test_a_durable_mode_without_a_spool_is_refused_and_with_one_starts(self):
        with self.assertRaises(telemetry.ConfigurationError) as raised:
            build(self, config_for(telemetry.robotics))
        self.assertEqual([problem.field for problem in raised.exception.problems], ["delivery.mode"])
        self.assertEqual(raised.exception.problems[0].code, "config_field_invalid")
        core = build(self, config_for(telemetry.robotics), telemetry.with_local_spool(FakeLocalSpool()))
        self.assertIsInstance(core, Core)

    def test_a_durable_mode_without_a_spool_and_with_a_bad_configuration_reports_both_together(self):
        base = telemetry.offline_device()
        config = dataclasses.replace(base, resource=telemetry.ResourceConfig())  # no service name, no spool directory
        with self.assertRaises(telemetry.ConfigurationError) as raised:
            build(self, config)
        fields = [problem.field for problem in raised.exception.problems]
        self.assertEqual(fields, ["resource.service_name", "spool.directory", "delivery.mode"])

    def test_an_unknown_mode_does_not_add_the_spool_problem(self):
        base = config_for()
        config = dataclasses.replace(base, delivery=dataclasses.replace(base.delivery, mode="sometimes"))
        with self.assertRaises(telemetry.ConfigurationError) as raised:
            build(self, config)
        self.assertEqual([problem.field for problem in raised.exception.problems], ["delivery.mode"])
        self.assertEqual(raised.exception.problems[0].code, "delivery_mode_unknown")

    def test_a_failing_id_generator_at_start_raises_a_classified_error(self):
        generator = FakeIdGenerator()
        generator.new_instance_id = lambda: (_ for _ in ()).throw(RuntimeError("no entropy"))  # type: ignore[method-assign]
        config = dataclasses.replace(config_for(), resource=telemetry.ResourceConfig(service_name="svc"))
        with self.assertRaises(Exception) as raised:
            build(self, config, telemetry.with_id_generator(generator))
        self.assertEqual(raised.exception.code, "instance_id_unavailable")  # type: ignore[attr-defined]

    def test_a_failing_id_generator_while_emitting_drops_the_event_and_never_raises(self):
        generator = FakeIdGenerator()
        generator.fail_with(IdGenerationFailedError("no"))
        sink = FakeLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink), telemetry.with_id_generator(generator))
        result = core.emit(log_event())
        self.assertEqual((result.dropped, result.reason), (1, "event_build_failed"))
        time.sleep(0.03)
        self.assertEqual(sink.calls, 0)


class IsolationTest(unittest.TestCase):
    def test_two_cores_in_one_process_share_nothing(self):
        sink_a, sink_b = FakeLogSink(), FakeLogSink()
        core_a = build(
            self,
            config_for(),
            telemetry.with_log_sink(sink_a),
            telemetry.with_clock(FakeClock()),
            telemetry.with_id_generator(FakeIdGenerator()),
        )
        masking = telemetry.MaskingConfig(extra_patterns=("order-[0-9]+",))
        core_b = build(
            self,
            dataclasses.replace(config_for(), masking=masking),
            telemetry.with_log_sink(sink_b),
            telemetry.with_clock(FakeClock()),
            telemetry.with_id_generator(FakeIdGenerator()),
        )
        core_a.emit(log_event("order-123"))
        core_b.emit(log_event("order-123"))
        received(sink_a, 1)
        received(sink_b, 1)
        self.assertEqual((sink_a.calls, sink_b.calls), (1, 1))
        self.assertEqual(sink_a.batches[0][0].data["message"], "order-123")
        self.assertNotEqual(sink_b.batches[0][0].data["message"], "order-123")
        # the same id: each core has its own generator
        self.assertEqual(sink_a.batches[0][0].event_id, sink_b.batches[0][0].event_id)

    def test_a_sink_of_one_core_never_receives_the_events_of_another(self):
        sink_a, sink_b = FakeLogSink(), FakeLogSink()
        core_a = build(self, config_for(), telemetry.with_log_sink(sink_a))
        build(self, config_for(), telemetry.with_log_sink(sink_b))
        for _ in range(5):
            core_a.emit(log_event())
        received(sink_a, 5)
        time.sleep(0.03)
        self.assertEqual(sum(len(batch) for batch in sink_a.batches), 5)
        self.assertEqual(sink_b.calls, 0)

    def test_the_configuration_object_of_the_caller_is_not_shared(self):
        masking_patterns = ["order-[0-9]+"]
        config = dataclasses.replace(
            config_for(), masking=telemetry.MaskingConfig(extra_patterns=tuple(masking_patterns))
        )
        sink = FakeLogSink()
        core = build(self, config, telemetry.with_log_sink(sink))
        masking_patterns.clear()
        core.emit(log_event("order-123"))
        received(sink, 1)
        self.assertNotEqual(sink.batches[0][0].data["message"], "order-123")


class ConcurrencyTest(unittest.TestCase):
    def test_sixteen_threads_emit_on_one_core_and_the_counts_add_up(self):
        sink = FakeLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink))
        per_thread = 50
        totals: list = []
        lock = threading.Lock()

        def work(number: int) -> None:
            accepted = 0
            for index in range(per_thread):
                result = core.emit(log_event(f"thread {number} event {index}"))
                accepted += result.accepted
            with lock:
                totals.append(accepted)

        threads = [threading.Thread(target=work, args=(number,)) for number in range(16)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        self.assertEqual(sum(totals), 16 * per_thread)
        received(sink, 16 * per_thread)
        events = [envelope for batch in sink.batches for envelope in batch]
        self.assertEqual(len(events), 16 * per_thread)
        ids = {canonical_json(envelope) for envelope in events}
        self.assertEqual(len(ids), 16 * per_thread)

    def test_the_call_context_of_the_calling_thread_is_attached(self):
        sink = FakeLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink))

        def work(execution_id: str) -> None:
            created = telemetry.new_call_context(execution_id=execution_id)
            with telemetry.use_call_context(created.call_context):
                for _ in range(20):
                    core.emit(log_event(execution_id))

        threads = [threading.Thread(target=work, args=(f"run-{number}",)) for number in range(8)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()
        received(sink, 160)
        for batch in sink.batches:
            for envelope in batch:
                self.assertEqual(envelope.context.execution_id, envelope.data["message"])

    def test_without_a_call_context_the_event_has_no_context(self):
        sink = FakeLogSink()
        build(self, config_for(), telemetry.with_log_sink(sink)).emit(log_event())
        received(sink, 1)
        self.assertIsNone(sink.batches[0][0].context)


def delivery_workers() -> list[threading.Thread]:
    """The delivery worker threads that are alive now."""
    return [thread for thread in threading.enumerate() if thread.name.startswith("telemetry-delivery-")]


class DeliveryOfTheCoreTest(unittest.TestCase):
    def test_new_starts_one_daemon_worker_per_destination_and_nothing_else_does(self):
        before = len(delivery_workers())
        core = build(
            self, config_for(), telemetry.with_log_sink(FakeLogSink()), telemetry.with_event_sink(FakeLogSink())
        )
        workers = delivery_workers()
        self.assertEqual(len(workers), before + 2)
        self.assertTrue(all(worker.daemon for worker in workers))
        core._delivery.stop()
        self.assertEqual(len(delivery_workers()), before)

    def test_a_core_without_destinations_starts_no_worker(self):
        before = len(delivery_workers())
        build(self, config_for())
        self.assertEqual(len(delivery_workers()), before)

    def test_the_queue_and_batch_sizes_of_the_configuration_are_used(self):
        base = config_for()
        delivery = dataclasses.replace(base.delivery, queue_size=3, batch_size=3, flush_interval_ms=60_000)
        sink = BlockingLogSink()
        core = build(self, dataclasses.replace(base, delivery=delivery), telemetry.with_log_sink(sink))
        self.addCleanup(sink.release)  # runs before the stop of the workers
        results = [core.emit(log_event(f"event {number}")) for number in range(3)]
        self.assertTrue(sink.entered.wait(5.0))  # a full batch of 3 leaves at once, whatever the interval
        results += [core.emit(log_event(f"event {number}")) for number in range(3, 9)]
        reasons = [result.reason for result in results]
        self.assertEqual(reasons[:3], ["", "", ""])
        self.assertEqual(reasons[3:].count("queue_full"), 3)  # the queue holds 3 more while the worker is stuck
        self.assertEqual(sum(result.accepted for result in results), 6)

    def test_the_clock_of_the_options_does_not_decide_when_a_batch_leaves(self):
        sink = FakeLogSink()
        build(
            self, config_for(), telemetry.with_log_sink(sink), telemetry.with_clock(FakeClock(step_nanoseconds=0))
        ).emit(log_event())
        received(sink, 1)

    def test_the_core_runs_inside_asyncio_and_never_blocks_the_event_loop(self):
        sink = BlockingLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink))
        self.addCleanup(sink.release)  # runs before the stop of the workers

        async def main() -> tuple[float, float, int, bool]:
            loop_thread = threading.get_ident()
            core.logger.info(telemetry.LogEntry(category="http", message="stuck"))
            while not sink.entered.is_set():  # the worker takes the first event and the destination never answers
                await asyncio.sleep(0.001)
            started = time.monotonic()
            for number in range(200):
                core.logger.info(telemetry.LogEntry(category="http", message=f"queued {number}"))
            log_seconds = time.monotonic() - started
            ticks, slowest = 0, 0.0
            for _ in range(10):
                before = time.monotonic()
                await asyncio.sleep(0.01)
                slowest = max(slowest, time.monotonic() - before)
                ticks += 1
            worker_threads = [thread.ident for thread in delivery_workers()]
            return log_seconds, slowest, ticks, loop_thread not in worker_threads

        log_seconds, slowest, ticks, loop_is_not_a_worker = asyncio.run(main())
        self.assertLess(log_seconds, 1.0)  # 200 log calls with the destination blocked
        self.assertEqual(ticks, 10)
        self.assertLess(slowest, 0.5)  # the loop kept running while the worker held the blocked destination
        self.assertTrue(loop_is_not_a_worker)
        self.assertEqual(len(sink.batches), 1)

    def test_asyncio_tasks_log_while_a_destination_is_blocked(self):
        sink = BlockingLogSink()
        core = build(self, config_for(), telemetry.with_log_sink(sink))
        self.addCleanup(sink.release)

        async def task(number: int) -> int:
            accepted = 0
            for index in range(20):
                accepted += core.emit(log_event(f"task {number} event {index}")).accepted
                await asyncio.sleep(0)
            return accepted

        async def main() -> list[int]:
            return list(await asyncio.gather(*(task(number) for number in range(10))))

        self.assertEqual(asyncio.run(main()), [20] * 10)


class ConfigurationOfTheQueueTest(unittest.TestCase):
    def problems(self, **fields: int) -> list[tuple[str, str]]:
        base = telemetry.microservice()
        config = dataclasses.replace(
            base,
            resource=dataclasses.replace(base.resource, service_name="svc"),
            delivery=dataclasses.replace(base.delivery, **fields),
        )
        with self.assertRaises(telemetry.ConfigurationError) as raised:
            telemetry.new(config)
        return [(problem.field, problem.reason) for problem in raised.exception.problems]

    def test_a_queue_a_batch_or_a_flush_interval_of_zero_is_refused_with_the_field_and_the_reason(self):
        greater_than_zero = "it must be an integer greater than zero"
        self.assertEqual(self.problems(queue_size=0), [("delivery.queue_size", greater_than_zero)])
        self.assertEqual(self.problems(batch_size=0), [("delivery.batch_size", greater_than_zero)])
        self.assertEqual(self.problems(flush_interval_ms=0), [("delivery.flush_interval_ms", greater_than_zero)])
        self.assertEqual(self.problems(queue_size=-1), [("delivery.queue_size", greater_than_zero)])

    def test_a_batch_larger_than_the_queue_is_refused(self):
        self.assertEqual(
            self.problems(queue_size=10, batch_size=11),
            [("delivery.batch_size", "it must not be greater than the queue size")],
        )


class OptionsTest(unittest.TestCase):
    def apply(self, *options: Any) -> Options:
        applied = Options()
        for option in options:
            option(applied)
        return applied

    def test_each_sink_option_appends_and_keeps_the_order(self):
        first, second = FakeLogSink(), FakeLogSink()
        applied = self.apply(telemetry.with_log_sink(first), telemetry.with_log_sink(second))
        self.assertEqual(applied.log_sinks, [first, second])

    def test_each_kind_has_its_own_list(self):
        sink = FakeLogSink()
        applied = self.apply(
            telemetry.with_event_sink(sink),
            telemetry.with_metrics_sink(sink),
            telemetry.with_trace_sink(sink),
            telemetry.with_device_telemetry_sink(sink),
            telemetry.with_ai_telemetry_sink(sink),
        )
        self.assertEqual(applied.log_sinks, [])
        for kind in ("event_sinks", "metrics_sinks", "trace_sinks", "device_telemetry_sinks", "ai_telemetry_sinks"):
            self.assertEqual(getattr(applied, kind), [sink], kind)

    def test_none_is_ignored_by_every_option(self):
        options = (
            telemetry.with_log_sink(None),
            telemetry.with_event_sink(None),
            telemetry.with_metrics_sink(None),
            telemetry.with_trace_sink(None),
            telemetry.with_device_telemetry_sink(None),
            telemetry.with_ai_telemetry_sink(None),
            telemetry.with_local_spool(None),
            telemetry.with_clock(None),
            telemetry.with_id_generator(None),
        )
        self.assertEqual(self.apply(*options), Options())

    def test_the_single_adapters_are_set_and_the_last_wins(self):
        spool_one, spool_two = FakeLocalSpool(), FakeLocalSpool()
        clock_one, clock_two = FakeClock(), FakeClock()
        generator_one, generator_two = FakeIdGenerator(), FakeIdGenerator()
        applied = self.apply(
            telemetry.with_local_spool(spool_one),
            telemetry.with_local_spool(spool_two),
            telemetry.with_clock(clock_one),
            telemetry.with_clock(clock_two),
            telemetry.with_id_generator(generator_one),
            telemetry.with_id_generator(generator_two),
            telemetry.with_local_spool(None),
        )
        self.assertIs(applied.spool, spool_two)
        self.assertIs(applied.clock, clock_two)
        self.assertIs(applied.id_generator, generator_two)

    def test_two_options_objects_do_not_share_lists(self):
        self.assertIsNot(Options().log_sinks, Options().log_sinks)

    def test_several_destinations_of_one_kind_all_receive_through_new(self):
        sinks = [FakeLogSink(), FakeLogSink()]
        core = build(
            self, config_for(), *(telemetry.with_log_sink(sink) for sink in sinks), telemetry.with_log_sink(None)
        )
        self.assertEqual(core.emit(log_event()).accepted, 2)
        for sink in sinks:
            received(sink, 1)
        self.assertEqual([sink.calls for sink in sinks], [1, 1])


if __name__ == "__main__":
    unittest.main()
