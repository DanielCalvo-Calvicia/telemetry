"""Black-box tests of the two encoders of the console destination."""

import json
import unittest
from typing import Any

from internal.tests.application.services.masking_helpers import envelope_from_json
from telemetry.internal.domain.errors import ClassifiedError
from telemetry.internal.domain.operations.canonical_json.canonical_json import canonical_json
from telemetry.internal.infrastructure.outbound.logging.console.helpers.encode_json_line import encode_json_line
from telemetry.internal.infrastructure.outbound.logging.console.helpers.encode_text_line import (
    encode_text_line,
    escape_text,
)

OCCURRED_AT = "2026-10-28T10:00:00.123456Z"


def event(severity: str = "INFO", data: Any = None, attributes: Any = None, event_type: str = "log.entry.recorded"):
    """An envelope with the given parts."""
    fields: dict[str, Any] = {
        "schema_version": "0.1",
        "event_id": "01900000-0000-7000-8000-000000000001",
        "event_type": event_type,
        "occurred_at": OCCURRED_AT,
        "recorded_at": OCCURRED_AT,
        "severity": severity,
        "resource": {"service_name": "sensor-hub", "instance_id": "hub-01"},
        "data": {"category": "http", "message": "started"} if data is None else data,
    }
    if attributes is not None:
        fields["attributes"] = attributes
    return envelope_from_json(fields)


class EscapeTextTest(unittest.TestCase):
    def test_every_class_of_character(self):
        table = [
            ("plain", "plain"),
            ("a\nb", "a\\nb"),
            ("a\rb", "a\\rb"),
            ("a\tb", "a\\tb"),
            ("\x00", "\\u0000"),
            ("\x01", "\\u0001"),
            ("\x08", "\\u0008"),
            ("\x0b", "\\u000b"),
            ("\x1b[0m", "\\u001b[0m"),
            ("\x1f", "\\u001f"),
            ("\x7f", "\\u007f"),
            (" ", " "),
            ("~", "~"),
            ("\x80", "\x80"),
            ("señal", "señal"),
            ("\U0001f600", "\U0001f600"),
            ('back\\slash "quote"', 'back\\slash "quote"'),
            ("", ""),
        ]
        for text, expected in table:
            with self.subTest(text=text):
                self.assertEqual(escape_text(text), expected)

    def test_the_result_has_no_control_character(self):
        result = escape_text("".join(chr(code) for code in range(0, 0x80)))
        self.assertTrue(all(character >= " " and character != "\x7f" for character in result))


class EncodeJsonLineTest(unittest.TestCase):
    def test_the_line_is_the_canonical_json_and_a_line_feed(self):
        envelope = event(attributes={"order": 7})
        line = encode_json_line(envelope)
        self.assertEqual(line, canonical_json(envelope).decode("utf-8") + "\n")
        self.assertEqual(line.count("\n"), 1)
        self.assertEqual(json.loads(line)["attributes"], {"order": 7})

    def test_an_event_without_a_canonical_form_raises_a_classified_error(self):
        with self.assertRaises(ClassifiedError):
            encode_json_line(event(attributes={"ratio": float("nan")}))


class EncodeTextLineTest(unittest.TestCase):
    def test_a_log_entry(self):
        self.assertEqual(encode_text_line(event()), f"{OCCURRED_AT} INFO     [http] started\n")

    def test_the_severity_is_padded_to_eight_characters(self):
        for severity in ("TRACE", "DEBUG", "INFO", "WARN", "ERROR", "CRITICAL"):
            with self.subTest(severity=severity):
                line = encode_text_line(event(severity=severity))
                self.assertEqual(line, f"{OCCURRED_AT} {severity.ljust(8)} [http] started\n")

    def test_a_function_is_printed_with_a_colon(self):
        data = {"category": "http", "function": "serve", "message": "slow"}
        self.assertEqual(encode_text_line(event(data=data)), f"{OCCURRED_AT} INFO     [http] serve: slow\n")

    def test_an_empty_or_missing_or_non_text_function_is_left_out(self):
        for function_value in ("", None, 7, ["x"]):
            data = {"category": "http", "message": "slow"}
            if function_value is not None:
                data["function"] = function_value
            with self.subTest(function=function_value):
                self.assertEqual(encode_text_line(event(data=data)), f"{OCCURRED_AT} INFO     [http] slow\n")

    def test_a_missing_category_and_a_non_text_message_are_empty_text(self):
        self.assertEqual(
            encode_text_line(event(data={"message": "only message"})), f"{OCCURRED_AT} INFO     [] only message\n"
        )
        self.assertEqual(
            encode_text_line(event(data={"category": "http", "message": 42})), f"{OCCURRED_AT} INFO     [http] \n"
        )
        self.assertEqual(encode_text_line(event(data={"category": None})), f"{OCCURRED_AT} INFO     [] \n")

    def test_the_text_is_escaped_so_the_line_stays_one_line(self):
        data = {"category": "a\nb", "function": "f\tg", "message": "x\r\ny\x1b"}
        line = encode_text_line(event(data=data))
        self.assertEqual(line, f"{OCCURRED_AT} INFO     [a\\nb] f\\tg: x\\r\\ny\\u001b\n")
        self.assertEqual(line.count("\n"), 1)

    def test_the_attributes_are_sorted_and_printed_as_canonical_json(self):
        attributes = {"zeta": 'text "q"', "alpha": [1, ["b", True], None], "mid": 1.5, "ok": False, "n": 7}
        line = encode_text_line(event(attributes=attributes))
        expected = (
            f"{OCCURRED_AT} INFO     [http] started"
            ' alpha=[1,["b",true],null] mid=1.5 n=7 ok=false zeta="text \\"q\\""\n'
        )
        self.assertEqual(line, expected)

    def test_a_key_that_needs_escaping(self):
        line = encode_text_line(event(attributes={"a\nb": "v", "c d": 1}))
        self.assertEqual(line, f'{OCCURRED_AT} INFO     [http] started a\\nb="v" c d=1\n')

    def test_the_keys_are_sorted_by_utf16_code_units(self):
        # U+1F600 is the pair D83D DE00 in UTF-16, which sorts before U+FF5E, unlike its code point.
        line = encode_text_line(event(attributes={"～": 1, "\U0001f600": 2}))
        self.assertTrue(line.endswith(" \U0001f600=2 ～=1\n"))

    def test_empty_attributes_print_nothing(self):
        self.assertEqual(encode_text_line(event(attributes={})), f"{OCCURRED_AT} INFO     [http] started\n")

    def test_another_event_type_prints_its_type_and_the_canonical_json_of_its_data(self):
        envelope = event(event_type="report.sample.taken", data={"b": 2, "a": "x\ny"}, attributes={"k": "v"})
        self.assertEqual(
            encode_text_line(envelope), f'{OCCURRED_AT} INFO     report.sample.taken {{"a":"x\\ny","b":2}} k="v"\n'
        )

    def test_non_ascii_text_is_kept(self):
        data = {"category": "señal", "message": "température \U0001f600"}
        self.assertEqual(encode_text_line(event(data=data)), f"{OCCURRED_AT} INFO     [señal] température \U0001f600\n")

    def test_a_value_without_a_canonical_form_raises_a_classified_error(self):
        with self.assertRaises(ClassifiedError):
            encode_text_line(event(attributes={"ratio": float("inf")}))
        with self.assertRaises(ClassifiedError):
            encode_text_line(event(event_type="report.sample.taken", data={"ratio": float("nan")}))
