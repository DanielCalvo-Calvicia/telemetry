// Package external does the file I/O of the writer of generated files.
package external

import (
	"os"
	"path/filepath"
)

// Disk writes files into folders of the disk.
type Disk struct{}

// WriteFile writes a file with exactly the given bytes, creating the folder if it does not exist.
func (Disk) WriteFile(directory string, name string, content []byte) error {
	if make_error := os.MkdirAll(directory, 0o755); make_error != nil {
		return make_error
	}
	return os.WriteFile(filepath.Join(directory, name), content, 0o644)
}
