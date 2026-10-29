// Package external does the file I/O of the finder of test files.
package external

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Disk is the root of the repository on disk.
type Disk struct {
	root string
}

// NewDisk returns the disk under a root folder.
func NewDisk(root string) Disk {
	return Disk{root: root}
}

// IsFile reports whether a slash path, relative to the root, is a regular file. A path that does not exist is not an
// error: it is false.
func (disk Disk) IsFile(relative_path string) (bool, error) {
	info, stat_error := os.Stat(filepath.Join(disk.root, filepath.FromSlash(relative_path)))
	if errors.Is(stat_error, fs.ErrNotExist) {
		return false, nil
	}
	if stat_error != nil {
		return false, stat_error
	}
	return info.Mode().IsRegular(), nil
}
