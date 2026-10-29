package application_test

import (
	"errors"
	"testing"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

type fake_reader struct {
	contract   domain.Contract
	read_error error
}

func (reader fake_reader) Read() (domain.Contract, error) { return reader.contract, reader.read_error }

type fake_renderer struct {
	file         string
	render_error error
	received     *domain.Contract
}

func (renderer fake_renderer) Render(contract domain.Contract) ([]domain.GeneratedFile, error) {
	if renderer.received != nil {
		*renderer.received = contract
	}
	return []domain.GeneratedFile{{Name: renderer.file, Content: []byte(renderer.file)}}, renderer.render_error
}

type fake_writer struct {
	written     *[]string
	write_error error
}

func (writer fake_writer) Write(files []domain.GeneratedFile) error {
	for _, generated := range files {
		*writer.written = append(*writer.written, generated.Name)
	}
	return writer.write_error
}

var (
	read_failure   = errors.New("read failure")
	render_failure = errors.New("render failure")
	write_failure  = errors.New("write failure")
)

func valid_contract() domain.Contract {
	return domain.Contract{
		Enums: []domain.Enum{
			{Name: "severity", Source: "contracts/enums/severity.json", Values: []string{"INFO"}},
			{Name: "failure_category", Source: "contracts/enums/failure_category.json", Values: []string{"validation"}},
		},
	}
}

func TestTheCodeOfBothLanguagesIsRenderedFromTheSortedContractAndWritten(test *testing.T) {
	var written []string
	var received domain.Contract
	use_case := application.NewGenerateCode(
		fake_reader{contract: valid_contract()},
		application.Target{Renderer: fake_renderer{file: "go_file", received: &received}, Writer: fake_writer{written: &written}},
		application.Target{Renderer: fake_renderer{file: "python_file"}, Writer: fake_writer{written: &written}},
		application.Target{Renderer: fake_renderer{file: "go_validation_file"}, Writer: fake_writer{written: &written}},
	)
	if execute_error := use_case.Execute(); execute_error != nil {
		test.Fatal(execute_error)
	}
	if len(written) != 3 || written[0] != "go_file" || written[1] != "python_file" || written[2] != "go_validation_file" {
		test.Errorf("written: %v", written)
	}
	if received.Enums[0].Name != "failure_category" {
		test.Error("the renderer must receive the sorted contract")
	}
}

func TestNothingIsWrittenWhenTheContractCannotBeUsed(test *testing.T) {
	invalid := valid_contract()
	invalid.Enums[0].Values = []string{"INFO", "INFO"}
	unvalidatable := valid_contract()
	unvalidatable.EventTypes = []domain.EventType{{Name: "demo.thing.happened", Schema: "schemas/demo/thing_happened.schema.json"}}
	unvalidatable.DataSchemas = []string{"schemas/demo/thing_happened.schema.json"}
	unvalidatable.Structs = []domain.Struct{{Name: "demo_thing_happened", Source: "contracts/schemas/demo/thing_happened.schema.json",
		Fields: []domain.Field{{JSONName: "note", Unsupported: "pattern"}}}}
	cases := []struct {
		name     string
		reader   fake_reader
		first    fake_renderer
		second   fake_renderer
		sentinel error
	}{
		{"the contract cannot be read", fake_reader{read_error: read_failure}, fake_renderer{}, fake_renderer{}, read_failure},
		{"the contract is invalid", fake_reader{contract: invalid}, fake_renderer{}, fake_renderer{}, application.ErrInvalidContract},
		{"the contract has a construct that cannot be validated", fake_reader{contract: unvalidatable}, fake_renderer{}, fake_renderer{}, application.ErrUnsupportedSchema},
		{"the first renderer fails", fake_reader{contract: valid_contract()}, fake_renderer{render_error: render_failure}, fake_renderer{}, render_failure},
		{"the second renderer fails after the first one rendered", fake_reader{contract: valid_contract()}, fake_renderer{}, fake_renderer{render_error: render_failure}, render_failure},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			var written []string
			use_case := application.NewGenerateCode(each.reader,
				application.Target{Renderer: each.first, Writer: fake_writer{written: &written}},
				application.Target{Renderer: each.second, Writer: fake_writer{written: &written}})
			if execute_error := use_case.Execute(); !errors.Is(execute_error, each.sentinel) {
				test.Errorf("error %v does not wrap %v", execute_error, each.sentinel)
			}
			if len(written) != 0 {
				test.Errorf("nothing must be written, got %v", written)
			}
		})
	}
}

func TestAWriteFailureIsReturned(test *testing.T) {
	var written []string
	cases := []struct {
		name          string
		go_writer     fake_writer
		python_writer fake_writer
	}{
		{"the Go writer fails", fake_writer{written: &written, write_error: write_failure}, fake_writer{written: &written}},
		{"the Python writer fails", fake_writer{written: &written}, fake_writer{written: &written, write_error: write_failure}},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			use_case := application.NewGenerateCode(fake_reader{contract: valid_contract()},
				application.Target{Renderer: fake_renderer{file: "a"}, Writer: each.go_writer},
				application.Target{Renderer: fake_renderer{file: "b"}, Writer: each.python_writer})
			if execute_error := use_case.Execute(); !errors.Is(execute_error, write_failure) {
				test.Errorf("error %v does not wrap %v", execute_error, write_failure)
			}
		})
	}
}
