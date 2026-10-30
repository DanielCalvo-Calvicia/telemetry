package masking

import (
	"fmt"
	"sort"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// FIELD CLASS RULES
// =============================================================================

// field_class_rules is the classification of fields: the class of each field name and the rule of each class (drop,
// redact or allow). It is immutable after its construction.
type field_class_rules struct {
	class_by_field map[string]string
	rule_by_class  map[string]string
}

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrMaskRuleInvalid is domain.ErrMaskRuleInvalid.
	ErrMaskRuleInvalid = domain.ErrMaskRuleInvalid
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// new_field_class_rules copies the classes and the rules. The field names are read in snake case (see
// normalize_field_name); when two names read the same, the one that sorts last wins, so the result never depends on the
// order of a map. A rule that is not drop, redact or allow is config_field_invalid with ErrMaskRuleInvalid.
func new_field_class_rules(field_classes map[string]string, class_rules map[string]string) (field_class_rules, error) {
	rule_by_class := make(map[string]string, len(class_rules))
	for _, class_name := range sorted_keys(class_rules) {
		rule := class_rules[class_name]
		if rule != domain.MaskRuleDrop && rule != domain.MaskRuleRedact && rule != domain.MaskRuleAllow {
			return field_class_rules{}, domain.NewClassifiedError(
				value_objects.ErrorCodeConfigFieldInvalid, ErrMaskRuleInvalid, fmt.Errorf("class %q has the rule %q", class_name, rule))
		}
		rule_by_class[class_name] = rule
	}

	class_by_field := make(map[string]string, len(field_classes))
	for _, field_name := range sorted_keys(field_classes) {
		if normalized_name := normalize_field_name(field_name); normalized_name != "" {
			class_by_field[normalized_name] = field_classes[field_name]
		}
	}
	return field_class_rules{class_by_field: class_by_field, rule_by_class: rule_by_class}, nil
}

// ----- METHODS --------------------------------------------------------------

// rule_of returns the rule of the class of a field and whether the field has a class. A class that has no rule is
// redact, the safe default.
func (rules field_class_rules) rule_of(field_name string) (string, bool) {
	class_name, has_class := rules.class_by_field[normalize_field_name(field_name)]
	if !has_class {
		return "", false
	}
	rule, has_rule := rules.rule_by_class[class_name]
	if !has_rule {
		return domain.MaskRuleRedact, true
	}
	return rule, true
}

// ----- HELPERS --------------------------------------------------------------

// sorted_keys returns the keys of a map in order.
func sorted_keys(source_map map[string]string) []string {
	keys := make([]string, 0, len(source_map))
	for key := range source_map {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
