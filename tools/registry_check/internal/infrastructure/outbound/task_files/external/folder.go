// Package external does the file I/O of the catalog of tasks: it lists the files of the tasks folder.
package external

import (
	"io/fs"
	"os"
	"sort"
)

// Folder is the tasks folder on disk.
type Folder struct {
	root fs.FS
	path string
}

// NewFolder returns the folder at a directory.
func NewFolder(directory string) Folder {
	return Folder{root: os.DirFS(directory), path: directory}
}

// Files returns every file of the folder, in every sub folder, as a slash path relative to it, in alphabetical order.
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

// Path returns the directory the folder was created with, to name it in messages.
func (folder Folder) Path() string {
	return folder.path
}
