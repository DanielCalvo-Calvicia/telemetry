"""Black-box tests of the sanitized reason of a diagnostic: a code, never a message, a value or a URL."""

import unittest

from telemetry.internal.domain.operations.diagnostics_reason.sanitize_reason import (
    MAX_REASON_CHARACTERS,
    UNSPECIFIED_REASON,
    sanitize_reason,
)


class SanitizeReasonTest(unittest.TestCase):
    def test_a_code_is_kept_as_it_is(self):
        for reason in ("status_503", "retry_limit_reached", "a.b:c-d", "ABCxyz019"):
            with self.subTest(reason=reason):
                self.assertEqual(sanitize_reason(reason), reason)

    def test_every_other_character_becomes_an_underscore(self):
        self.assertEqual(sanitize_reason("bad token://host/x?y=1&z"), "bad_token:__host_x_y_1_z")
        self.assertEqual(sanitize_reason("line\nfeed\ttab"), "line_feed_tab")
        self.assertEqual(sanitize_reason("café ☃ \U0001f600"), "caf_____")

    def test_it_is_cut_to_64_characters(self):
        self.assertEqual(MAX_REASON_CHARACTERS, 64)
        self.assertEqual(sanitize_reason("x" * 200), "x" * 64)
        self.assertEqual(sanitize_reason("x" * 64), "x" * 64)

    def test_an_empty_reason_or_one_that_is_not_text_is_unspecified(self):
        self.assertEqual(UNSPECIFIED_REASON, "unspecified")
        for reason in ("", None, 7, b"bytes"):
            with self.subTest(reason=reason):
                self.assertEqual(sanitize_reason(reason), "unspecified")


if __name__ == "__main__":
    unittest.main()
