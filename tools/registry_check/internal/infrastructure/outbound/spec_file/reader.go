// Package spec_file reads the requirement identifiers from the specification (PR-001): every requirement is defined by
// a bold paragraph start such as **REQ-ARC-001.** The I/O lives in its external folder.
package spec_file

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/domain"
)

// definition_pattern matches the definition of a requirement: a line that starts with **REQ-AAA-000.**
var definition_pattern = regexp.MustCompile(`(?m)^\*\*(REQ-[A-Z]{2,4}-[0-9]{3})\.\*\*`)

// File is the access to the specification file.
type File interface {
	// Read returns the content of the file.
	Read() ([]byte, error)
	// Path returns the path of the file, to name it in messages.
	Path() string
}

// Reader reads a specification file.
type Reader struct {
	file File
}

// NewReader returns a reader of the file.
func NewReader(file File) Reader {
	return Reader{file: file}
}

// Read returns the identifiers the file defines, without repeats, sorted.
func (reader Reader) Read() (domain.Spec, error) {
	content, read_error := reader.file.Read()
	if read_error != nil {
		return domain.Spec{}, fmt.Errorf("%w: %s: %w", application.ErrReadSpec, reader.file.Path(), read_error)
	}
	seen := map[string]bool{}
	var ids []string
	for _, match := range definition_pattern.FindAllSubmatch(content, -1) {
		id := string(match[1])
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return domain.Spec{Path: reader.file.Path(), IDs: ids}, nil
}
