"""Black-box tests of HttpClient against local servers on 127.0.0.1."""

import dataclasses
import gzip
import shutil
import threading
import time
import unittest

from internal.tests.application.services.masking_helpers import envelope_from_json
from internal.tests.infrastructure.outbound.shared.http_client.server_helpers import (
    SECRET,
    Reply,
    StubServer,
    free_port,
)
from telemetry.internal.domain.value_objects.limits import Limits
from telemetry.internal.infrastructure.config.errors import ConfigurationError
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.build_batch_body import build_batch_body
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.classify_response import (
    CLASS_PERMANENT,
    CLASS_SUCCESS,
    CLASS_TRANSIENT,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.http_client import HttpClient, Outcome
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import HttpSettings

BODY = b'{"events":[],"schema_version":"0.1"}'
HAS_OPENSSL = shutil.which("openssl") is not None


def client_for(url: str, limits: Limits = Limits(), **overrides) -> HttpClient:
    return HttpClient(HttpSettings(url=url, **overrides), limits)


def envelope(number: int):
    return envelope_from_json(
        {
            "schema_version": "0.1",
            "event_id": f"01900000-0000-7000-8000-00000000000{number}",
            "event_type": "log.entry.recorded",
            "occurred_at": "2026-10-28T10:00:00.123456Z",
            "recorded_at": "2026-10-28T10:00:00.123456Z",
            "severity": "INFO",
            "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
            "data": {"category": "http", "message": f"m{number}"},
        }
    )


class StatusTest(unittest.TestCase):
    def test_every_status_gives_its_class_and_its_reason(self):
        table = {
            200: CLASS_SUCCESS,
            204: CLASS_SUCCESS,
            301: CLASS_PERMANENT,
            400: CLASS_PERMANENT,
            401: CLASS_PERMANENT,
            404: CLASS_PERMANENT,
            408: CLASS_TRANSIENT,
            429: CLASS_TRANSIENT,
            500: CLASS_TRANSIENT,
            503: CLASS_TRANSIENT,
        }
        for status, klass in table.items():
            with self.subTest(status=status), StubServer(lambda received, status=status: Reply(status)) as server:
                outcome = client_for(server.url()).send(BODY)
                self.assertEqual(outcome.klass, klass)
                self.assertEqual(outcome.status_code, status)
                self.assertEqual(outcome.reason, "" if klass == CLASS_SUCCESS else f"status_{status}")

    def test_retry_after_of_a_429_is_parsed(self):
        with StubServer(lambda received: Reply(429, {"Retry-After": "7"})) as server:
            outcome = client_for(server.url()).send(BODY)
        self.assertEqual(outcome, Outcome(CLASS_TRANSIENT, 429, 7000, "status_429"))

    def test_a_retry_after_that_asks_nothing_valid_gives_none(self):
        with StubServer(lambda received: Reply(503, {"Retry-After": "soon"})) as server:
            outcome = client_for(server.url()).send(BODY)
        self.assertIsNone(outcome.retry_after_milliseconds)

    def test_a_redirect_is_not_followed(self):
        with StubServer() as target:
            with StubServer(lambda received: Reply(302, {"Location": target.url()})) as server:
                outcome = client_for(server.url()).send(BODY)
            self.assertEqual(target.requests, [])
        self.assertEqual(outcome, Outcome(CLASS_PERMANENT, 302, None, "status_302"))

    def test_a_response_that_cannot_be_read_is_transient(self):
        raw = b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\nZZ\r\n"
        with StubServer(lambda received: Reply(raw=raw)) as server:
            outcome = client_for(server.url()).send(BODY)
        self.assertEqual(outcome, Outcome(CLASS_TRANSIENT, 200, None, "response_unreadable"))

    def test_garbage_instead_of_a_response_is_a_network_error(self):
        with StubServer(lambda received: Reply(raw=b"not http at all\r\n\r\n")) as server:
            outcome = client_for(server.url()).send(BODY)
        self.assertEqual(outcome, Outcome(CLASS_TRANSIENT, 0, None, "network_error"))


class TransportTest(unittest.TestCase):
    def test_a_server_that_answers_late_ends_at_the_timeout(self):
        with StubServer(lambda received: Reply(delay_seconds=2.5)) as server:
            started = time.monotonic()
            outcome = client_for(server.url(), timeout_ms=200).send(BODY)
            elapsed = time.monotonic() - started
        self.assertEqual(outcome, Outcome(CLASS_TRANSIENT, 0, None, "timeout"))
        self.assertLess(elapsed, 2.4)

    def test_a_closed_port_is_a_network_error(self):
        outcome = client_for(f"http://127.0.0.1:{free_port()}/ingest").send(BODY)
        self.assertEqual(outcome, Outcome(CLASS_TRANSIENT, 0, None, "network_error"))

    def test_the_query_of_the_url_is_sent(self):
        with StubServer() as server:
            client_for(server.url("/a/b?x=1&y=2")).send(BODY)
            client_for(server.url("")).send(BODY)
        self.assertEqual([received.path for received in server.requests], ["/a/b?x=1&y=2", "/"])

    def test_the_request_has_the_method_the_content_type_and_the_body(self):
        for method in ("POST", "PUT", "PATCH"):
            with self.subTest(method=method), StubServer() as server:
                outcome = client_for(server.url(), method=method).send(BODY)
                self.assertEqual(outcome, Outcome(CLASS_SUCCESS, 200))
                (received,) = server.requests
                self.assertEqual(received.method, method)
                self.assertEqual(received.headers["content-type"], "application/json")
                self.assertNotIn("content-encoding", received.headers)
                self.assertNotIn("authorization", received.headers)
                self.assertEqual(received.body, BODY)

    def test_the_bearer_token_goes_in_the_authorization_header(self):
        with StubServer() as server:
            client_for(server.url(), bearer_token="tok.en-1_2").send(BODY)
        self.assertEqual(server.requests[0].headers["authorization"], "Bearer tok.en-1_2")

    def test_the_api_key_goes_in_its_header(self):
        with StubServer() as server:
            client_for(server.url(), api_key_header="X-Api-Key", api_key_value="k3y").send(BODY)
        self.assertEqual(server.requests[0].headers["x-api-key"], "k3y")
        self.assertNotIn("authorization", server.requests[0].headers)

    def test_a_compressed_body_has_the_encoding_header_and_gunzips_to_the_plain_body(self):
        with StubServer() as server:
            outcome = client_for(server.url(), compress=True).send(BODY)
        self.assertEqual(outcome.klass, CLASS_SUCCESS)
        received = server.requests[0]
        self.assertEqual(received.headers["content-encoding"], "gzip")
        self.assertEqual(gzip.decompress(received.body), BODY)

    def test_a_body_over_the_batch_limit_is_permanent_and_makes_no_request(self):
        with StubServer() as server:
            limits = Limits(max_event_bytes=10, max_batch_bytes=len(BODY) - 1)
            outcome = client_for(server.url(), limits=limits, compress=True).send(BODY)
            self.assertEqual(outcome, Outcome(CLASS_PERMANENT, 0, None, "batch_too_large"))
            self.assertEqual(server.requests, [])
            limits = Limits(max_event_bytes=10, max_batch_bytes=len(BODY))
            self.assertEqual(client_for(server.url(), limits=limits).send(BODY).klass, CLASS_SUCCESS)

    def test_send_never_raises(self):
        with StubServer() as server:
            outcome = client_for(server.url()).send(None)  # type: ignore[arg-type]
        self.assertEqual(outcome, Outcome(CLASS_TRANSIENT, 0, None, "client_failed"))

    def test_sixteen_threads_can_send_on_one_client(self):
        with StubServer() as server:
            client = client_for(server.url())
            outcomes: list[Outcome] = []

            def send() -> None:
                outcomes.append(client.send(BODY))

            threads = [threading.Thread(target=send) for _ in range(16)]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join()
        self.assertEqual(outcomes, [Outcome(CLASS_SUCCESS, 200)] * 16)
        self.assertEqual(len(server.requests), 16)


class SendBatchTest(unittest.TestCase):
    def test_two_events_arrive_as_the_batch_body(self):
        events = [envelope(1), envelope(2)]
        with StubServer() as server:
            outcome = client_for(server.url()).send_batch(events)
        self.assertEqual(outcome.klass, CLASS_SUCCESS)
        self.assertEqual(server.requests[0].body, build_batch_body(events))
        self.assertIn(b'"m1"', server.requests[0].body)
        self.assertIn(b'"m2"', server.requests[0].body)

    def test_an_event_that_cannot_be_serialized_is_permanent_and_makes_no_request(self):
        broken = dataclasses.replace(envelope(1), data={"value": float("nan")})
        with StubServer() as server:
            outcome = client_for(server.url()).send_batch([broken])
            self.assertEqual(outcome, Outcome(CLASS_PERMANENT, 0, None, "serialization_failed"))
            self.assertEqual(server.requests, [])


class NextDelayTest(unittest.TestCase):
    def setUp(self):
        self.client = client_for("https://logs.example.invalid/ingest")
        self.transient = Outcome(CLASS_TRANSIENT, 503, None, "status_503")

    def test_the_max_attempts_is_the_one_of_the_settings(self):
        self.assertEqual(self.client.max_attempts, 5)
        self.assertEqual(client_for("https://logs.example.invalid/x", max_attempts=2).max_attempts, 2)

    def test_a_transient_outcome_retries_until_the_last_attempt(self):
        self.assertEqual(self.client.next_delay(0, self.transient, 0.0), (100, True))
        self.assertEqual(self.client.next_delay(3, self.transient, 1.0), (1600, True))
        self.assertEqual(self.client.next_delay(4, self.transient, 0.5), (0, False))

    def test_a_permanent_or_successful_outcome_never_retries(self):
        for klass in (CLASS_PERMANENT, CLASS_SUCCESS):
            self.assertEqual(self.client.next_delay(0, Outcome(klass), 0.5), (0, False))

    def test_a_retry_after_larger_than_the_backoff_wins(self):
        asked = Outcome(CLASS_TRANSIENT, 429, 5000, "status_429")
        self.assertEqual(self.client.next_delay(0, asked, 1.0), (5000, True))
        small = Outcome(CLASS_TRANSIENT, 429, 10, "status_429")
        self.assertEqual(self.client.next_delay(0, small, 1.0), (200, True))


class InvalidSettingsTest(unittest.TestCase):
    def test_invalid_settings_raise_a_configuration_error_with_every_problem_and_no_value(self):
        settings = HttpSettings(
            url=f"http://logs.example.invalid/ingest?token={SECRET}",
            bearer_token=SECRET,
            method="GET",
        )
        with self.assertRaises(ConfigurationError) as raised:
            HttpClient(settings, Limits())
        self.assertEqual([problem.field for problem in raised.exception.problems], ["url", "method"])
        self.assertNotIn(SECRET, str(raised.exception))
        self.assertNotIn("logs.example.invalid", str(raised.exception))


class SecretsTest(unittest.TestCase):
    def assert_no_secret(self, outcome: Outcome) -> None:
        for text in (repr(outcome), str(outcome)):
            self.assertNotIn(SECRET, text)
            self.assertNotIn("token=", text)
            self.assertNotIn("?", text)

    def secret_client(self, url: str, **overrides) -> HttpClient:
        return client_for(f"{url}?token={SECRET}", bearer_token=SECRET, **overrides)

    def test_a_closed_port(self):
        outcome = self.secret_client(f"http://127.0.0.1:{free_port()}/ingest").send(BODY)
        self.assertEqual(outcome.reason, "network_error")
        self.assert_no_secret(outcome)

    def test_a_timeout(self):
        with StubServer(lambda received: Reply(delay_seconds=1.0)) as server:
            outcome = self.secret_client(server.url(), timeout_ms=200).send(BODY)
        self.assertEqual(outcome.reason, "timeout")
        self.assert_no_secret(outcome)

    def test_a_server_error(self):
        with StubServer(lambda received: Reply(500, body=SECRET.encode())) as server:
            outcome = self.secret_client(server.url()).send(BODY)
        self.assertEqual(outcome.reason, "status_500")
        self.assert_no_secret(outcome)


@unittest.skipUnless(HAS_OPENSSL, "the TLS tests need the openssl command line to make a throwaway certificate")
class TlsTest(unittest.TestCase):
    def test_an_untrusted_certificate_is_refused_by_default(self):
        with StubServer(tls=True) as server:
            outcome = client_for(server.url()).send(BODY)
            self.assertEqual(outcome, Outcome(CLASS_TRANSIENT, 0, None, "tls_verification_failed"))
            self.assertEqual(server.requests, [])

    def test_the_explicit_setting_turns_the_verification_off(self):
        with StubServer(tls=True) as server:
            outcome = client_for(server.url(), skip_tls_verification=True, bearer_token="tok").send(BODY)
            self.assertEqual(outcome, Outcome(CLASS_SUCCESS, 200))
            self.assertEqual(server.requests[0].headers["authorization"], "Bearer tok")

    def test_the_refusal_holds_no_secret(self):
        with StubServer(tls=True) as server:
            outcome = client_for(f"{server.url()}?token={SECRET}", bearer_token=SECRET).send(BODY)
        self.assertEqual(outcome.reason, "tls_verification_failed")
        self.assertNotIn(SECRET, repr(outcome))
        self.assertNotIn("token=", repr(outcome))
