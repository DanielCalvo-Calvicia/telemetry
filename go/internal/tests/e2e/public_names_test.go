package e2e_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const public_package_folder = "../../../pkg/telemetry"

type public_name struct {
	Go string `json:"go"`
}

type public_names_file struct {
	Functions []public_name `json:"functions"`
	Types     []public_name `json:"types"`
}

// exported_identifier is one exported declaration of the public package with its doc comment.
type exported_identifier struct {
	name string
	kind string
	doc  string
}

func parse_public_package(t *testing.T) []exported_identifier {
	t.Helper()
	file_set := token.NewFileSet()
	packages, parse_error := parser.ParseDir(file_set, public_package_folder, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if parse_error != nil {
		t.Fatal(parse_error)
	}
	package_found, has_package := packages["telemetry"]
	if !has_package {
		t.Fatalf("no package telemetry in %s", public_package_folder)
	}

	var identifiers []exported_identifier
	for _, file := range package_found.Files {
		for _, declaration := range file.Decls {
			switch typed_declaration := declaration.(type) {
			case *ast.FuncDecl:
				if typed_declaration.Recv == nil && typed_declaration.Name.IsExported() {
					identifiers = append(identifiers, exported_identifier{typed_declaration.Name.Name, "func", typed_declaration.Doc.Text()})
				}
			case *ast.GenDecl:
				for _, spec := range typed_declaration.Specs {
					switch typed_spec := spec.(type) {
					case *ast.TypeSpec:
						doc := typed_spec.Doc
						if doc == nil && len(typed_declaration.Specs) == 1 {
							doc = typed_declaration.Doc
						}
						if typed_spec.Name.IsExported() {
							identifiers = append(identifiers, exported_identifier{typed_spec.Name.Name, "type", doc.Text()})
						}
					case *ast.ValueSpec:
						doc := typed_spec.Doc
						if doc == nil && len(typed_declaration.Specs) == 1 {
							doc = typed_declaration.Doc
						}
						for _, name := range typed_spec.Names {
							if name.IsExported() {
								identifiers = append(identifiers, exported_identifier{name.Name, typed_declaration.Tok.String(), doc.Text()})
							}
						}
					}
				}
			}
		}
	}
	return identifiers
}

func Test_the_public_package_exports_every_name_of_the_shared_list(t *testing.T) {
	content, read_error := os.ReadFile(filepath.Join(core_fixtures_folder, "public_names.json"))
	if read_error != nil {
		t.Fatal(read_error)
	}
	var names public_names_file
	if decode_error := json.Unmarshal(content, &names); decode_error != nil {
		t.Fatal(decode_error)
	}
	if len(names.Functions) == 0 || len(names.Types) == 0 {
		t.Fatal("public_names.json lists no function or no type")
	}

	exported_kinds := map[string]string{}
	for _, identifier := range parse_public_package(t) {
		exported_kinds[identifier.name] = identifier.kind
	}
	for _, function := range names.Functions {
		if exported_kinds[function.Go] != "func" {
			t.Errorf("the function %s is not exported by the public package", function.Go)
		}
	}
	for _, type_name := range names.Types {
		if exported_kinds[type_name.Go] != "type" {
			t.Errorf("the type %s is not exported by the public package", type_name.Go)
		}
	}
}

func Test_every_exported_identifier_has_a_doc_comment_that_starts_with_its_name(t *testing.T) {
	identifiers := parse_public_package(t)
	if len(identifiers) == 0 {
		t.Fatal("the public package exports nothing")
	}
	for _, identifier := range identifiers {
		if !strings.HasPrefix(identifier.doc, identifier.name+" ") {
			t.Errorf("the %s %s has no doc comment that starts with its name (got %q)", identifier.kind, identifier.name, identifier.doc)
		}
	}
}
