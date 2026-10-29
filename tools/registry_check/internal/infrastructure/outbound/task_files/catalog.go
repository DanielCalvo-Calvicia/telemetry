// Package task_files knows which tasks of the plan exist, from the names of the files of the tasks folder. The I/O
// lives in its external folder.
package task_files

import (
	"fmt"
	"path"
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/tools/registry_check/internal/application"
)

// task_file_pattern matches the name of a task file: T-04.7_replace-and-remove-the-old-library.md or T-04_name.md.
var task_file_pattern = regexp.MustCompile(`^(T-[0-9]{2}(\.[0-9]+)?)_.+\.md$`)

// Folder is the access to the files of the tasks folder.
type Folder interface {
	// Files returns every file of the folder, in every sub folder, as a slash path relative to it.
	Files() ([]string, error)
	// Path returns the directory of the folder, to name it in messages.
	Path() string
}

// Catalog is the set of tasks of a tasks folder. The folder is read the first time a task is asked for.
type Catalog struct {
	folder Folder
	ids    map[string]bool
}

// NewCatalog returns the catalog of a tasks folder.
func NewCatalog(folder Folder) *Catalog {
	return &Catalog{folder: folder}
}

// Exists reports whether a task identifier such as T-04.7 has a file in the folder.
func (catalog *Catalog) Exists(task_id string) (bool, error) {
	if catalog.ids == nil {
		file_paths, list_error := catalog.folder.Files()
		if list_error != nil {
			return false, fmt.Errorf("%w: %s: %w", application.ErrReadTasks, catalog.folder.Path(), list_error)
		}
		catalog.ids = map[string]bool{}
		for _, file_path := range file_paths {
			if match := task_file_pattern.FindStringSubmatch(path.Base(file_path)); match != nil {
				catalog.ids[match[1]] = true
			}
		}
	}
	return catalog.ids[task_id], nil
}
