package e2e_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/tests/fixtures"
)

func TestAConstructThatTheValidationCannotHonourStopsTheGeneratorAndWritesNothing(test *testing.T) {
	data_schema := func(property string) string {
		return `{"$id": "https://telemetry.example/fixture/schemas/demo/thing_happened.schema.json", "type": "object",
		  "properties": {"category": {"type": "string"}, "note": ` + property + `}}`
	}
	cases := []struct {
		name    string
		file    string
		content string
		keyword string
	}{
		{"a pattern in the data schema", "schemas/demo/thing_happened.schema.json",
			data_schema(`{"type": "string", "pattern": "^a"}`), "pattern"},
		{"a format in the data schema", "schemas/demo/thing_happened.schema.json",
			data_schema(`{"type": "string", "format": "uri"}`), "format"},
		{"a pattern in a common schema the data reaches", "schemas/common/resource.schema.json",
			`{"$id": "https://telemetry.example/fixture/schemas/common/resource.schema.json", "type": "object",
			  "properties": {"service_name": {"type": "string", "pattern": "^a"}}}`, "pattern"},
		{"a keyword of the object in the data schema", "schemas/demo/thing_happened.schema.json",
			`{"$id": "https://telemetry.example/fixture/schemas/demo/thing_happened.schema.json", "type": "object",
			  "minProperties": 1, "properties": {"category": {"type": "string"}}}`, "minProperties"},
		{"an additionalProperties that is a schema", "schemas/demo/thing_happened.schema.json",
			`{"$id": "https://telemetry.example/fixture/schemas/demo/thing_happened.schema.json", "type": "object",
			  "additionalProperties": {"type": "string"}, "properties": {"category": {"type": "string"}}}`, "additionalProperties"},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			contracts_dir := fixtures.CopyContract(test)
			fixtures.Write(test, contracts_dir, each.file, each.content)
			folders := missing_output_folders(test)
			generate_error := generate(contracts_dir, folders)
			if !errors.Is(generate_error, application.ErrUnsupportedSchema) {
				test.Fatalf("error %v does not wrap ErrUnsupportedSchema", generate_error)
			}
			message := generate_error.Error()
			if !strings.Contains(message, "unsupported schema construct: contracts/"+each.file) || !strings.Contains(message, each.keyword) {
				test.Errorf("error %q does not name the file and the keyword %q", message, each.keyword)
			}
			for _, folder := range folders.all() {
				if snapshot(test, folder) != nil {
					test.Errorf("nothing must be written, but %s exists", folder)
				}
			}
		})
	}
}

func TestAPatternInAnObjectNoEventReachesIsNotAnError(test *testing.T) {
	contracts_dir := fixtures.CopyContract(test)
	fixtures.Write(test, contracts_dir, "schemas/common/context.schema.json",
		`{"$id": "https://telemetry.example/fixture/schemas/common/context.schema.json", "type": "object",
		  "properties": {"trace_id": {"type": "string", "pattern": "^[0-9a-f]{32}$"}}, "additionalProperties": false}`)
	if generate_error := generate(contracts_dir, new_output_folders(test)); generate_error != nil {
		test.Fatal(generate_error)
	}
}
