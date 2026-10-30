"""The detector of the password of a URL with credentials."""

from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern
from telemetry.internal.domain.operations.masking.text_detector import TextDetector
from telemetry.internal.domain.rules import (
    CONNECTION_STRING_PASSWORD_PATTERN,
    DETECTOR_CONNECTION_STRING_PASSWORD,
    REPLACEMENT_KEEP_OUTER_GROUPS,
)


def new_connection_string_password_detector() -> TextDetector:
    """Create the detector of the password of a URL with credentials.

    Scheme://user:password@host keeps all but the password.
    """
    return TextDetector(
        DETECTOR_CONNECTION_STRING_PASSWORD,
        compile_shared_pattern(CONNECTION_STRING_PASSWORD_PATTERN),
        REPLACEMENT_KEEP_OUTER_GROUPS,
    )
