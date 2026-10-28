// Package file_writer writes the generated files into an output folder. The I/O lives in its external folder.
package file_writer

import (
	"fmt"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/tools/contract_generator/internal/domain"
)

// Disk is the access to the disk the writer needs.
type Disk interface {
	// WriteFile writes a file in a folder, creating the folder if needed.
	WriteFile(directory string, name string, content []byte) error
}

// Writer writes the files of one language into one folder.
type Writer struct {
	disk      Disk
	directory string
}

// NewWriter returns a writer of the folder.
func NewWriter(disk Disk, directory string) Writer {
	return Writer{disk: disk, directory: directory}
}

// Write writes every file, replacing the ones that exist, and stops at the first one it cannot write.
func (writer Writer) Write(files []domain.GeneratedFile) error {
	for _, generated := range files {
		if write_error := writer.disk.WriteFile(writer.directory, generated.Name, generated.Content); write_error != nil {
			return fmt.Errorf("%w: %s/%s: %w", application.ErrWriteOutput, writer.directory, generated.Name, write_error)
		}
	}
	return nil
}
