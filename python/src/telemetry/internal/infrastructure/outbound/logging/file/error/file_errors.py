"""The classified errors of the file destination, and the reasons it puts in a SendResult.

The delivery codes of the catalog arrive with T-05: until then these errors keep the default code of
``ClassifiedError`` and the class says what failed. The text of the cause is never copied into the message: it is
chained (``raise ... from cause``) and the caller can read it there.
"""

from telemetry.internal.domain.errors import ClassifiedError

# The reasons of a SendResult: the SAME texts as the Go half.
REASON_WRITE_FAILED = "write_failed"
REASON_ROTATION_FAILED = "rotation_failed"
REASON_SERIALIZATION_FAILED = "serialization_failed"
REASON_DESTINATION_FAILED = "destination_failed"


class FileWriteFailedError(ClassifiedError):
    """A line could not be appended to the file; the cause is in ``__cause__``."""


class FileRotationFailedError(ClassifiedError):
    """The files could not be rotated; the cause is in ``__cause__``."""


class FileSerializationFailedError(ClassifiedError):
    """An event could not be turned into its line; the cause is in ``__cause__``."""
