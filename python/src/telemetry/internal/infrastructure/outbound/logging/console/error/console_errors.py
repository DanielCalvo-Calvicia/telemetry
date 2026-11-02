"""The classified errors of the console destination.

The delivery codes of the catalog arrive with T-05: until then these errors keep the default code of
``ClassifiedError`` (there is no cleaner way without inventing a code), and the class says what failed. The text of
the cause is never copied into the message: it is chained (``raise ... from cause``) and the caller can read it there.
"""

from telemetry.internal.domain.errors import ClassifiedError


class ConsoleWriteFailedError(ClassifiedError):
    """The writer of the console failed while writing one line; the cause is in ``__cause__``."""


class ConsoleEncodingFailedError(ClassifiedError):
    """An event could not be turned into its line (JSON or text); the cause is in ``__cause__``."""
