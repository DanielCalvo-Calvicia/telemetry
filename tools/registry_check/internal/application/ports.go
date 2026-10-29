package application

import "github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"

// RegistryReader reads the registry file into the model. It stops with an error that names the file when the file
// cannot be read or is not the expected YAML.
type RegistryReader interface {
	// Read returns the registry of the file the reader was created for.
	Read() (domain.Registry, error)
}

// SpecReader reads the identifiers of the requirements that the specification defines.
type SpecReader interface {
	// Read returns the identifiers of the file the reader was created for.
	Read() (domain.Spec, error)
}

// TestFiles tells whether a test file of the repository exists.
type TestFiles interface {
	// Exists reports whether the slash path, relative to the root of the repository, is a file that exists.
	Exists(test_path string) (bool, error)
}

// Tasks tells whether a task of the plan exists.
type Tasks interface {
	// Exists reports whether a task identifier such as T-04.7 has its file in the tasks folder.
	Exists(task_id string) (bool, error)
}
