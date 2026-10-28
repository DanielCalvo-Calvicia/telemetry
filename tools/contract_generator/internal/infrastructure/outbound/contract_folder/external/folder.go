// Package external does the file I/O of the contract reader: it lists and reads the files of a folder on disk.
package external

import (
	"io/fs"
	"os"
	"sort"
)

// Folder is a contracts folder on disk.
type Folder struct {
	root fs.FS
}

// NewFolder returns the folder at a directory.
func NewFolder(directory string) Folder {
	return Folder{root: os.DirFS(directory)}
}

// ReadFile returns the content of a file, given its slash path relative to the folder.
func (folder Folder) ReadFile(file_path string) ([]byte, error) {
	return fs.ReadFile(folder.root, file_path)
}

// Files returns every file of the folder as a slash path relative to it, in alphabetical order.
func (folder Folder) Files() ([]string, error) {
	var file_paths []string
	walk_error := fs.WalkDir(folder.root, ".", func(file_path string, entry fs.DirEntry, entry_error error) error {
		if entry_error != nil {
			return entry_error
		}
		if !entry.IsDir() {
			file_paths = append(file_paths, file_path)
		}
		return nil
	})
	sort.Strings(file_paths)
	return file_paths, walk_error
}
