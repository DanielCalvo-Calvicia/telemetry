package base_value_objects_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
)

// fixtures_directory is the folder of the tables that the Python tests read too (the same files).
const fixtures_directory = "../../../../../integration-tests/fixtures/value_objects"

// value_object_tables are the tables of the value objects; they are read by the tests of that package.
var value_object_tables = map[string]bool{
	"severity.json": true, "actor_kind.json": true, "event_id.json": true, "utc_instant.json": true,
	"resource.json": true, "actor.json": true, "context.json": true, "attributes.json": true,
}

type shared_case struct {
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input"`
	Valid       bool            `json:"valid"`
}

type shared_table struct {
	Description string        `json:"description"`
	Cases       []shared_case `json:"cases"`
}

// hex_digits_input is the input of the hex_digits table.
type hex_digits_input struct {
	Value string `json:"value"`
	Width int    `json:"width"`
}

// checkers maps each table of this package to the function that builds its value object from one input.
var checkers = map[string]func(test *testing.T, input json.RawMessage) error{
	"identifier.json": func(test *testing.T, input json.RawMessage) error {
		var text string
		if decode_error := json.Unmarshal(input, &text); decode_error != nil {
			test.Fatalf("the input %s is not a JSON string: %v", input, decode_error)
		}
		identifier, construction_error := base_value_objects.NewIdentifier(text)
		if construction_error == nil && identifier.String() != text {
			test.Errorf("the identifier was changed from %q to %q", text, identifier.String())
		}
		return construction_error
	},
	"hex_digits.json": func(test *testing.T, input json.RawMessage) error {
		var decoded hex_digits_input
		if decode_error := json.Unmarshal(input, &decoded); decode_error != nil {
			test.Fatalf("the input %s is not {value, width}: %v", input, decode_error)
		}
		hex_digits, construction_error := base_value_objects.NewHexDigits(decoded.Value, decoded.Width)
		if construction_error == nil && hex_digits.String() != decoded.Value {
			test.Errorf("the digits were changed from %q to %q", decoded.Value, hex_digits.String())
		}
		return construction_error
	},
}

func TestEveryTableOfTheSharedFixturesIsKnown(test *testing.T) {
	entries, read_error := os.ReadDir(fixtures_directory)
	if read_error != nil {
		test.Fatalf("cannot read the fixtures folder: %v", read_error)
	}
	if len(entries) == 0 {
		test.Fatal("the fixtures folder is empty")
	}
	for _, entry := range entries {
		if _, is_checked := checkers[entry.Name()]; !is_checked && !value_object_tables[entry.Name()] {
			test.Errorf("the table %s has no check: add it to checkers (or to value_object_tables)", entry.Name())
		}
	}
	for table_name := range checkers {
		if _, stat_error := os.Stat(filepath.Join(fixtures_directory, table_name)); stat_error != nil {
			test.Errorf("the table %s of checkers does not exist: %v", table_name, stat_error)
		}
	}
}

func TestEveryBaseValueObjectAcceptsExactlyTheValidCasesOfTheSharedTables(test *testing.T) {
	for table_name, check := range checkers {
		content, read_error := os.ReadFile(filepath.Join(fixtures_directory, table_name))
		if read_error != nil {
			test.Fatalf("cannot read %s: %v", table_name, read_error)
		}
		var table shared_table
		if decode_error := json.Unmarshal(content, &table); decode_error != nil {
			test.Fatalf("cannot decode %s: %v", table_name, decode_error)
		}
		if len(table.Cases) == 0 {
			test.Fatalf("the table %s has no case", table_name)
		}
		for _, current := range table.Cases {
			test.Run(strings.TrimSuffix(table_name, ".json")+"/"+current.Description, func(test *testing.T) {
				construction_error := check(test, current.Input)
				if current.Valid && construction_error != nil {
					test.Errorf("a valid input was rejected: %v", construction_error)
				}
				if !current.Valid && construction_error == nil {
					test.Errorf("an invalid input %s was accepted", current.Input)
				}
			})
		}
	}
}
