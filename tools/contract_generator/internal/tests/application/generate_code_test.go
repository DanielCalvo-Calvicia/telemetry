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
		fake_renderer{file: "go_file", received: &received},
		fake_renderer{file: "python_file"},
		fake_writer{written: &written},
		fake_writer{written: &written},
	)
	if execute_error := use_case.Execute(); execute_error != nil {
		test.Fatal(execute_error)
	}
	if len(written) != 2 || written[0] != "go_file" || written[1] != "python_file" {
		test.Errorf("written: %v", written)
	}
	if received.Enums[0].Name != "failure_category" {
		test.Error("the renderer must receive the sorted contract")
	}
}

func TestNothingIsWrittenWhenTheContractCannotBeUsed(test *testing.T) {
	invalid := valid_contract()
	invalid.Enums[0].Values = []string{"INFO", "INFO"}
	cases := []struct {
		name        string
		reader      fake_reader
		go_renderer fake_renderer
		py_renderer fake_renderer
		sentinel    error
	}{
		{"the contract cannot be read", fake_reader{read_error: read_failure}, fake_renderer{}, fake_renderer{}, read_failure},
		{"the contract is invalid", fake_reader{contract: invalid}, fake_renderer{}, fake_renderer{}, application.ErrInvalidContract},
		{"the Go renderer fails", fake_reader{contract: valid_contract()}, fake_renderer{render_error: render_failure}, fake_renderer{}, render_failure},
		{"the Python renderer fails", fake_reader{contract: valid_contract()}, fake_renderer{}, fake_renderer{render_error: render_failure}, render_failure},
	}
	for _, each := range cases {
		test.Run(each.name, func(test *testing.T) {
			var written []string
			use_case := application.NewGenerateCode(each.reader, each.go_renderer, each.py_renderer, fake_writer{written: &written}, fake_writer{written: &written})
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
			use_case := application.NewGenerateCode(fake_reader{contract: valid_contract()}, fake_renderer{file: "a"}, fake_renderer{file: "b"}, each.go_writer, each.python_writer)
			if execute_error := use_case.Execute(); !errors.Is(execute_error, write_failure) {
				test.Errorf("error %v does not wrap %v", execute_error, write_failure)
			}
		})
	}
}
