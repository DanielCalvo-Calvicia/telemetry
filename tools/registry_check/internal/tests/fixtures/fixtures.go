// Package fixtures gives the tests access to the fixture registries, the small specification, the fixture root with its
// test files and the fixture tasks folder, and a way to write a variant of a registry into a temporary folder.
package fixtures

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// directory returns the folder of the fixtures: this package's folder, then registry.
func directory() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(source), "registry")
}

// SpecPath is the small specification: three requirements, REQ-AAA-001, REQ-AAA-002 and REQ-BBB-001.
func SpecPath() string {
	return filepath.Join(directory(), "spec.md")
}

// RootDir is the fixture root of the repository: it holds the test files the registries list under tests/.
func RootDir() string {
	return filepath.Join(directory(), "root")
}

// TasksDir is the fixture tasks folder: T-01, T-01.1, T-01.2, T-02 and T-02.1 exist.
func TasksDir() string {
	return filepath.Join(directory(), "tasks")
}

// RegistryPath is a fixture registry by name, without the extension: "valid" is registries/valid.yaml.
func RegistryPath(name string) string {
	return filepath.Join(directory(), "registries", name+".yaml")
}

// Variant writes a copy of a fixture registry in which a text is replaced and returns its path. The text must be in
// the registry, so a fixture that changes never makes a test pass without testing anything.
func Variant(test *testing.T, name string, old_text string, new_text string) string {
	test.Helper()
	content, read_error := os.ReadFile(RegistryPath(name))
	if read_error != nil {
		test.Fatal(read_error)
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.Contains(text, old_text) {
		test.Fatalf("the fixture %s does not contain %q", name, old_text)
	}
	return Write(test, name+"_variant.yaml", strings.Replace(text, old_text, new_text, 1))
}

// Write writes a file with a content into a new temporary folder and returns its path.
func Write(test *testing.T, name string, content string) string {
	test.Helper()
	file_path := filepath.Join(test.TempDir(), name)
	if write_error := os.WriteFile(file_path, []byte(content), 0o644); write_error != nil {
		test.Fatal(write_error)
	}
	return file_path
}
