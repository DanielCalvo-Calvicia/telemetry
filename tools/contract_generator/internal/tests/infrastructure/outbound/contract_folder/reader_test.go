package contract_folder_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/contract_folder"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/outbound/contract_folder/external"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/tests/fixtures"
)

func read(directory string) (domain.Contract, error) {
	return contract_folder.NewReader(external.NewFolder(directory)).Read()
}

func TestTheFixtureContractIsReadIntoTheModel(test *testing.T) {
	contract, read_error := read(fixtures.ContractDir())
	if read_error != nil {
		test.Fatal(read_error)
	}
	contract = contract.Sorted()
	if len(contract.Enums) != 2 || contract.Enums[1].Name != "severity" || contract.Enums[1].Values[1] != "ERROR" {
		test.Errorf("enums: %+v", contract.Enums)
	}
	if len(contract.EventTypes) != 1 || contract.EventTypes[0].Name != "demo.thing.happened" {
		test.Errorf("event types: %+v", contract.EventTypes)
	}
	if len(contract.ErrorCodes) != 2 || contract.ErrorCodes[0].Code != "demo_failed" || contract.ErrorCodes[0].Category != "validation" {
		test.Errorf("error codes: %+v", contract.ErrorCodes)
	}
	names := []string{}
	for _, declared := range contract.Structs {
		names = append(names, declared.Name)
	}
	if strings.Join(names, ",") != "context,demo_thing_happened,envelope,resource" {
		test.Errorf("structs: %v", names)
	}
	if len(contract.DataSchemas) != 1 || contract.DataSchemas[0] != "schemas/demo/thing_happened.schema.json" {
		test.Errorf("data schemas: %v", contract.DataSchemas)
	}
	envelope := contract.Structs[2]
	kinds := map[string]domain.FieldKind{}
	for _, field := range envelope.Fields {
		kinds[field.JSONName] = field.Kind
	}
	expected := map[string]domain.FieldKind{
		"occurred_at": domain.KindString, "severity": domain.KindEnum, "resource": domain.KindStruct, "extra": domain.KindStruct,
		"attributes": domain.KindAttributes, "data": domain.KindMap, "count": domain.KindInteger, "ratio": domain.KindNumber,
		"flag": domain.KindBoolean, "event_id": domain.KindString, "context": domain.KindStruct,
	}
	for name, kind := range expected {
		if kinds[name] != kind {
			test.Errorf("the field %s has the kind %d, want %d", name, kinds[name], kind)
		}
	}
}

func TestAContractFolderThatCannotBeReadIsAnError(test *testing.T) {
	_, read_error := read(test.TempDir() + "/missing")
	if !errors.Is(read_error, application.ErrReadContract) {
		test.Errorf("error %v", read_error)
	}
}

func TestAFolderWithoutTheRegistryOrTheCatalogIsAnError(test *testing.T) {
	for _, missing := range []string{"events/registry.yaml", "errors/catalog.yaml"} {
		directory := fixtures.CopyContract(test)
		fixtures.Remove(test, directory, missing)
		_, read_error := read(directory)
		if !errors.Is(read_error, application.ErrReadContract) || !strings.Contains(read_error.Error(), missing) {
			test.Errorf("%s: error %v", missing, read_error)
		}
	}
}

func TestEveryProblemOfAFileNamesTheFile(test *testing.T) {
	cases := []struct {
		name     string
		file     string
		content  string
		sentinel error
	}{
		{"JSON that is not JSON", "enums/severity.json", "{ not json", application.ErrInvalidJSON},
		{"a JSON document that is not an object", "schemas/common/context.schema.json", "[]", application.ErrInvalidSchema},
		{"a schema without $id", "schemas/common/context.schema.json", `{"type": "object"}`, application.ErrInvalidSchema},
		{"a schema with an invalid keyword", "schemas/common/resource.schema.json",
			`{"$id": "https://telemetry.example/fixture/schemas/common/resource.schema.json", "type": 5}`, application.ErrInvalidSchema},
		{"a reference that does not resolve", "schemas/common/context.schema.json",
			`{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json", "type": "object",
			  "properties": {"x": {"$ref": "https://telemetry.example/fixture/nothing.json"}}}`, application.ErrInvalidSchema},
		{"an enum without a list", "enums/severity.json",
			`{"$id": "https://telemetry.example/fixture/enums/severity.json", "type": "string"}`, application.ErrUnsupportedSchema},
		{"an enum with a value that is not a string", "enums/severity.json",
			`{"$id": "https://telemetry.example/fixture/enums/severity.json", "enum": [1]}`, application.ErrUnsupportedSchema},
		{"a registry that is not YAML", "events/registry.yaml", "events: [unclosed", application.ErrInvalidYAML},
		{"a catalog of the wrong shape", "errors/catalog.yaml", "errors: 5", application.ErrInvalidYAML},
		{"a struct that is not an object", "schemas/common/context.schema.json",
			`{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json", "type": "string"}`, application.ErrUnsupportedSchema},
		{"a required name that is not a property", "schemas/common/context.schema.json",
			`{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json", "type": "object", "required": ["ghost"]}`,
			application.ErrUnsupportedSchema},
		{"an array property", "schemas/common/context.schema.json", property_schema(`{"type": "array"}`), application.ErrUnsupportedSchema},
		{"a property without a type", "schemas/common/context.schema.json", property_schema(`{}`), application.ErrUnsupportedSchema},
		{"a nested object", "schemas/common/context.schema.json",
			property_schema(`{"type": "object", "properties": {"y": {"type": "string"}}}`), application.ErrUnsupportedSchema},
		{"a reference to another definition", "schemas/common/context.schema.json",
			property_schema(`{"$ref": "#/$defs/other"}`, `"other": {"type": "string"}`), application.ErrUnsupportedSchema},
		{"a reference to a schema that is not mapped", "schemas/common/context.schema.json",
			property_schema(`{"$ref": "https://telemetry.example/fixture/schemas/demo/unregistered.schema.json"}`), application.ErrUnsupportedSchema},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			directory := fixtures.CopyContract(test)
			fixtures.Write(test, directory, "schemas/demo/unregistered.schema.json",
				`{"$id": "https://telemetry.example/fixture/schemas/demo/unregistered.schema.json", "type": "object"}`)
			fixtures.Write(test, directory, each.file, each.content)
			_, read_error := read(directory)
			if !errors.Is(read_error, each.sentinel) {
				test.Fatalf("error %v does not wrap %v", read_error, each.sentinel)
			}
			if !strings.Contains(read_error.Error(), each.file) {
				test.Errorf("error %q does not name %s", read_error, each.file)
			}
		})
	}
}

// property_schema is the schema of the context with one property named x, and optionally one extra definition.
func property_schema(property string, definitions ...string) string {
	return `{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json", "type": "object",
	  "properties": {"x": ` + property + `}, "$defs": {` + strings.Join(definitions, ",") + `}}`
}

func TestEveryBrokenFileIsReportedNotOnlyTheFirst(test *testing.T) {
	directory := fixtures.CopyContract(test)
	fixtures.Write(test, directory, "enums/severity.json", "{ not json")
	fixtures.Write(test, directory, "schemas/common/resource.schema.json", "[1,")
	_, read_error := read(directory)
	if read_error == nil || !strings.Contains(read_error.Error(), "enums/severity.json") ||
		!strings.Contains(read_error.Error(), "schemas/common/resource.schema.json") {
		test.Errorf("error %v", read_error)
	}
}
