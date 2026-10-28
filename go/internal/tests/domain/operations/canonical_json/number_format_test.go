package canonical_json_test

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

type number_case struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Input     string  `json:"input"`
	Expected  *string `json:"expected"`
	ErrorCode string  `json:"error_code"`
}

func TestNumbersOfTheSharedTable(test *testing.T) {
	for _, current := range load_cases[number_case](test, "numbers.json") {
		test.Run(current.Name, func(test *testing.T) {
			var through_value any
			var through_function func() (string, error)
			switch current.Kind {
			case "bits":
				bit_pattern, parse_error := strconv.ParseUint(current.Input, 16, 64)
				if parse_error != nil {
					test.Fatalf("bad bit pattern %q: %v", current.Input, parse_error)
				}
				number := math.Float64frombits(bit_pattern)
				through_value = number
				through_function = func() (string, error) { return canonical_json.FormatFloat(number) }
			case "integer":
				through_value = json.Number(current.Input)
				if integer, parse_error := strconv.ParseInt(current.Input, 10, 64); parse_error == nil {
					through_function = func() (string, error) { return canonical_json.FormatInt(integer) }
				}
			default:
				test.Fatalf("unknown kind %q", current.Kind)
			}
			serialized, serialize_error := canonical_json.CanonicalJSON(through_value)
			check_number_result(test, current, string(serialized), serialize_error)
			if through_function != nil {
				text, format_error := through_function()
				check_number_result(test, current, text, format_error)
			}
		})
	}
}

func check_number_result(test *testing.T, current number_case, got string, got_error error) {
	test.Helper()
	if current.Expected == nil {
		if got_error == nil {
			test.Fatalf("expected the error %q, got the text %q", current.ErrorCode, got)
		}
		if code := error_code(got_error); code != current.ErrorCode {
			test.Fatalf("the error code is %q, want %q (%v)", code, current.ErrorCode, got_error)
		}
		if !errors.Is(got_error, domain.ErrNumberOutOfRange) {
			test.Errorf("the error does not wrap ErrNumberOutOfRange: %v", got_error)
		}
		return
	}
	if got_error != nil {
		test.Fatalf("unexpected error: %v", got_error)
	}
	if got != *current.Expected {
		test.Fatalf("got %q, want %q", got, *current.Expected)
	}
}

func TestIntegerBeyondTheExactRangeIsLimitExceeded(test *testing.T) {
	for _, number := range []int64{canonical_json.MaxExactInteger + 1, -canonical_json.MaxExactInteger - 1, math.MaxInt64, math.MinInt64} {
		_, format_error := canonical_json.FormatInt(number)
		if error_code(format_error) != value_objects.ErrorCodeLimitExceeded || !errors.Is(format_error, domain.ErrNumberOutOfRange) {
			test.Errorf("%d: %v", number, format_error)
		}
	}
	text, format_error := canonical_json.FormatInt(canonical_json.MaxExactInteger)
	if format_error != nil || text != "9007199254740991" {
		test.Errorf("the largest exact integer: %q %v", text, format_error)
	}
}

func TestEcmaScriptFormLaysOutTheDigits(test *testing.T) {
	cases := []struct {
		digits         string
		point_position int
		expected       string
	}{
		{"1", 1, "1"}, {"1", 21, "100000000000000000000"}, {"1", 22, "1e+21"}, {"15", 22, "1.5e+21"},
		{"15", 1, "1.5"}, {"1", 0, "0.1"}, {"1", -5, "0.000001"}, {"1", -6, "1e-7"}, {"12", -6, "1.2e-7"},
	}
	for _, current := range cases {
		if got := canonical_json.EcmaScriptForm(current.digits, current.point_position); got != current.expected {
			test.Errorf("%s at %d: got %q, want %q", current.digits, current.point_position, got, current.expected)
		}
	}
}

func TestFloat32PrintsTheShortestDigitsOfThe32BitValue(test *testing.T) {
	serialized, serialize_error := canonical_json.CanonicalJSON(float32(0.1))
	if serialize_error != nil || string(serialized) != "0.1" {
		test.Errorf("got %q %v", serialized, serialize_error)
	}
}

func TestJSONNumberTextThatIsNotANumberIsRejected(test *testing.T) {
	for _, text := range []string{"1e999", "-1e999", "1.0.0"} {
		_, serialize_error := canonical_json.CanonicalJSON(json.Number(text))
		if serialize_error == nil || error_code(serialize_error) == "" {
			test.Errorf("%q: %v", text, serialize_error)
		}
	}
	_, serialize_error := canonical_json.CanonicalJSON(json.Number("abc"))
	if serialize_error != nil && error_code(serialize_error) != value_objects.ErrorCodeLimitExceeded {
		test.Errorf("abc: %v", serialize_error)
	}
}
