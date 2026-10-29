package application

import (
	"fmt"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

// Target is one output of the generator: the renderer of one kind of file in one language and the writer of the
// folder where those files go.
type Target struct {
	// Renderer turns the contract into the files of this output.
	Renderer CodeRenderer
	// Writer writes those files into their folder.
	Writer FileWriter
}

// GenerateCode is the use case of the generator: read the contract, check it, render every output and write them.
// Nothing is written unless the contract is valid and every output rendered.
type GenerateCode struct {
	reader  ContractReader
	targets []Target
}

// NewGenerateCode builds the use case from its ports: the reader and the outputs, which are rendered and written in
// the order they are given.
func NewGenerateCode(reader ContractReader, targets ...Target) GenerateCode {
	return GenerateCode{reader: reader, targets: targets}
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
	if validatable_error := contract.CheckValidatable(); validatable_error != nil {
		return validatable_error
	}
	rendered := make([][]domain.GeneratedFile, len(use_case.targets))
	for position, target := range use_case.targets {
		files, render_error := target.Renderer.Render(contract)
		if render_error != nil {
			return render_error
		}
		rendered[position] = files
	}
	for position, target := range use_case.targets {
		if write_error := target.Writer.Write(rendered[position]); write_error != nil {
			return write_error
		}
	}
	return nil
}
