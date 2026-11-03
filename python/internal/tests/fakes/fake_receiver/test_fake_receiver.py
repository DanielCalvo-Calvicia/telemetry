"""Black-box tests of the fake receiver itself (T-04.8)."""

import gzip
import http.client
import json
import socket
import threading
import time
import unittest
from typing import Optional

from internal.tests.fakes.fake_receiver import MAX_BODY_BYTES, Answer, Config, Receiver

BATCH = json.dumps({"schema_version": "0.1", "events": [{"n": 1}, {"n": 2}]}).encode("utf-8")


def batch(count: int = 1, start: int = 0) -> bytes:
    """A valid batch of ``count`` events numbered from ``start``."""
    return json.dumps({"schema_version": "0.1", "events": [{"n": start + i} for i in range(count)]}).encode("utf-8")


def post(
    receiver: Receiver, body: bytes = BATCH, headers: Optional[dict] = None, path: str = "/", timeout: float = 5.0
) -> tuple[int, dict, bytes]:
    """POST on a new connection; return the status, the lower-cased headers and the body."""
    host, port = receiver.url().removeprefix("http://").split(":")
    connection = http.client.HTTPConnection(host, int(port), timeout=timeout)
    try:
        connection.request("POST", path, body=body, headers=headers or {})
        response = connection.getresponse()
        return response.status, {k.lower(): v for k, v in response.getheaders()}, response.read()
    finally:
        connection.close()


class ReceiverTest(unittest.TestCase):
    def setUp(self):
        self.receiver = Receiver()
        self.addCleanup(self.receiver.close)

    def test_it_listens_only_on_the_loopback_address(self):
        self.assertTrue(self.receiver.url().startswith("http://127.0.0.1:"))
        port = int(self.receiver.url().rsplit(":", 1)[1])
        self.assertGreater(port, 0)
        self.assertEqual(self.receiver._server.server_address[0], "127.0.0.1")

    def test_a_valid_batch_is_recorded_with_its_details(self):
        status, _, _ = post(self.receiver, batch(2), {"X-Test": "yes"}, path="/ingest?a=1")
        self.assertEqual(status, 200)
        (request,) = self.receiver.requests()
        self.assertEqual((request.method, request.path, request.query), ("POST", "/ingest", "a=1"))
        self.assertEqual(request.headers["x-test"], "yes")
        self.assertEqual(request.events, [{"n": 0}, {"n": 1}])
        self.assertEqual((request.valid, request.invalid_reason, request.authenticated), (True, "", True))
        self.assertEqual((request.gzipped, request.answered_status), (False, 200))
        self.assertEqual(request.raw_body, request.body)
        self.assertGreater(request.received_at, 0)

    def test_gzip_is_decompressed_and_the_raw_body_is_kept(self):
        compressed = gzip.compress(BATCH)
        status, _, _ = post(self.receiver, compressed, {"Content-Encoding": "GZIP"})
        self.assertEqual(status, 200)
        (request,) = self.receiver.requests()
        self.assertTrue(request.gzipped)
        self.assertEqual((request.raw_body, request.body), (compressed, BATCH))

    def test_a_body_that_does_not_gunzip_is_invalid(self):
        status, headers, body = post(self.receiver, b"not gzip", {"Content-Encoding": "gzip"})
        self.assertEqual(status, 400)
        self.assertTrue(headers["content-type"].startswith("text/plain"))
        self.assertEqual(body, b"the body is not valid gzip")
        self.assertEqual(self.receiver.requests()[0].invalid_reason, "the body is not valid gzip")

    def test_every_reason_of_an_invalid_body(self):
        table = [
            (b"\xff\xfe", "the body is not valid JSON"),
            (b"{", "the body is not valid JSON"),
            (b"[1]", "the body is not a JSON object"),
            (b'{"events":[]}', "schema_version is missing"),
            (b'{"schema_version":"0.1"}', "events is missing"),
            (b'{"schema_version":"0.1","events":[],"extra":1}', "an unknown key: extra"),
            (b'{"schema_version":"0.2","events":[]}', "schema_version must be 0.1"),
            (b'{"schema_version":1,"events":[]}', "schema_version must be 0.1"),
            (b'{"schema_version":"0.1","events":{}}', "events must be a list"),
            (b'{"schema_version":"0.1","events":[1]}', "every event must be an object"),
        ]
        for body, reason in table:
            with self.subTest(reason=reason, body=body):
                self.receiver.reset()
                status, _, answer = post(self.receiver, body)
                self.assertEqual((status, answer), (400, reason.encode()))
                (request,) = self.receiver.requests()
                self.assertEqual((request.valid, request.invalid_reason, request.events), (False, reason, None))
                self.assertEqual(request.answered_status, 400)

    def test_an_empty_batch_is_valid(self):
        self.assertEqual(post(self.receiver, batch(0))[0], 200)
        self.assertEqual(self.receiver.events(), [])
        self.assertTrue(self.receiver.requests()[0].valid)

    def test_a_request_without_a_body_is_invalid(self):
        host, port = self.receiver.url().removeprefix("http://").split(":")
        connection = http.client.HTTPConnection(host, int(port), timeout=5)
        connection.request("GET", "/")
        self.assertEqual(connection.getresponse().status, 400)
        connection.close()
        self.assertEqual(self.receiver.requests()[0].method, "GET")

    def test_scripted_answers_are_used_only_by_valid_requests(self):
        self.receiver.script(Answer(status=503), Answer(status=429, retry_after="7", location="/x"))
        self.assertEqual(post(self.receiver, b"nope")[0], 400)
        self.assertEqual(post(self.receiver)[0], 503)
        status, headers, _ = post(self.receiver)
        self.assertEqual((status, headers["retry-after"], headers["location"]), (429, "7", "/x"))
        self.assertEqual(post(self.receiver)[0], 200)
        self.assertEqual([r.answered_status for r in self.receiver.requests()], [400, 503, 429, 200])

    def test_an_unauthenticated_request_does_not_consume_the_script(self):
        with Receiver(Config(expected_bearer="placeholder-token")) as receiver:
            receiver.script(Answer(status=503))
            status, _, _ = post(receiver)
            self.assertEqual(status, 401)
            self.assertEqual(post(receiver, headers={"Authorization": "Bearer placeholder-token"})[0], 503)
            first, second = receiver.requests()
            self.assertEqual(
                (first.authenticated, first.valid, first.invalid_reason, first.answered_status), (False, False, "", 401)
            )
            self.assertEqual((second.authenticated, second.valid), (True, True))

    def test_the_api_key_needs_header_and_value(self):
        with Receiver(Config(expected_api_key_header="X-Key", expected_api_key_value="placeholder-key")) as receiver:
            self.assertEqual(post(receiver)[0], 401)
            self.assertEqual(post(receiver, headers={"X-Key": "other"})[0], 401)
            self.assertEqual(post(receiver, headers={"x-key": "placeholder-key"})[0], 200)

    def test_both_authentications_are_required_together(self):
        config = Config("placeholder-token", "X-Key", "placeholder-key")
        with Receiver(config) as receiver:
            self.assertEqual(post(receiver, headers={"Authorization": "Bearer placeholder-token"})[0], 401)
            both = {"Authorization": "Bearer placeholder-token", "X-Key": "placeholder-key"}
            self.assertEqual(post(receiver, headers=both)[0], 200)

    def test_the_default_status_applies_when_the_script_is_empty(self):
        with Receiver(Config(default_status=204)) as receiver:
            self.assertEqual(post(receiver)[0], 204)

    def test_close_answers_nothing(self):
        self.receiver.script(Answer(close=True))
        with self.assertRaises((ConnectionError, http.client.BadStatusLine)):
            post(self.receiver)
        self.assertEqual(self.receiver.requests()[0].answered_status, 0)

    def test_truncate_announces_more_than_it_writes(self):
        self.receiver.script(Answer(truncate=True))
        with self.assertRaises(http.client.IncompleteRead):
            post(self.receiver)

    def test_a_delay_postpones_the_answer(self):
        self.receiver.script(Answer(delay_seconds=0.3))
        started = time.monotonic()
        self.assertEqual(post(self.receiver)[0], 200)
        self.assertGreaterEqual(time.monotonic() - started, 0.25)

    def test_close_releases_a_waiting_delay(self):
        receiver = Receiver()
        receiver.script(Answer(delay_seconds=30))
        outcome: list = []

        def client() -> None:
            try:
                outcome.append(post(receiver, timeout=10)[0])
            except (ConnectionError, http.client.BadStatusLine, OSError) as error:
                outcome.append(error)

        thread = threading.Thread(target=client)
        thread.start()
        self.assertTrue(receiver.wait_for_events(2, 5))
        started = time.monotonic()
        receiver.close()
        thread.join(timeout=10)
        self.assertFalse(thread.is_alive())
        self.assertLess(time.monotonic() - started, 5)
        self.assertEqual(len(outcome), 1)
        self.assertNotEqual(outcome[0], 200)

    def test_close_is_idempotent_and_refuses_new_connections(self):
        receiver = Receiver()
        host, port = receiver.url().removeprefix("http://").split(":")
        receiver.close()
        receiver.close()
        with self.assertRaises(OSError):
            socket.create_connection((host, int(port)), timeout=2)

    def test_the_context_manager_closes(self):
        with Receiver() as receiver:
            self.assertEqual(post(receiver)[0], 200)
        self.assertTrue(receiver._closed.is_set())

    def test_wait_for_events_is_true_when_events_arrive_from_another_thread(self):
        timer = threading.Timer(0.2, lambda: post(self.receiver, batch(3)))
        timer.start()
        self.addCleanup(timer.join)
        started = time.monotonic()
        self.assertTrue(self.receiver.wait_for_events(3, 5))
        self.assertLess(time.monotonic() - started, 4)

    def test_wait_for_events_is_false_at_the_timeout(self):
        post(self.receiver, batch(1))
        started = time.monotonic()
        self.assertFalse(self.receiver.wait_for_events(2, 0.3))
        self.assertGreaterEqual(time.monotonic() - started, 0.25)
        self.assertTrue(self.receiver.wait_for_events(1, 0))

    def test_invalid_events_do_not_count_for_the_wait(self):
        post(self.receiver, b'{"schema_version":"0.1","events":[1]}')
        self.assertFalse(self.receiver.wait_for_events(1, 0.1))

    def test_reset_clears_the_script_and_the_records(self):
        self.receiver.script(Answer(status=500))
        post(self.receiver)
        self.receiver.reset()
        self.assertEqual((self.receiver.requests(), self.receiver.events()), ([], []))
        self.assertFalse(self.receiver.wait_for_events(1, 0))
        self.assertEqual(post(self.receiver)[0], 200)

    def test_requests_returns_a_copy(self):
        post(self.receiver, batch(1))
        copy_of = self.receiver.requests()
        copy_of[0].events[0]["n"] = 99  # type: ignore[index]
        copy_of[0].headers["x"] = "y"
        copy_of.clear()
        again = self.receiver.requests()
        self.assertEqual(len(again), 1)
        self.assertEqual(again[0].events, [{"n": 0}])
        self.assertNotIn("x", again[0].headers)
        self.receiver.events()[0]["n"] = 99
        self.assertEqual(self.receiver.events(), [{"n": 0}])

    def test_an_oversized_body_is_answered_413_and_recorded_invalid(self):
        status, _, body = post(self.receiver, b"a" * (MAX_BODY_BYTES + 1))
        self.assertEqual((status, body), (413, b"the body is larger than 8 MiB"))
        (request,) = self.receiver.requests()
        self.assertEqual(
            (request.valid, request.invalid_reason, request.answered_status),
            (False, "the body is larger than 8 MiB", 413),
        )

    def test_oversized_or_broken_bodies_from_an_unauthenticated_client_are_401(self):
        with Receiver(Config(expected_bearer="placeholder-token")) as receiver:
            receiver.script(Answer(status=503))
            for body, headers in ((b"a" * (MAX_BODY_BYTES + 1), {}), (b"not gzip", {"Content-Encoding": "gzip"})):
                self.assertEqual(post(receiver, body, headers)[0], 401)
            for item in receiver.requests():
                self.assertEqual(
                    (item.authenticated, item.valid, item.invalid_reason, item.events), (False, False, "", None)
                )
            self.assertEqual(post(receiver, headers={"Authorization": "Bearer placeholder-token"})[0], 503)

    def test_bad_gzip_and_a_gzip_bomb_from_an_authenticated_client(self):
        with Receiver(Config(expected_bearer="placeholder-token")) as receiver:
            headers = {"Authorization": "Bearer placeholder-token", "Content-Encoding": "gzip"}
            self.assertEqual(post(receiver, b"not gzip", headers)[0], 400)
            self.assertEqual(post(receiver, gzip.compress(BATCH)[:-4], headers)[0], 400)
            status, _, body = post(receiver, gzip.compress(b"0" * (MAX_BODY_BYTES + 1)), headers)
            self.assertEqual((status, body), (413, b"the body is larger than 8 MiB"))
            reasons = [item.invalid_reason for item in receiver.requests()]
            self.assertEqual(reasons, ["the body is not valid gzip"] * 2 + ["the body is larger than 8 MiB"])

    def test_a_client_that_disconnects_in_the_middle_does_not_break_the_fake(self):
        host, port = self.receiver.url().removeprefix("http://").split(":")
        with socket.create_connection((host, int(port)), timeout=2) as raw:
            raw.sendall(b"POST / HTTP/1.1\r\nContent-Length: 100\r\n\r\n0123456789")
        time.sleep(0.2)
        self.assertEqual(post(self.receiver)[0], 200)
        self.assertEqual(len(self.receiver.requests()), 1)

    def test_a_bad_content_length_reads_no_body(self):
        host, port = self.receiver.url().removeprefix("http://").split(":")
        with socket.create_connection((host, int(port)), timeout=2) as raw:
            raw.sendall(b"POST / HTTP/1.0\r\nContent-Length: abc\r\n\r\n")
            self.assertTrue(raw.recv(1024).startswith(b"HTTP/1.0 400"))

    def test_sixteen_threads_send_valid_batches(self):
        results: list = []
        lock = threading.Lock()

        def work(index: int) -> None:
            for step in range(5):
                status = post(self.receiver, batch(2, index * 100 + step * 2))[0]
                with lock:
                    results.append(status)

        threads = [threading.Thread(target=work, args=(i,)) for i in range(16)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join(timeout=60)
        self.assertEqual(results, [200] * 80)
        self.assertEqual(len(self.receiver.requests()), 80)
        self.assertEqual(len(self.receiver.events()), 160)
        self.assertEqual(len({event["n"] for event in self.receiver.events()}), 160)


if __name__ == "__main__":
    unittest.main()
