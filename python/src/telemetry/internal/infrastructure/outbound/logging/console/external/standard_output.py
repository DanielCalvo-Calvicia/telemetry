"""The standard output of the process: the only module of the console destination that touches ``sys.stdout``."""

import sys


class Utf8StandardOutput:
    """Write text to the standard output as UTF-8, whatever the encoding of the console.

    A console encoding such as cp1252 would fail on a character outside it, and the Go half writes UTF-8: the text is
    encoded here and written to the binary buffer when the stream has one, else to the text stream.
    """

    def write(self, text: str) -> None:
        """Write the text and flush, so a line reaches the console when ``write`` returns."""
        stream = sys.stdout
        buffer = getattr(stream, "buffer", None)
        if buffer is not None:
            buffer.write(text.encode("utf-8"))
            buffer.flush()
        else:
            stream.write(text)
            stream.flush()
