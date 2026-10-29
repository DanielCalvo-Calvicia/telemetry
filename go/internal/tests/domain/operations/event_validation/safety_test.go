package event_validation_test

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/event_validation"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// A structure that holds itself, and a very deep one: nothing may loop or overflow the stack on them.
func cyclic_map() map[string]any {
	cyclic := map[string]any{"category": "http"}
	cyclic["message"] = cyclic
	cyclic["self"] = cyclic
	return cyclic
}

func deep_value(depth int) any {
	var value any = "leaf"
	for level := 0; level < depth; level++ {
		value = map[string]any{"next": value}
	}
	return value
}

func deep_list(depth int) any {
	var value any = "leaf"
	for level := 0; level < depth; level++ {
		value = []any{value}
	}
	return value
}

type weird_struct struct{ Message string }

func weird_inputs() map[string]any {
	var typed_nil_map map[string]any
	var typed_nil_pointer *weird_struct
	return map[string]any{
		"nil":                       nil,
		"a string":                  "FAKE-SECRET-VALUE-1234",
		"an int":                    42,
		"a float":                   3.14,
		"NaN":                       math.NaN(),
		"an infinity":               math.Inf(1),
		"a bool":                    true,
		"an empty list":             []any{},
		"a list of maps":            []any{map[string]any{"category": "c", "message": "m"}},
		"a list of strings":         []string{"a"},
		"a map with integer keys":   map[int]any{1: "a"},
		"a map with any keys":       map[any]any{"category": "c", "message": "m"},
		"a map of strings":          map[string]string{"category": "c", "message": "m"},
		"a typed nil map":           typed_nil_map,
		"a typed nil pointer":       typed_nil_pointer,
		"a pointer to a struct":     &weird_struct{Message: "m"},
		"a struct":                  weird_struct{Message: "m"},
		"a pointer to a map":        &map[string]any{"category": "c", "message": "m"},
		"a channel":                 make(chan int),
		"a function":                func() {},
		"bytes":                     []byte("FAKE-SECRET-VALUE-1234"),
		"a json.Number":             json.Number("12"),
		"a big integer":             big.NewInt(math.MaxInt64),
		"an error":                  errors.New("FAKE-SECRET-VALUE-1234"),
		"a cyclic map":              cyclic_map(),
		"a map with a deep message": map[string]any{"category": "c", "message": deep_value(100000)},
		"a map with a deep list":    map[string]any{"category": "c", "message": deep_list(100000)},
		"a map with a huge integer": map[string]any{"category": "c", "message": uint64(math.MaxUint64)},
		"a map with the min int":    map[string]any{"category": math.MinInt64, "message": "m"},
		"a map with NaN":            map[string]any{"category": "c", "message": math.NaN()},
		"a map with an infinity":    map[string]any{"category": math.Inf(-1), "message": "m"},
		"a map with a nil value":    map[string]any{"category": nil, "message": nil},
		"a map with a nil key name": map[string]any{"": nil, "category": "c", "message": "m"},
		"a map with a very long key": map[string]any{
			"category": "c", "message": "m", strings.Repeat("k", 1_000_000): 1,
		},
		"a map with a key that is not UTF-8": map[string]any{"category": "c", "message": "m", "\xff\xfe": 1},
	}
}

func TestNoInputPanicsAndEveryRejectionIsClassified(test *testing.T) {
	event_types := []string{value_objects.EventTypeLogEntryRecorded, "", "no.such.type", "\xff", strings.Repeat("t", 100000)}
	for name, input := range weird_inputs() {
		for _, event_type := range event_types {
			test.Run(name, func(test *testing.T) {
				defer func() {
					if recovered := recover(); recovered != nil {
						test.Fatalf("ValidateEvent(%.20q, %s) panicked: %v", event_type, name, recovered)
					}
				}()
				validation_error := event_validation.ValidateEvent(event_type, input)
				code := classified_code(validation_error)
				if validation_error == nil {
					return
				}
				if code != value_objects.ErrorCodeEventSchemaInvalid && code != value_objects.ErrorCodeEventTypeUnknown {
					test.Errorf("the error is not classified: %v", validation_error)
				}
				if len(validation_error.Error()) > 1000 {
					test.Errorf("the message is %d bytes long: a long name must be cut", len(validation_error.Error()))
				}
				if strings.Contains(validation_error.Error(), "FAKE-SECRET-VALUE-1234") {
					test.Errorf("the message contains the offending value: %q", validation_error)
				}
			})
		}
	}
}

func TestAValidEventWithExtraTypesOfNumbersIsAccepted(test *testing.T) {
	for _, message := range []string{"", "m"} {
		data := map[string]any{"category": "c", "message": message, "function": "f"}
		if validation_error := event_validation.ValidateEvent(value_objects.EventTypeLogEntryRecorded, data); validation_error != nil {
			test.Errorf("%v", validation_error)
		}
	}
}

func TestAPanicInsideAValidationBecomesAClassifiedError(test *testing.T) {
	var nil_map map[string]int
	var short []int
	causes := map[string]func() error{
		"a panic with a string that holds a secret": func() error { panic("boom FAKE-SECRET-VALUE-1234") },
		"a panic with an error":                     func() error { panic(errors.New("FAKE-SECRET-VALUE-1234")) },
		"a panic with nil":                          func() error { panic(nil) },
		"a write to a nil map":                      func() error { nil_map["key"] = 1; return nil },
		"an index out of range":                     func() error { return errors.New(string(rune(short[3]))) },
		"a nil function call":                       func() error { var call func() error; return call() },
	}
	for name, cause := range causes {
		test.Run(name, func(test *testing.T) {
			validation_error := event_validation.Safely(value_objects.EventTypeLogEntryRecorded, cause)
			if code := classified_code(validation_error); code != value_objects.ErrorCodeEventSchemaInvalid {
				test.Fatalf("code %q: %v", code, validation_error)
			}
			if !strings.HasSuffix(validation_error.Error(), "\nlog.entry.recorded: validation failed") {
				test.Errorf("message %q", validation_error)
			}
			if strings.Contains(validation_error.Error(), "FAKE-SECRET-VALUE-1234") {
				test.Errorf("the message contains the panic value: %q", validation_error)
			}
		})
	}
}

func TestSafelyReturnsWhatTheValidationReturns(test *testing.T) {
	if validation_error := event_validation.Safely("x", func() error { return nil }); validation_error != nil {
		test.Errorf("got %v", validation_error)
	}
	given := errors.New("given")
	if validation_error := event_validation.Safely("x", func() error { return given }); !errors.Is(validation_error, given) {
		test.Errorf("got %v", validation_error)
	}
}

func TestAnEventTypeThatIsLongOrHostileIsCutInTheMessage(test *testing.T) {
	hostile := strings.Repeat("a", 500) + "\n[forged log line]"
	validation_error := event_validation.ValidateEvent(hostile, map[string]any{})
	if validation_error == nil || strings.Contains(validation_error.Error(), "forged") || strings.Count(validation_error.Error(), "\n") != 1 {
		test.Errorf("the message must not carry the tail of a hostile name nor a line break: %q", validation_error)
	}
}
