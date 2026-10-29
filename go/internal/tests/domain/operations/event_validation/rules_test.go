package event_validation_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
)

// demo_validator builds by hand the rules that tools/contract_generator generates for the demo event type of its
// fixture contract (see event_validators_generated.py.golden): an enum, a nested object, attributes, an integer, a
// number, a boolean, string limits and an open object. The shared tables of fixtures/event_validation/constructs run
// on it, and in Python they run on the generated validator itself.
func demo_validator() event_validation.Validator {
	resource := func(value any, path string) *event_validation.Problem {
		return event_validation.ValidateObject(value, path, event_validation.ObjectRules{
			Required: []string{"service_name"},
			Closed:   true,
			Fields: []event_validation.FieldRule{
				{Name: "host", Check: event_validation.CheckString(0, 0)},
				{Name: "service_name", Check: event_validation.CheckString(1, 0)},
			},
		})
	}
	return func(value any, path string) *event_validation.Problem {
		return event_validation.ValidateObject(value, path, event_validation.ObjectRules{
			Required: []string{"category", "count", "severity"},
			Closed:   true,
			Fields: []event_validation.FieldRule{
				{Name: "category", Check: event_validation.CheckString(0, 0)},
				{Name: "count", Check: event_validation.CheckInteger},
				{Name: "flag", Check: event_validation.CheckBoolean},
				{Name: "labels", Check: event_validation.CheckAttributes(event_validation.AttributesLimits{
					MaxProperties: 3, MinKeyLength: 1, MaxKeyLength: 8, MaxStringLength: 10,
				})},
				{Name: "note", Check: event_validation.CheckString(1, 20)},
				{Name: "payload", Check: event_validation.CheckObject},
				{Name: "ratio", Check: event_validation.CheckNumber},
				{Name: "resource", Check: resource},
				{Name: "severity", Check: event_validation.CheckEnum("INFO", "ERROR")},
			},
		})
	}
}

// construct_case is a case of constructs/valid.json and constructs/invalid.json.
type construct_case struct {
	Name      string `json:"name"`
	Data      any    `json:"data"`
	FieldPath string `json:"field_path"`
	Reason    string `json:"reason"`
	Secret    string `json:"secret"`
}

func TestEveryValidConstructFixtureIsAccepted(test *testing.T) {
	validator := demo_validator()
	for _, each := range load_cases[construct_case](test, "constructs/valid.json") {
		test.Run(each.Name, func(test *testing.T) {
			if problem := validator(each.Data, "data"); problem != nil {
				test.Errorf("rejected at %s: %s", problem.Path, problem.Reason)
			}
		})
	}
}

func TestEveryInvalidConstructFixtureGivesTheExpectedFieldPathAndReason(test *testing.T) {
	validator := demo_validator()
	for _, each := range load_cases[construct_case](test, "constructs/invalid.json") {
		test.Run(each.Name, func(test *testing.T) {
			problem := validator(each.Data, "data")
			if problem == nil {
				test.Fatal("an invalid value was accepted")
			}
			if problem.Path != each.FieldPath || problem.Reason != each.Reason {
				test.Errorf("got %s: %s, want %s: %s", problem.Path, problem.Reason, each.FieldPath, each.Reason)
			}
			if each.Secret != "" && strings.Contains(problem.Path+problem.Reason, each.Secret) {
				test.Error("the problem contains the offending value")
			}
		})
	}
}

func problem_reason(problem *event_validation.Problem) string {
	if problem == nil {
		return "valid"
	}
	return problem.Reason
}

func TestNumbersAndIntegers(test *testing.T) {
	var float32_value float32 = 2
	cases := []struct {
		name    string
		value   any
		number  string
		integer string
	}{
		{"an int", 3, "valid", "valid"},
		{"an int8", int8(-3), "valid", "valid"},
		{"an int16", int16(3), "valid", "valid"},
		{"an int32", int32(3), "valid", "valid"},
		{"an int64", int64(math.MinInt64), "valid", "valid"},
		{"a uint", uint(3), "valid", "valid"},
		{"a uint8", uint8(3), "valid", "valid"},
		{"a uint16", uint16(3), "valid", "valid"},
		{"a uint32", uint32(3), "valid", "valid"},
		{"a uint64 beyond the int64 range", uint64(math.MaxUint64), "valid", "valid"},
		{"a uintptr", uintptr(3), "valid", "valid"},
		{"a float32 whole number", float32_value, "valid", "valid"},
		{"a float32 with a fraction", float32(2.5), "valid", "expected integer"},
		{"a float64 whole number", 2.0, "valid", "valid"},
		{"a float64 with a fraction", 1.5, "valid", "expected integer"},
		{"a float64 that is huge", 1e300, "valid", "valid"},
		{"zero", 0.0, "valid", "valid"},
		{"NaN", math.NaN(), "number is not finite", "number is not finite"},
		{"positive infinity", math.Inf(1), "number is not finite", "number is not finite"},
		{"negative infinity", math.Inf(-1), "number is not finite", "number is not finite"},
		{"a float32 NaN", float32(math.NaN()), "number is not finite", "number is not finite"},
		{"a json.Number integer", json.Number("12"), "valid", "valid"},
		{"a json.Number integer beyond int64", json.Number("123456789012345678901234567890"), "valid", "valid"},
		{"a json.Number with a zero fraction", json.Number("1.0"), "valid", "valid"},
		{"a json.Number with an exponent", json.Number("1e2"), "valid", "valid"},
		{"a json.Number with a fraction", json.Number("1.5"), "valid", "expected integer"},
		{"a json.Number that is not a number", json.Number("abc"), "expected number", "expected integer"},
		{"a json.Number out of the float range", json.Number("1e999"), "expected number", "expected integer"},
		{"true", true, "expected number", "expected integer"},
		{"false", false, "expected number", "expected integer"},
		{"a string", "1", "expected number", "expected integer"},
		{"nil", nil, "expected number", "expected integer"},
		{"a list", []any{1}, "expected number", "expected integer"},
		{"a map", map[string]any{}, "expected number", "expected integer"},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			if got := problem_reason(event_validation.CheckNumber(each.value, "data.x")); got != each.number {
				test.Errorf("CheckNumber: %q, want %q", got, each.number)
			}
			if got := problem_reason(event_validation.CheckInteger(each.value, "data.x")); got != each.integer {
				test.Errorf("CheckInteger: %q, want %q", got, each.integer)
			}
		})
	}
}

func TestStringsBooleansEnumsAndObjects(test *testing.T) {
	bounded := event_validation.CheckString(2, 3)
	unbounded := event_validation.CheckString(0, 0)
	only_min := event_validation.CheckString(2, 0)
	enum := event_validation.CheckEnum("a", "b")
	empty_enum := event_validation.CheckEnum()
	checks := []struct {
		name   string
		check  event_validation.Validator
		value  any
		reason string
	}{
		{"a string inside the limits", bounded, "ab", "valid"},
		{"a string at the maximum", bounded, "abc", "valid"},
		{"a string too short", bounded, "a", "string is shorter than 2 characters"},
		{"a string too long", bounded, "abcd", "string is longer than 3 characters"},
		{"characters are not bytes (too long)", bounded, "éééé", "string is longer than 3 characters"},
		{"characters are not bytes (inside)", bounded, "éé", "valid"},
		{"an emoji counts as one character", bounded, "\U0001F600\U0001F600", "valid"},
		{"no limits", unbounded, strings.Repeat("x", 100000), "valid"},
		{"an empty string without limits", unbounded, "", "valid"},
		{"only a minimum", only_min, strings.Repeat("x", 1000), "valid"},
		{"a number is not a string", bounded, 12, "expected string"},
		{"nil is not a string", unbounded, nil, "expected string"},
		{"bytes are not a string", unbounded, []byte("a"), "expected string"},
		{"a boolean", event_validation.CheckBoolean, true, "valid"},
		{"false is a boolean", event_validation.CheckBoolean, false, "valid"},
		{"1 is not a boolean", event_validation.CheckBoolean, 1, "expected boolean"},
		{"a string is not a boolean", event_validation.CheckBoolean, "true", "expected boolean"},
		{"an enum value", enum, "b", "valid"},
		{"a value that is not in the enum", enum, "c", "not one of the allowed values"},
		{"the case of an enum value matters", enum, "A", "not one of the allowed values"},
		{"an empty enum allows nothing", empty_enum, "a", "not one of the allowed values"},
		{"a number is not an enum value", enum, 1, "expected string"},
		{"an object", event_validation.CheckObject, map[string]any{"a": 1}, "valid"},
		{"an empty object", event_validation.CheckObject, map[string]any{}, "valid"},
		{"a typed nil map is an object", event_validation.CheckObject, map[string]any(nil), "valid"},
		{"a list is not an object", event_validation.CheckObject, []any{}, "expected object"},
		{"nil is not an object", event_validation.CheckObject, nil, "expected object"},
		{"a map with other keys is not an object", event_validation.CheckObject, map[int]any{}, "expected object"},
	}
	for _, each := range checks {
		test.Run(each.name, func(test *testing.T) {
			if got := problem_reason(each.check(each.value, "data.x")); got != each.reason {
				test.Errorf("got %q, want %q", got, each.reason)
			}
		})
	}
}

func TestAttributesAndTheirLimits(test *testing.T) {
	limited := event_validation.CheckAttributes(event_validation.AttributesLimits{MaxProperties: 2, MinKeyLength: 1, MaxKeyLength: 3, MaxStringLength: 4})
	unlimited := event_validation.CheckAttributes(event_validation.AttributesLimits{})
	cases := []struct {
		name   string
		check  event_validation.Validator
		value  any
		path   string
		reason string
	}{
		{"scalars and lists", limited, map[string]any{"a": "abcd", "b": []any{1, true, 2.5}}, "", "valid"},
		{"an empty list", limited, map[string]any{"a": []any{}}, "", "valid"},
		{"too many keys", limited, map[string]any{"a": 1, "b": 2, "c": 3}, "data.x", "too many keys"},
		{"a key that is too long", limited, map[string]any{"abcd": 1}, "data.x.abcd", "the key has an invalid length"},
		{"an empty key", limited, map[string]any{"": 1}, "data.x.", "the key has an invalid length"},
		{"a string that is too long", limited, map[string]any{"a": "abcde"}, "data.x.a", "string is longer than 4 characters"},
		{"a string in a list that is too long", limited, map[string]any{"a": []any{"ab", "abcde"}}, "data.x.a[1]", "string is longer than 4 characters"},
		{"NaN", limited, map[string]any{"a": math.NaN()}, "data.x.a", "number is not finite"},
		{"NaN in a list", limited, map[string]any{"a": []any{math.Inf(1)}}, "data.x.a[0]", "number is not finite"},
		{"nil", limited, map[string]any{"a": nil}, "data.x.a", "expected a scalar or a list of scalars"},
		{"a nested object", limited, map[string]any{"a": map[string]any{}}, "data.x.a", "expected a scalar or a list of scalars"},
		{"a nested list", limited, map[string]any{"a": []any{[]any{1}}}, "data.x.a[0]", "expected a string, a number or a boolean"},
		{"a typed slice is not a list", limited, map[string]any{"a": []string{"x"}}, "data.x.a", "expected a scalar or a list of scalars"},
		{"the first key in order is reported", limited, map[string]any{"b": nil, "a": nil}, "data.x.a", "expected a scalar or a list of scalars"},
		{"not an object", limited, []any{}, "data.x", "expected object"},
		{"nil is not an object", limited, nil, "data.x", "expected object"},
		{"no limits at all", unlimited, map[string]any{"": strings.Repeat("x", 100000), "k": 1, "l": 2, "m": 3}, "", "valid"},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			problem := each.check(each.value, "data.x")
			if got := problem_reason(problem); got != each.reason {
				test.Fatalf("got %q, want %q", got, each.reason)
			}
			if problem != nil && problem.Path != each.path {
				test.Errorf("path %q, want %q", problem.Path, each.path)
			}
		})
	}
}

func TestAnObjectIsCheckedInTheDocumentedOrder(test *testing.T) {
	rules := event_validation.ObjectRules{
		Required: []string{"a", "b"},
		Closed:   true,
		Fields: []event_validation.FieldRule{
			{Name: "a", Check: event_validation.CheckString(0, 0)},
			{Name: "b", Check: event_validation.CheckString(0, 0)},
			{Name: "c", Check: event_validation.CheckString(0, 0)},
		},
	}
	open_rules := event_validation.ObjectRules{Fields: rules.Fields}
	cases := []struct {
		name   string
		rules  event_validation.ObjectRules
		value  any
		path   string
		reason string
	}{
		{"a valid object", rules, map[string]any{"a": "1", "b": "2"}, "", "valid"},
		{"a required field is missing", rules, map[string]any{"a": "1", "z": 1}, "data.b", "required field is missing"},
		{"a missing field comes before a wrong type", rules, map[string]any{"a": 1}, "data.b", "required field is missing"},
		{"an optional field of the wrong type", rules, map[string]any{"a": "1", "b": "2", "c": 3}, "data.c", "expected string"},
		{"a wrong type comes before an unknown field", rules, map[string]any{"a": 1, "b": "2", "z": 1}, "data.a", "expected string"},
		{"an unknown field", rules, map[string]any{"a": "1", "b": "2", "z": 1}, "data.z", "unknown field"},
		{"the smallest unknown field is reported", rules, map[string]any{"a": "1", "b": "2", "z": 1, "m": 1}, "data.m", "unknown field"},
		{"an open object allows an unknown field", open_rules, map[string]any{"z": 1}, "", "valid"},
		{"an empty object with no rules", event_validation.ObjectRules{}, map[string]any{"z": 1}, "", "valid"},
		{"not an object", rules, "text", "data", "expected object"},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			problem := event_validation.ValidateObject(each.value, "data", each.rules)
			if got := problem_reason(problem); got != each.reason {
				test.Fatalf("got %q, want %q", got, each.reason)
			}
			if problem != nil && problem.Path != each.path {
				test.Errorf("path %q, want %q", problem.Path, each.path)
			}
		})
	}
}

func TestDisplayTextCutsAndCleansANameButNeverAValue(test *testing.T) {
	cases := []struct{ name, text, expected string }{
		{"a plain name", "category", "category"},
		{"an empty name", "", ""},
		{"a control character", "a\nb\x00c\x7fd\te", "a?b?c?d?e"},
		{"exactly the limit", strings.Repeat("a", 64), strings.Repeat("a", 64)},
		{"one more than the limit", strings.Repeat("a", 65), strings.Repeat("a", 64) + "..."},
		{"characters, not bytes", strings.Repeat("é", 64), strings.Repeat("é", 64)},
		{"many characters", strings.Repeat("é", 100), strings.Repeat("é", 64) + "..."},
		{"text that is not UTF-8", "a\xffb", "a�b"},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			if got := event_validation.DisplayText(each.text); got != each.expected {
				test.Errorf("got %q, want %q", got, each.expected)
			}
		})
	}
}
