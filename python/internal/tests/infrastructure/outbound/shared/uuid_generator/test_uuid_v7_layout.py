"""Tests of the pure layout of a UUIDv7, against values computed by hand (the Go half checks the same ones)."""

import unittest

from telemetry.internal.infrastructure.outbound.shared.uuid_generator.helpers.uuid_v7_layout import uuid_v7_text

# (milliseconds, ten random bytes, expected text). Each one was derived by hand from the layout of RFC 9562:
# 6 bytes of time, then 0x70 | low nibble of byte 0, byte 1, 0x80 | low six bits of byte 2, bytes 3 to 9.
TABLE = (
    (0, bytes(10), "00000000-0000-7000-8000-000000000000"),
    (0x0123456789AB, b"\xff" * 10, "01234567-89ab-7fff-bfff-ffffffffffff"),
    (1, bytes(range(10)), "00000000-0001-7001-8203-040506070809"),
    (
        1_700_000_000_000,
        bytes.fromhex("abcdef123456789abcde"),
        "018bcfe5-6800-7bcd-af12-3456789abcde",
    ),
    (0xFFFFFFFFFFFF, bytes.fromhex("f00040000000000000" + "01"), "ffffffff-ffff-7000-8000-000000000001"),
    (1_700_000_000_000, b"\x0a" * 10, "018bcfe5-6800-7a0a-8a0a-0a0a0a0a0a0a"),
    (0x0123456789AB, b"\xc0" * 10, "01234567-89ab-70c0-80c0-c0c0c0c0c0c0"),
)


class UuidV7LayoutTest(unittest.TestCase):
    def test_the_table_of_hand_computed_values(self):
        for milliseconds, random_bytes, expected in TABLE:
            with self.subTest(expected=expected):
                self.assertEqual(uuid_v7_text(milliseconds, random_bytes), expected)

    def test_a_bytearray_is_accepted(self):
        self.assertEqual(uuid_v7_text(0, bytearray(10)), "00000000-0000-7000-8000-000000000000")

    def test_the_time_out_of_range_is_refused(self):
        for milliseconds in (-1, 1 << 48, True, "1", 1.5, None):
            with self.subTest(milliseconds=milliseconds):
                with self.assertRaises(ValueError):
                    uuid_v7_text(milliseconds, bytes(10))  # type: ignore[arg-type]

    def test_other_than_ten_bytes_is_refused(self):
        for random_bytes in (b"", bytes(9), bytes(11), "0123456789", None):
            with self.subTest(random_bytes=random_bytes):
                with self.assertRaises(ValueError):
                    uuid_v7_text(0, random_bytes)  # type: ignore[arg-type]


if __name__ == "__main__":
    unittest.main()
