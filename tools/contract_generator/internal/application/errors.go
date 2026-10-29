package application

import (
	"errors"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

// The errors of the use case and of the ports. Each one is wrapped with the path of the file it is about.
var (
	// ErrReadContract is returned when a contract file cannot be read or the folder cannot be listed.
	ErrReadContract = errors.New("cannot read the contract")
	// ErrInvalidJSON is returned for a contract file that is not valid JSON.
	ErrInvalidJSON = errors.New("invalid JSON")
	// ErrInvalidYAML is returned for a registry or a catalog that is not valid YAML of the expected shape.
	ErrInvalidYAML = errors.New("invalid YAML")
	// ErrInvalidSchema is returned for a file that is not a valid JSON Schema (draft 2020-12) whose references resolve.
	ErrInvalidSchema = errors.New("invalid JSON Schema")
	// ErrUnsupportedSchema is returned for a schema that uses a construct the generator cannot turn into a field or into validation code.
	ErrUnsupportedSchema = domain.ErrUnsupportedSchema
	// ErrInvalidContract is returned when the contract breaks one of its rules; it wraps the breaches.
	ErrInvalidContract = errors.New("the contract is not valid")
	// ErrRenderCode is returned when a renderer cannot produce a file.
	ErrRenderCode = errors.New("cannot render the code")
	// ErrWriteOutput is returned when a generated file cannot be written.
	ErrWriteOutput = errors.New("cannot write the generated code")
)
