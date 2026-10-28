package canonical_json_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// fixtures_directory is the folder of the tables that the Python tests read too (the same files).
const fixtures_directory = "../../../fixtures/canonical_json"

type fixture_file struct {
	Cases []json.RawMessage `json:"cases"`
}

// load_cases reads the cases of a fixture file into a slice of the given case type.
func load_cases[Case any](test *testing.T, file_name string) []Case {
	test.Helper()
	content, read_error := os.ReadFile(filepath.Join(fixtures_directory, file_name))
	if read_error != nil {
		test.Fatalf("cannot read the fixture %s: %v", file_name, read_error)
	}
	var file fixture_file
	if decode_error := json.Unmarshal(content, &file); decode_error != nil {
		test.Fatalf("cannot decode the fixture %s: %v", file_name, decode_error)
	}
	if len(file.Cases) == 0 {
		test.Fatalf("the fixture %s has no case", file_name)
	}
	cases := make([]Case, 0, len(file.Cases))
	for _, raw_case := range file.Cases {
		var current Case
		if decode_error := json.Unmarshal(raw_case, &current); decode_error != nil {
			test.Fatalf("cannot decode a case of %s: %v", file_name, decode_error)
		}
		cases = append(cases, current)
	}
	return cases
}

// decode_with_numbers decodes JSON text keeping the text of the numbers (json.Number).
func decode_with_numbers(test *testing.T, raw json.RawMessage) any {
	test.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if decode_error := decoder.Decode(&decoded); decode_error != nil {
		test.Fatalf("cannot decode %s: %v", raw, decode_error)
	}
	return decoded
}

// error_code returns the code of a classified error, or "" when the error is not one.
func error_code(failure error) string {
	var classified *domain.ClassifiedError
	if errors.As(failure, &classified) {
		return classified.Code
	}
	return ""
}
