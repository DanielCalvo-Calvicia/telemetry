package application

import "github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"

// ContractReader reads the contract folder into the model. It stops with an error that names the file when a file
// is not valid JSON, not a valid JSON Schema or not the expected YAML.
type ContractReader interface {
	// Read returns the contract of the folder the reader was created for.
	Read() (domain.Contract, error)
}

// CodeRenderer turns the contract into the files of one language.
type CodeRenderer interface {
	// Render returns the files of the language, in a deterministic order and with a deterministic content.
	Render(contract domain.Contract) ([]domain.GeneratedFile, error)
}

// FileWriter writes the generated files into the output folder it was created for.
type FileWriter interface {
	// Write writes every file, replacing the ones that exist.
	Write(files []domain.GeneratedFile) error
}
