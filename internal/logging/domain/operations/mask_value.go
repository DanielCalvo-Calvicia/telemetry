package operations

import (
	"encoding/json"
	"reflect"
	"time"

	domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
	value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

// MaskValue returns a masked copy of a single value that belongs to the field called field_name (an empty name
// for a value without one, such as a list element). Rules, in order:
//   - a nil value stays nil;
//   - a field with a sensitive name (IsSensitiveFieldName) is replaced by domain.MaskedText whatever it holds,
//     including maps and lists;
//   - a text is masked by MaskText, an error is turned into its masked text and bytes are always hidden;
//   - numbers, booleans and times are kept;
//   - maps and lists are masked element by element, with the name of each map key;
//   - any other value (a struct, a pointer, a typed map) is read through its JSON form and masked like the
//     rest; a value that cannot be encoded is hidden, because it cannot be inspected.
func (sensitive_data_masker SensitiveDataMasker) MaskValue(field_name string, value any) any {
	if value == nil {
		return nil
	}
	if IsSensitiveFieldName(field_name) {
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
		return sensitive_data_masker.maskMap(typed_value)
	case value_objects.Fields:
		return sensitive_data_masker.maskMap(typed_value)
	case []any:
		return sensitive_data_masker.maskList(typed_value)
	case []string:
		masked_texts := make([]string, len(typed_value))
		for index, text := range typed_value {
			masked_texts[index] = sensitive_data_masker.MaskText(text)
		}
		return masked_texts
	}

	if isPlainScalar(value) {
		return value
	}
	return sensitive_data_masker.maskThroughJSON(value)
}

func (sensitive_data_masker SensitiveDataMasker) maskMap(source_map map[string]any) map[string]any {
	masked_map := make(map[string]any, len(source_map))
	for key, value := range source_map {
		masked_map[key] = sensitive_data_masker.MaskValue(key, value)
	}
	return masked_map
}

func (sensitive_data_masker SensitiveDataMasker) maskList(source_list []any) []any {
	masked_list := make([]any, len(source_list))
	for index, value := range source_list {
		masked_list[index] = sensitive_data_masker.MaskValue("", value)
	}
	return masked_list
}

// maskThroughJSON reads a value that has no dedicated case through its JSON form, which is the form the sinks
// send, and masks that form.
func (sensitive_data_masker SensitiveDataMasker) maskThroughJSON(value any) any {
	encoded_value, err := json.Marshal(value)
	if err != nil {
		return domain.MaskedText
	}

	var decoded_value any
	if err := json.Unmarshal(encoded_value, &decoded_value); err != nil {
		return domain.MaskedText
	}
	return sensitive_data_masker.MaskValue("", decoded_value)
}

func isPlainScalar(value any) bool {
	switch reflect.TypeOf(value).Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
