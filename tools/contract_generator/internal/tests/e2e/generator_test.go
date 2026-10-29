package e2e_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/composition_root"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/infrastructure/inbound/cli"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/tests/fixtures"
)

var generated_names = []string{"enums_generated", "event_types_generated", "error_codes_generated", "envelope_generated"}

// validators_name is the file of the generated validators, in the folder of each language.
const validators_name = "event_validators_generated"

// output_folders are the four folders the generator writes.
type output_folders struct {
	go_out, python_out, go_validation_out, python_validation_out string
}

func new_output_folders(test *testing.T) output_folders {
	return output_folders{test.TempDir(), test.TempDir(), test.TempDir(), test.TempDir()}
}

// missing_output_folders are four folders that do not exist yet, to prove that nothing is written into them.
func missing_output_folders(test *testing.T) output_folders {
	missing := func(name string) string { return filepath.Join(test.TempDir(), name) }
	return output_folders{missing("go"), missing("py"), missing("go_validation"), missing("py_validation")}
}

func (folders output_folders) all() []string {
	return []string{folders.go_out, folders.python_out, folders.go_validation_out, folders.python_validation_out}
}

// snapshot returns the content of every file of a folder by name, or nil if it does not exist.
func snapshot(test *testing.T, directory string) map[string]string {
	test.Helper()
	entries, read_error := os.ReadDir(directory)
	if read_error != nil {
		return nil
	}
	files := map[string]string{}
	for _, entry := range entries {
		content, _ := os.ReadFile(filepath.Join(directory, entry.Name()))
		files[entry.Name()] = string(content)
	}
	return files
}

func generate(contracts_dir string, folders output_folders) error {
	return composition_root.NewGenerator(
		contracts_dir, folders.go_out, folders.python_out, folders.go_validation_out, folders.python_validation_out,
	).Execute()
}

func TestTheFixtureContractGeneratesTheExpectedFilesByteByByte(test *testing.T) {
	folders := new_output_folders(test)
	if generate_error := generate(fixtures.ContractDir(), folders); generate_error != nil {
		test.Fatal(generate_error)
	}
	go_files, python_files := snapshot(test, folders.go_out), snapshot(test, folders.python_out)
	if len(go_files) != 4 || len(python_files) != 4 {
		test.Fatalf("got %d Go and %d Python files, want 4 and 4", len(go_files), len(python_files))
	}
	go_validation, python_validation := snapshot(test, folders.go_validation_out), snapshot(test, folders.python_validation_out)
	if len(go_validation) != 1 || len(python_validation) != 1 {
		test.Fatalf("got %d Go and %d Python validation files, want 1 and 1", len(go_validation), len(python_validation))
	}
	if go_validation[validators_name+".go"] != fixtures.Expected(test, "go/"+validators_name+".go.golden") {
		test.Error("the Go validators differ from their expected file")
	}
	if python_validation[validators_name+".py"] != fixtures.Expected(test, "python/"+validators_name+".py.golden") {
		test.Error("the Python validators differ from their expected file")
	}
	for _, name := range generated_names {
		if got, expected := go_files[name+".go"], fixtures.Expected(test, "go/"+name+".go.golden"); got != expected {
			test.Errorf("%s.go differs from its expected file", name)
		}
		if got, expected := python_files[name+".py"], fixtures.Expected(test, "python/"+name+".py.golden"); got != expected {
			test.Errorf("%s.py differs from its expected file", name)
		}
	}
}

func TestASecondRunChangesNothing(test *testing.T) {
	folders := new_output_folders(test)
	if generate_error := generate(fixtures.ContractDir(), folders); generate_error != nil {
		test.Fatal(generate_error)
	}
	take := func() []map[string]string {
		var taken []map[string]string
		for _, folder := range folders.all() {
			taken = append(taken, snapshot(test, folder))
		}
		return taken
	}
	first := take()
	if generate_error := generate(fixtures.ContractDir(), folders); generate_error != nil {
		test.Fatal(generate_error)
	}
	for position, second := range take() {
		if len(second) != len(first[position]) || len(second) == 0 {
			test.Errorf("folder %d has a different number of files in the second run", position)
		}
		for name, content := range first[position] {
			if second[name] != content {
				test.Errorf("%s changed in the second run", name)
			}
		}
	}
}

func TestABrokenContractStopsTheGeneratorWithTheFileAndWritesNothing(test *testing.T) {
	registry_with := func(event_type string, schema string) string {
		return "events:\n  - event_type: " + event_type + "\n    schema: " + schema + "\n"
	}
	catalog_with := func(entries ...string) string { return "errors:\n" + strings.Join(entries, "") }
	entry := func(code string, category string, message string) string {
		return "  - code: " + code + "\n    category: " + category + "\n    default_message: " + message + "\n"
	}
	cases := []struct {
		name     string
		file     string
		content  string
		removes  string
		sentinel error
	}{
		{name: "a schema that is not registered", file: "schemas/demo/other_thing.schema.json",
			content:  `{"$id": "https://telemetry.example/fixture/schemas/demo/other_thing.schema.json", "type": "object"}`,
			sentinel: domain.ErrSchemaNotRegistered},
		{name: "a registered type with no schema", removes: "schemas/demo/thing_happened.schema.json",
			file: "events/registry.yaml", sentinel: domain.ErrSchemaMissing},
		{name: "a duplicated error code", file: "errors/catalog.yaml",
			content:  catalog_with(entry("demo_failed", "validation", "one"), entry("demo_failed", "validation", "two")),
			sentinel: domain.ErrDuplicateErrorCode},
		{name: "a duplicated enum value", file: "enums/severity.json",
			content:  `{"$id": "https://telemetry.example/fixture/enums/severity.json", "type": "string", "enum": ["INFO", "INFO"]}`,
			sentinel: domain.ErrDuplicateEnumValue},
		{name: "an invalid JSON file", file: "enums/severity.json", content: "{ nope", sentinel: application.ErrInvalidJSON},
		{name: "an invalid schema", file: "schemas/common/resource.schema.json",
			content:  `{"$id": "https://telemetry.example/fixture/schemas/common/resource.schema.json", "properties": 7}`,
			sentinel: application.ErrInvalidSchema},
		{name: "an unknown error category", file: "errors/catalog.yaml",
			content: catalog_with(entry("demo_failed", "nonsense", "one")), sentinel: domain.ErrUnknownErrorCategory},
		{name: "an error code in capitals", file: "errors/catalog.yaml",
			content: catalog_with(entry("Demo_Failed", "validation", "one")), sentinel: domain.ErrInvalidErrorCode},
		{name: "an error code without a message", file: "errors/catalog.yaml",
			content: catalog_with("  - code: demo_failed\n    category: validation\n"), sentinel: domain.ErrMissingDefaultMessage},
		{name: "an event type of two segments", file: "events/registry.yaml",
			content: registry_with("demo.thing", "schemas/demo/thing_happened.schema.json"), sentinel: domain.ErrInvalidEventType},
		{name: "a schema that does not match the type", file: "events/registry.yaml",
			content: registry_with("demo.thing.happened", "schemas/demo/elsewhere.schema.json"), sentinel: domain.ErrSchemaPathMismatch},
		{name: "a duplicated event type", file: "events/registry.yaml",
			content: registry_with("demo.thing.happened", "schemas/demo/thing_happened.schema.json") +
				"  - event_type: demo.thing.happened\n    schema: schemas/demo/thing_happened.schema.json\n",
			sentinel: domain.ErrDuplicateEventType},
		{name: "an enum value that is not an identifier", file: "enums/severity.json",
			content:  `{"$id": "https://telemetry.example/fixture/enums/severity.json", "type": "string", "enum": ["not valid"]}`,
			sentinel: domain.ErrInvalidEnumValue},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			contracts_dir := fixtures.CopyContract(test)
			if each.removes != "" {
				fixtures.Remove(test, contracts_dir, each.removes)
			}
			if each.content != "" {
				fixtures.Write(test, contracts_dir, each.file, each.content)
			}
			folders := missing_output_folders(test)
			generate_error := generate(contracts_dir, folders)
			if !errors.Is(generate_error, each.sentinel) {
				test.Fatalf("error %v does not wrap %v", generate_error, each.sentinel)
			}
			if !strings.Contains(generate_error.Error(), each.file) {
				test.Errorf("error %q does not name %s", generate_error, each.file)
			}
			for _, folder := range folders.all() {
				if snapshot(test, folder) != nil {
					test.Errorf("nothing must be written for a broken contract, but %s exists", folder)
				}
			}
		})
	}
}

func TestTheCommandLineGeneratesAndReportsFailures(test *testing.T) {
	var messages strings.Builder
	arguments := func(contracts_dir string) []string {
		return []string{"--contracts", contracts_dir, "--go-out", test.TempDir(), "--python-out", test.TempDir(),
			"--go-validation-out", test.TempDir(), "--python-validation-out", test.TempDir()}
	}
	if code := cli.Run(arguments(fixtures.ContractDir()), &messages, composition_root.NewGenerator); code != cli.ExitOK {
		test.Fatalf("code %d: %s", code, messages.String())
	}
	broken := fixtures.CopyContract(test)
	fixtures.Remove(test, broken, "events/registry.yaml")
	if code := cli.Run(arguments(broken), &messages, composition_root.NewGenerator); code != cli.ExitFailure ||
		!strings.Contains(messages.String(), "events/registry.yaml") {
		test.Errorf("code %d, messages %q", code, messages.String())
	}
}
