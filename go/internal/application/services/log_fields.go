package services

import (
	"math"
	"reflect"
	"strconv"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// LOG FIELDS TO ATTRIBUTES
// =============================================================================

const (
	// max_exact_integer is the largest integer that every language keeps exactly as a number: 2^53 - 1.
	max_exact_integer = 1<<53 - 1
	// unsupported_value_text is the attribute of a value that has no canonical JSON form.
	unsupported_value_text = "[unsupported value]"
)

// LogFieldsToAttributes turns the fields of a log call into the attributes of the event. The rule is the same in Go and
// in Python (shared fixtures), key by key:
//
//   - An empty key is dropped. A nil value is dropped.
//   - A string is kept as it is. A bool is kept.
//   - Any Go integer or float kind (named kinds included) is a number and is kept as a number, EXCEPT an integer outside
//     -(2^53-1)..(2^53-1), which becomes its decimal text, and a NaN or an infinity, which becomes the text "NaN",
//     "+Inf" or "-Inf".
//   - A value of type error becomes its Error() text (and "[unsupported value]" when that call panics).
//   - A slice or an array is kept as a list ONLY when every element is a scalar by these same rules (a string, a bool or
//     a finite number in range).
//   - Anything else (a list with a non-scalar element, a map, a struct, a pointer, a time, a channel...) becomes ONE
//     string: the canonical JSON of the whole value (canonical_json.CanonicalJSON), or exactly "[unsupported value]"
//     when that fails.
//
// The result goes through value_objects.NewEventAttributes. It returns nil when no field is left, and also when that
// constructor still refuses the result: the attributes of the call are dropped, never the event. The limits of
// T-03.5 are applied later, by the pipeline.
func LogFieldsToAttributes(fields map[string]any) *value_objects.EventAttributes {
	converted := make(map[string]any, len(fields))
	for key, value := range fields {
		if key == "" || value == nil {
			continue
		}
		converted[key] = log_field_value(value)
	}
	if len(converted) == 0 {
		return nil
	}
	attributes, attributes_error := value_objects.NewEventAttributes(converted)
	if attributes_error != nil {
		return nil
	}
	return &attributes
}

// ----- HELPERS --------------------------------------------------------------

// log_field_value converts one non-nil value by the rule of LogFieldsToAttributes.
func log_field_value(value any) any {
	if value_error, is_error := value.(error); is_error {
		return error_text(value_error)
	}
	if scalar, is_scalar := strict_scalar(value); is_scalar {
		return scalar
	}
	if text, is_number := number_text(value); is_number {
		return text
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Slice || reflected.Kind() == reflect.Array {
		items := make([]any, 0, reflected.Len())
		for position := 0; position < reflected.Len(); position++ {
			item, is_scalar := strict_scalar(reflected.Index(position).Interface())
			if !is_scalar {
				return canonical_text(value)
			}
			items = append(items, item)
		}
		return items
	}
	return canonical_text(value)
}

// strict_scalar returns the value as a string, a bool or a number when it is one of them and the number is finite and
// in range (an integer inside -(2^53-1)..(2^53-1)). Otherwise the second result is false.
func strict_scalar(value any) (any, bool) {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.String:
		return reflected.String(), true
	case reflect.Bool:
		return reflected.Bool(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		number := reflected.Int()
		return number, number >= -max_exact_integer && number <= max_exact_integer
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		number := reflected.Uint()
		return int64(number), number <= max_exact_integer
	case reflect.Float32:
		number := shortest_float32(reflected.Float())
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	case reflect.Float64:
		number := reflected.Float()
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	}
	return nil, false
}

// number_text is the text of a number that strict_scalar refused: the decimal text of an integer out of range, or "NaN",
// "+Inf" or "-Inf". The second result is false when the value is not such a number.
func number_text(value any) (string, bool) {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(reflected.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(reflected.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		number := reflected.Float()
		switch {
		case math.IsNaN(number):
			return "NaN", true
		case math.IsInf(number, 1):
			return "+Inf", true
		default:
			return "-Inf", true
		}
	}
	return "", false
}

// shortest_float32 returns the float64 that has the shortest text of the float32, so a float32 0.1 stays 0.1.
func shortest_float32(number float64) float64 {
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return number
	}
	shortened, parse_error := strconv.ParseFloat(strconv.FormatFloat(number, 'g', -1, 32), 64)
	if parse_error != nil {
		return number
	}
	return shortened
}

// canonical_text is the canonical JSON of the value as text, or unsupported_value_text when it has none. A panic is
// the same as a failure.
func canonical_text(value any) (text string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			text = unsupported_value_text
		}
	}()
	canonical_bytes, canonical_error := canonical_json.CanonicalJSON(value)
	if canonical_error != nil {
		return unsupported_value_text
	}
	return string(canonical_bytes)
}

// error_text is the Error() text of the error, or unsupported_value_text when the call panics (a nil pointer that is
// an error, for example).
func error_text(value_error error) (text string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			text = unsupported_value_text
		}
	}()
	return value_error.Error()
}
