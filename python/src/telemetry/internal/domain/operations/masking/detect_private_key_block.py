"""The detector of PEM private key blocks."""

from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern
from telemetry.internal.domain.operations.masking.text_detector import TextDetector
from telemetry.internal.domain.rules import (
    DETECTOR_PRIVATE_KEY_BLOCK,
    PRIVATE_KEY_BLOCK_PATTERN,
    REPLACEMENT_WHOLE,
)


def new_private_key_block_detector() -> TextDetector:
    """Create the detector of PEM private key blocks.

    The whole block is hidden, to the end of the text when it is cut.
    """
    return TextDetector(
        DETECTOR_PRIVATE_KEY_BLOCK, compile_shared_pattern(PRIVATE_KEY_BLOCK_PATTERN), REPLACEMENT_WHOLE
    )
