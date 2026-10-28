package checker

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// goFilesUnder returns the non-test Go files under a folder; a folder that does not exist has none.
func goFilesUnder(folder string) ([]string, error) {
	var files []string
	walk_error := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if walk_error != nil && !errors.Is(walk_error, fs.ErrNotExist) {
		return nil, walk_error
	}
	sort.Strings(files)
	return files, nil
}

// importsOfFile returns the import paths of one Go file.
func importsOfFile(path string) ([]string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var imports []string
	for _, spec := range parsed.Imports {
		// The parser has already accepted the literal, so it is a plain quoted path.
		imports = append(imports, strings.Trim(spec.Path.Value, "\"`"))
	}
	return imports, nil
}

// isStandardLibrary tells whether an import path belongs to the standard library: its first element has no dot.
func isStandardLibrary(import_path string) bool {
	first_element, _, _ := strings.Cut(import_path, "/")
	return !strings.Contains(first_element, ".")
}

// underPath tells whether an import path is the given path or inside it.
func underPath(import_path string, base string) bool {
	return import_path == base || strings.HasPrefix(import_path, base+"/")
}
