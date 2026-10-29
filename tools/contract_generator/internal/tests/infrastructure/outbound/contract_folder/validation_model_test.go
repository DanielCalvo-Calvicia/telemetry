package contract_folder_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/tests/fixtures"
)

// field_of returns a field of a struct of the contract the reader built.
func field_of(test *testing.T, contract domain.Contract, struct_name string, json_name string) domain.Field {
	test.Helper()
	for _, declared := range contract.Structs {
		if declared.Name != struct_name {
			continue
		}
		for _, field := range declared.Fields {
			if field.JSONName == json_name {
				return field
			}
		}
	}
	test.Fatalf("no field %s in %s", json_name, struct_name)
	return domain.Field{}
}

func TestTheFixtureContractCarriesWhatTheValidationNeeds(test *testing.T) {
	contract, read_error := read(fixtures.ContractDir())
	if read_error != nil {
		test.Fatal(read_error)
	}
	expected_limits := domain.AttributesLimits{
		Source: "contracts/schemas/common/attributes.schema.json", MaxProperties: 3, MinKeyLength: 1, MaxKeyLength: 8, MaxStringLength: 10,
	}
	if contract.Attributes != expected_limits {
		test.Errorf("attributes limits: %+v, want %+v", contract.Attributes, expected_limits)
	}
	if note := field_of(test, contract, "demo_thing_happened", "note"); note.MinLength != 1 || note.MaxLength != 20 || note.Unsupported != "" {
		test.Errorf("note: %+v", note)
	}
	if category := field_of(test, contract, "demo_thing_happened", "category"); category.MinLength != 0 || category.MaxLength != 0 {
		test.Errorf("category: %+v", category)
	}
	for _, declared := range contract.Structs {
		if !declared.Closed || declared.Unsupported != "" {
			test.Errorf("%s: closed %v, unsupported %q", declared.Name, declared.Closed, declared.Unsupported)
		}
	}
}

func TestAnObjectWithoutAdditionalPropertiesFalseIsOpen(test *testing.T) {
	directory := fixtures.CopyContract(test)
	fixtures.Write(test, directory, "schemas/common/context.schema.json", `{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json",
	  "type": "object", "properties": {"trace_id": {"type": "string"}}, "additionalProperties": true}`)
	contract, read_error := read(directory)
	if read_error != nil {
		test.Fatal(read_error)
	}
	for _, declared := range contract.Structs {
		if declared.Name == "context" && (declared.Closed || declared.Unsupported != "") {
			test.Errorf("context: closed %v, unsupported %q", declared.Closed, declared.Unsupported)
		}
	}
}

func TestTheKeywordsThatTheValidationCannotHonourAreRecordedNotRejectedWhileNoEventReachesThem(test *testing.T) {
	directory := fixtures.CopyContract(test)
	fixtures.Write(test, directory, "schemas/common/context.schema.json", `{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json",
	  "type": "object", "minProperties": 1, "additionalProperties": {"type": "string"},
	  "properties": {"trace_id": {"type": "string", "pattern": "^a", "format": "hex"}, "span_id": {"type": "string", "description": "ok", "title": "t",
	  "$comment": "c", "examples": ["a"]}}}`)
	contract, read_error := read(directory)
	if read_error != nil {
		test.Fatal(read_error)
	}
	if trace := field_of(test, contract, "context", "trace_id"); trace.Unsupported != "format, pattern" {
		test.Errorf("trace_id: %q", trace.Unsupported)
	}
	if span := field_of(test, contract, "context", "span_id"); span.Unsupported != "" {
		test.Errorf("span_id: %q", span.Unsupported)
	}
	for _, declared := range contract.Structs {
		if declared.Name == "context" && (declared.Closed || declared.Unsupported != "additionalProperties, minProperties") {
			test.Errorf("context: closed %v, unsupported %q", declared.Closed, declared.Unsupported)
		}
	}
}

func TestAMaxLengthBelowOneIsRejected(test *testing.T) {
	directory := fixtures.CopyContract(test)
	fixtures.Write(test, directory, "schemas/common/context.schema.json", `{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json",
	  "type": "object", "properties": {"trace_id": {"type": "string", "maxLength": 0}}}`)
	_, read_error := read(directory)
	if !errors.Is(read_error, application.ErrUnsupportedSchema) || !strings.Contains(read_error.Error(), "schemas/common/context.schema.json") ||
		!strings.Contains(read_error.Error(), "maxLength") {
		test.Errorf("error %v", read_error)
	}
}

func TestTheLimitsOfTheAttributesAreReadWhenTheyAreWrittenAsWholeNumbers(test *testing.T) {
	cases := []struct {
		name     string
		schema   string
		expected domain.AttributesLimits
	}{
		{"whole numbers with a zero fraction", `"maxProperties": 4.0, "propertyNames": {"minLength": 2.0, "maxLength": 9.0}`,
			domain.AttributesLimits{MaxProperties: 4, MinKeyLength: 2, MaxKeyLength: 9}},
		{"a number that is not whole means no limit", `"maxProperties": 4.5, "propertyNames": {"maxLength": 9.5}`, domain.AttributesLimits{}},
		{"no limit stated", `"additionalProperties": true`, domain.AttributesLimits{}},
		{"a number too large for any float", `"maxProperties": 1e999`, domain.AttributesLimits{}},
		{"a limit that is not a number", `"maxProperties": "many"`, domain.AttributesLimits{}},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			directory := fixtures.CopyContract(test)
			fixtures.Write(test, directory, "schemas/common/attributes.schema.json",
				`{"$id": "https://telemetry.example/fixture/schemas/common/attributes.schema.json", "type": "object", `+each.schema+`}`)
			contract, read_error := read(directory)
			if read_error != nil {
				// A limit that is not a valid number is a schema error: the reader reports it and the model keeps no limit.
				if !errors.Is(read_error, application.ErrInvalidSchema) {
					test.Fatal(read_error)
				}
				return
			}
			expected := each.expected
			expected.Source = "contracts/schemas/common/attributes.schema.json"
			if contract.Attributes != expected {
				test.Errorf("limits %+v, want %+v", contract.Attributes, expected)
			}
		})
	}
}
