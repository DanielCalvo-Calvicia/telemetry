package masking

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// MaskValue returns a masked copy of a single value that belongs to the field called field_name (an empty name
// for a value without one, such as a list element). Rules, in order:
//   - a nil value stays nil;
//   - a field whose class has the rule redact (or no rule) is replaced by domain.MaskedText whatever it holds;
//   - a field with a sensitive name (the active list of the masker) is replaced by domain.MaskedText whatever it holds,
//     including maps and lists, unless its class has the rule allow, which skips this rule only;
//   - a text is masked by MaskText, an error is turned into its masked text and bytes are always hidden;
//   - numbers, booleans and times are kept;
//   - maps and lists are masked element by element, with the name of each map key;
//   - any other value (a struct, a pointer, a typed map) is read through its JSON form and masked like the
//     rest; a value that cannot be encoded is hidden, because it cannot be inspected.
//
// The rule drop is applied by the enclosing map (MaskMap and the maps inside the value), which leaves the key out; a
// call of MaskValue alone cannot remove a field, so it masks the value by the rules above.
func (sensitive_data_masker SensitiveDataMasker) MaskValue(field_name string, value any) any {
	if value == nil {
		return nil
	}
	class_rule, has_class := sensitive_data_masker.class_rules.rule_of(field_name)
	if has_class && class_rule == domain.MaskRuleRedact {
		return domain.MaskedText
	}
	if !(has_class && class_rule == domain.MaskRuleAllow) && sensitive_data_masker.is_sensitive(field_name) {
		return domain.MaskedText
	}

	switch typed_value := value.(type) {
	case string:
		return sensitive_data_masker.MaskText(typed_value)
	case []byte:
		return domain.MaskedText
	case error:
		return sensitive_data_masker.MaskText(typed_value.Error())
	case time.Time, time.Duration:
		return typed_value
	case map[string]any:
		return sensitive_data_masker.mask_map(typed_value)
	case value_objects.Attributes:
		return sensitive_data_masker.mask_map(typed_value)
	case []any:
		return sensitive_data_masker.mask_list(typed_value)
	case []string:
		masked_texts := make([]string, len(typed_value))
		for index, text := range typed_value {
			masked_texts[index] = sensitive_data_masker.MaskText(text)
		}
		return masked_texts
	}

	if is_plain_scalar(value) {
		return value
	}
	return sensitive_data_masker.mask_through_json(value)
}

// mask_map masks every entry of the map with the name of its key and leaves out the keys whose class has the rule drop.
func (sensitive_data_masker SensitiveDataMasker) mask_map(source_map map[string]any) map[string]any {
	masked_map := make(map[string]any, len(source_map))
	for key, value := range source_map {
		if class_rule, has_class := sensitive_data_masker.class_rules.rule_of(key); has_class && class_rule == domain.MaskRuleDrop {
			continue
		}
		masked_map[key] = sensitive_data_masker.MaskValue(key, value)
	}
	return masked_map
}

func (sensitive_data_masker SensitiveDataMasker) mask_list(source_list []any) []any {
	masked_list := make([]any, len(source_list))
	for index, value := range source_list {
		masked_list[index] = sensitive_data_masker.MaskValue("", value)
	}
	return masked_list
}

// mask_through_json reads a value that has no dedicated case through its JSON form, which is the form the sinks
// send, and masks that form.
func (sensitive_data_masker SensitiveDataMasker) mask_through_json(value any) any {
	encoded_value, encode_error := json.Marshal(value)
	if encode_error != nil {
		return domain.MaskedText
	}

	var decoded_value any
	if decode_error := json.Unmarshal(encoded_value, &decoded_value); decode_error != nil {
		return domain.MaskedText
	}
	return sensitive_data_masker.MaskValue("", decoded_value)
}

func is_plain_scalar(value any) bool {
	switch reflect.TypeOf(value).Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
