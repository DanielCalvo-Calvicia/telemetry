// Package fixtures gives the tests access to the fixture contract and to its expected output, and a way to copy the
// contract and break it.
package fixtures

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// directory returns the folder of this package, which holds the fixture files.
func directory() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Dir(source)
}

// ContractDir is the fixture contract: two enums, an envelope-like schema, one event type and two error codes.
func ContractDir() string {
	return filepath.Join(directory(), "contract")
}

// Expected returns an expected output file ("go/enums_generated.go.golden"), with the line ends normalized to LF so
// a checkout that converts them does not change the comparison.
func Expected(test *testing.T, relative_path string) string {
	test.Helper()
	content, read_error := os.ReadFile(filepath.Join(directory(), "expected", filepath.FromSlash(relative_path)))
	if read_error != nil {
		test.Fatal(read_error)
	}
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}

// CopyContract copies the fixture contract into a new temporary folder and returns it, so a test can break the copy.
func CopyContract(test *testing.T) string {
	test.Helper()
	target := test.TempDir()
	walk_error := filepath.WalkDir(ContractDir(), func(source string, entry fs.DirEntry, entry_error error) error {
		if entry_error != nil || entry.IsDir() {
			return entry_error
		}
		relative, _ := filepath.Rel(ContractDir(), source)
		content, read_error := os.ReadFile(source)
		if read_error != nil {
			return read_error
		}
		Write(test, target, filepath.ToSlash(relative), string(content))
		return nil
	})
	if walk_error != nil {
		test.Fatal(walk_error)
	}
	return target
}

// Write writes a file, given its slash path relative to a folder, creating its folders.
func Write(test *testing.T, directory_path string, relative_path string, content string) {
	test.Helper()
	full_path := filepath.Join(directory_path, filepath.FromSlash(relative_path))
	if make_error := os.MkdirAll(filepath.Dir(full_path), 0o755); make_error != nil {
		test.Fatal(make_error)
	}
	if write_error := os.WriteFile(full_path, []byte(content), 0o644); write_error != nil {
		test.Fatal(write_error)
	}
}

// Remove deletes a file, given its slash path relative to a folder.
func Remove(test *testing.T, directory_path string, relative_path string) {
	test.Helper()
	if remove_error := os.Remove(filepath.Join(directory_path, filepath.FromSlash(relative_path))); remove_error != nil {
		test.Fatal(remove_error)
	}
}
