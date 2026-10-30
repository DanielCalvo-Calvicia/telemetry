"""The detector of the credential after an authorization scheme."""

from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern
from telemetry.internal.domain.operations.masking.text_detector import TextDetector
from telemetry.internal.domain.rules import (
    BEARER_CREDENTIAL_PATTERN,
    DETECTOR_BEARER_CREDENTIAL,
    REPLACEMENT_KEEP_FIRST_GROUP,
)


def new_bearer_credential_detector() -> TextDetector:
    """Create the detector of the credential after an authorization scheme.

    The scheme (Bearer, Basic, Token) is kept and the credential is hidden.
    """
    return TextDetector(
        DETECTOR_BEARER_CREDENTIAL, compile_shared_pattern(BEARER_CREDENTIAL_PATTERN), REPLACEMENT_KEEP_FIRST_GROUP
    )
