"""The detector of credentials written as a pair."""

from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern
from telemetry.internal.domain.operations.masking.text_detector import TextDetector
from telemetry.internal.domain.rules import (
    CREDENTIAL_PAIR_PATTERN,
    DETECTOR_CREDENTIAL_PAIR,
    REPLACEMENT_KEEP_FIRST_GROUP,
)


def new_credential_pair_detector() -> TextDetector:
    """Create the detector of credentials written as a pair.

    It reads password=..., name: value and X-Amz-Signature=...; the name is kept and the value is hidden.
    """
    return TextDetector(
        DETECTOR_CREDENTIAL_PAIR, compile_shared_pattern(CREDENTIAL_PAIR_PATTERN), REPLACEMENT_KEEP_FIRST_GROUP
    )
