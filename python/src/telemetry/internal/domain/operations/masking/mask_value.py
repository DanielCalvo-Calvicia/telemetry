"""Mask a single value that belongs to a field: by the name of the field, by its class and by what the value is."""

import datetime
from typing import Any, Protocol

from telemetry.internal.domain.rules import MASK_RULE_ALLOW, MASK_RULE_REDACT, MASKED_TEXT


class ValueMasker(Protocol):
    """What mask_value needs from the masker that walks the containers."""

    def mask_text(self, text: str) -> str:
        """Mask a text."""

    def mask_map(self, mapping: Any) -> Any:
        """Mask a copy of a mapping."""

    def rule_of_field(self, field_name: str) -> Any:
        """The rule of the class of a field, or None."""

    def is_sensitive_name(self, field_name: str) -> bool:
        """Whether the name of the field is sensitive."""


def mask_value(masker: ValueMasker, field_name: str, value: Any) -> Any:
    """Return a masked copy of a single value that belongs to the field called field_name.

    An empty name stands for a value without one, such as a list element. The rules, in order:

    * None stays None.
    * A field of a class with the rule ``redact`` (or a class without a rule) is replaced by the masked text. The
      rule ``drop`` is applied by the map that holds the field (mask_map leaves the key out); a value that reaches
      this function with that rule is masked like any other, never returned as it is. The rule ``allow`` skips the
      next step only.
    * A field with a sensitive name is replaced by the masked text whatever it holds, containers included.
    * A text is masked by the text detectors; an error becomes its masked text; bytes are always hidden; a bool, an
      int, a float and a datetime are kept; a dict is masked key by key with the name of each key; a list or a tuple
      becomes a list masked element by element.
    * Any other type is hidden, because it cannot be inspected.
    """
    if value is None:
        return None
    rule = masker.rule_of_field(field_name)
    if rule == MASK_RULE_REDACT:
        return MASKED_TEXT
    if rule != MASK_RULE_ALLOW and masker.is_sensitive_name(field_name):
        return MASKED_TEXT
    if isinstance(value, str):
        return masker.mask_text(value)
    if isinstance(value, (bool, int, float, datetime.datetime)):
        return value
    if isinstance(value, (bytes, bytearray)):
        return MASKED_TEXT
    if isinstance(value, BaseException):
        return masker.mask_text(str(value))
    if isinstance(value, dict):
        return masker.mask_map(value)
    if isinstance(value, (list, tuple)):
        return [mask_value(masker, "", element) for element in value]
    return MASKED_TEXT
