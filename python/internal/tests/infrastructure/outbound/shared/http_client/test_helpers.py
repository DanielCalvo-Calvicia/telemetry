"""Black-box tests of the pure pieces of the shared HTTP client that the shared cases do not cover."""

import datetime
import unittest

from telemetry.internal.infrastructure.outbound.shared.http_client.error.map_transport_error import (
    REASON_NETWORK_ERROR,
    REASON_TIMEOUT,
    REASON_TLS_VERIFICATION_FAILED,
    map_transport_error,
)
from telemetry.internal.infrastructure.outbound.shared.http_client.external.parse_url import parse_url
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.build_batch_body import build_batch_body
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.compress_gzip import compress_gzip
from telemetry.internal.infrastructure.outbound.shared.http_client.helpers.parse_retry_after import parse_retry_after
from telemetry.internal.infrastructure.outbound.shared.http_client.settings import HttpSettings, validate_settings

NOW = datetime.datetime(2026, 10, 21, 7, 27, tzinfo=datetime.timezone.utc)
URL = "https://logs.example.invalid/ingest"


def fields_of(**overrides) -> list[str]:
    return [problem.field for problem in validate_settings(HttpSettings(**{"url": URL, **overrides}))]


class BodyTest(unittest.TestCase):
    def test_no_events_give_an_empty_list(self):
        self.assertEqual(build_batch_body([]), b'{"events":[],"schema_version":"0.1"}')

    def test_the_gzip_header_is_deterministic(self):
        first = compress_gzip(b"abc")
        self.assertEqual(first, compress_gzip(b"abc"))
        self.assertEqual(first[4:8], b"\x00\x00\x00\x00")  # the modification time
        self.assertEqual(first[3], 0)  # no flags, so no name


class RetryAfterTest(unittest.TestCase):
    def test_a_huge_number_of_digits_is_cut_to_a_day(self):
        self.assertEqual(parse_retry_after("9" * 5000, NOW), 86_400_000)
        self.assertEqual(parse_retry_after("0" * 5000 + "3", NOW), 3000)

    def test_a_non_ascii_digit_asks_nothing(self):
        self.assertIsNone(parse_retry_after("٣", NOW))

    def test_an_http_date_with_a_naive_now_is_read_as_utc(self):
        naive = NOW.replace(tzinfo=None)
        self.assertEqual(parse_retry_after("Wed, 21 Oct 2026 07:28:00 GMT", naive), 60_000)

    def test_the_two_accepted_date_forms_are_read_as_utc(self):
        self.assertEqual(parse_retry_after("Wed, 21 Oct 2026 07:28:00 GMT", NOW), 60_000)
        self.assertEqual(parse_retry_after("Wed Oct 21 07:28:00 2026", NOW), 60_000)
        self.assertEqual(parse_retry_after("Wed Oct  1 07:28:00 2026", NOW), 0)  # padded day, in the past
        self.assertEqual(parse_retry_after("Mon Nov 02 07:27:00 2026", NOW), 86_400_000)  # the weekday is not checked

    def test_every_other_date_form_asks_nothing(self):
        for text in (
            "Wed, 21 Oct 2026 07:28:00 +0100",
            "Wed, 21 Oct 2026 07:28:00 UTC",
            "21 Oct 2026 07:28:00",
            "Wed, 21 Oct 2026 07:28:00",
            "Wednesday, 21-Oct-26 07:28:00 GMT",
            "Wed, 1 Oct 2026 07:28:00 GMT",
            "Wed, 21 oct 2026 07:28:00 GMT",
            "Xyz, 21 Oct 2026 07:28:00 GMT",
            "Wed, 31 Feb 2026 07:28:00 GMT",
            "Wed, 21 Oct 2026 07:28:60 GMT",
            "Wed Oct 0 07:28:00 2026",
            "Wed Oct 1 07:28:00 2026",
            "Wed Oct 21 07:28:00 26",
        ):
            with self.subTest(text=text):
                self.assertIsNone(parse_retry_after(text, NOW))

    def test_a_trailing_line_break_is_white_space(self):
        self.assertEqual(parse_retry_after("5\n", NOW), 5000)


class MapTransportErrorTest(unittest.TestCase):
    def test_the_reasons_are_fixed(self):
        self.assertEqual(map_transport_error(TimeoutError("http://x/?token=abc")), REASON_TIMEOUT)
        self.assertEqual(map_transport_error(ConnectionRefusedError("http://x/?token=abc")), REASON_NETWORK_ERROR)
        self.assertEqual(map_transport_error(ValueError("x")), REASON_NETWORK_ERROR)
        self.assertEqual(map_transport_error(_ssl_error()), REASON_TLS_VERIFICATION_FAILED)


def _ssl_error() -> BaseException:
    import ssl

    return ssl.SSLCertVerificationError("the text is never used")


class ParseUrlTest(unittest.TestCase):
    def test_the_scheme_and_the_host_are_lower_case(self):
        self.assertEqual(parse_url("HTTPS://Logs.Example.Invalid:8443/x?y=1"), ("https", "logs.example.invalid"))
        self.assertEqual(parse_url("http://[::1]:80/x"), ("http", "::1"))
        self.assertEqual(parse_url("https:///path"), ("https", ""))
        self.assertEqual(parse_url("https://a.invalid:1/"), ("https", "a.invalid"))
        self.assertEqual(parse_url("https://a.invalid:65535/"), ("https", "a.invalid"))

    def test_a_space_a_control_character_or_a_bad_port_is_not_a_url(self):
        for url in ("https://a.invalid/ x", "https://a.invalid/\n", "https://a.invalid:99999/", "http://[::1/"):
            with self.subTest(url=url):
                self.assertIsNone(parse_url(url))


class ValidateSettingsTest(unittest.TestCase):
    def test_a_trailing_line_break_does_not_pass(self):
        self.assertEqual(fields_of(bearer_token="tok\n"), ["bearer_token"])
        self.assertEqual(fields_of(api_key_header="X-Key\n", api_key_value="k"), ["api_key_header"])
        self.assertEqual(fields_of(api_key_header="X-Key", api_key_value="k\n"), ["api_key_value"])

    def test_http_to_every_local_host_may_carry_a_token(self):
        for host in ("localhost", "127.0.0.1", "[::1]", "LOCALHOST"):
            with self.subTest(host=host):
                self.assertEqual(fields_of(url=f"http://{host}:8080/x", bearer_token="tok"), [])

    def test_the_url_reason_for_plain_http_names_no_value(self):
        problems = validate_settings(
            HttpSettings(url="http://secret-host.invalid/x?k=v", api_key_header="X", api_key_value="v")
        )
        self.assertEqual([problem.field for problem in problems], ["url"])
        self.assertNotIn("secret-host", problems[0].reason)
        self.assertNotIn("k=v", problems[0].reason)

    def test_wrong_types_are_problems(self):
        settings = HttpSettings(
            url=5,  # type: ignore[arg-type]
            method=None,  # type: ignore[arg-type]
            timeout_ms=True,
            bearer_token=1,  # type: ignore[arg-type]
            api_key_header=2,  # type: ignore[arg-type]
            api_key_value=3,  # type: ignore[arg-type]
            max_attempts=1.5,  # type: ignore[arg-type]
            retry_base_delay_ms="1",  # type: ignore[arg-type]
            retry_max_delay_ms=0,
        )
        self.assertEqual(
            [problem.field for problem in validate_settings(settings)],
            [
                "url",
                "method",
                "timeout_ms",
                "bearer_token",
                "api_key_header",
                "api_key_value",
                "max_attempts",
                "retry_base_delay_ms",
                "retry_max_delay_ms",
            ],
        )

    def test_a_bad_api_key_value_and_a_bearer_with_a_key_header(self):
        self.assertEqual(fields_of(api_key_header="X-Key", api_key_value="a b"), ["api_key_value"])
        self.assertEqual(fields_of(bearer_token="t", api_key_header="X-Key"), ["api_key_header", "api_key_value"])

    def test_the_maximum_delay_is_compared_only_when_the_base_is_valid(self):
        self.assertEqual(fields_of(retry_base_delay_ms=0, retry_max_delay_ms=5), ["retry_base_delay_ms"])
        self.assertEqual(fields_of(retry_base_delay_ms=5, retry_max_delay_ms=5), [])
