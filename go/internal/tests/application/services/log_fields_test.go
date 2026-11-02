package services_test

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
)

type named_string string
type named_int int16
type named_float float32

type payload_struct struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type nil_error_pointer struct{}

// Error panics on purpose: it reads a field of a nil pointer.
func (pointer *nil_error_pointer) Error() string {
	var inner *payload_struct
	return inner.Name
}

func Test_log_fields_follow_the_rule_of_the_attributes(t *testing.T) {
	const big = int64(1) << 53
	cases := []struct {
		name   string
		fields map[string]any
		want   map[string]any
	}{
		{"nil map is no attributes", nil, nil},
		{"empty map is no attributes", map[string]any{}, nil},
		{"empty key is dropped", map[string]any{"": "x", "a": "b"}, map[string]any{"a": "b"}},
		{"only an empty key leaves nothing", map[string]any{"": "x"}, nil},
		{"nil value is dropped", map[string]any{"a": nil, "b": "c"}, map[string]any{"b": "c"}},
		{"a string is kept", map[string]any{"a": "text"}, map[string]any{"a": "text"}},
		{"an empty string is kept", map[string]any{"a": ""}, map[string]any{"a": ""}},
		{"a named string is a string", map[string]any{"a": named_string("x")}, map[string]any{"a": "x"}},
		{"a bool is kept", map[string]any{"a": true, "b": false}, map[string]any{"a": true, "b": false}},
		{"an int is a number", map[string]any{"a": 7}, map[string]any{"a": int64(7)}},
		{"a named int is a number", map[string]any{"a": named_int(-3)}, map[string]any{"a": int64(-3)}},
		{"a uint8 is a number", map[string]any{"a": uint8(200)}, map[string]any{"a": int64(200)}},
		{"a float64 is a number", map[string]any{"a": 0.5}, map[string]any{"a": 0.5}},
		{"an integral float64 stays a number", map[string]any{"a": float64(7)}, map[string]any{"a": float64(7)}},
		{"a float32 keeps its short text", map[string]any{"a": float32(0.1)}, map[string]any{"a": 0.1}},
		{"a named float is a number", map[string]any{"a": named_float(0.25)}, map[string]any{"a": 0.25}},
		{"the largest exact integer is a number", map[string]any{"a": int64(1<<53 - 1), "b": -int64(1<<53 - 1)}, map[string]any{"a": int64(1<<53 - 1), "b": -int64(1<<53 - 1)}},
		{"an integer beyond the range is its text", map[string]any{"a": big, "b": -big, "c": int64(math.MaxInt64)}, map[string]any{"a": "9007199254740992", "b": "-9007199254740992", "c": "9223372036854775807"}},
		{"a uint beyond the range is its text", map[string]any{"a": uint64(math.MaxUint64)}, map[string]any{"a": "18446744073709551615"}},
		{"NaN is its text", map[string]any{"a": math.NaN(), "b": float32(math.NaN())}, map[string]any{"a": "NaN", "b": "NaN"}},
		{"an infinity is its text", map[string]any{"a": math.Inf(1), "b": math.Inf(-1)}, map[string]any{"a": "+Inf", "b": "-Inf"}},
		{"an error is its text", map[string]any{"a": errors.New("boom")}, map[string]any{"a": "boom"}},
		{"an error that panics is the unsupported text", map[string]any{"a": (*nil_error_pointer)(nil)}, map[string]any{"a": "[unsupported value]"}},
		{"a list of strings is kept", map[string]any{"a": []string{"x", "y"}}, map[string]any{"a": []any{"x", "y"}}},
		{"a list of numbers is kept", map[string]any{"a": []int{1, 2}, "b": []float64{0.5, 2}}, map[string]any{"a": []any{int64(1), int64(2)}, "b": []any{0.5, float64(2)}}},
		{"a list of mixed scalars is kept", map[string]any{"a": []any{"x", true, 1.5, 3}}, map[string]any{"a": []any{"x", true, 1.5, int64(3)}}},
		{"an array is a list", map[string]any{"a": [2]string{"x", "y"}}, map[string]any{"a": []any{"x", "y"}}},
		{"an empty list is kept", map[string]any{"a": []string{}, "b": []string(nil)}, map[string]any{"a": []any{}, "b": []any{}}},
		{"a list with a map is one text", map[string]any{"a": []any{map[string]any{"a": 1}, []any{2}}}, map[string]any{"a": `[{"a":1},[2]]`}},
		{"a list with a nil is one text", map[string]any{"a": []any{"x", nil}}, map[string]any{"a": `["x",null]`}},
		{"a list with a big integer is the unsupported text", map[string]any{"a": []int64{1, big}}, map[string]any{"a": "[unsupported value]"}},
		{"a list with a NaN is the unsupported text", map[string]any{"a": []float64{1, math.NaN()}}, map[string]any{"a": "[unsupported value]"}},
		{"a nested map is its canonical JSON", map[string]any{"a": map[string]any{"b": []any{1.0, 2.0}, "a": 1.0}}, map[string]any{"a": `{"a":1,"b":[1,2]}`}},
		{"a struct is its canonical JSON", map[string]any{"a": payload_struct{Name: "bob", ID: 1}}, map[string]any{"a": `{"id":1,"name":"bob"}`}},
		{"a pointer to a struct is its canonical JSON", map[string]any{"a": &payload_struct{Name: "bob", ID: 1}}, map[string]any{"a": `{"id":1,"name":"bob"}`}},
		{"a time in UTC is its canonical JSON", map[string]any{"a": time.Date(2026, 10, 28, 10, 0, 0, 123456000, time.UTC)}, map[string]any{"a": `"2026-10-28T10:00:00.123456Z"`}},
		{"a time that is not UTC is the unsupported text", map[string]any{"a": time.Date(2026, 10, 28, 10, 0, 0, 0, time.FixedZone("x", 3600))}, map[string]any{"a": "[unsupported value]"}},
		{"a channel is the unsupported text", map[string]any{"a": make(chan int)}, map[string]any{"a": "[unsupported value]"}},
		{"a function is the unsupported text", map[string]any{"a": func() {}}, map[string]any{"a": "[unsupported value]"}},
		{"a map with a big integer is the unsupported text", map[string]any{"a": map[string]any{"a": big}}, map[string]any{"a": "[unsupported value]"}},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			attributes := services.LogFieldsToAttributes(test_case.fields)
			if test_case.want == nil {
				if attributes != nil {
					t.Fatalf("got %v, want no attributes", attributes.Entries())
				}
				return
			}
			if attributes == nil {
				t.Fatalf("got no attributes, want %v", test_case.want)
			}
			if got := attributes.Entries(); !reflect.DeepEqual(got, test_case.want) {
				t.Fatalf("got %#v, want %#v", got, test_case.want)
			}
		})
	}
}
