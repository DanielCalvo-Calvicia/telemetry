"""Black-box tests of the standard output of the console destination (its module is outside the coverage)."""

import io
import sys
import unittest
from unittest import mock

from telemetry.internal.infrastructure.outbound.logging.console.external.standard_output import Utf8StandardOutput


class BufferedStream:
    """A stream like sys.stdout: a text layer over a binary buffer, with a console encoding that is not UTF-8."""

    def __init__(self) -> None:
        self.buffer = io.BytesIO()
        self.flushes = 0
        self.text_writes: list[str] = []

    def write(self, text: str) -> int:
        self.text_writes.append(text)
        text.encode("cp1252")  # what a Windows console would do
        return len(text)


class Unbuffered:
    """A stream with no binary buffer."""

    def __init__(self) -> None:
        self.text = ""
        self.flushes = 0

    def write(self, text: str) -> int:
        self.text += text
        return len(text)

    def flush(self) -> None:
        self.flushes += 1


class Utf8StandardOutputTest(unittest.TestCase):
    def test_it_writes_utf8_bytes_to_the_buffer_even_when_the_console_encoding_cannot_hold_them(self):
        stream = BufferedStream()
        with mock.patch.object(sys, "stdout", stream):
            Utf8StandardOutput().write("température \U0001f600\n")
        self.assertEqual(stream.buffer.getvalue(), "température \U0001f600\n".encode("utf-8"))
        self.assertEqual(stream.text_writes, [])

    def test_it_flushes_the_buffer(self):
        class Tracked(io.BytesIO):
            flushed = 0

            def flush(self) -> None:
                Tracked.flushed += 1

        stream = BufferedStream()
        stream.buffer = Tracked()
        with mock.patch.object(sys, "stdout", stream):
            Utf8StandardOutput().write("line\n")
        self.assertEqual(Tracked.flushed, 1)

    def test_it_writes_to_the_text_stream_when_there_is_no_buffer(self):
        stream = Unbuffered()
        with mock.patch.object(sys, "stdout", stream):
            Utf8StandardOutput().write("line\n")
        self.assertEqual(stream.text, "line\n")
        self.assertEqual(stream.flushes, 1)
