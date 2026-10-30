"""The sensitive data masker (ADR-006): it hides sensitive data in an event before any sink sees it."""

import re
from dataclasses import dataclass, field
from typing import Any, Mapping, Optional, Sequence

from telemetry.internal.domain.errors import MaskPatternInvalidError
from telemetry.internal.domain.operations.masking.detect_aws_access_key import new_aws_access_key_detector
from telemetry.internal.domain.operations.masking.detect_bearer_credential import new_bearer_credential_detector
from telemetry.internal.domain.operations.masking.detect_connection_string_password import (
    new_connection_string_password_detector,
)
from telemetry.internal.domain.operations.masking.detect_credential_pair import new_credential_pair_detector
from telemetry.internal.domain.operations.masking.detect_jwt import new_jwt_detector
from telemetry.internal.domain.operations.masking.detect_private_key_block import new_private_key_block_detector
from telemetry.internal.domain.operations.masking.field_class_rules import FieldClassRules
from telemetry.internal.domain.operations.masking.is_sensitive_field_name import (
    DEFAULT_SENSITIVE_NAMES,
    is_sensitive_field_name,
    normalize_field_names,
)
from telemetry.internal.domain.operations.masking.mask_value import mask_value
from telemetry.internal.domain.operations.masking.shared_pattern import compile_shared_pattern
from telemetry.internal.domain.operations.masking.text_detector import TextDetector
from telemetry.internal.domain.rules import MASK_RULE_DROP


@dataclass(frozen=True)
class MaskerOptions:
    """MaskerOptions are the options of the masker; they come from the configuration (T-03.6), never from the core.

    ``extra_patterns`` are regular expressions (written for Go, see shared_pattern.py) whose every match is hidden.
    ``field_names`` is the list of sensitive field names: None keeps the default list of domain/rules.py and any
    sequence, even an empty one, REPLACES it; ``extra_field_names`` are added to the active list. ``field_classes``
    gives the class of a field name and ``class_rules`` the rule of a class: ``drop``, ``redact`` or ``allow``.
    """

    extra_patterns: Sequence[str] = ()
    field_names: Optional[Sequence[str]] = None
    extra_field_names: Sequence[str] = ()
    field_classes: Mapping[str, str] = field(default_factory=dict)
    class_rules: Mapping[str, str] = field(default_factory=dict)


class SensitiveDataMasker:
    """SensitiveDataMasker hides sensitive data in an event: a field by its name or class, a value, a text segment.

    Whatever is found is replaced by MASKED_TEXT. The six built-in detectors are always on, even when the options
    replace the field names. The masker never modifies what it receives: it returns masked copies. It is immutable
    and safe for concurrent use.

    Creating it raises MaskPatternInvalidError for an extra pattern that does not compile and MaskRuleInvalidError for
    a class rule that is not drop, redact or allow (both with the code ``config_field_invalid``), so that the start
    fails instead of logging without the protection.
    """

    def __init__(self, options: Optional[MaskerOptions] = None) -> None:
        """Build the detectors and the rules from the options (the defaults when none are given)."""
        options = options if options is not None else MaskerOptions()
        detectors = [
            new_private_key_block_detector(),
            new_connection_string_password_detector(),
            new_bearer_credential_detector(),
            new_jwt_detector(),
            new_aws_access_key_detector(),
            new_credential_pair_detector(),
        ]
        for extra_pattern in options.extra_patterns:
            if extra_pattern == "":
                continue
            try:
                compiled_pattern = compile_shared_pattern(extra_pattern)
            except re.error as compile_error:
                raise MaskPatternInvalidError("an extra masking pattern is not a valid regular expression") from (
                    compile_error
                )
            detectors.append(TextDetector(extra_pattern, compiled_pattern))
        self._text_detectors = tuple(detectors)

        base_names = (
            DEFAULT_SENSITIVE_NAMES if options.field_names is None else normalize_field_names(options.field_names)
        )
        self._sensitive_names = base_names + normalize_field_names(options.extra_field_names)
        self._class_rules = FieldClassRules(options.field_classes, options.class_rules)

    def mask_text(self, text: str) -> str:
        """Return the text with every sensitive segment hidden, the whole text or a part. It is idempotent."""
        for text_detector in self._text_detectors:
            text = text_detector.apply(text)
        return text

    def mask_value(self, field_name: str, value: Any) -> Any:
        """Return a masked copy of a value of the field called field_name (see mask_value.py for the rules)."""
        return mask_value(self, field_name, value)

    def mask_map(self, mapping: Optional[Mapping[str, Any]]) -> Optional[dict[str, Any]]:
        """Return a masked copy of a mapping, without the keys of a class with the rule drop. None gives None."""
        if mapping is None:
            return None
        masked_map: dict[str, Any] = {}
        for key, value in mapping.items():
            field_name = key if isinstance(key, str) else str(key)
            if self._class_rules.rule_of(field_name) == MASK_RULE_DROP:
                continue
            masked_map[key] = self.mask_value(field_name, value)
        return masked_map

    def rule_of_field(self, field_name: str) -> Optional[str]:
        """The rule of the class of the field (drop, redact or allow), or None when it has no class."""
        return self._class_rules.rule_of(field_name)

    def is_sensitive_name(self, field_name: str) -> bool:
        """Whether the value of a field with this name is always hidden, by the active list of names."""
        return is_sensitive_field_name(field_name, self._sensitive_names)


def new_sensitive_data_masker(options: Optional[MaskerOptions] = None) -> SensitiveDataMasker:
    """Create a SensitiveDataMasker from the options; it raises a ClassifiedError for an invalid option."""
    return SensitiveDataMasker(options)
