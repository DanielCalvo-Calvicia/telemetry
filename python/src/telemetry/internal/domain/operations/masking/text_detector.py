"""The text detector: one kind of sensitive data found inside a text and hidden."""

import re

from telemetry.internal.domain.rules import (
    MASKED_TEXT,
    REPLACEMENT_KEEP_FIRST_GROUP,
    REPLACEMENT_KEEP_OUTER_GROUPS,
    REPLACEMENT_WHOLE,
)


class TextDetector:
    """TextDetector finds one kind of sensitive data inside a text and hides it.

    Every detector of the masker is a TextDetector built by its own module (detect_*.py) from the patterns of
    domain/rules.py. ``replacement_kind`` is one of REPLACEMENT_WHOLE, REPLACEMENT_KEEP_FIRST_GROUP and
    REPLACEMENT_KEEP_OUTER_GROUPS (the Go twin writes them as ``${1}`` templates).
    """

    def __init__(self, name: str, pattern: "re.Pattern[str]", replacement_kind: str = REPLACEMENT_WHOLE) -> None:
        """Keep the name, the compiled pattern and the replacement kind."""
        if replacement_kind not in (REPLACEMENT_WHOLE, REPLACEMENT_KEEP_FIRST_GROUP, REPLACEMENT_KEEP_OUTER_GROUPS):
            raise ValueError("unknown replacement kind")
        self._name = name
        self._pattern = pattern
        self._replacement_kind = replacement_kind

    @property
    def name(self) -> str:
        """The name of the detector."""
        return self._name

    def apply(self, text: str) -> str:
        """Return the text with everything the detector finds hidden; the text itself is not modified."""
        return self._pattern.sub(self._replace, text)

    def _replace(self, match: "re.Match[str]") -> str:
        if self._replacement_kind == REPLACEMENT_KEEP_FIRST_GROUP:
            return (match.group(1) or "") + MASKED_TEXT
        if self._replacement_kind == REPLACEMENT_KEEP_OUTER_GROUPS:
            return (match.group(1) or "") + MASKED_TEXT + (match.group(3) or "")
        return MASKED_TEXT
