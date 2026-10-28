package application

import "fmt"

// GenerateCode is the use case of the generator: read the contract, check it, render the code of both languages and
// write it. Nothing is written unless the contract is valid and both languages rendered.
type GenerateCode struct {
	reader          ContractReader
	go_renderer     CodeRenderer
	python_renderer CodeRenderer
	go_writer       FileWriter
	python_writer   FileWriter
}

// NewGenerateCode builds the use case from its ports.
func NewGenerateCode(
	reader ContractReader,
	go_renderer CodeRenderer,
	python_renderer CodeRenderer,
	go_writer FileWriter,
	python_writer FileWriter,
) GenerateCode {
	return GenerateCode{
		reader:          reader,
		go_renderer:     go_renderer,
		python_renderer: python_renderer,
		go_writer:       go_writer,
		python_writer:   python_writer,
	}
}

// Execute runs the use case. The same contract always gives the same bytes.
func (use_case GenerateCode) Execute() error {
	contract, read_error := use_case.reader.Read()
	if read_error != nil {
		return read_error
	}
	contract = contract.Sorted()
	if validation_error := contract.Validate(); validation_error != nil {
		return fmt.Errorf("%w:\n%w", ErrInvalidContract, validation_error)
	}
	go_files, go_error := use_case.go_renderer.Render(contract)
	if go_error != nil {
		return go_error
	}
	python_files, python_error := use_case.python_renderer.Render(contract)
	if python_error != nil {
		return python_error
	}
	if write_error := use_case.go_writer.Write(go_files); write_error != nil {
		return write_error
	}
	return use_case.python_writer.Write(python_files)
}
