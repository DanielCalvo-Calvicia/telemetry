// Package test_files tells whether the test files a registry lists exist. The I/O lives in its external folder.
package test_files

// Disk is the access to the files under the root of the repository.
type Disk interface {
	// IsFile reports whether a slash path relative to the root is a regular file; a path that does not exist is false
	// and not an error.
	IsFile(relative_path string) (bool, error)
}

// Finder looks for test files.
type Finder struct {
	disk Disk
}

// NewFinder returns a finder over a disk.
func NewFinder(disk Disk) Finder {
	return Finder{disk: disk}
}

// Exists reports whether the slash path, relative to the root of the repository, is a file that exists.
func (finder Finder) Exists(test_path string) (bool, error) {
	return finder.disk.IsFile(test_path)
}
