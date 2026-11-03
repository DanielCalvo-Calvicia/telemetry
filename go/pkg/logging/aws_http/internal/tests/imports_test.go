package aws_http_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const (
	core_module_path    = "github.com/DanielCalvo-Calvicia/telemetry/go"
	adapter_module_path = "github.com/DanielCalvo-Calvicia/telemetry/go/pkg/logging/aws_http"
)

// Test_the_module_imports_only_the_standard_library_the_core_and_itself checks the rule of the acceptance criteria: the
// module builds with its own dependencies and the core only, so no code of it imports a third-party package. It also
// keeps the adapter out of the old library: it never imports the module of the repository root.
func Test_the_module_imports_only_the_standard_library_the_core_and_itself(t *testing.T) {
	module_root := filepath.Join("..", "..")
	checked_files := 0
	walk_error := filepath.WalkDir(module_root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		parsed, parse_error := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parse_error != nil {
			return parse_error
		}
		checked_files++
		for _, spec := range parsed.Imports {
			import_path := strings.Trim(spec.Path.Value, "\"`")
			first_element, _, _ := strings.Cut(import_path, "/")
			is_standard_library := !strings.Contains(first_element, ".")
			is_core := import_path == core_module_path || strings.HasPrefix(import_path, core_module_path+"/")
			if !is_standard_library && !is_core {
				t.Errorf("%s imports %s, which is neither the standard library nor the core", path, import_path)
			}
			if is_core && !strings.HasPrefix(import_path, core_module_path+"/internal/") && !strings.HasPrefix(import_path, core_module_path+"/pkg/") &&
				import_path != adapter_module_path && !strings.HasPrefix(import_path, adapter_module_path+"/") {
				t.Errorf("%s imports %s, which is not a package of the core", path, import_path)
			}
		}
		return nil
	})
	if walk_error != nil {
		t.Fatal(walk_error)
	}
	if checked_files < 10 {
		t.Fatalf("only %d Go files were checked: the path of the module is wrong", checked_files)
	}
}
