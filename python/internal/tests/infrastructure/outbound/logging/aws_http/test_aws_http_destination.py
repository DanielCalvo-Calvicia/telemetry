"""Black-box tests of the AWS HTTP destination against local servers on 127.0.0.1 (T-04.6)."""

import dataclasses
import json
import threading
import unittest
from typing import Any

import telemetry
from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.eventually import eventually, stop_workers_on_cleanup
from internal.tests.fakes import FakeClock, FakeIdGenerator
from internal.tests.infrastructure.config.helpers import merged_config
from internal.tests.infrastructure.outbound.shared.http_client.server_helpers import Reply, StubServer, free_port
from telemetry.internal.application.dtos.send_result import SendResult
from telemetry.internal.application.ports.outbound.event_sink_port import EventSinkPort
from telemetry.internal.application.ports.outbound.log_sink_port import LogSinkPort
from telemetry.internal.domain.value_objects.enums_generated import Severity
from telemetry.internal.domain.value_objects.event_id import EventId
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.logging.aws_http.aws_http_destination import (
    AwsHttpDestination,
    Delivery,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.build_add_log_request import (
    build_add_log_request,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.build_endpoint import build_endpoint
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.map_level import map_level
from telemetry.internal.infrastructure.outbound.logging.aws_http.helpers.timestamp_milliseconds import (
    timestamp_milliseconds,
)
from telemetry.internal.infrastructure.outbound.logging.aws_http.settings import Settings, validate_settings
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.classify_response import (
    CLASS_PERMANENT,
    CLASS_TRANSIENT,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.http_client import HttpClient, Outcome
from telemetry.logging import aws_http as public_aws_http

SECRET = "placeholder-secret-not-real"
PATH = "/api/v1/cloudwatch/logs/add"


def settings_for(server_port: int, **overrides: Any) -> Settings:
    """Valid settings that point to a local server (or to a closed port)."""
    values: dict[str, Any] = {
        "host": "http://127.0.0.1",
        "port": str(server_port),
        "path": PATH,
        "log_group_name": "my-log-group",
        "log_stream_name": "my-log-stream",
    }
    values.update(overrides)
    return Settings(**values)


def valid_settings(**overrides: Any) -> Settings:
    return settings_for(8080, **{"host": "http://localhost", **overrides})


def event(
    event_type: str = "log.entry.recorded",
    message: str = "started",
    severity: str = "INFO",
    occurred_at: str = "2026-10-28T10:00:00.123456Z",
    attributes: Any = None,
):
    """An envelope; ``attributes`` may hold something that cannot be encoded to break the body."""
    value: dict[str, Any] = {
        "schema_version": "0.1",
        "event_id": "01900000-0000-7000-8000-000000000001",
        "event_type": event_type,
        "occurred_at": occurred_at,
        "recorded_at": "2026-10-28T10:00:00.123456Z",
        "severity": severity,
        "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
        "data": {"category": "http", "message": message},
    }
    if attributes is not None:
        value["attributes"] = attributes
    return envelope_from_json(value)


def counts(result: SendResult) -> tuple[int, int, int, str]:
    return result.accepted, result.dropped, result.failed, result.reason


def sent_messages(server: StubServer) -> list[str]:
    return [json.loads(json.loads(request.body)["message"])["Message"] for request in server.requests]


class HelpersTest(unittest.TestCase):
    def test_the_endpoint_is_host_port_and_path(self):
        self.assertEqual(
            build_endpoint("https://logs.example.invalid", "443", "/x/y"), "https://logs.example.invalid:443/x/y"
        )

    def test_critical_is_fatal_and_the_rest_keep_their_name(self):
        self.assertEqual(map_level(Severity.CRITICAL), "FATAL")
        self.assertEqual([map_level(item) for item in Severity if item != Severity.CRITICAL][:2], ["TRACE", "DEBUG"])

    def test_the_timestamp_is_cut_with_integer_arithmetic(self):
        table = {
            "1970-01-01T00:00:00Z": 0,
            "1970-01-01T00:00:00.9Z": 900,
            "1970-01-01T00:00:00.0019999Z": 1,
            "2026-10-28T10:00:00.999999Z": 1793181600999,
            "2024-02-29T23:59:59.999Z": 1709251199999,
            "1969-12-31T23:59:59.500Z": -500,
        }
        for text, expected in table.items():
            with self.subTest(text=text):
                self.assertEqual(timestamp_milliseconds(text), expected)

    def test_a_text_that_is_not_a_utc_time_raises(self):
        for text in (
            "",
            "2026-10-28 10:00:00Z",
            "2026-10-28T10:00:00+01:00",
            "2026-10-28T10:00:00",
            "2026-10-28T10:00:00Z\n",
            "2026-13-28T10:00:00Z",
            "2026-02-30T10:00:00Z",
            "2026-10-28T24:00:00Z",
            "2026-10-28T10:60:00Z",
            "2026-10-28T10:00:60Z",
            "٢٠٢٦-10-28T10:00:00Z",
        ):
            with self.subTest(text=text):
                with self.assertRaises(ValueError):
                    timestamp_milliseconds(text)

    def test_the_text_escapes_are_the_ones_of_the_fixture_and_nothing_else(self):
        body = build_add_log_request(
            event(message="a\bb\fc\td\re\x1f\x7f<&>  é\U0001f600/"), "my-log-group", "my-log-stream"
        ).decode("utf-8")
        message = json.loads(json.loads(body)["message"])["Message"]
        self.assertEqual(message, "a\bb\fc\td\re\x1f\x7f<&>  é\U0001f600/")
        self.assertIn("\\\\b", body)
        self.assertIn("\\\\u001f", body)
        self.assertIn("\x7f<&>  é\U0001f600/", body)

    def test_a_lone_surrogate_is_written_as_the_replacement_character(self):
        body = build_add_log_request(event(message="a\ud800b"), "my-log-group", "my-log-stream")
        message = json.loads(json.loads(body)["message"])["Message"]
        self.assertEqual(message, "a�b")

    def test_a_data_value_that_is_not_a_text_is_an_empty_text(self):
        value = {
            "schema_version": "0.1",
            "event_id": "01900000-0000-7000-8000-000000000001",
            "event_type": "log.entry.recorded",
            "occurred_at": "2026-10-28T10:00:00Z",
            "recorded_at": "2026-10-28T10:00:00Z",
            "severity": "INFO",
            "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
            "data": {"category": 5, "message": None, "function": ["x"]},
        }
        message = json.loads(json.loads(build_add_log_request(envelope_from_json(value), "g", "s"))["message"])
        self.assertEqual((message["Category"], message["Function"], message["Message"]), ("", "", ""))

    def test_empty_attributes_are_null_fields(self):
        value = json.loads(build_add_log_request(event(attributes={}), "g", "s"))
        self.assertIsNone(json.loads(value["message"])["Fields"])

    def test_an_attribute_without_canonical_form_raises(self):
        for attributes in ({"n": float("nan")}, {"n": 2**60}, {"t": "a\ud800"}):
            with self.subTest(attributes=attributes):
                with self.assertRaises(Exception):
                    build_add_log_request(event(attributes=attributes), "g", "s")


class SettingsTest(unittest.TestCase):
    def fields(self, settings: Settings) -> list[str]:
        return [problem.field for problem in validate_settings(settings)]

    def test_the_defaults(self):
        settings = Settings()
        self.assertEqual(
            (settings.timeout_ms, settings.max_attempts, settings.retry_base_delay_ms, settings.retry_max_delay_ms),
            (3000, 5, 200, 30000),
        )
        self.assertEqual(
            (settings.skip_tls_verification, settings.bearer_token, settings.api_key_value), (False, "", "")
        )
        self.assertEqual(len(dataclasses.fields(Settings)), 13)

    def test_the_client_settings_are_the_endpoint_post_and_no_compression(self):
        client_settings = valid_settings(bearer_token=SECRET, timeout_ms=900).client_settings()
        self.assertEqual(client_settings.url, f"http://localhost:8080{PATH}")
        self.assertEqual(
            (client_settings.method, client_settings.compress, client_settings.timeout_ms), ("POST", False, 900)
        )
        self.assertEqual(client_settings.bearer_token, SECRET)

    def test_the_host_rule(self):
        for host, valid in {
            "http://localhost": True,
            "https://logs.example.invalid": True,
            "http://[::1]": True,
            "localhost": False,
            "ftp://localhost": False,
            "http://": False,
            "http:///x": False,
            "http://localhost/": False,
            "http://localhost/x": False,
            "http://localhost?x": False,
            "http://localhost#x": False,
            "http://local host": False,
            "http://localhost\n": False,
            "http://localhost\t": False,
            "HTTP://localhost": False,
            "": False,
        }.items():
            with self.subTest(host=host):
                self.assertEqual("host" not in self.fields(valid_settings(host=host)), valid)

    def test_the_port_rule_accepts_only_ascii_digits_from_1_to_65535(self):
        for port, valid in {
            "1": True,
            "80": True,
            "0080": True,
            "65535": True,
            "0": False,
            "00": False,
            "65536": False,
            "70000": False,
            "999999": False,
            "0" * 5000: False,
            "-1": False,
            "+80": False,
            "8 0": False,
            "80\n": False,
            "80a": False,
            "": False,
            "٨٠": False,
            "８０": False,
            "²": False,
        }.items():
            with self.subTest(port=port[:10]):
                self.assertEqual("port" not in self.fields(valid_settings(port=port)), valid)

    def test_the_path_group_and_stream_rules(self):
        self.assertEqual(self.fields(valid_settings(path="api")), ["path"])
        self.assertEqual(self.fields(valid_settings(path="")), ["path"])
        self.assertEqual(self.fields(valid_settings(path="/")), [])
        self.assertEqual(self.fields(valid_settings(log_group_name="")), ["log_group_name"])
        self.assertEqual(self.fields(valid_settings(log_stream_name="")), ["log_stream_name"])

    def test_a_value_of_a_wrong_type_is_a_problem_and_never_an_exception(self):
        settings = Settings(host=5, port=8080, path=None, log_group_name=b"g", log_stream_name=["s"])  # type: ignore
        self.assertEqual(self.fields(settings), ["host", "port", "path", "log_group_name", "log_stream_name"])

    def test_every_problem_comes_in_the_order_of_the_schema(self):
        settings = Settings(
            timeout_ms=0, bearer_token="a b", api_key_value="x y", max_attempts=0, retry_base_delay_ms=0
        )
        self.assertEqual(
            self.fields(settings),
            [
                "host",
                "port",
                "path",
                "log_group_name",
                "log_stream_name",
                "timeout_ms",
                "bearer_token",
                "api_key_header",
                "api_key_value",
                "max_attempts",
                "retry_base_delay_ms",
            ],
        )
        self.assertTrue(all(problem.code == "config_field_invalid" for problem in validate_settings(settings)))

    def test_authentication_over_plain_http_is_a_problem_of_the_host_unless_it_is_local(self):
        remote = settings_for(80, host="http://logs.example.invalid", bearer_token=SECRET)
        self.assertEqual(self.fields(remote), ["host"])
        self.assertEqual(self.fields(settings_for(80, host="https://logs.example.invalid", bearer_token=SECRET)), [])
        self.assertEqual(self.fields(settings_for(80, host="http://localhost", bearer_token=SECRET)), [])
        both = Settings(host="http://logs.example.invalid", port="x", bearer_token=SECRET, log_group_name="g")
        self.assertEqual(self.fields(both), ["host", "port", "path", "log_stream_name"])

    def test_a_bad_port_or_path_does_not_hide_or_invent_a_client_problem(self):
        settings = settings_for(80, port="x", path="y", timeout_ms=-1, api_key_value=SECRET)
        self.assertEqual(self.fields(settings), ["port", "path", "timeout_ms", "api_key_header"])

    def test_the_reasons_are_fixed_texts(self):
        settings = Settings(
            host=f"http://{SECRET} x", port=SECRET, path=SECRET, bearer_token=SECRET + " ", api_key_value=SECRET
        )
        for problem in validate_settings(settings):
            self.assertNotIn(SECRET, repr(problem))


class DeliveryTest(unittest.TestCase):
    def test_it_satisfies_both_sink_ports(self):
        destination = AwsHttpDestination(valid_settings())
        self.assertIsInstance(destination, LogSinkPort)
        self.assertIsInstance(destination, EventSinkPort)

    def test_a_2xx_accepts_and_one_request_goes_per_log_event(self):
        for status in (200, 201, 204):
            with self.subTest(status=status):
                with StubServer(lambda received, status=status: Reply(status)) as server:
                    delivery = AwsHttpDestination(settings_for(server.port)).deliver(
                        [event(message="a"), event(message="b")]
                    )
                    self.assertEqual(sent_messages(server), ["a", "b"])
                self.assertEqual(counts(delivery.result), (2, 0, 0, ""))
                self.assertEqual((delivery.sent, delivery.skipped), (2, 0))
                self.assertEqual((delivery.outcome.klass, delivery.outcome.status_code), ("success", status))
                with self.assertRaises(dataclasses.FrozenInstanceError):
                    delivery.sent = 5  # type: ignore[misc]

    def test_a_transient_status_fails_the_event_and_a_permanent_one_drops_it(self):
        table = {
            503: (0, 0, 1, "status_503", CLASS_TRANSIENT),
            500: (0, 0, 1, "status_500", CLASS_TRANSIENT),
            429: (0, 0, 1, "status_429", CLASS_TRANSIENT),
            408: (0, 0, 1, "status_408", CLASS_TRANSIENT),
            400: (0, 1, 0, "status_400", CLASS_PERMANENT),
            404: (0, 1, 0, "status_404", CLASS_PERMANENT),
            302: (0, 1, 0, "status_302", CLASS_PERMANENT),
        }
        for status, (accepted, dropped, failed, reason, klass) in table.items():
            with self.subTest(status=status):
                headers = {"Location": "/redirected"} if status == 302 else {}
                with StubServer(lambda received, status=status, headers=headers: Reply(status, headers)) as server:
                    delivery = AwsHttpDestination(settings_for(server.port)).deliver([event()])
                    self.assertEqual(len(server.requests), 1, "a redirect is not followed")
                    self.assertEqual(server.requests[0].path, PATH)
                self.assertEqual(counts(delivery.result), (accepted, dropped, failed, reason))
                self.assertEqual(delivery.outcome.klass, klass)

    def test_the_retry_after_is_the_one_of_the_last_event_that_did_not_get_accepted(self):
        def script(*replies: tuple[int, dict[str, str]]):
            return lambda received: Reply(*replies[len(server.requests) - 1])

        table = [
            ("one transient", [(503, {"Retry-After": "3"})], 3000),
            ("none given", [(503, {})], 0),
            ("the last of two transient", [(503, {"Retry-After": "5"}), (429, {"Retry-After": "9"})], 9000),
            ("an accepted one after the transient", [(503, {"Retry-After": "5"}), (200, {})], 5000),
            ("a permanent one after the transient", [(503, {"Retry-After": "5"}), (400, {"Retry-After": "8"})], 0),
            ("a permanent one only", [(404, {"Retry-After": "8"})], 0),
            ("a success", [(200, {"Retry-After": "8"})], 0),
        ]
        for name, replies, expected in table:
            with self.subTest(case=name):
                with StubServer(script(*replies)) as server:
                    result = AwsHttpDestination(settings_for(server.port)).send([event() for _ in replies])
                self.assertEqual(result.retry_after_ms, expected)

    def test_an_event_that_is_skipped_or_cannot_be_built_leaves_the_retry_after_alone(self):
        with StubServer(lambda received: Reply(503, {"Retry-After": "4"})) as server:
            result = AwsHttpDestination(settings_for(server.port)).send([event(), event("metric.x")])
        self.assertEqual((result.accepted, result.failed, result.retry_after_ms), (1, 1, 4000))

    def test_a_closed_port_fails_the_events_with_network_error(self):
        destination = AwsHttpDestination(settings_for(free_port()))
        delivery = destination.deliver([event(), event()])
        self.assertEqual(counts(delivery.result), (0, 0, 2, "network_error"))
        self.assertEqual((delivery.sent, delivery.outcome.reason), (2, "network_error"))

    def test_a_server_that_answers_late_fails_the_event_with_timeout(self):
        with StubServer(lambda received: Reply(200, delay_seconds=1.5)) as server:
            result = AwsHttpDestination(settings_for(server.port, timeout_ms=200)).send([event()])
        self.assertEqual(counts(result), (0, 0, 1, "timeout"))

    def test_the_authentication_headers_go_with_the_request(self):
        with StubServer() as server:
            AwsHttpDestination(settings_for(server.port, bearer_token=SECRET)).send([event()])
            AwsHttpDestination(settings_for(server.port, api_key_header="X-Api-Key", api_key_value=SECRET)).send(
                [event()]
            )
            first, second = server.requests
        self.assertEqual(first.headers["authorization"], f"Bearer {SECRET}")
        self.assertEqual(second.headers["x-api-key"], SECRET)

    def test_a_request_larger_than_the_limit_is_dropped_and_nothing_is_sent(self):
        with StubServer() as server:
            limits = Limits(max_event_bytes=100, max_batch_bytes=100)
            result = AwsHttpDestination(settings_for(server.port), limits).send([event(), event()])
            self.assertEqual(server.requests, [])
        self.assertEqual(counts(result), (0, 2, 0, "batch_too_large"))

    def test_no_events_and_only_skipped_events_make_no_request(self):
        with StubServer() as server:
            destination = AwsHttpDestination(settings_for(server.port))
            empty = destination.deliver([])
            skipped = destination.deliver([event("metric.counter.incremented"), event("logger.x"), event("audit.log")])
            self.assertEqual(server.requests, [])
        self.assertEqual(empty, Delivery(SendResult(0, 0, 0), None, 0, 0))
        self.assertEqual(skipped, Delivery(SendResult(3, 0, 0), None, 0, 3))

    def test_a_mixed_batch_counts_every_event_once_and_keeps_the_first_reason(self):
        def respond(received):
            message = json.loads(json.loads(received.body)["message"])["Message"]
            return Reply({"ok": 200, "busy": 503, "bad": 400, "bad_again": 400, "last": 200}[message])

        batch = [
            event(message="ok"),
            event("metric.x"),
            event(message="busy"),
            event(message="bad"),
            event(message="bad_again"),
            event("logger.x"),
            event(message="last"),
        ]
        with StubServer(respond) as server:
            delivery = AwsHttpDestination(settings_for(server.port)).deliver(batch)
            self.assertEqual(sent_messages(server), ["ok", "busy", "bad", "bad_again", "last"])
        self.assertEqual(counts(delivery.result), (4, 2, 1, "status_503"))
        self.assertEqual((delivery.sent, delivery.skipped), (5, 2))
        self.assertEqual(delivery.outcome, Outcome("success", 200))
        self.assertEqual(delivery.result.accepted + delivery.result.dropped + delivery.result.failed, len(batch))

    def test_the_outcome_is_the_one_of_the_last_request(self):
        with StubServer(lambda received: Reply(200 if len(server.requests) == 1 else 400)) as server:
            delivery = AwsHttpDestination(settings_for(server.port)).deliver([event(), event(), event("metric.x")])
        self.assertEqual(delivery.outcome, Outcome(CLASS_PERMANENT, 400, None, "status_400"))
        self.assertEqual(counts(delivery.result), (2, 1, 0, "status_400"))

    def test_an_event_that_cannot_be_built_is_dropped_and_the_others_are_sent(self):
        batch = [event(attributes={"n": float("nan")}), event(message="fine"), event(occurred_at="yesterday")]
        with StubServer() as server:
            delivery = AwsHttpDestination(settings_for(server.port)).deliver(batch)
            self.assertEqual(sent_messages(server), ["fine"])
        self.assertEqual(counts(delivery.result), (1, 2, 0, "serialization_failed"))
        self.assertEqual((delivery.sent, delivery.outcome), (1, Outcome("success", 200)))

    def test_when_every_event_is_unbuildable_there_is_no_outcome(self):
        with StubServer() as server:
            delivery = AwsHttpDestination(settings_for(server.port)).deliver([event(occurred_at="x")])
            self.assertEqual(server.requests, [])
        self.assertEqual(delivery, Delivery(SendResult(0, 1, 0, "serialization_failed"), None, 0, 0))

    def test_next_delay_is_the_one_of_the_shared_client(self):
        settings = valid_settings(max_attempts=3)
        destination = AwsHttpDestination(settings)
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
        self.assertTrue(destination.next_delay(0, outcomes[0], 0.5)[1])


class RecoveryTest(unittest.TestCase):
    def setUp(self):
        self.destination = AwsHttpDestination(settings_for(free_port()))

    def test_an_object_that_is_not_an_event_fails_the_events_with_destination_failed(self):
        result = self.destination.send([object(), object()])  # type: ignore[list-item]
        self.assertEqual(counts(result), (0, 0, 2, "destination_failed"))

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

    def test_no_failing_path_shows_the_token_or_the_key(self):
        for status in (503, 429, 400, 302):
            with StubServer(lambda received, status=status: Reply(status, {"Location": f"/other?token={SECRET}"})) as s:
                for overrides in ({"bearer_token": SECRET}, {"api_key_header": "X-Key", "api_key_value": SECRET}):
                    delivery = AwsHttpDestination(settings_for(s.port, **overrides)).deliver([event()])
                    self.assert_clean(delivery, delivery.result, delivery.outcome)
        delivery = AwsHttpDestination(settings_for(free_port(), bearer_token=SECRET)).deliver([event()])
        self.assert_clean(delivery)
        with StubServer(lambda received: Reply(200, delay_seconds=1.0)) as server:
            self.assert_clean(
                AwsHttpDestination(settings_for(server.port, bearer_token=SECRET, timeout_ms=200)).deliver([event()])
            )
        with StubServer() as server:
            limits = Limits(max_event_bytes=100, max_batch_bytes=100)
            self.assert_clean(
                AwsHttpDestination(settings_for(server.port, bearer_token=SECRET), limits).deliver([event()])
            )
        self.assert_clean(AwsHttpDestination(settings_for(1, bearer_token=SECRET)).send([object()]))  # type: ignore

    def test_a_refused_configuration_does_not_show_the_secret(self):
        for overrides in (
            {"host": "http://logs.example.invalid", "bearer_token": SECRET},
            {"bearer_token": SECRET, "api_key_value": SECRET},
            {"api_key_header": "bad header", "api_key_value": SECRET + " "},
            {"host": f"{SECRET}", "port": SECRET, "path": SECRET, "bearer_token": SECRET + " "},
        ):
            with self.subTest(fields=sorted(overrides)):
                with self.assertRaises(ConfigurationError) as raised:
                    AwsHttpDestination(settings_for(80, **overrides))
                self.assert_clean(raised.exception, raised.exception.problems)


class ConcurrencyTest(unittest.TestCase):
    def test_sixteen_threads_share_one_destination(self):
        with StubServer() as server:
            destination = AwsHttpDestination(settings_for(server.port))
            results: list[SendResult] = []
            lock = threading.Lock()

            def work() -> None:
                for _ in range(3):
                    result = destination.send([event(), event("metric.x"), event(severity="DEBUG")])
                    with lock:
                        results.append(result)

            threads = [threading.Thread(target=work) for _ in range(16)]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join(timeout=60)
            self.assertEqual(len(server.requests), 96)
        self.assertEqual(len(results), 48)
        self.assertTrue(all(counts(result) == (3, 0, 0, "") for result in results))


class PublicModuleTest(unittest.TestCase):
    def test_the_public_functions_build_the_destination(self):
        self.assertEqual(public_aws_http.default_settings(), Settings())
        self.assertIs(public_aws_http.Destination, AwsHttpDestination)
        self.assertIs(public_aws_http.Delivery, Delivery)
        self.assertIs(public_aws_http.Settings, Settings)
        self.assertIsInstance(public_aws_http.new_destination(valid_settings()), AwsHttpDestination)
        self.assertEqual(public_aws_http.__name__, "telemetry.logging.aws_http")

    def test_the_default_settings_are_refused_and_name_the_five_fields(self):
        with self.assertRaises(ConfigurationError) as raised:
            public_aws_http.new_destination(public_aws_http.default_settings())
        self.assertEqual(
            [problem.field for problem in raised.exception.problems],
            ["host", "port", "path", "log_group_name", "log_stream_name"],
        )
        self.assertEqual(raised.exception.code, "config_field_invalid")

    def test_it_works_as_a_log_sink_of_the_core(self):
        with StubServer() as server:
            sink = public_aws_http.new_destination(settings_for(server.port))
            core = stop_workers_on_cleanup(
                self,
                telemetry.new(
                    merged_config(
                        "microservice",
                        {"resource": {"service_name": "sensor-hub"}, "delivery": {"flush_interval_ms": 5}},
                    ),
                    telemetry.with_log_sink(sink),
                    telemetry.with_clock(FakeClock()),
                    telemetry.with_id_generator(FakeIdGenerator([EventId("01900000-0000-7000-8000-000000000001")])),
                ),
            )
            core.logger.info(telemetry.LogEntry(category="http", message="first"))
            eventually(5.0, lambda: len(server.requests) >= 1, "the request of the worker")
            core.shutdown()
            self.assertEqual(sent_messages(server), ["first"])
            self.assertEqual(json.loads(server.requests[0].body)["log_group_name"], "my-log-group")
