package event_validation

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"unicode/utf8"
)

// Problem says where a value breaks its schema and why. It never carries the value: a value may be a secret.
type Problem struct {
	// Path is the field path, such as data.message or data.labels.region[2].
	Path string
	// Reason is a short fixed text, such as "expected string".
	Reason string
}

// Validator checks one value found at a field path and returns the first problem it finds, or nil.
type Validator func(value any, path string) *Problem

// FieldRule is a property an object declares and the check of its value.
type FieldRule struct {
	// Name is the name of the property in the JSON object.
	Name string
	// Check validates the value of the property.
	Check Validator
}

// ObjectRules are the rules of an object schema. The generated code lists Required and Fields in alphabetical order,
// so Go and Python report the same first problem.
type ObjectRules struct {
	// Required are the names of the properties that must be present.
	Required []string
	// Fields are the declared properties, each with the check of its value.
	Fields []FieldRule
	// Closed is true when a property that is not in Fields is invalid (additionalProperties false).
	Closed bool
}

// AttributesLimits are the limits of the attributes schema. A zero means no limit.
type AttributesLimits struct {
	// MaxProperties is the most keys an attributes object may have.
	MaxProperties int
	// MinKeyLength is the shortest key, in characters.
	MinKeyLength int
	// MaxKeyLength is the longest key, in characters.
	MaxKeyLength int
	// MaxStringLength is the longest string value, in characters.
	MaxStringLength int
}

// The reasons of a problem. They are fixed texts, shared by the Go and the Python validators.
const (
	reason_missing         = "required field is missing"
	reason_unknown         = "unknown field"
	reason_not_object      = "expected object"
	reason_not_string      = "expected string"
	reason_not_number      = "expected number"
	reason_not_integer     = "expected integer"
	reason_not_boolean     = "expected boolean"
	reason_not_finite      = "number is not finite"
	reason_not_allowed     = "not one of the allowed values"
	reason_not_scalar      = "expected a string, a number or a boolean"
	reason_not_scalar_list = "expected a scalar or a list of scalars"
	reason_too_many        = "too many keys"
	reason_bad_key         = "the key has an invalid length"
)

// The longest text of a name that a path or a message shows. A longer one is cut, so a hostile name cannot fill a log.
const display_limit = 64

// DisplayText returns a name safe to put in a path or a message: at most 64 characters, an ASCII control character
// replaced by "?" and "..." after a name that was cut. It is meant for names (a field, an event type), never for values.
func DisplayText(text string) string {
	shown := make([]rune, 0, display_limit)
	cut := false
	for _, character := range text {
		if len(shown) == display_limit {
			cut = true
			break
		}
		if character < 0x20 || character == 0x7f {
			character = '?'
		}
		shown = append(shown, character)
	}
	if cut {
		return string(shown) + "..."
	}
	return string(shown)
}

func join_path(path string, name string) string {
	return path + "." + DisplayText(name)
}

// ValidateObject checks that a value is an object (a map[string]any) and applies the rules: first every missing
// required field, then the declared fields that are present, in the order of the rules, and finally, when the object
// is closed, the smallest unknown field. It returns the first problem, or nil.
func ValidateObject(value any, path string, rules ObjectRules) *Problem {
	fields, is_object := value.(map[string]any)
	if !is_object {
		return &Problem{Path: path, Reason: reason_not_object}
	}
	for _, name := range rules.Required {
		if _, present := fields[name]; !present {
			return &Problem{Path: join_path(path, name), Reason: reason_missing}
		}
	}
	declared := make(map[string]bool, len(rules.Fields))
	for _, rule := range rules.Fields {
		declared[rule.Name] = true
		if field_value, present := fields[rule.Name]; present {
			if problem := rule.Check(field_value, join_path(path, rule.Name)); problem != nil {
				return problem
			}
		}
	}
	if rules.Closed {
		unknown, found := "", false
		for name := range fields {
			if !declared[name] && (!found || name < unknown) {
				unknown, found = name, true
			}
		}
		if found {
			return &Problem{Path: join_path(path, unknown), Reason: reason_unknown}
		}
	}
	return nil
}

// CheckObject accepts any object: the values are not checked. It is the check of a property of type object with no
// declared property.
func CheckObject(value any, path string) *Problem {
	if _, is_object := value.(map[string]any); !is_object {
		return &Problem{Path: path, Reason: reason_not_object}
	}
	return nil
}

// CheckString returns the check of a string whose length, in characters, is at least min_length and at most
// max_length; a zero means no limit.
func CheckString(min_length int, max_length int) Validator {
	return func(value any, path string) *Problem {
		text, is_string := value.(string)
		if !is_string {
			return &Problem{Path: path, Reason: reason_not_string}
		}
		length := utf8.RuneCountInString(text)
		if length < min_length {
			return &Problem{Path: path, Reason: "string is shorter than " + strconv.Itoa(min_length) + " characters"}
		}
		if max_length > 0 && length > max_length {
			return &Problem{Path: path, Reason: "string is longer than " + strconv.Itoa(max_length) + " characters"}
		}
		return nil
	}
}

// CheckBoolean accepts a bool and nothing else.
func CheckBoolean(value any, path string) *Problem {
	if _, is_boolean := value.(bool); !is_boolean {
		return &Problem{Path: path, Reason: reason_not_boolean}
	}
	return nil
}

// CheckNumber accepts a finite number: any Go integer, a finite float or a json.Number that is one. A bool is not a
// number. NaN and the infinities are invalid.
func CheckNumber(value any, path string) *Problem {
	switch classify_number(value) {
	case number_none:
		return &Problem{Path: path, Reason: reason_not_number}
	case number_not_finite:
		return &Problem{Path: path, Reason: reason_not_finite}
	}
	return nil
}

// CheckInteger accepts a whole number: any Go integer, a float or a json.Number with no fraction. 1.5 is invalid, a
// bool is not an integer, and NaN and the infinities are invalid.
func CheckInteger(value any, path string) *Problem {
	switch classify_number(value) {
	case number_none, number_fraction:
		return &Problem{Path: path, Reason: reason_not_integer}
	case number_not_finite:
		return &Problem{Path: path, Reason: reason_not_finite}
	}
	return nil
}

// CheckEnum returns the check of a string that is one of the allowed values. The message never names the value.
func CheckEnum(allowed ...string) Validator {
	return func(value any, path string) *Problem {
		text, is_string := value.(string)
		if !is_string {
			return &Problem{Path: path, Reason: reason_not_string}
		}
		for _, candidate := range allowed {
			if candidate == text {
				return nil
			}
		}
		return &Problem{Path: path, Reason: reason_not_allowed}
	}
}

// CheckAttributes returns the check of the attributes: an object whose values are scalars (a string, a number or a
// boolean) or lists of scalars, within the limits. The keys are checked in alphabetical order.
func CheckAttributes(limits AttributesLimits) Validator {
	return func(value any, path string) *Problem {
		attributes, is_object := value.(map[string]any)
		if !is_object {
			return &Problem{Path: path, Reason: reason_not_object}
		}
		if limits.MaxProperties > 0 && len(attributes) > limits.MaxProperties {
			return &Problem{Path: path, Reason: reason_too_many}
		}
		for _, key := range sorted_keys(attributes) {
			key_length := utf8.RuneCountInString(key)
			if key_length < limits.MinKeyLength || (limits.MaxKeyLength > 0 && key_length > limits.MaxKeyLength) {
				return &Problem{Path: join_path(path, key), Reason: reason_bad_key}
			}
			if problem := check_attribute_value(attributes[key], join_path(path, key), limits); problem != nil {
				return problem
			}
		}
		return nil
	}
}

// check_attribute_value checks the value of one attribute: a scalar or a list of scalars.
func check_attribute_value(value any, path string, limits AttributesLimits) *Problem {
	items, is_list := value.([]any)
	if !is_list {
		return check_scalar(value, path, limits, reason_not_scalar_list)
	}
	for position, item := range items {
		if problem := check_scalar(item, path+"["+strconv.Itoa(position)+"]", limits, reason_not_scalar); problem != nil {
			return problem
		}
	}
	return nil
}

// check_scalar checks a string within the limit, a boolean or a finite number; not_scalar is the reason for any other value.
func check_scalar(value any, path string, limits AttributesLimits, not_scalar string) *Problem {
	if text, is_string := value.(string); is_string {
		if limits.MaxStringLength > 0 && utf8.RuneCountInString(text) > limits.MaxStringLength {
			return &Problem{Path: path, Reason: "string is longer than " + strconv.Itoa(limits.MaxStringLength) + " characters"}
		}
		return nil
	}
	if _, is_boolean := value.(bool); is_boolean {
		return nil
	}
	switch classify_number(value) {
	case number_none:
		return &Problem{Path: path, Reason: not_scalar}
	case number_not_finite:
		return &Problem{Path: path, Reason: reason_not_finite}
	}
	return nil
}

// number_class is what classify_number says about a value.
type number_class int

const (
	// number_none is a value that is not a number (a bool, a string, nil...).
	number_none number_class = iota
	// number_not_finite is NaN or an infinity.
	number_not_finite
	// number_fraction is a finite number that is not a whole number.
	number_fraction
	// number_whole is a whole number.
	number_whole
)

// classify_number tells whether a value is a number and which kind. It never panics: a type it does not know is none.
func classify_number(value any) number_class {
	switch number := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
		return number_whole
	case float32:
		return classify_float(float64(number))
	case float64:
		return classify_float(number)
	case json.Number:
		if _, whole_error := number.Int64(); whole_error == nil {
			return number_whole
		}
		parsed, parse_error := strconv.ParseFloat(string(number), 64)
		if parse_error != nil {
			return number_none
		}
		return classify_float(parsed)
	}
	return number_none
}

func classify_float(number float64) number_class {
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return number_not_finite
	}
	if number != math.Trunc(number) {
		return number_fraction
	}
	return number_whole
}

// sorted_keys returns the keys of a map in ascending order, the order in which both languages check them.
func sorted_keys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
