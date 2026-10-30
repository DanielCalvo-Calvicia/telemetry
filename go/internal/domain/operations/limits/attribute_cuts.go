package limits

import (
	"maps"
	"reflect"
	"slices"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// ===== Attribute cuts =======================================================

// cut_attributes returns a limited copy of the attributes and the counts of the cuts. Nothing in the input is changed.
// It returns nil when no key remains.
func cut_attributes(attributes value_objects.Attributes, limits value_objects.Limits) (value_objects.Attributes, Report) {
	var report Report
	if len(attributes) == 0 {
		return nil, report
	}
	sorted_keys := slices.SortedFunc(maps.Keys(attributes), func(left string, right string) int {
		switch {
		case canonical_json.KeyLess(left, right):
			return -1
		case canonical_json.KeyLess(right, left):
			return 1
		}
		return 0
	})

	limited := make(value_objects.Attributes, min(len(sorted_keys), limits.MaxAttributeKeys()))
	for position, key := range sorted_keys {
		if position >= limits.MaxAttributeKeys() {
			report.AttributeKeysDropped++
			continue
		}
		kept_key, is_cut := cut_text(key, limits.MaxAttributeKeyChars())
		if _, taken := limited[kept_key]; taken {
			report.AttributeKeysDropped++
			continue
		}
		if is_cut {
			report.AttributeKeysTruncated++
		}
		limited[kept_key] = cut_value(attributes[key], limits.MaxAttributeValueChars(), &report)
	}
	if len(limited) == 0 {
		return nil, report
	}
	return limited, report
}

// cut_value returns the value with its texts cut: a text is cut, a list is copied into a new []any with each text cut
// on its own, and any other value (number, boolean, null) is returned as it is.
func cut_value(value any, max_chars int, report *Report) any {
	if text, is_text := value.(string); is_text {
		cut, is_cut := cut_text(text, max_chars)
		if is_cut {
			report.AttributeValuesTruncated++
		}
		return cut
	}
	if value == nil || reflect.TypeOf(value).Kind() != reflect.Slice {
		return value
	}
	list := reflect.ValueOf(value)
	copied := make([]any, list.Len())
	for position := range copied {
		element := list.Index(position).Interface()
		if text, is_text := element.(string); is_text {
			cut, is_cut := cut_text(text, max_chars)
			if is_cut {
				report.AttributeValuesTruncated++
			}
			element = cut
		}
		copied[position] = element
	}
	return copied
}

// cut_text returns the text cut to max_chars characters (Unicode code points, never bytes) and whether it was cut.
func cut_text(text string, max_chars int) (string, bool) {
	characters := 0
	for byte_offset := range text {
		if characters == max_chars {
			return text[:byte_offset], true
		}
		characters++
	}
	return text, false
}
