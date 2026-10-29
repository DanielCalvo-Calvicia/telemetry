// Package registry_file reads the registry of capabilities (contracts/capabilities/registry.yaml) into the model of
// the domain. The I/O lives in its external folder.
package registry_file

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
)

// File is the access to the registry file.
type File interface {
	// Read returns the content of the file.
	Read() ([]byte, error)
	// Path returns the path of the file, to name it in messages.
	Path() string
}

// Reader reads a registry file.
type Reader struct {
	file File
}

// NewReader returns a reader of the file.
func NewReader(file File) Reader {
	return Reader{file: file}
}

type language_document struct {
	State  string `yaml:"state"`
	Reason string `yaml:"reason"`
}

type languages_document struct {
	Go     language_document `yaml:"go"`
	Python language_document `yaml:"python"`
}

type test_document struct {
	Language string `yaml:"language"`
	Path     string `yaml:"path"`
}

type requirement_document struct {
	ID      string          `yaml:"id"`
	Tests   []test_document `yaml:"tests"`
	Pending string          `yaml:"pending"`
}

type capability_document struct {
	ID            string                 `yaml:"id"`
	Title         string                 `yaml:"title"`
	Level         string                 `yaml:"level"`
	Status        string                 `yaml:"status"`
	Description   string                 `yaml:"description"`
	PublicAPI     []string               `yaml:"public_api"`
	Schemas       []string               `yaml:"schemas"`
	Configuration []string               `yaml:"configuration"`
	Errors        []string               `yaml:"errors"`
	Ports         []string               `yaml:"ports"`
	Adapters      []string               `yaml:"adapters"`
	Languages     languages_document     `yaml:"languages"`
	Requirements  []requirement_document `yaml:"requirements"`
}

type registry_document struct {
	RegistryVersion int                   `yaml:"registry_version"`
	Capabilities    []capability_document `yaml:"capabilities"`
}

// Read reads the registry. An unknown key is an error, so a misspelled field is never ignored; the error names the file.
func (reader Reader) Read() (domain.Registry, error) {
	content, read_error := reader.file.Read()
	if read_error != nil {
		return domain.Registry{}, fmt.Errorf("%w: %s: %w", application.ErrReadRegistry, reader.file.Path(), read_error)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	var document registry_document
	if decode_error := decoder.Decode(&document); decode_error != nil {
		return domain.Registry{}, fmt.Errorf("%w: %s: %w", application.ErrInvalidYAML, reader.file.Path(), decode_error)
	}
	registry := domain.Registry{Path: reader.file.Path(), Version: document.RegistryVersion}
	for _, capability := range document.Capabilities {
		registry.Capabilities = append(registry.Capabilities, to_capability(capability))
	}
	return registry, nil
}

func to_capability(document capability_document) domain.Capability {
	capability := domain.Capability{
		ID: document.ID, Title: document.Title, Level: document.Level, Status: document.Status,
		Description: document.Description, PublicAPI: document.PublicAPI, Schemas: document.Schemas,
		Configuration: document.Configuration, Errors: document.Errors, Ports: document.Ports, Adapters: document.Adapters,
		Go:     domain.LanguageState{State: document.Languages.Go.State, Reason: document.Languages.Go.Reason},
		Python: domain.LanguageState{State: document.Languages.Python.State, Reason: document.Languages.Python.Reason},
	}
	for _, requirement := range document.Requirements {
		entry := domain.Requirement{ID: requirement.ID, Pending: requirement.Pending}
		for _, test := range requirement.Tests {
			entry.Tests = append(entry.Tests, domain.TestRef{Language: test.Language, Path: test.Path})
		}
		capability.Requirements = append(capability.Requirements, entry)
	}
	return capability
}
