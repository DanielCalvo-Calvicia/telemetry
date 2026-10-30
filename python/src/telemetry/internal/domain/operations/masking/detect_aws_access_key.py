"""The detector of AWS access key IDs (AKIA..., ASIA..., ...)."""

from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern
from telemetry.internal.domain.operations.masking.text_detector import TextDetector
from telemetry.internal.domain.rules import (
    AWS_ACCESS_KEY_PATTERN,
    DETECTOR_AWS_ACCESS_KEY,
    REPLACEMENT_WHOLE,
)


def new_aws_access_key_detector() -> TextDetector:
    """Create the detector of AWS access key IDs (AKIA..., ASIA..., ...).

    The whole key is hidden.
    """
    return TextDetector(DETECTOR_AWS_ACCESS_KEY, compile_shared_pattern(AWS_ACCESS_KEY_PATTERN), REPLACEMENT_WHOLE)
