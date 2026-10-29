// Package external does the file I/O of the specification reader.
package external

import "os"

// File is a file on disk.
type File struct {
	path string
}

// NewFile returns the file at a path.
func NewFile(path string) File {
	return File{path: path}
}

// Path returns the path the file was created with.
func (file File) Path() string {
	return file.path
}

// Read returns the content of the file.
func (file File) Read() ([]byte, error) {
	return os.ReadFile(file.path)
}
