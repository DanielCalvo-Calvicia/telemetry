package masking_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
)

// fixtures_directory is the folder of the tables that the Python tests read too (the same files).
const fixtures_directory = "../../../../../../integration-tests/fixtures/masking"

type table_case struct {
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input"`
	Expected    json.RawMessage `json:"expected"`
}

type table_file struct {
	Description string       `json:"description"`
	Oracle      string       `json:"oracle"`
	Cases       []table_case `json:"cases"`
}

// load_table reads a table of the shared fixtures and fails when it is missing, unreadable or empty.
func load_table(test *testing.T, file_name string) []table_case {
	test.Helper()
	content, read_error := os.ReadFile(filepath.Join(fixtures_directory, file_name))
	if read_error != nil {
		test.Fatalf("cannot read the table %s: %v", file_name, read_error)
	}
	var table table_file
	if decode_error := json.Unmarshal(content, &table); decode_error != nil {
		test.Fatalf("cannot decode the table %s: %v", file_name, decode_error)
	}
	if len(table.Cases) == 0 {
		test.Fatalf("the table %s has no case", file_name)
	}
	return table.Cases
}

func decode_any(test *testing.T, raw json.RawMessage) any {
	test.Helper()
	var decoded any
	if decode_error := json.Unmarshal(raw, &decoded); decode_error != nil {
		test.Fatalf("cannot decode %s: %v", raw, decode_error)
	}
	return decoded
}

func canonical_bytes(test *testing.T, value any) string {
	test.Helper()
	bytes, serialize_error := canonical_json.CanonicalJSON(value)
	if serialize_error != nil {
		test.Fatalf("cannot serialize %v: %v", value, serialize_error)
	}
	return string(bytes)
}

func Test_the_text_table_gives_the_same_masked_text(t *testing.T) {
	masker := new_masker(t)
	for _, table_case := range load_table(t, "text.json") {
		t.Run(table_case.Description, func(t *testing.T) {
			input := decode_any(t, table_case.Input).(string)
			expected := decode_any(t, table_case.Expected).(string)
			if actual := masker.MaskText(input); actual != expected {
				t.Errorf("MaskText(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func Test_the_unicode_folding_table_gives_the_same_masked_text(t *testing.T) {
	masker := new_masker(t)
	for _, table_case := range load_table(t, "text_unicode_folding.json") {
		t.Run(table_case.Description, func(t *testing.T) {
			input := decode_any(t, table_case.Input).(string)
			expected := decode_any(t, table_case.Expected).(string)
			if actual := masker.MaskText(input); actual != expected {
				t.Errorf("MaskText(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func Test_the_field_names_table_gives_the_same_answer(t *testing.T) {
	for _, table_case := range load_table(t, "field_names.json") {
		t.Run(table_case.Description, func(t *testing.T) {
			input := decode_any(t, table_case.Input).(string)
			expected := decode_any(t, table_case.Expected).(bool)
			if actual := masking.IsSensitiveFieldName(input); actual != expected {
				t.Errorf("IsSensitiveFieldName(%q) = %v, want %v", input, actual, expected)
			}
		})
	}
}

func Test_the_values_table_gives_the_same_canonical_json(t *testing.T) {
	masker := new_masker(t)
	for _, table_case := range load_table(t, "values.json") {
		t.Run(table_case.Description, func(t *testing.T) {
			var input struct {
				FieldName string          `json:"field_name"`
				Value     json.RawMessage `json:"value"`
			}
			if decode_error := json.Unmarshal(table_case.Input, &input); decode_error != nil {
				t.Fatal(decode_error)
			}
			actual := canonical_bytes(t, masker.MaskValue(input.FieldName, decode_any(t, input.Value)))
			expected := canonical_bytes(t, decode_any(t, table_case.Expected))
			if actual != expected {
				t.Errorf("MaskValue(%q, %s) = %s, want %s", input.FieldName, input.Value, actual, expected)
			}
		})
	}
}
