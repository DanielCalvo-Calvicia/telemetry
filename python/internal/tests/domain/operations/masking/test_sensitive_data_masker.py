"""The masker: the ported cases of the old library, the new options and the walk of an event's data."""

import datetime
import unittest

from internal.tests.domain.operations.masking.helpers import (
    FAKE_ACCESS_KEY_ID,
    FAKE_JWT,
    FAKE_PRIVATE_KEY,
    FAKE_SECRET_KEY,
)
from telemetry.internal.domain.errors import ClassifiedError, MaskPatternInvalidError, MaskRuleInvalidError
from telemetry.internal.domain.operations.masking.sensitive_data_masker import (
    MaskerOptions,
    SensitiveDataMasker,
    new_sensitive_data_masker,
)
from telemetry.internal.domain.rules import MASKED_TEXT


def make_masker(**options) -> SensitiveDataMasker:
    return new_sensitive_data_masker(MaskerOptions(**options))


class MaskValueTest(unittest.TestCase):
    def test_a_value_is_hidden_by_the_name_of_its_field_whatever_it_holds(self):
        masker = make_masker()
        cases = [
            ("password", "hunter2"),
            ("secret_string", "plain text with no pattern at all"),
            ("secret_binary", b"\x01\x02\x03"),
            ("token", 123456),
            ("credentials", {"user": "bob"}),
            ("api_key", ["a", "b"]),
            ("body", "some file content"),
            ("message_body", '{"order": 17}'),
            ("accessKey", "whatever"),
        ]
        for field_name, value in cases:
            with self.subTest(field_name):
                self.assertEqual(masker.mask_value(field_name, value), MASKED_TEXT)

    def test_a_value_is_hidden_by_what_it_is_even_in_an_innocent_field(self):
        masker = make_masker()
        for value in (FAKE_ACCESS_KEY_ID, FAKE_JWT, FAKE_PRIVATE_KEY):
            self.assertEqual(masker.mask_value("detail", value), MASKED_TEXT)

    def test_innocent_values_are_kept(self):
        masker = make_masker()
        moment = datetime.datetime(2026, 10, 7, 12, 0, tzinfo=datetime.timezone.utc)
        for field_name, value in [
            ("anything", None),
            ("bucket", "my-bucket"),
            ("secret_id", "prod/db/credentials"),
            ("count", 42),
            ("ratio", 0.5),
            ("found", True),
            ("created_at", moment),
        ]:
            with self.subTest(field_name):
                self.assertEqual(masker.mask_value(field_name, value), value)

    def test_none_stays_none_even_under_a_sensitive_name_or_a_class(self):
        masker = make_masker(field_classes={"x": "c"}, class_rules={"c": "redact"})
        self.assertIsNone(masker.mask_value("password", None))
        self.assertIsNone(masker.mask_value("x", None))

    def test_bytes_bytearrays_and_values_that_cannot_be_inspected_are_hidden(self):
        masker = make_masker()
        for value in (b"abc", bytearray(b"abc"), {1, 2}, object(), lambda: None):
            self.assertEqual(masker.mask_value("detail", value), MASKED_TEXT)

    def test_an_error_value_is_masked_as_text(self):
        masker = make_masker()
        masked = masker.mask_value("cause", ValueError("dial postgres://u:hunter2@h/db failed"))
        self.assertEqual(masked, "dial postgres://u:*****MASKED*****@h/db failed")

    def test_containers_are_masked_element_by_element(self):
        masker = make_masker()
        original = {
            "bucket": "my-bucket",
            "password": "hunter2",
            "error": "failed with token=abc123",
            "nested": {"api_key": "k", "count": 3},
            "list": ["password=x", 5, {"secret": "y"}],
            "tuple": ("safe", "password=z"),
        }
        masked = masker.mask_value("details", original)
        self.assertEqual(masked["bucket"], "my-bucket")
        self.assertEqual(masked["password"], MASKED_TEXT)
        self.assertEqual(masked["error"], "failed with token=*****MASKED*****")
        self.assertEqual(masked["nested"], {"api_key": MASKED_TEXT, "count": 3})
        self.assertEqual(masked["list"], ["password=*****MASKED*****", 5, {"secret": MASKED_TEXT}])
        self.assertEqual(masked["tuple"], ["safe", "password=*****MASKED*****"])
        self.assertEqual(original["password"], "hunter2")
        self.assertEqual(original["nested"]["api_key"], "k")
        self.assertEqual(original["list"][0], "password=x")

    def test_a_non_text_key_is_read_as_its_text(self):
        masker = make_masker()
        self.assertEqual(masker.mask_map({1: "password=x"}), {1: "password=*****MASKED*****"})


class MaskTextTest(unittest.TestCase):
    def test_a_whole_text_that_is_sensitive_is_hidden(self):
        masker = make_masker()
        for text in (FAKE_ACCESS_KEY_ID, FAKE_JWT, FAKE_PRIVATE_KEY):
            self.assertEqual(masker.mask_text(text), MASKED_TEXT)

    def test_a_sensitive_segment_inside_a_longer_text_is_hidden_and_the_rest_is_kept(self):
        masker = make_masker()
        cases = [
            (
                "login failed for user=bob password=hunter2 from 10.0.0.1",
                "login failed for user=bob password=*****MASKED***** from 10.0.0.1",
            ),
            (
                "cannot connect to postgres://admin:hunter2@db.local:5432/app: timeout",
                "cannot connect to postgres://admin:*****MASKED*****@db.local:5432/app: timeout",
            ),
            (
                "request with Authorization: Bearer abcdef123456 failed",
                "request with Authorization: Bearer *****MASKED***** failed",
            ),
            ("using " + FAKE_ACCESS_KEY_ID + " in eu-west-1", "using *****MASKED***** in eu-west-1"),
            ("aws_secret_access_key=" + FAKE_SECRET_KEY + " set", "aws_secret_access_key=*****MASKED***** set"),
            ("got " + FAKE_JWT + " from the client", "got *****MASKED***** from the client"),
            ("key: " + FAKE_PRIVATE_KEY + "\nend", "key: *****MASKED*****\nend"),
            (
                "https://bucket.s3.eu-west-1.amazonaws.com/file?X-Amz-Credential="
                + FAKE_ACCESS_KEY_ID
                + "%2F20261007%2Feu-west-1%2Fs3%2Faws4_request&X-Amz-Signature=0123456789abcdef&X-Amz-Expires=300",
                "https://bucket.s3.eu-west-1.amazonaws.com/file?X-Amz-Credential=*****MASKED*****"
                "&X-Amz-Signature=*****MASKED*****&X-Amz-Expires=300",
            ),
            ("password=a token=b", "password=*****MASKED***** token=*****MASKED*****"),
            ("object copied from bucket a to bucket b", "object copied from bucket a to bucket b"),
            ("", ""),
        ]
        for text, expected in cases:
            with self.subTest(text[:30]):
                self.assertEqual(masker.mask_text(text), expected)

    def test_masking_a_masked_text_changes_nothing(self):
        masker = make_masker(extra_patterns=["order-[0-9]+"])
        text = (
            "password=hunter2 Authorization: Bearer abcdef123456 "
            + FAKE_ACCESS_KEY_ID
            + " order-123 postgres://u:p@h/db"
        )
        once = masker.mask_text(text)
        self.assertEqual(masker.mask_text(once), once)


class UnicodeFoldingTest(unittest.TestCase):
    def test_a_secret_written_with_the_kelvin_sign_or_the_long_s_is_hidden(self):
        masker = make_masker()
        for text, expected in [
            ("paſſword=hunter2", "paſſword=*****MASKED*****"),
            ("api_Key=abc", "api_Key=*****MASKED*****"),
            ("authorization=token Khunter2x", "authorization=token *****MASKED*****"),
            ("Bearer abcdeſgh1234", "Bearer *****MASKED*****"),
            ('{"token": "abKſ c"}', '{"token": *****MASKED*****}'),
        ]:
            with self.subTest(text):
                self.assertEqual(masker.mask_text(text), expected)

    def test_an_extra_case_insensitive_pattern_folds_too(self):
        masker = make_masker(extra_patterns=["(?i)ticket-[a-z]+"])
        self.assertEqual(masker.mask_text("see TICKET-abK ok"), "see *****MASKED***** ok")


class OptionsTest(unittest.TestCase):
    def test_extra_patterns_hide_every_match_whole(self):
        masker = make_masker(extra_patterns=["order-[0-9]{2,5}", "employee:[a-z]+"])
        self.assertEqual(
            masker.mask_text("shipped order-12345 for employee:bob and order-7"),
            "shipped *****MASKED***** for *****MASKED***** and order-7",
        )

    def test_an_empty_extra_pattern_is_skipped_and_does_not_hide_everything(self):
        masker = make_masker(extra_patterns=["", "never-matches-[0-9]+"])
        self.assertEqual(masker.mask_text("hello world"), "hello world")

    def test_an_invalid_extra_pattern_is_an_error_with_the_code_and_the_cause_but_no_data(self):
        with self.assertRaises(MaskPatternInvalidError) as raised:
            make_masker(extra_patterns=["valid-[0-9]+", "broken("])
        self.assertEqual(raised.exception.code, "config_field_invalid")
        self.assertIsInstance(raised.exception, ClassifiedError)
        self.assertIsNotNone(raised.exception.__cause__)
        self.assertNotIn("broken(", str(raised.exception))

    def test_an_invalid_class_rule_is_an_error_with_the_code(self):
        with self.assertRaises(MaskRuleInvalidError) as raised:
            make_masker(class_rules={"pii": "keep"})
        self.assertEqual(raised.exception.code, "config_field_invalid")

    def test_field_names_replace_the_default_list_and_the_detectors_stay_on(self):
        masker = make_masker(field_names=["pin"])
        self.assertEqual(masker.mask_value("pin", "1234"), MASKED_TEXT)
        self.assertEqual(masker.mask_value("user_pin", "1234"), MASKED_TEXT)
        self.assertEqual(masker.mask_value("password", "plain"), "plain")
        self.assertEqual(masker.mask_text("password=x"), "password=*****MASKED*****")

    def test_an_empty_list_of_names_replaces_the_default_list(self):
        masker = make_masker(field_names=[])
        self.assertEqual(masker.mask_value("password", "plain"), "plain")
        self.assertEqual(masker.mask_value("body", "plain"), "plain")

    def test_extra_field_names_are_added_to_the_default_list(self):
        masker = make_masker(extra_field_names=["SSN", ""])
        self.assertEqual(masker.mask_value("ssn", "1"), MASKED_TEXT)
        self.assertEqual(masker.mask_value("password", "1"), MASKED_TEXT)
        self.assertEqual(masker.mask_value("other", "1"), "1")
        replaced = make_masker(field_names=["pin"], extra_field_names=["ssn"])
        self.assertEqual(replaced.mask_value("ssn", "1"), MASKED_TEXT)
        self.assertEqual(replaced.mask_value("pin", "1"), MASKED_TEXT)
        self.assertEqual(replaced.mask_value("password", "1"), "1")

    def test_the_default_options_can_be_left_out(self):
        self.assertEqual(SensitiveDataMasker().mask_value("password", "x"), MASKED_TEXT)
        self.assertEqual(new_sensitive_data_masker().mask_value("token", "x"), MASKED_TEXT)

    def test_the_options_are_copied_so_a_later_change_changes_nothing(self):
        classes = {"x": "pii"}
        rules = {"pii": "drop"}
        masker = make_masker(field_classes=classes, class_rules=rules)
        classes["y"] = "pii"
        rules["pii"] = "allow"
        self.assertEqual(masker.mask_map({"x": 1, "y": 2}), {"y": 2})


class ClassificationTest(unittest.TestCase):
    def test_drop_removes_the_field_at_any_depth_and_by_any_spelling(self):
        masker = make_masker(field_classes={"patient_name": "pii"}, class_rules={"pii": "drop"})
        masked = masker.mask_map(
            {"PatientName": "Ann", "keep": 1, "n": {"patient-name": "Bob", "list": [{"patient_name": "C"}]}}
        )
        self.assertEqual(masked, {"keep": 1, "n": {"list": [{}]}})

    def test_drop_alone_does_not_leak_through_mask_value(self):
        masker = make_masker(field_classes={"patient_name": "pii"}, class_rules={"pii": "drop"})
        self.assertEqual(masker.mask_value("patient_name", "password=x"), "password=*****MASKED*****")
        self.assertEqual(masker.mask_value("patient_name", b"raw"), MASKED_TEXT)

    def test_redact_replaces_the_value(self):
        masker = make_masker(field_classes={"email": "pii"}, class_rules={"pii": "redact"})
        self.assertEqual(masker.mask_value("email", "a@b.c"), MASKED_TEXT)
        self.assertEqual(masker.mask_value("email", {"a": 1}), MASKED_TEXT)

    def test_a_class_without_a_rule_is_redacted(self):
        masker = make_masker(field_classes={"x": "mystery"})
        self.assertEqual(masker.mask_value("x", "v"), MASKED_TEXT)
        self.assertEqual(masker.rule_of_field("x"), "redact")

    def test_allow_skips_the_name_rule_only(self):
        masker = make_masker(field_classes={"body": "internal"}, class_rules={"internal": "allow"})
        self.assertEqual(masker.mask_value("body", "visible"), "visible")
        self.assertEqual(masker.mask_value("body", "password=hunter2"), "password=*****MASKED*****")
        self.assertEqual(masker.mask_value("body", {"token": "t", "ok": "fine"}), {"token": MASKED_TEXT, "ok": "fine"})
        self.assertEqual(masker.mask_value("body", ["Bearer abcdef123456"]), ["Bearer *****MASKED*****"])

    def test_a_field_without_a_class_has_no_rule(self):
        masker = make_masker(field_classes={"x": "c"}, class_rules={"c": "allow"})
        self.assertIsNone(masker.rule_of_field("y"))
        self.assertEqual(masker.rule_of_field("X"), "allow")
        self.assertTrue(masker.is_sensitive_name("db_password"))
