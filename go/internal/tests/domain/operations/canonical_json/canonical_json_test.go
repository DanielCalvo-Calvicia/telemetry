package canonical_json_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

type string_case struct {
	Name     string `json:"name"`
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

type value_case struct {
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	Expected string          `json:"expected"`
}

func TestStringsOfTheSharedTable(test *testing.T) {
	for _, current := range load_cases[string_case](test, "strings.json") {
		test.Run(current.Name, func(test *testing.T) {
			serialized, serialize_error := canonical_json.CanonicalJSON(current.Input)
			if serialize_error != nil || string(serialized) != current.Expected {
				test.Fatalf("got %q (%v), want %q", serialized, serialize_error, current.Expected)
			}
		})
	}
}

func TestSortingOfTheSharedTable(test *testing.T) {
	for _, current := range load_cases[value_case](test, "sorting.json") {
		test.Run(current.Name, func(test *testing.T) {
			var plain any
			if decode_error := json.Unmarshal(current.Input, &plain); decode_error != nil {
				test.Fatal(decode_error)
			}
			for label, input := range map[string]any{"float64": plain, "json.Number": decode_with_numbers(test, current.Input)} {
				serialized, serialize_error := canonical_json.CanonicalJSON(input)
				if serialize_error != nil || string(serialized) != current.Expected {
					test.Errorf("%s: got %q (%v), want %q", label, serialized, serialize_error, current.Expected)
				}
			}
		})
	}
}

func TestKeysOfNonBMPCharactersSortByUTF16CodeUnits(test *testing.T) {
	// U+1F600 is the surrogate pair D83D DE00, which sorts before U+FB33; by code point it would sort after.
	serialized, serialize_error := canonical_json.CanonicalJSON(map[string]int{"\ufb33": 1, "\U0001F600": 2})
	if serialize_error != nil || string(serialized) != "{\"\U0001F600\":2,\"\ufb33\":1}" {
		test.Errorf("got %q %v", serialized, serialize_error)
	}
}

func TestTheOrderOfTheFieldsOfAStructNeverChangesTheOutput(test *testing.T) {
	type one_order struct {
		Alpha string `json:"alpha"`
		Beta  int    `json:"beta"`
		Gamma bool   `json:"gamma"`
	}
	type other_order struct {
		Gamma bool   `json:"gamma"`
		Alpha string `json:"alpha"`
		Beta  int    `json:"beta"`
	}
	first, first_error := canonical_json.CanonicalJSON(one_order{Alpha: "a", Beta: 2, Gamma: true})
	second, second_error := canonical_json.CanonicalJSON(other_order{Alpha: "a", Beta: 2, Gamma: true})
	if first_error != nil || second_error != nil || string(first) != string(second) || string(first) != `{"alpha":"a","beta":2,"gamma":true}` {
		test.Errorf("%q %q %v %v", first, second, first_error, second_error)
	}
}

type tagged struct {
	Name     string         `json:"name"`
	Skipped  string         `json:"-"`
	Optional string         `json:"optional,omitempty"`
	Count    int            `json:"count,omitempty"`
	Flag     bool           `json:"flag,omitempty"`
	Ratio    float64        `json:"ratio,omitempty"`
	Number   uint8          `json:"number,omitempty"`
	List     []string       `json:"list,omitempty"`
	Map      map[string]int `json:"map,omitempty"`
	Pointer  *int           `json:"pointer,omitempty"`
	Interior any            `json:"interior,omitempty"`
	NoTag    string
	hidden   string
}

func TestStructTagsFollowEncodingJSON(test *testing.T) {
	serialized, serialize_error := canonical_json.CanonicalJSON(tagged{Name: "n", Skipped: "s", NoTag: "t", hidden: "h"})
	if serialize_error != nil || string(serialized) != `{"NoTag":"t","name":"n"}` {
		test.Errorf("empty optional fields: %q %v", serialized, serialize_error)
	}
	count := 5
	serialized, serialize_error = canonical_json.CanonicalJSON(&tagged{
		Name: "n", Optional: "o", Count: 1, Flag: true, Ratio: 0.5, Number: 3, List: []string{"x"},
		Map: map[string]int{"k": 1}, Pointer: &count, Interior: "i",
	})
	expected := `{"NoTag":"","count":1,"flag":true,"interior":"i","list":["x"],"map":{"k":1},"name":"n","number":3,"optional":"o","pointer":5,"ratio":0.5}`
	if serialize_error != nil || string(serialized) != expected {
		test.Errorf("full: %q %v", serialized, serialize_error)
	}
}

func TestNilAndEmptyValues(test *testing.T) {
	var nil_pointer *int
	var nil_map map[string]any
	var nil_slice []int
	cases := []struct {
		input    any
		expected string
	}{
		{nil, "null"}, {nil_pointer, "null"}, {nil_map, "null"}, {nil_slice, "null"},
		{map[string]any{}, "{}"}, {[]int{}, "[]"}, {[2]int{1, 2}, "[1,2]"}, {true, "true"}, {false, "false"},
		{[]any{nil, "a", 1, 2.5, false}, `[null,"a",1,2.5,false]`}, {int8(-5), "-5"}, {uint16(7), "7"},
	}
	for _, current := range cases {
		serialized, serialize_error := canonical_json.CanonicalJSON(current.input)
		if serialize_error != nil || string(serialized) != current.expected {
			test.Errorf("%#v: got %q (%v), want %q", current.input, serialized, serialize_error, current.expected)
		}
	}
}

type marshaling_type struct{}

func (marshaling_type) MarshalJSON() ([]byte, error) { return []byte(`{"b": 1.50, "a": "x"}`), nil }

type failing_type struct{}

func (failing_type) MarshalJSON() ([]byte, error) { return nil, errors.New("cannot marshal") }

type broken_type struct{}

func (broken_type) MarshalJSON() ([]byte, error) { return []byte(`{"a":`), nil }

type text_type struct{}

func (text_type) MarshalText() ([]byte, error) { return []byte("plain text"), nil }

type embedding_type struct {
	text_type
}

func TestTypesWithTheirOwnJSONFormAreCanonicalized(test *testing.T) {
	serialized, serialize_error := canonical_json.CanonicalJSON(marshaling_type{})
	if serialize_error != nil || string(serialized) != `{"a":"x","b":1.5}` {
		test.Errorf("marshaler: %q %v", serialized, serialize_error)
	}
	serialized, serialize_error = canonical_json.CanonicalJSON(text_type{})
	if serialize_error != nil || string(serialized) != `"plain text"` {
		test.Errorf("text marshaler: %q %v", serialized, serialize_error)
	}
	for _, input := range []any{failing_type{}, broken_type{}} {
		if _, serialize_error = canonical_json.CanonicalJSON(input); error_code(serialize_error) != value_objects.ErrorCodeEventSchemaInvalid {
			test.Errorf("%T: %v", input, serialize_error)
		}
	}
}

func TestValuesWithoutAJSONFormAreRejectedWithAClassifiedError(test *testing.T) {
	cases := map[string]any{
		"channel":              make(chan int),
		"function":             func() {},
		"complex":              complex(1, 2),
		"bytes":                []byte("x"),
		"integer keys":         map[int]string{1: "x"},
		"anonymous field":      embedding_type{},
		"invalid utf-8 string": "a\xffb",
		"invalid utf-8 key":    map[string]int{"a\xffb": 1},
		"nan":                  math.NaN(),
		"infinity inside":      map[string]any{"a": []any{math.Inf(1)}},
		"big integer inside":   map[string]any{"a": []any{int64(1) << 60}},
		"big unsigned":         uint64(1) << 63,
		"nan in a struct":      struct{ Value float64 }{math.NaN()},
		"nan in an array":      [1]float64{math.NaN()},
		"bad key in a map":     map[string]any{"a": make(chan int)},
	}
	for name, input := range cases {
		serialized, serialize_error := canonical_json.CanonicalJSON(input)
		if name == "anonymous field" {
			// The embedded type has its own text form, so the whole value is a string.
			if serialize_error != nil || string(serialized) != `"plain text"` {
				test.Errorf("%s: %q %v", name, serialized, serialize_error)
			}
			continue
		}
		if serialize_error == nil || serialized != nil || error_code(serialize_error) == "" {
			test.Errorf("%s: accepted (%q) or not classified: %v", name, serialized, serialize_error)
		}
	}
	_, serialize_error := canonical_json.CanonicalJSON("a\xffb")
	if !errors.Is(serialize_error, domain.ErrUnsupportedValue) {
		test.Errorf("the sentinel is lost: %v", serialize_error)
	}
}

func TestAnEmbeddedFieldIsNotSupported(test *testing.T) {
	type plain_embedded struct{ Name string }
	type outer struct {
		plain_embedded
	}
	if _, serialize_error := canonical_json.CanonicalJSON(outer{}); !errors.Is(serialize_error, domain.ErrUnsupportedValue) {
		test.Errorf("embedded: %v", serialize_error)
	}
}

// event_case is one case of events.json: the input envelope and the canonical text both languages must produce.
type event_case struct {
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	Expected string          `json:"expected"`
}

func TestEventsOfTheSharedTable(test *testing.T) {
	for _, current := range load_cases[event_case](test, "events.json") {
		test.Run(current.Name, func(test *testing.T) {
			var envelope value_objects.Envelope
			if decode_error := json.Unmarshal(current.Input, &envelope); decode_error != nil {
				test.Fatal(decode_error)
			}
			var plain any
			if decode_error := json.Unmarshal(current.Input, &plain); decode_error != nil {
				test.Fatal(decode_error)
			}
			inputs := map[string]any{
				"envelope":         envelope,
				"envelope pointer": &envelope,
				"generic":          plain,
				"generic numbers":  decode_with_numbers(test, current.Input),
			}
			for label, input := range inputs {
				serialized, serialize_error := canonical_json.CanonicalJSON(input)
				if serialize_error != nil || string(serialized) != current.Expected {
					test.Errorf("%s:\n got  %q (%v)\n want %q", label, serialized, serialize_error, current.Expected)
				}
			}
		})
	}
}

func TestEventTimesAlwaysHaveSixDecimalsAndTheZ(test *testing.T) {
	for _, current := range load_cases[event_case](test, "events.json") {
		var envelope value_objects.Envelope
		if decode_error := json.Unmarshal(current.Input, &envelope); decode_error != nil {
			test.Fatal(decode_error)
		}
		for _, text := range []string{envelope.OccurredAt, envelope.RecordedAt} {
			if len(text) != len("2026-10-28T10:00:00.000000Z") || text[19] != '.' || text[len(text)-1] != 'Z' {
				test.Errorf("%s: %q is not in the fixed format", current.Name, text)
			}
		}
	}
}

// TestWriteCanonicalFixtures writes the canonical bytes of every case of events.json to the folder named by
// CANONICAL_JSON_OUT_DIR, one file per case. The comparison script integration-tests/compare_canonical_json.py runs it
// and compares the files with the Python output byte by byte. Without the variable it does nothing.
func TestWriteCanonicalFixtures(test *testing.T) {
	out_directory := os.Getenv("CANONICAL_JSON_OUT_DIR")
	if out_directory == "" {
		test.Skip("CANONICAL_JSON_OUT_DIR is not set")
	}
	for _, current := range load_cases[event_case](test, "events.json") {
		var envelope value_objects.Envelope
		if decode_error := json.Unmarshal(current.Input, &envelope); decode_error != nil {
			test.Fatal(decode_error)
		}
		serialized, serialize_error := canonical_json.CanonicalJSON(envelope)
		if serialize_error != nil {
			test.Fatal(serialize_error)
		}
		if write_error := os.WriteFile(filepath.Join(out_directory, current.Name+".json"), serialized, 0o600); write_error != nil {
			test.Fatal(write_error)
		}
	}
}

func TestClassifiedErrorKeepsTheCodeTheSentinelAndTheDetail(test *testing.T) {
	cause := errors.New("the detail")
	with_detail := domain.NewClassifiedError(value_objects.ErrorCodeLimitExceeded, domain.ErrNumberOutOfRange, cause)
	if !errors.Is(with_detail, domain.ErrNumberOutOfRange) || !errors.Is(with_detail, cause) || error_code(with_detail) != "limit_exceeded" {
		test.Errorf("the error lost a part: %v", with_detail)
	}
	if message := with_detail.Error(); !strings.HasPrefix(message, "limit_exceeded: ") || !strings.Contains(message, "the detail") {
		test.Errorf("message: %q", message)
	}
	without_detail := domain.NewClassifiedError(value_objects.ErrorCodeEventSchemaInvalid, domain.ErrNonUTCTime, nil)
	if !errors.Is(without_detail, domain.ErrNonUTCTime) || without_detail.Error() != "event_schema_invalid: "+domain.ErrNonUTCTime.Error() {
		test.Errorf("without detail: %v", without_detail)
	}
}
