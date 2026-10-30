"""The classification of fields: an optional class per field name and a rule per class (REQ-SEC-003)."""

from typing import Mapping, Optional

from telemetry.internal.domain.errors import MaskRuleInvalidError
from telemetry.internal.domain.operations.masking.is_sensitive_field_name import normalize_field_name
from telemetry.internal.domain.rules import MASK_RULE_REDACT, MASK_RULES


class FieldClassRules:
    """FieldClassRules gives the rule (drop, redact or allow) of a field from its class. It is immutable.

    A field that has no class has no rule. A field of a class that has no rule in ``class_rules`` is redacted: the safe
    default. The names of the fields are read in snake case, like the sensitive names.
    """

    def __init__(self, field_classes: Mapping[str, str], class_rules: Mapping[str, str]) -> None:
        """Check the rules (a rule that is not drop, redact or allow raises MaskRuleInvalidError) and keep them."""
        for rule in class_rules.values():
            if rule not in MASK_RULES:
                raise MaskRuleInvalidError("the rule of a class must be drop, redact or allow")
        self._rule_by_name = {
            normalize_field_name(field_name): class_rules.get(field_class, MASK_RULE_REDACT)
            for field_name, field_class in field_classes.items()
        }

    def rule_of(self, field_name: str) -> Optional[str]:
        """The rule of the field, or None when the field has no class."""
        return self._rule_by_name.get(normalize_field_name(field_name))
