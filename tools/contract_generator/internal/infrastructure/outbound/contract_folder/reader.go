// Package contract_folder reads the contracts folder (enums, JSON Schemas, the registry of event types and the catalog
// of error codes) into the model of the domain. The I/O lives in its external folder.
package contract_folder

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

const (
	schema_suffix = ".schema.json"
	common_folder = "common"
)

// Folder is the access to the files of the contracts folder.
type Folder interface {
	// ReadFile returns the content of a file, given its slash path relative to the folder.
	ReadFile(file_path string) ([]byte, error)
	// Files returns every file of the folder as a slash path relative to it, in alphabetical order.
	Files() ([]string, error)
}

// Reader reads a contracts folder.
type Reader struct {
	folder Folder
}

// NewReader returns a reader of the folder.
func NewReader(folder Folder) Reader {
	return Reader{folder: folder}
}

// document is a JSON file of the contract, parsed.
type document struct {
	path string
	id   string
	body map[string]any
}

// Read reads the contract. It checks that every enum and schema file is valid JSON and a valid JSON Schema of
// the draft 2020-12 whose references resolve, and it reports every file that is not, each one by its path.
func (reader Reader) Read() (domain.Contract, error) {
	file_paths, list_error := reader.folder.Files()
	if list_error != nil {
		return domain.Contract{}, fmt.Errorf("%w: %w", application.ErrReadContract, list_error)
	}
	documents, documents_error := reader.read_documents(file_paths)
	if documents_error != nil {
		return domain.Contract{}, documents_error
	}
	if schemas_error := check_schemas(documents); schemas_error != nil {
		return domain.Contract{}, schemas_error
	}
	registry, registry_error := read_yaml[registry_file](reader.folder, domain.RegistryPath)
	if registry_error != nil {
		return domain.Contract{}, registry_error
	}
	catalog, catalog_error := read_yaml[catalog_file](reader.folder, domain.CatalogPath)
	if catalog_error != nil {
		return domain.Contract{}, catalog_error
	}
	return build_contract(documents, registry, catalog)
}

// read_documents reads and parses every enum file and every schema file.
func (reader Reader) read_documents(file_paths []string) ([]document, error) {
	var documents []document
	var failures []error
	for _, file_path := range file_paths {
		if !is_enum_file(file_path) && !is_schema_file(file_path) {
			continue
		}
		content, read_error := reader.folder.ReadFile(file_path)
		if read_error != nil {
			failures = append(failures, fmt.Errorf("%w: %s%s: %w", application.ErrReadContract, domain.SourcePrefix, file_path, read_error))
			continue
		}
		parsed, parse_error := jsonschema.UnmarshalJSON(bytes.NewReader(content))
		if parse_error != nil {
			failures = append(failures, fmt.Errorf("%w: %s%s: %w", application.ErrInvalidJSON, domain.SourcePrefix, file_path, parse_error))
			continue
		}
		body, is_object := parsed.(map[string]any)
		if !is_object {
			failures = append(failures, fmt.Errorf("%w: %s%s: the document is not an object", application.ErrInvalidSchema, domain.SourcePrefix, file_path))
			continue
		}
		identifier, _ := body["$id"].(string)
		documents = append(documents, document{path: file_path, id: identifier, body: body})
	}
	return documents, errors.Join(failures...)
}

// check_schemas compiles every document as a JSON Schema, with every document known by its $id so the references
// between them resolve without the network.
func check_schemas(documents []document) error {
	compiler := jsonschema.NewCompiler()
	var failures []error
	for _, declared := range documents {
		if declared.id == "" {
			failures = append(failures, fmt.Errorf("%w: %s%s: no $id", application.ErrInvalidSchema, domain.SourcePrefix, declared.path))
			continue
		}
		if add_error := compiler.AddResource(declared.id, declared.body); add_error != nil {
			failures = append(failures, fmt.Errorf("%w: %s%s: %w", application.ErrInvalidSchema, domain.SourcePrefix, declared.path, add_error))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	for _, declared := range documents {
		if _, compile_error := compiler.Compile(declared.id); compile_error != nil {
			failures = append(failures, fmt.Errorf("%w: %s%s: %w", application.ErrInvalidSchema, domain.SourcePrefix, declared.path, compile_error))
		}
	}
	return errors.Join(failures...)
}

// is_enum_file tells if a path is enums/<name>.json.
func is_enum_file(file_path string) bool {
	return path.Dir(file_path) == "enums" && path.Ext(file_path) == ".json"
}

// is_schema_file tells if a path is a schema file under schemas/.
func is_schema_file(file_path string) bool {
	return strings.HasPrefix(file_path, "schemas/") && strings.HasSuffix(file_path, schema_suffix)
}

// read_yaml reads and decodes a YAML file of the contract.
func read_yaml[Decoded any](folder Folder, file_path string) (Decoded, error) {
	var decoded Decoded
	content, read_error := folder.ReadFile(file_path)
	if read_error != nil {
		return decoded, fmt.Errorf("%w: %s%s: %w", application.ErrReadContract, domain.SourcePrefix, file_path, read_error)
	}
	if decode_error := yaml.Unmarshal(content, &decoded); decode_error != nil {
		return decoded, fmt.Errorf("%w: %s%s: %w", application.ErrInvalidYAML, domain.SourcePrefix, file_path, decode_error)
	}
	return decoded, nil
}
