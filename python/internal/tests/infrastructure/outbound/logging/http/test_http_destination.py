"""Black-box tests of the HTTP destination against local servers on 127.0.0.1 (REQ-OUT-002)."""

import dataclasses
import http.client
import io
import json
import threading
import time
import unittest
from typing import Any

import telemetry
from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.eventually import eventually, stop_workers_on_cleanup
from internal.tests.fakes import FakeClock, FakeIdGenerator
from internal.tests.fakes.fake_receiver import Answer, Receiver
from internal.tests.infrastructure.config.helpers import merged_config
from internal.tests.infrastructure.outbound.shared.http_client.server_helpers import Reply, StubServer, free_port
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.http.helpers.result_from_outcome import result_from_outcome
from telemetry.internal.infrastructure.outbound.logging.http.http_destination import Delivery, HttpDestination
from telemetry.internal.infrastructure.outbound.logging.http.settings import Settings, validate_settings
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.classify_response import (
    CLASS_PERMANENT,
    CLASS_SUCCESS,
    CLASS_TRANSIENT,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.http_client import HttpClient, Outcome
from telemetry.logging import console
from telemetry.logging import http as public_http

SECRET = "placeholder-secret-not-real"


def event(severity: str = "INFO", message: str = "started", data_extra: Any = None):
    """A log entry envelope; ``data_extra`` is put in the data (an object that cannot be encoded breaks the body)."""
    data: dict[str, Any] = {"category": "http", "message": message}
    if data_extra is not None:
        data["extra"] = data_extra
    return envelope_from_json(
        {
            "schema_version": "0.1",
            "event_id": "01900000-0000-7000-8000-000000000001",
            "event_type": "log.entry.recorded",
            "occurred_at": "2026-10-28T10:00:00.123456Z",
            "recorded_at": "2026-10-28T10:00:00.123456Z",
            "severity": severity,
            "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
            "data": data,
        }
    )


def counts(result: SendResult) -> tuple[int, int, int, str]:
    return result.accepted, result.dropped, result.failed, result.reason


class ResultFromOutcomeTest(unittest.TestCase):
    def test_every_class_maps_to_its_result(self):
        table = [
            (Outcome(CLASS_SUCCESS, 200), 3, 0, (3, 0, 0, "")),
            (Outcome(CLASS_SUCCESS, 200), 2, 4, (6, 0, 0, "")),
            (Outcome(CLASS_TRANSIENT, 503, None, "status_503"), 3, 0, (0, 0, 3, "status_503")),
            (Outcome(CLASS_TRANSIENT, 0, None, "timeout"), 2, 1, (1, 0, 2, "timeout")),
            (Outcome(CLASS_PERMANENT, 400, None, "status_400"), 3, 0, (0, 3, 0, "status_400")),
            (Outcome(CLASS_PERMANENT, 0, None, "batch_too_large"), 1, 5, (5, 1, 0, "batch_too_large")),
        ]
        for outcome, sent, filtered, expected in table:
            with self.subTest(outcome=outcome, sent=sent, filtered=filtered):
                self.assertEqual(counts(result_from_outcome(outcome, sent, filtered)), expected)


class SettingsTest(unittest.TestCase):
    def test_the_client_settings_carry_the_eleven_fields(self):
        settings = Settings(url="https://logs.example.invalid/x", compress=True, minimum_severity="ERROR")
        client_settings = settings.client_settings()
        expected = {key: value for key, value in dataclasses.asdict(settings).items() if key != "minimum_severity"}
        self.assertEqual(dataclasses.asdict(client_settings), expected)
        self.assertEqual(len(expected), 11)

    def test_a_wrong_type_is_a_problem_and_never_an_exception(self):
        for value in (5, None, "info", b"INFO", ["INFO"]):
            with self.subTest(value=value):
                problems = validate_settings(Settings(url="https://logs.example.invalid/x", minimum_severity=value))
                self.assertEqual([problem.field for problem in problems], ["minimum_severity"])
                self.assertNotIn("info", problems[0].reason)

    def test_the_problems_of_the_client_come_first_with_fixed_reasons(self):
        settings = Settings(url="", minimum_severity="LOUD", bearer_token=SECRET, api_key_value=SECRET)
        problems = validate_settings(settings)
        self.assertEqual(problems[-1].field, "minimum_severity")
        self.assertEqual(problems[0].field, "url")
        for problem in problems:
            self.assertNotIn(SECRET, repr(problem))
            self.assertNotIn("LOUD", problem.reason)


class DeliveryTest(unittest.TestCase):
    def test_it_satisfies_both_sink_ports(self):
        destination = HttpDestination(Settings(url="https://logs.example.invalid/x"))
        self.assertIsInstance(destination, LogSinkPort)
        self.assertIsInstance(destination, EventSinkPort)

    def test_deliver_returns_the_outcome_and_the_counts(self):
        with StubServer(lambda received: Reply(503)) as server:
            delivery = HttpDestination(Settings(url=server.url(), minimum_severity="WARN")).deliver(
                [event("INFO"), event("WARN"), event("ERROR")]
            )
        self.assertEqual(delivery.outcome, Outcome(CLASS_TRANSIENT, 503, None, "status_503"))
        self.assertEqual((delivery.sent, delivery.filtered), (2, 1))
        self.assertEqual(counts(delivery.result), (1, 0, 2, "status_503"))
        with self.assertRaises(dataclasses.FrozenInstanceError):
            delivery.sent = 5  # type: ignore[misc]

    def test_a_success_has_the_outcome_and_a_rejection_drops(self):
        with StubServer(lambda received: Reply(204)) as server:
            delivery = HttpDestination(Settings(url=server.url())).deliver([event()])
        self.assertEqual(delivery.outcome, Outcome(CLASS_SUCCESS, 204))
        with StubServer(lambda received: Reply(400)) as server:
            delivery = HttpDestination(Settings(url=server.url())).deliver([event()])
        self.assertEqual(counts(delivery.result), (0, 1, 0, "status_400"))

    def test_when_nothing_is_left_no_request_is_made(self):
        with StubServer() as server:
            destination = HttpDestination(Settings(url=server.url(), minimum_severity="CRITICAL"))
            delivery = destination.deliver([event("INFO"), event("ERROR")])
            empty = destination.deliver([])
            self.assertEqual(server.requests, [])
        self.assertEqual(delivery, Delivery(SendResult(2, 0, 0), None, 0, 2))
        self.assertEqual(empty, Delivery(SendResult(0, 0, 0), None, 0, 0))

    def test_the_events_are_sent_in_one_request(self):
        with StubServer() as server:
            HttpDestination(Settings(url=server.url())).send([event(message="a"), event(message="b")])
            self.assertEqual(len(server.requests), 1)
            body = json.loads(server.requests[0].body)
        self.assertEqual([item["data"]["message"] for item in body["events"]], ["a", "b"])

    def test_a_closed_port_fails_the_events_with_network_error(self):
        result = HttpDestination(Settings(url=f"http://127.0.0.1:{free_port()}/ingest")).send([event(), event()])
        self.assertEqual(counts(result), (0, 0, 2, "network_error"))

    def test_a_server_that_answers_late_fails_the_events_with_timeout(self):
        with StubServer(lambda received: Reply(200, delay_seconds=1.5)) as server:
            destination = HttpDestination(Settings(url=server.url(), timeout_ms=200))
            started = time.monotonic()
            result = destination.send([event(), event(), event()])
            elapsed = time.monotonic() - started
        self.assertEqual(counts(result), (0, 0, 3, "timeout"))
        self.assertLess(elapsed, 3.0)

    def test_a_batch_larger_than_the_limit_is_dropped_and_nothing_is_sent(self):
        with StubServer() as server:
            destination = HttpDestination(Settings(url=server.url()), Limits(max_event_bytes=100, max_batch_bytes=100))
            result = destination.send([event(), event()])
            self.assertEqual(server.requests, [])
        self.assertEqual(counts(result), (0, 2, 0, "batch_too_large"))

    def test_next_delay_is_the_one_of_the_shared_client(self):
        settings = Settings(url="https://logs.example.invalid/x", max_attempts=3)
        destination = HttpDestination(settings)
        client = HttpClient(settings.client_settings(), Limits())
        outcomes = [
            Outcome(CLASS_TRANSIENT, 503, None, "status_503"),
            Outcome(CLASS_TRANSIENT, 429, 7000, "status_429"),
            Outcome(CLASS_PERMANENT, 400, None, "status_400"),
        ]
        for outcome in outcomes:
            for attempt in (0, 1, 2):
                with self.subTest(outcome=outcome, attempt=attempt):
                    self.assertEqual(
                        destination.next_delay(attempt, outcome, 0.5), client.next_delay(attempt, outcome, 0.5)
                    )
        self.assertFalse(destination.next_delay(0, outcomes[2], 0.5)[1])
        self.assertFalse(destination.next_delay(2, outcomes[0], 0.5)[1])
        self.assertTrue(destination.next_delay(0, outcomes[0], 0.5)[1])

    def test_the_standard_library_http_is_not_shadowed(self):
        self.assertTrue(hasattr(http.client, "HTTPConnection"))
        self.assertEqual(public_http.__name__, "telemetry.logging.http")


class RecoveryTest(unittest.TestCase):
    def setUp(self):
        self.destination = HttpDestination(Settings(url=f"http://127.0.0.1:{free_port()}/ingest"))

    def test_an_object_that_is_not_an_event_fails_the_events_with_destination_failed(self):
        result = self.destination.send([object(), object()])  # type: ignore[list-item]
        self.assertEqual(counts(result), (0, 0, 2, "destination_failed"))

    def test_an_event_the_encoder_cannot_serialize_is_dropped_as_serialization_failed(self):
        result = self.destination.send([event(data_extra=object())])
        self.assertEqual(counts(result), (0, 1, 0, "serialization_failed"))

    def test_a_sequence_that_breaks_while_it_is_read_fails_what_it_can_count(self):
        class Broken:
            def __len__(self) -> int:
                return 4

            def __iter__(self):
                raise RuntimeError("broken iterator")

        class Unmeasurable(Broken):
            def __len__(self) -> int:
                raise RuntimeError("no length")

        self.assertEqual(counts(self.destination.send(Broken())), (0, 0, 4, "destination_failed"))  # type: ignore
        self.assertEqual(counts(self.destination.send(Unmeasurable())), (0, 0, 0, "destination_failed"))  # type: ignore

    def test_keyboard_interrupt_propagates(self):
        class Interrupting:
            def __iter__(self):
                raise KeyboardInterrupt()

        with self.assertRaises(KeyboardInterrupt):
            self.destination.send(Interrupting())  # type: ignore[arg-type]


class SecretsTest(unittest.TestCase):
    def assert_clean(self, *things: object) -> None:
        for thing in things:
            text = repr(thing) + str(thing)
            self.assertNotIn(SECRET, text)
            self.assertNotIn("token=", text)

    def destination(self, url: str, **overrides: Any) -> HttpDestination:
        return HttpDestination(Settings(url=url, bearer_token=SECRET, **overrides))

    def test_no_failing_path_shows_the_token_or_the_query(self):
        query = f"/ingest?token={SECRET}"
        paths = {
            "closed port": lambda: self.destination(f"http://127.0.0.1:{free_port()}{query}").deliver([event()]),
        }
        for status in (503, 429, 400, 302):
            with StubServer(lambda received, status=status: Reply(status, {"Location": f"/other?token={SECRET}"})) as s:
                delivery = self.destination(s.url(query)).deliver([event()])
                self.assert_clean(delivery, delivery.result, delivery.outcome)
        for name, run in paths.items():
            with self.subTest(path=name):
                delivery = run()
                self.assert_clean(delivery, delivery.result, delivery.outcome)
        with StubServer(lambda received: Reply(200, delay_seconds=1.0)) as server:
            delivery = self.destination(server.url(query), timeout_ms=200).deliver([event()])
            self.assert_clean(delivery)
        with StubServer() as server:
            limits = Limits(max_event_bytes=100, max_batch_bytes=100)
            delivery = HttpDestination(Settings(url=server.url(query), bearer_token=SECRET), limits).deliver([event()])
            self.assert_clean(delivery)

    def test_a_refused_configuration_does_not_show_the_secret_or_the_query(self):
        for overrides in (
            {"url": f"http://logs.example.invalid/ingest?token={SECRET}", "bearer_token": SECRET},
            {
                "url": f"https://logs.example.invalid/ingest?token={SECRET}",
                "bearer_token": SECRET,
                "api_key_value": SECRET,
            },
            {
                "url": f"https://logs.example.invalid/ingest?token={SECRET}",
                "api_key_header": "bad header",
                "minimum_severity": SECRET,
            },
        ):
            with self.subTest(fields=sorted(overrides)):
                with self.assertRaises(ConfigurationError) as raised:
                    HttpDestination(Settings(**overrides))
                self.assert_clean(raised.exception, raised.exception.problems)


class ConcurrencyTest(unittest.TestCase):
    def test_sixteen_threads_share_one_destination(self):
        with StubServer() as server:
            destination = HttpDestination(Settings(url=server.url()))
            results: list[SendResult] = []
            lock = threading.Lock()

            def work() -> None:
                for _ in range(3):
                    result = destination.send([event(), event("DEBUG")])
                    with lock:
                        results.append(result)

            threads = [threading.Thread(target=work) for _ in range(16)]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join(timeout=60)
            self.assertEqual(len(server.requests), 48)
        self.assertEqual(len(results), 48)
        self.assertTrue(all(counts(result) == (2, 0, 0, "") for result in results))


class PublicModuleTest(unittest.TestCase):
    def test_the_public_functions_build_the_destination(self):
        self.assertEqual(public_http.default_settings(), Settings())
        self.assertIs(public_http.Destination, HttpDestination)
        self.assertIsInstance(
            public_http.new_destination(Settings(url="https://logs.example.invalid/x")), HttpDestination
        )
        with StubServer() as server:
            destination = public_http.new_destination_with_limits(
                Settings(url=server.url()), telemetry.Limits(max_event_bytes=100, max_batch_bytes=100)
            )
            self.assertEqual(counts(destination.send([event()])), (0, 1, 0, "batch_too_large"))
        with self.assertRaises(ConfigurationError) as raised:
            public_http.new_destination(public_http.default_settings())
        self.assertEqual([problem.field for problem in raised.exception.problems], ["url"])

    def test_limits_are_reachable_from_the_public_package(self):
        self.assertIs(telemetry.Limits, Limits)
        self.assertIn("Limits", telemetry.__all__)


class TwoDestinationsTest(unittest.TestCase):
    """The acceptance criterion: a failing HTTP destination does not stop the console one in the same run."""

    def run_core(self, url: str, receiver: Any = None):
        stream = io.StringIO()
        sink_http = public_http.new_destination(public_http.Settings(url=url))
        sink_console = console.new_destination_with_writer(console.Settings(), stream)
        ids = [EventId("01900000-0000-7000-8000-000000000001"), EventId("01900000-0000-7000-8000-000000000002")]
        core = stop_workers_on_cleanup(
            self,
            telemetry.new(
                merged_config(
                    "microservice", {"resource": {"service_name": "sensor-hub"}, "delivery": {"flush_interval_ms": 5}}
                ),
                telemetry.with_log_sink(sink_http),
                telemetry.with_log_sink(sink_console),
                telemetry.with_clock(FakeClock()),
                telemetry.with_id_generator(FakeIdGenerator(ids)),
            ),
        )
        core.logger.info(telemetry.LogEntry(category="http", message="first"))
        # delivery is asynchronous: wait for the first event to leave, so the two events travel one by one
        eventually(5.0, lambda: stream.getvalue().count(chr(10)) >= 1, "the first console line")
        if receiver is not None:
            eventually(5.0, lambda: len(receiver.requests()) >= 1, "the first http request")
        result = core.emit(
            telemetry.NewEventInput(
                event_type="log.entry.recorded",
                severity=telemetry.Severity.INFO,
                data={"category": "http", "message": "second"},
            )
        )
        eventually(5.0, lambda: stream.getvalue().count(chr(10)) >= 2, "the second console line")
        if receiver is not None:
            eventually(5.0, lambda: len(receiver.requests()) >= 2, "the second http request")
        core.shutdown()
        return stream.getvalue(), result

    def test_a_receiver_that_answers_503_does_not_stop_the_console(self):
        with Receiver() as receiver:
            receiver.script(Answer(status=503), Answer(status=503))
            output, result = self.run_core(receiver.url(), receiver)
            received = receiver.requests()
        self.assertEqual(len(output.strip().split("\n")), 2)
        self.assertIn('"message":"first"', output)
        # delivery is asynchronous: the call says the event was queued for both destinations (the 503 is a counter)
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (2, 0, 0, ""))
        self.assertEqual(len(received), 2)
        self.assertEqual([item.answered_status for item in received], [503, 503])
        self.assertTrue(all(item.valid for item in received))
        self.assertEqual(json.loads(received[1].body)["schema_version"], "0.1")
        self.assertEqual(received[1].events[0]["data"]["message"], "second")
        self.assertIn('"message":"second"', output)

    def test_a_receiver_that_does_not_answer_does_not_stop_the_console(self):
        output, result = self.run_core(f"http://127.0.0.1:{free_port()}/ingest")
        self.assertEqual(len(output.strip().split("\n")), 2)
        self.assertEqual((result.accepted, result.dropped, result.failed, result.reason), (2, 0, 0, ""))


if __name__ == "__main__":
    unittest.main()
