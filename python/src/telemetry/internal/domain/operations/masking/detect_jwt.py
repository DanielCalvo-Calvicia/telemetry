"""The detector of JSON Web Tokens."""

from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern
from telemetry.internal.domain.operations.masking.text_detector import TextDetector
from telemetry.internal.domain.rules import (
    DETECTOR_JWT,
    JWT_PATTERN,
    REPLACEMENT_WHOLE,
)


def new_jwt_detector() -> TextDetector:
    """Create the detector of JSON Web Tokens.

    The whole token is hidden.
    """
    return TextDetector(DETECTOR_JWT, compile_shared_pattern(JWT_PATTERN), REPLACEMENT_WHOLE)
